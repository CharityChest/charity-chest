package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"charity-chest/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

const testSecret = "test-signing-secret-for-unit-tests"

// signedToken creates and signs a JWT with the given claims using testSecret.
func signedToken(t *testing.T, claims middleware.Claims) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return tok
}

// validClaims returns claims that expire one hour in the future.
func validClaims(userID uint, email string) middleware.Claims {
	return middleware.Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
}

// invoke runs the JWT middleware with the given Authorization header value and
// returns the HTTP status code plus the context values set by the middleware.
func invoke(t *testing.T, secret, authHeader string) (code int, userID uint, email string, nextCalled bool) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := middleware.JWT(secret)(func(c echo.Context) error {
		nextCalled = true
		if v, ok := c.Get("user_id").(uint); ok {
			userID = v
		}
		if v, ok := c.Get("email").(string); ok {
			email = v
		}
		return c.String(http.StatusOK, "ok")
	})

	if err := h(c); err != nil {
		if he, ok := err.(*echo.HTTPError); ok {
			return he.Code, 0, "", false
		}
		t.Fatalf("unexpected non-HTTP error: %v", err)
	}
	return rec.Code, userID, email, nextCalled
}

func TestJWT_NoAuthorizationHeader(t *testing.T) {
	code, _, _, called := invoke(t, testSecret, "")
	if code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", code)
	}
	if called {
		t.Error("next handler must not be called when header is missing")
	}
}

func TestJWT_MissingBearerPrefix(t *testing.T) {
	tok := signedToken(t, validClaims(1, "user@example.com"))
	// Send token without the "Bearer " prefix
	code, _, _, called := invoke(t, testSecret, tok)
	if code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", code)
	}
	if called {
		t.Error("next handler must not be called")
	}
}

func TestJWT_GarbageToken(t *testing.T) {
	code, _, _, _ := invoke(t, testSecret, "Bearer this.is.not.a.jwt")
	if code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", code)
	}
}

func TestJWT_WrongSecret(t *testing.T) {
	tok := signedToken(t, validClaims(1, "user@example.com"))
	code, _, _, _ := invoke(t, "a-completely-different-secret", "Bearer "+tok)
	if code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", code)
	}
}

func TestJWT_ExpiredToken(t *testing.T) {
	claims := middleware.Claims{
		UserID: 1,
		Email:  "user@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // already expired
		},
	}
	tok := signedToken(t, claims)
	code, _, _, _ := invoke(t, testSecret, "Bearer "+tok)
	if code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", code)
	}
}

func TestJWT_ValidToken_PassesThrough(t *testing.T) {
	tok := signedToken(t, validClaims(42, "alice@example.com"))
	code, userID, email, called := invoke(t, testSecret, "Bearer "+tok)

	if code != http.StatusOK {
		t.Errorf("status = %d, want 200", code)
	}
	if !called {
		t.Error("next handler was not called")
	}
	if userID != 42 {
		t.Errorf("user_id = %d, want 42", userID)
	}
	if email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", email)
	}
}

func TestJWT_ValidToken_ContextValues(t *testing.T) {
	// Verify that user_id and email are independently set for different tokens
	cases := []struct {
		userID uint
		email  string
	}{
		{1, "one@example.com"},
		{99, "ninety-nine@example.com"},
	}

	for _, tc := range cases {
		tok := signedToken(t, validClaims(tc.userID, tc.email))
		_, gotID, gotEmail, _ := invoke(t, testSecret, "Bearer "+tok)

		if gotID != tc.userID {
			t.Errorf("user_id = %d, want %d", gotID, tc.userID)
		}
		if gotEmail != tc.email {
			t.Errorf("email = %q, want %q", gotEmail, tc.email)
		}
	}
}