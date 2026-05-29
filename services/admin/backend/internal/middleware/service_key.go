package middleware

import (
	"crypto/subtle"
	"net/http"

	"charity-chest/services/admin/backend/internal/i18n"

	"github.com/labstack/echo/v4"
)

// ServiceKeyHeader is the HTTP header carrying the shared service-to-service
// secret. It is compared in constant time against the configured key by the
// ServiceKey middleware.
const ServiceKeyHeader = "X-Service-Key"

// ServiceKey returns middleware that admits a request only when the
// X-Service-Key header matches the expected value via a constant-time
// comparison. Missing headers return 401 and mismatches return 403; both use
// localised messages so callers see consistent error envelopes.
//
// The expected value must be non-empty — main.go must avoid mounting the
// /v1/internal/* group when SERVICE_API_KEY is unset, so this middleware never
// sees an empty expected value at request time.
func ServiceKey(expected string) echo.MiddlewareFunc {
	expectedBytes := []byte(expected)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			loc := localeFrom(c)

			provided := c.Request().Header.Get(ServiceKeyHeader)
			if provided == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(loc, i18n.KeyMissingServiceKey))
			}
			if subtle.ConstantTimeCompare([]byte(provided), expectedBytes) != 1 {
				return echo.NewHTTPError(http.StatusForbidden, i18n.T(loc, i18n.KeyInvalidServiceKey))
			}
			return next(c)
		}
	}
}
