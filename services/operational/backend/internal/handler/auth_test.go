package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charity-chest/services/operational/backend/internal/adminclient"
	"charity-chest/services/operational/backend/internal/config"
	"charity-chest/services/operational/backend/internal/handler"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// --- fakes ---

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

type fakeGoogle struct {
	payload *handler.GooglePayload
	err     error
}

func (f *fakeGoogle) Validate(_ context.Context, _, _ string) (*handler.GooglePayload, error) {
	return f.payload, f.err
}

func testCfg() *config.Config {
	return &config.Config{JWTSecret: "handler-test-secret", GoogleAudience: "aud"}
}

// invokeAuth builds an echo context for a POST with the given JSON body,
// runs the handler, and returns the resulting HTTP status code.
func invokeAuth(t *testing.T, h func(echo.Context) error, body string) int {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	err := h(c)
	if err == nil {
		return rec.Code
	}
	var he *echo.HTTPError
	if errors.As(err, &he) {
		return he.Code
	}
	t.Fatalf("unexpected error: %T %v", err, err)
	return 0
}

// --- Login ---

func TestLogin_MalformedBody_400(t *testing.T) {
	h := handler.NewAuthHandler(testCfg(), &fakeAdmin{}, &fakeGoogle{})
	if code := invokeAuth(t, h.Login, `{not json`); code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", code)
	}
}

func TestLogin_UserNotFound_404(t *testing.T) {
	admin := &fakeAdmin{loginFn: func(context.Context, string, string) (*adminclient.User, error) {
		return nil, adminclient.ErrUserNotFound
	}}
	h := handler.NewAuthHandler(testCfg(), admin, &fakeGoogle{})
	if code := invokeAuth(t, h.Login, `{"email":"a@b.c","password":"pw"}`); code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", code)
	}
}

func TestLogin_BadResponse_MapsTo502(t *testing.T) {
	admin := &fakeAdmin{loginFn: func(context.Context, string, string) (*adminclient.User, error) {
		return nil, adminclient.ErrBadResponse
	}}
	h := handler.NewAuthHandler(testCfg(), admin, &fakeGoogle{})
	if code := invokeAuth(t, h.Login, `{"email":"a@b.c","password":"pw"}`); code != http.StatusBadGateway {
		t.Fatalf("code = %d, want 502", code)
	}
}

// --- GoogleLogin ---

func TestGoogleLogin_MalformedBody_400(t *testing.T) {
	h := handler.NewAuthHandler(testCfg(), &fakeAdmin{}, &fakeGoogle{})
	if code := invokeAuth(t, h.GoogleLogin, `{not json`); code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", code)
	}
}

func TestGoogleLogin_EmptyClaims_401(t *testing.T) {
	// Validator succeeds but returns a payload with no subject/email.
	google := &fakeGoogle{payload: &handler.GooglePayload{Subject: "", Email: ""}}
	h := handler.NewAuthHandler(testCfg(), &fakeAdmin{}, google)
	if code := invokeAuth(t, h.GoogleLogin, `{"id_token":"tok"}`); code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", code)
	}
}

func TestGoogleLogin_MFAEnabled_409(t *testing.T) {
	google := &fakeGoogle{payload: &handler.GooglePayload{Subject: "s", Email: "g@x.io", Name: "G"}}
	admin := &fakeAdmin{googleFn: func(context.Context, string, string, string) (*adminclient.User, error) {
		return &adminclient.User{UUID: uuid.New(), Email: "g@x.io", MFAEnabled: true}, nil
	}}
	h := handler.NewAuthHandler(testCfg(), admin, google)
	if code := invokeAuth(t, h.GoogleLogin, `{"id_token":"tok"}`); code != http.StatusConflict {
		t.Fatalf("code = %d, want 409", code)
	}
}

func TestGoogleLogin_AdminError_502(t *testing.T) {
	google := &fakeGoogle{payload: &handler.GooglePayload{Subject: "s", Email: "g@x.io", Name: "G"}}
	admin := &fakeAdmin{googleFn: func(context.Context, string, string, string) (*adminclient.User, error) {
		return nil, adminclient.ErrAdminUnavailable
	}}
	h := handler.NewAuthHandler(testCfg(), admin, google)
	if code := invokeAuth(t, h.GoogleLogin, `{"id_token":"tok"}`); code != http.StatusBadGateway {
		t.Fatalf("code = %d, want 502", code)
	}
}
