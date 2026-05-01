package cache_test

import (
	"context"
	"testing"
	"time"

	"charity-chest/internal/cache"

	"github.com/alicebob/miniredis/v2"
)

// startMiniRedis starts an in-memory Redis-compatible server and returns a
// connected Cache whose TTL is 1 minute. The server is closed on test cleanup.
func startMiniRedis(t *testing.T) (*miniredis.Miniredis, *cache.Cache) {
	t.Helper()
	mr := miniredis.RunT(t)
	c, err := cache.New("redis://"+mr.Addr(), time.Minute)
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	return mr, c
}

// --- Disabled() ---

func TestDisabled_GetReturnsFalseNil(t *testing.T) {
	c := cache.Disabled()
	var v any
	hit, err := c.Get(context.Background(), "k", &v)
	if hit || err != nil {
		t.Errorf("Disabled.Get: hit=%v err=%v; want false, nil", hit, err)
	}
}

func TestDisabled_SetIsNoOp(t *testing.T) {
	c := cache.Disabled()
	if err := c.Set(context.Background(), "k", 42); err != nil {
		t.Errorf("Disabled.Set: %v", err)
	}
}

func TestDisabled_DelIsNoOp(t *testing.T) {
	c := cache.Disabled()
	if err := c.Del(context.Background(), "k"); err != nil {
		t.Errorf("Disabled.Del: %v", err)
	}
}

func TestDisabled_DelPatternIsNoOp(t *testing.T) {
	c := cache.Disabled()
	if err := c.DelPattern(context.Background(), "*"); err != nil {
		t.Errorf("Disabled.DelPattern: %v", err)
	}
}

// --- New ---

func TestNew_ConnectsToMiniRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	c, err := cache.New("redis://"+mr.Addr(), time.Minute)
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	// Round-trip sanity: set + get
	if err := c.Set(context.Background(), "ping", 1); err != nil {
		t.Fatalf("Set after New: %v", err)
	}
}

func TestNew_InvalidURL_ReturnsError(t *testing.T) {
	_, err := cache.New("://bad-url", time.Minute)
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}

func TestNew_UnreachableServer_ReturnsError(t *testing.T) {
	// Use a closed miniredis address so the Ping fails without lingering pool retries.
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()
	_, err := cache.New("redis://"+addr, time.Minute)
	if err == nil {
		t.Error("expected error for closed server, got nil")
	}
}

// --- Get ---

func TestGet_MissReturnsFalseNil(t *testing.T) {
	_, c := startMiniRedis(t)
	var v any
	hit, err := c.Get(context.Background(), "nonexistent", &v)
	if hit || err != nil {
		t.Errorf("miss: hit=%v err=%v; want false, nil", hit, err)
	}
}

