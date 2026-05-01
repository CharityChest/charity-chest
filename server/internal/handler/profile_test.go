package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"charity-chest/internal/handler"
	"charity-chest/internal/cache"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
)

// decodeProfileBody parses a response body into a generic map.
// Kept separate from decodeBody in auth_test.go to avoid duplicate declaration.
func decodeProfileBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return m
}

// makeUserForProfile creates a plain user in the DB and returns it.
func makeUserForProfile(t *testing.T, db *gorm.DB, email string) *model.User {
	t.Helper()
	user := &model.User{Email: email, Name: "Test User"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

// setUserMFA directly sets MFA fields on a user row in the DB.
func setUserMFA(t *testing.T, db *gorm.DB, userID uint, secret string, enabled bool) {
	t.Helper()
	db.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
		"totp_secret": secret,
		"mfa_enabled": enabled,
	})
}

// newProfileEchoContext creates an Echo context with the user_id injected.
func newProfileEchoContext(t *testing.T, method, path, body string, userID uint) (echo.Context, *httptest.ResponseRecorder) {
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
	c.Set(middleware.UserIDContextKey, userID)
	return c, rec
}

// --- SetupMFA ---

func TestSetupMFA_ReturnsURIAndSecret(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewProfileHandler(db, testCfg(), cache.Disabled())

	user := makeUserForProfile(t, db, "setup@example.com")
	c, rec := newProfileEchoContext(t, http.MethodGet, "/v1/api/profile/mfa/setup", "", user.ID)

	if err := h.SetupMFA(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	body := decodeProfileBody(t, rec)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'data' object")
	}
	if data["uri"] == nil || data["uri"] == "" {
		t.Error("uri missing in response")
	}
	if data["secret"] == nil || data["secret"] == "" {
		t.Error("secret missing in response")
	}
	if uri, _ := data["uri"].(string); !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Errorf("uri %q does not start with otpauth://totp/", uri)
	}

	var updated model.User
	db.First(&updated, user.ID)
	if updated.TOTPSecret == nil {
		t.Error("totp_secret not persisted to DB")
	}
	if updated.MFAEnabled {
		t.Error("mfa_enabled must remain false after setup — not yet verified")
	}
}

func TestSetupMFA_AlreadyEnabled_Returns409(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewProfileHandler(db, testCfg(), cache.Disabled())

	user := makeUserForProfile(t, db, "already@example.com")
	setUserMFA(t, db, user.ID, "JBSWY3DPEHPK3PXP", true)

	c, _ := newProfileEchoContext(t, http.MethodGet, "/v1/api/profile/mfa/setup", "", user.ID)
	err := h.SetupMFA(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusConflict {
		t.Errorf("expected 409 HTTPError, got %v", err)
	}
}

// --- EnableMFA ---

func TestEnableMFA_ValidCode_EnablesMFA(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewProfileHandler(db, testCfg(), cache.Disabled())

	user := makeUserForProfile(t, db, "enable@example.com")
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "test", AccountName: user.Email})
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	setUserMFA(t, db, user.ID, key.Secret(), false)

	code, err := totp.GenerateCode(key.Secret(), time.Now())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}

	c, rec := newProfileEchoContext(t, http.MethodPost, "/v1/api/profile/mfa/enable",
		`{"code":"`+code+`"}`, user.ID)

	if err := h.EnableMFA(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	body := decodeProfileBody(t, rec)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'data' object")
	}
	if data["mfa_enabled"] != true {
		t.Errorf("mfa_enabled = %v, want true", data["mfa_enabled"])
	}

	var updated model.User
	db.First(&updated, user.ID)
	if !updated.MFAEnabled {
		t.Error("mfa_enabled not persisted to DB")
	}
}

func TestEnableMFA_InvalidCode_Returns401(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewProfileHandler(db, testCfg(), cache.Disabled())

	user := makeUserForProfile(t, db, "badcode@example.com")
	setUserMFA(t, db, user.ID, "JBSWY3DPEHPK3PXP", false)

	c, _ := newProfileEchoContext(t, http.MethodPost, "/v1/api/profile/mfa/enable",
		`{"code":"000000"}`, user.ID)

	err := h.EnableMFA(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %v", err)
	}
}

func TestEnableMFA_NoSetup_Returns400(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewProfileHandler(db, testCfg(), cache.Disabled())

	user := makeUserForProfile(t, db, "nosetup@example.com")
	c, _ := newProfileEchoContext(t, http.MethodPost, "/v1/api/profile/mfa/enable",
		`{"code":"123456"}`, user.ID)

	err := h.EnableMFA(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %v", err)
	}
}

func TestEnableMFA_AlreadyEnabled_Returns409(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewProfileHandler(db, testCfg(), cache.Disabled())

	user := makeUserForProfile(t, db, "alreadyon@example.com")
	setUserMFA(t, db, user.ID, "JBSWY3DPEHPK3PXP", true)

	c, _ := newProfileEchoContext(t, http.MethodPost, "/v1/api/profile/mfa/enable",
		`{"code":"123456"}`, user.ID)

	err := h.EnableMFA(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusConflict {
		t.Errorf("expected 409, got %v", err)
	}
}

func TestEnableMFA_MissingCode_Returns400(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewProfileHandler(db, testCfg(), cache.Disabled())

	user := makeUserForProfile(t, db, "nocode@example.com")
	setUserMFA(t, db, user.ID, "JBSWY3DPEHPK3PXP", false)

	c, _ := newProfileEchoContext(t, http.MethodPost, "/v1/api/profile/mfa/enable",
		`{}`, user.ID)

	err := h.EnableMFA(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %v", err)
	}
}

// --- DisableMFA ---

func TestDisableMFA_ValidCode_DisablesMFA(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewProfileHandler(db, testCfg(), cache.Disabled())

	user := makeUserForProfile(t, db, "disable@example.com")
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "test", AccountName: user.Email})
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	setUserMFA(t, db, user.ID, key.Secret(), true)

	code, err := totp.GenerateCode(key.Secret(), time.Now())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}

	c, rec := newProfileEchoContext(t, http.MethodDelete, "/v1/api/profile/mfa",
		`{"code":"`+code+`"}`, user.ID)

	if err := h.DisableMFA(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	body := decodeProfileBody(t, rec)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'data' object")
	}
	if data["mfa_enabled"] != false {
		t.Errorf("mfa_enabled = %v, want false", data["mfa_enabled"])
	}

	var updated model.User
	db.First(&updated, user.ID)
	if updated.MFAEnabled {
		t.Error("mfa_enabled still true in DB after disable")
	}
	if updated.TOTPSecret != nil {
		t.Error("totp_secret not cleared in DB after disable")
	}
}

func TestDisableMFA_InvalidCode_Returns401(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewProfileHandler(db, testCfg(), cache.Disabled())

	user := makeUserForProfile(t, db, "disablebad@example.com")
	setUserMFA(t, db, user.ID, "JBSWY3DPEHPK3PXP", true)

	c, _ := newProfileEchoContext(t, http.MethodDelete, "/v1/api/profile/mfa",
		`{"code":"000000"}`, user.ID)

	err := h.DisableMFA(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %v", err)
	}
}

func TestDisableMFA_NotEnabled_Returns400(t *testing.T) {
	db := newTestDB(t)
	h := handler.NewProfileHandler(db, testCfg(), cache.Disabled())

	user := makeUserForProfile(t, db, "notmfa@example.com")
	c, _ := newProfileEchoContext(t, http.MethodDelete, "/v1/api/profile/mfa",
		`{"code":"123456"}`, user.ID)

	err := h.DisableMFA(c)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %v", err)
	}
}
