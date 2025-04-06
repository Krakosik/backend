package service

type EventService interface {
}
type eventService struct {
}

func newEventService() EventService {
	return &eventService{}
}
