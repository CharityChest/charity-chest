package v1

import (
	"charity-chest/internal/cache"
	"charity-chest/internal/config"
	"charity-chest/internal/handler"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RegisterBilling registers billing and plan management routes.
//
// Unversioned (no auth — Stripe signature is the authentication mechanism):
//
//	POST /stripe/webhook
//
// Protected under /v1/api/orgs/:orgID/billing (org owner or root/system):
//
//	POST   /v1/api/orgs/:orgID/billing/checkout
//	DELETE /v1/api/orgs/:orgID/billing/subscription
//
// Protected under /v1/api/orgs/:orgID/plan (root/system only):
//
//	POST /v1/api/orgs/:orgID/plan/enterprise
func RegisterBilling(e *echo.Echo, v1 *echo.Group, db *gorm.DB, c *cache.Cache, cfg *config.Config, jwtSecret string) {
	h := handler.NewBillingHandler(db, c, cfg)

	// Stripe webhook — not versioned, no JWT (signature is the auth).
	e.POST("/stripe/webhook", h.HandleWebhook)

	// Checkout + cancel — org owner or root/system bypass.
	ownerOrHigher := middleware.RequireOrgRole(db, model.OrgRoleOwner)
	billing := v1.Group("/api/orgs/:orgID/billing")
	billing.Use(middleware.JWT(jwtSecret))
	billing.POST("/checkout", h.CreateCheckout, ownerOrHigher)
	billing.DELETE("/subscription", h.CancelSubscription, ownerOrHigher)

	// Enterprise activation — root/system only.
	systemOrRoot := middleware.RequireSystemRole(model.RoleSystem, model.RoleRoot)
	plan := v1.Group("/api/orgs/:orgID/plan")
	plan.Use(middleware.JWT(jwtSecret))
	plan.POST("/enterprise", h.AssignEnterprisePlan, systemOrRoot)
}
