package use_cases

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	// Generate the PDF
	pdfReader, err := u.PDFGenerator.GeneratePDF(request)
	if err != nil {
		return "", err
	}

	// Just use the filename from the request
	fileName := request.Config.FileName

	// Create the local file
	file, err := os.Create(fileName)
	if err != nil {
		return "", fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	// Copy the PDF content to the file
	if _, err := io.Copy(file, pdfReader); err != nil {
		return "", fmt.Errorf("error writing to file: %w", err)
	}

	// Return the absolute path to the file as a URL for now
	absPath, err := filepath.Abs(fileName)
	if err != nil {
		return fileName, nil // Return the filename if absolute path fails
	}

	// Return file:// URL for local testing
	return "file://" + absPath, nil
}
