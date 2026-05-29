package v1_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"charity-chest/services/operational/backend/internal/adminclient"
	"charity-chest/services/operational/backend/internal/config"
	"charity-chest/services/operational/backend/internal/handler"
	mw "charity-chest/services/operational/backend/internal/middleware"
	routesv1 "charity-chest/services/operational/backend/internal/routes/v1"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const (
	testJWTSecret = "op-routes-test-secret"
	testAudience  = "test-audience"
)

// fakeAdmin implements handler.AdminAPI in-memory.
type fakeAdmin struct {
	loginFn  func(ctx context.Context, email, password string) (*adminclient.User, error)
	googleFn func(ctx context.Context, sub, email, name string) (*adminclient.User, error)
	getFn    func(ctx context.Context, userUUID uuid.UUID) (*adminclient.User, error)
}

func (f *fakeAdmin) Login(ctx context.Context, email, password string) (*adminclient.User, error) {
	return f.loginFn(ctx, email, password)
}
func (f *fakeAdmin) GoogleAuth(ctx context.Context, sub, email, name string) (*adminclient.User, error) {
	return f.googleFn(ctx, sub, email, name)
}
func (f *fakeAdmin) GetUser(ctx context.Context, userUUID uuid.UUID) (*adminclient.User, error) {
	return f.getFn(ctx, userUUID)
}

// fakeGoogle implements handler.GoogleValidator.
type fakeGoogle struct {
	payload *handler.GooglePayload
	err     error
}

func (f *fakeGoogle) Validate(ctx context.Context, idToken, audience string) (*handler.GooglePayload, error) {
	if f.err != nil {
		return nil, f.err
	}
	if audience != testAudience {
		return nil, errors.New("audience mismatch")
	}
	return f.payload, nil
}

func newServer(t *testing.T, admin handler.AdminAPI, google handler.GoogleValidator) *echo.Echo {
	t.Helper()
	cfg := &config.Config{
		JWTSecret:      testJWTSecret,
		GoogleAudience: testAudience,
		Port:           "8081",
	}
	authH := handler.NewAuthHandler(cfg, admin, google)
	meH := handler.NewMeHandler(admin)

	e := echo.New()
	e.Use(mw.Locale())
	routesv1.RegisterHealth(e)
	v1 := e.Group("/v1")
	routesv1.RegisterAuth(v1, authH)
	routesv1.RegisterAPI(v1, meH, cfg.JWTSecret)
	return e
}

func makeOpToken(t *testing.T, userUUID uuid.UUID, email string) string {
	t.Helper()
	claims := mw.Claims{
		UserUUID: userUUID,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return tok
}

func postJSON(e *echo.Echo, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body.String())
	}
	return m
}

// --- /health ---

