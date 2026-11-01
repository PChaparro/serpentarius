package e2e

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	sharedDefinitions "github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
	sharedImplementations "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/implementations"
	"github.com/redis/go-redis/v9"
	"github.com/rs/xid"
	"github.com/stretchr/testify/require"
)

// redisIdleStorage builds a real Redis storage with an explicit idle timeout
// and exposes its concrete type so tests can inspect the client lifecycle.
func redisIdleStorage(t *testing.T, idleTimeout time.Duration) *sharedImplementations.RedisCacheStorage {
	t.Helper()

	storage, ok := sharedImplementations.NewRedisCacheStorageWithIdleTimeout(idleTimeout).(*sharedImplementations.RedisCacheStorage)
	require.True(t, ok, "NewRedisCacheStorageWithIdleTimeout must return *RedisCacheStorage")

	return storage
}

// waitForRedisClientClose asserts that the idle timer closed the given client.
func waitForRedisClientClose(t *testing.T, client *redis.Client) {
	t.Helper()

	require.Eventually(t, func() bool {
		return client.Ping(context.Background()).Err() == redis.ErrClosed
	}, 5*time.Second, 10*time.Millisecond, "idle timer must close the Redis client")
}

// TestRedisCacheIdleCloseReconnectsTransparently covers the full lifecycle:
// an idle client is closed, and both reads and writes after the close recreate
// it and succeed without surfacing an error.
func TestRedisCacheIdleCloseReconnectsTransparently(t *testing.T) {
	const idleTimeout = 200 * time.Millisecond

	storage := redisIdleStorage(t, idleTimeout)
	key := "e2e:redis-idle:" + xid.New().String()

	require.NoError(t, storage.Set(sharedDefinitions.SetPathCacheRequest{
		Key:        key,
		Value:      "before-idle",
		Expiration: 60,
	}))

	firstClient := storage.ClientForTesting()
	require.NotNil(t, firstClient, "storage must own a client after a command")

	waitForRedisClientClose(t, firstClient)
	require.Nil(t, storage.ClientForTesting(), "the closed client must be discarded")

	value, err := storage.Get(key)
	require.NoError(t, err, "a read after the idle close must reconnect transparently")
	require.NotNil(t, value)
	require.Equal(t, "before-idle", *value)
	require.NotNil(t, storage.ClientForTesting(), "the read must leave a live client behind")

	secondClient := storage.ClientForTesting()
	waitForRedisClientClose(t, secondClient)
	require.Nil(t, storage.ClientForTesting())

	require.NoError(t, storage.Set(sharedDefinitions.SetPathCacheRequest{
		Key:        key,
		Value:      "after-idle",
		Expiration: 60,
	}), "a write after the idle close must reconnect transparently")

	value, err = storage.Get(key)
	require.NoError(t, err)
	require.NotNil(t, value)
	require.Equal(t, "after-idle", *value)

	require.NoError(t, storage.Delete(key))

	// Let the last timer fire so this test does not leak a callback into the
	// next one.
	if client := storage.ClientForTesting(); client != nil {
		waitForRedisClientClose(t, client)
	}
}

// TestRedisCacheIdleCloseKeepsConcurrentCommandsSafe hammers the storage with
// short idle timeouts so the timer fires between operations. Concurrent
// commands must never observe a client closed under them.
func TestRedisCacheIdleCloseKeepsConcurrentCommandsSafe(t *testing.T) {
	const (
		idleTimeout = 20 * time.Millisecond
		workers     = 8
		iterations  = 20
	)

	storage := redisIdleStorage(t, idleTimeout)
	prefix := "e2e:redis-idle:concurrent:" + xid.New().String()

	// Start from a deterministically closed client: every worker races to
	// recreate it, which is the critical concurrent admission path. Waiting for
	// this close also keeps the timer from leaking into the next test.
	require.NoError(t, storage.Set(sharedDefinitions.SetPathCacheRequest{
		Key:        prefix + ":prime",
		Value:      "prime",
		Expiration: 60,
	}))
	require.NotNil(t, storage.ClientForTesting())
	require.Eventually(t, func() bool {
		return storage.ClientForTesting() == nil
	}, 5*time.Second, 10*time.Millisecond, "idle timer must discard the primed client")

	var wg sync.WaitGroup
	errs := make(chan error, workers*iterations)

	for worker := 0; worker < workers; worker++ {
		wg.Add(1)

		go func(worker int) {
			defer wg.Done()

			for iteration := 0; iteration < iterations; iteration++ {
				key := fmt.Sprintf("%s:%d:%d", prefix, worker, iteration)
				value := fmt.Sprintf("value-%d-%d", worker, iteration)

				if err := storage.Set(sharedDefinitions.SetPathCacheRequest{
					Key:        key,
					Value:      value,
					Expiration: 60,
				}); err != nil {
					errs <- fmt.Errorf("worker %d iteration %d set: %w", worker, iteration, err)
					return
				}

				// Let the idle timer fire between command pairs, exercising the
				// close/reconnect race under concurrent admissions.
				time.Sleep(idleTimeout / 2)

				got, err := storage.Get(key)
				if err != nil {
					errs <- fmt.Errorf("worker %d iteration %d get: %w", worker, iteration, err)
					return
				}
				if got == nil || *got != value {
					errs <- fmt.Errorf("worker %d iteration %d got %v, want %q", worker, iteration, got, value)
					return
				}
			}
		}(worker)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	// Let the last timer fire so this test does not leak a callback into the
	// next one.
	if client := storage.ClientForTesting(); client != nil {
		waitForRedisClientClose(t, client)
	}
}

// TestRedisCacheIdleTimeoutDisabled verifies that a non-positive timeout never
// closes the client.
func TestRedisCacheIdleTimeoutDisabled(t *testing.T) {
	storage := redisIdleStorage(t, 0)
	key := "e2e:redis-idle:disabled:" + xid.New().String()

	require.NoError(t, storage.Set(sharedDefinitions.SetPathCacheRequest{
		Key:        key,
		Value:      "kept",
		Expiration: 60,
	}))

	client := storage.ClientForTesting()
	require.NotNil(t, client)

	time.Sleep(3 * 20 * time.Millisecond)

	require.Same(t, client, storage.ClientForTesting(), "disabled timeout must keep the same client")
	require.NoError(t, client.Ping(context.Background()).Err())

	require.NoError(t, storage.Delete(key))
}
