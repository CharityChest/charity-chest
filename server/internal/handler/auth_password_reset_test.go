package handler_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"charity-chest/internal/cache"
	"charity-chest/internal/handler"
	ccmiddleware "charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// fakeMailerCall captures one Send invocation so tests can assert what landed
// in the email body without doing network IO.
type fakeMailerCall struct {
	To       string
	Subject  string
	HTMLBody string
	TextBody string
}

// fakeMailer is the test double for handler.MailerGateway. It records every
// Send and can optionally return a fixed error so failure paths are exercised.
type fakeMailer struct {
	mu    sync.Mutex
	calls []fakeMailerCall
	err   error
}

// Send records the call and returns the configured error (nil by default).
func (f *fakeMailer) Send(_ context.Context, to, subject, htmlBody, textBody string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeMailerCall{To: to, Subject: subject, HTMLBody: htmlBody, TextBody: textBody})
	return f.err
}

// snapshot returns a copy of the recorded calls; the copy keeps assertions
// race-free with the background goroutine in ForgotPassword.
func (f *fakeMailer) snapshot() []fakeMailerCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]fakeMailerCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// waitForCalls polls until at least n calls are recorded or the deadline
// passes. Required because the recovery email is dispatched from a goroutine
// off the handler hot path.
func (f *fakeMailer) waitForCalls(t *testing.T, n int) []fakeMailerCall {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snap := f.snapshot()
		if len(snap) >= n {
			return snap
		}
		time.Sleep(5 * time.Millisecond)
	}
	snap := f.snapshot()
	t.Fatalf("waitForCalls: expected %d mailer calls, got %d", n, len(snap))
	return snap
}

// assertNoCallsWithin polls mailer.snapshot() until the timeout elapses and
// fails the test as soon as any call is recorded. Replaces a fixed time.Sleep
// so the negative assertion stays deterministic under scheduler jitter without
// pessimistically slowing down the happy path.
func assertNoCallsWithin(t *testing.T, m *fakeMailer, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if snap := m.snapshot(); len(snap) != 0 {
			t.Fatalf("mailer called %d times; should never send", len(snap))
		}
		if !time.Now().Before(deadline) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// newServerWithMailer wires a fresh Echo + handler stack with an injectable
// mailer. The FrontendURL is set so the reset URL builder produces a value
// tests can assert on deterministically.
func newServerWithMailer(t *testing.T) (*echo.Echo, *handler.AuthHandler, *gorm.DB, *fakeMailer) {
	t.Helper()
	db := newTestDB(t)
	cfg := testCfg()
	cfg.FrontendURL = "https://app.example.test"
	mailer := &fakeMailer{}
	h := handler.NewAuthHandlerWithMailer(db, cfg, cache.Disabled(), mailer)

	e := echo.New()
	v1 := e.Group("/v1")
	auth := v1.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	auth.POST("/password/forgot", h.ForgotPassword)
	auth.POST("/password/reset", h.ResetPassword)

	return e, h, db, mailer
}

// hashToken matches the server's SHA-256 hex digest of a plaintext reset token.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// urlToken extracts the ?token=... parameter from a reset URL embedded in an
// email body. Handles both HTML and plaintext bodies.
func urlToken(t *testing.T, body string) string {
	t.Helper()
	idx := strings.Index(body, "?token=")
	if idx < 0 {
		t.Fatalf("body does not contain ?token=: %s", body)
	}
	rest := body[idx+len("?token="):]
	end := len(rest)
	for i, r := range rest {
		if r == '"' || r == '\'' || r == ' ' || r == '\n' || r == '\r' || r == '<' || r == '&' {
			end = i
			break
		}
	}
	tok, err := url.QueryUnescape(rest[:end])
	if err != nil {
		t.Fatalf("decode token: %v", err)
	}
	return tok
}

// mustJSON encodes v to JSON or fails the calling test.
func mustJSON(v any) string {
	out, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("mustJSON: %v", err))
	}
	return string(out)
}

