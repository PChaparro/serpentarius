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
	"time"

	"slices"

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

	// PageIdleTimeout defines how long a page can remain idle before being closed
	PageIdleTimeout = time.Duration(sharedInfrastructure.GetEnvironment().MaxChromiumTabIdleSeconds) * time.Second
)

// PageWithBrowser associates a Rod Page with its parent Browser instance.
// This structure is used in the page pool to track which page belongs to which browser.
type PageWithBrowser struct {
	Page    *rod.Page    // The browser page instance for rendering content
	Browser *rod.Browser // The parent browser instance that owns this page
}

// PageWithTimeout extends PageWithBrowser to include timeout management.
// This structure is used to track pages and their activity for dynamic resource management.
type PageWithTimeout struct {
	PageWithBrowser
	LastUsed  time.Time   // Timestamp when the page was last returned to the pool
	Timer     *time.Timer // Timer to track inactivity and trigger cleanup
	InUse     bool        // Whether this page is currently being used
	BrowserID string      // Unique identifier for the browser
}

// BrowserInfo tracks information about a browser instance
type BrowserInfo struct {
	Browser   *rod.Browser       // The browser instance
	PageCount int                // Current number of pages (tabs) in this browser
	Pages     []*PageWithTimeout // References to pages created with this browser
	ID        string             // Unique identifier for this browser
}

// PDFGeneratorRod implements PDF generation functionality using the Rod library
// to control headless Chrome browsers. It dynamically manages browser and page resources,
// creating them on-demand and cleaning them up after periods of inactivity.
type PDFGeneratorRod struct {
	mutex          sync.Mutex              // Mutex to protect concurrent access to the generator state
	browsers       map[string]*BrowserInfo // Map of browser instances by their unique IDs
	availablePages []*PageWithTimeout      // List of available pages
	waitingQueue   []chan *PageWithTimeout // Channels for clients waiting for a page
	pageWaitGroup  sync.WaitGroup          // Used to track when pages are being used
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
		pdfGeneratorInstance = &PDFGeneratorRod{
			browsers:       make(map[string]*BrowserInfo),
			availablePages: make([]*PageWithTimeout, 0),
			waitingQueue:   make([]chan *PageWithTimeout, 0),
		}

		// Set up a finalizer to clean up resources when the generator is garbage collected
		runtime.SetFinalizer(pdfGeneratorInstance, func(p *PDFGeneratorRod) {
			p.ReleaseBrowserPool()
		})
	})

	return pdfGeneratorInstance
}

// createBrowser launches a new browser instance and adds it to the pool
func (p *PDFGeneratorRod) createBrowser() (*BrowserInfo, error) {
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

	// Generate a unique ID for this browser
	browserID := sharedInfrastructure.GenerateXID()

	// Create browser info
	info := &BrowserInfo{
		Browser:   browser,
		PageCount: 0,
		Pages:     make([]*PageWithTimeout, 0),
		ID:        browserID,
	}

	// Store in browsers map
	p.browsers[browserID] = info

	sharedInfrastructure.GetLogger().
		WithField("browser_id", browserID).
		Info("Created new browser instance")

	return info, nil
}

// createPage creates a new page in the given browser
func (p *PDFGeneratorRod) createPage(browserInfo *BrowserInfo) (*PageWithTimeout, error) {
	// Create a new incognito page
	page := browserInfo.Browser.MustIncognito().MustPage()

	// Create PageWithTimeout
	pwt := &PageWithTimeout{
		PageWithBrowser: PageWithBrowser{
			Page:    page,
			Browser: browserInfo.Browser,
		},
		LastUsed:  time.Now(),
		InUse:     false,
		BrowserID: browserInfo.ID,
	}

	// Add to browser's pages
	browserInfo.Pages = append(browserInfo.Pages, pwt)
	browserInfo.PageCount++

	sharedInfrastructure.GetLogger().
		WithField("browser_id", browserInfo.ID).
		Info("Created new page")

	return pwt, nil
}

// findOrCreateAvailablePage finds an available page or creates a new one if needed
func (p *PDFGeneratorRod) findOrCreateAvailablePage() (*PageWithTimeout, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Check if there are available pages already
	if len(p.availablePages) > 0 {
		// Take the first available page
		page := p.availablePages[0]
		p.availablePages = p.availablePages[1:]

		// Stop the timer if it's running
		if page.Timer != nil {
			page.Timer.Stop()
		}

		page.InUse = true
		return page, nil
	}

	// Need to create a new page

	// First, see if we have any browser with capacity for a new page
	for _, browserInfo := range p.browsers {
		if browserInfo.PageCount < MaxPagesPerBrowser {
			// This browser can take another page
			page, err := p.createPage(browserInfo)
			if err != nil {
				return nil, err
			}

			page.InUse = true
			return page, nil
		}
	}

	// No browser with capacity, need to create a new browser if allowed
	if len(p.browsers) < MaxBrowsers {
		browserInfo, err := p.createBrowser()
		if err != nil {
			return nil, err
		}

		// Create a page in this new browser
		page, err := p.createPage(browserInfo)
		if err != nil {
			return nil, err
		}

		page.InUse = true
		return page, nil
	}

	// All browsers are at capacity and we can't create more
	// Create a channel to receive a page when one becomes available
	pageChannel := make(chan *PageWithTimeout, 1)
	p.waitingQueue = append(p.waitingQueue, pageChannel)

	// Release the lock while waiting
	p.mutex.Unlock()
	page := <-pageChannel
	p.mutex.Lock()

	return page, nil
}

