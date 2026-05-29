package handler_test

import (
	"context"
	"net/http"
	"testing"

	"charity-chest/services/operational/backend/internal/adminclient"
	"charity-chest/services/operational/backend/internal/handler"

	"github.com/google/uuid"
)

// --- Login: remaining branches ---

func TestLogin_MissingFields_400(t *testing.T) {
	h := handler.NewAuthHandler(testCfg(), &fakeAdmin{}, &fakeGoogle{})
	if code := invokeAuth(t, h.Login, `{"email":"","password":""}`); code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", code)
	}
}

func TestLogin_InvalidCredentials_401(t *testing.T) {
	admin := &fakeAdmin{loginFn: func(context.Context, string, string) (*adminclient.User, error) {
		return nil, adminclient.ErrInvalidCredentials
	}}
	h := handler.NewAuthHandler(testCfg(), admin, &fakeGoogle{})
	if code := invokeAuth(t, h.Login, `{"email":"a@b.c","password":"pw"}`); code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", code)
	}
}

func TestLogin_MFAEnabled_409(t *testing.T) {
	admin := &fakeAdmin{loginFn: func(context.Context, string, string) (*adminclient.User, error) {
		return &adminclient.User{UUID: uuid.New(), Email: "a@b.c", MFAEnabled: true}, nil
	}}
	h := handler.NewAuthHandler(testCfg(), admin, &fakeGoogle{})
	if code := invokeAuth(t, h.Login, `{"email":"a@b.c","password":"pw"}`); code != http.StatusConflict {
		t.Fatalf("code = %d, want 409", code)
	}
}

// TestLogin_Success_200 also exercises generateJWT, the real token signer
// wired in by NewAuthHandler.
func TestLogin_Success_200(t *testing.T) {
	admin := &fakeAdmin{loginFn: func(context.Context, string, string) (*adminclient.User, error) {
		return &adminclient.User{UUID: uuid.New(), Email: "a@b.c"}, nil
	}}
	h := handler.NewAuthHandler(testCfg(), admin, &fakeGoogle{})
	if code := invokeAuth(t, h.Login, `{"email":"a@b.c","password":"pw"}`); code != http.StatusOK {
		t.Fatalf("code = %d, want 200", code)
	}
}

// --- GoogleLogin: remaining branches ---

func TestGoogleLogin_MissingIDToken_400(t *testing.T) {
	h := handler.NewAuthHandler(testCfg(), &fakeAdmin{}, &fakeGoogle{})
	if code := invokeAuth(t, h.GoogleLogin, `{"id_token":""}`); code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", code)
	}
}

func TestGoogleLogin_ValidateError_401(t *testing.T) {
	google := &fakeGoogle{err: context.DeadlineExceeded}
	h := handler.NewAuthHandler(testCfg(), &fakeAdmin{}, google)
	if code := invokeAuth(t, h.GoogleLogin, `{"id_token":"tok"}`); code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", code)
	}
}

// TestGoogleLogin_Success_200 also exercises generateJWT.
func TestGoogleLogin_Success_200(t *testing.T) {
	google := &fakeGoogle{payload: &handler.GooglePayload{Subject: "s", Email: "g@x.io", Name: "G"}}
	admin := &fakeAdmin{googleFn: func(context.Context, string, string, string) (*adminclient.User, error) {
		return &adminclient.User{UUID: uuid.New(), Email: "g@x.io"}, nil
	}}
	h := handler.NewAuthHandler(testCfg(), admin, google)
	if code := invokeAuth(t, h.GoogleLogin, `{"id_token":"tok"}`); code != http.StatusOK {
		t.Fatalf("code = %d, want 200", code)
	}
}

// --- Me: remaining error branches ---

func TestMe_AdminUnavailable_502(t *testing.T) {
	id := uuid.New()
	admin := &fakeAdmin{getFn: func(context.Context, uuid.UUID) (*adminclient.User, error) {
		return nil, adminclient.ErrAdminUnavailable
	}}
	if code := invokeMe(t, admin, &id); code != http.StatusBadGateway {
		t.Fatalf("code = %d, want 502", code)
	}
}
