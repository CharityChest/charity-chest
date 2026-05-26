package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"charity-chest/services/admin/backend/internal/middleware"
	"charity-chest/services/admin/backend/internal/model"
	"charity-chest/services/admin/backend/internal/testdb"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

func newACLTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testdb.Open(t)
}

// invokeSystemRole runs RequireSystemRole(allowed...) with the given caller role
// (nil = no role) and returns the HTTP status code and whether next was called.
func invokeSystemRole(t *testing.T, callerRole *model.AdministrativeRole, allowed ...model.AdministrativeRole) (int, bool) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.RoleContextKey, callerRole)

	var nextCalled bool
	h := middleware.RequireSystemRole(allowed...)(func(c echo.Context) error {
		nextCalled = true
		return c.String(http.StatusOK, "ok")
	})
	if err := h(c); err != nil {
		if he, ok := err.(*echo.HTTPError); ok {
			return he.Code, false
		}
		t.Fatalf("unexpected error: %v", err)
	}
	return rec.Code, nextCalled
}

// --- RequireSystemRole ---

func TestRequireSystemRole_AllowedRole_Passes(t *testing.T) {
	root := model.RoleRoot
	code, called := invokeSystemRole(t, &root, model.RoleRoot)
	if code != http.StatusOK || !called {
		t.Errorf("code = %d, called = %v; want 200, true", code, called)
	}
}

func TestRequireSystemRole_MultipleAllowed_EachPasses(t *testing.T) {
	root := model.RoleRoot
	sys := model.RoleSystem

	codeRoot, calledRoot := invokeSystemRole(t, &root, model.RoleRoot, model.RoleSystem)
	if codeRoot != http.StatusOK || !calledRoot {
		t.Errorf("root: code = %d, called = %v; want 200, true", codeRoot, calledRoot)
	}
	codeSys, calledSys := invokeSystemRole(t, &sys, model.RoleRoot, model.RoleSystem)
	if codeSys != http.StatusOK || !calledSys {
		t.Errorf("system: code = %d, called = %v; want 200, true", codeSys, calledSys)
	}
}

func TestRequireSystemRole_NoRole_Forbidden(t *testing.T) {
	code, called := invokeSystemRole(t, nil, model.RoleRoot)
	if code != http.StatusForbidden || called {
		t.Errorf("code = %d, called = %v; want 403, false", code, called)
	}
}

func TestRequireSystemRole_WrongRole_Forbidden(t *testing.T) {
	sys := model.RoleSystem
	code, called := invokeSystemRole(t, &sys, model.RoleRoot)
	if code != http.StatusForbidden || called {
		t.Errorf("code = %d, called = %v; want 403, false", code, called)
	}
}

func TestRequireSystemRole_RoleNotInAllowedSet_Forbidden(t *testing.T) {
	root := model.RoleRoot
	code, called := invokeSystemRole(t, &root, model.RoleSystem)
	if code != http.StatusForbidden || called {
		t.Errorf("code = %d, called = %v; want 403, false", code, called)
	}
}

// --- RequireOrgRole ---

// invokeOrgRole runs RequireOrgRole(db, allowed...) with the given caller
// context. orgUUID is what the middleware reads from :orgUUID; pass an empty
// string to omit the param (covers the bypass paths where the middleware
// short-circuits before any DB lookup needs to succeed — but in practice the
// middleware still parses the param first, so the bypass tests use a stored
// org's UUID anyway).
func invokeOrgRole(t *testing.T, db *gorm.DB, callerSysRole *model.AdministrativeRole, userID uint, orgUUID string, allowed ...model.MemberRole) (int, bool) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("orgUUID")
	c.SetParamValues(orgUUID)
	c.Set(middleware.UserIDContextKey, userID)
	c.Set(middleware.RoleContextKey, callerSysRole)

	var nextCalled bool
	h := middleware.RequireOrgRole(db, allowed...)(func(c echo.Context) error {
		nextCalled = true
		return c.String(http.StatusOK, "ok")
	})
	if err := h(c); err != nil {
		if he, ok := err.(*echo.HTTPError); ok {
			return he.Code, false
		}
		t.Fatalf("unexpected error: %v", err)
	}
	return rec.Code, nextCalled
}

func TestRequireOrgRole_RootBypasses(t *testing.T) {
	db := newACLTestDB(t)
	org := model.Organization{Name: "Org"}
	db.Create(&org)

	root := model.RoleRoot
	code, called := invokeOrgRole(t, db, &root, 0, org.UUID.String(), model.OrgRoleOwner)
	if code != http.StatusOK || !called {
		t.Errorf("root bypass: code = %d, called = %v; want 200, true", code, called)
	}
}

func TestRequireOrgRole_SystemBypasses(t *testing.T) {
	db := newACLTestDB(t)
	org := model.Organization{Name: "Org"}
	db.Create(&org)

	sys := model.RoleSystem
	code, called := invokeOrgRole(t, db, &sys, 0, org.UUID.String(), model.OrgRoleOwner)
	if code != http.StatusOK || !called {
		t.Errorf("system bypass: code = %d, called = %v; want 200, true", code, called)
	}
}

