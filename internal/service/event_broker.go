package service

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/krakosik/backend/internal/dto"
	"github.com/krakosik/backend/internal/model"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// EventSubscriber represents a subscriber to the event broker
type EventSubscriber struct {
	ID       string
	Events   chan model.Event
	Location LocationInfo
}

// LocationInfo stores the last known location of a subscriber
type LocationInfo struct {
	Latitude  float64
	Longitude float64
	Timestamp int64
}

// EventBroker handles the pub/sub system for events
type EventBroker interface {
	Subscribe(id string) *EventSubscriber
	Unsubscribe(id string)
	Publish(event model.Event)
	UpdateLocation(id string, latitude, longitude float64, timestamp int64)
	GetLocation(id string) (LocationInfo, bool)
	Close() error
}

type rabbitMQEventBroker struct {
	conn            *amqp.Connection
	channel         *amqp.Channel
	exchangeName    string
	subscribers     map[string]*EventSubscriber
	subscriberMutex sync.RWMutex
	locationCache   map[string]LocationInfo
	locationMutex   sync.RWMutex
}

func newEventBroker(config dto.Config) EventBroker {
	// Connection to RabbitMQ
	// Get RabbitMQ URL from config, with fallback to a default value
	connectionStr := config.RabbitMQURL
	if connectionStr == "" {
		connectionStr = "amqp://guest:guest@rabbitmq:5672/"
	}

	conn, err := amqp.Dial(connectionStr)
	if err != nil {
		logrus.Errorf("Failed to connect to RabbitMQ: %v", err)
		// Fall back to in-memory broker if RabbitMQ is not available
		// This allows the application to start even if RabbitMQ is not yet ready
		return newInMemoryEventBroker()
	}

	ch, err := conn.Channel()
	if err != nil {
		logrus.Errorf("Failed to open a channel: %v", err)
		conn.Close()
		return newInMemoryEventBroker()
	}

	// Declare exchange for publishing events
	exchangeName := "events"
	err = ch.ExchangeDeclare(
		exchangeName, // name
		"fanout",     // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		logrus.Errorf("Failed to declare an exchange: %v", err)
		ch.Close()
		conn.Close()
		return newInMemoryEventBroker()
	}

	broker := &rabbitMQEventBroker{
		conn:          conn,
		channel:       ch,
		exchangeName:  exchangeName,
		subscribers:   make(map[string]*EventSubscriber),
		locationCache: make(map[string]LocationInfo),
	}

	// Setup a system to reconnect if the connection is lost
	go broker.monitorConnection(connectionStr)

	return broker
}

// monitorConnection watches the connection and tries to reconnect if it's lost
func (b *rabbitMQEventBroker) monitorConnection(connectionStr string) {
	connCloseChan := make(chan *amqp.Error)
	b.conn.NotifyClose(connCloseChan)

	// Block until connection is closed
	err := <-connCloseChan
	logrus.Errorf("RabbitMQ connection closed: %v", err)

	// Try to reconnect
	for {
		time.Sleep(5 * time.Second) // Wait before reconnecting

		logrus.Info("Attempting to reconnect to RabbitMQ...")
		conn, err := amqp.Dial(connectionStr)
		if err != nil {
			logrus.Errorf("Failed to reconnect to RabbitMQ: %v", err)
			continue
		}

		ch, err := conn.Channel()
		if err != nil {
			logrus.Errorf("Failed to open a channel: %v", err)
			conn.Close()
			continue
		}

		// Redeclare exchange
		err = ch.ExchangeDeclare(
			b.exchangeName, // name
			"fanout",       // type
			true,           // durable
			false,          // auto-deleted
			false,          // internal
			false,          // no-wait
			nil,            // arguments
		)
		if err != nil {
			logrus.Errorf("Failed to declare an exchange: %v", err)
			ch.Close()
			conn.Close()
			continue
		}

		// Update broker with new connection and channel
		b.subscriberMutex.Lock()
		oldConn := b.conn
		oldChannel := b.channel
		b.conn = conn
		b.channel = ch
		b.subscriberMutex.Unlock()

		// Close old connection and channel
		if oldChannel != nil {
			oldChannel.Close()
		}
		if oldConn != nil {
			oldConn.Close()
		}

		// Resubscribe all clients
		b.resubscribeAll()

		// Start monitoring the new connection
		go b.monitorConnection(connectionStr)
		break
	}
}

