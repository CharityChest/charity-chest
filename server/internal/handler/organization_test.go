package handler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charity-chest/internal/cache"
	"charity-chest/internal/handler"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"
	"charity-chest/internal/testdb"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// newOrgTestDB returns a fresh per-test Postgres database with all migrations
// applied. The DB is dropped on cleanup.
func newOrgTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testdb.Open(t)
}

// userUUIDByID looks up the user's public UUID by int id. Falls back to a
// random UUID when no such user exists.
func userUUIDByID(t *testing.T, db *gorm.DB, id uint) string {
	t.Helper()
	if id == 0 {
		return ""
	}
	var u model.User
	if err := db.Unscoped().Select("uuid").First(&u, id).Error; err != nil {
		return uuid.New().String()
	}
	return u.UUID.String()
}

// newOrgContext creates an Echo context for org handler unit tests.
//
// When orgID != 0 the handler expects RequireOrgRole to have resolved the
// :orgUUID path param into middleware.OrgIDContextKey. We mirror that here by
// (a) looking up the org's UUID and setting it as the :orgUUID path param and
// (b) seeding middleware.OrgIDContextKey with the int id directly. Tests that
// pass a non-zero orgID for a row that doesn't actually exist (e.g. 9999) get
// a random UUID as :orgUUID and no OrgIDContextKey — which is exactly what the
// real middleware would produce for an unknown UUID, so the not-found tests
// still exercise the handler's "orgID == 0" guard.
func newOrgContext(t *testing.T, db *gorm.DB, method, path, body string, orgID uint, userID uint, sysRole *model.AdministrativeRole, orgRole model.MemberRole) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	if orgID != 0 {
		var org model.Organization
		err := db.Unscoped().Select("id", "uuid").First(&org, orgID).Error
		if err == nil {
			c.SetParamNames("orgUUID")
			c.SetParamValues(org.UUID.String())
			c.Set(middleware.OrgIDContextKey, org.ID)
		} else {
			// Org doesn't exist — simulate the middleware not finding it. Set a
			// random UUID as the path param so handlers that resolve it
			// themselves (UpdateOrg, DeleteOrg) get a clean ErrRecordNotFound.
			c.SetParamNames("orgUUID")
			c.SetParamValues(uuid.New().String())
		}
	}
	c.Set(middleware.UserIDContextKey, userID)
	if sysRole != nil {
		c.Set(middleware.RoleContextKey, sysRole)
	}
	if orgRole != "" {
		c.Set("org_member_role", orgRole)
	}
	return c, rec
}

// newOrgContextWithUserID creates a context with the :userUUID path param set
// for member endpoints. When targetUserID refers to a real row we use that
// user's UUID; otherwise we substitute a random UUID so the handler observes
// the equivalent of "no such user".
func newOrgContextWithUserID(t *testing.T, db *gorm.DB, method, path, body string, orgID, targetUserID, callerUserID uint, sysRole *model.AdministrativeRole, orgRole model.MemberRole) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, rec := newOrgContext(t, db, method, path, body, orgID, callerUserID, sysRole, orgRole)

	userUUID := userUUIDByID(t, db, targetUserID)

	names := append(c.ParamNames(), "userUUID")
	values := append(c.ParamValues(), userUUID)
	c.SetParamNames(names...)
	c.SetParamValues(values...)
	return c, rec
}

// decodeOrgBody decodes the response body into a generic map.
func decodeOrgBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	return decodeBody(t, rec) // reuse auth_test.go helper
}

// memberAddBody builds the JSON body used by AddMember tests. Pass a real
// user's UUID (or a random one for not-found cases).
func memberAddBody(userUUID, role string) string {
	return fmt.Sprintf(`{"user_uuid":%q,"role":%q}`, userUUID, role)
}

// --- ListOrgs ---

func TestListOrgs_Empty(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())

	root := model.RoleRoot
	c, rec := newOrgContext(t, db, http.MethodGet, "/v1/api/orgs", "", 0, 1, &root, "")
	if err := h.ListOrgs(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decodeOrgBody(t, rec)
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatal("response missing 'data' array")
	}
	if len(data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(data))
	}
}

func TestListOrgs_ReturnsAll(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	db.Create(&model.Organization{Name: "Org A"})
	db.Create(&model.Organization{Name: "Org B"})

	root := model.RoleRoot
	c, rec := newOrgContext(t, db, http.MethodGet, "/v1/api/orgs", "", 0, 1, &root, "")
	if err := h.ListOrgs(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body := decodeOrgBody(t, rec)
	data := body["data"].([]any)
	if len(data) != 2 {
		t.Errorf("len(data) = %d, want 2", len(data))
	}
}

// --- CreateOrg ---

func TestCreateOrg_Success(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())

	root := model.RoleRoot
	c, rec := newOrgContext(t, db, http.MethodPost, "/v1/api/orgs", `{"name":"New Org"}`, 0, 1, &root, "")
	if err := h.CreateOrg(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeOrgBody(t, rec)
	data := body["data"].(map[string]any)
	if data["name"] != "New Org" {
		t.Errorf("name = %v, want New Org", data["name"])
	}
	if data["uuid"] == nil || data["uuid"] == "" {
		t.Error("response missing uuid")
	}

	var org model.Organization
	db.Where("name = ?", "New Org").First(&org)
	if org.ID == 0 {
		t.Error("org not persisted to DB")
	}
}

func TestCreateOrg_EmptyName_Returns400(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())

	root := model.RoleRoot
	c, _ := newOrgContext(t, db, http.MethodPost, "/v1/api/orgs", `{"name":""}`, 0, 1, &root, "")
	err := h.CreateOrg(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400 HTTPError, got %v", err)
	}
}

