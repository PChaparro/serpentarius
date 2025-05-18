// Package implementations provides concrete implementations of the PDF generation interfaces.
// This file contains the Rod-based PDF generator implementation which uses the Rod library
// to control headless Chrome browsers for PDF generation.
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

// Variables defining the resource limits for the browser pool
var (
	// MaxBrowsers defines the maximum number of browser instances to create
	MaxBrowsers = sharedInfrastructure.GetEnvironment().MaxChromiumBrowsers

	// MaxPagesPerBrowser defines the maximum number of pages per browser instance
	MaxPagesPerBrowser = sharedInfrastructure.GetEnvironment().MaxChromiumTabsPerBrowser
)

// PageWithBrowser associates a Rod Page with its parent Browser instance.
// This structure is used in the page pool to track which page belongs to which browser.
type PageWithBrowser struct {
	Page    *rod.Page    // The browser page instance for rendering content
	Browser *rod.Browser // The parent browser instance that owns this page
}

// PDFGeneratorRod implements PDF generation functionality using the Rod library
// to control headless Chrome browsers. It maintains a pool of browser instances and pages
// to optimize resource usage and improve performance with concurrent PDF generation tasks.
type PDFGeneratorRod struct {
	pagePool    chan *PageWithBrowser // Pool of available browser pages for PDF generation
	mutex       sync.Mutex            // Mutex to protect concurrent access to the generator state
	initialized bool                  // Flag indicating if the generator has been initialized
	browsers    []*rod.Browser        // List of browser instances managed by this generator
}

// Global singleton instance and initialization control
var pdfGeneratorInstance *PDFGeneratorRod
var pdfGeneratorOnce sync.Once

// GetPDFGeneratorRod returns the singleton instance of the PDF generator.
// It initializes the generator on the first call and sets up a finalizer
// to ensure resources are properly released when the generator is garbage collected.
// This follows the singleton pattern to ensure there's only one instance
// managing the browser pool across the application.
func GetPDFGeneratorRod() *PDFGeneratorRod {
	pdfGeneratorOnce.Do(func() {
		pdfGeneratorInstance = &PDFGeneratorRod{}
		pdfGeneratorInstance.Initialize()

		// Set up a finalizer to clean up resources when the generator is garbage collected
		runtime.SetFinalizer(pdfGeneratorInstance, func(p *PDFGeneratorRod) {
			p.ReleaseBrowserPool()
		})
	})

	return pdfGeneratorInstance
}

// Initialize sets up the browser pool and page pool for PDF generation.
// It creates multiple browser instances and initializes pages for each browser,
// adding them to the page pool for future use. This method is thread-safe
// and will only initialize the generator once, regardless of how many times it's called.
func (p *PDFGeneratorRod) Initialize() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Skip initialization if already done
	if p.initialized {
		return
	}

	// Determine the optimal number of browsers based on system resources,
	// but never exceed the defined maximum
	numBrowsers := min(runtime.GOMAXPROCS(0), MaxBrowsers)

	// Create the page pool channel with capacity for all pages across all browsers
	p.pagePool = make(chan *PageWithBrowser, numBrowsers*MaxPagesPerBrowser)

	// Create and configure each browser instance
	for i := 0; i < numBrowsers; i++ {
		// Launch a new browser instance with optimized settings for headless PDF generation
		launcherURL := launcher.New().
			Bin(sharedInfrastructure.GetEnvironment().ChromiumBinaryPath). // Use the configured Chromium binary
			Headless(true).                                                // Run in headless mode (no UI)
			Leakless(true).                                                // Ensure process cleanup on unexpected termination
			Set("disable-gpu", "1").                                       // Disable GPU acceleration
			Set("disable-dev-shm-usage", "1").                             // Avoid using shared memory
			Set("disable-extensions", "1").                                // Disable browser extensions
			MustLaunch()

		// Connect to the launched browser
		browser := rod.New().ControlURL(launcherURL).MustConnect()
		p.browsers = append(p.browsers, browser)

		// Create pages for this browser and add them to the page pool
		for j := 0; j < MaxPagesPerBrowser; j++ {
			page := browser.MustIncognito().MustPage() // Use incognito for isolation
			p.pagePool <- &PageWithBrowser{Page: page, Browser: browser}
		}
	}

	p.initialized = true

	// Log initialization details
	sharedInfrastructure.GetLogger().
		WithField("browsers", numBrowsers).
		WithField("pages_total", numBrowsers*MaxPagesPerBrowser).
		Info("PDF generator page pool initialized")
}

