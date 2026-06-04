package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"charity-chest/services/admin/backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// invokeServiceKey runs the ServiceKey middleware with the expected secret and
// the given header value and returns the resulting HTTPError (or nil) plus
// whether the inner handler was reached.
func invokeServiceKey(t *testing.T, expected, header string) (*echo.HTTPError, bool) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if header != "" {
		req.Header.Set(middleware.ServiceKeyHeader, header)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var reached bool
	h := middleware.ServiceKey(expected)(func(c echo.Context) error {
		reached = true
		return c.String(http.StatusOK, "ok")
	})
	err := h(c)
	if err == nil {
		return nil, reached
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T (%v)", err, err)
	}
	return he, reached
}

func TestServiceKey_OK(t *testing.T) {
	he, reached := invokeServiceKey(t, "secret-abc", "secret-abc")
	if he != nil {
		t.Fatalf("unexpected error: %+v", he)
	}
	if !reached {
		t.Error("inner handler was not reached on a valid key")
	}
}

func TestServiceKey_MissingHeader_401(t *testing.T) {
	he, reached := invokeServiceKey(t, "secret-abc", "")
	if he == nil {
		t.Fatal("expected HTTPError, got nil")
	}
	if he.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, want 401", he.Code)
	}
	if reached {
		t.Error("inner handler must not be reached on missing header")
	}
}

func TestServiceKey_WrongHeader_403(t *testing.T) {
	he, reached := invokeServiceKey(t, "secret-abc", "nope")
	if he == nil {
		t.Fatal("expected HTTPError, got nil")
	}
	if he.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", he.Code)
	}
	if reached {
		t.Error("inner handler must not be reached on wrong header")
	}
}

func TestServiceKey_EmptyExpected_BlocksAllHeaders(t *testing.T) {
	// Empty expected value must never silently admit a request. We rely on
	// main.go not mounting the group at all when the expected value is empty,
	// but the middleware must still refuse to accept any provided header in
	// that pathological case.
	he, reached := invokeServiceKey(t, "", "anything")
	if he == nil {
		t.Fatal("expected HTTPError on empty expected, got nil")
	}
	if he.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", he.Code)
	}
	if reached {
		t.Error("inner handler must not be reached when expected is empty")
	}
}
