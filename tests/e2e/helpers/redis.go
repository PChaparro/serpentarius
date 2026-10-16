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

// GetRedisValueArgs groups the parameters for reading a raw cache entry.
type GetRedisValueArgs struct {
	Key string
}

// GetRedisValueResult carries the state of a cache entry.
type GetRedisValueResult struct {
	Found bool
	Value string
}

// GetRedisValue reads a single cache entry. Found is false when the key is
// absent instead of failing, so suites can assert entry creation directly.
func GetRedisValue(t *testing.T, args GetRedisValueArgs) GetRedisValueResult {
	t.Helper()
	require.NotEmpty(t, args.Key, "cache key must not be empty")

	client := redisClientFromEnv()
	defer func() { _ = client.Close() }()

	value, err := client.Get(context.Background(), args.Key).Result()
	if err == redis.Nil {
		return GetRedisValueResult{Found: false}
	}
	require.NoError(t, err, "reading cache entry %q", args.Key)

	return GetRedisValueResult{Found: true, Value: value}
}
