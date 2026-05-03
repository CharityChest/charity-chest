package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	stripe "github.com/stripe/stripe-go/v82"
	stripewebhook "github.com/stripe/stripe-go/v82/webhook"

	"charity-chest/internal/cache"
	"charity-chest/internal/config"
	"charity-chest/internal/i18n"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// StripeGateway abstracts the Stripe API calls used by BillingHandler.
// The interface is exported so tests can inject a mock implementation.
type StripeGateway interface {
	CreateCheckoutSession(c context.Context, params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error)
	CancelSubscription(id string) error
	RefundPayment(paymentIntentID string) error
}

// BillingHandler handles subscription plan management and Stripe integration.
type BillingHandler struct {
	db     *gorm.DB
	cache  *cache.Cache
	cfg    *config.Config
	stripe StripeGateway
}

// NewBillingHandler creates a BillingHandler. When cfg.StripeSecretKey is set,
// a real Stripe gateway is wired in automatically.
func NewBillingHandler(db *gorm.DB, c *cache.Cache, cfg *config.Config) *BillingHandler {
	h := &BillingHandler{db: db, cache: c, cfg: cfg}
	if cfg.StripeSecretKey != "" {
		h.stripe = newStripeGoGateway(cfg.StripeSecretKey)
	}
	return h
}

// NewBillingHandlerWithGateway creates a BillingHandler with a custom Stripe
// gateway — used in tests to inject a mock without real network calls.
func NewBillingHandlerWithGateway(db *gorm.DB, c *cache.Cache, cfg *config.Config, gw StripeGateway) *BillingHandler {
	return &BillingHandler{db: db, cache: c, cfg: cfg, stripe: gw}
}

// --- Real Stripe gateway ---

// stripeGoGateway is the production StripeGateway implementation backed by stripe-go.
type stripeGoGateway struct {
	client *stripe.Client
}

// newStripeGoGateway creates a gateway with its own pre-configured Stripe client.
// The API key is bound at construction time so no global state is ever mutated.
func newStripeGoGateway(secretKey string) *stripeGoGateway {
	return &stripeGoGateway{client: stripe.NewClient(secretKey)}
}

// CreateCheckoutSession delegates to the Stripe CheckoutSessions API.
func (g *stripeGoGateway) CreateCheckoutSession(c context.Context, params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
	return g.client.V1CheckoutSessions.Create(c, params)
}

// CancelSubscription cancels a Stripe subscription immediately.
func (g *stripeGoGateway) CancelSubscription(id string) error {
	params := &stripe.SubscriptionCancelParams{}
	_, err := g.client.V1Subscriptions.Cancel(context.TODO(), id, params)
	return err
}

// RefundPayment issues a full refund for the given PaymentIntent.
func (g *stripeGoGateway) RefundPayment(paymentIntentID string) error {
	params := &stripe.RefundCreateParams{
		PaymentIntent: stripe.String(paymentIntentID),
	}
	_, err := g.client.V1Refunds.Create(context.TODO(), params)
	return err
}

// --- Handlers ---

// CreateCheckout godoc — POST /v1/api/orgs/:orgID/billing/checkout
// Returns a Stripe Checkout URL to upgrade the org to Pro.
func (h *BillingHandler) CreateCheckout(c echo.Context) error {
	loc := locale(c)
	if h.stripe == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, i18n.T(loc, i18n.KeyStripeNotConfigured))
	}
	orgID, err := parseOrgID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}
	var org model.Organization
	if err := h.db.First(&org, orgID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
	}
	if org.Plan == model.PlanPro || org.Plan == model.PlanEnterprise {
		return echo.NewHTTPError(http.StatusConflict, i18n.T(loc, i18n.KeyPlanAlreadyActive))
	}

	requestedLocale := c.QueryParam("locale")
	if requestedLocale != "it" {
		requestedLocale = "en"
	}
	successURL := fmt.Sprintf("%s/%s/billing/success?org_id=%d&session_id={CHECKOUT_SESSION_ID}",
		h.cfg.FrontendURL, requestedLocale, orgID)
	cancelURL := fmt.Sprintf("%s/%s/billing/cancel?org_id=%d",
		h.cfg.FrontendURL, requestedLocale, orgID)

	params := &stripe.CheckoutSessionCreateParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{
				Price:    stripe.String(h.cfg.StripePriceIDPro),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"org_id": strconv.FormatUint(uint64(orgID), 10),
		},
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
	}
	if org.StripeCustomerID != nil {
		params.Customer = org.StripeCustomerID
	}

	sess, err := h.stripe.CreateCheckoutSession(context.TODO(), params)
	if err != nil {
		log.Printf("billing: create checkout for org %d: %v", orgID, err)
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyBillingCheckoutFailed))
	}

	return dataJSON(c, http.StatusOK, map[string]string{"url": sess.URL})
}

