package e2e

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/application/use_cases"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"
	sharedDefinitions "github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
	sharedImplementations "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/implementations"
	"github.com/PChaparro/serpentarius/tests/e2e/stubs"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// This suite covers GeneratePDFUseCase.Execute directly with the shared
// fakes (no Chromium, no network). The real xxhash generator is used so the
// cache-key derivation under test is the production one.

// useCaseObjectSeed selects which stored object backs a pre-seeded cache entry.
type useCaseObjectSeed int

const (
	// objectAbsent seeds a cache entry whose key points at nothing: StatFile
	// reports Exists=false.
	objectAbsent useCaseObjectSeed = iota
	// objectWithMatchingHash seeds the object tagged with the current request hash.
	objectWithMatchingHash
	// objectWithForeignHash seeds the object tagged with an unrelated hash.
	objectWithForeignHash
)

// pdfUseCaseCase is one Execute scenario: stub seeding plus the observable
// effects expected afterwards (returned reference, render count, stored
// object identity, cache entry contents).
type pdfUseCaseCase struct {
	name string
	ttl  int64
	// namespace is the request cache namespace; empty means no namespace segment.
	namespace string
	// seedCache pre-seeds the derived cache key pointing at cachedKey.
	seedCache  bool
	cachedKey  string
	objectSeed useCaseObjectSeed
	// generateErr makes the generator stub fail; uploadErr makes the cloud
	// storage stub fail.
	generateErr error
	uploadErr   error
	// wantErr is non-empty when Execute must fail with an error containing it.
	wantErr string
	// wantKey is the expected result Key on success.
	wantKey string
	// wantGenerateCalls is the only stub-call count asserted: rendering is
	// the expensive side effect the cache exists to skip. Upload effects are
	// asserted through stored object state instead.
	wantGenerateCalls int
}

// pdfRequestArgs groups the fields of the DTO built for each scenario.
type pdfRequestArgs struct {
	Bucket    string
	Key       string
	TTL       int64
	Namespace string
}

// basePDFRequest builds a minimal but complete generation request.
func basePDFRequest(args pdfRequestArgs) *dto.PDFGenerationDTO {
	return &dto.PDFGenerationDTO{
		Items:  []dto.PDFItem{{BodyHTML: "<p>Quarterly summary</p>"}},
		Output: dto.OutputConfig{Bucket: args.Bucket, Key: args.Key},
		Cache:  dto.CacheConfig{TTL: args.TTL, Namespace: args.Namespace},
	}
}

// renderHashArgs groups the inputs for deriving the request hash in tests.
type renderHashArgs struct {
	HashGen sharedDefinitions.HashGenerator
	Request *dto.PDFGenerationDTO
}

// mustRenderHash replays the key derivation contract of the use case
// (hash over the marshalled request) so tests can seed cache entries and
// stored objects for the exact request under test.
func mustRenderHash(t *testing.T, args renderHashArgs) string {
	t.Helper()

	raw, err := json.Marshal(args.Request)
	require.NoError(t, err, "request must be marshallable")

	hash, err := args.HashGen.GenerateHash(string(raw))
	require.NoError(t, err, "hash generator must not fail")

	return hash
}

// PDFUseCaseSuite exercises the cache HIT / stale / MISS / error branches of
// the use case against the shared stubs.
type PDFUseCaseSuite struct {
	suite.Suite
	useCase *use_cases.GeneratePDFUseCase
	hashGen sharedDefinitions.HashGenerator
}

func (s *PDFUseCaseSuite) SetupSuite() {
	s.hashGen = sharedImplementations.GetXxHashGenerator()
	s.useCase = &use_cases.GeneratePDFUseCase{
		PDFGenerator:     PDFGeneratorStub,
		CloudStorage:     CloudStorageStub,
		PathCacheStorage: CacheStorageStub,
		HashGenerator:    s.hashGen,
	}
}

func (s *PDFUseCaseSuite) SetupTest() {
	PDFGeneratorStub.Reset()
	CloudStorageStub.Reset()
	CacheStorageStub.Reset()
}