// resubscribeAll recreates all subscriptions after a reconnection
func (b *rabbitMQEventBroker) resubscribeAll() {
	b.subscriberMutex.RLock()
	defer b.subscriberMutex.RUnlock()

	for id, subscriber := range b.subscribers {
		// Create a new queue for this subscriber
		q, err := b.channel.QueueDeclare(
			"",    // name - let RabbitMQ generate a unique name
			false, // durable
			true,  // delete when unused
			true,  // exclusive
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			logrus.Errorf("Failed to declare a queue for subscriber %s: %v", id, err)
			continue
		}

		// Bind the queue to the exchange
		err = b.channel.QueueBind(
			q.Name,         // queue name
			"",             // routing key
			b.exchangeName, // exchange
			false,          // no-wait
			nil,            // arguments
		)
		if err != nil {
			logrus.Errorf("Failed to bind queue for subscriber %s: %v", id, err)
			continue
		}

		// Start consuming from the queue
		msgs, err := b.channel.Consume(
			q.Name, // queue
			"",     // consumer
			true,   // auto-ack
			true,   // exclusive
			false,  // no-local
			false,  // no-wait
			nil,    // args
		)
		if err != nil {
			logrus.Errorf("Failed to register a consumer for subscriber %s: %v", id, err)
			continue
		}

		// Process messages in a separate goroutine
		go func(subscriberID string, subscriberChan chan model.Event, deliveries <-chan amqp.Delivery) {
			for d := range deliveries {
				var event model.Event
				if err := json.Unmarshal(d.Body, &event); err != nil {
					logrus.Errorf("Error unmarshaling event for subscriber %s: %v", subscriberID, err)
					continue
				}

				// Non-blocking send to the subscriber
				select {
				case subscriberChan <- event:
					// Event sent successfully
				default:
					// Subscriber's channel is full, skip this event
				}
			}
		}(id, subscriber.Events, msgs)
	}
}

// Subscribe creates a new subscriber and returns it
func (b *rabbitMQEventBroker) Subscribe(id string) *EventSubscriber {
	b.subscriberMutex.Lock()
	defer b.subscriberMutex.Unlock()

	// Check if subscriber already exists
	if subscriber, exists := b.subscribers[id]; exists {
		return subscriber
	}

	// Create a new subscriber
	subscriber := &EventSubscriber{
		ID:     id,
		Events: make(chan model.Event, 100), // Buffered channel to prevent blocking
	}

	// Create a queue for this subscriber
	q, err := b.channel.QueueDeclare(
		"",    // name - let RabbitMQ generate a unique name
		false, // durable
		true,  // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		logrus.Errorf("Failed to declare a queue: %v", err)
		return subscriber // Return the subscriber anyway, but it won't receive messages
	}

	// Bind the queue to the exchange
	err = b.channel.QueueBind(
		q.Name,         // queue name
		"",             // routing key
		b.exchangeName, // exchange
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		logrus.Errorf("Failed to bind queue: %v", err)
		return subscriber
	}

	// Start consuming from the queue
	msgs, err := b.channel.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		true,   // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		logrus.Errorf("Failed to register a consumer: %v", err)
		return subscriber
	}

	// Store the subscriber before starting the goroutine to ensure it's recorded
	b.subscribers[id] = subscriber

	// Process messages in a separate goroutine
	go func() {
		for d := range msgs {
			// Check if the subscriber still exists and channel is not closed
			b.subscriberMutex.RLock()
			existingSubscriber, exists := b.subscribers[id]
			stillActive := exists && existingSubscriber == subscriber
			b.subscriberMutex.RUnlock()

			if !stillActive {
				// Subscriber has been removed or replaced, stop processing
				return
			}

			var event model.Event
			if err := json.Unmarshal(d.Body, &event); err != nil {
				logrus.Errorf("Error unmarshaling event: %v", err)
				continue
			}

			// Non-blocking send to the subscriber with panic recovery
			func() {
				defer func() {
					if r := recover(); r != nil {
						logrus.Errorf("Recovered from panic in message delivery: %v", r)
					}
				}()

				// Double check the subscriber is still active before sending
				b.subscriberMutex.RLock()
				stillExists := b.subscribers[id] == subscriber
				b.subscriberMutex.RUnlock()

				if !stillExists {
					return
				}

				select {
				case subscriber.Events <- event:
					// Event sent successfully
				default:
					// Subscriber's channel is full, skip this event
				}
			}()
		}
	}()

	return subscriber
}

