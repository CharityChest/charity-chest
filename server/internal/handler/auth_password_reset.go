package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"

	"charity-chest/internal/cache"
	"charity-chest/internal/i18n"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Password recovery tuning constants.
const (
	// passwordResetTokenTTL is how long a reset link remains valid after it is
	// issued. One hour is the industry baseline (GitHub, Stripe, AWS) and
	// balances usability against the window an intercepted link is usable.
	passwordResetTokenTTL = 1 * time.Hour

	// passwordResetMailerTimeout caps the time we'll spend trying to deliver a
	// recovery email. We dial, send, and tear down inside this budget; longer
	// dials silently time out and the error is logged.
	passwordResetMailerTimeout = 30 * time.Second

	// passwordResetThrottleWindow is the minimum time between two outbound
	// emails for the same user. It prevents an attacker from using the
	// endpoint as an email bomb against a victim — repeat requests within
	// the window are silently no-oped.
	passwordResetThrottleWindow = 60 * time.Second

	// passwordResetTokenBytes is the entropy of the URL token. 32 bytes →
	// 256 bits, base64url-encoded into 43 characters. Way above the
	// 128-bit minimum we'd need to make brute force pointless.
	passwordResetTokenBytes = 32
)

// forgotPasswordRequest is the JSON body for POST /auth/password/forgot.
type forgotPasswordRequest struct {
	Email string `json:"email"`
}

// resetPasswordRequest is the JSON body for POST /auth/password/reset.
type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// ForgotPassword godoc
// POST /v1/auth/password/forgot
//
// Initiates a self-service password recovery. The endpoint is intentionally
// enumeration-safe: it always returns 204 No Content regardless of whether
// the supplied email maps to an account. When the email does match a user we
// store a SHA-256 hash of a freshly-generated token and dispatch the
// plaintext token over email; the email is sent in a background goroutine so
// the response time does not depend on SMTP latency (another enumeration
// signal). Throttling: if an unused, unexpired token was issued for the same
// user in the last passwordResetThrottleWindow we skip the send silently.
// MailerGateway failures are logged but never surface to the caller — the
// 204 contract is absolute.
func (h *AuthHandler) ForgotPassword(c echo.Context) error {
	var req forgotPasswordRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyInvalidBody))
	}
	if req.Email == "" {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyEmailRequired))
	}

	loc := locale(c)

	var user model.User
	userErr := h.db.Where("email = ?", req.Email).First(&user).Error

	// Always generate the token regardless of the user-existence branch so the
	// wall-clock time of both code paths is comparable.
	rawToken, tokenHash, err := newPasswordResetToken()
	if err != nil {
		log.Printf("password reset: token generation failed: %v", err)
		return c.NoContent(http.StatusNoContent)
	}

	if errors.Is(userErr, gorm.ErrRecordNotFound) {
		// Unknown email — neutral response, no email sent, no token persisted.
		return c.NoContent(http.StatusNoContent)
	}
	if userErr != nil {
		log.Printf("password reset: user lookup failed for %q: %v", req.Email, userErr)
		return c.NoContent(http.StatusNoContent)
	}

	// Throttle: an unused, unexpired token issued in the throttle window
	// suppresses further sends. Returning early here is silent on purpose —
	// the response shape must not differ from the happy path.
	var recent int64
	cutoff := time.Now().Add(-passwordResetThrottleWindow)
	if err := h.db.Model(&model.PasswordResetToken{}).
		Where("user_id = ? AND used_at IS NULL AND expires_at > ? AND created_at > ?", user.ID, time.Now(), cutoff).
		Count(&recent).Error; err != nil {
		log.Printf("password reset: throttle count for user %d: %v", user.ID, err)
	}
	if recent > 0 {
		return c.NoContent(http.StatusNoContent)
	}

	record := model.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(passwordResetTokenTTL),
	}
	if err := h.db.Create(&record).Error; err != nil {
		log.Printf("password reset: persist token for user %d: %v", user.ID, err)
		return c.NoContent(http.StatusNoContent)
	}

	// Build the localized reset URL — the locale segment matches the webapp's
	// [locale] routing so users land on the page in the language they used to
	// request the reset.
	resetURL := h.buildResetURL(loc, rawToken)

	// Detach from the request context: Echo cancels c.Request().Context() the
	// moment the response is returned, which would abort an in-flight SMTP
	// dial. Background + timeout keeps the send alive long enough to finish.
	go h.dispatchPasswordResetEmail(loc, user.Email, user.Name, resetURL)

	return c.NoContent(http.StatusNoContent)
}

