package v1

import (
	"charity-chest/internal/handler"

	"github.com/labstack/echo/v4"
)

func RegisterAuth(v1 *echo.Group, h *handler.AuthHandler) {
	auth := v1.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	auth.GET("/google", h.GoogleLogin)
	auth.GET("/google/callback", h.GoogleCallback)
}