// Unsubscribe removes a subscriber from the broker
func (b *rabbitMQEventBroker) Unsubscribe(id string) {
	b.subscriberMutex.Lock()

	if subscriber, exists := b.subscribers[id]; exists {
		// First remove from the map before closing the channel
		delete(b.subscribers, id)
		// Then close the channel
		close(subscriber.Events)
	}

	b.subscriberMutex.Unlock()

	// Remove from location cache
	b.locationMutex.Lock()
	delete(b.locationCache, id)
	b.locationMutex.Unlock()
}

// Publish sends an event to all subscribers via RabbitMQ
func (b *rabbitMQEventBroker) Publish(event model.Event) {
	// Convert event to JSON
	eventJson, err := json.Marshal(event)
	if err != nil {
		logrus.Errorf("Error marshaling event: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Publish to the exchange
	err = b.channel.PublishWithContext(
		ctx,
		b.exchangeName, // exchange
		"",             // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        eventJson,
		})
	if err != nil {
		logrus.Errorf("Error publishing event: %v", err)
	}
}

// UpdateLocation updates the location info for a subscriber
func (b *rabbitMQEventBroker) UpdateLocation(id string, latitude, longitude float64, timestamp int64) {
	b.locationMutex.Lock()
	defer b.locationMutex.Unlock()

	b.locationCache[id] = LocationInfo{
		Latitude:  latitude,
		Longitude: longitude,
		Timestamp: timestamp,
	}
}

// GetLocation retrieves the location info for a subscriber
func (b *rabbitMQEventBroker) GetLocation(id string) (LocationInfo, bool) {
	b.locationMutex.RLock()
	defer b.locationMutex.RUnlock()

	location, exists := b.locationCache[id]
	return location, exists
}

// Close closes the RabbitMQ connection and channel
func (b *rabbitMQEventBroker) Close() error {
	if b.channel != nil {
		b.channel.Close()
	}
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}

// In-memory broker as fallback when RabbitMQ is not available
type inMemoryEventBroker struct {
	subscribers     map[string]*EventSubscriber
	subscriberMutex sync.RWMutex
}

func newInMemoryEventBroker() EventBroker {
	logrus.Warn("Using in-memory event broker (RabbitMQ not available)")
	return &inMemoryEventBroker{
		subscribers: make(map[string]*EventSubscriber),
	}
}

// Subscribe creates a new subscriber and returns it
func (b *inMemoryEventBroker) Subscribe(id string) *EventSubscriber {
	b.subscriberMutex.Lock()
	defer b.subscriberMutex.Unlock()

	// Check if subscriber already exists
	if subscriber, exists := b.subscribers[id]; exists {
		return subscriber
	}

	// Create a new subscriber
	subscriber := &EventSubscriber{
		ID:     id,
		Events: make(chan model.Event, 100), // Buffered channel to prevent blocking
	}

	b.subscribers[id] = subscriber
	return subscriber
}

// Unsubscribe removes a subscriber from the broker
func (b *inMemoryEventBroker) Unsubscribe(id string) {
	b.subscriberMutex.Lock()
	defer b.subscriberMutex.Unlock()

	if subscriber, exists := b.subscribers[id]; exists {
		close(subscriber.Events)
		delete(b.subscribers, id)
	}
}

// Publish sends an event to all subscribers
func (b *inMemoryEventBroker) Publish(event model.Event) {
	b.subscriberMutex.RLock()
	defer b.subscriberMutex.RUnlock()

	for _, subscriber := range b.subscribers {
		// Non-blocking send to prevent slow subscribers from blocking others
		select {
		case subscriber.Events <- event:
			// Event sent successfully
		default:
			// Channel is full, skip this subscriber for now
		}
	}
}

// UpdateLocation updates the location info for a subscriber
func (b *inMemoryEventBroker) UpdateLocation(id string, latitude, longitude float64, timestamp int64) {
	b.subscriberMutex.Lock()
	defer b.subscriberMutex.Unlock()

	if subscriber, exists := b.subscribers[id]; exists {
		subscriber.Location = LocationInfo{
			Latitude:  latitude,
			Longitude: longitude,
			Timestamp: timestamp,
		}
	}
}

// GetLocation retrieves the location info for a subscriber
func (b *inMemoryEventBroker) GetLocation(id string) (LocationInfo, bool) {
	b.subscriberMutex.RLock()
	defer b.subscriberMutex.RUnlock()

	if subscriber, exists := b.subscribers[id]; exists {
		return subscriber.Location, true
	}

	return LocationInfo{}, false
}

// Close is a no-op for the in-memory broker
func (b *inMemoryEventBroker) Close() error {
	return nil
}
