package middleware

import (
	"net/http"
	"strings"

	"charity-chest/services/operational/backend/internal/i18n"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Context keys injected by the JWT middleware. Unlike admin's middleware, the
// operational backend does NOT resolve the UUID to an internal id — it has no
// users table. Handlers that need user data call adminclient.GetUser(ctx, uuid).
const (
	UserUUIDContextKey = "user_uuid"
	EmailContextKey    = "email"
)

// Claims holds the operational backend's JWT payload. Intentionally smaller
// than admin's Claims: no role, no MFA-pending flag — operational issues
// regular 24h tokens after admin has confirmed the credentials.
type Claims struct {
	UserUUID uuid.UUID `json:"user_uuid"`
	Email    string    `json:"email"`
	jwt.RegisteredClaims
}

// JWT returns an Echo middleware that validates Bearer tokens signed with the
// operational backend's secret. It rejects missing/expired/invalid tokens with
// localised 401s and stashes the user's public UUID + email under the context
// keys above.
func JWT(secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			loc := LocaleFrom(c)

			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(loc, i18n.KeyMissingAuthHeader))
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, echo.NewHTTPError(http.StatusUnauthorized, i18n.T(loc, i18n.KeyUnexpectedSigning))
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(loc, i18n.KeyInvalidToken))
			}

			claims, ok := token.Claims.(*Claims)
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(loc, i18n.KeyInvalidClaims))
			}
			if claims.UserUUID == uuid.Nil {
				return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(loc, i18n.KeyInvalidClaims))
			}

			c.Set(UserUUIDContextKey, claims.UserUUID)
			c.Set(EmailContextKey, claims.Email)
			return next(c)
		}
	}
}
