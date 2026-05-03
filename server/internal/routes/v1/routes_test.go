package v1_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"charity-chest/internal/cache"
	"charity-chest/internal/config"
	"charity-chest/internal/handler"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"
	routesv1 "charity-chest/internal/routes/v1"

	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/pquerna/otp/totp"
	stripe "github.com/stripe/stripe-go/v82"
	"gorm.io/gorm"
)

// --- Setup helpers ---

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Organization{}, &model.OrgMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func testCfg() *config.Config {
	return &config.Config{
		AppEnv:              config.AppEnvTesting,
		JWTSecret:           "e2e-test-jwt-secret",
		GoogleClientID:      "test-google-client-id",
		GoogleClientSecret:  "test-google-client-secret",
		GoogleRedirectURL:   "http://localhost:8080/v1/auth/google/callback",
		Port:                "8080",
		StripeWebhookSecret: "whsec_test",
	}
}

// mockStripeGateway is a no-op StripeGateway used in e2e tests to exercise
// billing happy paths without real Stripe network calls.
type mockStripeGateway struct{}

func (m *mockStripeGateway) CreateCheckoutSession(_ context.Context, _ *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
	return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/pay/mock-session"}, nil
}

func (m *mockStripeGateway) CancelSubscription(_ string) error { return nil }

func (m *mockStripeGateway) RefundPayment(_ string) error { return nil }

// newServerWithStripe wires the full Echo instance with an injectable Stripe
// gateway. Pass nil to get the default behaviour (real gateway from cfg, which
// returns nil when StripeSecretKey is unset).
func newServerWithStripe(t *testing.T, gw handler.StripeGateway) (*echo.Echo, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	cfg := testCfg()
	noCache := cache.Disabled()
	h := handler.NewAuthHandler(db, cfg, noCache)

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Locale())

	routesv1.RegisterHealth(e)

	v1 := e.Group("/v1")
	routesv1.RegisterAuth(v1, h)
	routesv1.RegisterAPI(v1, h, cfg.JWTSecret)
	routesv1.RegisterSystem(v1, db, noCache, cfg.JWTSecret)
	routesv1.RegisterOrgs(v1, db, noCache, cfg.JWTSecret)
	routesv1.RegisterProfile(v1, db, cfg, noCache, cfg.JWTSecret)
	routesv1.RegisterAdmin(v1, db, noCache, cfg.JWTSecret)
	routesv1.RegisterBilling(e, v1, db, noCache, cfg, cfg.JWTSecret, gw)

	return e, db
}

// newServer wires the full Echo instance — all routes plus middleware — mirroring
// main.go so that locale detection and JWT enforcement behave identically in tests.
func newServer(t *testing.T) (*echo.Echo, *gorm.DB) {
	return newServerWithStripe(t, nil)
}

// makeUserWithRole creates a user in the DB with the given system-level role and
// returns a signed JWT for that user.
func makeUserWithRole(t *testing.T, db *gorm.DB, email, name string, role model.AdministrativeRole) (string, *model.User) {
	t.Helper()
	r := role
	user := &model.User{Email: email, Name: name, Role: &r}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("makeUserWithRole: %v", err)
	}
	cfg := testCfg()
	claims := middleware.Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatalf("makeUserWithRole sign: %v", err)
	}
	return tok, user
}

// makeOrgMember creates an org_member row and returns the OrgMember.
func makeOrgMember(t *testing.T, db *gorm.DB, orgID, userID uint, role model.MemberRole) model.OrgMember {
	t.Helper()
	m := model.OrgMember{OrgID: orgID, UserID: userID, Role: role}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("makeOrgMember: %v", err)
	}
	return m
}

// makeOrg creates an enterprise organisation and returns it.
// Enterprise plan avoids member-limit interference in tests that focus on
// role hierarchy or other non-plan-related behaviour.
func makeOrg(t *testing.T, db *gorm.DB, name string) model.Organization {
	t.Helper()
	org := model.Organization{Name: name, Plan: model.PlanEnterprise}
	if err := db.Create(&org).Error; err != nil {
		t.Fatalf("makeOrg: %v", err)
	}
	return org
}

// makeFreeOrg creates a free-plan organisation for plan-limit tests.
func makeFreeOrg(t *testing.T, db *gorm.DB, name string) model.Organization {
	t.Helper()
	org := model.Organization{Name: name, Plan: model.PlanFree}
	if err := db.Create(&org).Error; err != nil {
		t.Fatalf("makeFreeOrg: %v", err)
	}
	return org
}

// do fires an HTTP request through the full Echo pipeline.
// Pass locale="" to omit the X-Locale header (defaults to "en" on the server).
func do(e *echo.Echo, method, path, body, bearerToken, locale string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	if bearerToken != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+bearerToken)
	}
	if locale != "" {
		req.Header.Set("X-Locale", locale)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// decodeBody unmarshals the response body into a generic map.
func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return m
}

// decodeDataBody unwraps the {"data": {...}} envelope for single-object responses.
func decodeDataBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	outer := decodeBody(t, rec)
	data, ok := outer["data"].(map[string]any)
	if !ok {
		t.Fatalf("response missing 'data' key or not an object; body: %s", rec.Body.String())
	}
	return data
}

