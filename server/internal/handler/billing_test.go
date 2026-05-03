package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	stripe "github.com/stripe/stripe-go/v82"

	"charity-chest/internal/cache"
	"charity-chest/internal/config"
	"charity-chest/internal/handler"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
)

// --- Mock Stripe gateway ---

type mockStripeGateway struct {
	createFn func(ctx context.Context, params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error)
	cancelFn func(id string) error
	refundFn func(paymentIntentID string) error
}

func (m *mockStripeGateway) CreateCheckoutSession(c context.Context, params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
	if m.createFn != nil {
		return m.createFn(c, params)
	}
	return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/test"}, nil
}

func (m *mockStripeGateway) CancelSubscription(id string) error {
	if m.cancelFn != nil {
		return m.cancelFn(id)
	}
	return nil
}

func (m *mockStripeGateway) RefundPayment(paymentIntentID string) error {
	if m.refundFn != nil {
		return m.refundFn(paymentIntentID)
	}
	return nil
}

// --- Helpers ---

func newBillingCfg() *config.Config {
	return &config.Config{
		FrontendURL:      "http://localhost:3000",
		StripePriceIDPro: "price_test123",
	}
}

func newBillingCfgWithStripe() *config.Config {
	cfg := newBillingCfg()
	cfg.StripeSecretKey = "sk_test_xxx"
	cfg.StripeWebhookSecret = "" // empty → skip sig verification in tests
	return cfg
}

func newBillingContext(t *testing.T, method, query, body string, orgID uint, userID uint, sysRole *model.AdministrativeRole) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	path := "/"
	if query != "" {
		path = "/?" + strings.TrimPrefix(query, "?")
	}
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	if orgID != 0 {
		c.SetParamNames("orgID")
		c.SetParamValues(fmt.Sprintf("%d", orgID))
	}
	c.Set(middleware.UserIDContextKey, userID)
	if sysRole != nil {
		c.Set(middleware.RoleContextKey, sysRole)
	}
	return c, rec
}

func stripeEventBody(eventType string, object map[string]any) string {
	objBytes, _ := json.Marshal(object)
	return fmt.Sprintf(`{"type":%q,"data":{"object":%s}}`, eventType, string(objBytes))
}

func newWebhookContext(t *testing.T, body string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/stripe/webhook", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	return c, rec
}

// --- CreateCheckout ---

func TestCreateCheckout_StripeNotConfigured_Returns503(t *testing.T) {
	db := newOrgTestDB(t)
	org := model.Organization{Name: "Org", Plan: model.PlanFree}
	db.Create(&org)

	h := handler.NewBillingHandler(db, cache.Disabled(), newBillingCfg())
	root := model.RoleRoot
	c, _ := newBillingContext(t, http.MethodPost, "", "", org.ID, 1, &root)
	err := h.CreateCheckout(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 HTTPError, got %v", err)
	}
}

func TestCreateCheckout_OrgNotFound_Returns404(t *testing.T) {
	db := newOrgTestDB(t)
	mock := &mockStripeGateway{}
	h := handler.NewBillingHandlerWithGateway(db, cache.Disabled(), newBillingCfgWithStripe(), mock)

	root := model.RoleRoot
	c, _ := newBillingContext(t, http.MethodPost, "", "", 9999, 1, &root)
	err := h.CreateCheckout(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 HTTPError, got %v", err)
	}
}

