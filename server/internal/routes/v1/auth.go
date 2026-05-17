package v1

import (
	"charity-chest/internal/handler"

	"github.com/labstack/echo/v4"
)

// RegisterAuth mounts the public authentication routes under /auth.
func RegisterAuth(v1 *echo.Group, h *handler.AuthHandler) {
	auth := v1.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	auth.POST("/mfa/verify", h.VerifyMFA)
	auth.GET("/google", h.GoogleLogin)
	auth.GET("/google/callback", h.GoogleCallback)
	// Password recovery — both endpoints are public and enumeration-safe;
	// they return 204 regardless of whether the email maps to an account.
	auth.POST("/password/forgot", h.ForgotPassword)
	auth.POST("/password/reset", h.ResetPassword)
}
