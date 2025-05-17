package implementations

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"
	sharedInfrastructure "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	pdfProcessingAPI "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfProcessingModel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// Pool configuration constants
const (
	// MaxBrowsers is the maximum number of browser instances to maintain in the pool
	MaxBrowsers = 4

	// MaxPagesPerBrowser is the maximum number of pages per browser to maintain in the pool
	MaxPagesPerBrowser = 8
)

// PagePool holds and manages a pool of browser pages
type PagePool struct {
	pages chan *rod.Page
	mu    sync.Mutex
}

// Browser represents a browser instance in the pool
type Browser struct {
	instance *rod.Browser
	pagePool *PagePool
}

// PDFGeneratorRod implements the PDFGenerator interface using Rod
type PDFGeneratorRod struct {
	// browserPool holds available browsers
	browserPool chan *Browser
	// mutex protects concurrent access to the browserPool
	mutex sync.Mutex
	// initialized indicates if the browser pool has been initialized
	initialized bool
}

// pdfGeneratorInstance is the singleton instance of PDFGeneratorRod
var pdfGeneratorInstance *PDFGeneratorRod
var pdfGeneratorOnce sync.Once

// GetPDFGeneratorRod returns the singleton instance of PDFGeneratorRod
func GetPDFGeneratorRod() *PDFGeneratorRod {
	pdfGeneratorOnce.Do(func() {
		pdfGeneratorInstance = &PDFGeneratorRod{}
		pdfGeneratorInstance.Initialize()

		// Set up cleanup on program exit
		runtime.SetFinalizer(pdfGeneratorInstance, func(p *PDFGeneratorRod) {
			p.ReleaseBrowserPool()
		})
	})

	return pdfGeneratorInstance
}

// NewPagePool creates a new page pool with specified capacity
func NewPagePool(size int) *PagePool {
	return &PagePool{
		pages: make(chan *rod.Page, size),
	}
}

// Initialize sets up the browser pool for PDF generation
func (p *PDFGeneratorRod) Initialize() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.initialized {
		return
	}

	// Determine number of browsers based on GOMAXPROCS but cap at MaxBrowsers
	numBrowsers := min(runtime.GOMAXPROCS(0), MaxBrowsers)

	// Create the browser pool
	p.browserPool = make(chan *Browser, numBrowsers)

	// Initialize the browser pool
	for range numBrowsers {
		// Launch a new browser
		launcherURL := launcher.New().
			Headless(true).                    // Run without a GUI
			Leakless(true).                    // Prevent memory leaks
			Set("disable-gpu", "1").           // Disable GPU acceleration
			Set("disable-dev-shm-usage", "1"). // Disable /dev/shm usage
			Set("disable-extensions", "1").    // Disable extensions
			MustLaunch()

		browser := rod.New().ControlURL(launcherURL).MustConnect()

		// Create a page pool for this browser
		pagePool := NewPagePool(MaxPagesPerBrowser)

		// Add browser to pool
		p.browserPool <- &Browser{
			instance: browser,
			pagePool: pagePool,
		}
	}

	p.initialized = true

	// Log initialization
	sharedInfrastructure.GetLogger().
		WithField("browsers", numBrowsers).
		WithField("pages_per_browser", MaxPagesPerBrowser).
		Info("PDF generator browser pool initialized")
}

// getBrowser gets a browser from the pool or waits if none are available
func (p *PDFGeneratorRod) getBrowser() *Browser {
	return <-p.browserPool
}

// returnBrowser returns a browser to the pool
func (p *PDFGeneratorRod) returnBrowser(browser *Browser) {
	p.browserPool <- browser
	sharedInfrastructure.GetLogger().Debug("Browser freed up")
}

// ReleaseBrowserPool frees up resources used by the browser pool
func (p *PDFGeneratorRod) ReleaseBrowserPool() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.initialized {
		return
	}

	// Close browser pool channel
	close(p.browserPool)

	// Close all browsers and their page pools
	browserIndex := 0
	for browser := range p.browserPool {
		// Clean up page pool
		browser.pagePool.ReleasePagePool(func(page *rod.Page) {
			page.MustClose()
		})
		sharedInfrastructure.GetLogger().
			WithField("browser_index", browserIndex).
			Debug("Browser page pool cleaned up")

		// Close browser
		browser.instance.MustClose()
		sharedInfrastructure.GetLogger().
			WithField("browser_index", browserIndex).
			Debug("Browser instance closed")

		browserIndex++
	}

	p.initialized = false
	sharedInfrastructure.GetLogger().Info("PDF generator browser pool cleaned up")
}

