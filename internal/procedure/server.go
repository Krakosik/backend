package procedure

import (
	"context"
	"github.com/krakosik/backend/gen"
	"google.golang.org/grpc"
)

type server struct {
	gen.UnimplementedEventServiceServer

	eventProcedure Event
}

func (s *server) StreamLocation(biStreamingServer grpc.BidiStreamingServer[gen.LocationUpdate, gen.EventsResponse]) error {
	return s.StreamLocation(biStreamingServer)
}

func (s *server) ReportEvent(ctx context.Context, request *gen.ReportEventRequest) (*gen.ReportEventResponse, error) {
	return s.eventProcedure.ReportEvent(ctx, request)
}

func (s *server) VoteEvent(ctx context.Context, request *gen.VoteEventRequest) (*gen.VoteEventResponse, error) {
	return s.eventProcedure.VoteEvent(ctx, request)
}