// --- GetOrg ---

func TestGetOrg_Found(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	org := model.Organization{Name: "Found Org"}
	db.Create(&org)

	root := model.RoleRoot
	c, rec := newOrgContext(t, db, http.MethodGet, "/v1/api/orgs/"+org.UUID.String(), "", org.ID, 1, &root, "")
	if err := h.GetOrg(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decodeOrgBody(t, rec)
	data := body["data"].(map[string]any)
	if data["name"] != "Found Org" {
		t.Errorf("name = %v, want Found Org", data["name"])
	}
}

func TestGetOrg_NotFound_Returns404(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())

	root := model.RoleRoot
	// orgID 9999 doesn't exist → newOrgContext leaves OrgIDContextKey unset, the
	// handler observes orgID == 0 and returns 404.
	c, _ := newOrgContext(t, db, http.MethodGet, "/v1/api/orgs/missing", "", 9999, 1, &root, "")
	err := h.GetOrg(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 HTTPError, got %v", err)
	}
}

// --- UpdateOrg ---

func TestUpdateOrg_UpdatesName(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	org := model.Organization{Name: "Old Name"}
	db.Create(&org)

	root := model.RoleRoot
	c, rec := newOrgContext(t, db, http.MethodPut, "/v1/api/orgs/"+org.UUID.String(),
		`{"name":"New Name"}`, org.ID, 1, &root, "")
	if err := h.UpdateOrg(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeOrgBody(t, rec)
	data := body["data"].(map[string]any)
	if data["name"] != "New Name" {
		t.Errorf("name = %v, want New Name", data["name"])
	}

	var updated model.Organization
	db.First(&updated, org.ID)
	if updated.Name != "New Name" {
		t.Errorf("DB name = %q, want New Name", updated.Name)
	}
}

func TestUpdateOrg_NotFound_Returns404(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())

	root := model.RoleRoot
	c, _ := newOrgContext(t, db, http.MethodPut, "/v1/api/orgs/missing", `{"name":"X"}`, 9999, 1, &root, "")
	err := h.UpdateOrg(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 HTTPError, got %v", err)
	}
}

// --- DeleteOrg ---

func TestDeleteOrg_Success(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	org := model.Organization{Name: "ToDelete"}
	db.Create(&org)

	root := model.RoleRoot
	c, rec := newOrgContext(t, db, http.MethodDelete, "/v1/api/orgs/"+org.UUID.String(), "", org.ID, 1, &root, "")
	if err := h.DeleteOrg(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}

	var count int64
	db.Model(&model.Organization{}).Where("id = ? AND deleted_at IS NULL", org.ID).Count(&count)
	if count != 0 {
		t.Error("org still present in DB after delete")
	}
}

func TestDeleteOrg_NotFound_Returns404(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())

	root := model.RoleRoot
	c, _ := newOrgContext(t, db, http.MethodDelete, "/v1/api/orgs/missing", "", 9999, 1, &root, "")
	err := h.DeleteOrg(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 HTTPError, got %v", err)
	}
}

// --- ListMembers ---

func TestListMembers_Empty(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	org := model.Organization{Name: "Empty Org"}
	db.Create(&org)

	root := model.RoleRoot
	c, rec := newOrgContext(t, db, http.MethodGet, "/v1/api/orgs/"+org.UUID.String()+"/members", "", org.ID, 1, &root, "")
	if err := h.ListMembers(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decodeOrgBody(t, rec)
	data := body["data"].([]any)
	if len(data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(data))
	}
}

func TestListMembers_ReturnsMembersWithUser(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	user := model.User{Email: "member@example.com", Name: "Member"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOwner})

	root := model.RoleRoot
	c, rec := newOrgContext(t, db, http.MethodGet, "/v1/api/orgs/"+org.UUID.String()+"/members", "", org.ID, 1, &root, "")
	if err := h.ListMembers(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body := decodeOrgBody(t, rec)
	data := body["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("len(data) = %d, want 1", len(data))
	}
	m := data[0].(map[string]any)
	if m["role"] != string(model.OrgRoleOwner) {
		t.Errorf("role = %v, want owner", m["role"])
	}
	userObj, ok := m["user"].(map[string]any)
	if !ok {
		t.Fatal("member missing 'user' object (Preload failed)")
	}
	if userObj["email"] != "member@example.com" {
		t.Errorf("user.email = %v, want member@example.com", userObj["email"])
	}
}

// --- AddMember ---

func TestAddMember_SystemRole_Success(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	user := model.User{Email: "new@example.com", Name: "New"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)

	sys := model.RoleSystem
	body := memberAddBody(user.UUID.String(), "operational")
	c, rec := newOrgContext(t, db, http.MethodPost, "/v1/api/orgs/"+org.UUID.String()+"/members",
		body, org.ID, 1, &sys, "")
	if err := h.AddMember(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}

	var m model.OrgMember
	db.Where("org_id = ? AND user_id = ?", org.ID, user.ID).First(&m)
	if m.Role != model.OrgRoleOperational {
		t.Errorf("DB role = %q, want operational", m.Role)
	}
}