func TestHealth(t *testing.T) {
	e := newServer(t, &fakeAdmin{}, &fakeGoogle{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
}

// --- /v1/auth/login ---

func TestLogin_OK(t *testing.T) {
	want := &adminclient.User{UUID: uuid.New(), Email: "alice@example.com", Name: "Alice"}
	admin := &fakeAdmin{loginFn: func(_ context.Context, email, password string) (*adminclient.User, error) {
		if email != "alice@example.com" || password != "pw" {
			t.Errorf("admin Login got (%q,%q)", email, password)
		}
		return want, nil
	}}
	e := newServer(t, admin, &fakeGoogle{})

	rec := postJSON(e, "/v1/auth/login", `{"email":"alice@example.com","password":"pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d; body=%s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)["data"].(map[string]any)
	if body["token"].(string) == "" {
		t.Error("token empty")
	}
	user := body["user"].(map[string]any)
	if user["email"].(string) != "alice@example.com" {
		t.Errorf("user.email = %v", user["email"])
	}
}

func TestLogin_InvalidCredentials_401(t *testing.T) {
	admin := &fakeAdmin{loginFn: func(context.Context, string, string) (*adminclient.User, error) {
		return nil, adminclient.ErrInvalidCredentials
	}}
	e := newServer(t, admin, &fakeGoogle{})
	rec := postJSON(e, "/v1/auth/login", `{"email":"x","password":"y"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestLogin_MFAEnabled_409(t *testing.T) {
	admin := &fakeAdmin{loginFn: func(context.Context, string, string) (*adminclient.User, error) {
		return &adminclient.User{UUID: uuid.New(), Email: "x@x", Name: "X", MFAEnabled: true}, nil
	}}
	e := newServer(t, admin, &fakeGoogle{})
	rec := postJSON(e, "/v1/auth/login", `{"email":"x","password":"y"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestLogin_AdminDown_502(t *testing.T) {
	admin := &fakeAdmin{loginFn: func(context.Context, string, string) (*adminclient.User, error) {
		return nil, adminclient.ErrAdminUnavailable
	}}
	e := newServer(t, admin, &fakeGoogle{})
	rec := postJSON(e, "/v1/auth/login", `{"email":"x","password":"y"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestLogin_MissingFields_400(t *testing.T) {
	e := newServer(t, &fakeAdmin{}, &fakeGoogle{})
	rec := postJSON(e, "/v1/auth/login", `{"email":"","password":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d", rec.Code)
	}
}

// --- /v1/auth/google ---

func TestGoogleLogin_OK(t *testing.T) {
	google := &fakeGoogle{payload: &handler.GooglePayload{Subject: "sub-1", Email: "g@example.com", Name: "G"}}
	want := &adminclient.User{UUID: uuid.New(), Email: "g@example.com", Name: "G"}
	admin := &fakeAdmin{googleFn: func(_ context.Context, sub, email, name string) (*adminclient.User, error) {
		if sub != "sub-1" {
			t.Errorf("sub = %q", sub)
		}
		return want, nil
	}}
	e := newServer(t, admin, google)

	rec := postJSON(e, "/v1/auth/google", `{"id_token":"some-valid-token"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d; body=%s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)["data"].(map[string]any)
	if body["token"].(string) == "" {
		t.Error("token empty")
	}
}

func TestGoogleLogin_BadIDToken_401(t *testing.T) {
	google := &fakeGoogle{err: errors.New("bad token")}
	e := newServer(t, &fakeAdmin{}, google)
	rec := postJSON(e, "/v1/auth/google", `{"id_token":"bad"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestGoogleLogin_MissingFields_400(t *testing.T) {
	e := newServer(t, &fakeAdmin{}, &fakeGoogle{})
	rec := postJSON(e, "/v1/auth/google", `{"id_token":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d", rec.Code)
	}
}

// --- /v1/api/me ---

func TestMe_OK(t *testing.T) {
	id := uuid.New()
	want := &adminclient.User{UUID: id, Email: "me@example.com", Name: "Me"}
	admin := &fakeAdmin{getFn: func(_ context.Context, gotID uuid.UUID) (*adminclient.User, error) {
		if gotID != id {
			t.Errorf("uuid = %v, want %v", gotID, id)
		}
		return want, nil
	}}
	e := newServer(t, admin, &fakeGoogle{})

	req := httptest.NewRequest(http.MethodGet, "/v1/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+makeOpToken(t, id, "me@example.com"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d; body=%s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)["data"].(map[string]any)
	if body["email"].(string) != "me@example.com" {
		t.Errorf("email = %v", body["email"])
	}
}

func TestMe_NoToken_401(t *testing.T) {
	e := newServer(t, &fakeAdmin{}, &fakeGoogle{})
	req := httptest.NewRequest(http.MethodGet, "/v1/api/me", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestMe_AdminUserNotFound_404(t *testing.T) {
	id := uuid.New()
	admin := &fakeAdmin{getFn: func(context.Context, uuid.UUID) (*adminclient.User, error) {
		return nil, adminclient.ErrUserNotFound
	}}
	e := newServer(t, admin, &fakeGoogle{})
	req := httptest.NewRequest(http.MethodGet, "/v1/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+makeOpToken(t, id, "x@x"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestMe_AdminUnavailable_502(t *testing.T) {
	id := uuid.New()
	admin := &fakeAdmin{getFn: func(context.Context, uuid.UUID) (*adminclient.User, error) {
		return nil, adminclient.ErrAdminUnavailable
	}}
	e := newServer(t, admin, &fakeGoogle{})
	req := httptest.NewRequest(http.MethodGet, "/v1/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+makeOpToken(t, id, "x@x"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code = %d", rec.Code)
	}
}
