package middleware

import (
	"net/http"
	"strings"

	"charity-chest/internal/i18n"
	"charity-chest/internal/model"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Context keys injected by the JWT middleware.
const (
	UserIDContextKey = "user_id"
	EmailContextKey  = "email"
	RoleContextKey   = "role"

	// OrgIDContextKey holds the internal integer org id resolved from the
	// `:orgUUID` path parameter. Set by RequireOrgRole (or by a handler that
	// performs the UUID lookup itself) so subsequent handler code can use the
	// int FK without a second lookup.
	OrgIDContextKey = "org_id"
)

// Claims holds the JWT payload stored in each token.
// UserUUID is the user's public UUID; the middleware resolves it to the internal
// int id once per request so downstream handlers keep using the int id.
// MFAPending, when true, marks a short-lived token issued mid-login for MFA verification;
// the JWT middleware rejects these so they cannot be used as full auth tokens.
type Claims struct {
	UserUUID   uuid.UUID                 `json:"user_uuid"`
	Email      string                    `json:"email"`
	Role       *model.AdministrativeRole `json:"role,omitempty"`
	MFAPending *bool                     `json:"mfa_pending,omitempty"`
	jwt.RegisteredClaims
}

// JWT returns an Echo middleware that validates Bearer tokens, resolves the
// token's user UUID to the internal int id via a single DB lookup, and injects
// the int "user_id", "email", and "role" into the request context. Downstream
// handlers continue to read the int id from UserIDContextKey.
func JWT(db *gorm.DB, secret string) echo.MiddlewareFunc {
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

			// Resolve the public UUID to the internal int id. A valid signature
			// over a UUID that no longer maps to a user (e.g. deleted account) is
			// treated as an invalid token — same response, no enumeration signal.
			var u model.User
			if err := db.Select("id").Where("uuid = ?", claims.UserUUID).First(&u).Error; err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(loc, i18n.KeyInvalidToken))
			}

			c.Set(UserIDContextKey, u.ID)
			c.Set(EmailContextKey, claims.Email)
			c.Set(RoleContextKey, claims.Role)

			return next(c)
		}
	}
}
