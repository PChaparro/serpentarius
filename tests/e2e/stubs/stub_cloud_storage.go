package stubs

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	sharedDefinitions "github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
)

// StoredObject is the in-memory representation of an object in cloud storage.
type StoredObject struct {
	Bytes    []byte
	Metadata map[string]string
}

// StubCloudStorage is a test double for definitions.CloudStorage backed by a
// map keyed by "bucket/key". It covers the stale-object branches (missing
// object, mismatched render-hash) and upload failures in later phases.
type StubCloudStorage struct {
	mu                sync.Mutex
	Objects           map[string]StoredObject
	UploadCalls       int
	StatCalls         int
	ReturnUploadError error // when set, UploadFile returns this error
	ReturnStatError   error // when set, StatFile returns this error
}

// objectKey builds the map key for a bucket/key pair.
func objectKey(bucket, key string) string {
	return fmt.Sprintf("%s/%s", bucket, key)
}

// UploadFile stores the payload in memory and echoes the requested key,
// mirroring the real S3 contract (the object lives at the given key).
func (s *StubCloudStorage) UploadFile(request sharedDefinitions.UploadFileRequest) (string, error) {
	payload, err := io.ReadAll(request.FileReader)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.UploadCalls++

	if s.ReturnUploadError != nil {
		return "", s.ReturnUploadError
	}

	if s.Objects == nil {
		s.Objects = make(map[string]StoredObject)
	}

	metadata := make(map[string]string, len(request.Metadata))
	for k, v := range request.Metadata {
		metadata[k] = v
	}

	s.Objects[objectKey(request.Bucket, request.Key)] = StoredObject{
		Bytes:    bytes.Clone(payload),
		Metadata: metadata,
	}

	return request.Key, nil
}

// StatFile reports existence and stored metadata for a bucket/key pair.
func (s *StubCloudStorage) StatFile(request sharedDefinitions.FileExistsRequest) (sharedDefinitions.StatFileResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.StatCalls++

	if s.ReturnStatError != nil {
		return sharedDefinitions.StatFileResult{Exists: false}, s.ReturnStatError
	}

	stored, ok := s.Objects[objectKey(request.Bucket, request.Key)]
	if !ok {
		return sharedDefinitions.StatFileResult{Exists: false}, nil
	}

	return sharedDefinitions.StatFileResult{Exists: true, Metadata: stored.Metadata}, nil
}

// Reset clears all captured state. Call it in SetupTest when the stub is
// shared across tests within the same suite.
func (s *StubCloudStorage) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Objects = make(map[string]StoredObject)
	s.UploadCalls = 0
	s.StatCalls = 0
	s.ReturnUploadError = nil
	s.ReturnStatError = nil
}
