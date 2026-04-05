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

// Locale reads the Accept-Language request header, resolves it to a supported
// locale ("en" or "it"), and stores the result in the Echo context under
// LocaleContextKey. Register it as a global middleware so it runs before any
// group-level middleware (e.g. JWT).
func Locale() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(LocaleContextKey, detectLocale(c.Request().Header.Get("Accept-Language")))
			return next(c)
		}
	}
}

// detectLocale parses the first language tag from an Accept-Language header
// value. It supports only "en" and "it"; everything else maps to "en".
//
// Examples:
//
//	"it,en;q=0.9"  → "it"
//	"it-IT"        → "it"
//	"en-GB"        → "en"
//	""             → "en"
//	"fr"           → "en"
func detectLocale(header string) string {
	first, _, _ := strings.Cut(header, ",")
	tag, _, _ := strings.Cut(first, ";")
	tag = strings.TrimSpace(strings.ToLower(tag))
	if tag == LocaleIT || strings.HasPrefix(tag, LocaleIT+"-") {
		return LocaleIT
	}
	return LocaleEN
}
