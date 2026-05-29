package v1

import (
	"charity-chest/services/admin/backend/internal/cache"
	"charity-chest/services/admin/backend/internal/config"
	"charity-chest/services/admin/backend/internal/handler"
	"charity-chest/services/admin/backend/internal/middleware"
	"charity-chest/services/admin/backend/internal/model"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RegisterBilling registers billing and plan management routes.
//
// Unversioned (no auth — Stripe signature is the authentication mechanism):
//
//	POST /stripe/webhook
//
// Protected under /v1/api/orgs/:orgUUID/billing (org owner or root/system):
//
//	POST   /v1/api/orgs/:orgUUID/billing/checkout
//	DELETE /v1/api/orgs/:orgUUID/billing/subscription
//
// Protected under /v1/api/orgs/:orgUUID/plan (root/system only):
//
//	POST /v1/api/orgs/:orgUUID/plan/enterprise
//
// gw may be nil; when nil the handler auto-constructs a real Stripe gateway
// from cfg.StripeSecretKey. Pass a non-nil value in tests to inject a mock.
func RegisterBilling(e *echo.Echo, v1 *echo.Group, db *gorm.DB, c *cache.Cache, cfg *config.Config, jwtSecret string, gw handler.StripeGateway) {
	var h *handler.BillingHandler
	if gw != nil {
		h = handler.NewBillingHandlerWithGateway(db, c, cfg, gw)
	} else {
		h = handler.NewBillingHandler(db, c, cfg)
	}

	// Stripe webhook — not versioned, no JWT (signature is the auth).
	e.POST("/stripe/webhook", h.HandleWebhook)

	// Checkout + cancel — org owner or root/system bypass.
	// RequireOrgRole also resolves :orgUUID into OrgIDContextKey so the
	// handlers can skip a second lookup.
	ownerOrHigher := middleware.RequireOrgRole(db, model.OrgRoleOwner)
	billing := v1.Group("/api/orgs/:orgUUID/billing")
	billing.Use(middleware.JWT(db, jwtSecret))
	billing.POST("/checkout", h.CreateCheckout, ownerOrHigher)
	billing.DELETE("/subscription", h.CancelSubscription, ownerOrHigher)

	// Enterprise activation — root/system only.
	systemOrRoot := middleware.RequireSystemRole(model.RoleSystem, model.RoleRoot)
	plan := v1.Group("/api/orgs/:orgUUID/plan")
	plan.Use(middleware.JWT(db, jwtSecret))
	plan.POST("/enterprise", h.AssignEnterprisePlan, systemOrRoot)
}
