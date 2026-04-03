package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charity-chest/internal/config"
	"charity-chest/internal/handler"
	"charity-chest/internal/model"
	"charity-chest/internal/routes"

	"github.com/glebarez/sqlite"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// --- Setup helpers ---

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
		JWTSecret:          "e2e-test-jwt-secret",
		GoogleClientID:     "test-google-client-id",
		GoogleClientSecret: "test-google-client-secret",
		GoogleRedirectURL:  "http://localhost:8080/v1/auth/google/callback",
		Port:               "8080",
	}
}

// newServer wires the full Echo instance — all routes plus middleware — and
// returns it alongside the DB so tests can seed state when needed.
func newServer(t *testing.T) (*echo.Echo, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	cfg := testCfg()
	h := handler.NewAuthHandler(db, cfg)

	e := echo.New()
	e.HideBanner = true

	routes.RegisterHealth(e)

	v1 := e.Group("/v1")
	routes.RegisterAuth(v1, h)
	routes.RegisterAPI(v1, h, cfg.JWTSecret)

	return e, db
}

// do fires an HTTP request through the full Echo pipeline.
func do(e *echo.Echo, method, path, body, bearerToken string) *httptest.ResponseRecorder {
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	if bearerToken != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+bearerToken)
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

// registerUser registers a user and returns the JWT from the response.
func registerUser(t *testing.T, e *echo.Echo, email, password, name string) string {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q,"name":%q}`, email, password, name)
	rec := do(e, http.MethodPost, "/v1/auth/register", body, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("registerUser: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	token, ok := m["token"].(string)
	if !ok || token == "" {
		t.Fatal("registerUser: missing token in response")
	}
	return token
}

// loginUser logs in and returns the JWT.
func loginUser(t *testing.T, e *echo.Echo, email, password string) string {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	rec := do(e, http.MethodPost, "/v1/auth/login", body, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("loginUser: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	token, ok := m["token"].(string)
	if !ok || token == "" {
		t.Fatal("loginUser: missing token in response")
	}
	return token
}

// --- Health ---

func TestHealth(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/health", "", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decodeBody(t, rec)
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
}

// --- Register ---

func TestRegister_Success(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodPost, "/v1/auth/register",
		`{"email":"alice@example.com","password":"password123","name":"Alice"}`, "")

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
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
	do(e, http.MethodPost, "/v1/auth/register", body, "")        // first — OK
	rec := do(e, http.MethodPost, "/v1/auth/register", body, "") // duplicate

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
			rec := do(e, http.MethodPost, "/v1/auth/register", tc.body, "")
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodPost, "/v1/auth/register",
		`{"email":"a@b.com","password":"short","name":"User"}`, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- Login ---

func TestLogin_Success(t *testing.T) {
	e, _ := newServer(t)
	registerUser(t, e, "bob@example.com", "password123", "Bob")

	rec := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"bob@example.com","password":"password123"}`, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
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
		`{"email":"carol@example.com","password":"wrong-password"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"ghost@example.com","password":"password123"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_WrongPassword_SameStatusAsUnknownEmail(t *testing.T) {
	// Both cases must return 401 — no user enumeration.
	e, _ := newServer(t)
	registerUser(t, e, "dave@example.com", "correct-password", "Dave")

	recWrongPw := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"dave@example.com","password":"wrong"}`, "")
	recUnknown := do(e, http.MethodPost, "/v1/auth/login",
		`{"email":"nobody@example.com","password":"password123"}`, "")

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
		`{"email":"google@example.com","password":"any-password"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- Google OAuth ---

func TestGoogleLogin_Redirects(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/v1/auth/google", "", "")

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
	rec := do(e, http.MethodGet, "/v1/auth/google", "", "")

	var stateCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "oauth_state" {
			stateCookie = c
			break
		}
	}
	if stateCookie == nil {
		t.Fatal("oauth_state cookie was not set")
	}
	if stateCookie.Value == "" {
		t.Error("oauth_state cookie value is empty")
	}
	if !stateCookie.HttpOnly {
		t.Error("oauth_state cookie must be HttpOnly")
	}
}

func TestGoogleCallback_MissingStateCookie(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/v1/auth/google/callback?state=somestate&code=somecode", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestGoogleCallback_StateMismatch(t *testing.T) {
	e, _ := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?state=url-state&code=somecode", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "different-cookie-state"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestGoogleCallback_MissingCode(t *testing.T) {
	e, _ := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?state=matching-state", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "matching-state"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- Me ---

func TestMe_Success(t *testing.T) {
	e, _ := newServer(t)
	token := registerUser(t, e, "me@example.com", "password123", "Me User")

	rec := do(e, http.MethodGet, "/v1/api/me", "", token)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
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
	rec := do(e, http.MethodGet, "/v1/api/me", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMe_InvalidToken(t *testing.T) {
	e, _ := newServer(t)
	rec := do(e, http.MethodGet, "/v1/api/me", "", "this-is-not-a-valid-jwt")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMe_TokenSignedWithWrongSecret(t *testing.T) {
	// Register on a server with a different JWT secret — its token must be rejected.
	cfg := testCfg()
	cfg.JWTSecret = "attacker-secret"
	db := newTestDB(t)
	attackerHandler := handler.NewAuthHandler(db, cfg)
	attackerEcho := echo.New()
	v1 := attackerEcho.Group("/v1")
	routes.RegisterAuth(v1, attackerHandler)

	rec := do(attackerEcho, http.MethodPost, "/v1/auth/register",
		`{"email":"attacker@example.com","password":"password123","name":"Attacker"}`, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("attacker register: %d", rec.Code)
	}
	m := decodeBody(t, rec)
	foreignToken := m["token"].(string)

	// Present that token to our legitimate server.
	e, _ := newServer(t)
	result := do(e, http.MethodGet, "/v1/api/me", "", foreignToken)
	if result.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", result.Code)
	}
}

func TestMe_TokenRefreshedAfterReLogin(t *testing.T) {
	e, _ := newServer(t)
	registerUser(t, e, "eve@example.com", "password123", "Eve")
	token := loginUser(t, e, "eve@example.com", "password123")

	rec := do(e, http.MethodGet, "/v1/api/me", "", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
