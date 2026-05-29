package v1

import (
	"charity-chest/services/operational/backend/internal/handler"
	"charity-chest/services/operational/backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// RegisterAPI mounts the JWT-protected core API routes (currently GET /api/me).
func RegisterAPI(v1 *echo.Group, h *handler.MeHandler, jwtSecret string) {
	api := v1.Group("/api")
	api.Use(middleware.JWT(jwtSecret))
	api.GET("/me", h.Me)
}
