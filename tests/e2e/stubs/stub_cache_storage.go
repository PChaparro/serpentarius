package stubs

import (
	"sync"

	sharedDefinitions "github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
)

// CacheEntry is the in-memory representation of a cached path entry.
type CacheEntry struct {
	Value      string
	Expiration int64
}

// StubCacheStorage is a test double for definitions.PathCacheStorage backed by
// a map. It covers the HIT/MISS/TTL/namespace branches in later phases.
type StubCacheStorage struct {
	mu             sync.Mutex
	Entries        map[string]CacheEntry
	ReturnSetError error // when set, Set returns this error
	ReturnGetError error // when set, Get returns this error
	ReturnDelError error // when set, Delete returns this error
}

// Set stores a cache entry.
func (s *StubCacheStorage) Set(request sharedDefinitions.SetPathCacheRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ReturnSetError != nil {
		return s.ReturnSetError
	}

	if s.Entries == nil {
		s.Entries = make(map[string]CacheEntry)
	}

	s.Entries[request.Key] = CacheEntry{Value: request.Value, Expiration: request.Expiration}

	return nil
}

// Get returns the cached value or nil when the key is missing.
func (s *StubCacheStorage) Get(key string) (*string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ReturnGetError != nil {
		return nil, s.ReturnGetError
	}

	entry, ok := s.Entries[key]
	if !ok {
		return nil, nil
	}

	value := entry.Value

	return &value, nil
}

// Delete removes a cache entry. Deleting a missing key is a no-op.
func (s *StubCacheStorage) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ReturnDelError != nil {
		return s.ReturnDelError
	}

	delete(s.Entries, key)

	return nil
}

// Reset clears all captured state. Call it in SetupTest when the stub is
// shared across tests within the same suite.
func (s *StubCacheStorage) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Entries = make(map[string]CacheEntry)
	s.ReturnSetError = nil
	s.ReturnGetError = nil
	s.ReturnDelError = nil
}
