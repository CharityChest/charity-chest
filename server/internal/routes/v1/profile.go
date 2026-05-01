package v1

import (
	"charity-chest/internal/cache"
	"charity-chest/internal/config"
	"charity-chest/internal/handler"
	"charity-chest/internal/middleware"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RegisterProfile mounts the JWT-protected MFA profile routes under /api/profile.
func RegisterProfile(v1 *echo.Group, db *gorm.DB, cfg *config.Config, c *cache.Cache, jwtSecret string) {
	h := handler.NewProfileHandler(db, cfg, c)

	profile := v1.Group("/api/profile")
	profile.Use(middleware.JWT(jwtSecret))
	profile.GET("/mfa/setup", h.SetupMFA)
	profile.POST("/mfa/enable", h.EnableMFA)
	profile.DELETE("/mfa", h.DisableMFA)
}
