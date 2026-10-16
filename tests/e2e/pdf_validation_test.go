package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/PChaparro/serpentarius/internal/testutils"
	"github.com/PChaparro/serpentarius/tests/e2e/helpers"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// invalidJSONMessage is the exact 400 envelope message returned when the body
// is not valid JSON or its types do not match the request struct.
const invalidJSONMessage = "Could not validate request. Please, make sure all fields are of the correct type (E.g, ints are not strings) and that the request body is a valid JSON and try again."

// Shared friendly-text suffixes for the custom format validators. They are
// factored out because they repeat across cases, but every case still
// asserts the full "Field '<path>': <text>" string byte for byte.
const (
	invalidBucketMessage    = "Must be a valid S3 bucket name (3-63 lowercase alphanumeric characters, dots or hyphens)"
	invalidKeyMessage       = "Must be a valid S3 key (1-1024 characters, must not start with '/')"
	invalidNamespaceMessage = "Must be a valid cache namespace (1-128 characters, alphanumeric, '_', '.', '-' and ':' separators)"
)

// pdfValidationCase is one row of the validation matrix: how to break an
// otherwise valid payload plus the 400 envelope expected back. mutate
// receives the valid base payload and breaks a single field. raw, when set,
// is sent verbatim instead of the payload; omitBody skips the body entirely.
//
// Assertion scheme is hybrid: messages from our own custom validators
// (s3bucket/s3key/cachenamespace) and the malformed-body envelope are
// asserted byte for byte through wantErrors, because that prose is our own
// contract. Messages from go-validator built-ins (required/min/oneof and
// friends) are deliberately not pinned: wantFields only requires that an
// error mentions each field path, so library rewordings keep the suite green
// while dropped constraints still turn it red (the request would sail
// through to a 200 or surface another shape).
type pdfValidationCase struct {
	name        string
	mutate      func(payload map[string]any)
	raw         []byte
	omitBody    bool
	wantMessage string
	wantErrors  []string
	wantFields  []string
}

// firstItemOf returns the first item map of the base payload.
func firstItemOf(payload map[string]any) map[string]any {
	return payload["items"].([]any)[0].(map[string]any)
}

// outputOf returns the output map of the base payload.
func outputOf(payload map[string]any) map[string]any {
	return payload["output"].(map[string]any)
}

// configOf returns the item config map, creating it when absent.
func configOf(payload map[string]any) map[string]any {
	item := firstItemOf(payload)
	config, ok := item["config"].(map[string]any)
	if !ok {
		config = map[string]any{}
		item["config"] = config
	}
	return config
}

// cacheOf returns the cache map, creating it when absent.
func cacheOf(payload map[string]any) map[string]any {
	cache, ok := payload["cache"].(map[string]any)
	if !ok {
		cache = map[string]any{}
		payload["cache"] = cache
	}
	return cache
}

// pageRangeOf returns the page range map, creating it when absent.
func pageRangeOf(payload map[string]any) map[string]any {
	config := configOf(payload)
	pageRange, ok := config["pageRange"].(map[string]any)
	if !ok {
		pageRange = map[string]any{}
		config["pageRange"] = pageRange
	}
	return pageRange
}

// marginOf returns the margin map, creating it when absent.
func marginOf(payload map[string]any) map[string]any {
	config := configOf(payload)
	margin, ok := config["margin"].(map[string]any)
	if !ok {
		margin = map[string]any{}
		config["margin"] = margin
	}
	return margin
}

// newValidPDFPayload builds the valid base payload every case starts from.
func newValidPDFPayload() map[string]any {
	return map[string]any{
		"items": []any{map[string]any{"bodyHTML": "<p>Hello</p>"}},
		"output": map[string]any{
			"bucket": TestBucket,
			"key":    "reports/result.pdf",
		},
		"cache": map[string]any{
			"ttl":       3600,
			"namespace": "tenant:pdf",
		},
	}
}

// PDFValidationSuite covers the validation middleware over POST /api/v1/pdf.
// Requests stop at the middleware, so the PDF stubs are never reached and
// no Chromium render happens.
type PDFValidationSuite struct {
	suite.Suite
	client *http.Client
	token  string
}

func (s *PDFValidationSuite) SetupSuite() {
	t := s.T()

	s.client = &http.Client{}
	s.token = authToken(t)

	helpers.FlushRedis(t)
	helpers.EnsureBucket(t, helpers.EnsureBucketArgs{Bucket: TestBucket})
	helpers.CleanupBucket(t, helpers.CleanupBucketArgs{Bucket: TestBucket})
}

