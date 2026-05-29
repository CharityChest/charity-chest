package v1

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// RegisterHealth mounts the unversioned GET /health liveness probe.
// Returns the same envelope shape as the rest of the API for symmetry.
func RegisterHealth(e *echo.Echo) {
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"data": map[string]string{"status": "ok"}})
	})
}
