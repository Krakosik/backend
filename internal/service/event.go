package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/krakosik/backend/gen"
	"github.com/krakosik/backend/internal/client"
	"github.com/krakosik/backend/internal/dto"
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
	voteRepository  repository.VoteRepository
	rabbitClient    client.RabbitClient
	locationService LocationService
}

func newEventService(eventRepository repository.EventRepository, voteRepository repository.VoteRepository, rabbitClient client.RabbitClient) EventService {
	return &eventService{
		eventRepository: eventRepository,
		voteRepository:  voteRepository,
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

	user, ok := dto.GetUserFromContext(srv.Context())
	if !ok {
		return fmt.Errorf("user not found in context")
	}

	userIdentifier := user.Email
	if user.FirstName != nil && *user.FirstName != "" {
		userIdentifier = *user.FirstName
		if user.LastName != nil && *user.LastName != "" {
			userIdentifier = fmt.Sprintf("%s %s", *user.FirstName, *user.LastName)
		}
	}

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
				logrus.Errorf("Error receiving location update from user %s: %v", userIdentifier, err)
				cancel()
				return
			}

			e.locationService.UpdateLocation(
				connectionID,
				locationUpdate.GetLatitude(),
				locationUpdate.GetLongitude(),
				locationUpdate.GetTimestamp(),
			)

			logrus.Infof("User %s updated location to (%f, %f)", userIdentifier, locationUpdate.GetLatitude(), locationUpdate.GetLongitude())

			events, err := e.FindEventsWithinDistance(
				locationUpdate.GetLatitude(),
				locationUpdate.GetLongitude(),
				1000.0,
			)
			if err != nil {
				logrus.Errorf("Error finding events near location for user %s: %v", userIdentifier, err)
				continue
			}

			logrus.Infof("Found %d events near location (%f, %f) for user %s",
				len(events),
				locationUpdate.GetLatitude(),
				locationUpdate.GetLongitude(),
				userIdentifier,
			)

			protoEvents := make([]*gen.Event, 0, len(events))
			for _, event := range events {
				protoEvent := &gen.Event{
					EventId:   uint32(event.ID),
					Type:      gen.EventType(event.EventType),
					Latitude:  event.Latitude,
					Longitude: event.Longitude,
					CreatedAt: event.CreatedAt.Unix(),
					CanVote:   event.CreatedBy != user.ID,
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
					logrus.Errorf("Error unmarshaling event for user %s: %v", userIdentifier, err)
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
					logrus.Errorf("Error finding events after notification for user %s: %v", userIdentifier, err)
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
						CanVote:   event.CreatedBy != user.ID,
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
	user, ok := dto.GetUserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("user not found in context")
	}

	userIdentifier := user.Email
	if user.FirstName != nil && *user.FirstName != "" {
		userIdentifier = *user.FirstName
		if user.LastName != nil && *user.LastName != "" {
			userIdentifier = fmt.Sprintf("%s %s", *user.FirstName, *user.LastName)
		}
	}

	existingEvents, err := e.eventRepository.FindEventsWithinDistance(
		request.GetLatitude(),
		request.GetLongitude(),
		100.0,
	)
	if err != nil {
		return nil, err
	}

	for _, existingEvent := range existingEvents {
		if existingEvent.EventType == model.EventType(request.GetType()) {
			newExpiration := time.Now().UTC().Add(model.EventType(request.GetType()).GetExpirationDuration())
			existingEvent.ExpiredAt = &newExpiration

			updatedEvent, err := e.eventRepository.Update(existingEvent)
			if err != nil {
				return nil, err
			}

			logrus.Infof("User %s updated event %d", userIdentifier, updatedEvent.ID)
			e.PublishEvent(updatedEvent)

			return &gen.ReportEventResponse{
				EventId: uint32(updatedEvent.ID),
			}, nil
		}
	}

	expiration := time.Now().UTC().Add(model.EventType(request.GetType()).GetExpirationDuration())
	event := model.Event{
		EventType: model.EventType(request.GetType()),
		Latitude:  request.GetLatitude(),
		Longitude: request.GetLongitude(),
		CreatedAt: time.Now().UTC(),
		CreatedBy: user.ID,
		ExpiredAt: &expiration,
	}

	createdEvent, err := e.eventRepository.Create(event)
	if err != nil {
		return nil, err
	}

	logrus.Infof("User %s created new event %d", userIdentifier, createdEvent.ID)
	e.PublishEvent(createdEvent)

	return &gen.ReportEventResponse{
		EventId: uint32(createdEvent.ID),
	}, nil
}

func (e *eventService) VoteEvent(ctx context.Context, request *gen.VoteEventRequest) (*gen.VoteEventResponse, error) {
	user, ok := dto.GetUserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("user not found in context")
	}

	userIdentifier := user.Email
	if user.FirstName != nil && *user.FirstName != "" {
		userIdentifier = *user.FirstName
		if user.LastName != nil && *user.LastName != "" {
			userIdentifier = fmt.Sprintf("%s %s", *user.FirstName, *user.LastName)
		}
	}

	vote := model.Vote{
		EventID: uint(request.GetEventId()),
		UserID:  user.ID,
		Upvote:  request.GetUpvote(),
	}

	_, err := e.voteRepository.Create(vote)
	if err != nil {
		return nil, err
	}

	logrus.Infof("User %s voted on event %d: upvote=%v", userIdentifier, request.GetEventId(), request.GetUpvote())

	voteCount, err := e.voteRepository.CountVotes(uint(request.GetEventId()))
	if err != nil {
		return nil, err
	}

	event, err := e.eventRepository.GetByID(uint(request.GetEventId()))
	if err != nil {
		return nil, err
	}

	e.PublishEvent(event)

	return &gen.VoteEventResponse{
		EventId: request.GetEventId(),
		Votes:   voteCount,
	}, nil
}
