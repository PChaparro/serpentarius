package implementations

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"
	sharedInfrastructure "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	pdfProcessingAPI "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfProcessingModel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type PDFGeneratorRod struct{}

// buildPDFOptions creates and configures a proto.PagePrintToPDF object based on the provided item configuration
func (p *PDFGeneratorRod) buildPDFOptions(config *dto.ItemConfig) *proto.PagePrintToPDF {
	// Create default PDF options
	pdfOpts := &proto.PagePrintToPDF{}

	if config == nil {
		return pdfOpts
	}

	// Set orientation
	if config.Orientation != nil {
		pdfOpts.Landscape = *config.Orientation == "landscape"
	}

	// Set display header/footer flag
	if config.DisplayHeaderFooter != nil {
		pdfOpts.DisplayHeaderFooter = *config.DisplayHeaderFooter
	}

	// Set print background flag
	if config.PrintBackground != nil {
		pdfOpts.PrintBackground = *config.PrintBackground
	}

	// Set scale
	if config.Scale != nil {
		pdfOpts.Scale = config.Scale
	}

	// Set page size (width and height)
	if config.Size != nil {
		if config.Size.Width != nil {
			pdfOpts.PaperWidth = config.Size.Width
		}
		if config.Size.Height != nil {
			pdfOpts.PaperHeight = config.Size.Height
		}
	}

	// Set margins
	if config.Margin != nil {
		if config.Margin.Top != nil {
			pdfOpts.MarginTop = config.Margin.Top
		}
		if config.Margin.Bottom != nil {
			pdfOpts.MarginBottom = config.Margin.Bottom
		}
		if config.Margin.Left != nil {
			pdfOpts.MarginLeft = config.Margin.Left
		}
		if config.Margin.Right != nil {
			pdfOpts.MarginRight = config.Margin.Right
		}
	}

	// Set page range
	if config.PageRanges != nil {
		pdfOpts.PageRanges = fmt.Sprintf("%d-%d", config.PageRanges.Start, config.PageRanges.End)
	}

	// Set header and footer HTML
	if config.HeaderHTML != nil {
		pdfOpts.HeaderTemplate = *config.HeaderHTML
	}
	if config.FooterHTML != nil {
		pdfOpts.FooterTemplate = *config.FooterHTML
	}

	return pdfOpts
}

// mergePDFs receives multiple io.Reader and returns a single combined PDF as io.Reader
func (p *PDFGeneratorRod) mergePDFs(readers []io.Reader) (io.Reader, error) {
	// Create slices for temporary files
	tempFilesNames := make([]string, len(readers))

	// Create a temporary directory for PDF files
	tempDir, err := os.MkdirTemp("", "pdf_merge")
	if err != nil {
		return nil, fmt.Errorf("error creating temporary directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			sharedInfrastructure.GetLogger().WithError(err).Error("error removing temporary directory")
		}
	}()

	// Save each reader as a temporary file
	for readerIndex, currentReader := range readers {
		// Create an unique identifier for the temporary file
		tempFileRandomId := sharedInfrastructure.GenerateXID()

		// Create a temporary file for each PDF
		tempFileName := filepath.Join(
			tempDir,
			fmt.Sprintf("temp_%s.pdf", tempFileRandomId),
		)
		tempFile, err := os.Create(tempFileName)
		if err != nil {
			return nil, fmt.Errorf("error creating temporary file: %w", err)
		}
		defer func() {
			if err := os.Remove(tempFileName); err != nil {
				sharedInfrastructure.GetLogger().WithError(err).Error("error removing temporary file")
			}
		}()

		_, err = io.Copy(tempFile, currentReader)
		if err != nil {
			return nil, fmt.Errorf("error writing to temporary file: %w", err)
		}

		tempFilesNames[readerIndex] = tempFileName
	}

	// Create an unique identifier for the merged file
	mergedFileRandomId := sharedInfrastructure.GenerateXID()

	// Create output file for the merged PDF
	mergedFileName := filepath.Join(
		tempDir,
		fmt.Sprintf("merged_%s.pdf", mergedFileRandomId),
	)

	// Merge the temporary files into the output file
	if err := pdfProcessingAPI.MergeCreateFile(
		tempFilesNames,
		mergedFileName,
		false,
		pdfProcessingModel.NewDefaultConfiguration(),
	); err != nil {
		return nil, fmt.Errorf("error merging PDF files: %w", err)
	}

	// Read the merged file
	mergedContent, err := os.ReadFile(mergedFileName)
	if err != nil {
		return nil, fmt.Errorf("error reading merged PDF file: %w", err)
	}

	// Return the merged content as an io.Reader
	return bytes.NewReader(mergedContent), nil
}

// GeneratePDF generates a PDF based on the provided request.
// It creates a new browser instance, generates individual PDFs for each item in the request,
// and merges them into a single PDF file.
func (p *PDFGeneratorRod) GeneratePDF(request *dto.PDFGenerationDTO) (io.Reader, error) {
	// Create a new browser instance
	launcherURL := launcher.New().
		Headless(true).                    // Run without a GUI
		Leakless(true).                    // Prevent memory leaks
		Set("disable-gpu", "1").           // Disable GPU acceleration
		Set("disable-dev-shm-usage", "1"). // Disable /dev/shm usage
		Set("disable-extensions", "1").    // Disable extensions
		MustLaunch()

	browser := rod.New().ControlURL(launcherURL).MustConnect()
	defer browser.MustClose()

	// Create a PDF files slice to merge later
	readers := make([]io.Reader, len(request.Items))

	// Create a page for each item in the request
	for itemIndex, item := range request.Items {
		// Build PDF options from item configuration
		pdfOpts := p.buildPDFOptions(item.Config)

		// Create a new page
		page := browser.MustPage()
		defer page.MustClose()

		// Set HTML content of the page
		err := page.SetDocumentContent(item.BodyHTML)
		if err != nil {
			return nil, fmt.Errorf("error setting document content: %w", err)
		}

		// Wait for the page to load
		page.MustWaitLoad().MustWaitIdle()

		// Generate PDF
		pdf, err := page.PDF(pdfOpts)
		if err != nil {
			return nil, err
		}

		// Store the generated PDF in the readers slice
		readers[itemIndex] = pdf
	}

	// Merge all generated PDFs into a single PDF
	mergedPDF, err := p.mergePDFs(readers)
	if err != nil {
		return nil, fmt.Errorf("error merging PDFs: %w", err)
	}

	return mergedPDF, nil
}
