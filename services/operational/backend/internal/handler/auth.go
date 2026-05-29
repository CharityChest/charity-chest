package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"charity-chest/services/operational/backend/internal/adminclient"
	"charity-chest/services/operational/backend/internal/config"
	"charity-chest/services/operational/backend/internal/i18n"
	"charity-chest/services/operational/backend/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AdminAPI is the slice of the admin client used by handlers. Defined here so
// tests can inject a fake without depending on the full *adminclient.Client.
type AdminAPI interface {
	Login(ctx context.Context, email, password string) (*adminclient.User, error)
	GoogleAuth(ctx context.Context, sub, email, name string) (*adminclient.User, error)
	GetUser(ctx context.Context, userUUID uuid.UUID) (*adminclient.User, error)
}

// GoogleValidator verifies a Google OIDC ID token against an expected audience
// and returns the OIDC payload (sub, email, name). The production wiring uses
// google.golang.org/api/idtoken; tests pass a stub.
type GoogleValidator interface {
	Validate(ctx context.Context, idToken, audience string) (*GooglePayload, error)
}

// GooglePayload mirrors the fields we use from idtoken.Payload.Claims so the
// interface boundary doesn't depend on the google library.
type GooglePayload struct {
	Subject string
	Email   string
	Name    string
}

// AuthHandler authenticates mobile clients. It does not store credentials —
// every check is delegated to the admin backend via AdminAPI. After admin
// confirms identity, AuthHandler signs an operational JWT and returns it.
type AuthHandler struct {
	cfg     *config.Config
	admin   AdminAPI
	google  GoogleValidator
	tokenFn func(uuid.UUID, string) (string, error)
}

// NewAuthHandler wires the handler with the production admin client and the
// real Google validator. Tests use NewAuthHandlerWithDeps to inject stubs.
func NewAuthHandler(cfg *config.Config, admin AdminAPI, google GoogleValidator) *AuthHandler {
	h := &AuthHandler{cfg: cfg, admin: admin, google: google}
	h.tokenFn = h.generateJWT
	return h
}

// --- Request / response types ---

// loginRequest is the body for POST /v1/auth/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// googleLoginRequest is the body for POST /v1/auth/google.
// The mobile app obtains id_token via expo-auth-session and forwards it here.
type googleLoginRequest struct {
	IDToken string `json:"id_token"`
}

// authResponse is the envelope returned to the mobile app on success.
type authResponse struct {
	Token string            `json:"token"`
	User  *adminclient.User `json:"user"`
}

// --- Handlers ---

// Login validates credentials via admin and issues an operational JWT.
// Returns 409 KeyMFANotSupported when admin reports the user has MFA enabled —
// v1 of the mobile app does not yet implement the TOTP step.
func (h *AuthHandler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(middleware.LocaleFrom(c), i18n.KeyInvalidBody))
	}
	if req.Email == "" || req.Password == "" {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(middleware.LocaleFrom(c), i18n.KeyFieldsRequired))
	}

	ctx := adminclient.ContextWithLocale(c.Request().Context(), middleware.LocaleFrom(c))
	user, err := h.admin.Login(ctx, req.Email, req.Password)
	if err != nil {
		return h.mapAdminError(c, err)
	}
	if user.MFAEnabled {
		return echo.NewHTTPError(http.StatusConflict, i18n.T(middleware.LocaleFrom(c), i18n.KeyMFANotSupported))
	}

	tok, err := h.tokenFn(user.UUID, user.Email)
	if err != nil {
		log.Printf("auth: sign token: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(middleware.LocaleFrom(c), i18n.KeyGenerateToken))
	}
	return dataJSON(c, http.StatusOK, authResponse{Token: tok, User: user})
}

// GoogleLogin verifies the Google ID token, hands the claims to admin to
// find-or-create the user, and issues an operational JWT.
func (h *AuthHandler) GoogleLogin(c echo.Context) error {
	var req googleLoginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(middleware.LocaleFrom(c), i18n.KeyInvalidBody))
	}
	if req.IDToken == "" {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(middleware.LocaleFrom(c), i18n.KeyFieldsRequired))
	}

	ctx := adminclient.ContextWithLocale(c.Request().Context(), middleware.LocaleFrom(c))
	payload, err := h.google.Validate(ctx, req.IDToken, h.cfg.GoogleAudience)
	if err != nil {
		log.Printf("auth: google verify: %v", err)
		return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(middleware.LocaleFrom(c), i18n.KeyGoogleVerifyFailed))
	}
	if payload.Subject == "" || payload.Email == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(middleware.LocaleFrom(c), i18n.KeyGoogleVerifyFailed))
	}

	user, err := h.admin.GoogleAuth(ctx, payload.Subject, payload.Email, payload.Name)
	if err != nil {
		return h.mapAdminError(c, err)
	}
	if user.MFAEnabled {
		return echo.NewHTTPError(http.StatusConflict, i18n.T(middleware.LocaleFrom(c), i18n.KeyMFANotSupported))
	}

	tok, err := h.tokenFn(user.UUID, user.Email)
	if err != nil {
		log.Printf("auth: sign token: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(middleware.LocaleFrom(c), i18n.KeyGenerateToken))
	}
	return dataJSON(c, http.StatusOK, authResponse{Token: tok, User: user})
}

// --- helpers ---

// mapAdminError converts adminclient errors into the appropriate HTTP status.
func (h *AuthHandler) mapAdminError(c echo.Context, err error) error {
	loc := middleware.LocaleFrom(c)
	switch {
	case errors.Is(err, adminclient.ErrInvalidCredentials):
		return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(loc, i18n.KeyInvalidCredentials))
	case errors.Is(err, adminclient.ErrUserNotFound):
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyUserNotFound))
	case errors.Is(err, adminclient.ErrAdminUnavailable):
		return echo.NewHTTPError(http.StatusBadGateway, i18n.T(loc, i18n.KeyAdminUnavailable))
	default:
		log.Printf("auth: admin error: %v", err)
		return echo.NewHTTPError(http.StatusBadGateway, i18n.T(loc, i18n.KeyAdminUnavailable))
	}
}

// generateJWT signs a 24h HS256 token over the user's public UUID + email.
func (h *AuthHandler) generateJWT(userUUID uuid.UUID, email string) (string, error) {
	claims := middleware.Claims{
		UserUUID: userUUID,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.cfg.JWTSecret))
}
