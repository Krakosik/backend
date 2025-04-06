package client

import "github.com/krakosik/backend/internal/dto"

type Clients interface{}

type clients struct{}

func NewClients(_ dto.Config) Clients {
	return &clients{}
}
