package e2e

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/application/use_cases"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/http/controllers"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/http/requests"
	pdfImplementations "github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/implementations"
	sharedDefinitions "github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
	sharedMiddlewares "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/http/middlewares"
	sharedImplementations "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/implementations"
	"github.com/PChaparro/serpentarius/internal/testutils"
	"github.com/PChaparro/serpentarius/tests/e2e/helpers"
	"github.com/gin-gonic/gin"
	"github.com/rs/xid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	pdfcpuAPI "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpuModel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// This suite renders a realistic monthly bank statement for real: the request
// travels over HTTP into a dedicated engine wired with the production use
// case and real implementations (rod generator, S3 storage, Redis cache). No
// doubles sit in the generation path, so the render exercises Chromium,
// pdfcpu, MinIO and Redis. The fixture stays small (about 22KB) and renders
// in seconds.

// statementHeaderTemplate is the printed page header: bank name plus the
// per-run header marker. Chromium header templates ignore body margins, so
// the padding below clears the page edges (horizontal) and the top edge
// (vertical). The single %s is the marker; the template carries no other
// percent signs so Sprintf stays safe. Classes follow the fixture BEM
// naming; inline styles do the rendering because the template is a
// standalone fragment without access to the fixture stylesheet.
const statementHeaderTemplate = `<div class="statement-chrome__header" style="font-family:Arial,sans-serif;font-size:9px;color:#5b6b7a;padding:2mm 12mm 0 12mm;">` +
	`<span class="statement-chrome__brand">Meridian Trust Bank</span>` +
	`<span class="statement-chrome__ref" style="float:right;">Statement ref %s</span></div>`

// statementFooterTemplate is the printed page footer: per-run footer marker
// plus Chromium page counters, which replace the pageNumber and totalPages
// spans at print time. Same edge-clearing padding as the header, mirrored
// to the bottom edge.
const statementFooterTemplate = `<div class="statement-chrome__footer" style="font-family:Arial,sans-serif;font-size:9px;color:#5b6b7a;padding:0 12mm 2mm 12mm;">` +
	`<span class="statement-chrome__ref">Statement ref %s</span>` +
	`<span class="statement-chrome__pages" style="float:right;">Page <span class="pageNumber"></span> of <span class="totalPages"></span></span></div>`

// statementSections lists the statement sections in document order. Every
// section carries a marker derived from the per-run uuid, so suites can
// assert section presence and order in the extracted text.
var statementSections = []string{"header", "summary", "chart", "movements", "notes"}

// markerOrderArgs groups the inputs for asserting section order in text.
type markerOrderArgs struct {
	Text    string
	Markers []string
}

// requireMarkerOrder asserts every marker appears in the text in the given
// order by first occurrence. Repeating markers (the header and footer print
// on every page) still order correctly: their first occurrence sits where
// the section starts.
func requireMarkerOrder(t *testing.T, args markerOrderArgs) {
	t.Helper()
	require.NotEmpty(t, args.Markers, "markers must not be empty")

	previous := -1
	for _, marker := range args.Markers {
		index := strings.Index(args.Text, marker)
		require.NotEqual(t, -1, index, "extracted text must contain marker %q", marker)
		require.Greater(t, index, previous,
			"marker %q must follow the previous section in reading order", marker)
		previous = index
	}
}

// statementObjectArgs groups the expectations for the stored statement.
type statementObjectArgs struct {
	Key     string
	Markers []string
	Hash    string
}

// statementObjectResult carries the downloaded statement and its extracted text.
type statementObjectResult struct {
	Object helpers.DownloadObjectResult
	Text   string
}

// PDFStatementSuite exercises POST /api/v1/pdf with the monthly statement
// fixture against the real Chromium binary and the real Redis and MinIO
// containers.
type PDFStatementSuite struct {
	suite.Suite
	client            *http.Client
	server            *httptest.Server
	hashGen           sharedDefinitions.HashGenerator
	previousServerURL string
}

func (s *PDFStatementSuite) SetupSuite() {
	t := s.T()
	s.client = &http.Client{}
	s.hashGen = sharedImplementations.GetXxHashGenerator()

	requireChromium(t)

	useCase := use_cases.GeneratePDFUseCase{
		PDFGenerator:     pdfImplementations.NewPDFGeneratorRod(),
		CloudStorage:     sharedImplementations.NewS3CloudStorage(),
		PathCacheStorage: sharedImplementations.NewRedisCacheStorage(),
		HashGenerator:    s.hashGen,
	}
	controller := &controllers.GeneratePDFController{UseCase: useCase}

	// Same route shape as production: default engine, error middleware and
	// POST /api/v1/pdf behind auth and validation.
	engine := gin.Default()
	engine.Use(sharedMiddlewares.ErrorHandlerMiddleware())
	apiV1 := engine.Group("/api/v1")
	pdfGroup := apiV1.Group("/pdf")
	pdfGroup.POST(
		"",
		sharedMiddlewares.AuthMiddleware(),
		sharedMiddlewares.RequestValidationMiddleware(requests.GeneratePDFRequest{}),
		controller.Handle,
	)

	s.server = httptest.NewServer(engine)

	// getRawJSONResponse targets the package server URL, so point it at the
	// real-wired engine for the whole suite and restore it afterwards. Suites
	// never run in parallel, so no other suite can observe the swap.
	s.previousServerURL = ServerURL
	ServerURL = s.server.URL

	helpers.FlushRedis(t)
	helpers.EnsureBucket(t, helpers.EnsureBucketArgs{Bucket: TestBucket})
}

