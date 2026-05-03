package handler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charity-chest/internal/cache"
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
	h := handler.NewSystemHandler(db, cache.Disabled())

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
	h := handler.NewSystemHandler(db, cache.Disabled())
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
	h := handler.NewSystemHandler(db, cache.Disabled())
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
	h := handler.NewSystemHandler(db, cache.Disabled())
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
	h := handler.NewSystemHandler(db, cache.Disabled())
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
	h := handler.NewSystemHandler(db, cache.Disabled())

	body := `{"user_id":99999,"role":"system"}`
	c, _ := newSystemContext(t, http.MethodPost, "/v1/api/system/assign-role", body)
	err := h.AssignSystemRole(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 HTTPError, got %v", err)
	}
}

func TestAssignSystemRole_CannotModifyRoot_Returns403(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewSystemHandler(db, cache.Disabled())
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
	h := handler.NewSystemHandler(db, cache.Disabled())
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
	h := handler.NewSystemHandler(db, cache.Disabled())
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

// --- Cache paths ---

func callSystemStatus(t *testing.T, h *handler.SystemHandler) map[string]any {
	t.Helper()
	c, rec := newSystemContext(t, http.MethodGet, "/v1/system/status", "")
	if err := h.SystemStatus(c); err != nil {
		t.Fatalf("SystemStatus: %v", err)
	}
	return decodeBody(t, rec)["data"].(map[string]any)
}

// TestSystemStatus_FalseIsNeverCached verifies that configured=false is never
// stored in the cache, so a subsequent call after seed-root creates the root user
// immediately reflects the real DB state instead of serving a stale response.
func TestSystemStatus_FalseIsNeverCached(t *testing.T) {
	db := newTestDB(t)
	_, c := newMiniRedisCache(t)
	h := handler.NewSystemHandler(db, c)

	// First call: no root → configured=false, must NOT be cached.
	data := callSystemStatus(t, h)
	if data["configured"] != false {
		t.Fatalf("expected configured=false before root exists")
	}

	// Simulate seed-root writing directly to the DB (no cache interaction).
	role := model.RoleRoot
	db.Create(&model.User{Email: "root@example.com", Name: "Root", Role: &role})

	// Second call: must reflect the new DB state, not a stale cached false.
	data = callSystemStatus(t, h)
	if data["configured"] != true {
		t.Errorf("configured = %v after root created; want true (false must not be cached)", data["configured"])
	}
}

// TestSystemStatus_TrueIsCachedAndServedOnHit verifies that configured=true is
// cached and subsequent calls are served from cache without hitting the DB.
func TestSystemStatus_TrueIsCachedAndServedOnHit(t *testing.T) {
	db := newTestDB(t)
	_, c := newMiniRedisCache(t)
	h := handler.NewSystemHandler(db, c)

	role := model.RoleRoot
	db.Create(&model.User{Email: "root@example.com", Name: "Root", Role: &role})

	// First call: configured=true → stored in cache.
	data := callSystemStatus(t, h)
	if data["configured"] != true {
		t.Fatalf("expected configured=true")
	}

	// Delete the root user from DB to prove the next call is served from cache.
	db.Exec("DELETE FROM users")

	// Second call: cache hit → still returns configured=true.
	data = callSystemStatus(t, h)
	if data["configured"] != true {
		t.Errorf("cache hit should return true, got %v", data["configured"])
	}
}

// TestAssignSystemRole_BrokenCacheInvalidation verifies the role assignment succeeds
// even when cache invalidation fails (covers cache error log paths in AssignSystemRole).
func TestAssignSystemRole_BrokenCacheInvalidation(t *testing.T) {
	db := newTestDB(t)
	mr, c := newMiniRedisCache(t)
	h := handler.NewSystemHandler(db, c)

	target := &model.User{Email: "target@example.com", Name: "Target"}
	db.Create(target)

	// Break cache before the write so Del/DelPattern will fail.
	mr.Close()

	body := `{"user_id":` + uid(target.ID) + `,"role":"system"}`
	ctx, rec := newSystemContext(t, http.MethodPost, "/v1/api/system/assign-role", body)
	if err := h.AssignSystemRole(ctx); err != nil {
		t.Fatalf("AssignSystemRole with broken cache: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var updated model.User
	if err := db.First(&updated, target.ID).Error; err != nil {
		t.Fatalf("reload user from DB: %v", err)
	}
	if updated.Role == nil || *updated.Role != model.RoleSystem {
		t.Errorf("DB role = %v, want system", updated.Role)
	}
}

// TestSystemStatus_CacheMiss_FallsThroughToDB verifies cache error falls through.
func TestSystemStatus_CacheMiss_FallsThroughToDB(t *testing.T) {
	db := newTestDB(t)
	mr, c := newMiniRedisCache(t)
	h := handler.NewSystemHandler(db, c)

	role := model.RoleRoot
	db.Create(&model.User{Email: "root@example.com", Name: "Root", Role: &role})

	// Kill cache before first call → fall through to DB.
	mr.Close()

	data := callSystemStatus(t, h)
	if data["configured"] != true {
		t.Errorf("expected DB fallthrough to return configured=true, got %v", data["configured"])
	}
}
