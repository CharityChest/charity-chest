package handler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charity-chest/internal/handler"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/glebarez/sqlite"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// newOrgTestDB opens an in-memory SQLite DB with all models needed for org tests.
func newOrgTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Organization{}, &model.OrgMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// newOrgContext creates an Echo context for org handler unit tests.
// orgID is set as the ":orgID" path parameter when non-zero.
func newOrgContext(t *testing.T, method, path, body string, orgID uint, userID uint, sysRole *model.AdministrativeRole, orgRole model.MemberRole) (echo.Context, *httptest.ResponseRecorder) {
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
		c.SetParamNames("orgID")
		c.SetParamValues(fmt.Sprintf("%d", orgID))
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

// newOrgContextWithUserID creates a context with orgID and userID params for member endpoints.
func newOrgContextWithUserID(t *testing.T, method, path, body string, orgID, targetUserID, callerUserID uint, sysRole *model.AdministrativeRole, orgRole model.MemberRole) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, rec := newOrgContext(t, method, path, body, orgID, callerUserID, sysRole, orgRole)
	names := c.ParamNames()
	values := c.ParamValues()
	c.SetParamNames(append(names, "userID")...)
	c.SetParamValues(append(values, fmt.Sprintf("%d", targetUserID))...)
	return c, rec
}

// decodeOrgBody decodes the response body into a generic map.
func decodeOrgBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	return decodeBody(t, rec) // reuse auth_test.go helper
}

// --- ListOrgs ---

func TestListOrgs_Empty(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db)

	root := model.RoleRoot
	c, rec := newOrgContext(t, http.MethodGet, "/v1/api/orgs", "", 0, 1, &root, "")
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
	h := handler.NewOrgHandler(db)
	db.Create(&model.Organization{Name: "Org A"})
	db.Create(&model.Organization{Name: "Org B"})

	root := model.RoleRoot
	c, rec := newOrgContext(t, http.MethodGet, "/v1/api/orgs", "", 0, 1, &root, "")
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
	h := handler.NewOrgHandler(db)

	root := model.RoleRoot
	c, rec := newOrgContext(t, http.MethodPost, "/v1/api/orgs", `{"name":"New Org"}`, 0, 1, &root, "")
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
	if data["id"] == nil {
		t.Error("response missing id")
	}

	var org model.Organization
	db.Where("name = ?", "New Org").First(&org)
	if org.ID == 0 {
		t.Error("org not persisted to DB")
	}
}

func TestCreateOrg_EmptyName_Returns400(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db)

	root := model.RoleRoot
	c, _ := newOrgContext(t, http.MethodPost, "/v1/api/orgs", `{"name":""}`, 0, 1, &root, "")
	err := h.CreateOrg(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400 HTTPError, got %v", err)
	}
}

// --- GetOrg ---

func TestGetOrg_Found(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db)
	org := model.Organization{Name: "Found Org"}
	db.Create(&org)

	root := model.RoleRoot
	c, rec := newOrgContext(t, http.MethodGet, "/v1/api/orgs/"+fmt.Sprintf("%d", org.ID), "", org.ID, 1, &root, "")
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
	h := handler.NewOrgHandler(db)

	root := model.RoleRoot
	c, _ := newOrgContext(t, http.MethodGet, "/v1/api/orgs/9999", "", 9999, 1, &root, "")
	err := h.GetOrg(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 HTTPError, got %v", err)
	}
}

// --- UpdateOrg ---

func TestUpdateOrg_UpdatesName(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db)
	org := model.Organization{Name: "Old Name"}
	db.Create(&org)

	root := model.RoleRoot
	c, rec := newOrgContext(t, http.MethodPut, "/v1/api/orgs/"+fmt.Sprintf("%d", org.ID),
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
	h := handler.NewOrgHandler(db)

	root := model.RoleRoot
	c, _ := newOrgContext(t, http.MethodPut, "/v1/api/orgs/9999", `{"name":"X"}`, 9999, 1, &root, "")
	err := h.UpdateOrg(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 HTTPError, got %v", err)
	}
}

// --- DeleteOrg ---