func TestAddMember_OwnerCanAddAdmin(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	ownerUser := model.User{Email: "owner@example.com", Name: "Owner"}
	db.Create(&ownerUser)
	targetUser := model.User{Email: "admin@example.com", Name: "Admin"}
	db.Create(&targetUser)
	org := model.Organization{Name: "Org", Plan: model.PlanEnterprise}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: ownerUser.ID, Role: model.OrgRoleOwner})

	body := memberAddBody(targetUser.UUID.String(), "admin")
	// No system role; org_member_role = owner (injected by RequireOrgRole middleware in real flow).
	c, rec := newOrgContext(t, db, http.MethodPost, "/v1/api/orgs/"+org.UUID.String()+"/members",
		body, org.ID, ownerUser.ID, nil, model.OrgRoleOwner)
	if err := h.AddMember(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAddMember_AdminCannotAddOwner_Returns403(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	adminUser := model.User{Email: "admin@example.com", Name: "Admin"}
	db.Create(&adminUser)
	targetUser := model.User{Email: "target@example.com", Name: "Target"}
	db.Create(&targetUser)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: adminUser.ID, Role: model.OrgRoleAdmin})

	body := memberAddBody(targetUser.UUID.String(), "owner")
	c, _ := newOrgContext(t, db, http.MethodPost, "/v1/api/orgs/"+org.UUID.String()+"/members",
		body, org.ID, adminUser.ID, nil, model.OrgRoleAdmin)
	err := h.AddMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusForbidden {
		t.Errorf("expected 403 HTTPError, got %v", err)
	}
}

func TestAddMember_InvalidRole_Returns400(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	user := model.User{Email: "u@example.com", Name: "U"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)

	sys := model.RoleSystem
	body := memberAddBody(user.UUID.String(), "superadmin")
	c, _ := newOrgContext(t, db, http.MethodPost, "/v1/api/orgs/"+org.UUID.String()+"/members",
		body, org.ID, 1, &sys, "")
	err := h.AddMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400 HTTPError, got %v", err)
	}
}

func TestAddMember_DuplicateMember_Returns409(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	user := model.User{Email: "dup@example.com", Name: "Dup"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOperational})

	sys := model.RoleSystem
	body := memberAddBody(user.UUID.String(), "operational")
	c, _ := newOrgContext(t, db, http.MethodPost, "/v1/api/orgs/"+org.UUID.String()+"/members",
		body, org.ID, 1, &sys, "")
	err := h.AddMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusConflict {
		t.Errorf("expected 409 HTTPError, got %v", err)
	}
}

// --- UpdateMember ---

func TestUpdateMember_Success(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	user := model.User{Email: "op@example.com", Name: "Op"}
	db.Create(&user)
	org := model.Organization{Name: "Org", Plan: model.PlanEnterprise}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOperational})

	sys := model.RoleSystem
	c, rec := newOrgContextWithUserID(t, db, http.MethodPut,
		fmt.Sprintf("/v1/api/orgs/%s/members/%s", org.UUID, user.UUID),
		`{"role":"admin"}`, org.ID, user.ID, 1, &sys, "")
	if err := h.UpdateMember(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var m model.OrgMember
	db.Where("org_id = ? AND user_id = ?", org.ID, user.ID).First(&m)
	if m.Role != model.OrgRoleAdmin {
		t.Errorf("DB role = %q, want admin", m.Role)
	}
}

func TestUpdateMember_NotFound_Returns404(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	org := model.Organization{Name: "Org", Plan: model.PlanEnterprise}
	db.Create(&org)

	sys := model.RoleSystem
	// targetUserID 9999 doesn't exist → newOrgContextWithUserID substitutes a
	// random UUID, which the handler's resolveUserIDFromUUIDParam reports as
	// ErrRecordNotFound, yielding 404 KeyMemberNotFound.
	c, _ := newOrgContextWithUserID(t, db, http.MethodPut,
		fmt.Sprintf("/v1/api/orgs/%s/members/missing", org.UUID),
		`{"role":"admin"}`, org.ID, 9999, 1, &sys, "")
	err := h.UpdateMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 HTTPError, got %v", err)
	}
}

func TestUpdateMember_InvalidRole_Returns400(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	user := model.User{Email: "op@example.com", Name: "Op"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOperational})

	sys := model.RoleSystem
	c, _ := newOrgContextWithUserID(t, db, http.MethodPut,
		fmt.Sprintf("/v1/api/orgs/%s/members/%s", org.UUID, user.UUID),
		`{"role":"superadmin"}`, org.ID, user.ID, 1, &sys, "")
	err := h.UpdateMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400 HTTPError, got %v", err)
	}
}

// --- RemoveMember ---

func TestRemoveMember_Success(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	user := model.User{Email: "op@example.com", Name: "Op"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOperational})

	sys := model.RoleSystem
	c, rec := newOrgContextWithUserID(t, db, http.MethodDelete,
		fmt.Sprintf("/v1/api/orgs/%s/members/%s", org.UUID, user.UUID),
		"", org.ID, user.ID, 1, &sys, "")
	if err := h.RemoveMember(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}

	var count int64
	db.Model(&model.OrgMember{}).Where("org_id = ? AND user_id = ?", org.ID, user.ID).Count(&count)
	if count != 0 {
		t.Error("org member still present in DB after removal")
	}
}

