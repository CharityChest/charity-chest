package v1

import (
	"charity-chest/internal/cache"
	"charity-chest/internal/handler"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RegisterOrgs registers organization CRUD and member management routes.
// All routes require a valid JWT. Role requirements are enforced per-route.
//
//	GET    /v1/api/orgs                            — system, root
//	POST   /v1/api/orgs                            — system, root
//	GET    /v1/api/orgs/:orgUUID                   — any org member OR system/root
//	PUT    /v1/api/orgs/:orgUUID                   — system, root
//	DELETE /v1/api/orgs/:orgUUID                   — system, root
//	GET    /v1/api/orgs/:orgUUID/members           — any org member OR system/root
//	POST   /v1/api/orgs/:orgUUID/members           — hierarchy enforced in handler
//	PUT    /v1/api/orgs/:orgUUID/members/:userUUID — hierarchy enforced in handler
//	DELETE /v1/api/orgs/:orgUUID/members/:userUUID — hierarchy enforced in handler
func RegisterOrgs(v1 *echo.Group, db *gorm.DB, c *cache.Cache, jwtSecret string) {
	h := handler.NewOrgHandler(db, c)

	orgs := v1.Group("/api/orgs")
	orgs.Use(middleware.JWT(jwtSecret))

	systemOrRoot := middleware.RequireSystemRole(model.RoleSystem, model.RoleRoot)
	orgs.GET("", h.ListOrgs, systemOrRoot)
	orgs.POST("", h.CreateOrg, systemOrRoot)
	orgs.PUT("/:orgUUID", h.UpdateOrg, systemOrRoot)
	orgs.DELETE("/:orgUUID", h.DeleteOrg, systemOrRoot)

	// Any org member passes; root/system bypass the membership check automatically.
	// The middleware also resolves :orgUUID to an int id stored under
	// middleware.OrgIDContextKey, so handlers downstream avoid a second lookup.
	anyMember := middleware.RequireOrgRole(db,
		model.OrgRoleOwner, model.OrgRoleAdmin, model.OrgRoleOperational)
	orgs.GET("/:orgUUID", h.GetOrg, anyMember)
	orgs.GET("/:orgUUID/members", h.ListMembers, anyMember)
	orgs.POST("/:orgUUID/members", h.AddMember, anyMember)
	orgs.PUT("/:orgUUID/members/:userUUID", h.UpdateMember, anyMember)
	orgs.DELETE("/:orgUUID/members/:userUUID", h.RemoveMember, anyMember)
}
