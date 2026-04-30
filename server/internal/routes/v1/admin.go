package v1

import (
	"charity-chest/internal/cache"
	"charity-chest/internal/handler"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RegisterAdmin registers root-only administration routes.
//
// Protected routes (root JWT required):
//
//	GET /v1/api/admin/users  — search users with pagination
func RegisterAdmin(v1 *echo.Group, db *gorm.DB, c *cache.Cache, jwtSecret string) {
	h := handler.NewAdminHandler(db, c)

	admin := v1.Group("/api/admin")
	admin.Use(middleware.JWT(jwtSecret))
	admin.GET("/users", h.SearchUsers, middleware.RequireSystemRole(model.RoleRoot))
}
