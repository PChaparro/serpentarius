package definitions

// GetRequest represents a request to fetch content from a URL.
type GetRequest struct {
	// URL is the URL to fetch.
	URL string `json:"url" binding:"required"`
	// Headers are the headers to include in the request.
	Headers map[string]string `json:"headers" binding:"required"`
}

type Fetcher interface {
	Get(request GetRequest) ([]byte, error)
}