// registerUser registers a user and returns the JWT from the response.
func registerUser(t *testing.T, e *echo.Echo, email, password, name string) string {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q,"name":%q}`, email, password, name)
	rec := do(e, http.MethodPost, "/v1/auth/register", body, "", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("registerUser: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	data := decodeDataBody(t, rec)
	token, ok := data["token"].(string)
	if !ok || token == "" {
		t.Fatal("registerUser: missing token in response")
	}
	return token
}

// loginUser logs in and returns the JWT.
func loginUser(t *testing.T, e *echo.Echo, email, password string) string {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	rec := do(e, http.MethodPost, "/v1/auth/login", body, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("loginUser: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	data := decodeDataBody(t, rec)
	token, ok := data["token"].(string)
	if !ok || token == "" {
		t.Fatal("loginUser: missing token in response")
	}
	return token
}

// --- Health ---

func TestHealth(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/health", "", "", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decodeDataBody(t, rec)
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
}

// --- Register ---

func TestRegister_Success(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodPost, "/v1/auth/register",
		`{"email":"alice@example.com","password":"password123","name":"Alice"}`, "", "")

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeDataBody(t, rec)
	if body["token"] == nil || body["token"] == "" {
		t.Error("missing token in response")
	}
	user, ok := body["user"].(map[string]any)
	if !ok {
		t.Fatal("missing user object in response")
	}
	if user["email"] != "alice@example.com" {
		t.Errorf("email = %v, want alice@example.com", user["email"])
	}
	if user["name"] != "Alice" {
		t.Errorf("name = %v, want Alice", user["name"])
	}
	if _, present := user["password_hash"]; present {
		t.Error("password_hash must not be exposed in response")
	}
	if _, present := user["google_id"]; present {
		t.Error("google_id must not be exposed in response")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	e, _ := newServer(t)
	body := `{"email":"dup@example.com","password":"password123","name":"Dup"}`
	do(e, http.MethodPost, "/v1/auth/register", body, "", "")        // first — OK
	rec := do(e, http.MethodPost, "/v1/auth/register", body, "", "") // duplicate

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestRegister_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"no_email", `{"password":"password123","name":"No Email"}`},
		{"no_password", `{"email":"a@b.com","name":"No Pass"}`},
		{"no_name", `{"email":"a@b.com","password":"password123"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, _ := newServer(t)
			rec := do(e, http.MethodPost, "/v1/auth/register", tc.body, "", "")
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodPost, "/v1/auth/register",
		`{"email":"a@b.com","password":"short","name":"User"}`, "", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- Login ---

func TestLogin_Success(t *testing.T) {
	e, _ := newServer(t)
	registerUser(t, e, "bob@example.com", "password123", "Bob")

	rec := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"bob@example.com","password":"password123"}`, "", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeDataBody(t, rec)
	if body["token"] == nil {
		t.Error("missing token in response")
	}
	user, ok := body["user"].(map[string]any)
	if !ok {
		t.Fatal("missing user object in response")
	}
	if user["email"] != "bob@example.com" {
		t.Errorf("email = %v, want bob@example.com", user["email"])
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	e, _ := newServer(t)
	registerUser(t, e, "carol@example.com", "correct-password", "Carol")

	rec := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"carol@example.com","password":"wrong-password"}`, "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"ghost@example.com","password":"password123"}`, "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_WrongPassword_SameStatusAsUnknownEmail(t *testing.T) {
	// Both cases must return 401 — no user enumeration.
	e, _ := newServer(t)
	registerUser(t, e, "dave@example.com", "correct-password", "Dave")

	recWrongPw := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"dave@example.com","password":"wrong"}`, "", "")
	recUnknown := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"nobody@example.com","password":"password123"}`, "", "")

	if recWrongPw.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: status = %d, want 401", recWrongPw.Code)
	}
	if recUnknown.Code != http.StatusUnauthorized {
		t.Errorf("unknown email: status = %d, want 401", recUnknown.Code)
	}
}