func TestRemoveMember_NotFound_Returns404(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	org := model.Organization{Name: "Org"}
	db.Create(&org)

	sys := model.RoleSystem
	c, _ := newOrgContextWithUserID(t, db, http.MethodDelete,
		fmt.Sprintf("/v1/api/orgs/%s/members/missing", org.UUID),
		"", org.ID, 9999, 1, &sys, "")
	err := h.RemoveMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 HTTPError, got %v", err)
	}
}

// TestAddMember_FallbackDBQuery tests enforceCanAssign when org_member_role is not in
// context (the middleware was not wired). The handler falls back to a direct DB query.
func TestAddMember_FallbackDBQuery_MemberFound_Allowed(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	ownerUser := model.User{Email: "owner@example.com", Name: "Owner"}
	db.Create(&ownerUser)
	targetUser := model.User{Email: "target@example.com", Name: "Target"}
	db.Create(&targetUser)
	org := model.Organization{Name: "Org", Plan: model.PlanEnterprise}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: ownerUser.ID, Role: model.OrgRoleOwner})

	// No org_member_role in context — handler must fall back to DB query.
	body := memberAddBody(targetUser.UUID.String(), "admin")
	c, rec := newOrgContext(t, db, http.MethodPost, "/v1/api/orgs/"+org.UUID.String()+"/members",
		body, org.ID, ownerUser.ID, nil, "") // orgRole = "" triggers fallback
	if err := h.AddMember(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAddMember_FallbackDBQuery_NotMember_Returns403(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	outsider := model.User{Email: "outsider@example.com", Name: "Outsider"}
	db.Create(&outsider)
	targetUser := model.User{Email: "target@example.com", Name: "Target"}
	db.Create(&targetUser)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	// outsider is NOT in the org.

	body := memberAddBody(targetUser.UUID.String(), "operational")
	c, _ := newOrgContext(t, db, http.MethodPost, "/v1/api/orgs/"+org.UUID.String()+"/members",
		body, org.ID, outsider.ID, nil, "") // orgRole = "" triggers fallback, DB finds no member
	err := h.AddMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusForbidden {
		t.Errorf("expected 403 HTTPError, got %v", err)
	}
}

func TestRemoveMember_AdminCannotRemoveOwner_Returns403(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	adminUser := model.User{Email: "admin@example.com", Name: "Admin"}
	db.Create(&adminUser)
	ownerUser := model.User{Email: "owner@example.com", Name: "Owner"}
	db.Create(&ownerUser)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: adminUser.ID, Role: model.OrgRoleAdmin})
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: ownerUser.ID, Role: model.OrgRoleOwner})

	// Admin caller (no system role), targeting an owner.
	c, _ := newOrgContextWithUserID(t, db, http.MethodDelete,
		fmt.Sprintf("/v1/api/orgs/%s/members/%s", org.UUID, ownerUser.UUID),
		"", org.ID, ownerUser.ID, adminUser.ID, nil, model.OrgRoleAdmin)
	err := h.RemoveMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusForbidden {
		t.Errorf("expected 403 HTTPError, got %v", err)
	}
}

// --- Cache paths ---

// TestListOrgs_CacheHit verifies the second call is served from cache even
// after the DB rows are deleted.
func TestListOrgs_CacheHit(t *testing.T) {
	db := newOrgTestDB(t)
	_, c := newMiniRedisCache(t)
	h := handler.NewOrgHandler(db, c)
	db.Create(&model.Organization{Name: "Cached Org"})

	root := model.RoleRoot
	callListOrgs := func() *httptest.ResponseRecorder {
		ctx, rec := newOrgContext(t, db, http.MethodGet, "/v1/api/orgs", "", 0, 1, &root, "")
		if err := h.ListOrgs(ctx); err != nil {
			t.Fatalf("ListOrgs: %v", err)
		}
		return rec
	}

	// First call: cache miss → DB read → cache populated.
	rec1 := callListOrgs()
	if rec1.Code != http.StatusOK {
		t.Fatalf("first ListOrgs status = %d", rec1.Code)
	}

	// Delete all orgs from DB.
	db.Exec("DELETE FROM organizations")

	// Second call: cache hit → still returns "Cached Org".
	rec2 := callListOrgs()
	body := decodeOrgBody(t, rec2)
	data := body["data"].([]any)
	if len(data) != 1 {
		t.Errorf("len(data) from cache = %d, want 1", len(data))
	}
}

// TestListOrgs_CacheInvalidatedOnCreate verifies that CreateOrg clears the list cache.
func TestListOrgs_CacheInvalidatedOnCreate(t *testing.T) {
	db := newOrgTestDB(t)
	_, c := newMiniRedisCache(t)
	h := handler.NewOrgHandler(db, c)

	root := model.RoleRoot
	callListOrgs := func() []any {
		ctx, rec := newOrgContext(t, db, http.MethodGet, "/v1/api/orgs", "", 0, 1, &root, "")
		if err := h.ListOrgs(ctx); err != nil {
			t.Fatalf("ListOrgs: %v", err)
		}
		return decodeOrgBody(t, rec)["data"].([]any)
	}

	// Populate cache with 0 orgs.
	if len(callListOrgs()) != 0 {
		t.Fatal("expected empty list initially")
	}

	// Create an org (should invalidate orgs:list).
	ctx, _ := newOrgContext(t, db, http.MethodPost, "/v1/api/orgs", `{"name":"New Org"}`, 0, 1, &root, "")
	if err := h.CreateOrg(ctx); err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}

	// Next list call must reflect the new org (cache was cleared).
	if got := callListOrgs(); len(got) != 1 {
		t.Errorf("len(data) after create = %d, want 1", len(got))
	}
}

