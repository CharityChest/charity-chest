package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charity-chest/services/admin/backend/internal/cache"
	"charity-chest/services/admin/backend/internal/handler"
	"charity-chest/services/admin/backend/internal/middleware"
	"charity-chest/services/admin/backend/internal/model"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const testServiceKey = "test-service-key"

// newInternalServer wires up an Echo with the /v1/internal/* group mounted
// behind the ServiceKey middleware, mirroring production wiring exactly.
// Returns the Echo and the per-test DB so tests can seed rows directly.
func newInternalServer(t *testing.T) (*echo.Echo, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	cfg := testCfg()
	c := cache.Disabled()
	internal := handler.NewInternalHandler(db, c, cfg)

	e := echo.New()
	e.Use(middleware.Locale())
	v1 := e.Group("/v1")
	g := v1.Group("/internal", middleware.ServiceKey(testServiceKey))
	g.POST("/auth/login", internal.InternalLogin)
	g.POST("/auth/google", internal.InternalGoogleAuth)
	g.GET("/users/:userUUID", internal.InternalGetUser)
	return e, db
}

// sendInternal runs a request against /v1/internal/* with the given service
// key header (empty string skips the header entirely).
func sendInternal(e *echo.Echo, method, path, body, serviceKey string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if serviceKey != "" {
		req.Header.Set(middleware.ServiceKeyHeader, serviceKey)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func seedPasswordUser(t *testing.T, db *gorm.DB, email, password, name string, mfa bool) *model.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	hashStr := string(hash)
	u := &model.User{Email: email, PasswordHash: &hashStr, Name: name, MFAEnabled: mfa}
	if mfa {
		secret := "JBSWY3DPEHPK3PXP" // any non-empty seed is fine; tests don't validate TOTP here
		u.TOTPSecret = &secret
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	return u
}

// --- Service-key gating ---

func TestInternalAPI_MissingServiceKey_401(t *testing.T) {
	e, _ := newInternalServer(t)
	rec := sendInternal(e, http.MethodPost, "/v1/internal/auth/login", `{"email":"a@b.c","password":"x"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body: %s", rec.Code, rec.Body.String())
	}
}

func TestInternalAPI_WrongServiceKey_403(t *testing.T) {
	e, _ := newInternalServer(t)
	rec := sendInternal(e, http.MethodPost, "/v1/internal/auth/login", `{"email":"a@b.c","password":"x"}`, "wrong-key")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
}

// --- InternalLogin ---

func TestInternalLogin_OK(t *testing.T) {
	e, db := newInternalServer(t)
	seedPasswordUser(t, db, "alice@example.com", "password123", "Alice", false)

	rec := sendInternal(e, http.MethodPost, "/v1/internal/auth/login",
		`{"email":"alice@example.com","password":"password123"}`, testServiceKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data envelope missing: %v", body)
	}
	if got, _ := data["email"].(string); got != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", got)
	}
	if got, _ := data["mfa_enabled"].(bool); got {
		t.Errorf("mfa_enabled = true, want false")
	}
	if got, _ := data["uuid"].(string); got == "" {
		t.Errorf("uuid empty in response")
	}
	// Ensure password_hash never appears in the slim DTO.
	if _, leaked := data["password_hash"]; leaked {
		t.Error("password_hash leaked in slim DTO")
	}
}

func TestInternalLogin_WrongPassword_401(t *testing.T) {
	e, db := newInternalServer(t)
	seedPasswordUser(t, db, "bob@example.com", "correct", "Bob", false)

	rec := sendInternal(e, http.MethodPost, "/v1/internal/auth/login",
		`{"email":"bob@example.com","password":"wrong"}`, testServiceKey)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body: %s", rec.Code, rec.Body.String())
	}
}

func TestInternalLogin_UnknownUser_401(t *testing.T) {
	e, _ := newInternalServer(t)
	rec := sendInternal(e, http.MethodPost, "/v1/internal/auth/login",
		`{"email":"nobody@example.com","password":"whatever"}`, testServiceKey)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body: %s", rec.Code, rec.Body.String())
	}
}

func TestInternalLogin_GoogleOnlyAccount_401(t *testing.T) {
	e, db := newInternalServer(t)
	gid := "google-1"
	if err := db.Create(&model.User{Email: "carol@example.com", GoogleID: &gid, Name: "Carol"}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	rec := sendInternal(e, http.MethodPost, "/v1/internal/auth/login",
		`{"email":"carol@example.com","password":"anything"}`, testServiceKey)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body: %s", rec.Code, rec.Body.String())
	}
}

func TestInternalLogin_MFAEnabled_StillReturnsUserWithFlag(t *testing.T) {
	e, db := newInternalServer(t)
	seedPasswordUser(t, db, "dave@example.com", "password123", "Dave", true)

	rec := sendInternal(e, http.MethodPost, "/v1/internal/auth/login",
		`{"email":"dave@example.com","password":"password123"}`, testServiceKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	data := decodeBody(t, rec)["data"].(map[string]any)
	if got, _ := data["mfa_enabled"].(bool); !got {
		t.Errorf("mfa_enabled = false, want true")
	}
}

func TestInternalLogin_MissingFields_400(t *testing.T) {
	e, _ := newInternalServer(t)
	rec := sendInternal(e, http.MethodPost, "/v1/internal/auth/login",
		`{"email":"","password":""}`, testServiceKey)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

// --- InternalGoogleAuth ---

func TestInternalGoogleAuth_CreatesNewUser(t *testing.T) {
	e, db := newInternalServer(t)
	rec := sendInternal(e, http.MethodPost, "/v1/internal/auth/google",
		`{"google_sub":"sub-new","email":"new@example.com","name":"New User"}`, testServiceKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	data := decodeBody(t, rec)["data"].(map[string]any)
	if got, _ := data["email"].(string); got != "new@example.com" {
		t.Errorf("email = %q, want new@example.com", got)
	}
	var u model.User
	if err := db.Where("google_id = ?", "sub-new").First(&u).Error; err != nil {
		t.Fatalf("expected user with sub-new google_id, got: %v", err)
	}
}

func TestInternalGoogleAuth_LinksExistingEmail(t *testing.T) {
	e, db := newInternalServer(t)
	seedPasswordUser(t, db, "linked@example.com", "pw", "Linked", false)

	rec := sendInternal(e, http.MethodPost, "/v1/internal/auth/google",
		`{"google_sub":"sub-link","email":"linked@example.com","name":"Linked"}`, testServiceKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var got model.User
	if err := db.Where("email = ?", "linked@example.com").First(&got).Error; err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got.GoogleID == nil || *got.GoogleID != "sub-link" {
		t.Errorf("google_id not linked: %v", got.GoogleID)
	}
}

func TestInternalGoogleAuth_FindsExistingBySub(t *testing.T) {
	e, db := newInternalServer(t)
	gid := "sub-existing"
	want := &model.User{Email: "sub@example.com", GoogleID: &gid, Name: "Sub"}
	if err := db.Create(want).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := sendInternal(e, http.MethodPost, "/v1/internal/auth/google",
		`{"google_sub":"sub-existing","email":"sub@example.com","name":"Sub"}`, testServiceKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	data := decodeBody(t, rec)["data"].(map[string]any)
	if got, _ := data["uuid"].(string); got != want.UUID.String() {
		t.Errorf("uuid = %q, want %q", got, want.UUID.String())
	}
}

func TestInternalGoogleAuth_MissingFields_400(t *testing.T) {
	e, _ := newInternalServer(t)
	rec := sendInternal(e, http.MethodPost, "/v1/internal/auth/google",
		`{"google_sub":"","email":""}`, testServiceKey)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

// --- InternalGetUser ---

func TestInternalGetUser_OK(t *testing.T) {
	e, db := newInternalServer(t)
	u := &model.User{Email: "find@example.com", Name: "Find"}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := sendInternal(e, http.MethodGet, "/v1/internal/users/"+u.UUID.String(), "", testServiceKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	data := decodeBody(t, rec)["data"].(map[string]any)
	if got, _ := data["email"].(string); got != "find@example.com" {
		t.Errorf("email = %q, want find@example.com", got)
	}
}

func TestInternalGetUser_NotFound_404(t *testing.T) {
	e, _ := newInternalServer(t)
	rec := sendInternal(e, http.MethodGet, "/v1/internal/users/"+uuid.New().String(), "", testServiceKey)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

func TestInternalGetUser_BadUUID_400(t *testing.T) {
	e, _ := newInternalServer(t)
	rec := sendInternal(e, http.MethodGet, "/v1/internal/users/not-a-uuid", "", testServiceKey)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}
