package controller

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/krakosik/backend/internal/dto"
)

type InfoController interface {
	Info(c echo.Context) error
}
type infoController struct{}

func newInfoController() InfoController {
	return &infoController{}
}

func (infoController) Info(c echo.Context) error {
	return c.JSON(http.StatusOK, dto.ServerInfo{
		Healthy: true,
	})
}
