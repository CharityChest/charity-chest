package handler

import (
	"log"
	"net/http"

	"charity-chest/internal/cache"
	"charity-chest/internal/i18n"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// SystemHandler handles system-level administration endpoints.
type SystemHandler struct {
	db    *gorm.DB
	cache *cache.Cache
}

// NewSystemHandler creates a SystemHandler backed by the given database.
func NewSystemHandler(db *gorm.DB, c *cache.Cache) *SystemHandler {
	return &SystemHandler{db: db, cache: c}
}

// systemStatusResponse is the JSON body returned by GET /v1/system/status.
type systemStatusResponse struct {
	Configured bool `json:"configured"`
}

// SystemStatus godoc
// GET /v1/system/status — public, no auth required.
// Returns {"configured": true} if at least one root user exists.
func (h *SystemHandler) SystemStatus(c echo.Context) error {
	ctx := c.Request().Context()

	var resp systemStatusResponse
	if hit, err := h.cache.Get(ctx, cache.KeySystemStatus, &resp); err != nil {
		log.Printf("cache: get %s: %v", cache.KeySystemStatus, err)
	} else if hit {
		return dataJSON(c, http.StatusOK, resp)
	}

	var count int64
	if err := h.db.Model(&model.User{}).Where("role = ? AND deleted_at IS NULL", model.RoleRoot).Count(&count).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(locale(c), i18n.KeySystemStatusQueryFailed))
	}
	resp = systemStatusResponse{Configured: count > 0}

	// Only cache the configured=true state. configured=false is transient (exists only
	// before seed-root runs) and must not be cached — the CLI writes directly to the DB
	// without touching the cache, so a cached false would stay stale until TTL expiry.
	if resp.Configured {
		if err := h.cache.Set(ctx, cache.KeySystemStatus, resp); err != nil {
			log.Printf("cache: set %s: %v", cache.KeySystemStatus, err)
		}
	}

	return dataJSON(c, http.StatusOK, resp)
}

// assignSystemRoleRequest is the JSON body for POST /v1/api/system/assign-role.
type assignSystemRoleRequest struct {
	UserID uint                     `json:"user_id"`
	Role   model.AdministrativeRole `json:"role"`
}

// AssignSystemRole godoc
// POST /v1/api/system/assign-role — requires root JWT.
// Assigns role="system" to a user, or removes their system role by passing role="".
// Cannot demote or promote another root account via this endpoint.
func (h *SystemHandler) AssignSystemRole(c echo.Context) error {
	loc := locale(c)

	var req assignSystemRoleRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}

	// Only "system" and "" (remove) are assignable through the API.
	if req.Role != "" && req.Role != model.RoleSystem {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidRole))
	}

	var target model.User
	if err := h.db.First(&target, req.UserID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyUserNotFound))
	}

	// Protect root accounts from being changed via API.
	if target.Role != nil && *target.Role == model.RoleRoot {
		return echo.NewHTTPError(http.StatusForbidden, i18n.T(loc, i18n.KeyForbidden))
	}

	var roleVal *model.AdministrativeRole
	if req.Role != "" {
		roleVal = &req.Role
	}
	if err := h.db.Model(&target).Update("role", roleVal).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyCreateUser))
	}

	ctx := c.Request().Context()
	if err := h.cache.Del(ctx, cache.KeyUser(req.UserID)); err != nil {
		log.Printf("cache: invalidate user after assign-role: %v", err)
	}
	if err := h.cache.DelPattern(ctx, cache.KeyAdminUsersGlob); err != nil {
		log.Printf("cache: invalidate admin users after assign-role: %v", err)
	}

	return dataJSON(c, http.StatusOK, &target)
}
