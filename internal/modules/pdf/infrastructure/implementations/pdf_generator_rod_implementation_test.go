package implementations

import (
	"testing"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"
	"github.com/PChaparro/serpentarius/internal/testutils"
	"github.com/go-rod/rod/lib/proto"
	"github.com/stretchr/testify/assert"
)

// pdfOptionsCase is one row of the buildPDFOptions matrix.
// Expected options are precomputed literals, never derived from prod logic.
// The mutant is documented in a `// Mutant:` comment on each row.
type pdfOptionsCase struct {
	name   string
	config *dto.ItemConfig
	want   *proto.PagePrintToPDF
}

func TestBuildPDFOptions(t *testing.T) {
	gen := &PDFGeneratorRod{}

	cases := []pdfOptionsCase{
		{
			// Mutant: if the `if config == nil` early return is removed, this case panics (nil-deref).
			name:   "nil config returns empty options",
			config: nil,
			want:   &proto.PagePrintToPDF{},
		},
		{
			// Mutant: if `pdfOpts.Landscape = *config.Orientation == "landscape"` is removed, this case fails.
			name:   "landscape orientation enables landscape",
			config: &dto.ItemConfig{Orientation: testutils.StrPtr("landscape")},
			want:   &proto.PagePrintToPDF{Landscape: true},
		},
		{
			// Mutant: if the comparison were `!= "portrait"` instead of `== "landscape"`, this case fails.
			name:   "portrait orientation leaves landscape false",
			config: &dto.ItemConfig{Orientation: testutils.StrPtr("portrait")},
			want:   &proto.PagePrintToPDF{Landscape: false},
		},
		{
			// Mutant: if the DisplayHeaderFooter copy is removed, this case fails.
			name:   "displayHeaderFooter propagates",
			config: &dto.ItemConfig{DisplayHeaderFooter: testutils.BoolPtr(true)},
			want:   &proto.PagePrintToPDF{DisplayHeaderFooter: true},
		},
		{
			// Mutant: if `pdfOpts.PrintBackground = *config.PrintBackground` is removed, this case fails.
			name:   "printBackground propagates",
			config: &dto.ItemConfig{PrintBackground: testutils.BoolPtr(true)},
			want:   &proto.PagePrintToPDF{PrintBackground: true},
		},
		{
			// Mutant: if `pdfOpts.Scale = config.Scale` is removed, this case fails.
			name:   "scale propagates",
			config: &dto.ItemConfig{Scale: testutils.FloatPtr(1.5)},
			want:   &proto.PagePrintToPDF{Scale: testutils.FloatPtr(1.5)},
		},
		{
			// Mutant: if the PaperWidth/PaperHeight copy is removed, this case fails.
			name:   "full size maps width and height",
			config: &dto.ItemConfig{Size: &dto.PageSize{Width: testutils.FloatPtr(8.27), Height: testutils.FloatPtr(11.7)}},
			want:   &proto.PagePrintToPDF{PaperWidth: testutils.FloatPtr(8.27), PaperHeight: testutils.FloatPtr(11.7)},
		},
		{
			// Mutant: if the `if config.Size.Width/Height != nil` guards are removed, this case panics or overfills.
			name:   "partial size width only",
			config: &dto.ItemConfig{Size: &dto.PageSize{Width: testutils.FloatPtr(8.27)}},
			want:   &proto.PagePrintToPDF{PaperWidth: testutils.FloatPtr(8.27), PaperHeight: nil},
		},
		{
			// Mutant: if any margin copy is removed, this case fails.
			name: "full margins propagate",
			config: &dto.ItemConfig{Margin: &dto.PageMargin{
				TopInches:    testutils.FloatPtr(0.5),
				BottomInches: testutils.FloatPtr(0.6),
				LeftInches:   testutils.FloatPtr(0.7),
				RightInches:  testutils.FloatPtr(0.8),
			}},
			want: &proto.PagePrintToPDF{
				MarginTop:    testutils.FloatPtr(0.5),
				MarginBottom: testutils.FloatPtr(0.6),
				MarginLeft:   testutils.FloatPtr(0.7),
				MarginRight:  testutils.FloatPtr(0.8),
			},
		},
		{
			// Mutant: if the per-margin guards are removed, a partial Margin panics.
			name: "partial margin top only",
			config: &dto.ItemConfig{Margin: &dto.PageMargin{
				TopInches: testutils.FloatPtr(0.5),
			}},
			want: &proto.PagePrintToPDF{MarginTop: testutils.FloatPtr(0.5)},
		},
		{
			// Mutant: if the `"%d-%d"` format changes, this case fails (precomputed value "1-3").
			name:   "pageRange formats start-end",
			config: &dto.ItemConfig{PageRange: &dto.PageRange{Start: 1, End: 3}},
			want:   &proto.PagePrintToPDF{PageRanges: "1-3"},
		},
		{
			// Mutant: if the HeaderTemplate/FooterTemplate copy is removed, this case fails.
			name:   "header and footer propagate",
			config: &dto.ItemConfig{HeaderHTML: testutils.StrPtr("<h>hdr</h>"), FooterHTML: testutils.StrPtr("<f>ftr</f>")},
			want:   &proto.PagePrintToPDF{HeaderTemplate: "<h>hdr</h>", FooterTemplate: "<f>ftr</f>"},
		},
		{
			// Mutant: if any field stops being copied, this combined case fails.
			name: "full config combines everything",
			config: &dto.ItemConfig{
				Orientation:         testutils.StrPtr("landscape"),
				DisplayHeaderFooter: testutils.BoolPtr(true),
				PrintBackground:     testutils.BoolPtr(true),
				Scale:               testutils.FloatPtr(0.8),
				Size:                &dto.PageSize{Width: testutils.FloatPtr(11.7), Height: testutils.FloatPtr(16.5)},
				Margin: &dto.PageMargin{
					TopInches:    testutils.FloatPtr(1),
					BottomInches: testutils.FloatPtr(1),
					LeftInches:   testutils.FloatPtr(1),
					RightInches:  testutils.FloatPtr(1),
				},
				PageRange:  &dto.PageRange{Start: 2, End: 4},
				HeaderHTML: testutils.StrPtr("<h>h</h>"),
				FooterHTML: testutils.StrPtr("<f>f</f>"),
			},
			want: &proto.PagePrintToPDF{
				Landscape:           true,
				DisplayHeaderFooter: true,
				PrintBackground:     true,
				Scale:               testutils.FloatPtr(0.8),
				PaperWidth:          testutils.FloatPtr(11.7),
				PaperHeight:         testutils.FloatPtr(16.5),
				MarginTop:           testutils.FloatPtr(1),
				MarginBottom:        testutils.FloatPtr(1),
				MarginLeft:          testutils.FloatPtr(1),
				MarginRight:         testutils.FloatPtr(1),
				PageRanges:          "2-4",
				HeaderTemplate:      "<h>h</h>",
				FooterTemplate:      "<f>f</f>",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, gen.buildPDFOptions(tc.config))
		})
	}
}
