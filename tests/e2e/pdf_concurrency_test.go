package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/http/requests"
	sharedDefinitions "github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
	sharedInfrastructure "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure"
	sharedImplementations "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/implementations"
	"github.com/PChaparro/serpentarius/tests/e2e/helpers"
	"github.com/rs/xid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// This suite measures parallel-processing capacity with the real Chromium
// renderer: every request travels over HTTP into the shared real-wired engine
// with the production use case and real implementations (rod generator, S3
// storage, Redis cache). Payloads stay small (one marker page per render) so
// the suite exercises our page pool under contention without exhausting the
// host. The burst doubles the effective pool capacity, proving requests
// beyond the initial page count still get served through the waiting queue.

// burstRequestTimeout bounds every burst request: renders take about a
// second on a warm pool, so anything near this limit means the pool stalled
// instead of queueing.
const burstRequestTimeout = 120 * time.Second

// concurrentRenderArgs groups the inputs for one burst request. It carries
// no *testing.T on purpose: workers run off the test goroutine, where
// require calls are forbidden, so failures travel back as errors instead.
type concurrentRenderArgs struct {
	Client  *http.Client
	BaseURL string
	Body    requests.GeneratePDFRequest
	Token   string
	Marker  string
}

// concurrentRenderResult carries the outcome of one burst request.
type concurrentRenderResult struct {
	Marker     string
	StatusCode int
	Key        string
	Err        error
}

// postConcurrentRender posts one generation request and decodes the 200
// envelope, returning every failure as a value for the test goroutine to
// assert. It never touches require: it runs on a worker goroutine.
func postConcurrentRender(args concurrentRenderArgs) concurrentRenderResult {
	payload, err := json.Marshal(args.Body)
	if err != nil {
		return concurrentRenderResult{Marker: args.Marker, Err: fmt.Errorf("marshalling request: %w", err)}
	}

	request, err := http.NewRequest(http.MethodPost, args.BaseURL+pdfEndpointPath, bytes.NewReader(payload))
	if err != nil {
		return concurrentRenderResult{Marker: args.Marker, Err: fmt.Errorf("building request: %w", err)}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+args.Token)

	response, err := args.Client.Do(request)
	if err != nil {
		return concurrentRenderResult{Marker: args.Marker, Err: fmt.Errorf("executing request: %w", err)}
	}
	defer func() { _ = response.Body.Close() }()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return concurrentRenderResult{Marker: args.Marker, StatusCode: response.StatusCode,
			Err: fmt.Errorf("reading response body: %w", err)}
	}

	if response.StatusCode != http.StatusOK {
		return concurrentRenderResult{Marker: args.Marker, StatusCode: response.StatusCode,
			Err: fmt.Errorf("unexpected status %d: %s", response.StatusCode, string(raw))}
	}

	var decoded pdfSuccessResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return concurrentRenderResult{Marker: args.Marker, StatusCode: response.StatusCode,
			Err: fmt.Errorf("decoding success envelope: %w", err)}
	}

	return concurrentRenderResult{
		Marker:     args.Marker,
		StatusCode: response.StatusCode,
		Key:        decoded.Key,
	}
}

// concurrentBurstSize returns twice the effective pool capacity (browsers by
// tabs per browser) read from the same configuration the rod pool uses, so
// the burst always oversubscribes the pool no matter the environment.
func concurrentBurstSize(t *testing.T) int {
	t.Helper()

	env := sharedInfrastructure.GetEnvironment()
	require.Greater(t, env.MaxChromiumBrowsers, 0, "pool must configure at least one browser")
	require.Greater(t, env.MaxChromiumTabsPerBrowser, 0, "pool must configure at least one tab per browser")

	return 2 * env.MaxChromiumBrowsers * env.MaxChromiumTabsPerBrowser
}

// PDFConcurrencySuite exercises POST /api/v1/pdf beyond pool capacity
// against the real Chromium binary and the real Redis and MinIO containers.
type PDFConcurrencySuite struct {
	suite.Suite
	client            *http.Client
	hashGen           sharedDefinitions.HashGenerator
	previousServerURL string
}

func (s *PDFConcurrencySuite) SetupSuite() {
	t := s.T()
	s.client = &http.Client{}
	s.hashGen = sharedImplementations.GetXxHashGenerator()

	// getRawJSONResponse targets the package server URL, so point it at the
	// shared real-wired engine for the whole suite and restore it afterwards.
	// Suites never run in parallel, so no other suite can observe the swap.
	s.previousServerURL = ServerURL
	ServerURL = sharedRealServerURL(t)

	helpers.FlushRedis(t)
	helpers.EnsureBucket(t, helpers.EnsureBucketArgs{Bucket: TestBucket})
}

func (s *PDFConcurrencySuite) TearDownSuite() {
	ServerURL = s.previousServerURL
}

