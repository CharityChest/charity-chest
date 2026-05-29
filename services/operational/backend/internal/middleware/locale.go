// Package middleware holds Echo middleware functions used by the operational
// backend. Mirrors services/admin/backend/internal/middleware where the
// behaviour is identical so error UX stays consistent.
package middleware

import (
	"strings"

	"github.com/labstack/echo/v4"
)

// LocaleContextKey is the Echo context key under which the resolved locale is
// stored by the Locale middleware. Mirrors admin's middleware.
const (
	LocaleContextKey = "locale"
	LocaleEN         = "en"
	LocaleIT         = "it"
)

// Locale reads the X-Locale request header, resolves it to a supported locale
// ("en" or "it"), and stores the result under LocaleContextKey. Register it as
// a global middleware so it runs before any group-level middleware.
func Locale() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(LocaleContextKey, detectLocale(c.Request().Header.Get("X-Locale")))
			return next(c)
		}
	}
}

func detectLocale(header string) string {
	if strings.ToLower(strings.TrimSpace(header)) == LocaleIT {
		return LocaleIT
	}
	return LocaleEN
}

// LocaleFrom extracts the resolved locale from the Echo context, defaulting to
// English when the middleware was skipped (e.g. in tests bypassing the stack).
func LocaleFrom(c echo.Context) string {
	if l, ok := c.Get(LocaleContextKey).(string); ok && l != "" {
		return l
	}
	return LocaleEN
}
