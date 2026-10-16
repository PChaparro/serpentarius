package e2e

import (
	"net/http"
	"os"
	"testing"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/http/requests"
	sharedDefinitions "github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
	sharedImplementations "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/implementations"
	"github.com/PChaparro/serpentarius/tests/e2e/helpers"
	"github.com/rs/xid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// This suite renders PDFs for real: each request travels over HTTP into the
// shared real-wired engine built once per run (see real_engine_test.go) with
// the production use case and real implementations (rod generator, S3
// storage, Redis cache), so every render exercises Chromium, pdfcpu, MinIO
// and Redis.

// renderRequestArgs groups the fields of a generation request built for a render test.
type renderRequestArgs struct {
	Items []string // raw HTML bodies, one per item
	Key   string   // output object key
	// Namespace is the cache namespace; empty means unnamespaced.
	Namespace string
}

// buildRenderRequest assembles the wire request from raw HTML items.
func buildRenderRequest(args renderRequestArgs) requests.GeneratePDFRequest {
	items := make([]requests.PDFItem, len(args.Items))
	for i, html := range args.Items {
		items[i] = requests.PDFItem{BodyHTML: html}
	}

	request := requests.GeneratePDFRequest{
		Items:  items,
		Output: requests.OutputConfig{Bucket: TestBucket, Key: args.Key},
	}
	if args.Namespace != "" {
		request.Cache = &requests.CacheConfig{Namespace: args.Namespace}
	}

	return request
}

// renderKeysArgs groups the inputs for deriving the expected request hash and cache key.
type renderKeysArgs struct {
	HashGen   sharedDefinitions.HashGenerator
	Request   requests.GeneratePDFRequest
	Namespace string
}

// renderKeysResult carries the request hash and the cache key derived from it.
type renderKeysResult struct {
	Hash     string
	CacheKey string
}

// expectedRenderKeys replays the documented key layout (pdf:render with an
// optional namespace segment ahead of the production DTO hash) so suites can
// assert metadata identity and cache entry placement.
func expectedRenderKeys(t *testing.T, args renderKeysArgs) renderKeysResult {
	t.Helper()

	hash := mustRenderHash(t, renderHashArgs{HashGen: args.HashGen, Request: args.Request.ToDTO()})

	cacheKey := "pdf:render:" + hash
	if args.Namespace != "" {
		cacheKey = "pdf:render:" + args.Namespace + ":" + hash
	}

	return renderKeysResult{Hash: hash, CacheKey: cacheKey}
}

// renderPostResult carries the decoded success envelope of a 200 response.
type renderPostResult struct {
	Message string
	Bucket  string
	Key     string
}

// renderObjectArgs groups the expectations for a stored render.
type renderObjectArgs struct {
	Key     string
	Pages   int
	Markers []string
	Hash    string
}

// requireChromium fails fast when no Chromium binary exists at the configured
// path. Rendering for real is mandatory here: replacing the generator with a
// double would turn these tests into fakes, so a missing binary is a hard
// error with install guidance instead of a silent skip.
func requireChromium(t *testing.T) {
	t.Helper()

	binaryPath := os.Getenv("CHROMIUM_BINARY_PATH")
	if binaryPath == "" {
		binaryPath = "/usr/bin/chromium"
	}
	require.FileExists(t, binaryPath,
		"Chromium binary not found at %q: install it (Debian/Ubuntu: apt install chromium, "+
			"Fedora: dnf install chromium, macOS: brew install chromium) or set CHROMIUM_BINARY_PATH",
		binaryPath)
}

// PDFRenderSuite exercises POST /api/v1/pdf against the real Chromium binary
// and the real Redis and MinIO containers.
type PDFRenderSuite struct {
	suite.Suite
	client            *http.Client
	hashGen           sharedDefinitions.HashGenerator
	previousServerURL string
}

func (s *PDFRenderSuite) SetupSuite() {
	t := s.T()
	s.client = &http.Client{}
	s.hashGen = sharedImplementations.GetXxHashGenerator()

	// getRawJSONResponse targets the package server URL, so point it at the
	// shared real-wired engine for the suite and restore it afterwards.
	s.previousServerURL = ServerURL
	ServerURL = sharedRealServerURL(t)

	helpers.FlushRedis(t)
	helpers.EnsureBucket(t, helpers.EnsureBucketArgs{Bucket: TestBucket})
}

func (s *PDFRenderSuite) TearDownSuite() {
	ServerURL = s.previousServerURL
}

// postRender posts a generation request with a valid bearer token and asserts
// the exact 200 envelope (message, bucket, key and nothing else).
func (s *PDFRenderSuite) postRender(body requests.GeneratePDFRequest) renderPostResult {
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
func (s *PDFRenderSuite) assertStoredRender(args renderObjectArgs) helpers.DownloadObjectResult {
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

// TestHappyPathSingleItem renders one item and asserts every observable side
// effect: exact 200 envelope, stored PDF semantics, hash metadata and the
// cache entry. The PDF is kept under testdata/output for visual review.
func (s *PDFRenderSuite) TestHappyPathSingleItem() {
	// Mutant: returning a fixed key instead of the uploaded one breaks the
	// envelope assert; skipping the upload breaks the download below.
	t := s.T()

	marker := xid.New().String()
	html := helpers.RenderTemplate(t, helpers.TemplateArgs{Marker: marker}).HTML
	key := "renders/" + marker + ".pdf"

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

	cached := helpers.GetRedisValue(t, helpers.GetRedisValueArgs{Key: keys.CacheKey})
	require.True(t, cached.Found, "render must create a cache entry")
	require.Equal(t, key, cached.Value, "cache entry must point at the uploaded key")

	saved := helpers.SavePDF(t, helpers.SavePDFArgs{Suite: "render", Test: "happy", Bytes: downloaded.Bytes})
	require.FileExists(t, saved.Path)
}

// TestMergeMultipleItems renders two items with distinct markers and asserts
// the merged document carries both pages and both markers. The PDF is kept
// under testdata/output for visual review.
func (s *PDFRenderSuite) TestMergeMultipleItems() {
	// Mutant: merging only the first item breaks the page count assert;
	// dropping an item breaks its marker assert.
	t := s.T()

	firstMarker := xid.New().String()
	secondMarker := xid.New().String()
	firstHTML := helpers.RenderTemplate(t, helpers.TemplateArgs{Marker: firstMarker}).HTML
	secondHTML := helpers.RenderTemplate(t, helpers.TemplateArgs{Marker: secondMarker}).HTML
	key := "renders/merge-" + firstMarker + ".pdf"

	body := buildRenderRequest(renderRequestArgs{
		Items: []string{firstHTML, secondHTML},
		Key:   key,
	})
	keys := expectedRenderKeys(t, renderKeysArgs{HashGen: s.hashGen, Request: body})

	response := s.postRender(body)
	require.Equal(t, key, response.Key)

	downloaded := s.assertStoredRender(renderObjectArgs{
		Key:     key,
		Pages:   2,
		Markers: []string{firstMarker, secondMarker},
		Hash:    keys.Hash,
	})

	saved := helpers.SavePDF(t, helpers.SavePDFArgs{Suite: "render", Test: "merge", Bytes: downloaded.Bytes})
	require.FileExists(t, saved.Path)
}

// TestIdempotentRepeatRequest posts the same body twice and asserts the second
// call is a hit: identical envelope, same key and a byte-identical object
// (a re-render would embed fresh Chromium timestamps).
func (s *PDFRenderSuite) TestIdempotentRepeatRequest() {
	// Mutant: skipping the render-hash identity check turns the repeat into
	// a re-render, changing the stored bytes and breaking the equality assert.
	t := s.T()

	marker := xid.New().String()
	html := helpers.RenderTemplate(t, helpers.TemplateArgs{Marker: marker}).HTML
	key := "renders/idempotent-" + marker + ".pdf"

	body := buildRenderRequest(renderRequestArgs{Items: []string{html}, Key: key})
	keys := expectedRenderKeys(t, renderKeysArgs{HashGen: s.hashGen, Request: body})

	first := s.postRender(body)
	second := s.postRender(body)
	require.Equal(t, first, second, "repeat request must return the identical envelope")
	require.Equal(t, key, second.Key)

	firstStored := s.assertStoredRender(renderObjectArgs{
		Key:     key,
		Pages:   1,
		Markers: []string{marker},
		Hash:    keys.Hash,
	})
	secondDownload := helpers.DownloadObject(t, helpers.DownloadObjectArgs{
		Bucket: TestBucket,
		Key:    key,
	})
	require.Equal(t, firstStored.Bytes, secondDownload.Bytes,
		"repeat request must leave the stored object byte-identical")
}

// TestNamespaceIsolation posts the same HTML under two namespaces and asserts
// both renders land on distinct keys, distinct objects and distinct cache entries.
func (s *PDFRenderSuite) TestNamespaceIsolation() {
	// Mutant: dropping the namespace segment from the cache key makes both
	// requests share one entry, so the namespaced lookups below miss.
	t := s.T()

	marker := xid.New().String()
	html := helpers.RenderTemplate(t, helpers.TemplateArgs{Marker: marker}).HTML

	firstNamespace := "teamalpha"
	secondNamespace := "teambeta"
	firstKey := "renders/alpha-" + marker + ".pdf"
	secondKey := "renders/beta-" + marker + ".pdf"

	firstBody := buildRenderRequest(renderRequestArgs{
		Items:     []string{html},
		Key:       firstKey,
		Namespace: firstNamespace,
	})
	secondBody := buildRenderRequest(renderRequestArgs{
		Items:     []string{html},
		Key:       secondKey,
		Namespace: secondNamespace,
	})

	firstResponse := s.postRender(firstBody)
	secondResponse := s.postRender(secondBody)
	require.NotEqual(t, firstResponse.Key, secondResponse.Key,
		"namespaces must isolate stored objects")

	firstKeys := expectedRenderKeys(t, renderKeysArgs{
		HashGen:   s.hashGen,
		Request:   firstBody,
		Namespace: firstNamespace,
	})
	secondKeys := expectedRenderKeys(t, renderKeysArgs{
		HashGen:   s.hashGen,
		Request:   secondBody,
		Namespace: secondNamespace,
	})
	require.NotEqual(t, firstKeys.CacheKey, secondKeys.CacheKey,
		"namespaces must isolate cache entries")

	s.assertStoredRender(renderObjectArgs{
		Key:     firstKey,
		Pages:   1,
		Markers: []string{marker},
		Hash:    firstKeys.Hash,
	})
	s.assertStoredRender(renderObjectArgs{
		Key:     secondKey,
		Pages:   1,
		Markers: []string{marker},
		Hash:    secondKeys.Hash,
	})

	firstCached := helpers.GetRedisValue(t, helpers.GetRedisValueArgs{Key: firstKeys.CacheKey})
	require.True(t, firstCached.Found, "first namespace must own a cache entry")
	require.Equal(t, firstKey, firstCached.Value)

	secondCached := helpers.GetRedisValue(t, helpers.GetRedisValueArgs{Key: secondKeys.CacheKey})
	require.True(t, secondCached.Found, "second namespace must own a cache entry")
	require.Equal(t, secondKey, secondCached.Value)
}

func TestPDFRenderSuite(t *testing.T) {
	suite.Run(t, new(PDFRenderSuite))
}