// ResetPassword godoc
// POST /v1/auth/password/reset
//
// Consumes a reset token issued by ForgotPassword. The whole flow runs inside
// a single GORM transaction so two concurrent requests carrying the same
// token cannot both succeed. On success we (a) hash and persist the new
// password, (b) mark the consumed token as used, and (c) invalidate every
// other outstanding token for the user as defence-in-depth in case multiple
// were ever issued. MFA settings are left untouched — a password reset must
// not be a path to bypass MFA. We deliberately do NOT issue a JWT: the user
// has to go through the normal login flow (and MFA, if enabled).
//
// Every failure mode (missing, malformed, expired, used) collapses to a
// single i18n message so an attacker cannot probe which tokens ever existed.
func (h *AuthHandler) ResetPassword(c echo.Context) error {
	loc := locale(c)

	var req resetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyInvalidBody))
	}
	if req.Token == "" || req.Password == "" {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyPasswordResetTokenRequired))
	}
	if len(req.Password) < 8 {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyPasswordTooShort))
	}

	hashed := hashPasswordResetToken(req.Token)

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyProcessPassword))
	}
	hashStr := string(hash)

	var userID uint
	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		var tok model.PasswordResetToken
		if err := tx.Where("token_hash = ?", hashed).First(&tok).Error; err != nil {
			return errPasswordResetInvalid
		}
		if tok.UsedAt != nil || time.Now().After(tok.ExpiresAt) {
			return errPasswordResetInvalid
		}

		now := time.Now()
		// Mark this token used.
		if err := tx.Model(&model.PasswordResetToken{}).
			Where("id = ?", tok.ID).
			Update("used_at", now).Error; err != nil {
			return err
		}
		// Defence-in-depth: invalidate every other unused token for this user
		// so a second token issued during the reset window cannot be redeemed.
		if err := tx.Model(&model.PasswordResetToken{}).
			Where("user_id = ? AND used_at IS NULL", tok.UserID).
			Update("used_at", now).Error; err != nil {
			return err
		}
		// Persist the new password hash.
		if err := tx.Model(&model.User{}).
			Where("id = ?", tok.UserID).
			Update("password_hash", hashStr).Error; err != nil {
			return err
		}
		userID = tok.UserID
		return nil
	})

	if errors.Is(txErr, errPasswordResetInvalid) {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(loc, i18n.KeyPasswordResetTokenInvalid))
	}
	if txErr != nil {
		log.Printf("password reset: tx failed: %v", txErr)
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(loc, i18n.KeyDatabaseError))
	}

	// Cache invalidation mirrors Register: the user row and any admin search
	// page that may contain this user's record are now stale.
	ctx := c.Request().Context()
	if err := h.cache.Del(ctx, cache.KeyUser(userID)); err != nil {
		log.Printf("cache: invalidate user %d after password reset: %v", userID, err)
	}
	if err := h.cache.DelPattern(ctx, cache.KeyAdminUsersGlob); err != nil {
		log.Printf("cache: invalidate admin users after password reset: %v", err)
	}

	return c.NoContent(http.StatusNoContent)
}

// errPasswordResetInvalid is a sentinel returned from the reset transaction
// to collapse missing/expired/used token errors into a single response.
var errPasswordResetInvalid = errors.New("password reset: token invalid")

// newPasswordResetToken returns a freshly-generated plaintext token (for the
// email URL) and its SHA-256 hex digest (for storage). The plaintext value is
// 32 random bytes, base64url-encoded and stripped of padding so it sits cleanly
// in a URL.
func newPasswordResetToken() (raw, hash string, err error) {
	buf := make([]byte, passwordResetTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	hash = hashPasswordResetToken(raw)
	return raw, hash, nil
}

// hashPasswordResetToken returns the SHA-256 hex digest of a plaintext token.
// Used both when issuing a token (storing the hash) and when verifying one
// (looking it up by hash). SHA-256 is appropriate here — these tokens are
// high-entropy random strings, not low-entropy passwords, so bcrypt's slow
// hashing would buy us nothing.
func hashPasswordResetToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// buildResetURL composes the link the user clicks in the email. The locale
// segment matches the webapp's [locale] routing so the reset page renders in
// the language used to request the reset.
func (h *AuthHandler) buildResetURL(loc, token string) string {
	if loc != middleware.LocaleEN && loc != middleware.LocaleIT {
		loc = middleware.LocaleEN
	}
	q := url.Values{}
	q.Set("token", token)
	return h.cfg.FrontendURL + "/" + loc + "/reset-password?" + q.Encode()
}

// dispatchPasswordResetEmail renders and sends the recovery email on a
// best-effort basis. Every failure is logged but never surfaced to the
// HTTP caller — the endpoint contract is to return 204 regardless. Runs on a
// background context with a hard timeout so the send is independent of the
// originating request's lifecycle.
func (h *AuthHandler) dispatchPasswordResetEmail(loc, to, name, resetURL string) {
	if errors.Is(deliverPasswordReset(h.mailer, loc, to, name, resetURL), ErrMailerDisabled) {
		log.Printf("password reset: mailer disabled — would have emailed %s", to)
		return
	}
}

// deliverPasswordReset renders the email body and hands it to the gateway.
// Extracted from dispatchPasswordResetEmail so tests can drive it directly
// without setting up a goroutine.
func deliverPasswordReset(mailer MailerGateway, loc, to, name, resetURL string) error {
	htmlBody, textBody, err := renderPasswordResetEmail(loc, name, resetURL)
	if err != nil {
		log.Printf("password reset: render email: %v", err)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), passwordResetMailerTimeout)
	defer cancel()

	subject := i18n.T(loc, i18n.KeyPasswordResetEmailSubject)
	if err := mailer.Send(ctx, to, subject, htmlBody, textBody); err != nil {
		if !errors.Is(err, ErrMailerDisabled) {
			log.Printf("password reset: send email to %s: %v", to, err)
		}
		return err
	}
	return nil
}
