package v1

import (
	"charity-chest/services/admin/backend/internal/cache"
	"charity-chest/services/admin/backend/internal/handler"
	"charity-chest/services/admin/backend/internal/middleware"
	"charity-chest/services/admin/backend/internal/model"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RegisterSystem registers system administration routes.
//
// Public routes (no JWT):
//
//	GET /v1/system/status
//
// Protected routes (root JWT required):
//
//	POST /v1/api/system/assign-role
func RegisterSystem(v1 *echo.Group, db *gorm.DB, c *cache.Cache, jwtSecret string) {
	h := handler.NewSystemHandler(db, c)

	// Public — sits outside /v1/api/ to signal it requires no authentication.
	v1.GET("/system/status", h.SystemStatus)

	// Protected — root only.
	sys := v1.Group("/api/system")
	sys.Use(middleware.JWT(db, jwtSecret))
	sys.POST("/assign-role", h.AssignSystemRole, middleware.RequireSystemRole(model.RoleRoot))
}
