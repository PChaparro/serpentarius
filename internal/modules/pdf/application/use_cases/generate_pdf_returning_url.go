package use_cases

import (
	"fmt"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/definitions"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"
	sharedDefinitions "github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
)

// GeneratePDFReturningURLUseCase is the use case for generating a PDF and returning its public URL.
type GeneratePDFReturningURLUseCase struct {
	// PDFGenerator is the interface for generating PDFs
	PDFGenerator definitions.PDFGenerator
	// CloudStorage is the interface for cloud storage operations
	CloudStorage sharedDefinitions.CloudStorage
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

	// Upload the PDF to cloud storage
	uploadRequest := sharedDefinitions.UploadFileRequest{
		FileReader:      pdfReader,
		FileFolder:      request.Config.Directory,
		FilePath:        request.Config.FileName,
		ContentType:     "application/pdf",
		PublicURLPrefix: request.Config.PublicURLPrefix,
	}
	url, err := u.CloudStorage.UploadFile(uploadRequest)
	if err != nil {
		return "", fmt.Errorf("error uploading file to cloud storage: %w", err)
	}

	// Return the public URL of the uploaded PDF
	return url, nil
}
