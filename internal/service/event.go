package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/krakosik/backend/gen"
	"github.com/krakosik/backend/internal/client"
	"github.com/krakosik/backend/internal/model"
	"github.com/krakosik/backend/internal/repository"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type EventService interface {
	FindEventsWithinDistance(latitude, longitude float64, radius float64) ([]model.Event, error)
	PublishEvent(event model.Event)
	GetVoteCount(eventID uint) (int32, error)
	StreamLocation(srv grpc.BidiStreamingServer[gen.LocationUpdate, gen.EventsResponse]) error
	ReportEvent(ctx context.Context, request *gen.ReportEventRequest) (*gen.ReportEventResponse, error)
	VoteEvent(ctx context.Context, request *gen.VoteEventRequest) (*gen.VoteEventResponse, error)
}

type eventService struct {
	eventRepository repository.EventRepository
	rabbitClient    client.RabbitClient
	locationService LocationService
}

func newEventService(eventRepository repository.EventRepository, rabbitClient client.RabbitClient) EventService {
	return &eventService{
		eventRepository: eventRepository,
		rabbitClient:    rabbitClient,
		locationService: newLocationService(),
	}
}

func (e *eventService) FindEventsWithinDistance(latitude, longitude float64, radius float64) ([]model.Event, error) {
	return e.eventRepository.FindEventsWithinDistance(latitude, longitude, radius)
}

func (e *eventService) PublishEvent(event model.Event) {
	eventJson, err := json.Marshal(event)
	if err != nil {
		logrus.Errorf("Error marshaling event: %v", err)
		return
	}

	err = e.rabbitClient.PublishMessage(eventJson)
	if err != nil {
		logrus.Errorf("Error publishing event: %v", err)
	}
}

func (e *eventService) GetVoteCount(eventID uint) (int32, error) {
	return e.eventRepository.CountVotes(eventID)
}

func (e *eventService) StreamLocation(srv grpc.BidiStreamingServer[gen.LocationUpdate, gen.EventsResponse]) error {
	connectionID := fmt.Sprintf("conn_%d", time.Now().UTC().UnixNano())

	msgChan, err := e.rabbitClient.SubscribeToMessages(connectionID)
	if err != nil {
		return fmt.Errorf("failed to subscribe to messages: %v", err)
	}
	defer e.rabbitClient.UnsubscribeFromMessages(connectionID)
	defer e.locationService.RemoveLocation(connectionID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			locationUpdate, err := srv.Recv()
			if err != nil {
				logrus.Errorf("Error receiving location update: %v", err)
				cancel()
				return
			}

			e.locationService.UpdateLocation(
				connectionID,
				locationUpdate.GetLatitude(),
				locationUpdate.GetLongitude(),
				locationUpdate.GetTimestamp(),
			)

			events, err := e.FindEventsWithinDistance(
				locationUpdate.GetLatitude(),
				locationUpdate.GetLongitude(),
				1000.0,
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

			protoEvents := make([]*gen.Event, 0, len(events))
			for _, event := range events {
				protoEvent := &gen.Event{
					EventId:   uint32(event.ID),
					Type:      gen.EventType(event.EventType),
					Latitude:  event.Latitude,
					Longitude: event.Longitude,
					CreatedAt: event.CreatedAt.Unix(),
				}
				if event.ExpiredAt != nil {
					unixTime := event.ExpiredAt.Unix()
					protoEvent.ExpiresAt = &unixTime
				}

				voteCount, err := e.GetVoteCount(event.ID)
				if err == nil {
					protoEvent.Votes = voteCount
				}

				protoEvents = append(protoEvents, protoEvent)
			}

			if err := srv.Send(&gen.EventsResponse{
				Events: protoEvents,
			}); err != nil {
				logrus.Errorf("Error sending events response: %v", err)
				cancel()
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			select {
			case msg, ok := <-msgChan:
				if !ok {
					return
				}

				var event model.Event
				if err := json.Unmarshal(msg, &event); err != nil {
					logrus.Errorf("Error unmarshaling event: %v", err)
					continue
				}

				location, exists := e.locationService.GetLocation(connectionID)
				if !exists {
					continue
				}

				events, err := e.FindEventsWithinDistance(
					location.Latitude,
					location.Longitude,
					1000.0,
				)
				if err != nil {
					logrus.Errorf("Error finding events after notification: %v", err)
					continue
				}

				protoEvents := make([]*gen.Event, 0, len(events))
				for _, event := range events {
					protoEvent := &gen.Event{
						EventId:   uint32(event.ID),
						Type:      gen.EventType(event.EventType),
						Latitude:  event.Latitude,
						Longitude: event.Longitude,
						CreatedAt: event.CreatedAt.Unix(),
					}
					if event.ExpiredAt != nil {
						unixTime := event.ExpiredAt.Unix()
						protoEvent.ExpiresAt = &unixTime
					}

					voteCount, err := e.GetVoteCount(event.ID)
					if err == nil {
						protoEvent.Votes = voteCount
					}

					protoEvents = append(protoEvents, protoEvent)
				}

				if err := srv.Send(&gen.EventsResponse{
					Events: protoEvents,
				}); err != nil {
					logrus.Errorf("Error sending updated events response: %v", err)
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
	return nil
}

func (e *eventService) ReportEvent(ctx context.Context, request *gen.ReportEventRequest) (*gen.ReportEventResponse, error) {
	expiration := time.Now().UTC().Add(model.EventType(request.GetType()).GetExpirationDuration())
	event := model.Event{
		EventType: model.EventType(request.GetType()),
		Latitude:  request.GetLatitude(),
		Longitude: request.GetLongitude(),
		CreatedAt: time.Now().UTC(),
		CreatedBy: 1,
		ExpiredAt: &expiration,
	}

	createdEvent, err := e.eventRepository.Create(event)
	if err != nil {
		return nil, err
	}

	e.PublishEvent(createdEvent)

	return &gen.ReportEventResponse{
		EventId: uint32(createdEvent.ID),
	}, nil
}

func (e *eventService) VoteEvent(ctx context.Context, request *gen.VoteEventRequest) (*gen.VoteEventResponse, error) {
	voteCount, err := e.GetVoteCount(uint(request.GetEventId()))
	if err != nil {
		return nil, err
	}

	return &gen.VoteEventResponse{
		Votes: voteCount,
	}, nil
}
