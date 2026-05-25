package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"charity-chest/internal/middleware"
	"charity-chest/internal/model"
	"charity-chest/internal/testdb"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
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
func validClaims(userUUID uuid.UUID, email string) middleware.Claims {
	return middleware.Claims{
		UserUUID: userUUID,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
}

// createUser persists a minimal user and returns it (with its generated UUID/ID).
func createUser(t *testing.T, db *gorm.DB, email string) model.User {
	t.Helper()
	u := model.User{Email: email, Name: "Test"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

// invoke runs the JWT middleware with the given Authorization header value and
// returns the HTTP status code plus the context values set by the middleware.
// The middleware resolves the token's UUID against db, so callers that expect a
// pass-through must persist a matching user first.
func invoke(t *testing.T, db *gorm.DB, secret, authHeader string) (code int, userID uint, email string, nextCalled bool) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := middleware.JWT(db, secret)(func(c echo.Context) error {
		nextCalled = true
		if v, ok := c.Get(middleware.UserIDContextKey).(uint); ok {
			userID = v
		}
		if v, ok := c.Get(middleware.EmailContextKey).(string); ok {
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
	db := testdb.Open(t)
	code, _, _, called := invoke(t, db, testSecret, "")
	if code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", code)
	}
	if called {
		t.Error("next handler must not be called when header is missing")
	}
}

func TestJWT_MissingBearerPrefix(t *testing.T) {
	db := testdb.Open(t)
	tok := signedToken(t, validClaims(uuid.New(), "user@example.com"))
	// Send token without the "Bearer " prefix
	code, _, _, called := invoke(t, db, testSecret, tok)
	if code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", code)
	}
	if called {
		t.Error("next handler must not be called")
	}
}

func TestJWT_GarbageToken(t *testing.T) {
	db := testdb.Open(t)
	code, _, _, _ := invoke(t, db, testSecret, "Bearer this.is.not.a.jwt")
	if code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", code)
	}
}

func TestJWT_WrongSecret(t *testing.T) {
	db := testdb.Open(t)
	tok := signedToken(t, validClaims(uuid.New(), "user@example.com"))
	code, _, _, _ := invoke(t, db, "a-completely-different-secret", "Bearer "+tok)
	if code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", code)
	}
}

func TestJWT_ExpiredToken(t *testing.T) {
	db := testdb.Open(t)
	claims := middleware.Claims{
		UserUUID: uuid.New(),
		Email:    "user@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // already expired
		},
	}
	tok := signedToken(t, claims)
	code, _, _, _ := invoke(t, db, testSecret, "Bearer "+tok)
	if code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", code)
	}
}

// A correctly-signed, unexpired token whose UUID maps to no user is rejected as
// an invalid token (e.g. the account was deleted after the token was issued).
func TestJWT_UnknownUser_Returns401(t *testing.T) {
	db := testdb.Open(t)
	tok := signedToken(t, validClaims(uuid.New(), "ghost@example.com"))
	code, _, _, called := invoke(t, db, testSecret, "Bearer "+tok)
	if code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", code)
	}
	if called {
		t.Error("next handler must not be called for an unknown user")
	}
}

func TestJWT_ValidToken_PassesThrough(t *testing.T) {
	db := testdb.Open(t)
	u := createUser(t, db, "alice@example.com")
	tok := signedToken(t, validClaims(u.UUID, u.Email))
	code, userID, email, called := invoke(t, db, testSecret, "Bearer "+tok)

	if code != http.StatusOK {
		t.Errorf("status = %d, want 200", code)
	}
	if !called {
		t.Error("next handler was not called")
	}
	// The middleware resolves the UUID to the internal int id.
	if userID != u.ID {
		t.Errorf("user_id = %d, want %d", userID, u.ID)
	}
	if email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", email)
	}
}

func TestJWT_ValidToken_ContextValues(t *testing.T) {
	db := testdb.Open(t)
	// Verify that the resolved int user_id and email are independently set for
	// different tokens.
	cases := []struct {
		email string
	}{
		{"one@example.com"},
		{"ninety-nine@example.com"},
	}

	for _, tc := range cases {
		u := createUser(t, db, tc.email)
		tok := signedToken(t, validClaims(u.UUID, u.Email))
		_, gotID, gotEmail, _ := invoke(t, db, testSecret, "Bearer "+tok)

		if gotID != u.ID {
			t.Errorf("user_id = %d, want %d", gotID, u.ID)
		}
		if gotEmail != tc.email {
			t.Errorf("email = %q, want %q", gotEmail, tc.email)
		}
	}
}