func TestLogin_GoogleOnlyAccount(t *testing.T) {
	e, db := newServer(t)
	googleID := "google-uid-123"
	db.Create(&model.User{Email: "google@example.com", Name: "Google User", GoogleID: &googleID})

	rec := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"google@example.com","password":"any-password"}`, "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- Google OAuth ---

func TestGoogleLogin_Redirects(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/v1/auth/google", "", "", "")

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want 307", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "accounts.google.com") {
		t.Errorf("Location %q does not point to accounts.google.com", loc)
	}
}

func TestGoogleLogin_SetsStateCookie(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/v1/auth/google", "", "", "")

	var stateCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == handler.CookieOAuthState {
			stateCookie = c
			break
		}
	}
	if stateCookie == nil {
		t.Fatalf("%s cookie was not set", handler.CookieOAuthState)
	}
	if stateCookie.Value == "" {
		t.Errorf("%s cookie value is empty", handler.CookieOAuthState)
	}
	if !stateCookie.HttpOnly {
		t.Errorf("%s cookie must be HttpOnly", handler.CookieOAuthState)
	}
}

func TestGoogleCallback_MissingStateCookie(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/v1/auth/google/callback?state=somestate&code=somecode", "", "", "")
	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want 307", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "error=sign_in_failed") {
		t.Errorf("Location %q does not contain error=sign_in_failed", loc)
	}
}

func TestGoogleCallback_StateMismatch(t *testing.T) {
	e, _ := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?state=url-state&code=somecode", nil)
	req.AddCookie(&http.Cookie{Name: handler.CookieOAuthState, Value: "different-cookie-state"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want 307", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "error=sign_in_failed") {
		t.Errorf("Location %q does not contain error=sign_in_failed", loc)
	}
}

func TestGoogleCallback_MissingCode(t *testing.T) {
	e, _ := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?state=matching-state", nil)
	req.AddCookie(&http.Cookie{Name: handler.CookieOAuthState, Value: "matching-state"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want 307", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "error=sign_in_failed") {
		t.Errorf("Location %q does not contain error=sign_in_failed", loc)
	}
}

func TestGoogleLogin_SetsLocaleCookie_IT(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/v1/auth/google?locale=it", "", "", "")

	var localeCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == handler.CookieOAuthLocale {
			localeCookie = c
			break
		}
	}
	if localeCookie == nil {
		t.Fatalf("%s cookie was not set", handler.CookieOAuthLocale)
	}
	if localeCookie.Value != middleware.LocaleIT {
		t.Errorf("%s cookie value = %q, want %q", handler.CookieOAuthLocale, localeCookie.Value, middleware.LocaleIT)
	}
	if !localeCookie.HttpOnly {
		t.Errorf("%s cookie must be HttpOnly", handler.CookieOAuthLocale)
	}
}

func TestGoogleLogin_SetsLocaleCookie_DefaultEN(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/v1/auth/google", "", "", "")

	var localeCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == handler.CookieOAuthLocale {
			localeCookie = c
			break
		}
	}
	if localeCookie == nil {
		t.Fatalf("%s cookie was not set", handler.CookieOAuthLocale)
	}
	if localeCookie.Value != middleware.LocaleEN {
		t.Errorf("%s cookie value = %q, want %q", handler.CookieOAuthLocale, localeCookie.Value, middleware.LocaleEN)
	}
}

func TestGoogleCallback_UsesLocaleFromCookie(t *testing.T) {
	e, _ := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?state=some-state&code=some-code", nil)
	req.AddCookie(&http.Cookie{Name: handler.CookieOAuthState, Value: "some-state"})
	req.AddCookie(&http.Cookie{Name: handler.CookieOAuthLocale, Value: middleware.LocaleIT})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want 307", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "/"+middleware.LocaleIT+"/auth/callback") {
		t.Errorf("Location %q does not contain /%s/auth/callback", loc, middleware.LocaleIT)
	}
}

func TestGoogleCallback_DefaultsToENLocale_WhenCookieMissing(t *testing.T) {
	e, _ := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?state=some-state", nil)
	req.AddCookie(&http.Cookie{Name: handler.CookieOAuthState, Value: "some-state"})
	// no oauth_locale cookie — should default to "en"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want 307", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "/"+middleware.LocaleEN+"/auth/callback") {
		t.Errorf("Location %q does not contain /%s/auth/callback", loc, middleware.LocaleEN)
	}
}

// --- Me ---

func TestMe_Success(t *testing.T) {
	e, _ := newServer(t)
	token := registerUser(t, e, "me@example.com", "password123", "Me User")

	rec := do(e, http.MethodGet, "/v1/api/me", "", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeDataBody(t, rec)
	if body["email"] != "me@example.com" {
		t.Errorf("email = %v, want me@example.com", body["email"])
	}
	if body["name"] != "Me User" {
		t.Errorf("name = %v, want Me User", body["name"])
	}
	if _, present := body["password_hash"]; present {
		t.Error("password_hash must not be exposed in response")
	}
}

func TestMe_NoToken(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/v1/api/me", "", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMe_InvalidToken(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/v1/api/me", "", "this-is-not-a-valid-jwt", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMe_TokenSignedWithWrongSecret(t *testing.T) {
	// Register on a server with a different JWT secret — its token must be rejected.
	cfg := testCfg()
	cfg.JWTSecret = "attacker-secret"
	db := newTestDB(t)
	attackerHandler := handler.NewAuthHandler(db, cfg, cache.Disabled())
	attackerEcho := echo.New()
	attackerEcho.Use(middleware.Locale())
	v1 := attackerEcho.Group("/v1")
	routesv1.RegisterAuth(v1, attackerHandler)

	rec := do(attackerEcho, http.MethodPost, "/v1/auth/register",
		`{"email":"attacker@example.com","password":"password123","name":"Attacker"}`, "", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("attacker register: %d", rec.Code)
	}
	m := decodeDataBody(t, rec)
	foreignToken := m["token"].(string)

	// Present that token to our legitimate server.
	e, _ := newServer(t)
	result := do(e, http.MethodGet, "/v1/api/me", "", foreignToken, "")
	if result.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", result.Code)
	}
}

func TestMe_TokenRefreshedAfterReLogin(t *testing.T) {
	e, _ := newServer(t)
	registerUser(t, e, "eve@example.com", "password123", "Eve")
	token := loginUser(t, e, "eve@example.com", "password123")

	rec := do(e, http.MethodGet, "/v1/api/me", "", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestMe_TokenRefreshedAfterReLoginNotFoundITA(t *testing.T) {
	e, _ := newServer(t)
	email := "joe@example.com"
	password := "password123"
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	rec := do(e, http.MethodPost, "/v1/auth/login", body, "", "it")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	bodyMap := decodeBody(t, rec)
	const want = "Credenziali non valide"
	if bodyMap["message"] != want {
		t.Errorf("message = %q, want %q", bodyMap["message"], want)
	}
}

// --- i18n / locale ---

func TestRegister_LocaleIT(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodPost, "/v1/auth/register",
		`{"email":"a@b.com","password":"password123"}`, "", "it")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	body := decodeBody(t, rec)
	const want = "email, password e nome sono obbligatori"
	if body["message"] != want {
		t.Errorf("message = %q, want %q", body["message"], want)
	}
}

func TestLogin_LocaleIT(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"nobody@example.com","password":"password123"}`, "", "it")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	body := decodeBody(t, rec)
	const want = "Credenziali non valide"
	if body["message"] != want {
		t.Errorf("message = %q, want %q", body["message"], want)
	}
}

func TestMe_LocaleIT_NoToken(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/v1/api/me", "", "", "it")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	body := decodeBody(t, rec)
	const want = "intestazione di autorizzazione mancante o non valida"
	if body["message"] != want {
		t.Errorf("message = %q, want %q", body["message"], want)
	}
}

func TestLocale_DefaultsToEN(t *testing.T) {
	e, _ := newServer(t)
	// No X-Locale header → English
	rec := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"nobody@example.com","password":"p"}`, "", "")
	body := decodeBody(t, rec)
	if body["message"] != "invalid credentials" {
		t.Errorf("message = %q, want \"invalid credentials\"", body["message"])
	}
}

func TestLocale_UnknownLocale_DefaultsToEN(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"nobody@example.com","password":"p"}`, "", "fr")
	body := decodeBody(t, rec)
	if body["message"] != "invalid credentials" {
		t.Errorf("message = %q, want \"invalid credentials\"", body["message"])
	}
}

