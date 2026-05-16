package handler

// Internal tests — share package access with auth.go so the unexported
// Google-OAuth helpers (fetchGoogleUserInfo, findOrCreateGoogleUser) can be
// exercised directly. Public-API tests live in the *_test.go files in
// package handler_test.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"charity-chest/internal/cache"
	"charity-chest/internal/config"
	"charity-chest/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// roundTripperFunc adapts a function into an http.RoundTripper so tests can
// hijack http.DefaultClient.Transport without spinning up a real server.
type roundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip dispatches the call to the underlying function.
func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// newInternalTestDB mirrors the helper in auth_test.go but lives in package
// handler so internal tests can stand alone without importing handler_test.
func newInternalTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.PasswordResetToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// internalTestCfg returns a minimal config sufficient for testing handler
// internals — no Google credentials are required because the OAuth flow is
// stubbed at the HTTP layer.
func internalTestCfg() *config.Config {
	return &config.Config{
		JWTSecret:          "internal-test-secret",
		GoogleClientID:     "client",
		GoogleClientSecret: "secret",
		GoogleRedirectURL:  "http://localhost/cb",
		Port:               "8080",
	}
}

// --- fetchGoogleUserInfo ---

func TestFetchGoogleUserInfo_Success(t *testing.T) {
	old := http.DefaultClient.Transport
	t.Cleanup(func() { http.DefaultClient.Transport = old })

	var sawAuthHeader string
	http.DefaultClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		sawAuthHeader = r.Header.Get("Authorization")
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"id":"g-1","email":"u@example.com","name":"User"}`)),
			Header:     make(http.Header),
		}, nil
	})

	info, err := fetchGoogleUserInfo("the-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ID != "g-1" || info.Email != "u@example.com" || info.Name != "User" {
		t.Errorf("info = %+v", info)
	}
	if sawAuthHeader != "Bearer the-token" {
		t.Errorf("Authorization header = %q", sawAuthHeader)
	}
}

func TestFetchGoogleUserInfo_TransportError(t *testing.T) {
	old := http.DefaultClient.Transport
	t.Cleanup(func() { http.DefaultClient.Transport = old })

	wantErr := errors.New("network down")
	http.DefaultClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return nil, wantErr
	})

	if _, err := fetchGoogleUserInfo("t"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchGoogleUserInfo_InvalidJSON(t *testing.T) {
	old := http.DefaultClient.Transport
	t.Cleanup(func() { http.DefaultClient.Transport = old })

	http.DefaultClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`not json`)),
			Header:     make(http.Header),
		}, nil
	})

	if _, err := fetchGoogleUserInfo("t"); err == nil {
		t.Fatal("expected JSON decode error, got nil")
	}
}

// --- findOrCreateGoogleUser ---

func TestFindOrCreateGoogleUser_NewUser_Created(t *testing.T) {
	db := newInternalTestDB(t)
	h := NewAuthHandler(db, internalTestCfg(), cache.Disabled())

	got, err := h.findOrCreateGoogleUser(&googleUserInfo{
		ID:    "g-new",
		Email: "new@example.com",
		Name:  "New",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Email != "new@example.com" || got.GoogleID == nil || *got.GoogleID != "g-new" {
		t.Errorf("created user = %+v", got)
	}

	// And the row is persisted.
	var count int64
	db.Model(&model.User{}).Where("email = ?", "new@example.com").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 row in users; got %d", count)
	}
}

func TestFindOrCreateGoogleUser_ExistingByGoogleID_Found(t *testing.T) {
	db := newInternalTestDB(t)
	h := NewAuthHandler(db, internalTestCfg(), cache.Disabled())

	gid := "g-existing"
	pre := &model.User{Email: "pre@example.com", Name: "Pre", GoogleID: &gid}
	if err := db.Create(pre).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	got, err := h.findOrCreateGoogleUser(&googleUserInfo{
		ID:    "g-existing",
		Email: "pre@example.com",
		Name:  "Pre",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != pre.ID {
		t.Errorf("expected to reuse user id %d, got %d", pre.ID, got.ID)
	}
}

func TestFindOrCreateGoogleUser_ExistingByEmail_LinksGoogleID(t *testing.T) {
	db := newInternalTestDB(t)
	h := NewAuthHandler(db, internalTestCfg(), cache.Disabled())

	// User registered via email/password earlier — no Google ID yet.
	pre := &model.User{Email: "link@example.com", Name: "Link"}
	if err := db.Create(pre).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	got, err := h.findOrCreateGoogleUser(&googleUserInfo{
		ID:    "g-link",
		Email: "link@example.com",
		Name:  "Link",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != pre.ID {
		t.Errorf("expected to reuse user id %d, got %d", pre.ID, got.ID)
	}
	if got.GoogleID == nil || *got.GoogleID != "g-link" {
		t.Errorf("GoogleID = %v, want %q linked on existing email account", got.GoogleID, "g-link")
	}

	// Reload to confirm the link was persisted.
	var reloaded model.User
	if err := db.First(&reloaded, pre.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloaded.GoogleID == nil || *reloaded.GoogleID != "g-link" {
		t.Errorf("persisted GoogleID = %v", reloaded.GoogleID)
	}
}

// --- buildResetURL — exercise the invalid-locale fallback branch ---

func TestBuildResetURL_FallsBackToEN_OnUnknownLocale(t *testing.T) {
	db := newInternalTestDB(t)
	cfg := internalTestCfg()
	cfg.FrontendURL = "https://app.example.test"
	h := NewAuthHandler(db, cfg, cache.Disabled())

	url := h.buildResetURL("zz", "tok-abc")
	if !strings.Contains(url, "/en/reset-password?token=tok-abc") {
		t.Errorf("buildResetURL fallback URL = %q", url)
	}
}

func TestBuildResetURL_EnglishURL(t *testing.T) {
	db := newInternalTestDB(t)
	cfg := internalTestCfg()
	cfg.FrontendURL = "https://app.example.test"
	h := NewAuthHandler(db, cfg, cache.Disabled())

	url := h.buildResetURL("en", "abc")
	if !strings.HasPrefix(url, "https://app.example.test/en/reset-password?token=") {
		t.Errorf("buildResetURL EN = %q", url)
	}
}

func TestBuildResetURL_ItalianURL(t *testing.T) {
	db := newInternalTestDB(t)
	cfg := internalTestCfg()
	cfg.FrontendURL = "https://app.example.test"
	h := NewAuthHandler(db, cfg, cache.Disabled())

	url := h.buildResetURL("it", "abc")
	if !strings.HasPrefix(url, "https://app.example.test/it/reset-password?token=") {
		t.Errorf("buildResetURL IT = %q", url)
	}
}

// --- newGoMailMailer constructor ---

// TestNewGoMailMailer_PopulatesFields exercises the constructor path so it
// isn't 0% on the coverage report. We don't drive a real SMTP send (that
// would need a real or stub server); just verify the struct is built with
// auth disabled when credentials are absent.
func TestNewGoMailMailer_AuthDisabledWhenNoCreds(t *testing.T) {
	cfg := &config.Config{
		SMTPHost:     "mailhog",
		SMTPPort:     1025,
		SMTPFrom:     "no-reply@example.test",
		SMTPFromName: "Example",
	}
	m := newGoMailMailer(cfg)
	if m.host != "mailhog" || m.port != 1025 {
		t.Errorf("host/port = %s:%d", m.host, m.port)
	}
	if m.authSet {
		t.Errorf("authSet should be false when credentials are absent")
	}
}

func TestNewGoMailMailer_AuthEnabledWhenCredsPresent(t *testing.T) {
	cfg := &config.Config{
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		SMTPUsername: "user",
		SMTPPassword: "pass",
		SMTPFrom:     "no-reply@example.com",
	}
	m := newGoMailMailer(cfg)
	if !m.authSet {
		t.Errorf("authSet should be true when both credentials are present")
	}
}

// --- disabledMailer.Send ---

func TestDisabledMailer_Send_ReturnsErrMailerDisabled(t *testing.T) {
	err := disabledMailer{}.Send(context.TODO(), "to", "subj", "html", "text")
	if !errors.Is(err, ErrMailerDisabled) {
		t.Errorf("expected ErrMailerDisabled, got %v", err)
	}
}
