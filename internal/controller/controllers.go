package controller

import (
	"github.com/labstack/echo/v4"
	"github.com/krakosik/backend/internal/service"
)

type Controllers interface {
	User() UserController
	Info() InfoController

	Route(e *echo.Echo)
}

type controllers struct {
	userController UserController
	infoController InfoController
}

func NewControllers(services service.Services) Controllers {
	userController := newUserController(services.User())
	infoController := newInfoController()
	return &controllers{
		userController: userController,
		infoController: infoController,
	}

}

func (c controllers) User() UserController {
	return c.userController
}

func (c controllers) Info() InfoController {
	return c.infoController
}

func (c controllers) Route(e *echo.Echo) {
	e.GET("/", c.infoController.Info)

	e.POST("/api/auth/login", c.userController.Login)
	e.POST("/api/auth/register", c.userController.Register)
}
