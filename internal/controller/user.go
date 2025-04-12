package controller

import (
	"github.com/krakosik/backend/internal/service"
)

type UserController interface {
}

type userController struct {
	userService service.UserService
}

func newUserController(userService service.UserService) UserController {
	return &userController{
		userService: userService,
	}
}
