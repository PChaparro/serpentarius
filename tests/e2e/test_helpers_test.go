package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/PChaparro/serpentarius/tests/e2e/helpers"
	"github.com/stretchr/testify/require"
)

// This file holds package-level test helpers shared by multiple suites.
// Convention: helpers that take more than one non-`t` argument receive an args
// struct; helpers that return two or more values return a result struct (the
// only allowed exception is a trailing `error`, which is idiomatic Go). The
// `*testing.T` is always the leading parameter. Pure fmt.Sprintf path builders
// are the one deliberate exception and keep positional args.

// pdfEndpointPath is the only route under test in this package.
const pdfEndpointPath = "/api/v1/pdf"

// ---------------------------------------------------------------------------
// Named response types (match the API wire format)
// ---------------------------------------------------------------------------

// pdfSuccessResponse is the 200 envelope of POST /api/v1/pdf.
type pdfSuccessResponse struct {
	Message string `json:"message"`
	Bucket  string `json:"bucket"`
	Key     string `json:"key"`
}

// apiErrorResponse is the error envelope returned by middlewares.
// Auth failures: {"message": "..."}.
// Validation failures: {"message": "Validation failed", "errors": [...]}.
// Unexpected failures: {"message": "There was an error processing your request"}.
type apiErrorResponse struct {
	Message string   `json:"message"`
	Errors  []string `json:"errors,omitempty"`
}

// ---------------------------------------------------------------------------
// Raw request helpers
// ---------------------------------------------------------------------------

// RawRequestArgs groups the parameters for a raw HTTP request. AuthHeader is
// the exact Authorization header value to send; nil sends no header at all.
// Callers compose the bearer shape explicitly ("Bearer " + token) when they
// need it. RawBody, when set, is sent verbatim as application/json and takes
// precedence over Body.
type RawRequestArgs struct {
	Method     string
	Path       string
	Body       any // nil → no body sent, unless RawBody is set
	RawBody    []byte
	AuthHeader *string
	Client     *http.Client
}

// RawJSONResult carries the status code and the decoded generic payload.
// Decoding into maps preserves the wire shape (explicit nulls vs missing keys)
// that a typed struct would silently swallow.
type RawJSONResult struct {
	StatusCode int
	Payload    map[string]any
}

// getRawJSONResponse sends the request described by args and returns the
// status code with the decoded payload. Callers assert the status.
func getRawJSONResponse(t *testing.T, args RawRequestArgs) RawJSONResult {
	t.Helper()

	rb := helpers.NewRequestBuilder(helpers.RequestBuilderConfig{
		BaseURL: ServerURL,
		Client:  args.Client,
	})

	switch args.Method {
	case http.MethodGet:
		rb.Get(args.Path)
	case http.MethodPost:
		rb.Post(args.Path)
	case http.MethodPut:
		rb.Put(args.Path)
	case http.MethodPatch:
		rb.Patch(args.Path)
	case http.MethodDelete:
		rb.Delete(args.Path)
	default:
		require.FailNow(t, "unsupported HTTP method", args.Method)
	}

	if args.RawBody != nil {
		rb.WithRawBody("application/json", args.RawBody)
	} else if args.Body != nil {
		rb.WithJSON(args.Body)
	}

	if args.AuthHeader != nil {
		rb.WithHeader(helpers.HeaderArgs{Key: "Authorization", Value: *args.AuthHeader})
	}

	resp, err := rb.Do()
	require.NoError(t, err, "request execution failed")
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "reading response body")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload),
		"response should be valid JSON, got: %s", string(raw))

	return RawJSONResult{StatusCode: resp.StatusCode, Payload: payload}
}

// authToken returns the bearer value TestMain exported via AUTH_SECRET.
func authToken(t *testing.T) string {
	t.Helper()

	token := os.Getenv("AUTH_SECRET")
	require.NotEmpty(t, token, "AUTH_SECRET must be set by TestMain")

	return token
}

// ---------------------------------------------------------------------------
// Auth assertion helpers
// ---------------------------------------------------------------------------

// assertUnauthorized sends the request described by args and asserts the server
// responds with HTTP 401 and a non-empty error message.
func assertUnauthorized(t *testing.T, args RawRequestArgs) {
	t.Helper()

	result := getRawJSONResponse(t, args)
	require.Equal(t, http.StatusUnauthorized, result.StatusCode, "expected 401 Unauthorized")

	message, ok := result.Payload["message"].(string)
	require.True(t, ok, "error response should carry a message string, got: %v", result.Payload)
	require.NotEmpty(t, message, "error message must not be empty")
}
