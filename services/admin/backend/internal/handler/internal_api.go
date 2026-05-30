package handler

import (
	"errors"
	"log"
	"net/http"

	"charity-chest/services/admin/backend/internal/cache"
	"charity-chest/services/admin/backend/internal/config"
	"charity-chest/services/admin/backend/internal/i18n"
	"charity-chest/services/admin/backend/internal/model"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// InternalHandler serves the /v1/internal/* service-to-service API.
// Callers (e.g. the operational backend) authenticate via the ServiceKey
// middleware. The handler exposes the minimum surface needed to validate
// credentials and look up users — never enough to act as an identity provider
// by itself. It does NOT issue JWTs; the caller signs its own tokens.
type InternalHandler struct {
	db    *gorm.DB
	cfg   *config.Config
	cache *cache.Cache
}

// NewInternalHandler wires an InternalHandler with the same dependencies as
// AuthHandler so it can reuse cache keys and the find-or-create helper.
func NewInternalHandler(db *gorm.DB, c *cache.Cache, cfg *config.Config) *InternalHandler {
	return &InternalHandler{db: db, cfg: cfg, cache: c}
}

// internalLoginRequest is the body for POST /v1/internal/auth/login.
type internalLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// internalGoogleRequest is the body for POST /v1/internal/auth/google.
// The caller (operational backend) has already verified the Google ID token
// and forwards the three trustworthy claims.
type internalGoogleRequest struct {
	GoogleSub string `json:"google_sub"`
	Email     string `json:"email"`
	Name      string `json:"name"`
}

// internalUser is the slim DTO returned by every internal endpoint. It carries
// just enough for the caller to display the user and decide on next steps;
// internal columns (id, password_hash, totp_secret) are intentionally omitted.
type internalUser struct {
	UUID       uuid.UUID                 `json:"uuid"`
	Email      string                    `json:"email"`
	Name       string                    `json:"name"`
	Role       *model.AdministrativeRole `json:"role,omitempty"`
	MFAEnabled bool                      `json:"mfa_enabled"`
}

func toInternalUser(u *model.User) internalUser {
	return internalUser{
		UUID:       u.UUID,
		Email:      u.Email,
		Name:       u.Name,
		Role:       u.Role,
		MFAEnabled: u.MFAEnabled,
	}
}

// InternalLogin validates email + password and returns the slim user DTO.
// It uses generic 401s on every failure mode (no user, no password set, wrong
// password) to avoid user enumeration — same posture as the public Login.
func (h *InternalHandler) InternalLogin(c echo.Context) error {
	var req internalLoginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyInvalidBody))
	}
	if req.Email == "" || req.Password == "" {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyEmailPasswordRequired))
	}

	var user model.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(locale(c), i18n.KeyInvalidCredentials))
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if user.PasswordHash == nil {
		// Generic 401 — surfacing "google-only" here would leak that the email
		// matches an account, which the public Login also avoids.
		return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(locale(c), i18n.KeyInvalidCredentials))
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(locale(c), i18n.KeyInvalidCredentials))
	}

	return dataJSON(c, http.StatusOK, toInternalUser(&user))
}

// InternalGoogleAuth finds or creates a user given the Google identity claims
// that the caller has already verified. Reuses findOrCreateGoogleUser so the
// browser OAuth path and this service-to-service path stay in lock-step.
func (h *InternalHandler) InternalGoogleAuth(c echo.Context) error {
	var req internalGoogleRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyInvalidBody))
	}
	if req.GoogleSub == "" || req.Email == "" {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyGoogleSubEmailRequired))
	}

	gUser := &googleUserInfo{ID: req.GoogleSub, Email: req.Email, Name: req.Name}
	user, err := findOrCreateGoogleUser(h.db, h.cache, gUser)
	if err != nil {
		log.Printf("internal: findOrCreateGoogleUser: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(locale(c), i18n.KeyResolveUser))
	}
	return dataJSON(c, http.StatusOK, toInternalUser(user))
}

// InternalGetUser returns the slim user DTO for a given public UUID.
func (h *InternalHandler) InternalGetUser(c echo.Context) error {
	parsed, err := uuid.Parse(c.Param("userUUID"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyInvalidUserUUID))
	}

	var user model.User
	if err := h.db.Where("uuid = ?", parsed).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, i18n.T(locale(c), i18n.KeyUserNotFound))
		}
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(locale(c), i18n.KeyDatabaseError))
	}
	return dataJSON(c, http.StatusOK, toInternalUser(&user))
}
