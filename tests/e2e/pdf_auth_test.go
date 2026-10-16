package e2e

import (
	"net/http"
	"testing"

	"github.com/PChaparro/serpentarius/internal/testutils"
	"github.com/PChaparro/serpentarius/tests/e2e/helpers"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// pdfAuthCase is one row of the auth matrix: the exact Authorization header
// to send (nil sends no header at all) plus the exact 401 message expected
// back. Messages are asserted byte for byte: the middleware contract is the
// literal string, so any rewording must turn its case red.
type pdfAuthCase struct {
	name        string
	authHeader  *string
	wantMessage string
}

// PDFAuthSuite covers the four rejection branches of the auth middleware.
// Requests stop at the middleware, so the PDF stubs are never reached and
// no Chromium render happens.
type PDFAuthSuite struct {
	suite.Suite
	client *http.Client
	token  string
}

func (s *PDFAuthSuite) SetupSuite() {
	t := s.T()

	s.client = &http.Client{}
	s.token = authToken(t)

	helpers.FlushRedis(t)
	helpers.EnsureBucket(t, helpers.EnsureBucketArgs{Bucket: TestBucket})
	helpers.CleanupBucket(t, helpers.CleanupBucketArgs{Bucket: TestBucket})
}

func (s *PDFAuthSuite) SetupTest() {
	PDFGeneratorStub.Reset()
	CloudStorageStub.Reset()
	CacheStorageStub.Reset()
}

// TestUnauthorizedCases sends one request per rejection branch and asserts
// the exact 401 envelope of each.
func (s *PDFAuthSuite) TestUnauthorizedCases() {
	cases := []pdfAuthCase{
		{
			name: "missing header",
			// Mutant: removing the empty-header guard lets header-less requests past auth.
			authHeader:  nil,
			wantMessage: "Authorization header is required",
		},
		{
			name: "header without bearer prefix",
			// Mutant: removing the prefix check accepts the raw token without "Bearer ".
			authHeader:  testutils.StrPtr(s.token),
			wantMessage: "Authorization header must start with 'Bearer'",
		},
		{
			name: "bearer prefix with empty token",
			// Mutant: removing the bare-"Bearer" guard reports this input as a
			// missing prefix instead of an empty token. The trailing space the
			// client sends never survives transport parsing, so the server
			// always sees the bare "Bearer" shape here.
			authHeader:  testutils.StrPtr("Bearer "),
			wantMessage: "Token cannot be empty",
		},
		{
			name: "wrong token",
			// Mutant: replacing the constant-time comparison with an always-true
			// check lets this request through instead of rejecting it.
			authHeader:  testutils.StrPtr("Bearer wrong-token"),
			wantMessage: "Authorization token is wrong",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			t := s.T()

			result := getRawJSONResponse(t, RawRequestArgs{
				Method:     http.MethodPost,
				Path:       pdfEndpointPath,
				Body:       map[string]any{},
				AuthHeader: tc.authHeader,
				Client:     s.client,
			})

			require.Equal(t, http.StatusUnauthorized, result.StatusCode, "expected 401 Unauthorized")
			require.Len(t, result.Payload, 1, "auth errors carry only a message")
			require.Equal(t, tc.wantMessage, result.Payload["message"])
		})
	}
}

func TestPDFAuth(t *testing.T) {
	suite.Run(t, new(PDFAuthSuite))
}
