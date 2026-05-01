package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"charity-chest/internal/cache"
	"charity-chest/internal/i18n"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// OrgHandler handles organization CRUD and member management endpoints.
type OrgHandler struct {
	db    *gorm.DB
	cache *cache.Cache
}

// NewOrgHandler creates an OrgHandler backed by the given database.
func NewOrgHandler(db *gorm.DB, c *cache.Cache) *OrgHandler {
	return &OrgHandler{db: db, cache: c}
}

// --- Request types ---

type createOrgRequest struct {
	Name string `json:"name"`
}

type updateOrgRequest struct {
	Name string `json:"name"`
}

type addMemberRequest struct {
	UserID uint             `json:"user_id"`
	Role   model.MemberRole `json:"role"`
}

type updateMemberRequest struct {
	Role model.MemberRole `json:"role"`
}

// --- Org CRUD (system/root only) ---

// ListOrgs godoc — GET /v1/api/orgs
func (h *OrgHandler) ListOrgs(c echo.Context) error {
	ctx := c.Request().Context()

	var orgs []model.Organization
	if hit, err := h.cache.Get(ctx, cache.KeyOrgsList, &orgs); err != nil {
		log.Printf("cache: get %s: %v", cache.KeyOrgsList, err)
	} else if hit {
		return dataJSON(c, http.StatusOK, orgs)
	}

	if err := h.db.Find(&orgs).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list organizations")
	}

	if err := h.cache.Set(ctx, cache.KeyOrgsList, orgs); err != nil {
		log.Printf("cache: set %s: %v", cache.KeyOrgsList, err)
	}

	return dataJSON(c, http.StatusOK, orgs)
}

// CreateOrg godoc — POST /v1/api/orgs
func (h *OrgHandler) CreateOrg(c echo.Context) error {
	loc := locale(c)
	var req createOrgRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}
	if req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyFieldsRequired))
	}
	org := model.Organization{Name: req.Name}
	if err := h.db.Create(&org).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create organization")
	}
	if err := h.cache.Del(c.Request().Context(), cache.KeyOrgsList); err != nil {
		log.Printf("cache: invalidate after create org: %v", err)
	}
	return dataJSON(c, http.StatusCreated, &org)
}

// GetOrg godoc — GET /v1/api/orgs/:orgID
func (h *OrgHandler) GetOrg(c echo.Context) error {
	loc := locale(c)
	orgID, err := parseOrgID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
	}
	ctx := c.Request().Context()
	key := cache.KeyOrg(orgID)

	var org model.Organization
	if hit, err := h.cache.Get(ctx, key, &org); err != nil {
		log.Printf("cache: get %s: %v", key, err)
	} else if hit {
		return dataJSON(c, http.StatusOK, &org)
	}

	if err := h.db.First(&org, orgID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
	}

	if err := h.cache.Set(ctx, key, &org); err != nil {
		log.Printf("cache: set %s: %v", key, err)
	}

	return dataJSON(c, http.StatusOK, &org)
}

// UpdateOrg godoc — PUT /v1/api/orgs/:orgID
func (h *OrgHandler) UpdateOrg(c echo.Context) error {
	loc := locale(c)
	org, err := h.loadOrg(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
	}
	var req updateOrgRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}
	if req.Name != "" {
		org.Name = req.Name
	}
	if err := h.db.Save(org).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update organization")
	}
	if err := h.cache.Del(c.Request().Context(), cache.KeyOrgsList, cache.KeyOrg(org.ID)); err != nil {
		log.Printf("cache: invalidate after update org %d: %v", org.ID, err)
	}
	return dataJSON(c, http.StatusOK, org)
}

// DeleteOrg godoc — DELETE /v1/api/orgs/:orgID
func (h *OrgHandler) DeleteOrg(c echo.Context) error {
	loc := locale(c)
	org, err := h.loadOrg(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
	}
	if err := h.db.Delete(org).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete organization")
	}
	if err := h.cache.Del(c.Request().Context(), cache.KeyOrgsList, cache.KeyOrg(org.ID), cache.KeyOrgMembers(org.ID)); err != nil {
		log.Printf("cache: invalidate after delete org %d: %v", org.ID, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Member management ---

// ListMembers godoc — GET /v1/api/orgs/:orgID/members
func (h *OrgHandler) ListMembers(c echo.Context) error {
	loc := locale(c)
	orgID, err := parseOrgID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}
	ctx := c.Request().Context()
	key := cache.KeyOrgMembers(orgID)

	var members []model.OrgMember
	if hit, err := h.cache.Get(ctx, key, &members); err != nil {
		log.Printf("cache: get %s: %v", key, err)
	} else if hit {
		return dataJSON(c, http.StatusOK, members)
	}

	if err := h.db.Preload("User").Where("org_id = ?", orgID).Find(&members).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list members")
	}

	if err := h.cache.Set(ctx, key, members); err != nil {
		log.Printf("cache: set %s: %v", key, err)
	}

	return dataJSON(c, http.StatusOK, members)
}