// postRender posts a generation request with a valid bearer token and asserts
// the exact 200 envelope (message, bucket, key and nothing else).
func (s *PDFConcurrencySuite) postRender(body requests.GeneratePDFRequest) renderPostResult {
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

// assertStoredRender downloads the stored object and asserts the full semantic
// contract: magic bytes, page count and marker text via inspection, plus the
// render-hash metadata and the PDF content type.
func (s *PDFConcurrencySuite) assertStoredRender(args renderObjectArgs) helpers.DownloadObjectResult {
	t := s.T()

	downloaded := helpers.DownloadObject(t, helpers.DownloadObjectArgs{
		Bucket: TestBucket,
		Key:    args.Key,
	})

	helpers.InspectPDF(t, helpers.InspectPDFArgs{
		Bytes:   downloaded.Bytes,
		Pages:   args.Pages,
		Markers: args.Markers,
	})

	require.Equal(t, args.Hash, downloaded.Metadata["render-hash"],
		"stored object must carry the current request hash")
	require.Equal(t, "application/pdf", downloaded.ContentType,
		"stored object must be served as PDF")

	return downloaded
}

// TestConcurrentRenders fires twice the pool capacity in small renders at
// once and asserts every one lands a 200 with its own key and a
// semantically valid stored object. Distinct markers give distinct hashes,
// so no burst member can serve another member's cache entry. Every stored
// object is inspected and every PDF is kept under testdata/output.
func (s *PDFConcurrencySuite) TestConcurrentRenders() {
	// Mutant: removing the waiting queue drops every request beyond pool
	// capacity instead of serving it, so the overflow members never answer
	// 200; leaking a page on ReturnPage starves later workers the same way.
	t := s.T()
	token := authToken(t)

	burst := concurrentBurstSize(t)
	burstClient := &http.Client{Timeout: burstRequestTimeout}

	bodies := make([]requests.GeneratePDFRequest, burst)
	markers := make([]string, burst)
	for i := range bodies {
		markers[i] = xid.New().String()
		html := helpers.RenderTemplate(t, helpers.TemplateArgs{Marker: markers[i]}).HTML
		bodies[i] = buildRenderRequest(renderRequestArgs{
			Items: []string{html},
			Key:   "concurrency/concurrent-" + markers[i] + ".pdf",
		})
	}

	start := time.Now()
	var workerGroup sync.WaitGroup
	results := make(chan concurrentRenderResult, burst)
	for i := range bodies {
		workerGroup.Add(1)
		go func(index int) {
			defer workerGroup.Done()
			results <- postConcurrentRender(concurrentRenderArgs{
				Client:  burstClient,
				BaseURL: ServerURL,
				Body:    bodies[index],
				Token:   token,
				Marker:  markers[index],
			})
		}(i)
	}
	workerGroup.Wait()
	close(results)
	t.Logf("burst of %d renders answered in %s", burst, time.Since(start))

	collected := make(map[string]concurrentRenderResult, burst)
	for result := range results {
		require.NoError(t, result.Err, "burst request for marker %q must succeed", result.Marker)
		require.Equal(t, http.StatusOK, result.StatusCode,
			"burst request for marker %q must answer 200", result.Marker)
		collected[result.Marker] = result
	}
	require.Len(t, collected, burst, "every burst member must answer exactly once")

	for i := range bodies {
		result, ok := collected[markers[i]]
		require.True(t, ok, "burst member %d must have answered", i)
		require.Equal(t, bodies[i].Output.Key, result.Key,
			"burst member %d must keep its own key", i)

		keys := expectedRenderKeys(t, renderKeysArgs{HashGen: s.hashGen, Request: bodies[i]})
		downloaded := s.assertStoredRender(renderObjectArgs{
			Key:     bodies[i].Output.Key,
			Pages:   1,
			Markers: []string{markers[i]},
			Hash:    keys.Hash,
		})

		saved := helpers.SavePDF(t, helpers.SavePDFArgs{Suite: "concurrency", Test: "concurrent", Bytes: downloaded.Bytes})
		require.FileExists(t, saved.Path)
	}
}

// TestPoolHealthyAfterBurst posts one more small render and asserts it
// succeeds with a valid stored object, proving the burst left no page
// checked out or browser wedged. It stays green on its own: it only needs
// a working pool, not the burst test to have run first.
func (s *PDFConcurrencySuite) TestPoolHealthyAfterBurst() {
	// Mutant: a pool that never returns pages serves the first renders and
	// then stalls, so this trailing render never answers 200.
	t := s.T()

	marker := xid.New().String()
	html := helpers.RenderTemplate(t, helpers.TemplateArgs{Marker: marker}).HTML
	key := "concurrency/healthy-" + marker + ".pdf"

	body := buildRenderRequest(renderRequestArgs{Items: []string{html}, Key: key})
	keys := expectedRenderKeys(t, renderKeysArgs{HashGen: s.hashGen, Request: body})

	response := s.postRender(body)
	require.Equal(t, key, response.Key)

	downloaded := s.assertStoredRender(renderObjectArgs{
		Key:     key,
		Pages:   1,
		Markers: []string{marker},
		Hash:    keys.Hash,
	})

	saved := helpers.SavePDF(t, helpers.SavePDFArgs{Suite: "concurrency", Test: "healthy", Bytes: downloaded.Bytes})
	require.FileExists(t, saved.Path)
}

func TestPDFConcurrencySuite(t *testing.T) {
	suite.Run(t, new(PDFConcurrencySuite))
}
