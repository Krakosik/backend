package procedure

import (
	"context"

	"github.com/krakosik/backend/gen"
	"github.com/krakosik/backend/internal/service"
	"google.golang.org/grpc"
)

type Event interface {
	StreamLocation(srv grpc.BidiStreamingServer[gen.LocationUpdate, gen.EventsResponse]) error
	ReportEvent(context.Context, *gen.ReportEventRequest) (*gen.ReportEventResponse, error)
	VoteEvent(context.Context, *gen.VoteEventRequest) (*gen.VoteEventResponse, error)
}

type eventProcedure struct {
	eventService service.EventService
}

func newEventProcedure(eventService service.EventService) Event {
	return &eventProcedure{
		eventService: eventService,
	}
}

func (e eventProcedure) StreamLocation(srv grpc.BidiStreamingServer[gen.LocationUpdate, gen.EventsResponse]) error {
	return e.eventService.StreamLocation(srv)
}

func (e eventProcedure) ReportEvent(ctx context.Context, request *gen.ReportEventRequest) (*gen.ReportEventResponse, error) {
	return e.eventService.ReportEvent(ctx, request)
}

func (e eventProcedure) VoteEvent(ctx context.Context, request *gen.VoteEventRequest) (*gen.VoteEventResponse, error) {
	return e.eventService.VoteEvent(ctx, request)
}
