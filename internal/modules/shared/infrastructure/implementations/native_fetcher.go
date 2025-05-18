package implementations

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
)

// NativeFetcher implements the Fetcher interface using Go's native http package
type NativeFetcher struct {
	client *http.Client
}

var (
	nativeFetcher *NativeFetcher
	fetcherOnce   sync.Once
)

// GetNativeFetcher returns a singleton instance of NativeFetcher
func GetNativeFetcher() definitions.Fetcher {
	fetcherOnce.Do(func() {
		// Create an HTTP client with sensible defaults
		client := &http.Client{
			Timeout: 30 * time.Second, // Default timeout of 30 seconds
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		}

		nativeFetcher = &NativeFetcher{
			client: client,
		}
	})

	return nativeFetcher
}

// Get fetches content from the specified URL with the provided headers
func (f *NativeFetcher) Get(request definitions.GetRequest) ([]byte, error) {
	// Create a new request
	req, err := http.NewRequest(http.MethodGet, request.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Add headers to the request
	for key, value := range request.Headers {
		req.Header.Set(key, value)
	}

	// Execute the request
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check if the response status is successful (2xx)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return body, nil
}
