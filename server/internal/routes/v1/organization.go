package v1

import (
	"charity-chest/internal/handler"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RegisterOrgs registers organization CRUD and member management routes.
// All routes require a valid JWT. Role requirements are enforced per-route.
//
//	GET    /v1/api/orgs                         — system, root
//	POST   /v1/api/orgs                         — system, root
//	GET    /v1/api/orgs/:orgID                  — any org member OR system/root
//	PUT    /v1/api/orgs/:orgID                  — system, root
//	DELETE /v1/api/orgs/:orgID                  — system, root
//	GET    /v1/api/orgs/:orgID/members          — any org member OR system/root
//	POST   /v1/api/orgs/:orgID/members          — hierarchy enforced in handler
//	PUT    /v1/api/orgs/:orgID/members/:userID  — hierarchy enforced in handler
//	DELETE /v1/api/orgs/:orgID/members/:userID  — hierarchy enforced in handler
func RegisterOrgs(v1 *echo.Group, db *gorm.DB, jwtSecret string) {
	h := handler.NewOrgHandler(db)

	orgs := v1.Group("/api/orgs")
	orgs.Use(middleware.JWT(jwtSecret))

	systemOrRoot := middleware.RequireSystemRole(model.RoleSystem, model.RoleRoot)
	orgs.GET("", h.ListOrgs, systemOrRoot)
	orgs.POST("", h.CreateOrg, systemOrRoot)
	orgs.PUT("/:orgID", h.UpdateOrg, systemOrRoot)
	orgs.DELETE("/:orgID", h.DeleteOrg, systemOrRoot)

	// Any org member passes; root/system bypass the membership check automatically.
	anyMember := middleware.RequireOrgRole(db,
		model.OrgRoleOwner, model.OrgRoleAdmin, model.OrgRoleOperational)
	orgs.GET("/:orgID", h.GetOrg, anyMember)
	orgs.GET("/:orgID/members", h.ListMembers, anyMember)
	orgs.POST("/:orgID/members", h.AddMember, anyMember)
	orgs.PUT("/:orgID/members/:userID", h.UpdateMember, anyMember)
	orgs.DELETE("/:orgID/members/:userID", h.RemoveMember, anyMember)
}
