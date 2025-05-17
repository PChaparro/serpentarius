package use_cases

import (
	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/definitions"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"
)

// GeneratePDFReturningURLUseCase is the use case for generating a PDF and returning its public URL.
type GeneratePDFReturningURLUseCase struct {
	// PDFGenerator is the interface for generating PDFs
	PDFGenerator definitions.PDFGenerator
}

// Execute generates a PDF based on the provided request and returns the URL of the generated PDF.
func (u *GeneratePDFReturningURLUseCase) Execute(
	request *dto.PDFGenerationDTO,
) (string, error) {
	return "", nil
}
