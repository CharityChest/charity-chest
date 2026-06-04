package v1

import (
	"charity-chest/services/admin/backend/internal/handler"
	"charity-chest/services/admin/backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// RegisterInternal mounts the /v1/internal/* service-to-service routes.
// The whole group is gated by the ServiceKey middleware; callers must send
// the matching X-Service-Key header on every request.
//
// main.go only calls this when SERVICE_API_KEY is set, so the group simply
// does not exist on instances that have no service-to-service pair deployed.
func RegisterInternal(v1 *echo.Group, h *handler.InternalHandler, serviceKey string) {
	g := v1.Group("/internal", middleware.ServiceKey(serviceKey))
	g.POST("/auth/login", h.InternalLogin)
	g.POST("/auth/google", h.InternalGoogleAuth)
	g.GET("/users/:userUUID", h.InternalGetUser)
}
