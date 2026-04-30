package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"charity-chest/internal/config"
	"charity-chest/internal/handler"
	"charity-chest/internal/cache"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
)

// --- Test helpers ---

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func testCfg() *config.Config {
	return &config.Config{
		JWTSecret:          "test-jwt-secret-for-unit-tests",
		GoogleClientID:     "test-google-client-id",
		GoogleClientSecret: "test-google-client-secret",
		GoogleRedirectURL:  "http://localhost:8080/v1/auth/google/callback",
		Port:               "8080",
	}
}

// newServer wires up a fresh Echo instance with all auth routes for each test.
func newServer(t *testing.T) (*echo.Echo, *handler.AuthHandler, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	h := handler.NewAuthHandler(db, testCfg(), cache.Disabled())

	e := echo.New()
	v1 := e.Group("/v1")
	auth := v1.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	auth.POST("/mfa/verify", h.VerifyMFA)
	auth.GET("/google", h.GoogleLogin)
	auth.GET("/google/callback", h.GoogleCallback)

	return e, h, db
}

// makeMFAPendingToken creates a valid MFA-pending JWT for the given user.
func makeMFAPendingToken(t *testing.T, userID uint, email string) string {
	t.Helper()
	cfg := testCfg()
	pending := true
	claims := middleware.Claims{
		UserID:     userID,
		Email:      email,
		MFAPending: &pending,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatalf("makeMFAPendingToken: %v", err)
	}
	return tok
}

