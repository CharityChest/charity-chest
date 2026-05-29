package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"charity-chest/services/operational/backend/internal/adminclient"
	"charity-chest/services/operational/backend/internal/handler"
	mw "charity-chest/services/operational/backend/internal/middleware"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// invokeMe runs MeHandler.Me with an optional user UUID stashed in the context
// (as the JWT middleware would) and returns the HTTP status code.
func invokeMe(t *testing.T, admin handler.AdminAPI, setUUID *uuid.UUID) int {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if setUUID != nil {
		c.Set(mw.UserUUIDContextKey, *setUUID)
	}
	err := handler.NewMeHandler(admin).Me(c)
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

func TestMe_MissingUUIDContext_401(t *testing.T) {
	if code := invokeMe(t, &fakeAdmin{}, nil); code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", code)
	}
}

func TestMe_WrongTypeInContext_401(t *testing.T) {
	// A non-uuid value under the key must still be rejected as unauthorized.
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	c.Set(mw.UserUUIDContextKey, "not-a-uuid")
	err := handler.NewMeHandler(&fakeAdmin{}).Me(c)
	var he *echo.HTTPError
	if !errors.As(err, &he) || he.Code != http.StatusUnauthorized {
		t.Fatalf("err = %v, want 401 HTTPError", err)
	}
}

func TestMe_OK(t *testing.T) {
	id := uuid.New()
	admin := &fakeAdmin{getFn: func(_ context.Context, got uuid.UUID) (*adminclient.User, error) {
		if got != id {
			t.Errorf("uuid = %v, want %v", got, id)
		}
		return &adminclient.User{UUID: id, Email: "me@x.io"}, nil
	}}
	if code := invokeMe(t, admin, &id); code != http.StatusOK {
		t.Fatalf("code = %d, want 200", code)
	}
}

func TestMe_BadResponse_MapsTo502(t *testing.T) {
	id := uuid.New()
	admin := &fakeAdmin{getFn: func(context.Context, uuid.UUID) (*adminclient.User, error) {
		return nil, adminclient.ErrBadResponse
	}}
	if code := invokeMe(t, admin, &id); code != http.StatusBadGateway {
		t.Fatalf("code = %d, want 502", code)
	}
}
