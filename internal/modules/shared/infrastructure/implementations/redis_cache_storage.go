package implementations

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
	"github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure"
	sharedUtilities "github.com/PChaparro/serpentarius/internal/modules/shared/utilities"
	"github.com/redis/go-redis/v9"
)

// RedisCacheStorage implements the PathCacheStorage interface for Redis.
//
// The client is a singleton per storage with idle-close management: when
// REDIS_IDLE_TIMEOUT_MINUTES is positive, an inactivity timer closes it after
// no command has used it for that long. A closed client is recreated lazily on
// the next command, so cache operations stay transparent across the close.
//
// client, timer, generation and leases live under one mutex. acquireClient is
// the only way to reach the client; admitting a lease and deciding to close
// happen in the same critical section, so the timer can never close a client
// that a command is using.
type RedisCacheStorage struct {
	// lifecycleMutex guards every field below.
	lifecycleMutex sync.Mutex

	client      *redis.Client
	idleTimeout time.Duration
	idleTimer   *time.Timer
	// timerGeneration is bumped by every activity, so the callback of a timer
	// that already fired is discarded instead of closing a reused client.
	timerGeneration uint64
	// activeLeases counts the commands keeping the client open.
	activeLeases int
}

var (
	redisCacheStorage definitions.PathCacheStorage
	redisOnce         sync.Once
)

// NewRedisCacheStorage creates a new Redis-backed cache storage from the
// current environment. It backs the GetRedisCacheStorage singleton and lets
// tests build isolated real storages without touching global state.
func NewRedisCacheStorage() definitions.PathCacheStorage {
	return NewRedisCacheStorageWithIdleTimeout(redisIdleTimeoutFromEnvironment())
}

// NewRedisCacheStorageWithIdleTimeout creates a Redis-backed cache with an
// explicit idle timeout. A non-positive timeout disables the idle close. It
// exists so tests can exercise the lifecycle with sub-minute timeouts;
// production code uses NewRedisCacheStorage.
func NewRedisCacheStorageWithIdleTimeout(idleTimeout time.Duration) definitions.PathCacheStorage {
	storage := &RedisCacheStorage{
		client:      createRedisClient(),
		idleTimeout: idleTimeout,
	}

	storage.lifecycleMutex.Lock()
	storage.rearmIdleTimerLocked()
	storage.lifecycleMutex.Unlock()

	return storage
}

// GetRedisCacheStorage returns a singleton instance of RedisCacheStorage
func GetRedisCacheStorage() definitions.PathCacheStorage {
	redisOnce.Do(func() {
		if redisCacheStorage == nil {
			redisCacheStorage = NewRedisCacheStorage()
		}
	})

	return redisCacheStorage
}

// SetCacheStorageForTesting replaces the singleton with a test double (must be called before GetRedisCacheStorage).
func SetCacheStorageForTesting(storage definitions.PathCacheStorage) {
	redisCacheStorage = storage
}

// redisIdleTimeoutFromEnvironment converts the configured minutes into a
// duration. Non-positive values disable the idle close.
func redisIdleTimeoutFromEnvironment() time.Duration {
	minutes := infrastructure.GetEnvironment().RedisIdleTimeoutMinutes
	if minutes <= 0 {
		return 0
	}

	// Guard the minutes-to-duration conversion against int64 overflow.
	if int64(minutes) > math.MaxInt64/int64(time.Minute) {
		return time.Duration(math.MaxInt64)
	}

	return time.Duration(minutes) * time.Minute
}

// acquireClient returns the shared client together with the release function
// of the lease it takes on it. Creating the client, admitting the lease and
// rearming the idle timer happen atomically under the lifecycle mutex, so a
// command can never receive a client that the idle timer is closing.
//
// Release is idempotent, so callers may defer it right away.
func (r *RedisCacheStorage) acquireClient() (*redis.Client, func()) {
	r.lifecycleMutex.Lock()

	if r.client == nil {
		r.client = newRedisClient()
		sharedUtilities.GetLogger().Info("Redis cache client reconnected after inactivity")
	}

	r.activeLeases++
	r.rearmIdleTimerLocked()

	acquiredClient := r.client
	r.lifecycleMutex.Unlock()

	var once sync.Once
	return acquiredClient, func() {
		once.Do(func() {
			r.lifecycleMutex.Lock()
			r.activeLeases--
			// Releasing the last lease rearms the timer, so the client is closed
			// REDIS_IDLE_TIMEOUT_MINUTES after it became idle.
			r.rearmIdleTimerLocked()
			r.lifecycleMutex.Unlock()
		})
	}
}

