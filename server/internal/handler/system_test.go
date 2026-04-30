package handler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charity-chest/internal/handler"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
)

// newSystemContext creates an Echo context for system handler unit tests.
func newSystemContext(t *testing.T, method, path, body string) (echo.Context, *httptest.ResponseRecorder) {
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
	return c, rec
}

// --- SystemStatus ---

func TestSystemStatus_Unconfigured(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewSystemHandler(db)

	c, rec := newSystemContext(t, http.MethodGet, "/v1/system/status", "")
	if err := h.SystemStatus(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decodeBody(t, rec)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'data' object")
	}
	if data["configured"] != false {
		t.Errorf("configured = %v, want false", data["configured"])
	}
}

func TestSystemStatus_Configured(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewSystemHandler(db)
	role := model.RoleRoot
	db.Create(&model.User{Email: "root@example.com", Name: "Root", Role: &role})

	c, rec := newSystemContext(t, http.MethodGet, "/v1/system/status", "")
	if err := h.SystemStatus(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body := decodeBody(t, rec)
	data := body["data"].(map[string]any)
	if data["configured"] != true {
		t.Errorf("configured = %v, want true", data["configured"])
	}
}

func TestSystemStatus_SystemRoleNotCounted(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewSystemHandler(db)
	role := model.RoleSystem
	db.Create(&model.User{Email: "sys@example.com", Name: "System", Role: &role})

	c, rec := newSystemContext(t, http.MethodGet, "/v1/system/status", "")
	if err := h.SystemStatus(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body := decodeBody(t, rec)
	data := body["data"].(map[string]any)
	// A system-role user does not configure the system — only root does.
	if data["configured"] != false {
		t.Errorf("configured = %v, want false (system role does not count)", data["configured"])
	}
}

// --- AssignSystemRole ---

func TestAssignSystemRole_AssignsSystemRole(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewSystemHandler(db)
	target := &model.User{Email: "target@example.com", Name: "Target"}
	db.Create(target)

	body := `{"user_id":` + uid(target.ID) + `,"role":"system"}`
	c, rec := newSystemContext(t, http.MethodPost, "/v1/api/system/assign-role", body)
	if err := h.AssignSystemRole(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	resp := decodeBody(t, rec)
	data := resp["data"].(map[string]any)
	if data["role"] != "system" {
		t.Errorf("role = %v, want system", data["role"])
	}

	var updated model.User
	db.First(&updated, target.ID)
	if updated.Role == nil || *updated.Role != model.RoleSystem {
		t.Errorf("DB role = %v, want system", updated.Role)
	}
}

func TestAssignSystemRole_ClearsRole(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewSystemHandler(db)
	role := model.RoleSystem
	target := &model.User{Email: "sys@example.com", Name: "Sys", Role: &role}
	db.Create(target)

	body := `{"user_id":` + uid(target.ID) + `,"role":""}`
	c, _ := newSystemContext(t, http.MethodPost, "/v1/api/system/assign-role", body)
	if err := h.AssignSystemRole(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated model.User
	db.First(&updated, target.ID)
	if updated.Role != nil {
		t.Errorf("role = %v, want nil after clearing", updated.Role)
	}
}

func TestAssignSystemRole_UserNotFound_Returns404(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewSystemHandler(db)

	body := `{"user_id":99999,"role":"system"}`
	c, _ := newSystemContext(t, http.MethodPost, "/v1/api/system/assign-role", body)
	err := h.AssignSystemRole(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 HTTPError, got %v", err)
	}
}

func TestAssignSystemRole_CannotModifyRoot_Returns403(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewSystemHandler(db)
	role := model.RoleRoot
	target := &model.User{Email: "root@example.com", Name: "Root", Role: &role}
	db.Create(target)

	body := `{"user_id":` + uid(target.ID) + `,"role":"system"}`
	c, _ := newSystemContext(t, http.MethodPost, "/v1/api/system/assign-role", body)
	err := h.AssignSystemRole(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusForbidden {
		t.Errorf("expected 403 HTTPError, got %v", err)
	}
}

func TestAssignSystemRole_InvalidRole_Returns400(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewSystemHandler(db)
	target := &model.User{Email: "t@example.com", Name: "T"}
	db.Create(target)

	body := `{"user_id":` + uid(target.ID) + `,"role":"owner"}`
	c, _ := newSystemContext(t, http.MethodPost, "/v1/api/system/assign-role", body)
	err := h.AssignSystemRole(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400 HTTPError, got %v", err)
	}
}

func TestAssignSystemRole_RootRoleNotAssignable_Returns400(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewSystemHandler(db)
	target := &model.User{Email: "t@example.com", Name: "T"}
	db.Create(target)

	// "root" is not assignable via API — only "system" and "" are.
	body := `{"user_id":` + uid(target.ID) + `,"role":"root"}`
	c, _ := newSystemContext(t, http.MethodPost, "/v1/api/system/assign-role", body)
	err := h.AssignSystemRole(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400 HTTPError, got %v", err)
	}
}

// uid formats a uint as a decimal string for use in JSON request bodies.
func uid(n uint) string {
	return fmt.Sprintf("%d", n)
}
