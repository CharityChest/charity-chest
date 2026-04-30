package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"charity-chest/internal/middleware"

	"github.com/labstack/echo/v4"
)

// invokeLocale runs the Locale middleware with the given X-Locale header value and
// returns the resolved locale stored in the Echo context.
func invokeLocale(t *testing.T, header string) string {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if header != "" {
		req.Header.Set("X-Locale", header)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var got string
	h := middleware.Locale()(func(c echo.Context) error {
		got, _ = c.Get(middleware.LocaleContextKey).(string)
		return c.String(http.StatusOK, "ok")
	})
	if err := h(c); err != nil {
		t.Fatalf("locale middleware error: %v", err)
	}
	return got
}

func TestLocaleMiddleware_IT(t *testing.T) {
	if got := invokeLocale(t, "it"); got != middleware.LocaleIT {
		t.Errorf("locale = %q, want %q", got, middleware.LocaleIT)
	}
}

func TestLocaleMiddleware_EN(t *testing.T) {
	if got := invokeLocale(t, "en"); got != middleware.LocaleEN {
		t.Errorf("locale = %q, want %q", got, middleware.LocaleEN)
	}
}

func TestLocaleMiddleware_NoHeader_DefaultsToEN(t *testing.T) {
	if got := invokeLocale(t, ""); got != middleware.LocaleEN {
		t.Errorf("locale = %q, want %q", got, middleware.LocaleEN)
	}
}

func TestLocaleMiddleware_UnknownLocale_DefaultsToEN(t *testing.T) {
	for _, hdr := range []string{"fr", "de", "es", "zh", "invalid", "en-US"} {
		if got := invokeLocale(t, hdr); got != middleware.LocaleEN {
			t.Errorf("header %q: locale = %q, want %q", hdr, got, middleware.LocaleEN)
		}
	}
}

func TestLocaleMiddleware_CaseInsensitive_IT(t *testing.T) {
	for _, hdr := range []string{"IT", "It", "iT"} {
		if got := invokeLocale(t, hdr); got != middleware.LocaleIT {
			t.Errorf("header %q: locale = %q, want %q", hdr, got, middleware.LocaleIT)
		}
	}
}

func TestLocaleMiddleware_Whitespace_IT(t *testing.T) {
	if got := invokeLocale(t, "  it  "); got != middleware.LocaleIT {
		t.Errorf("locale = %q, want %q", got, middleware.LocaleIT)
	}
}

func TestLocaleMiddleware_SetsContextKey(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Locale", "it")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var keySet bool
	h := middleware.Locale()(func(c echo.Context) error {
		_, keySet = c.Get(middleware.LocaleContextKey).(string)
		return c.String(http.StatusOK, "ok")
	})
	if err := h(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !keySet {
		t.Error("LocaleContextKey not set in Echo context")
	}
}

func TestLocaleMiddleware_CallsNextHandler(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var called bool
	h := middleware.Locale()(func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})
	if err := h(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("next handler was not called")
	}
}