// postJSONWithHeader posts a JSON body with extra headers — used to set
// X-Locale so localized URL building can be exercised.
func postJSONWithHeader(e *echo.Echo, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// --- ForgotPassword ---

func TestForgotPassword_UnknownEmail_ReturnsNoContent_NoMail(t *testing.T) {
	e, _, _, mailer := newServerWithMailer(t)
	rec := postJSON(e, "/v1/auth/password/forgot", `{"email":"ghost@example.com"}`)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	assertNoCallsWithin(t, mailer, 50*time.Millisecond)
}

func TestForgotPassword_KnownEmail_SendsEmailWithEnglishURL(t *testing.T) {
	e, _, _, mailer := newServerWithMailer(t)
	postJSON(e, "/v1/auth/register", `{"email":"recover@example.com","password":"password123","name":"Recovery User"}`)

	rec := postJSON(e, "/v1/auth/password/forgot", `{"email":"recover@example.com"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	calls := mailer.waitForCalls(t, 1)
	if calls[0].To != "recover@example.com" {
		t.Errorf("To = %q", calls[0].To)
	}
	if !strings.Contains(calls[0].HTMLBody, "https://app.example.test/en/reset-password?token=") {
		t.Errorf("HTMLBody missing English reset URL: %s", calls[0].HTMLBody)
	}
	if !strings.Contains(calls[0].TextBody, "https://app.example.test/en/reset-password?token=") {
		t.Errorf("TextBody missing English reset URL: %s", calls[0].TextBody)
	}
	if calls[0].Subject == "" {
		t.Error("Subject is empty")
	}
}

func TestForgotPassword_ItalianLocale_BuildsItalianURL(t *testing.T) {
	// To exercise the locale path we'd need the locale middleware in the test
	// stack. The minimal harness above wires only the auth routes. Add the
	// middleware here so X-Locale resolves to LocaleIT.
	db := newTestDB(t)
	cfg := testCfg()
	cfg.FrontendURL = "https://app.example.test"
	mailer := &fakeMailer{}
	h := handler.NewAuthHandlerWithMailer(db, cfg, cache.Disabled(), mailer)
	e := echo.New()
	e.Use(ccmiddleware.Locale())
	v1 := e.Group("/v1")
	v1.Group("/auth").POST("/register", h.Register)
	v1.Group("/auth").POST("/password/forgot", h.ForgotPassword)

	postJSON(e, "/v1/auth/register", `{"email":"it@example.com","password":"password123","name":"Italo"}`)
	rec := postJSONWithHeader(e, "/v1/auth/password/forgot", `{"email":"it@example.com"}`, map[string]string{"X-Locale": "it"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	calls := mailer.waitForCalls(t, 1)
	if !strings.Contains(calls[0].HTMLBody, "https://app.example.test/it/reset-password?token=") {
		t.Errorf("HTMLBody missing /it/ in URL: %s", calls[0].HTMLBody)
	}
}

func TestForgotPassword_ThrottlesRepeatedRequests(t *testing.T) {
	e, _, _, mailer := newServerWithMailer(t)
	postJSON(e, "/v1/auth/register", `{"email":"throttle@example.com","password":"password123","name":"Throttle"}`)

	postJSON(e, "/v1/auth/password/forgot", `{"email":"throttle@example.com"}`)
	mailer.waitForCalls(t, 1)

	postJSON(e, "/v1/auth/password/forgot", `{"email":"throttle@example.com"}`)
	time.Sleep(100 * time.Millisecond)

	if got := mailer.snapshot(); len(got) != 1 {
		t.Errorf("mailer called %d times; throttle window should suppress duplicates", len(got))
	}
}

func TestForgotPassword_GoogleOnlyUser_StillIssuesToken(t *testing.T) {
	e, _, db, mailer := newServerWithMailer(t)
	googleID := "google-uid-pw-reset"
	db.Create(&model.User{Email: "go@example.com", Name: "Go", GoogleID: &googleID})

	rec := postJSON(e, "/v1/auth/password/forgot", `{"email":"go@example.com"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	calls := mailer.waitForCalls(t, 1)
	if calls[0].To != "go@example.com" {
		t.Errorf("To = %q", calls[0].To)
	}
}

func TestForgotPassword_MissingEmail_Returns400(t *testing.T) {
	e, _, _, _ := newServerWithMailer(t)
	rec := postJSON(e, "/v1/auth/password/forgot", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestForgotPassword_MailerDisabled_StillReturns204(t *testing.T) {
	db := newTestDB(t)
	cfg := testCfg()
	cfg.FrontendURL = "https://app.example.test"
	h := handler.NewAuthHandler(db, cfg, cache.Disabled()) // cfg.SMTPHost == "" → disabled mailer

	e := echo.New()
	v1 := e.Group("/v1")
	v1.Group("/auth").POST("/password/forgot", h.ForgotPassword)

	db.Create(&model.User{Email: "disabled@example.com", Name: "User"})

	rec := postJSON(e, "/v1/auth/password/forgot", `{"email":"disabled@example.com"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

// --- ResetPassword ---

func TestResetPassword_HappyPath_LoginsWithNewPassword(t *testing.T) {
	e, _, db, mailer := newServerWithMailer(t)
	postJSON(e, "/v1/auth/register", `{"email":"happy@example.com","password":"oldpassword123","name":"Happy"}`)
	postJSON(e, "/v1/auth/password/forgot", `{"email":"happy@example.com"}`)

	calls := mailer.waitForCalls(t, 1)
	token := urlToken(t, calls[0].HTMLBody)

	body := mustJSON(map[string]string{"token": token, "password": "brandNewPass!"})
	rec := postJSON(e, "/v1/auth/password/reset", body)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("reset status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	rec = postJSON(e, "/v1/auth/login", `{"email":"happy@example.com","password":"oldpassword123"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("login with old password should be unauthorized, got %d", rec.Code)
	}
	rec = postJSON(e, "/v1/auth/login", `{"email":"happy@example.com","password":"brandNewPass!"}`)
	if rec.Code != http.StatusOK {
		t.Errorf("login with new password should be 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var tok model.PasswordResetToken
	if err := db.Where("token_hash = ?", hashToken(token)).First(&tok).Error; err != nil {
		t.Fatalf("look up token: %v", err)
	}
	if tok.UsedAt == nil {
		t.Error("token UsedAt should be set after a successful reset")
	}
}

func TestResetPassword_MFAFlag_PreservedAfterReset(t *testing.T) {
	e, _, db, mailer := newServerWithMailer(t)
	postJSON(e, "/v1/auth/register", `{"email":"mfa-reset@example.com","password":"oldpassword123","name":"MFA"}`)
	secret := "JBSWY3DPEHPK3PXP"
	db.Model(&model.User{}).Where("email = ?", "mfa-reset@example.com").Updates(map[string]any{
		"mfa_enabled": true,
		"totp_secret": secret,
	})

	postJSON(e, "/v1/auth/password/forgot", `{"email":"mfa-reset@example.com"}`)
	calls := mailer.waitForCalls(t, 1)
	token := urlToken(t, calls[0].HTMLBody)

	body := mustJSON(map[string]string{"token": token, "password": "newpass1234"})
	rec := postJSON(e, "/v1/auth/password/reset", body)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("reset status = %d, want 204", rec.Code)
	}

	var user model.User
	if err := db.Where("email = ?", "mfa-reset@example.com").First(&user).Error; err != nil {
		t.Fatalf("look up user: %v", err)
	}
	if !user.MFAEnabled {
		t.Error("MFAEnabled must remain true after password reset")
	}
	if user.TOTPSecret == nil || *user.TOTPSecret != secret {
		t.Errorf("TOTPSecret = %v, want %q", user.TOTPSecret, secret)
	}

	rec = postJSON(e, "/v1/auth/login", `{"email":"mfa-reset@example.com","password":"newpass1234"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d", rec.Code)
	}
	resp := decodeBody(t, rec)
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("response missing data")
	}
	if data["mfa_required"] != true {
		t.Errorf("mfa_required = %v, want true", data["mfa_required"])
	}
}

func TestResetPassword_InvalidToken_Returns400(t *testing.T) {
	e, _, _, _ := newServerWithMailer(t)
	body := mustJSON(map[string]string{"token": "not-a-real-token", "password": "newpass1234"})
	rec := postJSON(e, "/v1/auth/password/reset", body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestResetPassword_ExpiredToken_Returns400(t *testing.T) {
	e, _, db, _ := newServerWithMailer(t)
	postJSON(e, "/v1/auth/register", `{"email":"expired@example.com","password":"password123","name":"E"}`)

	var user model.User
	db.Where("email = ?", "expired@example.com").First(&user)

	raw := "expired-raw-token-fixture"
	db.Create(&model.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hashToken(raw),
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	})

	body := mustJSON(map[string]string{"token": raw, "password": "newpass1234"})
	rec := postJSON(e, "/v1/auth/password/reset", body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestResetPassword_UsedToken_Returns400(t *testing.T) {
	e, _, db, _ := newServerWithMailer(t)
	postJSON(e, "/v1/auth/register", `{"email":"used@example.com","password":"password123","name":"U"}`)

	var user model.User
	db.Where("email = ?", "used@example.com").First(&user)

	raw := "consumed-raw-token-fixture"
	now := time.Now()
	db.Create(&model.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hashToken(raw),
		ExpiresAt: time.Now().Add(time.Hour),
		UsedAt:    &now,
	})

	body := mustJSON(map[string]string{"token": raw, "password": "newpass1234"})
	rec := postJSON(e, "/v1/auth/password/reset", body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestResetPassword_ShortPassword_Returns400(t *testing.T) {
	e, _, _, _ := newServerWithMailer(t)
	body := mustJSON(map[string]string{"token": "anything", "password": "short"})
	rec := postJSON(e, "/v1/auth/password/reset", body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestResetPassword_MissingFields_Returns400(t *testing.T) {
	e, _, _, _ := newServerWithMailer(t)
	rec := postJSON(e, "/v1/auth/password/reset", `{"token":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestResetPassword_InvalidatesOtherTokensForSameUser(t *testing.T) {
	e, _, db, mailer := newServerWithMailer(t)
	postJSON(e, "/v1/auth/register", `{"email":"multi@example.com","password":"password123","name":"M"}`)

	postJSON(e, "/v1/auth/password/forgot", `{"email":"multi@example.com"}`)
	calls := mailer.waitForCalls(t, 1)
	firstToken := urlToken(t, calls[0].HTMLBody)

	var user model.User
	db.Where("email = ?", "multi@example.com").First(&user)
	secondRaw := "extra-token-for-defense-in-depth-test"
	db.Create(&model.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hashToken(secondRaw),
		ExpiresAt: time.Now().Add(time.Hour),
	})

	body := mustJSON(map[string]string{"token": firstToken, "password": "brandnewpass!"})
	rec := postJSON(e, "/v1/auth/password/reset", body)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("first reset status = %d, want 204", rec.Code)
	}

	var stillUnused int64
	db.Model(&model.PasswordResetToken{}).
		Where("user_id = ? AND used_at IS NULL", user.ID).
		Count(&stillUnused)
	if stillUnused != 0 {
		t.Errorf("expected zero unused tokens, got %d", stillUnused)
	}

	body = mustJSON(map[string]string{"token": secondRaw, "password": "anothernewpass1"})
	rec = postJSON(e, "/v1/auth/password/reset", body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("second reset should be 400, got %d", rec.Code)
	}
}

func TestResetPassword_ConcurrentSameToken_OnlyOneSucceeds(t *testing.T) {
	e, _, db, _ := newServerWithMailer(t)
	postJSON(e, "/v1/auth/register", `{"email":"race@example.com","password":"password123","name":"R"}`)

	var user model.User
	db.Where("email = ?", "race@example.com").First(&user)

	raw := "race-token-fixture-string"
	db.Create(&model.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hashToken(raw),
		ExpiresAt: time.Now().Add(time.Hour),
	})

	body := mustJSON(map[string]string{"token": raw, "password": "concurrentpass!"})
	var wg sync.WaitGroup
	results := make([]int, 4)
	for i := range results {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rec := postJSON(e, "/v1/auth/password/reset", body)
			results[idx] = rec.Code
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, code := range results {
		if code == http.StatusNoContent {
			successes++
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly 1 success, got %d (codes=%v)", successes, results)
	}
}