// TestGetOrg_CacheHit verifies that GetOrg serves from cache after DB delete.
func TestGetOrg_CacheHit(t *testing.T) {
	db := newOrgTestDB(t)
	_, c := newMiniRedisCache(t)
	h := handler.NewOrgHandler(db, c)

	org := model.Organization{Name: "Org Hit"}
	db.Create(&org)

	root := model.RoleRoot
	callGetOrg := func() (*httptest.ResponseRecorder, error) {
		ctx, rec := newOrgContext(t, db, http.MethodGet, "/v1/api/orgs/"+org.UUID.String(), "", org.ID, 1, &root, "")
		err := h.GetOrg(ctx)
		return rec, err
	}

	// First call: cache miss.
	if _, err := callGetOrg(); err != nil {
		t.Fatalf("GetOrg first call: %v", err)
	}

	// Delete from DB.
	db.Unscoped().Delete(&org)

	// Second call: cache hit → still returns the org.
	rec, err := callGetOrg()
	if err != nil {
		t.Fatalf("GetOrg second call (cache hit): %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := decodeOrgBody(t, rec)
	data := body["data"].(map[string]any)
	if data["name"] != "Org Hit" {
		t.Errorf("name from cache = %v, want Org Hit", data["name"])
	}
}

// TestListMembers_CacheHit verifies that ListMembers is served from cache.
func TestListMembers_CacheHit(t *testing.T) {
	db := newOrgTestDB(t)
	_, c := newMiniRedisCache(t)
	h := handler.NewOrgHandler(db, c)

	user := model.User{Email: "member@example.com", Name: "Member"}
	db.Create(&user)
	org := model.Organization{Name: "Org Members"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOwner})

	root := model.RoleRoot
	callList := func() []any {
		ctx, rec := newOrgContext(t, db, http.MethodGet, fmt.Sprintf("/v1/api/orgs/%s/members", org.UUID), "", org.ID, 1, &root, "")
		if err := h.ListMembers(ctx); err != nil {
			t.Fatalf("ListMembers: %v", err)
		}
		return decodeOrgBody(t, rec)["data"].([]any)
	}

	// First call: cache miss → populates cache.
	if len(callList()) != 1 {
		t.Fatal("expected 1 member initially")
	}

	// Delete the member from DB.
	db.Exec("DELETE FROM org_members")

	// Second call: cache hit → still returns 1 member.
	if got := callList(); len(got) != 1 {
		t.Errorf("len from cache = %d, want 1", len(got))
	}
}

// TestListOrgs_BrokenCache_FallsThroughToDB verifies that a broken cache is non-fatal
// and the handler returns live DB data, covering the cache error log paths.
func TestListOrgs_BrokenCache_FallsThroughToDB(t *testing.T) {
	db := newOrgTestDB(t)
	mr, c := newMiniRedisCache(t)
	h := handler.NewOrgHandler(db, c)

	db.Create(&model.Organization{Name: "Live Org"})
	mr.Close() // break the cache — all operations will fail

	root := model.RoleRoot
	ctx, rec := newOrgContext(t, db, http.MethodGet, "/v1/api/orgs", "", 0, 1, &root, "")
	if err := h.ListOrgs(ctx); err != nil {
		t.Fatalf("ListOrgs with broken cache: %v", err)
	}
	body := decodeOrgBody(t, rec)
	data := body["data"].([]any)
	if len(data) != 1 {
		t.Errorf("len(data) = %d, want 1 (from DB fallthrough)", len(data))
	}
}

// TestGetOrg_BrokenCache_FallsThroughToDB verifies GetOrg falls through on cache errors.
func TestGetOrg_BrokenCache_FallsThroughToDB(t *testing.T) {
	db := newOrgTestDB(t)
	mr, c := newMiniRedisCache(t)
	h := handler.NewOrgHandler(db, c)

	org := model.Organization{Name: "Live Org"}
	db.Create(&org)
	mr.Close()

	root := model.RoleRoot
	ctx, rec := newOrgContext(t, db, http.MethodGet, "/v1/api/orgs/"+org.UUID.String(), "", org.ID, 1, &root, "")
	if err := h.GetOrg(ctx); err != nil {
		t.Fatalf("GetOrg with broken cache: %v", err)
	}
	body := decodeOrgBody(t, rec)
	if body["data"].(map[string]any)["name"] != "Live Org" {
		t.Error("expected Live Org from DB fallthrough")
	}
}

// TestListMembers_BrokenCache_FallsThroughToDB verifies ListMembers falls through.
func TestListMembers_BrokenCache_FallsThroughToDB(t *testing.T) {
	db := newOrgTestDB(t)
	mr, c := newMiniRedisCache(t)
	h := handler.NewOrgHandler(db, c)

	user := model.User{Email: "m@example.com", Name: "M"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOwner})
	mr.Close()

	root := model.RoleRoot
	ctx, rec := newOrgContext(t, db, http.MethodGet, fmt.Sprintf("/v1/api/orgs/%s/members", org.UUID), "", org.ID, 1, &root, "")
	if err := h.ListMembers(ctx); err != nil {
		t.Fatalf("ListMembers with broken cache: %v", err)
	}
	data := decodeOrgBody(t, rec)["data"].([]any)
	if len(data) != 1 {
		t.Errorf("len(data) = %d, want 1 from DB fallthrough", len(data))
	}
}

