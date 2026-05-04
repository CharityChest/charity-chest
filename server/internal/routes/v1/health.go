package v1

import (
	"github.com/labstack/echo/v4"
)

// RegisterHealth mounts the unversioned GET /health liveness probe.
func RegisterHealth(e *echo.Echo) {
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]any{"data": map[string]string{"status": "ok"}})
	})
}
