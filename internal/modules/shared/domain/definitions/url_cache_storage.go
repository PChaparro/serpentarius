package definitions

type SetLUrlCacheRequest struct {
	Key        string
	Value      string
	Expiration int64
}

// UrlCacheStorage is an interface for cache storage operations related to links.
type UrlCacheStorage interface {
	Set(request SetLUrlCacheRequest) error
	Get(key string) (*string, error)
	Delete(key string) error
}
