package v1

import (
	"charity-chest/services/operational/backend/internal/handler"

	"github.com/labstack/echo/v4"
)

// RegisterAuth mounts the public authentication routes under /auth.
// The mobile app's login flow hits exactly these two endpoints.
func RegisterAuth(v1 *echo.Group, h *handler.AuthHandler) {
	auth := v1.Group("/auth")
	auth.POST("/login", h.Login)
	auth.POST("/google", h.GoogleLogin)
}