func (s *PDFStatementSuite) TearDownSuite() {
	s.server.Close()
	ServerURL = s.previousServerURL
}

// postRender posts a generation request with a valid bearer token and asserts
// the exact 200 envelope (message, bucket, key and nothing else).
func (s *PDFStatementSuite) postRender(body requests.GeneratePDFRequest) renderPostResult {
	t := s.T()

	authHeader := "Bearer " + authToken(t)
	result := getRawJSONResponse(t, RawRequestArgs{
		Method:     http.MethodPost,
		Path:       pdfEndpointPath,
		Body:       body,
		AuthHeader: &authHeader,
		Client:     s.client,
	})

	require.Equal(t, http.StatusOK, result.StatusCode, "render request must succeed")
	require.Len(t, result.Payload, 3, "success envelope must carry exactly message, bucket and key")

	message, ok := result.Payload["message"].(string)
	require.True(t, ok, "success envelope message must be a string, got: %v", result.Payload["message"])
	require.Equal(t, "PDF generated successfully", message)

	bucket, ok := result.Payload["bucket"].(string)
	require.True(t, ok, "success envelope bucket must be a string, got: %v", result.Payload["bucket"])
	require.Equal(t, TestBucket, bucket)

	key, ok := result.Payload["key"].(string)
	require.True(t, ok, "success envelope key must be a string, got: %v", result.Payload["key"])
	require.NotEmpty(t, key, "success envelope key must not be empty")

	return renderPostResult{Message: message, Bucket: bucket, Key: key}
}

// assertStoredStatement downloads the stored statement and asserts the full
// semantic contract: magic bytes, at least three pages, marker text via
// inspection, plus the render-hash metadata and the PDF content type.
func (s *PDFStatementSuite) assertStoredStatement(args statementObjectArgs) statementObjectResult {
	t := s.T()

	downloaded := helpers.DownloadObject(t, helpers.DownloadObjectArgs{
		Bucket: TestBucket,
		Key:    args.Key,
	})

	inspected := helpers.InspectPDF(t, helpers.InspectPDFArgs{
		Bytes:   downloaded.Bytes,
		Markers: args.Markers,
	})

	pageCount, err := pdfcpuAPI.PageCount(
		bytes.NewReader(downloaded.Bytes),
		pdfcpuModel.NewDefaultConfiguration(),
	)
	require.NoError(t, err, "statement must be parseable by pdfcpu")
	require.GreaterOrEqual(t, pageCount, 3,
		"statement must span at least three pages, got %d", pageCount)

	require.Equal(t, args.Hash, downloaded.Metadata["render-hash"],
		"stored object must carry the current request hash")
	require.Equal(t, "application/pdf", downloaded.ContentType,
		"stored object must be served as PDF")

	return statementObjectResult{Object: downloaded, Text: inspected.Text}
}

// TestMonthlyStatement renders the statement fixture as a single A4 item
// with printed header, footer and backgrounds, then asserts every observable
// side effect: exact 200 envelope, stored PDF semantics with section markers
// in reading order, hash metadata and the cache entry. The PDF is kept under
// testdata/output for visual review.
func (s *PDFStatementSuite) TestMonthlyStatement() {
	// Mutant: dropping printBackground loses the summary card fills;
	// dropping the footer template loses page numbering; trimming the
	// movements table keeps the page count below three.
	t := s.T()

	marker := xid.New().String()
	html := helpers.RenderStatementTemplate(t, helpers.StatementTemplateArgs{Marker: marker}).HTML
	key := "statements/monthly-" + marker + ".pdf"

	sectionMarkers := make([]string, len(statementSections))
	for i, section := range statementSections {
		sectionMarkers[i] = marker + "-" + section
	}

	body := requests.GeneratePDFRequest{
		Items: []requests.PDFItem{
			{
				BodyHTML: html,
				Config: &requests.ItemConfig{
					Size:                testutils.StrPtr("a4"),
					PrintBackground:     testutils.BoolPtr(true),
					DisplayHeaderFooter: testutils.BoolPtr(true),
					HeaderHTML:          testutils.StrPtr(fmt.Sprintf(statementHeaderTemplate, sectionMarkers[0])),
					FooterHTML:          testutils.StrPtr(fmt.Sprintf(statementFooterTemplate, marker+"-footer")),
				},
			},
		},
		Output: requests.OutputConfig{Bucket: TestBucket, Key: key},
	}
	keys := expectedRenderKeys(t, renderKeysArgs{HashGen: s.hashGen, Request: body})

	response := s.postRender(body)
	require.Equal(t, key, response.Key)

	downloaded := s.assertStoredStatement(statementObjectArgs{
		Key:     key,
		Markers: sectionMarkers,
		Hash:    keys.Hash,
	})

	requireMarkerOrder(t, markerOrderArgs{Text: downloaded.Text, Markers: sectionMarkers})

	cached := helpers.GetRedisValue(t, helpers.GetRedisValueArgs{Key: keys.CacheKey})
	require.True(t, cached.Found, "render must create a cache entry")
	require.Equal(t, key, cached.Value, "cache entry must point at the uploaded key")

	saved := helpers.SavePDF(t, helpers.SavePDFArgs{Suite: "statement", Test: "monthly", Bytes: downloaded.Object.Bytes})
	require.FileExists(t, saved.Path)
}

func TestPDFStatementSuite(t *testing.T) {
	suite.Run(t, new(PDFStatementSuite))
}