func TestGet_HitReturnsTrueAndUnmarshals(t *testing.T) {
	_, c := startMiniRedis(t)
	ctx := context.Background()

	type payload struct{ Name string }
	if err := c.Set(ctx, "key", payload{Name: "hello"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	var got payload
	hit, err := c.Get(ctx, "key", &got)
	if !hit || err != nil {
		t.Fatalf("Get: hit=%v err=%v", hit, err)
	}
	if got.Name != "hello" {
		t.Errorf("Name = %q, want hello", got.Name)
	}
}

func TestGet_CorruptJSON_ReturnsError(t *testing.T) {
	mr, c := startMiniRedis(t)
	// Write raw invalid JSON directly into miniredis.
	if err := mr.Set("bad", "not-json"); err != nil {
		t.Fatalf("miniredis.Set: %v", err)
	}
	var v any
	// The key exists so the fetch succeeds, but unmarshal fails → non-nil error.
	_, err := c.Get(context.Background(), "bad", &v)
	if err == nil {
		t.Error("expected unmarshal error, got nil")
	}
}

// --- Set ---

func TestSet_OverwritesExistingKey(t *testing.T) {
	_, c := startMiniRedis(t)
	ctx := context.Background()
	_ = c.Set(ctx, "k", "first")
	_ = c.Set(ctx, "k", "second")

	var got string
	hit, err := c.Get(ctx, "k", &got)
	if !hit || err != nil || got != "second" {
		t.Errorf("Get after overwrite: hit=%v err=%v val=%q", hit, err, got)
	}
}

// --- Del ---

func TestDel_RemovesSpecifiedKey(t *testing.T) {
	_, c := startMiniRedis(t)
	ctx := context.Background()
	_ = c.Set(ctx, "a", 1)
	_ = c.Set(ctx, "b", 2)

	if err := c.Del(ctx, "a"); err != nil {
		t.Fatalf("Del: %v", err)
	}

	var v int
	hit, _ := c.Get(ctx, "a", &v)
	if hit {
		t.Error("key 'a' should be deleted")
	}
	hit, _ = c.Get(ctx, "b", &v)
	if !hit {
		t.Error("key 'b' should still exist")
	}
}

func TestDel_MultipleKeys(t *testing.T) {
	_, c := startMiniRedis(t)
	ctx := context.Background()
	_ = c.Set(ctx, "x", 1)
	_ = c.Set(ctx, "y", 2)
	_ = c.Set(ctx, "z", 3)

	if err := c.Del(ctx, "x", "y"); err != nil {
		t.Fatalf("Del: %v", err)
	}

	var v int
	if hit, _ := c.Get(ctx, "x", &v); hit {
		t.Error("'x' should be deleted")
	}
	if hit, _ := c.Get(ctx, "y", &v); hit {
		t.Error("'y' should be deleted")
	}
	if hit, _ := c.Get(ctx, "z", &v); !hit {
		t.Error("'z' should still exist")
	}
}

func TestDel_NoKeys_IsNoOp(t *testing.T) {
	_, c := startMiniRedis(t)
	if err := c.Del(context.Background()); err != nil {
		t.Errorf("Del with no keys: %v", err)
	}
}

// --- DelPattern ---

func TestDelPattern_RemovesMatchingKeys(t *testing.T) {
	_, c := startMiniRedis(t)
	ctx := context.Background()
	_ = c.Set(ctx, "admin:users:a:1:20", 1)
	_ = c.Set(ctx, "admin:users:b:1:20", 2)
	_ = c.Set(ctx, "orgs:list", 3)

	if err := c.DelPattern(ctx, "admin:users:*"); err != nil {
		t.Fatalf("DelPattern: %v", err)
	}

	var v int
	if hit, _ := c.Get(ctx, "admin:users:a:1:20", &v); hit {
		t.Error("admin:users:a:1:20 should be deleted")
	}
	if hit, _ := c.Get(ctx, "admin:users:b:1:20", &v); hit {
		t.Error("admin:users:b:1:20 should be deleted")
	}
	if hit, _ := c.Get(ctx, "orgs:list", &v); !hit {
		t.Error("orgs:list should not be deleted")
	}
}

func TestDelPattern_NoMatch_IsNoOp(t *testing.T) {
	_, c := startMiniRedis(t)
	ctx := context.Background()
	_ = c.Set(ctx, "orgs:list", 1)

	if err := c.DelPattern(ctx, "admin:users:*"); err != nil {
		t.Fatalf("DelPattern with no matches: %v", err)
	}

	var v int
	if hit, _ := c.Get(ctx, "orgs:list", &v); !hit {
		t.Error("orgs:list should be unaffected")
	}
}

// --- Error paths ---

func TestGet_ServerDown_ReturnsFalseError(t *testing.T) {
	mr, c := startMiniRedis(t)
	mr.Close() // kill server so Get fails with a network error
	var v any
	hit, err := c.Get(context.Background(), "key", &v)
	if hit {
		t.Error("hit should be false when server is down")
	}
	if err == nil {
		t.Error("expected error when server is down, got nil")
	}
}

func TestSet_ServerDown_ReturnsError(t *testing.T) {
	mr, c := startMiniRedis(t)
	mr.Close()
	if err := c.Set(context.Background(), "key", 42); err == nil {
		t.Error("expected error when server is down, got nil")
	}
}

func TestSet_UnmarshalableValue_ReturnsError(t *testing.T) {
	_, c := startMiniRedis(t)
	// Channels cannot be JSON-marshaled; Set must return the marshal error.
	ch := make(chan int)
	if err := c.Set(context.Background(), "k", ch); err == nil {
		t.Error("expected marshal error, got nil")
	}
}

func TestDelPattern_ServerDown_ReturnsError(t *testing.T) {
	mr, c := startMiniRedis(t)
	mr.Close()
	if err := c.DelPattern(context.Background(), "admin:users:*"); err == nil {
		t.Error("expected error when server is down, got nil")
	}
}

// --- TTL ---

func TestSet_KeyExpiresAfterTTL(t *testing.T) {
	mr, _ := startMiniRedis(t)
	// Create a separate cache with a very short TTL.
	short, err := cache.New("redis://"+mr.Addr(), 500*time.Millisecond)
	if err != nil {
		t.Fatalf("cache.New short TTL: %v", err)
	}

	ctx := context.Background()
	if err := short.Set(ctx, "ttl-key", 42); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Advance miniredis clock past the TTL.
	mr.FastForward(2 * time.Second)

	var v int
	hit, _ := short.Get(ctx, "ttl-key", &v)
	if hit {
		t.Error("ttl-key should have expired")
	}
}