func TestCreateCheckout_AlreadyPro_Returns409(t *testing.T) {
	db := newOrgTestDB(t)
	org := model.Organization{Name: "Org", Plan: model.PlanPro}
	db.Create(&org)

	mock := &mockStripeGateway{}
	h := handler.NewBillingHandlerWithGateway(db, cache.Disabled(), newBillingCfgWithStripe(), mock)

	root := model.RoleRoot
	c, _ := newBillingContext(t, http.MethodPost, "", "", org.ID, 1, &root)
	err := h.CreateCheckout(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusConflict {
		t.Errorf("expected 409 HTTPError, got %v", err)
	}
}

func TestCreateCheckout_AlreadyEnterprise_Returns409(t *testing.T) {
	db := newOrgTestDB(t)
	org := model.Organization{Name: "Org", Plan: model.PlanEnterprise}
	db.Create(&org)

	mock := &mockStripeGateway{}
	h := handler.NewBillingHandlerWithGateway(db, cache.Disabled(), newBillingCfgWithStripe(), mock)

	root := model.RoleRoot
	c, _ := newBillingContext(t, http.MethodPost, "", "", org.ID, 1, &root)
	err := h.CreateCheckout(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusConflict {
		t.Errorf("expected 409 HTTPError, got %v", err)
	}
}

func TestCreateCheckout_Success_ReturnsURL(t *testing.T) {
	db := newOrgTestDB(t)
	org := model.Organization{Name: "Org", Plan: model.PlanFree}
	db.Create(&org)

	mock := &mockStripeGateway{
		createFn: func(ctx context.Context, params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
			return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/pay/cs_test_abc"}, nil
		},
	}
	h := handler.NewBillingHandlerWithGateway(db, cache.Disabled(), newBillingCfgWithStripe(), mock)

	root := model.RoleRoot
	c, rec := newBillingContext(t, http.MethodPost, "locale=en", "", org.ID, 1, &root)
	if err := h.CreateCheckout(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatal("missing data object in response")
	}
	if data["url"] != "https://checkout.stripe.com/pay/cs_test_abc" {
		t.Errorf("url = %v, want checkout URL", data["url"])
	}
}

func TestCreateCheckout_ReuseExistingCustomer(t *testing.T) {
	db := newOrgTestDB(t)
	cusID := "cus_existing123"
	org := model.Organization{Name: "Org", Plan: model.PlanFree, StripeCustomerID: &cusID}
	db.Create(&org)

	var capturedParams *stripe.CheckoutSessionCreateParams
	mock := &mockStripeGateway{
		createFn: func(ctx context.Context, params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
			capturedParams = params
			return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/test"}, nil
		},
	}
	h := handler.NewBillingHandlerWithGateway(db, cache.Disabled(), newBillingCfgWithStripe(), mock)

	root := model.RoleRoot
	c, _ := newBillingContext(t, http.MethodPost, "", "", org.ID, 1, &root)
	if err := h.CreateCheckout(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedParams == nil || capturedParams.Customer == nil {
		t.Fatal("expected Customer to be set in params")
	}
	if *capturedParams.Customer != cusID {
		t.Errorf("Customer = %q, want %q", *capturedParams.Customer, cusID)
	}
}

// --- HandleWebhook ---

func TestHandleWebhook_CheckoutCompleted_UpgradesToPro(t *testing.T) {
	db := newOrgTestDB(t)
	org := model.Organization{Name: "Org", Plan: model.PlanFree}
	db.Create(&org)

	cfg := &config.Config{AppEnv: config.AppEnvLocal, StripeWebhookSecret: "whsec_test"} // non-prod: sig verification skipped
	h := handler.NewBillingHandler(db, cache.Disabled(), cfg)

	body := stripeEventBody("checkout.session.completed", map[string]any{
		"metadata":     map[string]string{"org_id": fmt.Sprintf("%d", org.ID)},
		"customer":     "cus_test123",
		"subscription": "sub_test123",
	})
	c, rec := newWebhookContext(t, body)
	if err := h.HandleWebhook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var updated model.Organization
	db.First(&updated, org.ID)
	if updated.Plan != model.PlanPro {
		t.Errorf("plan = %q, want pro", updated.Plan)
	}
	if updated.StripeCustomerID == nil || *updated.StripeCustomerID != "cus_test123" {
		t.Errorf("stripe_customer_id = %v, want cus_test123", updated.StripeCustomerID)
	}
	if updated.StripeSubscriptionID == nil || *updated.StripeSubscriptionID != "sub_test123" {
		t.Errorf("stripe_subscription_id = %v, want sub_test123", updated.StripeSubscriptionID)
	}
}

func TestHandleWebhook_CheckoutCompleted_EnterpriseOrg_CancelsAndRefundsReturns200(t *testing.T) {
	db := newOrgTestDB(t)
	org := model.Organization{Name: "Org", Plan: model.PlanEnterprise}
	db.Create(&org)

	var cancelledSub, refundedPI string
	mock := &mockStripeGateway{
		cancelFn: func(id string) error { cancelledSub = id; return nil },
		refundFn: func(id string) error { refundedPI = id; return nil },
	}
	cfg := &config.Config{AppEnv: config.AppEnvLocal, StripeWebhookSecret: "whsec_test",
		StripeSecretKey: "sk_test_xxx", StripePriceIDPro: "price_test"}
	h := handler.NewBillingHandlerWithGateway(db, cache.Disabled(), cfg, mock)

	body := stripeEventBody("checkout.session.completed", map[string]any{
		"metadata":       map[string]string{"org_id": fmt.Sprintf("%d", org.ID)},
		"customer":       "cus_test",
		"subscription":   "sub_test",
		"payment_intent": "pi_test",
	})
	c, _ := newWebhookContext(t, body)
	err := h.HandleWebhook(c)
	if err != nil {
		t.Errorf("expected 200 (no error), got %v", err)
	}
	if cancelledSub != "sub_test" {
		t.Errorf("CancelSubscription called with %q, want sub_test", cancelledSub)
	}
	if refundedPI != "pi_test" {
		t.Errorf("RefundPayment called with %q, want pi_test", refundedPI)
	}
	var unchanged model.Organization
	db.First(&unchanged, org.ID)
	if unchanged.Plan != model.PlanEnterprise {
		t.Errorf("plan = %q, want enterprise (must not be downgraded)", unchanged.Plan)
	}
}

func TestHandleWebhook_SubscriptionDeleted_DowngradesToFree(t *testing.T) {
	db := newOrgTestDB(t)
	subID := "sub_tobedeleted"
	org := model.Organization{Name: "Org", Plan: model.PlanPro, StripeSubscriptionID: &subID}
	db.Create(&org)

	cfg := &config.Config{AppEnv: config.AppEnvLocal, StripeWebhookSecret: "whsec_test"}
	h := handler.NewBillingHandler(db, cache.Disabled(), cfg)

	body := stripeEventBody("customer.subscription.deleted", map[string]any{
		"id": subID,
	})
	c, rec := newWebhookContext(t, body)
	if err := h.HandleWebhook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var updated model.Organization
	db.First(&updated, org.ID)
	if updated.Plan != model.PlanFree {
		t.Errorf("plan = %q, want free", updated.Plan)
	}
	if updated.StripeSubscriptionID != nil {
		t.Errorf("stripe_subscription_id should be nil after downgrade, got %v", *updated.StripeSubscriptionID)
	}
}

func TestHandleWebhook_InvalidSignature_Returns400(t *testing.T) {
	db := newOrgTestDB(t)
	cfg := &config.Config{AppEnv: config.AppEnvProduction, StripeWebhookSecret: "whsec_testsecret"}
	h := handler.NewBillingHandler(db, cache.Disabled(), cfg)

	c, _ := newWebhookContext(t, `{"type":"checkout.session.completed","data":{"object":{}}}`)
	// No Stripe-Signature header → signature verification fails in production.
	err := h.HandleWebhook(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400 HTTPError, got %v", err)
	}
}

func TestHandleWebhook_UnknownEvent_Returns200(t *testing.T) {
	db := newOrgTestDB(t)
	cfg := &config.Config{AppEnv: config.AppEnvLocal, StripeWebhookSecret: "whsec_test"}
	h := handler.NewBillingHandler(db, cache.Disabled(), cfg)

	body := stripeEventBody("payment_intent.created", map[string]any{"id": "pi_test"})
	c, rec := newWebhookContext(t, body)
	if err := h.HandleWebhook(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for unknown event", rec.Code)
	}
}

func TestHandleWebhook_ProductionWithoutSecret_Returns503(t *testing.T) {
	db := newOrgTestDB(t)
	cfg := &config.Config{AppEnv: config.AppEnvProduction, StripeWebhookSecret: ""}
	h := handler.NewBillingHandler(db, cache.Disabled(), cfg)

	body := stripeEventBody("checkout.session.completed", map[string]any{
		"metadata": map[string]string{"org_id": "1"},
	})
	c, _ := newWebhookContext(t, body)
	err := h.HandleWebhook(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 in production without webhook secret, got %v", err)
	}
}

func TestHandleWebhook_NonProductionWithoutSecret_Returns503(t *testing.T) {
	// An empty StripeWebhookSecret always returns 503, even outside production.
	db := newOrgTestDB(t)
	cfg := &config.Config{AppEnv: config.AppEnvLocal, StripeWebhookSecret: ""}
	h := handler.NewBillingHandler(db, cache.Disabled(), cfg)

	body := stripeEventBody("checkout.session.completed", map[string]any{
		"metadata": map[string]string{"org_id": "1"},
	})
	c, _ := newWebhookContext(t, body)
	err := h.HandleWebhook(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 without webhook secret, got %v", err)
	}
}

// --- CancelSubscription ---

func TestCancelSubscription_StripeNotConfigured_Returns503(t *testing.T) {
	db := newOrgTestDB(t)
	org := model.Organization{Name: "Org", Plan: model.PlanPro}
	db.Create(&org)

	h := handler.NewBillingHandler(db, cache.Disabled(), newBillingCfg())
	root := model.RoleRoot
	c, _ := newBillingContext(t, http.MethodDelete, "", "", org.ID, 1, &root)
	err := h.CancelSubscription(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 HTTPError, got %v", err)
	}
}

func TestCancelSubscription_NotPro_Returns422(t *testing.T) {
	db := newOrgTestDB(t)
	org := model.Organization{Name: "Org", Plan: model.PlanFree}
	db.Create(&org)

	mock := &mockStripeGateway{}
	h := handler.NewBillingHandlerWithGateway(db, cache.Disabled(), newBillingCfgWithStripe(), mock)

	root := model.RoleRoot
	c, _ := newBillingContext(t, http.MethodDelete, "", "", org.ID, 1, &root)
	err := h.CancelSubscription(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 HTTPError, got %v", err)
	}
}

func TestCancelSubscription_Success_Returns204(t *testing.T) {
	db := newOrgTestDB(t)
	subID := "sub_tocancel"
	org := model.Organization{Name: "Org", Plan: model.PlanPro, StripeSubscriptionID: &subID}
	db.Create(&org)

	var cancelledID string
	mock := &mockStripeGateway{
		cancelFn: func(id string) error {
			cancelledID = id
			return nil
		},
	}
	h := handler.NewBillingHandlerWithGateway(db, cache.Disabled(), newBillingCfgWithStripe(), mock)

	root := model.RoleRoot
	c, rec := newBillingContext(t, http.MethodDelete, "", "", org.ID, 1, &root)
	if err := h.CancelSubscription(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if cancelledID != subID {
		t.Errorf("cancelled subscription ID = %q, want %q", cancelledID, subID)
	}
}

// --- AssignEnterprisePlan ---

func TestAssignEnterprisePlan_Success(t *testing.T) {
	db := newOrgTestDB(t)
	org := model.Organization{Name: "Org", Plan: model.PlanFree}
	db.Create(&org)

	root := model.RoleRoot
	h := handler.NewBillingHandler(db, cache.Disabled(), newBillingCfg())
	c, rec := newBillingContext(t, http.MethodPost, "", "", org.ID, 1, &root)
	if err := h.AssignEnterprisePlan(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var updated model.Organization
	db.First(&updated, org.ID)
	if updated.Plan != model.PlanEnterprise {
		t.Errorf("plan = %q, want enterprise", updated.Plan)
	}
}

func TestAssignEnterprisePlan_AlreadyEnterprise_Returns409(t *testing.T) {
	db := newOrgTestDB(t)
	org := model.Organization{Name: "Org", Plan: model.PlanEnterprise}
	db.Create(&org)

	root := model.RoleRoot
	h := handler.NewBillingHandler(db, cache.Disabled(), newBillingCfg())
	c, _ := newBillingContext(t, http.MethodPost, "", "", org.ID, 1, &root)
	err := h.AssignEnterprisePlan(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusConflict {
		t.Errorf("expected 409 HTTPError, got %v", err)
	}
}

func TestAssignEnterprisePlan_CancelsStripeSubscription(t *testing.T) {
	db := newOrgTestDB(t)
	subID := "sub_existing"
	org := model.Organization{Name: "Org", Plan: model.PlanPro, StripeSubscriptionID: &subID}
	db.Create(&org)

	var cancelledID string
	mock := &mockStripeGateway{
		cancelFn: func(id string) error {
			cancelledID = id
			return nil
		},
	}
	root := model.RoleRoot
	h := handler.NewBillingHandlerWithGateway(db, cache.Disabled(), newBillingCfgWithStripe(), mock)
	c, _ := newBillingContext(t, http.MethodPost, "", "", org.ID, 1, &root)
	if err := h.AssignEnterprisePlan(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cancelledID != subID {
		t.Errorf("expected Stripe cancel called with %q, got %q", subID, cancelledID)
	}
	var updated model.Organization
	db.First(&updated, org.ID)
	if updated.StripeSubscriptionID != nil {
		t.Error("stripe_subscription_id should be nil after enterprise assignment")
	}
}

func TestAssignEnterprisePlan_CancelFails_Returns500(t *testing.T) {
	db := newOrgTestDB(t)
	subID := "sub_existing"
	org := model.Organization{Name: "Org", Plan: model.PlanPro, StripeSubscriptionID: &subID}
	db.Create(&org)

	mock := &mockStripeGateway{
		cancelFn: func(id string) error {
			return fmt.Errorf("stripe unreachable")
		},
	}
	root := model.RoleRoot
	h := handler.NewBillingHandlerWithGateway(db, cache.Disabled(), newBillingCfgWithStripe(), mock)
	c, _ := newBillingContext(t, http.MethodPost, "", "", org.ID, 1, &root)
	err := h.AssignEnterprisePlan(c)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 HTTPError, got %v", err)
	}
	// Stripe ID must be preserved; the org must not be promoted.
	var unchanged model.Organization
	db.First(&unchanged, org.ID)
	if unchanged.Plan != model.PlanPro {
		t.Errorf("plan = %q, want pro (upgrade must be aborted)", unchanged.Plan)
	}
	if unchanged.StripeSubscriptionID == nil || *unchanged.StripeSubscriptionID != subID {
		t.Error("stripe_subscription_id must be preserved when cancellation fails")
	}
}

func TestAssignEnterprisePlan_CacheInvalidated(t *testing.T) {
	db := newOrgTestDB(t)
	_, c := newMiniRedisCache(t)
	org := model.Organization{Name: "Org", Plan: model.PlanFree}
	db.Create(&org)

	// Populate cache.
	orgH := handler.NewOrgHandler(db, c)
	root := model.RoleRoot
	getCtx, _ := newOrgContext(t, http.MethodGet, fmt.Sprintf("/v1/api/orgs/%d", org.ID), "", org.ID, 1, &root, "")
	_ = orgH.GetOrg(getCtx)

	// Delete from DB to confirm cache was set.
	db.Exec("DELETE FROM organizations")
	getCtx2, rec2 := newOrgContext(t, http.MethodGet, fmt.Sprintf("/v1/api/orgs/%d", org.ID), "", org.ID, 1, &root, "")
	_ = orgH.GetOrg(getCtx2)
	if rec2.Code != http.StatusOK {
		t.Fatal("expected cache hit before enterprise assignment")
	}

	// Restore org in DB and assign enterprise.
	db.Create(&org)
	bilH := handler.NewBillingHandler(db, c, newBillingCfg())
	enterpriseCtx, _ := newBillingContext(t, http.MethodPost, "", "", org.ID, 1, &root)
	if err := bilH.AssignEnterprisePlan(enterpriseCtx); err != nil {
		t.Fatalf("AssignEnterprisePlan: %v", err)
	}

	// Cache should be cleared — next GetOrg reads from DB.
	db.Exec("DELETE FROM organizations WHERE id = ?", org.ID)
	getCtx3, rec3 := newOrgContext(t, http.MethodGet, fmt.Sprintf("/v1/api/orgs/%d", org.ID), "", org.ID, 1, &root, "")
	err := orgH.GetOrg(getCtx3)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusNotFound {
		_ = rec3
		t.Errorf("expected 404 after cache invalidation, got status %d err %v", rec3.Code, err)
	}
}

func TestAssignEnterprisePlan_BrokenCache_Succeeds(t *testing.T) {
	db := newOrgTestDB(t)
	mr, c := newMiniRedisCache(t)
	org := model.Organization{Name: "Org", Plan: model.PlanFree}
	db.Create(&org)
	mr.Close()

	root := model.RoleRoot
	h := handler.NewBillingHandler(db, c, newBillingCfg())
	ctx, rec := newBillingContext(t, http.MethodPost, "", "", org.ID, 1, &root)
	if err := h.AssignEnterprisePlan(ctx); err != nil {
		t.Fatalf("AssignEnterprisePlan with broken cache: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
