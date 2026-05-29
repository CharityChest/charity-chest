// Package adminclient is a typed wrapper around the admin backend's
// /v1/internal/* service-to-service API. It authenticates with a shared API
// key on every request and decodes admin's {"data": ...} envelope into the
// User DTO defined in this package.
package adminclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// ServiceKeyHeader is the HTTP header carrying the shared service secret.
// Must match the admin backend's middleware.ServiceKeyHeader.
const ServiceKeyHeader = "X-Service-Key"

// localeCtxKey is the private context key under which the locale forwarded to
// admin is stored. Callers attach a locale to ctx via ContextWithLocale; the
// client reads it back and copies it to the outbound X-Locale header.
type localeCtxKey struct{}

// ContextWithLocale returns a derived context carrying the given locale string.
// Handlers build their admin-call context like:
//
//	ctx := adminclient.ContextWithLocale(c.Request().Context(), middleware.LocaleFrom(c))
//	user, err := h.admin.Login(ctx, ...)
func ContextWithLocale(ctx context.Context, locale string) context.Context {
	if locale == "" {
		return ctx
	}
	return context.WithValue(ctx, localeCtxKey{}, locale)
}

func localeFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(localeCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// Client is a thin HTTP client for the admin internal API.
type Client struct {
	baseURL    string
	serviceKey string
	httpClient *http.Client
}

// New builds a Client. baseURL is the root admin URL (e.g. http://admin:8080);
// the client appends /v1/internal/... paths itself. The timeout caps every
// request — set to ADMIN_TIMEOUT from config.
func New(baseURL, serviceKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		serviceKey: serviceKey,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// envelope is the admin response shape: {"data": <T>}.
type envelope[T any] struct {
	Data T `json:"data"`
}

// loginRequest matches admin's internalLoginRequest.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// googleRequest matches admin's internalGoogleRequest.
type googleRequest struct {
	GoogleSub string `json:"google_sub"`
	Email     string `json:"email"`
	Name      string `json:"name"`
}

// Login validates the email + password pair against the admin backend.
// Returns ErrInvalidCredentials on 401, ErrAdminUnavailable on transport or
// 5xx failures, and ErrBadResponse on malformed responses.
func (c *Client) Login(ctx context.Context, email, password string) (*User, error) {
	return c.postUser(ctx, "/v1/internal/auth/login", loginRequest{Email: email, Password: password})
}

// GoogleAuth forwards the verified Google identity claims to admin. The
// operational handler verifies the ID token before calling this; admin trusts
// the claims because the service key authenticates the caller.
func (c *Client) GoogleAuth(ctx context.Context, sub, email, name string) (*User, error) {
	return c.postUser(ctx, "/v1/internal/auth/google", googleRequest{GoogleSub: sub, Email: email, Name: name})
}

// GetUser fetches the slim user DTO for a public UUID.
func (c *Client) GetUser(ctx context.Context, userUUID uuid.UUID) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/internal/users/"+userUUID.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %w", ErrAdminUnavailable, err)
	}
	c.setHeaders(ctx, req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAdminUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrUserNotFound
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: admin returned %d", ErrAdminUnavailable, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", ErrBadResponse, resp.StatusCode)
	}
	return decodeUser(resp.Body)
}

// postUser POSTs the body to the given admin path and decodes the user
// envelope. It centralises the status-code mapping so the three POST methods
// (Login, GoogleAuth, and any future ones) stay in lock-step.
func (c *Client) postUser(ctx context.Context, path string, body any) (*User, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal body: %w", ErrBadResponse, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %w", ErrAdminUnavailable, err)
	}
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c.setHeaders(ctx, req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAdminUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrInvalidCredentials
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: admin returned %d", ErrAdminUnavailable, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", ErrBadResponse, resp.StatusCode)
	}
	return decodeUser(resp.Body)
}

// setHeaders sets the service key and forwards the locale (when present on
// ctx via ContextWithLocale) so admin can return localised error messages.
func (c *Client) setHeaders(ctx context.Context, req *http.Request) {
	req.Header.Set(ServiceKeyHeader, c.serviceKey)
	if loc := localeFromCtx(ctx); loc != "" {
		req.Header.Set("X-Locale", loc)
	}
}

func decodeUser(body io.Reader) (*User, error) {
	var env envelope[User]
	if err := json.NewDecoder(body).Decode(&env); err != nil {
		return nil, fmt.Errorf("%w: decode body: %w", ErrBadResponse, err)
	}
	return &env.Data, nil
}
