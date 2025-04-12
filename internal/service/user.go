package service

import (
	"github.com/krakosik/backend/internal/dto"
	"github.com/krakosik/backend/internal/repository"
)

type UserService interface {
}

type userService struct {
	userRepository repository.UserRepository
	config         dto.Config
}

func newUserService(userRepository repository.UserRepository, config dto.Config) UserService {
	return &userService{userRepository, config}
}