// startPageTimer starts a timer to close the page after inactivity
func (p *PDFGeneratorRod) startPageTimer(page *PageWithTimeout) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Stop existing timer if any
	if page.Timer != nil {
		page.Timer.Stop()
	}

	// Start a new timer
	page.Timer = time.AfterFunc(PageIdleTimeout, func() {
		p.closeIdlePage(page)
	})
}

// closeIdlePage closes a page that has been idle for too long
func (p *PDFGeneratorRod) closeIdlePage(page *PageWithTimeout) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Make sure the page is still in our pool and not in use
	if page.InUse {
		return
	}

	// Find the page in the available list
	foundIdx := -1
	for i, p := range p.availablePages {
		if p == page {
			foundIdx = i
			break
		}
	}

	if foundIdx != -1 {
		// Remove from available pages
		p.availablePages = slices.Delete(p.availablePages, foundIdx, foundIdx+1)

		// Close the page
		page.Page.MustClose()

		// Update browser info
		browserInfo := p.browsers[page.BrowserID]
		browserInfo.PageCount--

		// Remove from browser's pages list
		for i, p := range browserInfo.Pages {
			if p == page {
				browserInfo.Pages = slices.Delete(browserInfo.Pages, i, i+1)
				break
			}
		}

		// If this was the last page, close the browser too
		if browserInfo.PageCount == 0 {
			browserInfo.Browser.MustClose()
			delete(p.browsers, page.BrowserID)
			sharedInfrastructure.GetLogger().
				WithField("browser_id", page.BrowserID).
				Info("Closed idle browser instance")
		}

		sharedInfrastructure.GetLogger().
			WithField("browser_id", page.BrowserID).
			Info("Closed idle page")
	}
}

// RequestPage retrieves an available page or creates a new one.
// This method will block if all allowed resources are in use until a page becomes available.
// The caller is responsible for returning the page to the pool after use.
func (p *PDFGeneratorRod) RequestPage() *PageWithBrowser {
	p.pageWaitGroup.Add(1)

	// Get a page (available or new)
	page, err := p.findOrCreateAvailablePage()
	if err != nil {
		sharedInfrastructure.GetLogger().WithError(err).Error("Failed to get page")
		p.pageWaitGroup.Done()
		return nil
	}

	// Convert to the interface expected by existing code
	return &PageWithBrowser{
		Page:    page.Page,
		Browser: page.Browser,
	}
}

// ReturnPage returns a page to the pool and starts its inactivity timer.
// This should be called after a page is no longer needed to prevent resource leaks.
func (p *PDFGeneratorRod) ReturnPage(pwb *PageWithBrowser) {
	defer p.pageWaitGroup.Done()
	p.mutex.Lock()

	// Find the corresponding PageWithTimeout
	var page *PageWithTimeout
	for _, browserInfo := range p.browsers {
		for _, p := range browserInfo.Pages {
			if p.Page == pwb.Page {
				page = p
				break
			}
		}
		if page != nil {
			break
		}
	}

	// If the page is not found, just close it
	if page == nil {
		pwb.Page.MustClose()
		p.mutex.Unlock()
		return
	}

	// Mark as not in use and update timestamp
	page.InUse = false
	page.LastUsed = time.Now()

	// Check if anyone is waiting for a page
	if len(p.waitingQueue) > 0 {
		// Give the page directly to the first waiter
		ch := p.waitingQueue[0]
		p.waitingQueue = p.waitingQueue[1:]

		page.InUse = true
		p.mutex.Unlock()

		ch <- page
	} else {
		// No one waiting, add to available pages
		p.availablePages = append(p.availablePages, page)
		p.mutex.Unlock()

		// Start the inactivity timer
		p.startPageTimer(page)
	}
}

// ReleaseBrowserPool cleans up all browser resources managed by this generator.
// It closes all browser instances and releases associated resources.
// This method is thread-safe and idempotent, so it's safe to call multiple times.
func (p *PDFGeneratorRod) ReleaseBrowserPool() {
	// Wait for all pages to finish processing before cleaning up
	p.pageWaitGroup.Wait()

	// Lock to ensure no concurrent access while cleaning up
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Close all browsers
	for id, browserInfo := range p.browsers {
		// Stop all timers
		for _, page := range browserInfo.Pages {
			if page.Timer != nil {
				page.Timer.Stop()
			}
			// Close each page
			page.Page.MustClose()
		}

		// Close the browser
		browserInfo.Browser.MustClose()
		delete(p.browsers, id)
	}

	// Clear pages
	p.availablePages = make([]*PageWithTimeout, 0)

	// Clear waiting queue and signal closure
	for _, ch := range p.waitingQueue {
		close(ch)
	}
	p.waitingQueue = make([]chan *PageWithTimeout, 0)

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
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

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

			// Ensure the file is closed after writing
			defer func() {
				_ = f.Close()
			}()

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
