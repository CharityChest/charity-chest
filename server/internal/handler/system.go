package handler

import (
	"net/http"

	"charity-chest/internal/i18n"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// SystemHandler handles system-level administration endpoints.
type SystemHandler struct {
	db *gorm.DB
}

// NewSystemHandler creates a SystemHandler backed by the given database.
func NewSystemHandler(db *gorm.DB) *SystemHandler {
	return &SystemHandler{db: db}
}

type systemStatusResponse struct {
	Configured bool `json:"configured"`
}

// SystemStatus godoc
// GET /v1/system/status — public, no auth required.
// Returns {"configured": true} if at least one root user exists.
func (h *SystemHandler) SystemStatus(c echo.Context) error {
	var count int64
	h.db.Model(&model.User{}).Where("role = ? AND deleted_at IS NULL", model.RoleRoot).Count(&count)
	return dataJSON(c, http.StatusOK, systemStatusResponse{Configured: count > 0})
}

type assignSystemRoleRequest struct {
	UserID uint                `json:"user_id"`
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

	return dataJSON(c, http.StatusOK, &target)
}