// RequestPage retrieves an available page from the page pool.
// This method will block until a page becomes available if all pages are currently in use.
// The caller is responsible for returning the page to the pool after use.
func (p *PDFGeneratorRod) RequestPage() *PageWithBrowser {
	return <-p.pagePool
}

// ReturnPage returns a previously requested page back to the page pool,
// making it available for future use by other operations.
// This should be called after a page is no longer needed to prevent resource leaks.
func (p *PDFGeneratorRod) ReturnPage(pwb *PageWithBrowser) {
	p.pagePool <- pwb
}

// ReleaseBrowserPool cleans up all browser resources managed by this generator.
// It closes all browser instances and releases associated resources.
// This method is thread-safe and idempotent, so it's safe to call multiple times.
func (p *PDFGeneratorRod) ReleaseBrowserPool() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Skip if not initialized
	if !p.initialized {
		return
	}

	// Close the page pool channel to prevent further page requests
	close(p.pagePool)

	// Close each browser instance
	for _, browser := range p.browsers {
		browser.MustClose()
	}

	// Reset the state to allow for potential re-initialization
	p.browsers = nil
	p.initialized = false

	// Log the cleanup
	sharedInfrastructure.GetLogger().Info("PDF generator browser pool cleaned up")
}

// buildPDFOptions converts a configuration object from the domain DTO into Chrome's PDF print options.
// It applies all specified configuration parameters such as orientation, margins, headers/footers, etc.
// If config is nil, default options will be returned.
func (p *PDFGeneratorRod) buildPDFOptions(config *dto.ItemConfig) *proto.PagePrintToPDF {
	pdfOpts := &proto.PagePrintToPDF{}
	if config == nil {
		return pdfOpts // Return default options if no config provided
	}

	// Set page orientation (portrait or landscape)
	if config.Orientation != nil {
		pdfOpts.Landscape = *config.Orientation == "landscape"
	}

	// Configure header and footer visibility
	if config.DisplayHeaderFooter != nil {
		pdfOpts.DisplayHeaderFooter = *config.DisplayHeaderFooter
	}

	// Configure background elements printing
	if config.PrintBackground != nil {
		pdfOpts.PrintBackground = *config.PrintBackground
	}

	// Set scale factor for the content
	if config.Scale != nil {
		pdfOpts.Scale = config.Scale
	}

	// Configure page size dimensions
	if config.Size != nil {
		if config.Size.Width != nil {
			pdfOpts.PaperWidth = config.Size.Width
		}
		if config.Size.Height != nil {
			pdfOpts.PaperHeight = config.Size.Height
		}
	}

	// Configure page margins
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

	// Set page range to be printed
	if config.PageRanges != nil {
		pdfOpts.PageRanges = fmt.Sprintf("%d-%d", config.PageRanges.Start, config.PageRanges.End)
	}

	// Set custom HTML for header and footer
	if config.HeaderHTML != nil {
		pdfOpts.HeaderTemplate = *config.HeaderHTML
	}
	if config.FooterHTML != nil {
		pdfOpts.FooterTemplate = *config.FooterHTML
	}

	return pdfOpts
}

