package helpers

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/require"
)

// smokeHTMLTemplate is the Phase 0 inline fixture: a single line carrying the
// per-run {{.Marker}} so later phases can assert marker propagation. Rich
// fixtures (invoice/cover/terms) arrive in Phase 4 under testdata/*.html.tmpl.
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
	t.Logf("PDF guardado en: %s", path)

	return SavePDFResult{Path: path}
}
