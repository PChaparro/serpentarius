package implementations

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
	"github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure"
	sharedUtilities "github.com/PChaparro/serpentarius/internal/modules/shared/utilities"
	"github.com/redis/go-redis/v9"
)

// RedisCacheStorage implements the UrlCacheStorage interface for Redis
type RedisCacheStorage struct {
	client *redis.Client
}

var (
	redisCacheStorage *RedisCacheStorage
	redisOnce         sync.Once
)

// GetRedisCacheStorage returns a singleton instance of RedisCacheStorage
func GetRedisCacheStorage() definitions.UrlCacheStorage {
	redisOnce.Do(func() {
		redisCacheStorage = &RedisCacheStorage{
			client: createRedisClient(),
		}
	})

	return redisCacheStorage
}

// createRedisClient creates a shared Redis client instance
func createRedisClient() *redis.Client {
	env := infrastructure.GetEnvironment()

	// Create the Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", env.RedisHost, env.RedisPort),
		Password: env.RedisPassword,
		DB:       env.RedisDB,
	})

	// Test the connection
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		sharedUtilities.GetLogger().
			WithError(err).
			Error("Failed to connect to Redis")
		panic("Unable to connect to Redis: " + err.Error())
	}

	sharedUtilities.GetLogger().
		WithField("host", env.RedisHost).
		WithField("port", env.RedisPort).
		Info("Redis cache client initialized")

	return client
}

// Set stores a key-value pair in the Redis cache with an optional expiration time
func (r *RedisCacheStorage) Set(request definitions.SetURLCacheRequest) error {
	ctx := context.Background()

	// Set expiration time if provided
	var expiration time.Duration
	if request.Expiration > 0 {
		expiration = time.Duration(request.Expiration) * time.Second
	}

	// Store the key-value pair
	err := r.client.Set(ctx, request.Key, request.Value, expiration).Err()
	if err != nil {
		return fmt.Errorf("error setting cache key: %w", err)
	}

	return nil
}

// Get retrieves a value from the Redis cache by key
func (r *RedisCacheStorage) Get(key string) (*string, error) {
	ctx := context.Background()

	// Get the value from Redis
	value, err := r.client.Get(ctx, key).Result()

	// Handle key not found (nil error)
	if err == redis.Nil {
		return nil, nil
	}

	// Handle other errors
	if err != nil {
		return nil, fmt.Errorf("error getting cache key: %w", err)
	}

	return &value, nil
}

// Delete removes a key from the Redis cache
func (r *RedisCacheStorage) Delete(key string) error {
	ctx := context.Background()

	// Delete the key from Redis
	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("error deleting cache key: %w", err)
	}

	return nil
}
