package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/stretchr/testify/require"
)

// RequestBuilderConfig holds the base URL and HTTP client for building requests.
type RequestBuilderConfig struct {
	BaseURL string
	Client  *http.Client
}

// RequestBuilder provides a fluent API for building and executing HTTP requests.
type RequestBuilder struct {
	baseURL string
	client  *http.Client
	method  string
	path    string
	body    io.Reader
	headers map[string]string
}

// NewRequestBuilder creates a new RequestBuilder.
func NewRequestBuilder(cfg RequestBuilderConfig) *RequestBuilder {
	return &RequestBuilder{
		baseURL: cfg.BaseURL,
		client:  cfg.Client,
		headers: make(map[string]string),
	}
}

// Post sets the HTTP method to POST.
func (rb *RequestBuilder) Post(path string) *RequestBuilder {
	rb.method = http.MethodPost
	rb.path = path
	return rb
}

// Get sets the HTTP method to GET.
func (rb *RequestBuilder) Get(path string) *RequestBuilder {
	rb.method = http.MethodGet
	rb.path = path
	return rb
}

// Put sets the HTTP method to PUT.
func (rb *RequestBuilder) Put(path string) *RequestBuilder {
	rb.method = http.MethodPut
	rb.path = path
	return rb
}

// Patch sets the HTTP method to PATCH.
func (rb *RequestBuilder) Patch(path string) *RequestBuilder {
	rb.method = http.MethodPatch
	rb.path = path
	return rb
}

// Delete sets the HTTP method to DELETE.
func (rb *RequestBuilder) Delete(path string) *RequestBuilder {
	rb.method = http.MethodDelete
	rb.path = path
	return rb
}

// WithRawBody sets the request body with a specific content type.
func (rb *RequestBuilder) WithRawBody(contentType string, body []byte) *RequestBuilder {
	rb.body = bytes.NewReader(body)
	rb.headers["Content-Type"] = contentType
	return rb
}

// WithJSON marshals the data as JSON and sets it as the request body.
func (rb *RequestBuilder) WithJSON(data any) *RequestBuilder {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		panic(fmt.Sprintf("RequestBuilder.WithJSON: %v", err))
	}
	rb.body = bytes.NewReader(jsonBytes)
	rb.headers["Content-Type"] = "application/json"
	return rb
}

// HeaderArgs groups the parameters for adding a single header.
type HeaderArgs struct {
	Key   string
	Value string
}

// WithHeader adds a header to the request.
func (rb *RequestBuilder) WithHeader(args HeaderArgs) *RequestBuilder {
	rb.headers[args.Key] = args.Value
	return rb
}

// QueryArgs groups the parameters for adding a single query parameter.
type QueryArgs struct {
	Key   string
	Value string
}

// WithQuery adds a query parameter to the request URL.
// Keys and values are URL-encoded via url.QueryEscape to handle spaces,
// ampersands, equals signs, and other special characters correctly.
func (rb *RequestBuilder) WithQuery(args QueryArgs) *RequestBuilder {
	encodedKey := url.QueryEscape(args.Key)
	encodedValue := url.QueryEscape(args.Value)
	if strings.Contains(rb.path, "?") {
		rb.path += "&" + encodedKey + "=" + encodedValue
	} else {
		rb.path += "?" + encodedKey + "=" + encodedValue
	}
	return rb
}

// Do executes the request and returns the HTTP response.
func (rb *RequestBuilder) Do() (*http.Response, error) {
	req, err := http.NewRequest(rb.method, rb.baseURL+rb.path, rb.body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	for key, value := range rb.headers {
		req.Header.Set(key, value)
	}

	resp, err := rb.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	return resp, nil
}

// AssertResponse provides assertion helpers for HTTP responses.
type AssertResponse struct {
	resp *http.Response
	err  error
	t    require.TestingT
}

// Expect creates an AssertResponse from the RequestBuilder.
func (rb *RequestBuilder) Expect(t require.TestingT) *AssertResponse {
	resp, err := rb.Do()
	return &AssertResponse{resp: resp, err: err, t: t}
}

// Status asserts the HTTP status code.
func (ar *AssertResponse) Status(code int) *AssertResponse {
	require.NoError(ar.t, ar.err)
	require.NotNil(ar.t, ar.resp)
	require.Equal(ar.t, code, ar.resp.StatusCode)
	return ar
}

// JSON decodes the response body into the target.
func (ar *AssertResponse) JSON(target any) *AssertResponse {
	require.NoError(ar.t, ar.err)
	require.NotNil(ar.t, ar.resp)
	defer ar.resp.Body.Close()

	if err := json.NewDecoder(ar.resp.Body).Decode(target); err != nil {
		require.FailNow(ar.t, "failed to decode JSON response", err.Error())
	}
	return ar
}

// Cookies returns the response cookies.
func (ar *AssertResponse) Cookies() []*http.Cookie {
	require.NoError(ar.t, ar.err)
	require.NotNil(ar.t, ar.resp)
	return ar.resp.Cookies()
}
