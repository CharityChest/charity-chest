package handler

import (
	"errors"
	"log"
	"net/http"

	"charity-chest/internal/cache"
	"charity-chest/internal/i18n"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// createOrgRequest is the JSON body for POST /v1/api/orgs.
type createOrgRequest struct {
	Name string `json:"name"`
}

// updateOrgRequest is the JSON body for PUT /v1/api/orgs/:orgUUID.
type updateOrgRequest struct {
	Name string `json:"name"`
}

// addMemberRequest is the JSON body for POST /v1/api/orgs/:orgUUID/members.
// The user is identified by their public UUID — not the internal int id —
// so admin tooling never has to expose enumerable identifiers.
type addMemberRequest struct {
	UserUUID string           `json:"user_uuid"`
	Role     model.MemberRole `json:"role"`
}

// updateMemberRequest is the JSON body for PUT /v1/api/orgs/:orgUUID/members/:userUUID.
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
	org := model.Organization{Name: req.Name, Plan: model.PlanFree}
	if err := h.db.Create(&org).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create organization")
	}
	if err := h.cache.Del(c.Request().Context(), cache.KeyOrgsList); err != nil {
		log.Printf("cache: invalidate after create org: %v", err)
	}
	return dataJSON(c, http.StatusCreated, &org)
}

// GetOrg godoc — GET /v1/api/orgs/:orgUUID
// Behind RequireOrgRole, so the int org id is already resolved into context.
func (h *OrgHandler) GetOrg(c echo.Context) error {
	loc := locale(c)
	orgID := orgIDFromContext(c)
	if orgID == 0 {
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

// UpdateOrg godoc — PUT /v1/api/orgs/:orgUUID
// Behind RequireSystemRole only (no RequireOrgRole), so the handler resolves
// the UUID to a row itself.
func (h *OrgHandler) UpdateOrg(c echo.Context) error {
	loc := locale(c)
	org, err := h.loadOrgByUUID(c)
	if err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) {
			return he
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
		}
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
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

// DeleteOrg godoc — DELETE /v1/api/orgs/:orgUUID
func (h *OrgHandler) DeleteOrg(c echo.Context) error {
	loc := locale(c)
	org, err := h.loadOrgByUUID(c)
	if err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) {
			return he
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
		}
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
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

// ListMembers godoc — GET /v1/api/orgs/:orgUUID/members
func (h *OrgHandler) ListMembers(c echo.Context) error {
	loc := locale(c)
	orgID := orgIDFromContext(c)
	if orgID == 0 {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
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

// AddMember godoc — POST /v1/api/orgs/:orgUUID/members
// Role hierarchy is enforced: caller may only assign roles below their own.
// The request body identifies the user by UUID; we resolve it to the int FK
// once and use the int thereafter.
func (h *OrgHandler) AddMember(c echo.Context) error {
	loc := locale(c)
	orgID := orgIDFromContext(c)
	if orgID == 0 {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
	}
	var req addMemberRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}
	if !model.ValidOrgRole(req.Role) {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidRole))
	}
	targetUserID, err := resolveUserIDByUUID(h.db, req.UserUUID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidUserUUID))
	}
	if err := h.enforceCanAssign(c, orgID, req.Role); err != nil {
		return err
	}

	// Lock the org row, re-check the plan limit, verify no duplicate, and insert —
	// all inside one transaction so concurrent requests cannot race past the cap.
	var member model.OrgMember
	txErr := h.db.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var org model.Organization
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "plan").First(&org, orgID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
			}
			return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
		}
		var existing model.OrgMember
		lookupErr := tx.Where("org_id = ? AND user_id = ?", orgID, targetUserID).First(&existing).Error
		if lookupErr == nil {
			return echo.NewHTTPError(http.StatusConflict, i18n.T(loc, i18n.KeyMemberExists))
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
		}
		if err := checkPlanLimit(tx, loc, org, req.Role); err != nil {
			return err
		}
		member = model.OrgMember{OrgID: orgID, UserID: targetUserID, Role: req.Role}
		if err := tx.Create(&member).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
		}
		return nil
	})
	if txErr != nil {
		var he *echo.HTTPError
		if errors.As(txErr, &he) {
			return he
		}
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
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

// UpdateMember godoc — PUT /v1/api/orgs/:orgUUID/members/:userUUID
func (h *OrgHandler) UpdateMember(c echo.Context) error {
	loc := locale(c)
	orgID := orgIDFromContext(c)
	if orgID == 0 {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
	}
	targetUserID, err := resolveUserIDFromUUIDParam(c, h.db)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyMemberNotFound))
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

	// Lock the org row, re-check the plan limit, and save — all in one transaction.
	var member model.OrgMember
	txErr := h.db.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var org model.Organization
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "plan").First(&org, orgID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
			}
			return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
		}
		if err := tx.Where("org_id = ? AND user_id = ?", orgID, targetUserID).First(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyMemberNotFound))
			}
			return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
		}
		if member.Role == req.Role {
			return nil
		}
		if err := checkPlanLimit(tx, loc, org, req.Role); err != nil {
			return err
		}
		member.Role = req.Role
		if err := tx.Save(&member).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
		}
		return nil
	})
	if txErr != nil {
		var he *echo.HTTPError
		if errors.As(txErr, &he) {
			return he
		}
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
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

// RemoveMember godoc — DELETE /v1/api/orgs/:orgUUID/members/:userUUID
func (h *OrgHandler) RemoveMember(c echo.Context) error {
	loc := locale(c)
	orgID := orgIDFromContext(c)
	if orgID == 0 {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyOrgNotFound))
	}
	targetUserID, err := resolveUserIDFromUUIDParam(c, h.db)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyMemberNotFound))
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