// HandleWebhook godoc — POST /stripe/webhook
// Processes Stripe subscription lifecycle events to update org plans.
// Returns 503 immediately when STRIPE_WEBHOOK_SECRET is not configured so that
// unsigned events can never alter plan state. Outside production, signature
// verification is skipped to support local dev and automated tests.
func (h *BillingHandler) HandleWebhook(c echo.Context) error {
	loc := locale(c)
	if h.cfg.StripeWebhookSecret == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, i18n.T(loc, i18n.KeyStripeNotConfigured))
	}

	payload, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyReadBodyFailed))
	}

	var event stripe.Event
	if h.cfg.AppEnv != config.AppEnvProduction {
		if err := json.Unmarshal(payload, &event); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidEventPayload))
		}
	} else {
		sig := c.Request().Header.Get("Stripe-Signature")
		event, err = stripewebhook.ConstructEvent(payload, sig, h.cfg.StripeWebhookSecret)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidWebhookSignature))
		}
	}

	ctx := c.Request().Context()

	switch event.Type {
	case "checkout.session.completed":
		var sess struct {
			Metadata      map[string]string `json:"metadata"`
			Customer      string            `json:"customer"`
			Subscription  string            `json:"subscription"`
			PaymentIntent string            `json:"payment_intent"`
		}
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			log.Printf("webhook: unmarshal checkout.session.completed: %v", err)
			break
		}
		orgIDStr, ok := sess.Metadata["org_id"]
		if !ok || orgIDStr == "" {
			break
		}
		orgID64, err := strconv.ParseUint(orgIDStr, 10, 64)
		if err != nil {
			break
		}
		orgID := uint(orgID64)

		// If the org is already on enterprise, cancel the new subscription and
		// refund the payment rather than silently downgrading the plan.
		var existing model.Organization
		if err := h.db.First(&existing, orgID).Error; err == nil && existing.Plan == model.PlanEnterprise {
			if h.stripe != nil {
				if sess.Subscription != "" {
					if err := h.stripe.CancelSubscription(sess.Subscription); err != nil {
						log.Printf("webhook: cancel subscription %s for enterprise org %d: %v", sess.Subscription, orgID, err)
					}
				}
				if sess.PaymentIntent != "" {
					if err := h.stripe.RefundPayment(sess.PaymentIntent); err != nil {
						log.Printf("webhook: refund payment %s for enterprise org %d: %v", sess.PaymentIntent, orgID, err)
					}
				}
			}
			return c.NoContent(http.StatusOK)
		}

		updates := map[string]any{
			"plan":                   model.PlanPro,
			"stripe_customer_id":     sess.Customer,
			"stripe_subscription_id": sess.Subscription,
		}
		if err := h.db.Model(&model.Organization{}).Where("id = ?", orgID).Updates(updates).Error; err != nil {
			log.Printf("webhook: upgrade org %d to pro: %v", orgID, err)
			return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
		}
		if err := h.cache.Del(ctx, cache.KeyOrg(orgID), cache.KeyOrgsList); err != nil {
			log.Printf("webhook: cache invalidate org %d: %v", orgID, err)
		}

	case "customer.subscription.deleted":
		var sub struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			log.Printf("webhook: unmarshal customer.subscription.deleted: %v", err)
			break
		}
		var org model.Organization
		if err := h.db.Where("stripe_subscription_id = ?", sub.ID).First(&org).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				break
			}
			log.Printf("webhook: lookup org by subscription %s: %v", sub.ID, err)
			return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
		}
		updates := map[string]any{
			"plan":                   model.PlanFree,
			"stripe_subscription_id": nil,
		}
		if err := h.db.Model(&org).Updates(updates).Error; err != nil {
			log.Printf("webhook: downgrade org %d to free: %v", org.ID, err)
			return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
		}
		if err := h.cache.Del(ctx, cache.KeyOrg(org.ID), cache.KeyOrgsList); err != nil {
			log.Printf("webhook: cache invalidate org %d: %v", org.ID, err)
		}
	}

	return c.NoContent(http.StatusOK)
}

// CancelSubscription godoc — DELETE /v1/api/orgs/:orgID/billing/subscription
// Cancels the Stripe subscription. The plan reverts to free via webhook.
func (h *BillingHandler) CancelSubscription(c echo.Context) error {
	loc := locale(c)
	if h.stripe == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, i18n.T(loc, i18n.KeyStripeNotConfigured))
	}
	orgID, err := parseOrgID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}
	var org model.Organization
	if err := h.db.First(&org, orgID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
	}
	if org.Plan != model.PlanPro || org.StripeSubscriptionID == nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, i18n.T(loc, i18n.KeySubscriptionNotFound))
	}
	if err := h.stripe.CancelSubscription(*org.StripeSubscriptionID); err != nil {
		log.Printf("billing: cancel subscription for org %d: %v", orgID, err)
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyCancelSubscriptionFailed))
	}
	return c.NoContent(http.StatusNoContent)
}

// AssignEnterprisePlan godoc — POST /v1/api/orgs/:orgID/plan/enterprise
// Manually activates the enterprise plan (root/system only).
// If the org has an active Stripe subscription it is cancelled best-effort.
func (h *BillingHandler) AssignEnterprisePlan(c echo.Context) error {
	loc := locale(c)
	orgID, err := parseOrgID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}
	var org model.Organization
	if err := h.db.First(&org, orgID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
	}
	if org.Plan == model.PlanEnterprise {
		return echo.NewHTTPError(http.StatusConflict, i18n.T(loc, i18n.KeyPlanAlreadyActive))
	}
	if h.stripe != nil && org.StripeSubscriptionID != nil {
		if err := h.stripe.CancelSubscription(*org.StripeSubscriptionID); err != nil {
			log.Printf("billing: cancel stripe subscription for org %d during enterprise upgrade: %v", orgID, err)
			return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyCancelSubscriptionFailed))
		}
	}

	updates := map[string]any{
		"plan":                   model.PlanEnterprise,
		"stripe_subscription_id": nil,
	}
	if err := h.db.Model(&org).Updates(updates).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
	}

	ctx := c.Request().Context()
	if err := h.cache.Del(ctx, cache.KeyOrg(orgID), cache.KeyOrgsList); err != nil {
		log.Printf("billing: cache invalidate org %d after enterprise: %v", orgID, err)
	}

	// Reload to return the updated org.
	if err := h.db.First(&org, orgID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
	}
	return dataJSON(c, http.StatusOK, &org)
}