func TestDeleteOrg_Success(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db)
	org := model.Organization{Name: "ToDelete"}
	db.Create(&org)

	root := model.RoleRoot
	c, rec := newOrgContext(t, http.MethodDelete, "/v1/api/orgs/"+fmt.Sprintf("%d", org.ID), "", org.ID, 1, &root, "")
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
	h := handler.NewOrgHandler(db)

	root := model.RoleRoot
	c, _ := newOrgContext(t, http.MethodDelete, "/v1/api/orgs/9999", "", 9999, 1, &root, "")
	err := h.DeleteOrg(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 HTTPError, got %v", err)
	}
}

// --- ListMembers ---

func TestListMembers_Empty(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db)
	org := model.Organization{Name: "Empty Org"}
	db.Create(&org)

	root := model.RoleRoot
	c, rec := newOrgContext(t, http.MethodGet, "/v1/api/orgs/"+fmt.Sprintf("%d", org.ID)+"/members", "", org.ID, 1, &root, "")
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
	h := handler.NewOrgHandler(db)
	user := model.User{Email: "member@example.com", Name: "Member"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOwner})

	root := model.RoleRoot
	c, rec := newOrgContext(t, http.MethodGet, "/v1/api/orgs/"+fmt.Sprintf("%d", org.ID)+"/members", "", org.ID, 1, &root, "")
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
	h := handler.NewOrgHandler(db)
	user := model.User{Email: "new@example.com", Name: "New"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)

	sys := model.RoleSystem
	body := fmt.Sprintf(`{"user_id":%d,"role":"operational"}`, user.ID)
	c, rec := newOrgContext(t, http.MethodPost, "/v1/api/orgs/"+fmt.Sprintf("%d", org.ID)+"/members",
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
	h := handler.NewOrgHandler(db)
	ownerUser := model.User{Email: "owner@example.com", Name: "Owner"}
	db.Create(&ownerUser)
	targetUser := model.User{Email: "admin@example.com", Name: "Admin"}
	db.Create(&targetUser)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: ownerUser.ID, Role: model.OrgRoleOwner})

	body := fmt.Sprintf(`{"user_id":%d,"role":"admin"}`, targetUser.ID)
	// No system role; org_member_role = owner (injected by RequireOrgRole middleware in real flow).
	c, rec := newOrgContext(t, http.MethodPost, "/v1/api/orgs/"+fmt.Sprintf("%d", org.ID)+"/members",
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
	h := handler.NewOrgHandler(db)
	adminUser := model.User{Email: "admin@example.com", Name: "Admin"}
	db.Create(&adminUser)
	targetUser := model.User{Email: "target@example.com", Name: "Target"}
	db.Create(&targetUser)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: adminUser.ID, Role: model.OrgRoleAdmin})

	body := fmt.Sprintf(`{"user_id":%d,"role":"owner"}`, targetUser.ID)
	c, _ := newOrgContext(t, http.MethodPost, "/v1/api/orgs/"+fmt.Sprintf("%d", org.ID)+"/members",
		body, org.ID, adminUser.ID, nil, model.OrgRoleAdmin)
	err := h.AddMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusForbidden {
		t.Errorf("expected 403 HTTPError, got %v", err)
	}
}

func TestAddMember_InvalidRole_Returns400(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db)
	org := model.Organization{Name: "Org"}
	db.Create(&org)

	sys := model.RoleSystem
	c, _ := newOrgContext(t, http.MethodPost, "/v1/api/orgs/"+fmt.Sprintf("%d", org.ID)+"/members",
		`{"user_id":1,"role":"superadmin"}`, org.ID, 1, &sys, "")
	err := h.AddMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400 HTTPError, got %v", err)
	}
}

func TestAddMember_DuplicateMember_Returns409(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db)
	user := model.User{Email: "dup@example.com", Name: "Dup"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOperational})

	sys := model.RoleSystem
	body := fmt.Sprintf(`{"user_id":%d,"role":"operational"}`, user.ID)
	c, _ := newOrgContext(t, http.MethodPost, "/v1/api/orgs/"+fmt.Sprintf("%d", org.ID)+"/members",
		body, org.ID, 1, &sys, "")
	err := h.AddMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusConflict {
		t.Errorf("expected 409 HTTPError, got %v", err)
	}
}

// --- UpdateMember ---

