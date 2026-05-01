package cache

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache wraps a Valkey/Redis client with typed helpers.
// When disabled all methods are no-ops so callers never need nil checks.
type Cache struct {
	enabled bool
	client  *redis.Client
	ttl     time.Duration
}

// New connects to Valkey at the given URL, pings it, and returns a ready Cache.
func New(url string, ttl time.Duration) (*Cache, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Cache{enabled: true, client: client, ttl: ttl}, nil
}

// Disabled returns a no-op Cache. All method calls are safe and do nothing.
func Disabled() *Cache {
	return &Cache{enabled: false}
}

// Get retrieves a cached value by key and JSON-unmarshals it into dest.
// Returns (true, nil) on hit, (false, nil) on miss, (false, err) on error.
func (c *Cache) Get(ctx context.Context, key string, dest any) (bool, error) {
	if !c.enabled {
		return false, nil
	}
	b, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(b, dest)
}

// Set JSON-marshals val and stores it under key with the configured TTL.
func (c *Cache) Set(ctx context.Context, key string, val any) error {
	if !c.enabled {
		return nil
	}
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, b, c.ttl).Err()
}

// Del removes one or more exact keys from the cache.
func (c *Cache) Del(ctx context.Context, keys ...string) error {
	if !c.enabled || len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

// DelPattern removes all keys matching a glob pattern via SCAN + DEL.
// SCAN errors are returned to the caller; individual DEL errors are logged and not propagated.
func (c *Cache) DelPattern(ctx context.Context, pattern string) error {
	if !c.enabled {
		return nil
	}
	var cursor uint64
	for {
		keys, next, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				log.Printf("cache: del pattern %q: %v", pattern, err)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}
