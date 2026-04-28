package middleware

import (
	"net/http"
	"strings"

	"charity-chest/internal/i18n"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// Context keys injected by the JWT middleware.
const (
	UserIDContextKey = "user_id"
	EmailContextKey  = "email"
	RoleContextKey   = "role"
)

// Claims holds the JWT payload stored in each token.
// MFAPending, when true, marks a short-lived token issued mid-login for MFA verification;
// the JWT middleware rejects these so they cannot be used as full auth tokens.
type Claims struct {
	UserID     uint    `json:"user_id"`
	Email      string  `json:"email"`
	Role       *string `json:"role,omitempty"`
	MFAPending *bool   `json:"mfa_pending,omitempty"`
	jwt.RegisteredClaims
}

// JWT returns an Echo middleware that validates Bearer tokens and injects
// "user_id" and "email" into the request context.
func JWT(secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			loc, _ := c.Get(LocaleContextKey).(string)
			if loc == "" {
				loc = LocaleEN
			}

			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(loc, i18n.KeyMissingAuthHeader))
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
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

			// Reject MFA-pending tokens — they may not be used as full auth tokens.
			if claims.MFAPending != nil && *claims.MFAPending {
				return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(loc, i18n.KeyInvalidToken))
			}

			c.Set(UserIDContextKey, claims.UserID)
			c.Set(EmailContextKey, claims.Email)
			c.Set(RoleContextKey, claims.Role)

			return next(c)
		}
	}
}
