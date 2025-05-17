package definitions

import (
	"io"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"
)

// PDFGenerator is the interface for generating PDFs
type PDFGenerator interface {
	// GeneratePDF generates a PDF based on the provided request.
	// It returns the generated PDF as an stream and an error if any occurred.
	GeneratePDF(request *dto.PDFGenerationDTO) (io.Reader, error)
}
