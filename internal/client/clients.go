package client

import (
	"context"

	firebase "firebase.google.com/go/v4"
	"github.com/krakosik/backend/internal/dto"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
)

type Clients interface {
	AuthClient() AuthClient
	RabbitMQClient() RabbitClient
}

type clients struct {
	authClient   AuthClient
	rabbitClient RabbitClient
}

func (c clients) AuthClient() AuthClient {
	return c.authClient
}

func (c clients) RabbitMQClient() RabbitClient {
	return c.rabbitClient
}

func NewClients(cfg dto.Config) Clients {
	decodedFirebaseKey, err := cfg.DecodeFirebaseKey()
	if err != nil {
		logrus.Panic(err)
	}
	app, err := firebase.NewApp(context.Background(), nil, option.WithCredentialsJSON(decodedFirebaseKey))
	if err != nil {
		logrus.Panic(err)
	}

	authClient, err := app.Auth(context.Background())
	if err != nil {
		logrus.Panic(err)
	}

	rabbitClient, err := NewRabbitMQClient(cfg)
	if err != nil {
		logrus.Panic(err)
	}

	return &clients{
		authClient:   authClient,
		rabbitClient: rabbitClient,
	}
}
