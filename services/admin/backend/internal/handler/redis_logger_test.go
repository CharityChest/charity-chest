package handler_test

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Several handler tests close miniredis mid-test to verify cache-error
// fall-through. That triggers go-redis's pool reconnect logger
// ("failed to dial after 5 attempts"), which spams CI output. Silencing the
// global logger keeps the test signal clean.
type silentRedisLogger struct{}

func (silentRedisLogger) Printf(context.Context, string, ...any) {}

func init() {
	redis.SetLogger(silentRedisLogger{})
}