func TestLocale_IT(t *testing.T) {
	// X-Locale: it should resolve to Italian
	e, _ := newServer(t)
	rec := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"nobody@example.com","password":"p"}`, "", "it")
	body := decodeBody(t, rec)
	if body["message"] != "Credenziali non valide" {
		t.Errorf("message = %q, want \"Credenziali non valide\"", body["message"])
	}
}

// --- System Status ---

func TestSystemStatus_Unconfigured(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/v1/system/status", "", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decodeDataBody(t, rec)
	if body["configured"] != false {
		t.Errorf("configured = %v, want false", body["configured"])
	}
}

func TestSystemStatus_Configured(t *testing.T) {
	e, db := newServer(t)
	makeUserWithRole(t, db, "root@example.com", "Root", model.RoleRoot)

	rec := do(e, http.MethodGet, "/v1/system/status", "", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decodeDataBody(t, rec)
	if body["configured"] != true {
		t.Errorf("configured = %v, want true", body["configured"])
	}
}

// --- Assign System Role ---

func TestAssignSystemRole_RootCanAssignSystem(t *testing.T) {
	e, db := newServer(t)
	rootToken, _ := makeUserWithRole(t, db, "root@example.com", "Root", model.RoleRoot)
	token := registerUser(t, e, "target@example.com", "password123", "Target")
	_ = token

	// Get the target user's ID.
	var target model.User
	db.Where("email = ?", "target@example.com").First(&target)

	body := fmt.Sprintf(`{"user_id":%d,"role":"system"}`, target.ID)
	rec := do(e, http.MethodPost, "/v1/api/system/assign-role", body, rootToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	resp := decodeDataBody(t, rec)
	if resp["role"] != "system" {
		t.Errorf("role = %v, want system", resp["role"])
	}
}

func TestAssignSystemRole_NonRootForbidden(t *testing.T) {
	e, db := newServer(t)
	sysToken, _ := makeUserWithRole(t, db, "sys@example.com", "System", model.RoleSystem)
	var target model.User
	db.Where("email = ?", "sys@example.com").First(&target)

	body := fmt.Sprintf(`{"user_id":%d,"role":"system"}`, target.ID)
	rec := do(e, http.MethodPost, "/v1/api/system/assign-role", body, sysToken, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestAssignSystemRole_NoJWTUnauthorized(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodPost, "/v1/api/system/assign-role", `{"user_id":1,"role":"system"}`, "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAssignSystemRole_CannotPromoteRoot(t *testing.T) {
	e, db := newServer(t)
	rootToken, rootUser := makeUserWithRole(t, db, "root@example.com", "Root", model.RoleRoot)
	body := fmt.Sprintf(`{"user_id":%d,"role":"system"}`, rootUser.ID)
	rec := do(e, http.MethodPost, "/v1/api/system/assign-role", body, rootToken, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestAssignSystemRole_InvalidRole(t *testing.T) {
	e, db := newServer(t)
	rootToken, _ := makeUserWithRole(t, db, "root@example.com", "Root", model.RoleRoot)
	registerUser(t, e, "target@example.com", "password123", "Target")
	var target model.User
	db.Where("email = ?", "target@example.com").First(&target)

	body := fmt.Sprintf(`{"user_id":%d,"role":"owner"}`, target.ID) // owner is not a system role
	rec := do(e, http.MethodPost, "/v1/api/system/assign-role", body, rootToken, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- Org CRUD ---

func TestCreateOrg_SystemRole(t *testing.T) {
	e, db := newServer(t)
	sysToken, _ := makeUserWithRole(t, db, "sys@example.com", "System", model.RoleSystem)

	rec := do(e, http.MethodPost, "/v1/api/orgs", `{"name":"Test Org"}`, sysToken, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeDataBody(t, rec)
	if body["name"] != "Test Org" {
		t.Errorf("name = %v, want Test Org", body["name"])
	}
}

func TestCreateOrg_RootRole(t *testing.T) {
	e, db := newServer(t)
	rootToken, _ := makeUserWithRole(t, db, "root@example.com", "Root", model.RoleRoot)

	rec := do(e, http.MethodPost, "/v1/api/orgs", `{"name":"Root Org"}`, rootToken, "")
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
}

func TestCreateOrg_NoRoleForbidden(t *testing.T) {
	e, _ := newServer(t)
	token := registerUser(t, e, "user@example.com", "password123", "User")

	rec := do(e, http.MethodPost, "/v1/api/orgs", `{"name":"Org"}`, token, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestCreateOrg_NoJWTUnauthorized(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodPost, "/v1/api/orgs", `{"name":"Org"}`, "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestListOrgs_SystemRole(t *testing.T) {
	e, db := newServer(t)
	sysToken, _ := makeUserWithRole(t, db, "sys@example.com", "System", model.RoleSystem)
	makeOrg(t, db, "Org 1")
	makeOrg(t, db, "Org 2")

	rec := do(e, http.MethodGet, "/v1/api/orgs", "", sysToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestDeleteOrg_SystemRole(t *testing.T) {
	e, db := newServer(t)
	sysToken, _ := makeUserWithRole(t, db, "sys@example.com", "System", model.RoleSystem)
	org := makeOrg(t, db, "ToDelete")

	rec := do(e, http.MethodDelete, fmt.Sprintf("/v1/api/orgs/%d", org.ID), "", sysToken, "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

// --- GetOrg and access control ---

func TestGetOrg_OrgMemberCanAccess(t *testing.T) {
	e, db := newServer(t)
	sysToken, _ := makeUserWithRole(t, db, "sys@example.com", "System", model.RoleSystem)

	// Create org via API so we get a real ID.
	rec := do(e, http.MethodPost, "/v1/api/orgs", `{"name":"Members Org"}`, sysToken, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create org: %d", rec.Code)
	}
	orgBody := decodeDataBody(t, rec)
	orgID := uint(orgBody["id"].(float64))

	// Create an owner user and add as member directly in DB.
	_, ownerUser := makeUserWithRole(t, db, "owner@example.com", "Owner", "")
	// Clear the empty role so user has no system role.
	db.Model(ownerUser).Update("role", nil)
	// Re-sign token without role for this user.
	cfg := testCfg()
	claims := middleware.Claims{
		UserID: ownerUser.ID,
		Email:  ownerUser.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	ownerToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))

	makeOrgMember(t, db, orgID, ownerUser.ID, model.OrgRoleOwner)

	rec = do(e, http.MethodGet, fmt.Sprintf("/v1/api/orgs/%d", orgID), "", ownerToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestGetOrg_NonMemberForbidden(t *testing.T) {
	e, db := newServer(t)
	sysToken, _ := makeUserWithRole(t, db, "sys@example.com", "System", model.RoleSystem)

	rec := do(e, http.MethodPost, "/v1/api/orgs", `{"name":"Private Org"}`, sysToken, "")
	orgBody := decodeDataBody(t, rec)
	orgID := uint(orgBody["id"].(float64))

	// A regular registered user (no org membership).
	userToken := registerUser(t, e, "outsider@example.com", "password123", "Outsider")

	rec = do(e, http.MethodGet, fmt.Sprintf("/v1/api/orgs/%d", orgID), "", userToken, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestGetOrg_SystemBypassesMembership(t *testing.T) {
	e, db := newServer(t)
	sysToken, _ := makeUserWithRole(t, db, "sys@example.com", "System", model.RoleSystem)
	org := makeOrg(t, db, "Any Org")

	rec := do(e, http.MethodGet, fmt.Sprintf("/v1/api/orgs/%d", org.ID), "", sysToken, "")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// --- Member management hierarchy ---

func TestAddMember_OwnerCanAddAdmin(t *testing.T) {
	e, db := newServer(t)
	org := makeOrg(t, db, "Hierarchy Org")

	_, ownerUser := makeUserWithRole(t, db, "owner@example.com", "Owner", "")
	db.Model(ownerUser).Update("role", nil)
	makeOrgMember(t, db, org.ID, ownerUser.ID, model.OrgRoleOwner)

	// Sign a token without system role for the owner.
	cfg := testCfg()
	claims := middleware.Claims{
		UserID: ownerUser.ID,
		Email:  ownerUser.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	ownerToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))

	adminToken := registerUser(t, e, "admin@example.com", "password123", "Admin")
	_ = adminToken
	var adminUser model.User
	db.Where("email = ?", "admin@example.com").First(&adminUser)

	body := fmt.Sprintf(`{"user_id":%d,"role":"admin"}`, adminUser.ID)
	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/members", org.ID), body, ownerToken, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAddMember_AdminCanAddOperational(t *testing.T) {
	e, db := newServer(t)
	org := makeOrg(t, db, "Hierarchy Org")

	_, adminUser := makeUserWithRole(t, db, "admin@example.com", "Admin", "")
	db.Model(adminUser).Update("role", nil)
	makeOrgMember(t, db, org.ID, adminUser.ID, model.OrgRoleAdmin)

	cfg := testCfg()
	claims := middleware.Claims{
		UserID: adminUser.ID,
		Email:  adminUser.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	adminToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))

	registerUser(t, e, "op@example.com", "password123", "Operational")
	var opUser model.User
	db.Where("email = ?", "op@example.com").First(&opUser)

	body := fmt.Sprintf(`{"user_id":%d,"role":"operational"}`, opUser.ID)
	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/members", org.ID), body, adminToken, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAddMember_AdminCannotAddOwner(t *testing.T) {
	e, db := newServer(t)
	org := makeOrg(t, db, "Hierarchy Org")

	_, adminUser := makeUserWithRole(t, db, "admin@example.com", "Admin", "")
	db.Model(adminUser).Update("role", nil)
	makeOrgMember(t, db, org.ID, adminUser.ID, model.OrgRoleAdmin)

	cfg := testCfg()
	claims := middleware.Claims{
		UserID: adminUser.ID,
		Email:  adminUser.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	adminToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))

	registerUser(t, e, "newowner@example.com", "password123", "NewOwner")
	var newOwner model.User
	db.Where("email = ?", "newowner@example.com").First(&newOwner)

	body := fmt.Sprintf(`{"user_id":%d,"role":"owner"}`, newOwner.ID)
	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/members", org.ID), body, adminToken, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestAddMember_AdminCannotAddAdmin(t *testing.T) {
	e, db := newServer(t)
	org := makeOrg(t, db, "Hierarchy Org")

	_, adminUser := makeUserWithRole(t, db, "admin@example.com", "Admin", "")
	db.Model(adminUser).Update("role", nil)
	makeOrgMember(t, db, org.ID, adminUser.ID, model.OrgRoleAdmin)

	cfg := testCfg()
	claims := middleware.Claims{
		UserID: adminUser.ID,
		Email:  adminUser.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	adminToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))

	registerUser(t, e, "another@example.com", "password123", "Another")
	var another model.User
	db.Where("email = ?", "another@example.com").First(&another)

	body := fmt.Sprintf(`{"user_id":%d,"role":"admin"}`, another.ID)
	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/members", org.ID), body, adminToken, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestAddMember_OperationalCannotAddAnyone(t *testing.T) {
	e, db := newServer(t)
	org := makeOrg(t, db, "Hierarchy Org")

	_, opUser := makeUserWithRole(t, db, "op@example.com", "Op", "")
	db.Model(opUser).Update("role", nil)
	makeOrgMember(t, db, org.ID, opUser.ID, model.OrgRoleOperational)

	cfg := testCfg()
	claims := middleware.Claims{
		UserID: opUser.ID,
		Email:  opUser.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	opToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))

	registerUser(t, e, "another@example.com", "password123", "Another")
	var another model.User
	db.Where("email = ?", "another@example.com").First(&another)

	body := fmt.Sprintf(`{"user_id":%d,"role":"operational"}`, another.ID)
	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/members", org.ID), body, opToken, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestAddMember_DuplicateMemberConflict(t *testing.T) {
	e, db := newServer(t)
	sysToken, _ := makeUserWithRole(t, db, "sys@example.com", "System", model.RoleSystem)

	rec := do(e, http.MethodPost, "/v1/api/orgs", `{"name":"DupOrg"}`, sysToken, "")
	orgBody := decodeDataBody(t, rec)
	orgID := uint(orgBody["id"].(float64))

	registerUser(t, e, "dup@example.com", "password123", "Dup")
	var dupUser model.User
	db.Where("email = ?", "dup@example.com").First(&dupUser)

	body := fmt.Sprintf(`{"user_id":%d,"role":"operational"}`, dupUser.ID)
	rec = do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/members", orgID), body, sysToken, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("first add: status = %d; body: %s", rec.Code, rec.Body.String())
	}
	rec = do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/members", orgID), body, sysToken, "")
	if rec.Code != http.StatusConflict {
		t.Errorf("duplicate: status = %d, want 409", rec.Code)
	}
}

func TestAddMember_InvalidRole(t *testing.T) {
	e, db := newServer(t)
	sysToken, _ := makeUserWithRole(t, db, "sys@example.com", "System", model.RoleSystem)
	org := makeOrg(t, db, "Org")

	body := `{"user_id":99,"role":"superadmin"}`
	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/members", org.ID), body, sysToken, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestRemoveMember_OwnerCanRemoveAdmin(t *testing.T) {
	e, db := newServer(t)
	org := makeOrg(t, db, "Org")

	_, ownerUser := makeUserWithRole(t, db, "owner@example.com", "Owner", "")
	db.Model(ownerUser).Update("role", nil)
	makeOrgMember(t, db, org.ID, ownerUser.ID, model.OrgRoleOwner)

	cfg := testCfg()
	claims := middleware.Claims{
		UserID: ownerUser.ID,
		Email:  ownerUser.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	ownerToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))

	registerUser(t, e, "admin@example.com", "password123", "Admin")
	var adminUser model.User
	db.Where("email = ?", "admin@example.com").First(&adminUser)
	makeOrgMember(t, db, org.ID, adminUser.ID, model.OrgRoleAdmin)

	rec := do(e, http.MethodDelete,
		fmt.Sprintf("/v1/api/orgs/%d/members/%d", org.ID, adminUser.ID), "", ownerToken, "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestRemoveMember_AdminCannotRemoveOwner(t *testing.T) {
	e, db := newServer(t)
	org := makeOrg(t, db, "Org")

	_, adminUser := makeUserWithRole(t, db, "admin@example.com", "Admin", "")
	db.Model(adminUser).Update("role", nil)
	makeOrgMember(t, db, org.ID, adminUser.ID, model.OrgRoleAdmin)

	cfg := testCfg()
	claims := middleware.Claims{
		UserID: adminUser.ID,
		Email:  adminUser.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	adminToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))

	_, ownerUser := makeUserWithRole(t, db, "owner@example.com", "Owner", "")
	db.Model(ownerUser).Update("role", nil)
	makeOrgMember(t, db, org.ID, ownerUser.ID, model.OrgRoleOwner)

	rec := do(e, http.MethodDelete,
		fmt.Sprintf("/v1/api/orgs/%d/members/%d", org.ID, ownerUser.ID), "", adminToken, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestListMembers_OrgMemberCanList(t *testing.T) {
	e, db := newServer(t)
	org := makeOrg(t, db, "Org")

	_, opUser := makeUserWithRole(t, db, "op@example.com", "Op", "")
	db.Model(opUser).Update("role", nil)
	makeOrgMember(t, db, org.ID, opUser.ID, model.OrgRoleOperational)

	cfg := testCfg()
	claims := middleware.Claims{
		UserID: opUser.ID,
		Email:  opUser.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	opToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))

	rec := do(e, http.MethodGet, fmt.Sprintf("/v1/api/orgs/%d/members", org.ID), "", opToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestMe_RoleIncludedInResponse(t *testing.T) {
	e, db := newServer(t)
	sysToken, _ := makeUserWithRole(t, db, "sys@example.com", "System", model.RoleSystem)

	rec := do(e, http.MethodGet, "/v1/api/me", "", sysToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decodeDataBody(t, rec)
	if body["role"] != string(model.RoleSystem) {
		t.Errorf("role = %v, want %s", body["role"], model.RoleSystem)
	}
}

// --- MFA E2E ---

func TestMFA_LoginNoMFA_ReturnsTokenDirectly(t *testing.T) {
	e, _ := newServer(t)
	registerUser(t, e, "nomfa@example.com", "password123", "No MFA")

	rec := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"nomfa@example.com","password":"password123"}`, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeDataBody(t, rec)
	if body["token"] == nil || body["token"] == "" {
		t.Error("token missing for user without MFA")
	}
	if body["mfa_required"] == true {
		t.Error("mfa_required must not be true for users without MFA")
	}
}

