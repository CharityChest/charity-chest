package handler_test

// Black-box tests for the Google OAuth callback flow. The fetchGoogleUserInfo
// and findOrCreateGoogleUser helpers are unexported and are exercised here
// through the public HTTP handler (GoogleCallback) by stubbing the upstream
// Google endpoints on http.DefaultClient.Transport. This is the same transport
// path the oauth2 library and the handler both reach into, so a single shim
// covers the token exchange and the userinfo lookup.

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charity-chest/internal/handler"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
)

// roundTripperFunc adapts a function into an http.RoundTripper so tests can
// hijack http.DefaultClient.Transport without spinning up a real server.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// stubGoogleTransport installs a transport on http.DefaultClient that answers
// the token-exchange call with a fixed access token and routes the userinfo
// call to the supplied function. The previous transport is restored on test
// cleanup so parallel-friendly tests stay isolated.
func stubGoogleTransport(t *testing.T, userinfo func(*http.Request) (*http.Response, error)) {
	t.Helper()
	old := http.DefaultClient.Transport
	t.Cleanup(func() { http.DefaultClient.Transport = old })

	http.DefaultClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "oauth2.googleapis.com":
			body := `{"access_token":"the-token","token_type":"Bearer","expires_in":3600}`
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "www.googleapis.com":
			return userinfo(r)
		}
		return nil, errors.New("stubGoogleTransport: unexpected URL " + r.URL.String())
	})
}

// doGoogleCallback drives the callback endpoint with a matching state cookie
// so the only variables under test are the stubbed upstream responses.
func doGoogleCallback(e *echo.Echo) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?state=abc&code=auth-code", nil)
	req.AddCookie(&http.Cookie{Name: handler.CookieOAuthState, Value: "abc"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// --- fetchGoogleUserInfo behaviours, exercised through GoogleCallback ---

func TestGoogleCallback_FetchUserInfo_Success(t *testing.T) {
	var sawAuthHeader string
	stubGoogleTransport(t, func(r *http.Request) (*http.Response, error) {
		sawAuthHeader = r.Header.Get("Authorization")
		return jsonResponse(`{"id":"g-1","email":"u@example.com","name":"User"}`), nil
	})

	e, _, db := newServer(t)
	rec := doGoogleCallback(e)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307; body: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "?token=") {
		t.Errorf("Location %q missing token query", loc)
	}
	if sawAuthHeader != "Bearer the-token" {
		t.Errorf("Authorization header forwarded to Google = %q", sawAuthHeader)
	}

	var user model.User
	if err := db.Where("email = ?", "u@example.com").First(&user).Error; err != nil {
		t.Fatalf("user not persisted: %v", err)
	}
	if user.GoogleID == nil || *user.GoogleID != "g-1" {
		t.Errorf("GoogleID = %v, want %q", user.GoogleID, "g-1")
	}
}

func TestGoogleCallback_FetchUserInfo_TransportError(t *testing.T) {
	stubGoogleTransport(t, func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network down")
	})

	e, _, _ := newServer(t)
	rec := doGoogleCallback(e)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "error=sign_in_failed") {
		t.Errorf("Location %q does not contain error=sign_in_failed", loc)
	}
}

func TestGoogleCallback_FetchUserInfo_InvalidJSON(t *testing.T) {
	stubGoogleTransport(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(`not json`), nil
	})

	e, _, _ := newServer(t)
	rec := doGoogleCallback(e)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "error=sign_in_failed") {
		t.Errorf("Location %q does not contain error=sign_in_failed", loc)
	}
}

// --- findOrCreateGoogleUser behaviours, exercised through GoogleCallback ---

func TestGoogleCallback_NewUser_Created(t *testing.T) {
	stubGoogleTransport(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"id":"g-new","email":"new@example.com","name":"New"}`), nil
	})

	e, _, db := newServer(t)
	if rec := doGoogleCallback(e); rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", rec.Code)
	}

	var count int64
	db.Model(&model.User{}).Where("email = ?", "new@example.com").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 row in users; got %d", count)
	}

	var user model.User
	if err := db.Where("email = ?", "new@example.com").First(&user).Error; err != nil {
		t.Fatalf("look up created user: %v", err)
	}
	if user.GoogleID == nil || *user.GoogleID != "g-new" {
		t.Errorf("GoogleID = %v, want %q", user.GoogleID, "g-new")
	}
}

func TestGoogleCallback_ExistingByGoogleID_Found(t *testing.T) {
	stubGoogleTransport(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"id":"g-existing","email":"pre@example.com","name":"Pre"}`), nil
	})

	e, _, db := newServer(t)
	gid := "g-existing"
	pre := &model.User{Email: "pre@example.com", Name: "Pre", GoogleID: &gid}
	db.Create(pre)

	if rec := doGoogleCallback(e); rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", rec.Code)
	}

	var count int64
	db.Model(&model.User{}).Where("email = ?", "pre@example.com").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 row (existing user reused); got %d", count)
	}
}

func TestGoogleCallback_ExistingByEmail_LinksGoogleID(t *testing.T) {
	stubGoogleTransport(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"id":"g-link","email":"link@example.com","name":"Link"}`), nil
	})

	e, _, db := newServer(t)
	pre := &model.User{Email: "link@example.com", Name: "Link"}
	db.Create(pre)

	if rec := doGoogleCallback(e); rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", rec.Code)
	}

	var reloaded model.User
	if err := db.First(&reloaded, pre.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloaded.GoogleID == nil || *reloaded.GoogleID != "g-link" {
		t.Errorf("GoogleID = %v, want %q linked on existing email account", reloaded.GoogleID, "g-link")
	}
}
