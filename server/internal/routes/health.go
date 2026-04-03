package routes

import (
	"github.com/labstack/echo/v4"
)

func RegisterHealth(e *echo.Echo) {
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})
}