func TestMFA_FullFlow(t *testing.T) {
	e, db := newServer(t)
	token := registerUser(t, e, "fullmfa@example.com", "password123", "Full MFA")

	// 1. Setup MFA — generates secret and QR URI.
	rec := do(e, http.MethodGet, "/v1/api/profile/mfa/setup", "", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("setup: status = %d; body: %s", rec.Code, rec.Body.String())
	}
	setupBody := decodeDataBody(t, rec)
	secret, _ := setupBody["secret"].(string)
	if secret == "" {
		t.Fatal("setup: secret missing")
	}

	// 2. Enable MFA with a valid TOTP code.
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	rec = do(e, http.MethodPost, "/v1/api/profile/mfa/enable",
		`{"code":"`+code+`"}`, token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("enable: status = %d; body: %s", rec.Code, rec.Body.String())
	}

	// 3. Verify MFA is reflected in GET /api/me.
	rec = do(e, http.MethodGet, "/v1/api/me", "", token, "")
	meBody := decodeDataBody(t, rec)
	if meBody["mfa_enabled"] != true {
		t.Errorf("me.mfa_enabled = %v, want true", meBody["mfa_enabled"])
	}

	// 4. Login now returns mfa_required.
	rec = do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"fullmfa@example.com","password":"password123"}`, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login: status = %d; body: %s", rec.Code, rec.Body.String())
	}
	loginBody := decodeDataBody(t, rec)
	if loginBody["mfa_required"] != true {
		t.Errorf("mfa_required = %v, want true", loginBody["mfa_required"])
	}
	mfaToken, _ := loginBody["mfa_token"].(string)
	if mfaToken == "" {
		t.Fatal("mfa_token missing")
	}

	// 5. Verify MFA — issue a fresh code (same 30-second window).
	code2, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("generate code2: %v", err)
	}
	rec = do(e, http.MethodPost, "/v1/auth/mfa/verify",
		`{"mfa_token":"`+mfaToken+`","code":"`+code2+`"}`, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("verify: status = %d; body: %s", rec.Code, rec.Body.String())
	}
	verifyBody := decodeDataBody(t, rec)
	fullToken, _ := verifyBody["token"].(string)
	if fullToken == "" {
		t.Fatal("full token missing after MFA verify")
	}

	// 6. Full token must work on protected routes.
	rec = do(e, http.MethodGet, "/v1/api/me", "", fullToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("me with full token: status = %d", rec.Code)
	}

	// 7. Disable MFA.
	code3, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("generate code3: %v", err)
	}
	rec = do(e, http.MethodDelete, "/v1/api/profile/mfa",
		`{"code":"`+code3+`"}`, fullToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("disable: status = %d; body: %s", rec.Code, rec.Body.String())
	}

	// 8. Verify login now returns a direct token again.
	rec = do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"fullmfa@example.com","password":"password123"}`, "", "")
	afterBody := decodeDataBody(t, rec)
	if afterBody["mfa_required"] == true {
		t.Error("mfa_required still true after disable")
	}
	if afterBody["token"] == nil || afterBody["token"] == "" {
		t.Error("token missing after MFA disabled")
	}
	_ = db
}

