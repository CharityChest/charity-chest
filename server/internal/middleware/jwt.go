package middleware

import (
	"net/http"
	"strings"

	"charity-chest/internal/i18n"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// Claims holds the JWT payload stored in each token.
type Claims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// JWT returns an Echo middleware that validates Bearer tokens and injects
// "user_id" and "email" into the request context.
func JWT(secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			loc, _ := c.Get(LocaleContextKey).(string)
			if loc == "" {
				loc = "en"
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

			c.Set("user_id", claims.UserID)
			c.Set("email", claims.Email)

			return next(c)
		}
	}
}