func TestRequireOrgRole_Member_AllowedRole_Passes(t *testing.T) {
	db := newACLTestDB(t)
	user := model.User{Email: "owner@example.com", Name: "Owner"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOwner})

	code, called := invokeOrgRole(t, db, nil, user.ID, org.UUID.String(), model.OrgRoleOwner)
	if code != http.StatusOK || !called {
		t.Errorf("code = %d, called = %v; want 200, true", code, called)
	}
}

func TestRequireOrgRole_Member_WrongRole_Forbidden(t *testing.T) {
	db := newACLTestDB(t)
	user := model.User{Email: "op@example.com", Name: "Op"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOperational})

	// Endpoint requires owner; caller is only operational.
	code, called := invokeOrgRole(t, db, nil, user.ID, org.UUID.String(), model.OrgRoleOwner)
	if code != http.StatusForbidden || called {
		t.Errorf("code = %d, called = %v; want 403, false", code, called)
	}
}

func TestRequireOrgRole_NonMember_Forbidden(t *testing.T) {
	db := newACLTestDB(t)
	user := model.User{Email: "outsider@example.com", Name: "Outsider"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	// No OrgMember row for this user+org.

	code, called := invokeOrgRole(t, db, nil, user.ID, org.UUID.String(), model.OrgRoleOwner)
	if code != http.StatusForbidden || called {
		t.Errorf("code = %d, called = %v; want 403, false", code, called)
	}
}

func TestRequireOrgRole_Member_MultipleAllowedRoles(t *testing.T) {
	db := newACLTestDB(t)
	user := model.User{Email: "admin@example.com", Name: "Admin"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleAdmin})

	// Admin is in the allowed set [owner, admin].
	code, called := invokeOrgRole(t, db, nil, user.ID, org.UUID.String(), model.OrgRoleOwner, model.OrgRoleAdmin)
	if code != http.StatusOK || !called {
		t.Errorf("code = %d, called = %v; want 200, true", code, called)
	}
}

// TestRequireOrgRole_OrgUUIDNotFound covers the new UUID→id resolution path:
// when the :orgUUID param is well-formed but no row matches, the middleware
// must return 404, not 403.
func TestRequireOrgRole_OrgUUIDNotFound_Returns404(t *testing.T) {
	db := newACLTestDB(t)
	user := model.User{Email: "u@example.com", Name: "U"}
	db.Create(&user)

	// Random UUID — parses cleanly but no organisations row matches.
	code, called := invokeOrgRole(t, db, nil, user.ID, uuid.New().String(), model.OrgRoleOwner)
	if code != http.StatusNotFound || called {
		t.Errorf("code = %d, called = %v; want 404, false", code, called)
	}
}

// TestRequireOrgRole_OrgUUIDMalformed covers the parse-failure branch: an
// unparseable :orgUUID returns 404 (treated as "org not found").
func TestRequireOrgRole_OrgUUIDMalformed_Returns404(t *testing.T) {
	db := newACLTestDB(t)
	user := model.User{Email: "u@example.com", Name: "U"}
	db.Create(&user)

	code, called := invokeOrgRole(t, db, nil, user.ID, "not-a-uuid", model.OrgRoleOwner)
	if code != http.StatusNotFound || called {
		t.Errorf("code = %d, called = %v; want 404, false", code, called)
	}
}

func TestRequireSystemRole_WithLocale_IT_Forbidden(t *testing.T) {
	// Setting "it" locale in context exercises the localeFrom "return l" branch.
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.LocaleContextKey, middleware.LocaleIT) // locale set
	c.Set(middleware.RoleContextKey, (*model.AdministrativeRole)(nil))

	h := middleware.RequireSystemRole(model.RoleRoot)(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	err := h(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusForbidden {
		t.Errorf("expected 403 HTTPError, got %v", err)
	}
}

func TestRequireOrgRole_InjectsOrgMemberRole(t *testing.T) {
	db := newACLTestDB(t)
	user := model.User{Email: "admin@example.com", Name: "Admin"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleAdmin})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("orgUUID")
	c.SetParamValues(org.UUID.String())
	c.Set(middleware.UserIDContextKey, user.ID)
	c.Set(middleware.RoleContextKey, (*model.AdministrativeRole)(nil))

	var injectedRole model.MemberRole
	var resolvedOrgID uint
	h := middleware.RequireOrgRole(db, model.OrgRoleAdmin)(func(c echo.Context) error {
		injectedRole, _ = c.Get("org_member_role").(model.MemberRole)
		resolvedOrgID, _ = c.Get(middleware.OrgIDContextKey).(uint)
		return c.String(http.StatusOK, "ok")
	})
	if err := h(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if injectedRole != model.OrgRoleAdmin {
		t.Errorf("org_member_role = %q, want %q", injectedRole, model.OrgRoleAdmin)
	}
	if resolvedOrgID != org.ID {
		t.Errorf("OrgIDContextKey = %d, want %d (middleware should resolve UUID→id)", resolvedOrgID, org.ID)
	}
}
