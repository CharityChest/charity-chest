package cache_test

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Tests in this package deliberately close the underlying miniredis server to
// exercise cache-error paths. Each operation then trips go-redis's pool
// reconnect logger ("failed to dial after 5 attempts"), spamming CI output.
// Silencing the global logger keeps the test signal clean.
type silentRedisLogger struct{}

func (silentRedisLogger) Printf(context.Context, string, ...any) {}

func init() {
	redis.SetLogger(silentRedisLogger{})
}
