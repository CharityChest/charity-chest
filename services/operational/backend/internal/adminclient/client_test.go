package adminclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"charity-chest/services/operational/backend/internal/adminclient"

	"github.com/google/uuid"
)

func writeUser(t *testing.T, w http.ResponseWriter, u adminclient.User) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": u})
}

func newStub(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *adminclient.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, adminclient.New(srv.URL, "test-service-key", 2*time.Second)
}

func TestLogin_OK_SendsHeaders(t *testing.T) {
	var captured *http.Request
	srv, client := newStub(t, func(w http.ResponseWriter, r *http.Request) {
		captured = r
		if r.URL.Path != "/v1/internal/auth/login" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get(adminclient.ServiceKeyHeader); got != "test-service-key" {
			t.Errorf("service key header = %q", got)
		}
		if got := r.Header.Get("X-Locale"); got != "it" {
			t.Errorf("locale header = %q, want it", got)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"alice@example.com"`) {
			t.Errorf("body missing email: %s", body)
		}
		writeUser(t, w, adminclient.User{UUID: uuid.New(), Email: "alice@example.com", Name: "Alice"})
	})
	_ = srv

	ctx := adminclient.ContextWithLocale(context.Background(), "it")
	u, err := client.Login(ctx, "alice@example.com", "pw")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("email = %q", u.Email)
	}
	if captured == nil {
		t.Fatal("stub server never received the request")
	}
}

func TestLogin_401_ReturnsInvalidCredentials(t *testing.T) {
	_, client := newStub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := client.Login(context.Background(), "x", "y")
	if !errors.Is(err, adminclient.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLogin_500_ReturnsAdminUnavailable(t *testing.T) {
	_, client := newStub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := client.Login(context.Background(), "x", "y")
	if !errors.Is(err, adminclient.ErrAdminUnavailable) {
		t.Fatalf("err = %v, want ErrAdminUnavailable", err)
	}
}

func TestLogin_BadEnvelope_ReturnsBadResponse(t *testing.T) {
	_, client := newStub(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	})
	_, err := client.Login(context.Background(), "x", "y")
	if !errors.Is(err, adminclient.ErrBadResponse) {
		t.Fatalf("err = %v, want ErrBadResponse", err)
	}
}

func TestGoogleAuth_PostsExpectedBody(t *testing.T) {
	_, client := newStub(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		for _, want := range []string{`"sub-1"`, `"new@example.com"`, `"New"`} {
			if !strings.Contains(string(body), want) {
				t.Errorf("body missing %q: %s", want, body)
			}
		}
		writeUser(t, w, adminclient.User{UUID: uuid.New(), Email: "new@example.com", Name: "New"})
	})
	if _, err := client.GoogleAuth(context.Background(), "sub-1", "new@example.com", "New"); err != nil {
		t.Fatalf("GoogleAuth: %v", err)
	}
}

func TestGetUser_OK(t *testing.T) {
	want := adminclient.User{UUID: uuid.New(), Email: "u@example.com", Name: "U"}
	_, client := newStub(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/internal/users/") {
			t.Errorf("path = %q", r.URL.Path)
		}
		writeUser(t, w, want)
	})
	got, err := client.GetUser(context.Background(), want.UUID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.UUID != want.UUID {
		t.Errorf("uuid = %v, want %v", got.UUID, want.UUID)
	}
}

func TestGetUser_404(t *testing.T) {
	_, client := newStub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := client.GetUser(context.Background(), uuid.New())
	if !errors.Is(err, adminclient.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}

func TestContextWithLocale_NoLocale_NoHeader(t *testing.T) {
	var captured *http.Request
	_, client := newStub(t, func(w http.ResponseWriter, r *http.Request) {
		captured = r
		writeUser(t, w, adminclient.User{UUID: uuid.New(), Email: "u@example.com", Name: "U"})
	})
	if _, err := client.Login(context.Background(), "x", "y"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if got := captured.Header.Get("X-Locale"); got != "" {
		t.Errorf("expected no X-Locale header, got %q", got)
	}
}
