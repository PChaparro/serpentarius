package e2e

import (
	"net/http"
	"testing"

	"github.com/PChaparro/serpentarius/tests/e2e/helpers"
	"github.com/rs/xid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// SmokeSuite is the harness gate: it proves the harness itself works (server +
// Redis + MinIO respond) and leaves a dummy PDF in testdata/output/.
type SmokeSuite struct {
	suite.Suite
	anonClient *http.Client
}

func (s *SmokeSuite) SetupSuite() {
	t := s.T()

	s.anonClient = &http.Client{}

	helpers.FlushRedis(t)
	helpers.EnsureBucket(t, helpers.EnsureBucketArgs{Bucket: TestBucket})
	helpers.CleanupBucket(t, helpers.CleanupBucketArgs{Bucket: TestBucket})
}

func (s *SmokeSuite) SetupTest() {
	PDFGeneratorStub.Reset()
	CloudStorageStub.Reset()
	CacheStorageStub.Reset()
}

// TestServerResponds proves the router is wired: an unauthenticated POST
// reaches the auth middleware and is rejected with 401.
func (s *SmokeSuite) TestServerResponds() {
	t := s.T()

	assertUnauthorized(t, RawRequestArgs{
		Method: http.MethodPost,
		Path:   pdfEndpointPath,
		Body:   map[string]any{},
		Client: s.anonClient,
	})
}

// TestRedisResponds proves the Redis container answers.
func (s *SmokeSuite) TestRedisResponds() {
	t := s.T()

	helpers.FlushRedis(t)
}

// TestMinIOBucketResponds proves the MinIO container answers and the
// ephemeral bucket from TestMain exists.
func (s *SmokeSuite) TestMinIOBucketResponds() {
	t := s.T()

	helpers.EnsureBucket(t, helpers.EnsureBucketArgs{Bucket: TestBucket})
}

// TestDummyPDFSaved proves the output dir is writable and leaves a dummy PDF
// behind for visual inspection of the save pipeline.
func (s *SmokeSuite) TestDummyPDFSaved() {
	t := s.T()

	marker := xid.New().String()
	rendered := helpers.RenderTemplate(t, helpers.TemplateArgs{Marker: marker})
	require.Contains(t, rendered.HTML, marker)

	result := helpers.SavePDF(t, helpers.SavePDFArgs{
		Suite: "smoke",
		Test:  "dummy",
		Bytes: []byte("%PDF-1.4 dummy\n%%EOF"),
	})
	require.FileExists(t, result.Path)
}

func TestSmokeSuite(t *testing.T) {
	suite.Run(t, new(SmokeSuite))
}
