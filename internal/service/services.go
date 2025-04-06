package service

import (
	authV4 "firebase.google.com/go/v4/auth"
	"github.com/krakosik/backend/internal/client"
	"github.com/krakosik/backend/internal/dto"
	"github.com/krakosik/backend/internal/repository"
)

type Services interface {
	User() UserService
	Auth() AuthService
}

type services struct {
	userService UserService
	authService AuthService
}

func NewServices(repositories repository.Repositories, config dto.Config, clients client.Clients) Services {
	userService := newUserService(repositories.User(), config)
	return &services{
		userService: userService,
		authService: newAuthService(repositories.User(), clients.AuthClient(), authV4.IsIDTokenExpired),
	}
}

func (s services) User() UserService {
	return s.userService
}

func (s services) Auth() AuthService {
	return s.authService
}
