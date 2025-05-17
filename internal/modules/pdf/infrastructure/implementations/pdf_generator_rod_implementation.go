package implementations

import "github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"

type PDFGeneratorRod struct{}

// GeneratePDF generates a PDF based on the provided request.
// It returns the generated PDF as a byte slice and an error if any occurred.
func (p *PDFGeneratorRod) GeneratePDF(request dto.PDFGenerationDTO) ([]byte, error) {
	return nil, nil
}
