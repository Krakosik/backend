package service

import (
	"github.com/krakosik/backend/internal/model"
	"github.com/krakosik/backend/internal/repository"
)

type EventService interface {
	CreateEvent(event model.Event) (model.Event, error)
	FindEventsWithinDistance(latitude, longitude float64, radius float64) ([]model.Event, error)
	PublishEvent(event model.Event)
	GetVoteCount(eventID uint) (int32, error)
}

type eventService struct {
	eventRepository repository.EventRepository
	eventBroker     EventBroker
}

func newEventService(eventRepository repository.EventRepository, eventBroker EventBroker) EventService {
	return &eventService{
		eventRepository: eventRepository,
		eventBroker:     eventBroker,
	}
}

func (e *eventService) CreateEvent(event model.Event) (model.Event, error) {
	createdEvent, err := e.eventRepository.Create(event)
	if err != nil {
		return model.Event{}, err
	}

	// Publish the event to all subscribers
	e.PublishEvent(createdEvent)

	return createdEvent, nil
}

func (e *eventService) FindEventsWithinDistance(latitude, longitude float64, radius float64) ([]model.Event, error) {
	return e.eventRepository.FindEventsWithinDistance(latitude, longitude, radius)
}

func (e *eventService) PublishEvent(event model.Event) {
	e.eventBroker.Publish(event)
}

func (e *eventService) GetVoteCount(eventID uint) (int32, error) {
	return e.eventRepository.CountVotes(eventID)
}
