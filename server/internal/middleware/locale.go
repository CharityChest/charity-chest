package middleware

import (
	"strings"

	"github.com/labstack/echo/v4"
)

// LocaleContextKey is the Echo context key under which the resolved locale is stored.
const (
	LocaleContextKey = "locale"
	LocaleEN         = "en"
	LocaleIT         = "it"
)

// Locale reads the X-Locale request header, resolves it to a supported
// locale ("en" or "it"), and stores the result in the Echo context under
// LocaleContextKey. Register it as a global middleware so it runs before any
// group-level middleware (e.g. JWT).
func Locale() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(LocaleContextKey, detectLocale(c.Request().Header.Get("X-Locale")))
			return next(c)
		}
	}
}

// detectLocale maps an X-Locale header value to a supported locale.
// It supports only "en" and "it"; everything else maps to "en".
//
// Examples:
//
//	"it" → "it"
//	"en" → "en"
//	""   → "en"
//	"fr" → "en"
func detectLocale(header string) string {
	if strings.ToLower(strings.TrimSpace(header)) == LocaleIT {
		return LocaleIT
	}
	return LocaleEN
}