// rearmIdleTimerLocked stops the current timer and arms a new one for the idle
// timeout. The caller must hold lifecycleMutex.
func (r *RedisCacheStorage) rearmIdleTimerLocked() {
	if r.idleTimer != nil {
		r.idleTimer.Stop()
		r.idleTimer = nil
	}

	r.timerGeneration++
	if r.idleTimeout <= 0 {
		return
	}

	currentGeneration := r.timerGeneration
	r.idleTimer = time.AfterFunc(r.idleTimeout, func() {
		r.closeClientIfIdle(currentGeneration)
	})
}

// closeClientIfIdle closes the client when the timer that fired still belongs
// to the current generation and no lease is alive. Both checks and the close
// itself happen under the lifecycle mutex, so an admission cannot slip between
// them. The callback never holds any other lock.
func (r *RedisCacheStorage) closeClientIfIdle(expectedGeneration uint64) {
	r.lifecycleMutex.Lock()
	defer r.lifecycleMutex.Unlock()

	if expectedGeneration != r.timerGeneration {
		// Stale callback: the client was used after this timer was armed.
		return
	}

	if r.activeLeases > 0 {
		// A command still needs the client: try again later.
		r.rearmIdleTimerLocked()
		return
	}

	r.idleTimer = nil
	r.closeClientLocked()
}

// closeClientLocked closes and discards the client. The caller must hold
// lifecycleMutex and must have verified that no lease is alive.
func (r *RedisCacheStorage) closeClientLocked() {
	if r.client == nil {
		return
	}

	currentClient := r.client
	r.client = nil

	if err := currentClient.Close(); err != nil {
		sharedUtilities.GetLogger().WithError(err).
			Error("Failed to close idle Redis connection")
		return
	}

	sharedUtilities.GetLogger().
		WithField("idle_timeout", r.idleTimeout.String()).
		Info("Redis cache client closed due to inactivity")
}

// ClientForTesting returns the current client, or nil when it is closed. It
// exists only so tests can assert the idle lifecycle.
func (r *RedisCacheStorage) ClientForTesting() *redis.Client {
	r.lifecycleMutex.Lock()
	defer r.lifecycleMutex.Unlock()

	return r.client
}

// createRedisClient creates the initial Redis client and verifies that Redis
// is reachable, preserving the fail-fast startup behavior.
func createRedisClient() *redis.Client {
	env := infrastructure.GetEnvironment()
	client := newRedisClient()

	ctx := context.Background()
	if _, err := client.Ping(ctx).Result(); err != nil {
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

// newRedisClient builds a client without dialing. It backs the lazy
// reconnection after an idle close: the first command dials, so a reconnect is
// transparent and never panics inside a request.
func newRedisClient() *redis.Client {
	env := infrastructure.GetEnvironment()

	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", env.RedisHost, env.RedisPort),
		Password: env.RedisPassword,
		DB:       env.RedisDB,
	})
}

// maxExpirationSeconds is the largest whole-second TTL that converts to a
// time.Duration without int64 overflow.
const maxExpirationSeconds = int64(math.MaxInt64 / int64(time.Second))

// Set stores a file path in the Redis cache with an optional expiration time
func (r *RedisCacheStorage) Set(request definitions.SetPathCacheRequest) error {
	ctx := context.Background()

	// Set expiration time if provided, guarding against overflow in the
	// seconds to time.Duration conversion (an overflow would silently
	// produce a non-positive duration, i.e. no expiry).
	var expiration time.Duration
	if request.Expiration > 0 {
		if request.Expiration > maxExpirationSeconds {
			return fmt.Errorf("expiration %d seconds exceeds the maximum supported TTL of %d seconds", request.Expiration, maxExpirationSeconds)
		}
		expiration = time.Duration(request.Expiration) * time.Second
	}

	client, release := r.acquireClient()
	defer release()

	// Store the key-value pair
	err := client.Set(ctx, request.Key, request.Value, expiration).Err()
	if err != nil {
		return fmt.Errorf("error setting cache key: %w", err)
	}

	return nil
}

// Get retrieves a value from the Redis cache by key
func (r *RedisCacheStorage) Get(key string) (*string, error) {
	ctx := context.Background()

	client, release := r.acquireClient()
	defer release()

	// Get the value from Redis
	value, err := client.Get(ctx, key).Result()

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

	client, release := r.acquireClient()
	defer release()

	// Delete the key from Redis
	err := client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("error deleting cache key: %w", err)
	}

	return nil
}