// TestCreateOrg_BrokenCacheInvalidation verifies CreateOrg succeeds even when
// the cache Del fails (covers the cache error log path after a successful write).
func TestCreateOrg_BrokenCacheInvalidation(t *testing.T) {
	db := newOrgTestDB(t)
	mr, c := newMiniRedisCache(t)
	h := handler.NewOrgHandler(db, c)

	// Populate the list cache so there's something to invalidate.
	root := model.RoleRoot
	listCtx, _ := newOrgContext(t, db, http.MethodGet, "/v1/api/orgs", "", 0, 1, &root, "")
	_ = h.ListOrgs(listCtx)

	// Kill cache — the Del call after CreateOrg will fail.
	mr.Close()

	createCtx, rec := newOrgContext(t, db, http.MethodPost, "/v1/api/orgs", `{"name":"New"}`, 0, 1, &root, "")
	if err := h.CreateOrg(createCtx); err != nil {
		t.Fatalf("CreateOrg with broken cache invalidation: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}

	var created model.Organization
	if err := db.Where("name = ?", "New").First(&created).Error; err != nil {
		t.Errorf("org 'New' not found in DB after CreateOrg: %v", err)
	}
}

// TestUpdateOrg_BrokenCacheInvalidation verifies UpdateOrg succeeds when Del fails.
func TestUpdateOrg_BrokenCacheInvalidation(t *testing.T) {
	db := newOrgTestDB(t)
	mr, c := newMiniRedisCache(t)
	h := handler.NewOrgHandler(db, c)

	org := model.Organization{Name: "Original"}
	db.Create(&org)

	// Populate the cache.
	root := model.RoleRoot
	getCtx, _ := newOrgContext(t, db, http.MethodGet, "/v1/api/orgs/"+org.UUID.String(), "", org.ID, 1, &root, "")
	_ = h.GetOrg(getCtx)
	mr.Close()

	updateCtx, rec := newOrgContext(t, db, http.MethodPut, "/v1/api/orgs/"+org.UUID.String(), `{"name":"Updated"}`, org.ID, 1, &root, "")
	if err := h.UpdateOrg(updateCtx); err != nil {
		t.Fatalf("UpdateOrg with broken cache: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var reloaded model.Organization
	if err := db.First(&reloaded, org.ID).Error; err != nil {
		t.Fatalf("reload org from DB: %v", err)
	}
	if reloaded.Name != "Updated" {
		t.Errorf("org name in DB = %q, want Updated", reloaded.Name)
	}
}

// TestDeleteOrg_BrokenCacheInvalidation verifies DeleteOrg succeeds when Del fails.
func TestDeleteOrg_BrokenCacheInvalidation(t *testing.T) {
	db := newOrgTestDB(t)
	mr, c := newMiniRedisCache(t)
	h := handler.NewOrgHandler(db, c)

	org := model.Organization{Name: "ToDelete"}
	db.Create(&org)
	mr.Close()

	root := model.RoleRoot
	ctx, rec := newOrgContext(t, db, http.MethodDelete, "/v1/api/orgs/"+org.UUID.String(), "", org.ID, 1, &root, "")
	if err := h.DeleteOrg(ctx); err != nil {
		t.Fatalf("DeleteOrg with broken cache: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}

	var gone model.Organization
	if err := db.First(&gone, org.ID).Error; err == nil {
		t.Errorf("org %d still found in DB after DeleteOrg", org.ID)
	}
}

// TestAddMember_BrokenCacheInvalidation verifies AddMember succeeds when Del fails.
func TestAddMember_BrokenCacheInvalidation(t *testing.T) {
	db := newOrgTestDB(t)
	mr, c := newMiniRedisCache(t)
	h := handler.NewOrgHandler(db, c)

	org := model.Organization{Name: "Org"}
	db.Create(&org)
	user := model.User{Email: "u@example.com", Name: "U"}
	db.Create(&user)

	// Populate member cache then break cache.
	root := model.RoleRoot
	listCtx, _ := newOrgContext(t, db, http.MethodGet, fmt.Sprintf("/v1/api/orgs/%s/members", org.UUID), "", org.ID, 1, &root, "")
	_ = h.ListMembers(listCtx)
	mr.Close()

	body := memberAddBody(user.UUID.String(), "owner")
	ctx, rec := newOrgContext(t, db, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%s/members", org.UUID), body, org.ID, 1, &root, "")
	if err := h.AddMember(ctx); err != nil {
		t.Fatalf("AddMember with broken cache: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}

	var member model.OrgMember
	if err := db.Where("org_id = ? AND user_id = ?", org.ID, user.ID).First(&member).Error; err != nil {
		t.Errorf("member not found in DB after AddMember: %v", err)
	}
}

// TestUpdateMember_BrokenCacheInvalidation verifies UpdateMember succeeds when Del fails.
func TestUpdateMember_BrokenCacheInvalidation(t *testing.T) {
	db := newOrgTestDB(t)
	mr, c := newMiniRedisCache(t)
	h := handler.NewOrgHandler(db, c)

	org := model.Organization{Name: "Org", Plan: model.PlanEnterprise}
	db.Create(&org)
	user := model.User{Email: "u@example.com", Name: "U"}
	db.Create(&user)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOwner})

	root := model.RoleRoot
	// Populate member cache then break cache.
	listCtx, _ := newOrgContext(t, db, http.MethodGet, fmt.Sprintf("/v1/api/orgs/%s/members", org.UUID), "", org.ID, 1, &root, "")
	_ = h.ListMembers(listCtx)
	mr.Close()

	body := `{"role":"admin"}`
	path := fmt.Sprintf("/v1/api/orgs/%s/members/%s", org.UUID, user.UUID)
	ctx, rec := newOrgContextWithUserID(t, db, http.MethodPut, path, body, org.ID, user.ID, 1, &root, "")
	if err := h.UpdateMember(ctx); err != nil {
		t.Fatalf("UpdateMember with broken cache: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var member model.OrgMember
	if err := db.Where("org_id = ? AND user_id = ?", org.ID, user.ID).First(&member).Error; err != nil {
		t.Fatalf("reload member from DB: %v", err)
	}
	if member.Role != model.OrgRoleAdmin {
		t.Errorf("member role in DB = %q, want admin", member.Role)
	}
}

// TestRemoveMember_BrokenCacheInvalidation verifies RemoveMember succeeds when Del fails.
func TestRemoveMember_BrokenCacheInvalidation(t *testing.T) {
	db := newOrgTestDB(t)
	mr, c := newMiniRedisCache(t)
	h := handler.NewOrgHandler(db, c)

	org := model.Organization{Name: "Org"}
	db.Create(&org)
	caller := model.User{Email: "owner@example.com", Name: "Owner"}
	db.Create(&caller)
	target := model.User{Email: "op@example.com", Name: "Op"}
	db.Create(&target)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: caller.ID, Role: model.OrgRoleOwner})
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: target.ID, Role: model.OrgRoleOperational})

	// Populate member cache then break cache.
	root := model.RoleRoot
	listCtx, _ := newOrgContext(t, db, http.MethodGet, fmt.Sprintf("/v1/api/orgs/%s/members", org.UUID), "", org.ID, 1, &root, "")
	_ = h.ListMembers(listCtx)
	mr.Close()

	path := fmt.Sprintf("/v1/api/orgs/%s/members/%s", org.UUID, target.UUID)
	ctx, rec := newOrgContextWithUserID(t, db, http.MethodDelete, path, "", org.ID, target.ID, caller.ID, &root, "")
	if err := h.RemoveMember(ctx); err != nil {
		t.Fatalf("RemoveMember with broken cache: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}

	var gone model.OrgMember
	if err := db.Where("org_id = ? AND user_id = ?", org.ID, target.ID).First(&gone).Error; err == nil {
		t.Errorf("member (user %d) still found in DB after RemoveMember", target.ID)
	}
}

// TestListMembers_CacheInvalidatedOnAdd verifies AddMember clears the member cache.
func TestListMembers_CacheInvalidatedOnAdd(t *testing.T) {
	db := newOrgTestDB(t)
	_, c := newMiniRedisCache(t)
	h := handler.NewOrgHandler(db, c)

	org := model.Organization{Name: "Org Inval"}
	db.Create(&org)
	existingUser := model.User{Email: "existing@example.com", Name: "Existing"}
	db.Create(&existingUser)
	newUser := model.User{Email: "new@example.com", Name: "New"}
	db.Create(&newUser)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: existingUser.ID, Role: model.OrgRoleOwner})

	root := model.RoleRoot
	callList := func() []any {
		ctx, rec := newOrgContext(t, db, http.MethodGet, fmt.Sprintf("/v1/api/orgs/%s/members", org.UUID), "", org.ID, 1, &root, "")
		if err := h.ListMembers(ctx); err != nil {
			t.Fatalf("ListMembers: %v", err)
		}
		return decodeOrgBody(t, rec)["data"].([]any)
	}

	// Populate cache with 1 member.
	if len(callList()) != 1 {
		t.Fatal("expected 1 member initially")
	}

	// Add a new member (should invalidate org:{id}:members).
	body := memberAddBody(newUser.UUID.String(), "operational")
	ctx, _ := newOrgContext(t, db, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%s/members", org.UUID), body, org.ID, 1, &root, "")
	if err := h.AddMember(ctx); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	// Next list call must see 2 members.
	if got := callList(); len(got) != 2 {
		t.Errorf("len after add = %d, want 2", len(got))
	}
}

// --- Plan limit enforcement ---

func TestAddMember_FreeOwnerLimit_Returns422(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	existing := model.User{Email: "owner@example.com", Name: "Owner"}
	db.Create(&existing)
	newUser := model.User{Email: "new@example.com", Name: "New"}
	db.Create(&newUser)
	org := model.Organization{Name: "Org", Plan: model.PlanFree}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: existing.ID, Role: model.OrgRoleOwner})

	sys := model.RoleSystem
	body := memberAddBody(newUser.UUID.String(), "owner")
	c, _ := newOrgContext(t, db, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%s/members", org.UUID), body, org.ID, 1, &sys, "")
	err := h.AddMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 HTTPError, got %v", err)
	}
}

func TestAddMember_FreeAdminNotAllowed_Returns422(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	newUser := model.User{Email: "new@example.com", Name: "New"}
	db.Create(&newUser)
	org := model.Organization{Name: "Org", Plan: model.PlanFree}
	db.Create(&org)

	sys := model.RoleSystem
	body := memberAddBody(newUser.UUID.String(), "admin")
	c, _ := newOrgContext(t, db, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%s/members", org.UUID), body, org.ID, 1, &sys, "")
	err := h.AddMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 HTTPError, got %v", err)
	}
}

func TestAddMember_FreeOperationalLimit_Returns422(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	org := model.Organization{Name: "Org", Plan: model.PlanFree}
	db.Create(&org)
	newUser := model.User{Email: "sixth@example.com", Name: "Sixth"}
	db.Create(&newUser)
	// Fill up to the 5-operational limit.
	for i := range 5 {
		u := model.User{Email: fmt.Sprintf("op%d@example.com", i), Name: fmt.Sprintf("Op%d", i)}
		db.Create(&u)
		db.Create(&model.OrgMember{OrgID: org.ID, UserID: u.ID, Role: model.OrgRoleOperational})
	}

	sys := model.RoleSystem
	body := memberAddBody(newUser.UUID.String(), "operational")
	c, _ := newOrgContext(t, db, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%s/members", org.UUID), body, org.ID, 1, &sys, "")
	err := h.AddMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 HTTPError, got %v", err)
	}
}

func TestAddMember_ProAdminLimit_Returns422(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	org := model.Organization{Name: "Org", Plan: model.PlanPro}
	db.Create(&org)
	newUser := model.User{Email: "fourth@example.com", Name: "Fourth"}
	db.Create(&newUser)
	// Fill up to the 3-admin limit.
	for i := range 3 {
		u := model.User{Email: fmt.Sprintf("admin%d@example.com", i), Name: fmt.Sprintf("Admin%d", i)}
		db.Create(&u)
		db.Create(&model.OrgMember{OrgID: org.ID, UserID: u.ID, Role: model.OrgRoleAdmin})
	}

	sys := model.RoleSystem
	body := memberAddBody(newUser.UUID.String(), "admin")
	c, _ := newOrgContext(t, db, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%s/members", org.UUID), body, org.ID, 1, &sys, "")
	err := h.AddMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 HTTPError, got %v", err)
	}
}

func TestAddMember_EnterpriseUnlimited_Success(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	org := model.Organization{Name: "Org", Plan: model.PlanEnterprise}
	db.Create(&org)
	// Add 20 operationals — well beyond any free/pro limit.
	for i := range 20 {
		u := model.User{Email: fmt.Sprintf("op%d@example.com", i), Name: fmt.Sprintf("Op%d", i)}
		db.Create(&u)
		db.Create(&model.OrgMember{OrgID: org.ID, UserID: u.ID, Role: model.OrgRoleOperational})
	}
	extra := model.User{Email: "extra@example.com", Name: "Extra"}
	db.Create(&extra)

	sys := model.RoleSystem
	body := memberAddBody(extra.UUID.String(), "operational")
	c, rec := newOrgContext(t, db, http.MethodPost, fmt.Sprintf("/v1/api/orgs/%s/members", org.UUID), body, org.ID, 1, &sys, "")
	if err := h.AddMember(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
}

func TestUpdateMember_FreeAdminNotAllowed_Returns422(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	user := model.User{Email: "op@example.com", Name: "Op"}
	db.Create(&user)
	org := model.Organization{Name: "Org", Plan: model.PlanFree}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOperational})

	sys := model.RoleSystem
	c, _ := newOrgContextWithUserID(t, db, http.MethodPut,
		fmt.Sprintf("/v1/api/orgs/%s/members/%s", org.UUID, user.UUID),
		`{"role":"admin"}`, org.ID, user.ID, 1, &sys, "")
	err := h.UpdateMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 HTTPError, got %v", err)
	}
}

func TestUpdateMember_ProLimitReached_Returns422(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db, cache.Disabled())
	org := model.Organization{Name: "Org", Plan: model.PlanPro}
	db.Create(&org)
	// Fill 3-admin limit.
	for i := range 3 {
		u := model.User{Email: fmt.Sprintf("admin%d@example.com", i), Name: fmt.Sprintf("Admin%d", i)}
		db.Create(&u)
		db.Create(&model.OrgMember{OrgID: org.ID, UserID: u.ID, Role: model.OrgRoleAdmin})
	}
	op := model.User{Email: "op@example.com", Name: "Op"}
	db.Create(&op)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: op.ID, Role: model.OrgRoleOperational})

	sys := model.RoleSystem
	c, _ := newOrgContextWithUserID(t, db, http.MethodPut,
		fmt.Sprintf("/v1/api/orgs/%s/members/%s", org.UUID, op.UUID),
		`{"role":"admin"}`, org.ID, op.ID, 1, &sys, "")
	err := h.UpdateMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 HTTPError, got %v", err)
	}
}
