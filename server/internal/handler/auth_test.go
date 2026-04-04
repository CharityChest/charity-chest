package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charity-chest/internal/config"
	"charity-chest/internal/handler"
	"charity-chest/internal/model"

	"github.com/glebarez/sqlite"
	"github.com/labstack/echo/v4"
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
	h := handler.NewAuthHandler(db, testCfg())

	e := echo.New()
	v1 := e.Group("/v1")
	auth := v1.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	auth.GET("/google", h.GoogleLogin)
	auth.GET("/google/callback", h.GoogleCallback)

	return e, h, db
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
	if body["token"] == nil || body["token"] == "" {
		t.Error("response missing token")
	}
	user, ok := body["user"].(map[string]any)
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
	postJSON(e, "/v1/auth/register", body) // first — succeeds
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
	if body["token"] == nil {
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
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "different-state-in-cookie"})
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
	h := handler.NewAuthHandler(db, testCfg())

	user := &model.User{Email: "me@example.com", Name: "Current User"}
	db.Create(user)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", user.ID) // simulate what JWT middleware injects

	if err := h.Me(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	body := decodeBody(t, rec)
	if body["email"] != "me@example.com" {
		t.Errorf("email = %v", body["email"])
	}
	if body["name"] != "Current User" {
		t.Errorf("name = %v", body["name"])
	}
}

func TestMe_UserNotFound(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewAuthHandler(db, testCfg())

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", uint(99999)) // ID that does not exist

	err := h.Me(c)
	if err == nil {
		t.Fatal("expected an error")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 HTTPError, got %v", err)
	}
}