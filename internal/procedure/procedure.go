package procedure

import (
	"crypto/tls"
	"github.com/krakosik/backend/gen"
	"github.com/krakosik/backend/internal/service"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net"
)

type Procedures interface {
	Serve(listener net.Listener) error
}

type procedures struct {
	eventProcedure Event
	grpcServer     *grpc.Server
}

func NewProcedures(services service.Services) Procedures {
	eventProcedure := newEventProcedure()

	grpcCredentials, err := tls.LoadX509KeyPair("server.crt", "server.key")
	if err != nil {
		logrus.Panic(err)
	}
	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewServerTLSFromCert(&grpcCredentials)))
	s := &server{}
	gen.RegisterEventServiceServer(grpcServer, s)
	return &procedures{
		eventProcedure: eventProcedure,
		grpcServer:     grpcServer,
	}
}

func (p *procedures) Serve(listener net.Listener) error {
	return p.grpcServer.Serve(listener)
}
