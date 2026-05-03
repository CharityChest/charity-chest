package handler

import (
	"log"
	"net/http"

	"charity-chest/internal/cache"
	"charity-chest/internal/config"
	"charity-chest/internal/i18n"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
)

// ProfileHandler handles user profile and MFA management.
type ProfileHandler struct {
	db    *gorm.DB
	cfg   *config.Config
	cache *cache.Cache
}

// NewProfileHandler creates a ProfileHandler wired to the given database, config, and cache.
func NewProfileHandler(db *gorm.DB, cfg *config.Config, c *cache.Cache) *ProfileHandler {
	return &ProfileHandler{db: db, cfg: cfg, cache: c}
}

// --- Request / Response types ---

// mfaSetupResponse carries the TOTP provisioning URI and raw secret for the setup step.
type mfaSetupResponse struct {
	URI    string `json:"uri"`
	Secret string `json:"secret"`
}

// mfaStatusResponse reports whether MFA is currently active for the user.
type mfaStatusResponse struct {
	MFAEnabled bool `json:"mfa_enabled"`
}

// mfaCodeRequest is the JSON body for the enable/disable MFA endpoints.
type mfaCodeRequest struct {
	Code string `json:"code"`
}

// --- Handlers ---

// SetupMFA godoc
// GET /api/profile/mfa/setup  — generates a new TOTP secret and stores it pending confirmation
func (h *ProfileHandler) SetupMFA(c echo.Context) error {
	userID := c.Get(middleware.UserIDContextKey).(uint)

	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(locale(c), i18n.KeyUserNotFound))
	}
	if user.MFAEnabled {
		return echo.NewHTTPError(http.StatusConflict, i18n.T(locale(c), i18n.KeyMFAAlreadyEnabled))
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Charity Chest",
		AccountName: user.Email,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(locale(c), i18n.KeyMFAGenerateSecret))
	}

	secret := key.Secret()
	user.TOTPSecret = &secret
	if err := h.db.Save(&user).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(locale(c), i18n.KeyMFAGenerateSecret))
	}

	return dataJSON(c, http.StatusOK, mfaSetupResponse{URI: key.URL(), Secret: secret})
}

// EnableMFA godoc
// POST /api/profile/mfa/enable  — verifies a TOTP code and activates MFA
func (h *ProfileHandler) EnableMFA(c echo.Context) error {
	userID := c.Get(middleware.UserIDContextKey).(uint)

	var req mfaCodeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyInvalidBody))
	}
	if req.Code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyMFACodeRequired))
	}

	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(locale(c), i18n.KeyUserNotFound))
	}
	if user.MFAEnabled {
		return echo.NewHTTPError(http.StatusConflict, i18n.T(locale(c), i18n.KeyMFAAlreadyEnabled))
	}
	if user.TOTPSecret == nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyMFASetupRequired))
	}

	if !totp.Validate(req.Code, *user.TOTPSecret) {
		return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(locale(c), i18n.KeyMFAInvalidCode))
	}

	user.MFAEnabled = true
	if err := h.db.Save(&user).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(locale(c), i18n.KeyMFAGenerateSecret))
	}

	if err := h.cache.Del(c.Request().Context(), cache.KeyUser(userID)); err != nil {
		log.Printf("cache: invalidate user after enable MFA: %v", err)
	}

	return dataJSON(c, http.StatusOK, mfaStatusResponse{MFAEnabled: true})
}

// DisableMFA godoc
// DELETE /api/profile/mfa  — verifies a TOTP code and deactivates MFA
func (h *ProfileHandler) DisableMFA(c echo.Context) error {
	userID := c.Get(middleware.UserIDContextKey).(uint)

	var req mfaCodeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyInvalidBody))
	}
	if req.Code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyMFACodeRequired))
	}

	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(locale(c), i18n.KeyUserNotFound))
	}
	if !user.MFAEnabled {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyMFANotEnabled))
	}

	if user.TOTPSecret == nil || !totp.Validate(req.Code, *user.TOTPSecret) {
		return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(locale(c), i18n.KeyMFAInvalidCode))
	}

	user.MFAEnabled = false
	user.TOTPSecret = nil
	if err := h.db.Save(&user).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(locale(c), i18n.KeyMFAGenerateSecret))
	}

	if err := h.cache.Del(c.Request().Context(), cache.KeyUser(userID)); err != nil {
		log.Printf("cache: invalidate user after disable MFA: %v", err)
	}

	return dataJSON(c, http.StatusOK, mfaStatusResponse{MFAEnabled: false})
}