// mergePDFs combines multiple PDF readers into a single PDF document.
// It works by writing each reader to a temporary file, then using the pdfcpu library
// to merge them into a single output file, which is then returned as a reader.
// This function handles concurrent writing of the input PDFs to optimize performance.
func (p *PDFGeneratorRod) mergePDFs(readers []io.Reader) (io.Reader, error) {
	// Create array to store temporary file paths
	tempFilesNames := make([]string, len(readers))

	// Create a temporary directory to store individual PDFs
	tempDir, err := os.MkdirTemp("", "pdf_merge")
	if err != nil {
		return nil, fmt.Errorf("error creating temporary directory: %w", err)
	}
	// Ensure cleanup of temporary files when function exits
	defer os.RemoveAll(tempDir)

	// Set up concurrency controls
	var wg sync.WaitGroup
	var mu sync.Mutex
	var processingErr error

	// Process each PDF reader concurrently
	for idx, reader := range readers {
		wg.Add(1)
		go func(i int, r io.Reader) {
			defer wg.Done()

			// Generate a unique ID for this temporary file
			id := sharedInfrastructure.GenerateXID()
			path := filepath.Join(tempDir, fmt.Sprintf("temp_%s.pdf", id))

			// Create and write to the temporary file
			f, err := os.Create(path)
			if err != nil {
				mu.Lock()
				if processingErr == nil {
					processingErr = err
				}
				mu.Unlock()
				return
			}
			defer f.Close()

			// Copy PDF content to the temporary file
			if _, err = io.Copy(f, r); err != nil {
				mu.Lock()
				if processingErr == nil {
					processingErr = err
				}
				mu.Unlock()
				return
			}

			// Store the temporary file path in our array
			mu.Lock()
			tempFilesNames[i] = path
			mu.Unlock()
		}(idx, reader)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Check if any errors occurred during processing
	if processingErr != nil {
		return nil, processingErr
	}

	// Create output file path with a unique name
	outputPath := filepath.Join(tempDir, fmt.Sprintf("merged_%s.pdf", sharedInfrastructure.GenerateXID()))

	// Merge all PDFs into a single file using pdfcpu library
	if err := pdfProcessingAPI.MergeCreateFile(tempFilesNames, outputPath, false, pdfProcessingModel.NewDefaultConfiguration()); err != nil {
		return nil, err
	}

	// Read the merged PDF file
	merged, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, err
	}

	// Return the merged PDF as a reader
	return bytes.NewReader(merged), nil
}

// GeneratePDF is the main method for generating PDFs from HTML content.
// It processes each PDF item concurrently using the browser pool, then merges
// all generated PDFs into a single document which is returned as an io.Reader.
// This method handles initializing the generator if needed and coordinates
// the parallel generation of multiple PDF items.
func (p *PDFGeneratorRod) GeneratePDF(request *dto.PDFGenerationDTO) (io.Reader, error) {
	// Ensure generator is initialized
	if !p.initialized {
		p.Initialize()
	}

	// Prepare storage for individual PDF readers
	readers := make([]io.Reader, len(request.Items))

	// Set up concurrency controls
	var wg sync.WaitGroup
	var mu sync.Mutex
	var processingErr error

	// Process each PDF item concurrently
	for idx, item := range request.Items {
		wg.Add(1)
		go func(i int, pdfItem dto.PDFItem) {
			defer wg.Done()

			// Get a page from the pool
			pwb := p.RequestPage()
			// Ensure page is returned to pool after use
			defer p.ReturnPage(pwb)

			// Build PDF options based on item configuration
			opts := p.buildPDFOptions(pdfItem.Config)

			// Set the HTML content to the page
			err := pwb.Page.SetDocumentContent(pdfItem.BodyHTML)
			if err != nil {
				mu.Lock()
				if processingErr == nil {
					processingErr = err
				}
				mu.Unlock()
				return
			}

			// Wait for page to fully load and become idle
			pwb.Page.MustWaitLoad().MustWaitIdle()

			// Wait for all images to load
			pwb.Page.MustEval(`() => {
				return Promise.all(
					Array.from(document.images).map(img => {
						if (img.complete) return Promise.resolve();
						return new Promise(resolve => img.onload = img.onerror = resolve);
					})
				);
			}`)

			// Generate the PDF from the page
			pdf, err := pwb.Page.PDF(opts)
			if err != nil {
				mu.Lock()
				if processingErr == nil {
					processingErr = err
				}
				mu.Unlock()
				return
			}

			// Store the generated PDF reader
			mu.Lock()
			readers[i] = pdf
			mu.Unlock()
		}(idx, item)
	}

	// Wait for all PDF generation to complete
	wg.Wait()

	// Check if any errors occurred during generation
	if processingErr != nil {
		return nil, processingErr
	}

	// Merge all generated PDFs into a single document
	return p.mergePDFs(readers)
}
