package v1

import (
	"charity-chest/services/admin/backend/internal/handler"
	"charity-chest/services/admin/backend/internal/middleware"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RegisterAPI mounts the JWT-protected core API routes (currently GET /api/me).
func RegisterAPI(v1 *echo.Group, h *handler.AuthHandler, db *gorm.DB, jwtSecret string) {
	api := v1.Group("/api")
	api.Use(middleware.JWT(db, jwtSecret))
	api.GET("/me", h.Me)
}