func TestUpdateMember_Success(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db)
	user := model.User{Email: "op@example.com", Name: "Op"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOperational})

	sys := model.RoleSystem
	c, rec := newOrgContextWithUserID(t, http.MethodPut,
		fmt.Sprintf("/v1/api/orgs/%d/members/%d", org.ID, user.ID),
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
	h := handler.NewOrgHandler(db)
	org := model.Organization{Name: "Org"}
	db.Create(&org)

	sys := model.RoleSystem
	c, _ := newOrgContextWithUserID(t, http.MethodPut,
		fmt.Sprintf("/v1/api/orgs/%d/members/9999", org.ID),
		`{"role":"admin"}`, org.ID, 9999, 1, &sys, "")
	err := h.UpdateMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 HTTPError, got %v", err)
	}
}

func TestUpdateMember_InvalidRole_Returns400(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db)
	user := model.User{Email: "op@example.com", Name: "Op"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOperational})

	sys := model.RoleSystem
	c, _ := newOrgContextWithUserID(t, http.MethodPut,
		fmt.Sprintf("/v1/api/orgs/%d/members/%d", org.ID, user.ID),
		`{"role":"superadmin"}`, org.ID, user.ID, 1, &sys, "")
	err := h.UpdateMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400 HTTPError, got %v", err)
	}
}

// --- RemoveMember ---

func TestRemoveMember_Success(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db)
	user := model.User{Email: "op@example.com", Name: "Op"}
	db.Create(&user)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOperational})

	sys := model.RoleSystem
	c, rec := newOrgContextWithUserID(t, http.MethodDelete,
		fmt.Sprintf("/v1/api/orgs/%d/members/%d", org.ID, user.ID),
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
	h := handler.NewOrgHandler(db)
	org := model.Organization{Name: "Org"}
	db.Create(&org)

	sys := model.RoleSystem
	c, _ := newOrgContextWithUserID(t, http.MethodDelete,
		fmt.Sprintf("/v1/api/orgs/%d/members/9999", org.ID),
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
	h := handler.NewOrgHandler(db)
	ownerUser := model.User{Email: "owner@example.com", Name: "Owner"}
	db.Create(&ownerUser)
	targetUser := model.User{Email: "target@example.com", Name: "Target"}
	db.Create(&targetUser)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: ownerUser.ID, Role: model.OrgRoleOwner})

	// No org_member_role in context — handler must fall back to DB query.
	body := fmt.Sprintf(`{"user_id":%d,"role":"admin"}`, targetUser.ID)
	c, rec := newOrgContext(t, http.MethodPost, "/v1/api/orgs/"+fmt.Sprintf("%d", org.ID)+"/members",
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
	h := handler.NewOrgHandler(db)
	outsider := model.User{Email: "outsider@example.com", Name: "Outsider"}
	db.Create(&outsider)
	targetUser := model.User{Email: "target@example.com", Name: "Target"}
	db.Create(&targetUser)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	// outsider is NOT in the org.

	body := fmt.Sprintf(`{"user_id":%d,"role":"operational"}`, targetUser.ID)
	c, _ := newOrgContext(t, http.MethodPost, "/v1/api/orgs/"+fmt.Sprintf("%d", org.ID)+"/members",
		body, org.ID, outsider.ID, nil, "") // orgRole = "" triggers fallback, DB finds no member
	err := h.AddMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusForbidden {
		t.Errorf("expected 403 HTTPError, got %v", err)
	}
}

func TestRemoveMember_AdminCannotRemoveOwner_Returns403(t *testing.T) {
	db := newOrgTestDB(t)
	h := handler.NewOrgHandler(db)
	adminUser := model.User{Email: "admin@example.com", Name: "Admin"}
	db.Create(&adminUser)
	ownerUser := model.User{Email: "owner@example.com", Name: "Owner"}
	db.Create(&ownerUser)
	org := model.Organization{Name: "Org"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: adminUser.ID, Role: model.OrgRoleAdmin})
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: ownerUser.ID, Role: model.OrgRoleOwner})

	// Admin caller (no system role), targeting an owner.
	c, _ := newOrgContextWithUserID(t, http.MethodDelete,
		fmt.Sprintf("/v1/api/orgs/%d/members/%d", org.ID, ownerUser.ID),
		"", org.ID, ownerUser.ID, adminUser.ID, nil, model.OrgRoleAdmin)
	err := h.RemoveMember(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusForbidden {
		t.Errorf("expected 403 HTTPError, got %v", err)
	}
}
