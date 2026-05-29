package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mw "charity-chest/services/operational/backend/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const testSecret = "op-test-secret"

func makeToken(t *testing.T, secret string, userUUID uuid.UUID, email string, exp time.Duration) string {
	t.Helper()
	claims := mw.Claims{
		UserUUID: userUUID,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(exp)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return tok
}

func invoke(t *testing.T, secret, authHeader string) (int, uuid.UUID, string) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var gotUUID uuid.UUID
	var gotEmail string
	h := mw.JWT(secret)(func(c echo.Context) error {
		gotUUID, _ = c.Get(mw.UserUUIDContextKey).(uuid.UUID)
		gotEmail, _ = c.Get(mw.EmailContextKey).(string)
		return c.NoContent(http.StatusOK)
	})
	err := h(c)
	if err == nil {
		return rec.Code, gotUUID, gotEmail
	}
	if he, ok := err.(*echo.HTTPError); ok {
		return he.Code, uuid.Nil, ""
	}
	t.Fatalf("unexpected error: %T %v", err, err)
	return 0, uuid.Nil, ""
}

func TestJWT_OK(t *testing.T) {
	id := uuid.New()
	tok := makeToken(t, testSecret, id, "alice@example.com", time.Hour)
	code, gotID, gotEmail := invoke(t, testSecret, "Bearer "+tok)
	if code != http.StatusOK {
		t.Fatalf("code = %d, want 200", code)
	}
	if gotID != id {
		t.Errorf("uuid = %v, want %v", gotID, id)
	}
	if gotEmail != "alice@example.com" {
		t.Errorf("email = %q", gotEmail)
	}
}

func TestJWT_MissingHeader_401(t *testing.T) {
	if code, _, _ := invoke(t, testSecret, ""); code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", code)
	}
}

func TestJWT_WrongScheme_401(t *testing.T) {
	if code, _, _ := invoke(t, testSecret, "Basic abc"); code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", code)
	}
}

func TestJWT_BadSignature_401(t *testing.T) {
	tok := makeToken(t, "other-secret", uuid.New(), "x", time.Hour)
	if code, _, _ := invoke(t, testSecret, "Bearer "+tok); code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", code)
	}
}

func TestJWT_Expired_401(t *testing.T) {
	tok := makeToken(t, testSecret, uuid.New(), "x", -time.Minute)
	if code, _, _ := invoke(t, testSecret, "Bearer "+tok); code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", code)
	}
}

func TestJWT_NilUUID_401(t *testing.T) {
	tok := makeToken(t, testSecret, uuid.Nil, "x", time.Hour)
	if code, _, _ := invoke(t, testSecret, "Bearer "+tok); code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", code)
	}
}
