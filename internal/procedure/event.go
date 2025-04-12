package procedure

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/krakosik/backend/gen"
	"github.com/krakosik/backend/internal/model"
	"github.com/krakosik/backend/internal/service"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type Event interface {
	StreamLocation(srv grpc.BidiStreamingServer[gen.LocationUpdate, gen.EventsResponse]) error
	ReportEvent(context.Context, *gen.ReportEventRequest) (*gen.ReportEventResponse, error)
	VoteEvent(context.Context, *gen.VoteEventRequest) (*gen.VoteEventResponse, error)
}

type eventProcedure struct {
	eventService service.EventService
	eventBroker  service.EventBroker
}

func newEventProcedure(eventService service.EventService, eventBroker service.EventBroker) Event {
	return &eventProcedure{
		eventService: eventService,
		eventBroker:  eventBroker,
	}
}

func (e eventProcedure) StreamLocation(srv grpc.BidiStreamingServer[gen.LocationUpdate, gen.EventsResponse]) error {
	// Generate a unique connection ID for this stream
	connectionID := fmt.Sprintf("conn_%d", time.Now().UnixNano())

	// Subscribe to the event broker to get notified about new events
	subscriber := e.eventBroker.Subscribe(connectionID)
	defer e.eventBroker.Unsubscribe(connectionID)

	// Create a context with a cancellation mechanism
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a WaitGroup to wait for the goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	// Handle location updates coming from the client
	go func() {
		defer wg.Done()
		for {
			// Receive location update from client
			locationUpdate, err := srv.Recv()
			if err != nil {
				logrus.Errorf("Error receiving location update: %v", err)
				cancel() // Cancel context to stop the other goroutine
				return
			}

			// Store the latest location for this client
			e.eventBroker.UpdateLocation(
				connectionID,
				locationUpdate.GetLatitude(),
				locationUpdate.GetLongitude(),
				locationUpdate.GetTimestamp(),
			)

			// Find all events within 1000 meters of the user's location
			events, err := e.eventService.FindEventsWithinDistance(
				locationUpdate.GetLatitude(),
				locationUpdate.GetLongitude(),
				1000.0, // 1000 meters = 1 km
			)
			if err != nil {
				logrus.Errorf("Error finding events near location: %v", err)
				continue
			}

			logrus.Infof("Found %d events near location (%f, %f)",
				len(events),
				locationUpdate.GetLatitude(),
				locationUpdate.GetLongitude(),
			)

			// Convert model events to proto events
			protoEvents := make([]*gen.Event, 0, len(events))
			for _, event := range events {
				protoEvent := &gen.Event{
					EventId:   uint32(event.ID),
					Type:      gen.EventType(event.EventType),
					Latitude:  event.Latitude,
					Longitude: event.Longitude,
				}

				// Try to get vote count for this event
				voteCount, err := e.eventService.GetVoteCount(event.ID)
				if err == nil {
					protoEvent.Votes = voteCount
				}

				protoEvents = append(protoEvents, protoEvent)
			}

			// Send all nearby events to the client
			if err := srv.Send(&gen.EventsResponse{
				Events: protoEvents,
			}); err != nil {
				logrus.Errorf("Error sending events response: %v", err)
				cancel()
				return
			}
		}
	}()

	// Handle new events coming from the event broker
	go func() {
		defer wg.Done()
		for {
			select {
			case _, ok := <-subscriber.Events:
				if !ok {
					// Channel closed
					return
				}

				// When a new event is published, get the client's last known location
				location, exists := e.eventBroker.GetLocation(connectionID)
				if !exists {
					// No location for this client yet
					continue
				}

				// Fetch all events again - this ensures the client always has the complete set
				events, err := e.eventService.FindEventsWithinDistance(
					location.Latitude,
					location.Longitude,
					1000.0, // 1000 meters = 1 km
				)
				if err != nil {
					logrus.Errorf("Error finding events after notification: %v", err)
					continue
				}

				// Convert model events to proto events
				protoEvents := make([]*gen.Event, 0, len(events))
				for _, event := range events {
					protoEvent := &gen.Event{
						EventId:   uint32(event.ID),
						Type:      gen.EventType(event.EventType),
						Latitude:  event.Latitude,
						Longitude: event.Longitude,
					}

					// Try to get vote count for this event
					voteCount, err := e.eventService.GetVoteCount(event.ID)
					if err == nil {
						protoEvent.Votes = voteCount
					}

					protoEvents = append(protoEvents, protoEvent)
				}

				// Send updated events list to the client
				if err := srv.Send(&gen.EventsResponse{
					Events: protoEvents,
				}); err != nil {
					logrus.Errorf("Error sending updated events response: %v", err)
					return
				}

			case <-ctx.Done():
				// Context cancelled
				return
			}
		}
	}()

	// Wait for both goroutines to finish
	wg.Wait()
	return nil
}

func (e eventProcedure) ReportEvent(ctx context.Context, request *gen.ReportEventRequest) (*gen.ReportEventResponse, error) {
	// Create a new event from the request
	event := model.Event{
		EventType: model.EventType(request.GetType()),
		Latitude:  request.GetLatitude(),
		Longitude: request.GetLongitude(),
		CreatedAt: time.Now(),
		// CreatedBy would typically come from authenticated user
		CreatedBy: 1, // Default user ID for now
	}

	// Save the event to the database
	createdEvent, err := e.eventService.CreateEvent(event)
	if err != nil {
		logrus.Errorf("Error creating event: %v", err)
		return nil, err
	}

	// Return the response with the event ID
	return &gen.ReportEventResponse{
		EventId: uint32(createdEvent.ID),
	}, nil
}

func (e eventProcedure) VoteEvent(ctx context.Context, request *gen.VoteEventRequest) (*gen.VoteEventResponse, error) {
	// TODO: Implement vote event functionality
	// This is not part of the current implementation requirements
	return &gen.VoteEventResponse{
		EventId: request.GetEventId(),
		Votes:   0,
	}, nil
}

// calculateDistance calculates the distance between two points using the Haversine formula
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371000.0 // Earth radius in meters

	// Convert to radians
	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	// Difference in coordinates
	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad

	// Haversine formula
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}
