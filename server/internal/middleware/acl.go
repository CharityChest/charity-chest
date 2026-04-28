package middleware

import (
	"net/http"

	"charity-chest/internal/i18n"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RequireSystemRole returns middleware that passes only when the caller's JWT role
// matches one of the allowed system-level roles.
// Must be used after the JWT middleware on the same group or route.
func RequireSystemRole(allowed ...model.AdministrativeRole) echo.MiddlewareFunc {
	set := make(map[model.AdministrativeRole]struct{}, len(allowed))
	for _, r := range allowed {
		set[r] = struct{}{}
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			rolePtr, _ := c.Get(RoleContextKey).(*model.AdministrativeRole)
			if rolePtr == nil {
				return echo.NewHTTPError(http.StatusForbidden, i18n.T(localeFrom(c), i18n.KeyForbidden))
			}
			if _, ok := set[*rolePtr]; !ok {
				return echo.NewHTTPError(http.StatusForbidden, i18n.T(localeFrom(c), i18n.KeyForbidden))
			}
			return next(c)
		}
	}
}

// RequireOrgRole returns middleware that verifies the caller is a member of the
// organisation identified by the ":orgID" path parameter with one of the allowed roles.
//
// Root and system users bypass the org membership check entirely.
//
// On success, injects "org_member_role" (string) into the Echo context so handlers
// can reuse it for hierarchy checks without an additional DB query.
func RequireOrgRole(db *gorm.DB, allowed ...model.MemberRole) echo.MiddlewareFunc {
	set := make(map[model.MemberRole]struct{}, len(allowed))
	for _, r := range allowed {
		set[r] = struct{}{}
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			loc := localeFrom(c)

			// System-level users bypass org membership.
			rolePtr, _ := c.Get(RoleContextKey).(*model.AdministrativeRole)
			if rolePtr != nil && (*rolePtr == model.RoleRoot || *rolePtr == model.RoleSystem) {
				return next(c)
			}

			userID, _ := c.Get(UserIDContextKey).(uint)

			var member model.OrgMember
			if err := db.Where("org_id = ? AND user_id = ?", c.Param("orgID"), userID).First(&member).Error; err != nil {
				return echo.NewHTTPError(http.StatusForbidden, i18n.T(loc, i18n.KeyForbidden))
			}
			if _, ok := set[member.Role]; !ok {
				return echo.NewHTTPError(http.StatusForbidden, i18n.T(loc, i18n.KeyForbidden))
			}

			// Inject the org role so handlers can use it without a second DB query.
			c.Set("org_member_role", member.Role)
			return next(c)
		}
	}
}

// localeFrom extracts the resolved locale from the Echo context.
func localeFrom(c echo.Context) string {
	if l, ok := c.Get(LocaleContextKey).(string); ok && l != "" {
		return l
	}
	return LocaleEN
}
