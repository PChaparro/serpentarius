package definitions

// SetPathCacheRequest represents the request for caching a generated file key.
type SetPathCacheRequest struct {
	Key        string
	Value      string
	Expiration int64
}

// PathCacheStorage is an interface for cache storage operations related to generated file keys.
type PathCacheStorage interface {
	Set(request SetPathCacheRequest) error
	Get(key string) (*string, error)
	Delete(key string) error
}