// ReleasePagePool cleans up all pages in the pool using the provided cleanup function
func (p *PagePool) ReleasePagePool(cleanup func(*rod.Page)) {
	p.mu.Lock()
	defer p.mu.Unlock()

	close(p.pages)
	for page := range p.pages {
		cleanup(page)
	}
}

// Get retrieves a page from the pool or creates a new one using the provided factory function
func (p *PagePool) Get(create func() *rod.Page) *rod.Page {
	p.mu.Lock()
	defer p.mu.Unlock()

	select {
	case page := <-p.pages:
		sharedInfrastructure.GetLogger().Debug("Page acquired from pool")
		return page
	default:
		sharedInfrastructure.GetLogger().Debug("Creating new page")
		return create()
	}
}

// Put returns a page to the pool
func (p *PagePool) Put(page *rod.Page) {
	p.mu.Lock()
	defer p.mu.Unlock()

	select {
	case p.pages <- page:
		// Successfully put the page back in the pool
		sharedInfrastructure.GetLogger().Debug("Page returned to pool")
	default:
		// Pool is full, close the page
		sharedInfrastructure.GetLogger().Debug("Page pool is full, closing page")
		page.MustClose()
	}
}

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
// It uses the browser and page pools to generate individual PDFs for each item in the request,
// and merges them into a single PDF file.
func (p *PDFGeneratorRod) GeneratePDF(request *dto.PDFGenerationDTO) (io.Reader, error) {
	// Initialize the browser pool if not already done
	if !p.initialized {
		p.Initialize()
	}

	// Create a PDF files slice to merge later
	readers := make([]io.Reader, len(request.Items))

	// Create a wait group to wait for all PDFs to be generated
	var wg sync.WaitGroup
	var mu sync.Mutex // Mutex to protect concurrent access to readers slice
	var processingErr error

	// Get a browser from the pool to process all items in the request
	browser := p.getBrowser()
	sharedInfrastructure.GetLogger().Debug("Browser acquired")
	defer p.returnBrowser(browser)

	// Process each item in the request
	for itemIndex, item := range request.Items {
		wg.Add(1)

		// Use goroutine to process items concurrently
		go func(idx int, pdfItem dto.PDFItem) {
			defer wg.Done()

			// Build PDF options from item configuration
			pdfOpts := p.buildPDFOptions(pdfItem.Config)

			// Create or get a page from pool
			createPage := func() *rod.Page {
				return browser.instance.MustIncognito().MustPage()
			}

			page := browser.pagePool.Get(createPage)
			defer browser.pagePool.Put(page)

			// Set HTML content of the page
			err := page.SetDocumentContent(pdfItem.BodyHTML)
			if err != nil {
				mu.Lock()
				if processingErr == nil {
					processingErr = fmt.Errorf("error setting document content: %w", err)
				}
				mu.Unlock()
				return
			}

			// Wait for the page to load
			page.MustWaitLoad().MustWaitIdle()

			// Generate PDF
			pdf, err := page.PDF(pdfOpts)
			if err != nil {
				mu.Lock()
				if processingErr == nil {
					processingErr = err
				}
				mu.Unlock()
				return
			}

			// Store the generated PDF in the readers slice
			mu.Lock()
			readers[idx] = pdf
			mu.Unlock()
		}(itemIndex, item)
	}

	// Wait for all PDFs to be generated
	wg.Wait()

	// Check if an error occurred during processing
	if processingErr != nil {
		sharedInfrastructure.GetLogger().
			WithError(processingErr).
			Error("error generating PDF")

		return nil, processingErr
	}

	// Merge all generated PDFs into a single PDF
	mergedPDF, err := p.mergePDFs(readers)
	if err != nil {
		return nil, fmt.Errorf("error merging PDFs: %w", err)
	}

	return mergedPDF, nil
}
