package v1

import (
	"charity-chest/internal/handler"
	"charity-chest/internal/middleware"

	"github.com/labstack/echo/v4"
)

// RegisterAPI mounts the JWT-protected core API routes (currently GET /api/me).
func RegisterAPI(v1 *echo.Group, h *handler.AuthHandler, jwtSecret string) {
	api := v1.Group("/api")
	api.Use(middleware.JWT(jwtSecret))
	api.GET("/me", h.Me)
}