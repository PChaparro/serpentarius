package helpers

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"time"

	pdfText "github.com/ledongthuc/pdf"
	pdfcpuAPI "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpuModel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/stretchr/testify/require"
)

// smokeHTMLTemplate is the inline fixture: a single line carrying the
// per-run {{.Marker}} so suites can assert marker propagation. Rich
// fixtures (invoice/cover/terms) live under testdata/*.html.tmpl.
const smokeHTMLTemplate = `<html><body><h1>Hello-{{.Marker}}</h1></body></html>`

// TemplateArgs groups the parameters for rendering an HTML template.
type TemplateArgs struct {
	Marker string // per-run uuid injected as {{.Marker}}
}

// TemplateResult carries the rendered HTML.
type TemplateResult struct {
	HTML string
}

// RenderTemplate renders the inline smoke template with the given marker.
func RenderTemplate(t *testing.T, args TemplateArgs) TemplateResult {
	t.Helper()
	require.NotEmpty(t, args.Marker, "marker must not be empty")

	tmpl, err := template.New("smoke").Parse(smokeHTMLTemplate)
	require.NoError(t, err, "parsing smoke HTML template")

	var rendered strings.Builder
	err = tmpl.Execute(&rendered, map[string]string{"Marker": args.Marker})
	require.NoError(t, err, "rendering smoke HTML template")

	html := rendered.String()
	require.Contains(t, html, args.Marker, "rendered HTML must contain the marker")

	return TemplateResult{HTML: html}
}

// SavePDFArgs groups the parameters for saving a PDF to disk.
type SavePDFArgs struct {
	Suite string // suite name, first segment of the file name
	Test  string // test name, second segment of the file name
	Bytes []byte // raw PDF bytes to persist
}

// SavePDFResult carries the path of the saved PDF.
type SavePDFResult struct {
	Path string
}

// SavePDF persists PDF bytes under testdata/output/<suite>-<test>-<unixnano>.pdf
// for human visual review and logs the path. The file content never decides
// the test outcome; semantic asserts do.
func SavePDF(t *testing.T, args SavePDFArgs) SavePDFResult {
	t.Helper()
	require.NotEmpty(t, args.Suite, "suite must not be empty")
	require.NotEmpty(t, args.Test, "test must not be empty")
	require.NotEmpty(t, args.Bytes, "PDF bytes must not be empty")

	outputDir := filepath.Join("testdata", "output")
	require.NoError(t, os.MkdirAll(outputDir, 0o755), "creating PDF output dir")

	fileName := args.Suite + "-" + args.Test + "-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10) + ".pdf"
	path := filepath.Join(outputDir, fileName)

	require.NoError(t, os.WriteFile(path, args.Bytes, 0o644), "writing PDF file")
	t.Logf("PDF saved at: %s", path)

	return SavePDFResult{Path: path}
}

// InspectPDFArgs groups the parameters for semantic PDF inspection.
type InspectPDFArgs struct {
	Bytes []byte // raw PDF under inspection
	// Pages is the expected page count; zero skips the page-count check.
	Pages int
	// Markers must each appear in the extracted text; nil skips the text check.
	Markers []string
}

// InspectPDFResult carries the text extracted from the PDF for extra asserts.
type InspectPDFResult struct {
	Text string
}

// InspectPDF asserts the observable PDF contract: magic bytes, parseability
// and page count via pdfcpu, plus marker presence in the extracted text via
// a pure-Go text extractor. It never compares golden bytes: Chromium embeds
// fresh timestamps and document IDs on every render.
func InspectPDF(t *testing.T, args InspectPDFArgs) InspectPDFResult {
	t.Helper()
	require.NotEmpty(t, args.Bytes, "PDF bytes must not be empty")
	require.True(t, bytes.HasPrefix(args.Bytes, []byte("%PDF-")),
		"PDF must start with the magic bytes %PDF-")

	pageCount, err := pdfcpuAPI.PageCount(
		bytes.NewReader(args.Bytes),
		pdfcpuModel.NewDefaultConfiguration(),
	)
	require.NoError(t, err, "PDF must be parseable by pdfcpu")

	if args.Pages > 0 {
		require.Equal(t, args.Pages, pageCount,
			"PDF page count must match the number of rendered items")
	}

	var text string
	if len(args.Markers) > 0 {
		reader, err := pdfText.NewReader(bytes.NewReader(args.Bytes), int64(len(args.Bytes)))
		require.NoError(t, err, "text extractor must open the PDF")

		// Fonts are resolved per page on purpose: merged documents can reuse
		// a font name for distinct font objects, so a cache shared across
		// pages decodes later pages with the first page encoding.
		var texts strings.Builder
		for i := 1; i <= reader.NumPage(); i++ {
			page := reader.Page(i)

			fonts := make(map[string]*pdfText.Font, len(page.Fonts()))
			for _, name := range page.Fonts() {
				font := page.Font(name)
				fonts[name] = &font
			}

			pageText, err := page.GetPlainText(fonts)
			require.NoError(t, err, "text extraction must succeed")
			texts.WriteString(pageText)
		}
		text = texts.String()

		for _, marker := range args.Markers {
			require.NotEmpty(t, marker, "markers must not be empty")
			require.Contains(t, text, marker,
				"extracted text must contain the marker")
		}
	}

	return InspectPDFResult{Text: text}
}
