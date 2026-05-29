package adminclient_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charity-chest/services/operational/backend/internal/adminclient"

	"github.com/google/uuid"
)

func TestPostUser_UnexpectedStatus_ReturnsBadResponse(t *testing.T) {
	// A 4xx that isn't 401 falls through to the ErrBadResponse branch.
	_, client := newStub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	_, err := client.Login(context.Background(), "x", "y")
	if !errors.Is(err, adminclient.ErrBadResponse) {
		t.Fatalf("err = %v, want ErrBadResponse", err)
	}
}

func TestGetUser_500_ReturnsAdminUnavailable(t *testing.T) {
	_, client := newStub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	_, err := client.GetUser(context.Background(), uuid.New())
	if !errors.Is(err, adminclient.ErrAdminUnavailable) {
		t.Fatalf("err = %v, want ErrAdminUnavailable", err)
	}
}

func TestGetUser_UnexpectedStatus_ReturnsBadResponse(t *testing.T) {
	_, client := newStub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	_, err := client.GetUser(context.Background(), uuid.New())
	if !errors.Is(err, adminclient.ErrBadResponse) {
		t.Fatalf("err = %v, want ErrBadResponse", err)
	}
}

func TestGetUser_BadEnvelope_ReturnsBadResponse(t *testing.T) {
	_, client := newStub(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	})
	_, err := client.GetUser(context.Background(), uuid.New())
	if !errors.Is(err, adminclient.ErrBadResponse) {
		t.Fatalf("err = %v, want ErrBadResponse", err)
	}
}

func TestGoogleAuth_TransportError_ReturnsAdminUnavailable(t *testing.T) {
	// Point the client at a closed server so the transport Do() fails.
	srv, client := newStub(t, func(http.ResponseWriter, *http.Request) {})
	srv.Close() // close immediately so the connection is refused
	_, err := client.GoogleAuth(context.Background(), "s", "e@x.io", "N")
	if !errors.Is(err, adminclient.ErrAdminUnavailable) {
		t.Fatalf("err = %v, want ErrAdminUnavailable", err)
	}
}
