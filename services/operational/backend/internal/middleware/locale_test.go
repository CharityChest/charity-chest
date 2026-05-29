package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	mw "charity-chest/services/operational/backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// runLocale runs the Locale middleware with the given X-Locale header and
// returns the locale the downstream handler observed via LocaleFrom.
func runLocale(t *testing.T, header string) string {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if header != "" {
		req.Header.Set("X-Locale", header)
	}
	c := e.NewContext(req, httptest.NewRecorder())

	var got string
	h := mw.Locale()(func(c echo.Context) error {
		got = mw.LocaleFrom(c)
		return c.NoContent(http.StatusOK)
	})
	if err := h(c); err != nil {
		t.Fatalf("handler: %v", err)
	}
	return got
}

func TestLocale_DetectsItalian(t *testing.T) {
	for _, h := range []string{"it", "IT", "  it  "} {
		if got := runLocale(t, h); got != mw.LocaleIT {
			t.Errorf("header %q → %q, want it", h, got)
		}
	}
}

func TestLocale_DefaultsToEnglish(t *testing.T) {
	for _, h := range []string{"", "en", "fr", "garbage"} {
		if got := runLocale(t, h); got != mw.LocaleEN {
			t.Errorf("header %q → %q, want en", h, got)
		}
	}
}

// LocaleFrom must default to English when the middleware never ran (e.g. a
// handler tested in isolation without the global stack).
func TestLocaleFrom_DefaultsWhenUnset(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	if got := mw.LocaleFrom(c); got != mw.LocaleEN {
		t.Errorf("LocaleFrom with no value = %q, want en", got)
	}
}