// TestExecute runs every cache and failure branch and asserts the observable
// contract of each: returned reference, render count, stored object identity
// and cache entry contents.
func (s *PDFUseCaseSuite) TestExecute() {
	const bucket = "test-bucket"
	const outputKey = "reports/quarterly.pdf"
	const cachedKey = "reports/cached.pdf"

	cases := []pdfUseCaseCase{
		{
			name: "cache hit returns stored key without rendering",
			// Mutant: skipping the render-hash comparison returns the cached
			// key without verifying identity, hiding stale objects.
			ttl:               600,
			seedCache:         true,
			cachedKey:         cachedKey,
			objectSeed:        objectWithMatchingHash,
			wantKey:           cachedKey,
			wantGenerateCalls: 0,
		},
		{
			name: "stale entry with foreign render hash is re-rendered",
			// Mutant: dropping the Delete call leaves the stale entry behind,
			// so the cache keeps pointing at the foreign object.
			ttl:               600,
			seedCache:         true,
			cachedKey:         cachedKey,
			objectSeed:        objectWithForeignHash,
			wantKey:           outputKey,
			wantGenerateCalls: 1,
		},
		{
			name: "total cache miss renders and stores without expiration",
			// Mutant: hardcoding a fallback TTL stores an expiration even
			// though the request asked for none (0 means no expiry).
			ttl:               0,
			wantKey:           outputKey,
			wantGenerateCalls: 1,
		},
		{
			name: "stale entry with missing object is re-rendered",
			// Mutant: ignoring StatFile existence treats a dangling cache
			// pointer as a hit and returns a key that does not exist.
			ttl:               600,
			seedCache:         true,
			cachedKey:         cachedKey,
			objectSeed:        objectAbsent,
			wantKey:           outputKey,
			wantGenerateCalls: 1,
		},
		{
			name: "generator failure returns error without storing anything",
			// Mutant: swallowing the generator error uploads an empty
			// payload and caches a key for a render that never happened.
			ttl:               600,
			generateErr:       errors.New("stub generator failure"),
			wantErr:           "stub generator failure",
			wantGenerateCalls: 1,
		},
		{
			name: "upload failure returns error without caching",
			// Mutant: caching the key despite the failed upload leaves a
			// pointer at an object that was never stored.
			ttl:               600,
			uploadErr:         errors.New("stub upload failure"),
			wantErr:           "stub upload failure",
			wantGenerateCalls: 1,
		},
		{
			name: "positive ttl is stored as entry expiration",
			// Mutant: ignoring the request TTL stores the entry without
			// expiration, keeping renders cached forever.
			ttl:               3600,
			wantKey:           outputKey,
			wantGenerateCalls: 1,
		},
		{
			name: "namespaced request uses namespaced cache key",
			// Mutant: dropping the namespace segment makes namespaced and
			// unnamespaced requests share one cache entry.
			ttl:               300,
			namespace:         "finance",
			wantKey:           outputKey,
			wantGenerateCalls: 1,
		},
	}

	for _, tc := range cases {
		tc := tc
		s.Run(tc.name, func() {
			t := s.T()

			PDFGeneratorStub.Reset()
			CloudStorageStub.Reset()
			CacheStorageStub.Reset()
			PDFGeneratorStub.ReturnError = tc.generateErr
			CloudStorageStub.ReturnUploadError = tc.uploadErr

			request := basePDFRequest(pdfRequestArgs{
				Bucket:    bucket,
				Key:       outputKey,
				TTL:       tc.ttl,
				Namespace: tc.namespace,
			})
			hash := mustRenderHash(t, renderHashArgs{HashGen: s.hashGen, Request: request})

			cacheKey := "pdf:render:" + hash
			if tc.namespace != "" {
				cacheKey = "pdf:render:" + tc.namespace + ":" + hash
			}

			if tc.seedCache {
				// A hit never rewrites the entry, so seeding it with the
				// scenario TTL lets the post-asserts prove it was left intact.
				CacheStorageStub.Entries[cacheKey] = stubs.CacheEntry{Value: tc.cachedKey, Expiration: tc.ttl}
				switch tc.objectSeed {
				case objectWithMatchingHash:
					CloudStorageStub.Objects[bucket+"/"+tc.cachedKey] = stubs.StoredObject{
						Metadata: map[string]string{"render-hash": hash},
					}
				case objectWithForeignHash:
					CloudStorageStub.Objects[bucket+"/"+tc.cachedKey] = stubs.StoredObject{
						Metadata: map[string]string{"render-hash": "stale-render-hash"},
					}
				}
			}

			result, err := s.useCase.Execute(request)

			if tc.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.wantErr)
				require.Nil(t, result, "failed execution must not return a reference")
				require.Empty(t, CacheStorageStub.Entries, "failed execution must not cache")
				require.Empty(t, CloudStorageStub.Objects, "failed execution must not store")
				require.Equal(t, tc.wantGenerateCalls, PDFGeneratorStub.GenerateCalls)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, bucket, result.Bucket)
			require.Equal(t, tc.wantKey, result.Key)
			require.Equal(t, tc.wantGenerateCalls, PDFGeneratorStub.GenerateCalls)

			require.Len(t, CacheStorageStub.Entries, 1, "one cache entry must exist")
			entry, ok := CacheStorageStub.Entries[cacheKey]
			require.True(t, ok, "cache entry must live under the derived key %q", cacheKey)
			require.Equal(t, tc.wantKey, entry.Value)
			require.Equal(t, tc.ttl, entry.Expiration)

			stat, err := CloudStorageStub.StatFile(sharedDefinitions.FileExistsRequest{
				Bucket: bucket,
				Key:    tc.wantKey,
			})
			require.NoError(t, err)
			require.True(t, stat.Exists, "stored object must exist")
			require.Equal(t, hash, stat.Metadata["render-hash"],
				"stored object must carry the current request hash")
		})
	}
}

func TestPDFUseCaseSuite(t *testing.T) {
	suite.Run(t, new(PDFUseCaseSuite))
}