// postJSON fires a POST request with a JSON body through the full Echo pipeline.
func postJSON(e *echo.Echo, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
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

// --- Register ---

func TestRegister_Success(t *testing.T) {
	e, _, _ := newServer(t)
	rec := postJSON(e, "/v1/auth/register", `{"email":"alice@example.com","password":"password123","name":"Alice"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'data' object")
	}
	if data["token"] == nil || data["token"] == "" {
		t.Error("response missing token")
	}
	user, ok := data["user"].(map[string]any)
	if !ok {
		t.Fatal("response missing user object")
	}
	if user["email"] != "alice@example.com" {
		t.Errorf("email = %v", user["email"])
	}
	if user["name"] != "Alice" {
		t.Errorf("name = %v", user["name"])
	}
	// Password hash must never be exposed
	if _, present := user["password_hash"]; present {
		t.Error("password_hash must not appear in response")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	e, _, _ := newServer(t)
	body := `{"email":"dup@example.com","password":"password123","name":"Dup"}`
	postJSON(e, "/v1/auth/register", body)        // first — succeeds
	rec := postJSON(e, "/v1/auth/register", body) // second — duplicate

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestRegister_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing_email", `{"password":"password123","name":"No Email"}`},
		{"missing_password", `{"email":"a@b.com","name":"No Pass"}`},
		{"missing_name", `{"email":"a@b.com","password":"password123"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, _, _ := newServer(t)
			rec := postJSON(e, "/v1/auth/register", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	e, _, _ := newServer(t)
	rec := postJSON(e, "/v1/auth/register", `{"email":"a@b.com","password":"short","name":"User"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- Login ---

func TestLogin_Success(t *testing.T) {
	e, _, _ := newServer(t)
	postJSON(e, "/v1/auth/register", `{"email":"bob@example.com","password":"password123","name":"Bob"}`)
	rec := postJSON(e, "/v1/auth/login", `{"email":"bob@example.com","password":"password123"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'data' object")
	}
	if data["token"] == nil {
		t.Error("response missing token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	e, _, _ := newServer(t)
	postJSON(e, "/v1/auth/register", `{"email":"carol@example.com","password":"correct-password","name":"Carol"}`)
	rec := postJSON(e, "/v1/auth/login", `{"email":"carol@example.com","password":"wrong-password"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	e, _, _ := newServer(t)
	rec := postJSON(e, "/v1/auth/login", `{"email":"ghost@example.com","password":"password123"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_GoogleOnlyAccount(t *testing.T) {
	e, _, db := newServer(t)
	googleID := "google-uid-123"
	db.Create(&model.User{Email: "google@example.com", Name: "Google User", GoogleID: &googleID})

	rec := postJSON(e, "/v1/auth/login", `{"email":"google@example.com","password":"any-password"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- Google OAuth ---

func TestGoogleLogin_Redirects(t *testing.T) {
	e, _, _ := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want 307", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "accounts.google.com") {
		t.Errorf("Location %q does not point to accounts.google.com", loc)
	}
}

func TestGoogleLogin_SetsStateCookie(t *testing.T) {
	e, _, _ := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

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
	e, _, _ := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?state=somestate&code=somecode", nil)
	// No oauth_state cookie — should redirect to webapp with error
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want 307", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc == "" {
		t.Error("Location header missing")
	} else if !strings.Contains(loc, "error=sign_in_failed") {
		t.Errorf("Location %q does not contain error=sign_in_failed", loc)
	}
}

func TestGoogleCallback_StateMismatch(t *testing.T) {
	e, _, _ := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?state=state-from-url&code=somecode", nil)
	req.AddCookie(&http.Cookie{Name: handler.CookieOAuthState, Value: "different-state-in-cookie"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want 307", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc == "" {
		t.Error("Location header missing")
	} else if !strings.Contains(loc, "error=sign_in_failed") {
		t.Errorf("Location %q does not contain error=sign_in_failed", loc)
	}
}

// --- Me ---

// TestMe_ReturnsCurrentUser calls the handler directly so we can inject
// the user_id that the JWT middleware would normally set.
func TestMe_ReturnsCurrentUser(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewAuthHandler(db, testCfg(), cache.Disabled())

	user := &model.User{Email: "me@example.com", Name: "Current User"}
	db.Create(user)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.UserIDContextKey, user.ID) // simulate what JWT middleware injects

	if err := h.Me(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	body := decodeBody(t, rec)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'data' object")
	}
	if data["email"] != "me@example.com" {
		t.Errorf("email = %v", data["email"])
	}
	if data["name"] != "Current User" {
		t.Errorf("name = %v", data["name"])
	}
}

func TestMe_UserNotFound(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewAuthHandler(db, testCfg(), cache.Disabled())

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.UserIDContextKey, uint(99999)) // ID that does not exist

	err := h.Me(c)
	if err == nil {
		t.Fatal("expected an error")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 HTTPError, got %v", err)
	}
}

// --- Login with MFA ---

func TestLogin_MFAEnabled_ReturnsMFAPending(t *testing.T) {
	e, _, db := newServer(t)

	// Register a user first so we have a valid bcrypt hash, then enable MFA directly.
	postJSON(e, "/v1/auth/register", `{"email":"mfa@example.com","password":"password123","name":"MFA User"}`)
	secret := "JBSWY3DPEHPK3PXP"
	db.Model(&model.User{}).Where("email = ?", "mfa@example.com").Updates(map[string]any{
		"mfa_enabled": true,
		"totp_secret": secret,
	})

	rec := postJSON(e, "/v1/auth/login", `{"email":"mfa@example.com","password":"password123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'data' object")
	}
	if data["mfa_required"] != true {
		t.Errorf("mfa_required = %v, want true", data["mfa_required"])
	}
	if data["mfa_token"] == nil || data["mfa_token"] == "" {
		t.Error("mfa_token missing in response")
	}
	if data["token"] != nil {
		t.Error("full token must not be returned when MFA is required")
	}
}

// --- VerifyMFA ---

func TestVerifyMFA_ValidCode_ReturnsToken(t *testing.T) {
	e, _, db := newServer(t)

	// Generate a real TOTP secret and a valid code.
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "test", AccountName: "test@example.com"})
	if err != nil {
		t.Fatalf("generate totp key: %v", err)
	}
	secret := key.Secret()
	user := &model.User{
		Email:      "mfaverify@example.com",
		Name:       "MFA Verify",
		MFAEnabled: true,
		TOTPSecret: &secret,
	}
	db.Create(user)

	mfaToken := makeMFAPendingToken(t, user.ID, user.Email)
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("generate totp code: %v", err)
	}

	body := `{"mfa_token":"` + mfaToken + `","code":"` + code + `"}`
	rec := postJSON(e, "/v1/auth/mfa/verify", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	resp := decodeBody(t, rec)
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'data' object")
	}
	if data["token"] == nil || data["token"] == "" {
		t.Error("token missing in response")
	}
	if data["user"] == nil {
		t.Error("user missing in response")
	}
}

func TestVerifyMFA_InvalidCode_Returns401(t *testing.T) {
	e, _, db := newServer(t)

	secret := "JBSWY3DPEHPK3PXP"
	user := &model.User{
		Email:      "mfabad@example.com",
		Name:       "MFA Bad",
		MFAEnabled: true,
		TOTPSecret: &secret,
	}
	db.Create(user)

	mfaToken := makeMFAPendingToken(t, user.ID, user.Email)
	body := `{"mfa_token":"` + mfaToken + `","code":"000000"}`
	rec := postJSON(e, "/v1/auth/mfa/verify", body)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestVerifyMFA_NonPendingToken_Returns401(t *testing.T) {
	e, _, db := newServer(t)

	user := &model.User{Email: "regular@example.com", Name: "Regular"}
	db.Create(user)

	// Issue a full JWT (not pending).
	cfg := testCfg()
	claims := middleware.Claims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	fullToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))

	body := `{"mfa_token":"` + fullToken + `","code":"123456"}`
	rec := postJSON(e, "/v1/auth/mfa/verify", body)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestVerifyMFA_MissingCode_Returns400(t *testing.T) {
	e, _, db := newServer(t)
	user := &model.User{Email: "u@example.com", Name: "U"}
	db.Create(user)
	mfaToken := makeMFAPendingToken(t, user.ID, user.Email)

	body := `{"mfa_token":"` + mfaToken + `"}`
	rec := postJSON(e, "/v1/auth/mfa/verify", body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestVerifyMFA_ExpiredPendingToken_Returns401(t *testing.T) {
	e, _, db := newServer(t)
	user := &model.User{Email: "expired@example.com", Name: "Expired"}
	db.Create(user)

	// Craft an already-expired MFA-pending JWT.
	cfg := testCfg()
	pending := true
	claims := middleware.Claims{
		UserID:     user.ID,
		Email:      user.Email,
		MFAPending: &pending,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-6 * time.Minute)),
		},
	}
	expiredToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))

	body := `{"mfa_token":"` + expiredToken + `","code":"123456"}`
	rec := postJSON(e, "/v1/auth/mfa/verify", body)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
