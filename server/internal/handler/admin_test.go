package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"charity-chest/internal/cache"
	"charity-chest/internal/handler"
	"charity-chest/internal/model"

	"github.com/glebarez/sqlite"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// newAdminTestDB opens an in-memory SQLite DB with all models needed for admin tests.
func newAdminTestDB(t *testing.T) *gorm.DB {
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

// newAdminContext creates an Echo context with optional query-string parameters.
func newAdminContext(t *testing.T, query string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	path := "/v1/api/admin/users"
	if query != "" {
		path = path + "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	return c, rec
}

// decodeAdminBody decodes the full paginated response body.
func decodeAdminBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return m
}

func createUser(t *testing.T, db *gorm.DB, email string) *model.User {
	t.Helper()
	u := &model.User{Email: email, Name: "Test"}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return u
}

// --- SearchUsers ---

func TestSearchUsers_ReturnsAllUsers(t *testing.T) {
	db := newAdminTestDB(t)
	h := handler.NewAdminHandler(db, cache.Disabled())

	createUser(t, db, "alice@example.com")
	createUser(t, db, "bob@example.com")

	c, rec := newAdminContext(t, "page=1&size=20")
	if err := h.SearchUsers(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	body := decodeAdminBody(t, rec)
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("response missing 'data' array; body: %s", rec.Body.String())
	}
	if len(data) != 2 {
		t.Errorf("len(data) = %d, want 2", len(data))
	}

	meta, ok := body["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("response missing 'metadata' object")
	}
	if meta["total"].(float64) != 2 {
		t.Errorf("metadata.total = %v, want 2", meta["total"])
	}
	if meta["page"].(float64) != 1 {
		t.Errorf("metadata.page = %v, want 1", meta["page"])
	}
}

func TestSearchUsers_EmailFilter(t *testing.T) {
	db := newAdminTestDB(t)
	h := handler.NewAdminHandler(db, cache.Disabled())

	createUser(t, db, "alice@example.com")
	createUser(t, db, "bob@example.com")

	c, rec := newAdminContext(t, "email=alice")
	if err := h.SearchUsers(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := decodeAdminBody(t, rec)
	data := body["data"].([]any)
	if len(data) != 1 {
		t.Errorf("len(data) = %d, want 1", len(data))
	}
	user := data[0].(map[string]any)
	if user["email"] != "alice@example.com" {
		t.Errorf("email = %v, want alice@example.com", user["email"])
	}

	meta := body["metadata"].(map[string]any)
	if meta["total"].(float64) != 1 {
		t.Errorf("metadata.total = %v, want 1", meta["total"])
	}
}

func TestSearchUsers_Pagination(t *testing.T) {
	db := newAdminTestDB(t)
	h := handler.NewAdminHandler(db, cache.Disabled())

	for i := range 5 {
		createUser(t, db, fmt.Sprintf("user%d@example.com", i))
	}

	c, rec := newAdminContext(t, "page=2&size=2")
	if err := h.SearchUsers(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := decodeAdminBody(t, rec)
	data := body["data"].([]any)
	if len(data) != 2 {
		t.Errorf("len(data) = %d, want 2", len(data))
	}

	meta := body["metadata"].(map[string]any)
	if meta["page"].(float64) != 2 {
		t.Errorf("metadata.page = %v, want 2", meta["page"])
	}
	if meta["total"].(float64) != 5 {
		t.Errorf("metadata.total = %v, want 5", meta["total"])
	}
}

func TestSearchUsers_PaginationMeta(t *testing.T) {
	db := newAdminTestDB(t)
	h := handler.NewAdminHandler(db, cache.Disabled())

	for i := range 7 {
		createUser(t, db, fmt.Sprintf("u%d@example.com", i))
	}

	c, rec := newAdminContext(t, "page=1&size=3")
	if err := h.SearchUsers(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	meta := decodeAdminBody(t, rec)["metadata"].(map[string]any)
	if meta["total"].(float64) != 7 {
		t.Errorf("total = %v, want 7", meta["total"])
	}
	if meta["total_pages"].(float64) != 3 {
		t.Errorf("total_pages = %v, want 3", meta["total_pages"])
	}
	if meta["size"].(float64) != 3 {
		t.Errorf("size = %v, want 3", meta["size"])
	}
}

func TestSearchUsers_IncludesOrgMemberships(t *testing.T) {
	db := newAdminTestDB(t)
	h := handler.NewAdminHandler(db, cache.Disabled())

	user := createUser(t, db, "member@example.com")
	org := model.Organization{Name: "Acme"}
	db.Create(&org)
	db.Create(&model.OrgMember{OrgID: org.ID, UserID: user.ID, Role: model.OrgRoleOwner})

	c, rec := newAdminContext(t, "")
	if err := h.SearchUsers(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := decodeAdminBody(t, rec)
	data := body["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("len(data) = %d, want 1", len(data))
	}
	u := data[0].(map[string]any)
	orgs, ok := u["organizations"].([]any)
	if !ok || len(orgs) != 1 {
		t.Fatalf("organizations = %v, want 1 entry", u["organizations"])
	}
	o := orgs[0].(map[string]any)
	if o["name"] != "Acme" {
		t.Errorf("org.name = %v, want Acme", o["name"])
	}
	if o["role"] != string(model.OrgRoleOwner) {
		t.Errorf("org.role = %v, want %s", o["role"], model.OrgRoleOwner)
	}
}

func TestSearchUsers_EmptyResults(t *testing.T) {
	db := newAdminTestDB(t)
	h := handler.NewAdminHandler(db, cache.Disabled())

	c, rec := newAdminContext(t, "email=nobody")
	if err := h.SearchUsers(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := decodeAdminBody(t, rec)
	data := body["data"].([]any)
	if len(data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(data))
	}
	meta := body["metadata"].(map[string]any)
	if meta["total"].(float64) != 0 {
		t.Errorf("total = %v, want 0", meta["total"])
	}
	if meta["total_pages"].(float64) != 1 {
		t.Errorf("total_pages = %v, want 1 (min)", meta["total_pages"])
	}
}

func TestSearchUsers_SizeClampedAt100(t *testing.T) {
	db := newAdminTestDB(t)
	h := handler.NewAdminHandler(db, cache.Disabled())

	for i := range 5 {
		createUser(t, db, fmt.Sprintf("x%d@example.com", i))
	}

	c, rec := newAdminContext(t, "size=999")
	if err := h.SearchUsers(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	meta := decodeAdminBody(t, rec)["metadata"].(map[string]any)
	if meta["size"].(float64) != 100 {
		t.Errorf("size = %v, want 100 (clamped)", meta["size"])
	}
}

// --- Cache paths ---

// callSearchUsers fires SearchUsers with the given query string.
func callSearchUsers(t *testing.T, h *handler.AdminHandler, query string) map[string]any {
	t.Helper()
	c, rec := newAdminContext(t, query)
	if err := h.SearchUsers(c); err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	return decodeAdminBody(t, rec)
}

// TestSearchUsers_CacheHit verifies the second call is served from cache even
// after users are deleted from the DB.
func TestSearchUsers_CacheHit(t *testing.T) {
	db := newAdminTestDB(t)
	_, c := newMiniRedisCache(t)
	h := handler.NewAdminHandler(db, c)

	createUser(t, db, "cached@example.com")

	// First call: cache miss → reads DB → stores result.
	body1 := callSearchUsers(t, h, "")
	if body1["metadata"].(map[string]any)["total"].(float64) != 1 {
		t.Fatal("expected total=1 on first call")
	}

	// Delete all users from DB.
	db.Exec("DELETE FROM users")

	// Second call: cache hit → still reports 1 user.
	body2 := callSearchUsers(t, h, "")
	if body2["metadata"].(map[string]any)["total"].(float64) != 1 {
		t.Errorf("expected total=1 from cache, got %v", body2["metadata"].(map[string]any)["total"])
	}
	data := body2["data"].([]any)
	if len(data) != 1 {
		t.Errorf("len(data) from cache = %d, want 1", len(data))
	}
}

// TestSearchUsers_CacheMiss_FallsThroughToDB verifies a broken cache falls through to DB.
func TestSearchUsers_CacheMiss_FallsThroughToDB(t *testing.T) {
	db := newAdminTestDB(t)
	mr, c := newMiniRedisCache(t)
	h := handler.NewAdminHandler(db, c)

	createUser(t, db, "live@example.com")

	// Stop miniredis — handler must fall through to DB and return live data.
	mr.Close()

	body := callSearchUsers(t, h, "")
	if body["metadata"].(map[string]any)["total"].(float64) != 1 {
		t.Errorf("expected DB fallthrough total=1, got %v", body["metadata"].(map[string]any)["total"])
	}
}
