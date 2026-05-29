package cache_test

import (
	"testing"
	"time"

	"charity-chest/services/operational/backend/internal/cache"

	"github.com/alicebob/miniredis/v2"
)

// newMiniRedisCache spins up an in-process Redis-compatible server and returns
// a Cache wired to it. The server is closed on test cleanup.
func newMiniRedisCache(t *testing.T) *cache.Cache {
	t.Helper()
	mr := miniredis.RunT(t)
	c, err := cache.New("redis://"+mr.Addr(), 5*time.Minute)
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	return c
}

// --- Disabled cache: every method is a safe no-op ---

func TestDisabled_AllMethodsNoop(t *testing.T) {
	c := cache.Disabled()
	ctx := t.Context()

	var dest string
	hit, err := c.Get(ctx, "k", &dest)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if hit {
		t.Error("Get on disabled cache reported a hit")
	}
	if err := c.Set(ctx, "k", "v"); err != nil {
		t.Errorf("Set: %v", err)
	}
	if err := c.Del(ctx, "k"); err != nil {
		t.Errorf("Del: %v", err)
	}
	if err := c.Del(ctx); err != nil {
		t.Errorf("Del (no keys): %v", err)
	}
	if err := c.DelPattern(ctx, "k:*"); err != nil {
		t.Errorf("DelPattern: %v", err)
	}
}

// --- New: connection error paths ---

func TestNew_InvalidURL(t *testing.T) {
	if _, err := cache.New("://not-a-url", time.Minute); err == nil {
		t.Fatal("expected error for malformed URL, got nil")
	}
}

func TestNew_UnreachableServer(t *testing.T) {
	// Port 1 is reserved and never accepts connections, so Ping fails fast.
	if _, err := cache.New("redis://127.0.0.1:1", 200*time.Millisecond); err == nil {
		t.Fatal("expected ping error for unreachable server, got nil")
	}
}

// --- Enabled cache: round-trips against miniredis ---

func TestEnabled_SetGetRoundTrip(t *testing.T) {
	c := newMiniRedisCache(t)
	ctx := t.Context()

	type payload struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	want := payload{Name: "alice", N: 7}
	if err := c.Set(ctx, "p:1", want); err != nil {
		t.Fatalf("Set: %v", err)
	}

	var got payload
	hit, err := c.Get(ctx, "p:1", &got)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !hit {
		t.Fatal("expected hit, got miss")
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestEnabled_GetMiss(t *testing.T) {
	c := newMiniRedisCache(t)
	var got string
	hit, err := c.Get(t.Context(), "absent", &got)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if hit {
		t.Error("expected miss, got hit")
	}
}

func TestEnabled_Del(t *testing.T) {
	c := newMiniRedisCache(t)
	ctx := t.Context()
	if err := c.Set(ctx, "x", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := c.Del(ctx, "x"); err != nil {
		t.Fatalf("Del: %v", err)
	}
	var got string
	hit, _ := c.Get(ctx, "x", &got)
	if hit {
		t.Error("key still present after Del")
	}
}

func TestEnabled_DelPattern(t *testing.T) {
	c := newMiniRedisCache(t)
	ctx := t.Context()
	for _, k := range []string{"org:1:members", "org:2:members", "user:1"} {
		if err := c.Set(ctx, k, "v"); err != nil {
			t.Fatalf("Set %q: %v", k, err)
		}
	}
	if err := c.DelPattern(ctx, "org:*:members"); err != nil {
		t.Fatalf("DelPattern: %v", err)
	}
	for _, k := range []string{"org:1:members", "org:2:members"} {
		var got string
		if hit, _ := c.Get(ctx, k, &got); hit {
			t.Errorf("key %q survived DelPattern", k)
		}
	}
	// Unmatched key must remain.
	var got string
	if hit, _ := c.Get(ctx, "user:1", &got); !hit {
		t.Error("unmatched key user:1 was deleted")
	}
}
