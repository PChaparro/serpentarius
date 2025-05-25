package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sharedHTTP "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/http"
	testConstants "github.com/PChaparro/serpentarius/tests/constants"
	testUtilities "github.com/PChaparro/serpentarius/tests/utilities"
	"github.com/stretchr/testify/assert"
)

const (
	// Hardcoded maximum duration for cache hit in milliseconds
	MAX_DURATION_FOR_CACHE_HIT = 15 // milliseconds
)

// TestPostPDFUrl_ValidDocument tests the API with a valid document
func TestPostPDFUrl_ValidDocument(t *testing.T) {
	router := sharedHTTP.RegisterRoutes()

	bodyBytes, err := testUtilities.ReadFileFromTestsDataDirectory("valid-request.json")
	if err != nil {
		t.Fatalf("Could not read valid body file: %v", err)
	}

	// We send the same request twice to test both PDF generation and cache behavior.
	// The first request should generate the PDF, the second should hit the cache and be much faster.
	var lastResponse *httptest.ResponseRecorder
	var lastDuration time.Duration
	var lastURL string

	for i := 1; i <= 2; i++ {
		start := time.Now()
		w := testUtilities.PostToAPI(testUtilities.PostAPIRequest{
			Router: router,
			URL:    testConstants.GENERATE_PDF_RETURNING_URL_ENDPOINT,
			Body:   string(bodyBytes),
		})
		duration := time.Since(start)
		assert.Equalf(t, http.StatusOK, w.Code, "Request #%d should return 200 with valid document (got %d)", i, w.Code)

		// Parse and validate the JSON response, ensuring a 'url' field is present and not empty
		respAny, err := testUtilities.ParseJSONResponse(w)
		assert.NoError(t, err, "Request #%d response should be valid JSON", i)

		resp, ok := respAny.(map[string]any)
		assert.Truef(t, ok, "Request #%d response should be a JSON object (got: %T)", i, respAny)

		url, ok := resp["url"].(string)
		assert.Truef(t, ok, "Request #%d response should contain a 'url' field (got: %v)", i, resp)
		assert.NotEmptyf(t, url, "Request #%d response 'url' field should not be empty (got: %v)", i, url)

		// Second request should be faster because it hits the cache and return the same URL
		shouldBeCached := lastResponse != nil
		if shouldBeCached {
			assert.Equalf(t, lastResponse.Body.String(), w.Body.String(), "Response #%d should be identical to previous (cache hit) (got: %s)", i, w.Body.String())
			assert.Lessf(t, duration, lastDuration, "Request #%d should be faster than the first (cache) (got: %v vs %v)", i, duration, lastDuration)
			assert.Lessf(t, int64(duration.Milliseconds()), int64(MAX_DURATION_FOR_CACHE_HIT), "Request #%d should be significantly faster (cache hit) (got: %d ms)", i, duration.Milliseconds())
			assert.Equalf(t, lastURL, url, "Request #%d should return the same url as previous (cache hit) (got: %s)", i, url)
		}

		lastResponse = w
		lastDuration = duration
		lastURL = url
	}
}

// TestPostPDFUrl_InvalidDocument tests the API with an invalid document
func TestPostPDFUrl_InvalidDocument(t *testing.T) {
	router := sharedHTTP.RegisterRoutes()

	bodyBytes, err := testUtilities.ReadFileFromTestsDataDirectory("not-valid-request.json")
	if err != nil {
		t.Fatalf("Could not read invalid body file: %v", err)
	}

	w := testUtilities.PostToAPI(testUtilities.PostAPIRequest{
		Router: router,
		URL:    testConstants.GENERATE_PDF_RETURNING_URL_ENDPOINT,
		Body:   string(bodyBytes),
	})

	// We expect a 400 Bad Request error for invalid input
	assert.Equalf(t, http.StatusBadRequest, w.Code, "Should return 400 Bad Request with invalid document (got %d)", w.Code)

	// Parse and validate the JSON response, ensuring an 'errors' array with more than one non-empty string
	respAny, err := testUtilities.ParseJSONResponse(w)
	assert.NoError(t, err, "Response should be valid JSON")

	resp, ok := respAny.(map[string]any)
	assert.Truef(t, ok, "Response should be a JSON object (got: %T)", respAny)

	errorsArr, ok := resp["errors"].([]any)
	assert.Truef(t, ok, "Response should contain an 'errors' array (got: %v)", resp)
	assert.Greaterf(t, len(errorsArr), 1, "'errors' array should have more than one element (got: %v)", errorsArr)

	for validationErrorIndex, validationError := range errorsArr {
		validationErrorMsg, ok := validationError.(string)
		assert.Truef(t, ok, "Element %d in 'errors' should be a string (got: %T)", validationErrorIndex, validationError)
		assert.NotEmptyf(t, validationErrorMsg, "Element %d in 'errors' should not be empty (got: %v)", validationErrorIndex, validationErrorMsg)
	}
}

// TestPostPDFUrl_AuthHeaderValidation tests the API when the Authorization header is missing or invalid
func TestPostPDFUrl_AuthHeaderValidation(t *testing.T) {
	router := sharedHTTP.RegisterRoutes()

	bodyBytes, err := testUtilities.ReadFileFromTestsDataDirectory("valid-request.json")
	if err != nil {
		t.Fatalf("Could not read valid body file: %v", err)
	}

	testCases := []struct {
		name                string
		authOption          testUtilities.AuthOptions
		expectedStatus      int
		expectedMsgContains string
	}{
		{
			name:                "Missing Authorization header",
			authOption:          testUtilities.AuthOptions{Skip: true},
			expectedStatus:      http.StatusUnauthorized,
			expectedMsgContains: "Authorization header is required",
		},
		{
			name:                "Invalid Authorization token",
			authOption:          testUtilities.AuthOptions{Token: "invalid-token", Skip: false},
			expectedStatus:      http.StatusUnauthorized,
			expectedMsgContains: "Authorization token is wrong",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := testUtilities.PostToAPI(testUtilities.PostAPIRequest{
				Router: router,
				URL:    testConstants.GENERATE_PDF_RETURNING_URL_ENDPOINT,
				Body:   string(bodyBytes),
				Auth:   tc.authOption,
			})

			assert.Equalf(t, tc.expectedStatus, w.Code, "Should return %d for case '%s' (got %d)", tc.expectedStatus, tc.name, w.Code)

			respAny, err := testUtilities.ParseJSONResponse(w)
			assert.NoError(t, err, "Response should be valid JSON")
			resp, ok := respAny.(map[string]any)
			assert.Truef(t, ok, "Response should be a JSON object (got: %T)", respAny)

			msg, ok := resp["message"].(string)
			assert.Truef(t, ok, "Response should contain a 'message' field (got: %v)", resp)
			assert.NotEmptyf(t, msg, "'message' field should not be empty (got: %v)", msg)
			assert.Containsf(t, msg, tc.expectedMsgContains, "'message' field should contain expected substring (got: %v)", msg)
		})
	}
}