// loadOrgByUUID parses :orgUUID from the route and fetches the organisation
// from the database. Used by handlers that are NOT behind RequireOrgRole
// (which would have resolved the UUID into OrgIDContextKey already).
//
// Returns a 400 *echo.HTTPError for a malformed UUID, gorm.ErrRecordNotFound
// for a well-formed UUID that doesn't match any row, and the raw DB error
// otherwise — letting callers translate each case into the right HTTP status.
func (h *OrgHandler) loadOrgByUUID(c echo.Context) (*model.Organization, error) {
	parsed, err := uuid.Parse(c.Param("orgUUID"))
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyInvalidOrgUUID))
	}
	var org model.Organization
	if err := h.db.Where("uuid = ?", parsed).First(&org).Error; err != nil {
		return nil, err
	}
	return &org, nil
}

// checkPlanLimit verifies that org's plan allows another member with targetRole.
// Must be called with a transaction (tx) that already holds a lock on the org row
// so the count cannot change between read and the subsequent write.
func checkPlanLimit(tx *gorm.DB, loc string, org model.Organization, targetRole model.MemberRole) error {
	limit := model.LimitsFor(org.Plan).ForRole(targetRole)
	if limit == -1 {
		return nil // unlimited
	}
	if limit == 0 {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, i18n.T(loc, i18n.KeyRoleNotAllowedOnPlan))
	}
	var count int64
	if err := tx.Model(&model.OrgMember{}).Where("org_id = ? AND role = ?", org.ID, targetRole).Count(&count).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
	}
	if int(count) >= limit {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, i18n.T(loc, i18n.KeyPlanMemberLimitReached))
	}
	return nil
}

// orgIDFromContext reads the internal int org id resolved by RequireOrgRole.
// Returns 0 if absent — the caller must treat 0 as "org not found".
func orgIDFromContext(c echo.Context) uint {
	id, _ := c.Get(middleware.OrgIDContextKey).(uint)
	return id
}

// resolveUserIDByUUID parses the supplied UUID string and looks up the user's
// internal int id. Returns gorm.ErrRecordNotFound for malformed UUIDs and for
// non-existent users alike.
func resolveUserIDByUUID(db *gorm.DB, raw string) (uint, error) {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return 0, gorm.ErrRecordNotFound
	}
	var u model.User
	if err := db.Select("id").Where("uuid = ?", parsed).First(&u).Error; err != nil {
		return 0, err
	}
	return u.ID, nil
}

// resolveUserIDFromUUIDParam parses :userUUID from the path and returns the
// user's int id.
func resolveUserIDFromUUIDParam(c echo.Context, db *gorm.DB) (uint, error) {
	return resolveUserIDByUUID(db, c.Param("userUUID"))
}