func TestMFA_VerifyMFA_WrongCode(t *testing.T) {
	e, db := newServer(t)
	registerUser(t, e, "wrongcode@example.com", "password123", "Wrong Code")

	secret := "JBSWY3DPEHPK3PXP"
	db.Model(&model.User{}).Where("email = ?", "wrongcode@example.com").Updates(map[string]any{
		"mfa_enabled": true,
		"totp_secret": secret,
	})

	rec := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"wrongcode@example.com","password":"password123"}`, "", "")
	loginBody := decodeDataBody(t, rec)
	mfaToken, _ := loginBody["mfa_token"].(string)

	rec = do(e, http.MethodPost, "/v1/auth/mfa/verify",
		`{"mfa_token":"`+mfaToken+`","code":"000000"}`, "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMFAPendingToken_CannotAccessProtectedRoutes(t *testing.T) {
	e, _ := newServer(t)
	registerUser(t, e, "pending@example.com", "password123", "Pending")

	cfg := testCfg()
	pending := true
	var user model.User
	e2, db2 := newServer(t)
	registerUser(t, e2, "pending2@example.com", "password123", "P2")
	db2.Where("email = ?", "pending2@example.com").First(&user)

	claims := middleware.Claims{
		UserID:     user.ID,
		Email:      user.Email,
		MFAPending: &pending,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	pendingTok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))

	rec := do(e2, http.MethodGet, "/v1/api/me", "", pendingTok, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("pending token used on protected route: status = %d, want 401", rec.Code)
	}
}

func TestSetupMFA_Unauthorized(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/v1/api/profile/mfa/setup", "", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestEnableMFA_Unauthorized(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodPost, "/v1/api/profile/mfa/enable", `{"code":"123456"}`, "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestDisableMFA_Unauthorized(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodDelete, "/v1/api/profile/mfa", `{"code":"123456"}`, "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- Admin: SearchUsers ---

func TestSearchUsers_RootCanAccess(t *testing.T) {
	e, db := newServer(t)
	rootToken, _ := makeUserWithRole(t, db, "root@example.com", "Root", model.RoleRoot)

	rec := do(e, http.MethodGet, "/v1/api/admin/users", "", rootToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)
	if _, ok := body["data"]; !ok {
		t.Error("response missing 'data' key")
	}
	if _, ok := body["metadata"]; !ok {
		t.Error("response missing 'metadata' key")
	}
}

func TestSearchUsers_NonRootForbidden(t *testing.T) {
	e, db := newServer(t)
	sysToken, _ := makeUserWithRole(t, db, "sys@example.com", "Sys", model.RoleSystem)

	rec := do(e, http.MethodGet, "/v1/api/admin/users", "", sysToken, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestSearchUsers_NoJWT_Unauthorized(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/v1/api/admin/users", "", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestSearchUsers_EmailFilter_E2E(t *testing.T) {
	e, db := newServer(t)
	rootToken, _ := makeUserWithRole(t, db, "root@example.com", "Root", model.RoleRoot)
	registerUser(t, e, "alpha@example.com", "password123", "Alpha")
	registerUser(t, e, "beta@example.com", "password123", "Beta")

	rec := do(e, http.MethodGet, "/v1/api/admin/users?email=alpha", "", rootToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data is not an array")
	}
	if len(data) != 1 {
		t.Errorf("len(data) = %d, want 1", len(data))
	}
	user := data[0].(map[string]any)
	if user["email"] != "alpha@example.com" {
		t.Errorf("email = %v, want alpha@example.com", user["email"])
	}

	meta := body["metadata"].(map[string]any)
	if meta["total"].(float64) != 1 {
		t.Errorf("metadata.total = %v, want 1", meta["total"])
	}
}

func TestSearchUsers_PaginationE2E(t *testing.T) {
	e, db := newServer(t)
	rootToken, _ := makeUserWithRole(t, db, "root@example.com", "Root", model.RoleRoot)
	for i := range 5 {
		registerUser(t, e, fmt.Sprintf("u%d@example.com", i), "password123", fmt.Sprintf("U%d", i))
	}

	rec := do(e, http.MethodGet, "/v1/api/admin/users?page=2&size=2", "", rootToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)
	data := body["data"].([]any)
	if len(data) != 2 {
		t.Errorf("len(data) = %d, want 2", len(data))
	}
	meta := body["metadata"].(map[string]any)
	if meta["page"].(float64) != 2 {
		t.Errorf("page = %v, want 2", meta["page"])
	}
	// root user + 5 registered = 6 total; size=2 → 3 pages
	if meta["total_pages"].(float64) != 3 {
		t.Errorf("total_pages = %v, want 3", meta["total_pages"])
	}
}

// --- Billing & plan management ---

func TestBillingCheckout_Unauthenticated_Returns401(t *testing.T) {
	e, db := newServer(t)
	org := makeFreeOrg(t, db, "Org")
	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/billing/checkout", org.ID), "", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestBillingCheckout_StripeNotConfigured_Returns503(t *testing.T) {
	e, db := newServer(t)
	rootToken, _ := makeUserWithRole(t, db, "root@example.com", "Root", model.RoleRoot)
	org := makeFreeOrg(t, db, "Org")
	makeOrgMember(t, db, org.ID, 1, model.OrgRoleOwner)

	// cfg has no StripeSecretKey → 503
	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/billing/checkout", org.ID), "", rootToken, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestAssignEnterprisePlan_ByRoot_Returns200(t *testing.T) {
	e, db := newServer(t)
	rootToken, _ := makeUserWithRole(t, db, "root@example.com", "Root", model.RoleRoot)
	org := makeFreeOrg(t, db, "Org")

	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/plan/enterprise", org.ID), "", rootToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	data := decodeDataBody(t, rec)
	if data["plan"] != string(model.PlanEnterprise) {
		t.Errorf("plan = %v, want enterprise", data["plan"])
	}
}

func TestAssignEnterprisePlan_BySystem_Returns200(t *testing.T) {
	e, db := newServer(t)
	sysToken, _ := makeUserWithRole(t, db, "sys@example.com", "System", model.RoleSystem)
	org := makeFreeOrg(t, db, "Org")

	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/plan/enterprise", org.ID), "", sysToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAssignEnterprisePlan_ByNonSystem_Returns403(t *testing.T) {
	e, db := newServer(t)
	// Regular user with no system role.
	userToken := registerUser(t, e, "plain@example.com", "password123", "Plain")
	org := makeFreeOrg(t, db, "Org")

	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/plan/enterprise", org.ID), "", userToken, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestWebhook_CheckoutCompleted_FlipsToPro(t *testing.T) {
	e, db := newServer(t)
	org := makeFreeOrg(t, db, "Org")

	body := fmt.Sprintf(`{"type":"checkout.session.completed","data":{"object":{"metadata":{"org_id":"%d"},"customer":"cus_test","subscription":"sub_test"}}}`, org.ID)
	rec := do(e, http.MethodPost, "/stripe/webhook", body, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("webhook status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var updated model.Organization
	db.First(&updated, org.ID)
	if updated.Plan != model.PlanPro {
		t.Errorf("plan = %q, want pro", updated.Plan)
	}
}

func TestWebhook_SubscriptionDeleted_FlipsToFree(t *testing.T) {
	e, db := newServer(t)
	subID := "sub_e2e_delete"
	org := model.Organization{Name: "Org", Plan: model.PlanPro, StripeSubscriptionID: &subID}
	db.Create(&org)

	body := fmt.Sprintf(`{"type":"customer.subscription.deleted","data":{"object":{"id":%q}}}`, subID)
	rec := do(e, http.MethodPost, "/stripe/webhook", body, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("webhook status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var updated model.Organization
	db.First(&updated, org.ID)
	if updated.Plan != model.PlanFree {
		t.Errorf("plan = %q, want free", updated.Plan)
	}
}

func TestAddMember_PlanLimitReached_E2e(t *testing.T) {
	e, db := newServer(t)
	rootToken, rootUser := makeUserWithRole(t, db, "root@example.com", "Root", model.RoleRoot)
	org := makeFreeOrg(t, db, "Free Org")
	makeOrgMember(t, db, org.ID, rootUser.ID, model.OrgRoleOwner)

	// Fill the 5-operational limit.
	for i := range 5 {
		registerUser(t, e, fmt.Sprintf("op%d@e2e.com", i), "password123", fmt.Sprintf("Op%d", i))
		var u model.User
		db.Where("email = ?", fmt.Sprintf("op%d@e2e.com", i)).First(&u)
		makeOrgMember(t, db, org.ID, u.ID, model.OrgRoleOperational)
	}

	// Adding a 6th operational must fail.
	registerUser(t, e, "sixth@e2e.com", "password123", "Sixth")
	var sixth model.User
	db.Where("email = ?", "sixth@e2e.com").First(&sixth)

	body := fmt.Sprintf(`{"user_id":%d,"role":"operational"}`, sixth.ID)
	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/members", org.ID), body, rootToken, "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestAssignEnterprisePlan_Unauthenticated_Returns401(t *testing.T) {
	e, db := newServer(t)
	org := makeFreeOrg(t, db, "Org")
	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/plan/enterprise", org.ID), "", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestBillingCheckout_ByNonOwner_Returns403(t *testing.T) {
	e, db := newServer(t)
	userToken := registerUser(t, e, "plain@example.com", "password123", "Plain")
	org := makeFreeOrg(t, db, "Org")
	// plain user is not an org member → RequireOrgRole returns 403
	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/billing/checkout", org.ID), "", userToken, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestCancelSubscription_Unauthenticated_Returns401(t *testing.T) {
	e, db := newServer(t)
	org := makeFreeOrg(t, db, "Org")
	rec := do(e, http.MethodDelete, fmt.Sprintf("/v1/api/orgs/%d/billing/subscription", org.ID), "", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestCancelSubscription_ByNonOwner_Returns403(t *testing.T) {
	e, db := newServer(t)
	userToken := registerUser(t, e, "plain@example.com", "password123", "Plain")
	org := makeFreeOrg(t, db, "Org")
	// plain user is not an org member → RequireOrgRole returns 403
	rec := do(e, http.MethodDelete, fmt.Sprintf("/v1/api/orgs/%d/billing/subscription", org.ID), "", userToken, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestCancelSubscription_ByOwner_StripeNotConfigured_Returns503(t *testing.T) {
	e, db := newServer(t)
	ownerToken := registerUser(t, e, "owner@example.com", "password123", "Owner")
	var ownerUser model.User
	db.Where("email = ?", "owner@example.com").First(&ownerUser)
	org := makeFreeOrg(t, db, "Org")
	makeOrgMember(t, db, org.ID, ownerUser.ID, model.OrgRoleOwner)
	// Owner passes JWT and role checks; cfg has no StripeSecretKey → 503.
	rec := do(e, http.MethodDelete, fmt.Sprintf("/v1/api/orgs/%d/billing/subscription", org.ID), "", ownerToken, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestBillingCheckout_ByOwner_Returns200(t *testing.T) {
	e, db := newServerWithStripe(t, &mockStripeGateway{})
	ownerToken := registerUser(t, e, "owner@example.com", "password123", "Owner")
	var ownerUser model.User
	db.Where("email = ?", "owner@example.com").First(&ownerUser)
	org := makeFreeOrg(t, db, "Org")
	makeOrgMember(t, db, org.ID, ownerUser.ID, model.OrgRoleOwner)

	rec := do(e, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%d/billing/checkout", org.ID), "", ownerToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	data := decodeDataBody(t, rec)
	url, ok := data["url"].(string)
	if !ok || url == "" {
		t.Errorf("url field missing or empty in response: %v", data)
	}
}

func TestCancelSubscription_ByOwner_Returns204(t *testing.T) {
	e, db := newServerWithStripe(t, &mockStripeGateway{})
	ownerToken := registerUser(t, e, "owner@example.com", "password123", "Owner")
	var ownerUser model.User
	db.Where("email = ?", "owner@example.com").First(&ownerUser)
	subID := "sub_mock_test_123"
	org := model.Organization{Name: "Org", Plan: model.PlanPro, StripeSubscriptionID: &subID}
	db.Create(&org)
	makeOrgMember(t, db, org.ID, ownerUser.ID, model.OrgRoleOwner)

	rec := do(e, http.MethodDelete, fmt.Sprintf("/v1/api/orgs/%d/billing/subscription", org.ID), "", ownerToken, "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
}
