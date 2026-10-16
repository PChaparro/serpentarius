package helpers

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// redisClientFromEnv builds a Redis client from the REDIS_* variables set by
// TestMain. Helpers own their client so suites never plumb connections around.
func redisClientFromEnv() *redis.Client {
	host := envOr("REDIS_HOST", "localhost")
	port := envOr("REDIS_PORT", "6379")

	return redis.NewClient(&redis.Options{
		Addr:     host + ":" + port,
		Password: envOr("REDIS_PASSWORD", ""),
		DB:       0,
	})
}

// FlushRedis removes every key from Redis. A successful flush proves the
// container responds; call it in SetupSuite so suites start from a clean slate.
func FlushRedis(t *testing.T) {
	t.Helper()

	client := redisClientFromEnv()
	defer func() { _ = client.Close() }()

	require.NoError(t, client.FlushAll(context.Background()).Err(), "flushing Redis")
}
