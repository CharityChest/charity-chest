package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"charity-chest/internal/config"
	"charity-chest/internal/i18n"
	"charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

// Cookie names used during the OAuth flow.
const (
	CookieOAuthState  = "oauth_state"
	CookieOAuthLocale = "oauth_locale"
)

// Query strings appended to the OAuth callback redirect URL.
const (
	callbackErrorQuery = "?error=sign_in_failed"
	callbackTokenQuery = "?token="
)

type AuthHandler struct {
	db          *gorm.DB
	cfg         *config.Config
	oauthConfig *oauth2.Config
}

func NewAuthHandler(db *gorm.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		db:  db,
		cfg: cfg,
		oauthConfig: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

// --- Request / Response types ---

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

// --- Handlers ---

// Register godoc
// POST /auth/register
func (h *AuthHandler) Register(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyInvalidBody))
	}
	if req.Email == "" || req.Password == "" || req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyFieldsRequired))
	}
	if len(req.Password) < 8 {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyPasswordTooShort))
	}

	var existing model.User
	if err := h.db.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		return echo.NewHTTPError(http.StatusConflict, i18n.T(locale(c), i18n.KeyEmailTaken))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(locale(c), i18n.KeyProcessPassword))
	}

	hashStr := string(hash)
	user := &model.User{
		Email:        req.Email,
		PasswordHash: &hashStr,
		Name:         req.Name,
	}
	if err := h.db.Create(user).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(locale(c), i18n.KeyCreateUser))
	}

	token, err := h.generateJWT(user)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(locale(c), i18n.KeyGenerateToken))
	}

	return c.JSON(http.StatusCreated, authResponse{Token: token, User: user})
}

// Login godoc
// POST /auth/login
func (h *AuthHandler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, i18n.T(locale(c), i18n.KeyInvalidBody))
	}

	var user model.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Return generic error to avoid user enumeration
		return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(locale(c), i18n.KeyInvalidCredentials))
	}

	if user.PasswordHash == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(locale(c), i18n.KeyGoogleOnly))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(locale(c), i18n.KeyInvalidCredentials))
	}

	token, err := h.generateJWT(&user)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(locale(c), i18n.KeyGenerateToken))
	}

	return c.JSON(http.StatusOK, authResponse{Token: token, User: &user})
}

// GoogleLogin godoc
// GET /auth/google  — redirects the browser to Google's consent screen
func (h *AuthHandler) GoogleLogin(c echo.Context) error {
	state, err := randomState()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, i18n.T(locale(c), i18n.KeyGenerateState))
	}

	oauthLocale := c.QueryParam("locale")
	if oauthLocale != middleware.LocaleIT {
		oauthLocale = middleware.LocaleEN
	}

	cookieDefaults := http.Cookie{
		Path:     "/",
		HttpOnly: true,
		MaxAge:   300, // 5 minutes
		SameSite: http.SameSiteLaxMode,
	}

	stateCookie := cookieDefaults
	stateCookie.Name = CookieOAuthState
	stateCookie.Value = state
	c.SetCookie(&stateCookie)

	localeCookie := cookieDefaults
	localeCookie.Name = CookieOAuthLocale
	localeCookie.Value = oauthLocale
	c.SetCookie(&localeCookie)

	return c.Redirect(http.StatusTemporaryRedirect, h.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline))
}

// GoogleCallback godoc
// GET /auth/google/callback  — exchanges the code, finds-or-creates the user, redirects to the
// webapp callback page with ?token=<jwt> on success or ?error=sign_in_failed on failure.
func (h *AuthHandler) GoogleCallback(c echo.Context) error {
	loc := middleware.LocaleEN
	if lc, err := c.Cookie(CookieOAuthLocale); err == nil && lc.Value != "" {
		loc = lc.Value
	}
	callbackBase := h.cfg.FrontendURL + "/" + loc + "/auth/callback"

	cookie, err := c.Cookie(CookieOAuthState)
	if err != nil || cookie.Value != c.QueryParam("state") {
		return c.Redirect(http.StatusTemporaryRedirect, callbackBase+callbackErrorQuery)
	}

	code := c.QueryParam("code")
	if code == "" {
		return c.Redirect(http.StatusTemporaryRedirect, callbackBase+callbackErrorQuery)
	}

	oauthToken, err := h.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		return c.Redirect(http.StatusTemporaryRedirect, callbackBase+callbackErrorQuery)
	}

	gUser, err := fetchGoogleUserInfo(oauthToken.AccessToken)
	if err != nil {
		return c.Redirect(http.StatusTemporaryRedirect, callbackBase+callbackErrorQuery)
	}

	user, err := h.findOrCreateGoogleUser(gUser)
	if err != nil {
		return c.Redirect(http.StatusTemporaryRedirect, callbackBase+callbackErrorQuery)
	}

	jwtToken, err := h.generateJWT(user)
	if err != nil {
		return c.Redirect(http.StatusTemporaryRedirect, callbackBase+callbackErrorQuery)
	}

	return c.Redirect(http.StatusTemporaryRedirect, callbackBase+callbackTokenQuery+jwtToken)
}

// Me godoc
// GET /api/me  — protected route, returns the current user
func (h *AuthHandler) Me(c echo.Context) error {
	userID := c.Get(middleware.UserIDContextKey).(uint)

	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, i18n.T(locale(c), i18n.KeyUserNotFound))
	}

	return c.JSON(http.StatusOK, &user)
}

// --- Helpers ---

// locale extracts the resolved locale from the Echo context.
// The Locale middleware always sets this key; the fallback guards against
// tests or callers that bypass the middleware stack.
func locale(c echo.Context) string {
	if l, ok := c.Get(middleware.LocaleContextKey).(string); ok && l != "" {
		return l
	}
	return middleware.LocaleEN
}

type googleUserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func fetchGoogleUserInfo(accessToken string) (*googleUserInfo, error) {
	req, err := http.NewRequest(http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (h *AuthHandler) findOrCreateGoogleUser(gUser *googleUserInfo) (*model.User, error) {
	var user model.User

	// Try by Google ID first
	err := h.db.Where("google_id = ?", gUser.ID).First(&user).Error
	if err == nil {
		return &user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Try by email — link Google ID to existing account
	err = h.db.Where("email = ?", gUser.Email).First(&user).Error
	if err == nil {
		user.GoogleID = &gUser.ID
		return &user, h.db.Save(&user).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Create new user
	user = model.User{
		Email:    gUser.Email,
		Name:     gUser.Name,
		GoogleID: &gUser.ID,
	}
	return &user, h.db.Create(&user).Error
}

type jwtClaims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func (h *AuthHandler) generateJWT(user *model.User) (string, error) {
	claims := jwtClaims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.cfg.JWTSecret))
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