// AddMember godoc — POST /v1/api/orgs/:orgID/members
// Role hierarchy is enforced: caller may only assign roles below their own.
func (h *OrgHandler) AddMember(c echo.Context) error {
	loc := locale(c)
	orgID, err := parseOrgID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}
	var req addMemberRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}
	if !model.ValidOrgRole(req.Role) {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidRole))
	}
	if err := h.enforceCanAssign(c, orgID, req.Role); err != nil {
		return err
	}

	var existing model.OrgMember
	lookupErr := h.db.Where("org_id = ? AND user_id = ?", orgID, req.UserID).First(&existing).Error
	if lookupErr == nil {
		return echo.NewHTTPError(http.StatusConflict, i18n.T(loc, i18n.KeyMemberExists))
	}
	if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}

	member := model.OrgMember{OrgID: orgID, UserID: req.UserID, Role: req.Role}
	if err := h.db.Create(&member).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to add member")
	}
	ctx := c.Request().Context()
	if err := h.cache.Del(ctx, cache.KeyOrgMembers(orgID)); err != nil {
		log.Printf("cache: invalidate after add member to org %d: %v", orgID, err)
	}
	if err := h.cache.DelPattern(ctx, cache.KeyAdminUsersGlob); err != nil {
		log.Printf("cache: invalidate admin users after add member: %v", err)
	}
	return dataJSON(c, http.StatusCreated, &member)
}

// UpdateMember godoc — PUT /v1/api/orgs/:orgID/members/:userID
func (h *OrgHandler) UpdateMember(c echo.Context) error {
	loc := locale(c)
	orgID, err := parseOrgID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}
	targetUserID, err := parseUserIDParam(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}
	var req updateMemberRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}
	if !model.ValidOrgRole(req.Role) {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidRole))
	}
	if err := h.enforceCanAssign(c, orgID, req.Role); err != nil {
		return err
	}

	var member model.OrgMember
	if err := h.db.Where("org_id = ? AND user_id = ?", orgID, targetUserID).First(&member).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyMemberNotFound))
	}
	member.Role = req.Role
	if err := h.db.Save(&member).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update member")
	}
	ctx := c.Request().Context()
	if err := h.cache.Del(ctx, cache.KeyOrgMembers(orgID)); err != nil {
		log.Printf("cache: invalidate after update member in org %d: %v", orgID, err)
	}
	if err := h.cache.DelPattern(ctx, cache.KeyAdminUsersGlob); err != nil {
		log.Printf("cache: invalidate admin users after update member: %v", err)
	}
	return dataJSON(c, http.StatusOK, &member)
}

// RemoveMember godoc — DELETE /v1/api/orgs/:orgID/members/:userID
func (h *OrgHandler) RemoveMember(c echo.Context) error {
	loc := locale(c)
	orgID, err := parseOrgID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}
	targetUserID, err := parseUserIDParam(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}

	var member model.OrgMember
	if err := h.db.Where("org_id = ? AND user_id = ?", orgID, targetUserID).First(&member).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyMemberNotFound))
	}
	// Enforce that the caller can manage the target's current role.
	if err := h.enforceCanAssign(c, orgID, member.Role); err != nil {
		return err
	}

	if err := h.db.Delete(&member).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to remove member")
	}
	ctx := c.Request().Context()
	if err := h.cache.Del(ctx, cache.KeyOrgMembers(orgID)); err != nil {
		log.Printf("cache: invalidate after remove member from org %d: %v", orgID, err)
	}
	if err := h.cache.DelPattern(ctx, cache.KeyAdminUsersGlob); err != nil {
		log.Printf("cache: invalidate admin users after remove member: %v", err)
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Helpers ---

// enforceCanAssign checks whether the caller may assign targetRole within the org.
// Root/system users are always allowed. Org members are checked against CanAssignOrgRole.
func (h *OrgHandler) enforceCanAssign(c echo.Context, orgID uint, targetRole model.MemberRole) error {
	loc := locale(c)

	rolePtr, _ := c.Get(middleware.RoleContextKey).(*model.AdministrativeRole)
	if rolePtr != nil && (*rolePtr == model.RoleRoot || *rolePtr == model.RoleSystem) {
		return nil
	}

	// Use the role injected by RequireOrgRole middleware to avoid an extra DB query.
	actorOrgRole, _ := c.Get("org_member_role").(model.MemberRole)
	if actorOrgRole == "" {
		// Fallback: query directly (should not occur if middleware is wired correctly).
		userID, _ := c.Get(middleware.UserIDContextKey).(uint)
		var m model.OrgMember
		if err := h.db.Where("org_id = ? AND user_id = ?", orgID, userID).First(&m).Error; err != nil {
			return echo.NewHTTPError(http.StatusForbidden, i18n.T(loc, i18n.KeyForbidden))
		}
		actorOrgRole = m.Role
	}

	if !model.CanAssignOrgRole(actorOrgRole, targetRole) {
		return echo.NewHTTPError(http.StatusForbidden, i18n.T(loc, i18n.KeyCannotManageRole))
	}
	return nil
}

func (h *OrgHandler) loadOrg(c echo.Context) (*model.Organization, error) {
	orgID, err := parseOrgID(c)
	if err != nil {
		return nil, err
	}
	var org model.Organization
	if err := h.db.First(&org, orgID).Error; err != nil {
		return nil, err
	}
	return &org, nil
}

func parseOrgID(c echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("orgID"), 10, 64)
	return uint(id), err
}

func parseUserIDParam(c echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("userID"), 10, 64)
	return uint(id), err
}
