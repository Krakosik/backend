package client

import (
	"context"
	"sync"
	"time"

	"github.com/krakosik/backend/internal/dto"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

type RabbitClient interface {
	PublishMessage(message []byte) error
	SubscribeToMessages(id string) (<-chan []byte, error)
	UnsubscribeFromMessages(id string) error
	Close() error
}

type rabbitClient struct {
	conn            *amqp.Connection
	channel         *amqp.Channel
	exchangeName    string
	subscribers     map[string]chan []byte
	subscriberMutex sync.RWMutex
}

func NewRabbitMQClient(config dto.Config) (RabbitClient, error) {
	connectionStr := config.RabbitMQURL
	if connectionStr == "" {
		connectionStr = "amqp://guest:guest@rabbitmq:5672/"
	}

	conn, err := amqp.Dial(connectionStr)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

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
		ch.Close()
		conn.Close()
		return nil, err
	}

	client := &rabbitClient{
		conn:         conn,
		channel:      ch,
		exchangeName: exchangeName,
		subscribers:  make(map[string]chan []byte),
	}

	go client.monitorConnection(connectionStr)

	return client, nil
}

func (c *rabbitClient) monitorConnection(connectionStr string) {
	connCloseChan := make(chan *amqp.Error)
	c.conn.NotifyClose(connCloseChan)

	err := <-connCloseChan
	logrus.Errorf("RabbitMQ connection closed: %v", err)

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

		err = ch.ExchangeDeclare(
			c.exchangeName, // name
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

		c.subscriberMutex.Lock()
		oldConn := c.conn
		oldChannel := c.channel
		c.conn = conn
		c.channel = ch
		c.subscriberMutex.Unlock()

		if oldChannel != nil {
			oldChannel.Close()
		}
		if oldConn != nil {
			oldConn.Close()
		}

		c.resubscribeAll()

		go c.monitorConnection(connectionStr)
		break
	}
}

func (c *rabbitClient) resubscribeAll() {
	c.subscriberMutex.RLock()
	defer c.subscriberMutex.RUnlock()

	for id, msgChan := range c.subscribers {
		q, err := c.channel.QueueDeclare(
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

		err = c.channel.QueueBind(
			q.Name,         // queue name
			"",             // routing key
			c.exchangeName, // exchange
			false,          // no-wait
			nil,            // arguments
		)
		if err != nil {
			logrus.Errorf("Failed to bind queue for subscriber %s: %v", id, err)
			continue
		}

		msgs, err := c.channel.Consume(
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

		go func(subscriberID string, subscriberChan chan []byte, deliveries <-chan amqp.Delivery) {
			for d := range deliveries {
				select {
				case subscriberChan <- d.Body:
				default:
				}
			}
		}(id, msgChan, msgs)
	}
}

func (c *rabbitClient) PublishMessage(message []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return c.channel.PublishWithContext(
		ctx,
		c.exchangeName, // exchange
		"",             // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        message,
		})
}

func (c *rabbitClient) SubscribeToMessages(id string) (<-chan []byte, error) {
	c.subscriberMutex.Lock()
	defer c.subscriberMutex.Unlock()

	if msgChan, exists := c.subscribers[id]; exists {
		return msgChan, nil
	}

	msgChan := make(chan []byte, 100) // Buffered channel to prevent blocking

	q, err := c.channel.QueueDeclare(
		"",    // name - let RabbitMQ generate a unique name
		false, // durable
		true,  // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return nil, err
	}

	err = c.channel.QueueBind(
		q.Name,         // queue name
		"",             // routing key
		c.exchangeName, // exchange
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		return nil, err
	}

	msgs, err := c.channel.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		true,   // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return nil, err
	}

	c.subscribers[id] = msgChan

	go func() {
		for d := range msgs {
			c.subscriberMutex.RLock()
			existingChan, exists := c.subscribers[id]
			stillActive := exists && existingChan == msgChan
			c.subscriberMutex.RUnlock()

			if !stillActive {
				return
			}

			func() {
				defer func() {
					if r := recover(); r != nil {
						logrus.Errorf("Recovered from panic in message delivery: %v", r)
					}
				}()

				c.subscriberMutex.RLock()
				stillExists := c.subscribers[id] == msgChan
				c.subscriberMutex.RUnlock()

				if !stillExists {
					return
				}

				select {
				case msgChan <- d.Body:
				default:
				}
			}()
		}
	}()

	return msgChan, nil
}

func (c *rabbitClient) UnsubscribeFromMessages(id string) error {
	c.subscriberMutex.Lock()
	defer c.subscriberMutex.Unlock()

	if msgChan, exists := c.subscribers[id]; exists {
		delete(c.subscribers, id)
		close(msgChan)
	}

	return nil
}

func (c *rabbitClient) Close() error {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