func (s *PDFValidationSuite) SetupTest() {
	PDFGeneratorStub.Reset()
	CloudStorageStub.Reset()
	CacheStorageStub.Reset()
}

// TestValidationCases breaks one field per row and asserts the 400 envelope
// decoded as raw JSON, so explicit nulls and missing keys stay distinguishable.
func (s *PDFValidationSuite) TestValidationCases() {
	bucket64 := strings.Repeat("a", 64)
	key1025 := strings.Repeat("k", 1025)
	namespace129 := strings.Repeat("n", 129)
	itemsWrongType := []byte(fmt.Sprintf(
		`{"items":"nope","output":{"bucket":%q,"key":"reports/result.pdf"}}`, TestBucket))

	cases := []pdfValidationCase{
		{
			name: "missing items key",
			// Mutant: dropping every Items constraint lets item-less requests through to a 200.
			mutate:      func(payload map[string]any) { delete(payload, "items") },
			wantMessage: "Validation failed",
			wantFields:  []string{"Items"},
		},
		{
			name: "null items",
			// Mutant: explicit null decodes to a nil slice that validation rejects; without it the request proceeds.
			mutate:      func(payload map[string]any) { payload["items"] = nil },
			wantMessage: "Validation failed",
			wantFields:  []string{"Items"},
		},
		{
			name: "item without bodyHTML",
			// Mutant: dropping every BodyHTML constraint lets content-less items through to a 200.
			mutate:      func(payload map[string]any) { delete(firstItemOf(payload), "bodyHTML") },
			wantMessage: "Validation failed",
			wantFields:  []string{"Items[0].BodyHTML"},
		},
		{
			name: "item with empty bodyHTML",
			// Mutant: an empty string is the zero value that validation rejects; without it blank pages render.
			mutate:      func(payload map[string]any) { firstItemOf(payload)["bodyHTML"] = "" },
			wantMessage: "Validation failed",
			wantFields:  []string{"Items[0].BodyHTML"},
		},
		{
			name: "missing output",
			// Mutant: without the Output constraint the failure surfaces as nested Bucket/Key errors instead.
			mutate:      func(payload map[string]any) { delete(payload, "output") },
			wantMessage: "Validation failed",
			wantFields:  []string{"Output"},
		},
		{
			name: "bucket below minimum length",
			// Mutant: removing the length floor accepts 2-char buckets.
			mutate:      func(payload map[string]any) { outputOf(payload)["bucket"] = "ab" },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Output.Bucket': " + invalidBucketMessage},
		},
		{
			name: "bucket above maximum length",
			// Mutant: removing the length cap accepts 64-char buckets.
			mutate:      func(payload map[string]any) { outputOf(payload)["bucket"] = bucket64 },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Output.Bucket': " + invalidBucketMessage},
		},
		{
			name: "bucket with uppercase letters",
			// Mutant: relaxing the charset to accept uppercase stops this case from failing.
			mutate:      func(payload map[string]any) { outputOf(payload)["bucket"] = "My-Bucket-01" },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Output.Bucket': " + invalidBucketMessage},
		},
		{
			name: "bucket with underscore",
			// Mutant: a charset accepting `_` stops this case from failing.
			mutate:      func(payload map[string]any) { outputOf(payload)["bucket"] = "my_bucket_01" },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Output.Bucket': " + invalidBucketMessage},
		},
		{
			name: "bucket with leading hyphen",
			// Mutant: a pattern allowing a non-alphanumeric start stops this case from failing.
			mutate:      func(payload map[string]any) { outputOf(payload)["bucket"] = "-mybucket" },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Output.Bucket': " + invalidBucketMessage},
		},
		{
			name: "bucket with trailing dot",
			// Mutant: a pattern allowing a non-alphanumeric end stops this case from failing.
			mutate:      func(payload map[string]any) { outputOf(payload)["bucket"] = "mybucket." },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Output.Bucket': " + invalidBucketMessage},
		},
		{
			name: "bucket with consecutive dots",
			// Mutant: removing the `..` check stops this case from failing.
			mutate:      func(payload map[string]any) { outputOf(payload)["bucket"] = "my..bucket" },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Output.Bucket': " + invalidBucketMessage},
		},
		{
			name: "bucket empty",
			// Mutant: dropping every Bucket constraint lets an empty bucket through to a 200.
			mutate:      func(payload map[string]any) { outputOf(payload)["bucket"] = "" },
			wantMessage: "Validation failed",
			wantFields:  []string{"Output.Bucket"},
		},
		{
			name: "key with leading slash",
			// Mutant: removing the leading-slash check stops this case from failing.
			mutate:      func(payload map[string]any) { outputOf(payload)["key"] = "/reports/result.pdf" },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Output.Key': " + invalidKeyMessage},
		},
		{
			name: "key empty",
			// Mutant: dropping every Key constraint lets an empty key through to a 200.
			mutate:      func(payload map[string]any) { outputOf(payload)["key"] = "" },
			wantMessage: "Validation failed",
			wantFields:  []string{"Output.Key"},
		},
		{
			name: "key above maximum length",
			// Mutant: removing the 1024 cap stops this case from failing.
			mutate:      func(payload map[string]any) { outputOf(payload)["key"] = key1025 },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Output.Key': " + invalidKeyMessage},
		},
		{
			name: "namespace with slash",
			// Mutant: a charset accepting `/` stops this case from failing.
			mutate:      func(payload map[string]any) { cacheOf(payload)["namespace"] = "tenant/pdf" },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Cache.Namespace': " + invalidNamespaceMessage},
		},
		{
			name: "namespace with space",
			// Mutant: a charset accepting spaces stops this case from failing.
			mutate:      func(payload map[string]any) { cacheOf(payload)["namespace"] = "tenant pdf" },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Cache.Namespace': " + invalidNamespaceMessage},
		},
		{
			name: "namespace with leading colon",
			// Mutant: a pattern accepting an empty leading segment stops this case from failing.
			mutate:      func(payload map[string]any) { cacheOf(payload)["namespace"] = ":pdf" },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Cache.Namespace': " + invalidNamespaceMessage},
		},
		{
			name: "namespace with trailing colon",
			// Mutant: a pattern accepting an empty trailing segment stops this case from failing.
			mutate:      func(payload map[string]any) { cacheOf(payload)["namespace"] = "tenant:" },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Cache.Namespace': " + invalidNamespaceMessage},
		},
		{
			name: "namespace with double colon",
			// Mutant: a pattern accepting `::` stops this case from failing.
			mutate:      func(payload map[string]any) { cacheOf(payload)["namespace"] = "tenant::pdf" },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Cache.Namespace': " + invalidNamespaceMessage},
		},
		{
			name: "namespace above maximum length",
			// Mutant: removing the 128 cap stops this case from failing.
			mutate:      func(payload map[string]any) { cacheOf(payload)["namespace"] = namespace129 },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Cache.Namespace': " + invalidNamespaceMessage},
		},
		{
			name: "namespace with consecutive dots",
			// Mutant: removing the `..` check stops this case from failing.
			mutate:      func(payload map[string]any) { cacheOf(payload)["namespace"] = "tenant..pdf" },
			wantMessage: "Validation failed",
			wantErrors:  []string{"Field 'Cache.Namespace': " + invalidNamespaceMessage},
		},
		{
			name: "scale below minimum",
			// Mutant: dropping the Scale bounds accepts near-zero scales.
			mutate:      func(payload map[string]any) { configOf(payload)["scale"] = 0.05 },
			wantMessage: "Validation failed",
			wantFields:  []string{"Items[0].Config.Scale"},
		},
		{
			name: "scale above maximum",
			// Mutant: dropping the Scale bounds accepts oversized scales.
			mutate:      func(payload map[string]any) { configOf(payload)["scale"] = 3.0 },
			wantMessage: "Validation failed",
			wantFields:  []string{"Items[0].Config.Scale"},
		},
		{
			name: "margin with negative top",
			// Mutant: dropping the margin floor accepts negative margins.
			mutate:      func(payload map[string]any) { marginOf(payload)["topInches"] = -1.0 },
			wantMessage: "Validation failed",
			wantFields:  []string{"Items[0].Config.Margin.TopInches"},
		},
		{
			name: "page range with zero start",
			// Mutant: dropping the Start constraints lets a zero start through validation.
			mutate: func(payload map[string]any) {
				pageRange := pageRangeOf(payload)
				pageRange["start"] = 0
				pageRange["end"] = 2
			},
			wantMessage: "Validation failed",
			wantFields:  []string{"Items[0].Config.PageRange.Start"},
		},
		{
			name: "page range with end before start",
			// Mutant: dropping the End constraints accepts inverted ranges.
			mutate: func(payload map[string]any) {
				pageRange := pageRangeOf(payload)
				pageRange["start"] = 5
				pageRange["end"] = 2
			},
			wantMessage: "Validation failed",
			wantFields:  []string{"Items[0].Config.PageRange.End"},
		},
		{
			name: "page range with missing end",
			// Mutant: dropping the End constraints lets an open-ended range through validation.
			mutate:      func(payload map[string]any) { pageRangeOf(payload)["start"] = 2 },
			wantMessage: "Validation failed",
			wantFields:  []string{"Items[0].Config.PageRange.End"},
		},
		{
			name: "size outside allowed set",
			// Mutant: dropping the Size allow-list accepts unknown page sizes.
			mutate:      func(payload map[string]any) { configOf(payload)["size"] = "folio" },
			wantMessage: "Validation failed",
			wantFields:  []string{"Items[0].Config.Size"},
		},
		{
			name: "orientation outside allowed set",
			// Mutant: dropping the Orientation allow-list accepts unknown orientations.
			mutate:      func(payload map[string]any) { configOf(payload)["orientation"] = "diagonal" },
			wantMessage: "Validation failed",
			wantFields:  []string{"Items[0].Config.Orientation"},
		},
		{
			name: "ttl below minimum",
			// Mutant: dropping the TTL bounds accepts negative TTLs.
			mutate:      func(payload map[string]any) { cacheOf(payload)["ttl"] = -1 },
			wantMessage: "Validation failed",
			wantFields:  []string{"Cache.TTL"},
		},
		{
			name: "ttl above maximum",
			// Mutant: dropping the TTL bounds accepts TTLs beyond one year.
			mutate:      func(payload map[string]any) { cacheOf(payload)["ttl"] = 31536001 },
			wantMessage: "Validation failed",
			wantFields:  []string{"Cache.TTL"},
		},
		{
			name: "malformed json body",
			// Mutant: removing the bind-error guard lets garbage reach the validator instead of this envelope.
			raw:         []byte("{not valid json"),
			wantMessage: invalidJSONMessage,
			wantErrors:  []string{},
		},
		{
			name: "items with wrong type",
			// Mutant: the same bind-error guard answers type mismatches; without it the validator reports another shape.
			raw:         itemsWrongType,
			wantMessage: invalidJSONMessage,
			wantErrors:  []string{},
		},
		{
			name: "empty body",
			// Mutant: the same bind-error guard answers the EOF path; without it an empty body proceeds further.
			omitBody:    true,
			wantMessage: invalidJSONMessage,
			wantErrors:  []string{},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			t := s.T()

			args := RawRequestArgs{
				Method:     http.MethodPost,
				Path:       pdfEndpointPath,
				AuthHeader: testutils.StrPtr("Bearer " + s.token),
				Client:     s.client,
			}
			switch {
			case tc.omitBody:
				// No body is sent at all.
			case tc.raw != nil:
				args.RawBody = tc.raw
			default:
				payload := newValidPDFPayload()
				if tc.mutate != nil {
					tc.mutate(payload)
				}
				args.Body = payload
			}

			result := getRawJSONResponse(t, args)

			require.Equal(t, http.StatusBadRequest, result.StatusCode, "expected 400 Bad Request")
			require.Len(t, result.Payload, 2, "validation errors carry exactly message and errors")
			require.Equal(t, tc.wantMessage, result.Payload["message"])

			rawErrors, ok := result.Payload["errors"].([]any)
			require.True(t, ok, "errors must decode as a JSON array, got: %v", result.Payload["errors"])
			gotErrors := make([]string, 0, len(rawErrors))
			for _, raw := range rawErrors {
				text, ok := raw.(string)
				require.True(t, ok, "every error must be a string, got: %v", raw)
				gotErrors = append(gotErrors, text)
			}

			if tc.wantErrors != nil {
				require.Equal(t, tc.wantErrors, gotErrors)
			}
			if tc.wantFields != nil {
				require.Len(t, gotErrors, len(tc.wantFields))
				joined := strings.Join(gotErrors, "\n")
				for _, field := range tc.wantFields {
					require.Contains(t, joined, field, "expected an error mentioning field %q", field)
				}
			}
		})
	}
}

func TestPDFValidation(t *testing.T) {
	suite.Run(t, new(PDFValidationSuite))
}
