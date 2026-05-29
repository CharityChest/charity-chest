package v1

import (
	"charity-chest/services/admin/backend/internal/cache"
	"charity-chest/services/admin/backend/internal/config"
	"charity-chest/services/admin/backend/internal/handler"
	"charity-chest/services/admin/backend/internal/middleware"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RegisterProfile mounts the JWT-protected MFA profile routes under /api/profile.
func RegisterProfile(v1 *echo.Group, db *gorm.DB, cfg *config.Config, c *cache.Cache, jwtSecret string) {
	h := handler.NewProfileHandler(db, cfg, c)

	profile := v1.Group("/api/profile")
	profile.Use(middleware.JWT(db, jwtSecret))
	profile.GET("/mfa/setup", h.SetupMFA)
	profile.POST("/mfa/enable", h.EnableMFA)
	profile.DELETE("/mfa", h.DisableMFA)
}
