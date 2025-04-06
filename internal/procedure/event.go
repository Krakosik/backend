package procedure

import (
	"context"
	"github.com/krakosik/backend/gen"
	"google.golang.org/grpc"
)

type Event interface {
	StreamLocation(grpc.BidiStreamingServer[gen.LocationUpdate, gen.EventsResponse]) error
	ReportEvent(context.Context, *gen.ReportEventRequest) (*gen.ReportEventResponse, error)
	VoteEvent(context.Context, *gen.VoteEventRequest) (*gen.VoteEventResponse, error)
}

type eventProcedure struct {
}

func newEventProcedure() Event {
	return &eventProcedure{}
}

func (e eventProcedure) StreamLocation(grpc.BidiStreamingServer[gen.LocationUpdate, gen.EventsResponse]) error {
	//TODO implement me
	panic("implement me")
}

func (e eventProcedure) ReportEvent(ctx context.Context, request *gen.ReportEventRequest) (*gen.ReportEventResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (e eventProcedure) VoteEvent(ctx context.Context, request *gen.VoteEventRequest) (*gen.VoteEventResponse, error) {
	//TODO implement me
	panic("implement me")
}
