package stubs

import (
	"bytes"
	"io"
	"sync"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"
)

// defaultStubPDFBytes is the minimal payload returned when PDFBytes is empty.
// It only carries the %PDF- magic header; semantic PDF asserts in later phases
// use payloads injected per test via PDFBytes.
var defaultStubPDFBytes = []byte("%PDF-1.4 stub\n%%EOF")

// StubPDFGenerator is a test double for definitions.PDFGenerator.
// GenerateCalls counts renders so cache HIT/MISS tests can assert the
// non-regeneration feature (the only allowed stub-call counting).
type StubPDFGenerator struct {
	mu            sync.Mutex
	GenerateCalls int
	LastRequest   *dto.PDFGenerationDTO
	PDFBytes      []byte // payload returned per call; defaults to defaultStubPDFBytes
	ReturnError   error  // when set, GeneratePDF returns this error
}

// GeneratePDF records the call and returns the configured payload.
func (s *StubPDFGenerator) GeneratePDF(request *dto.PDFGenerationDTO) (io.Reader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.GenerateCalls++
	s.LastRequest = request

	if s.ReturnError != nil {
		return nil, s.ReturnError
	}

	payload := s.PDFBytes
	if len(payload) == 0 {
		payload = defaultStubPDFBytes
	}

	return bytes.NewReader(payload), nil
}

// Reset clears all captured state. Call it in SetupTest when the stub is
// shared across tests within the same suite.
func (s *StubPDFGenerator) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.GenerateCalls = 0
	s.LastRequest = nil
	s.PDFBytes = nil
	s.ReturnError = nil
}
