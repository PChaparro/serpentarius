package requests

import (
	"testing"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"
	"github.com/PChaparro/serpentarius/internal/testutils"
	"github.com/stretchr/testify/assert"
)

// pageSizeCase is one row of the getPageSizeFromString matrix.
// Expected dimensions are precomputed literals, never recomputed with prod logic.
// The mutant is documented in a `// Mutant:` comment on each row.
type pageSizeCase struct {
	name  string
	size  string
	wantW float64
	wantH float64
}

func TestGetPageSizeFromString(t *testing.T) {
	cases := []pageSizeCase{
		// Mutant: if the `case "letter"` is removed, this case falls back to the default and stops matching if the default changed; covered by the shared default.
		{name: "letter", size: "letter", wantW: 8.5, wantH: 11.0},
		// Mutant: if the `case "legal"` (or its 14.0 height) is removed, this case fails.
		{name: "legal", size: "legal", wantW: 8.5, wantH: 14.0},
		// Mutant: if the `case "tabloid"` is removed, this case fails.
		{name: "tabloid", size: "tabloid", wantW: 11.0, wantH: 17.0},
		// Mutant: if the `case "ledger"` is removed, this case fails.
		{name: "ledger", size: "ledger", wantW: 17.0, wantH: 11.0},
		// Mutant: if the `case "a0"` is removed, this case fails.
		{name: "a0", size: "a0", wantW: 33.1, wantH: 46.8},
		// Mutant: if the `case "a1"` is removed, this case fails.
		{name: "a1", size: "a1", wantW: 23.4, wantH: 33.1},
		// Mutant: if the `case "a2"` is removed, this case fails.
		{name: "a2", size: "a2", wantW: 16.5, wantH: 23.4},
		// Mutant: if the `case "a3"` is removed, this case fails.
		{name: "a3", size: "a3", wantW: 11.7, wantH: 16.5},
		// Mutant: if the `case "a4"` is removed, this case fails.
		{name: "a4", size: "a4", wantW: 8.27, wantH: 11.7},
		// Mutant: if the `case "a5"` is removed, this case fails.
		{name: "a5", size: "a5", wantW: 5.875, wantH: 8.25},
		// Mutant: if the `case "a6"` is removed, this case fails.
		{name: "a6", size: "a6", wantW: 4.125, wantH: 5.875},
		// Mutant: if the default changes (width 8.5 / height 11.0), these cases fail.
		{name: "unknown falls back to default", size: "folio", wantW: 8.5, wantH: 11.0},
		// Mutant: if the default changes (width 8.5 / height 11.0), this case fails.
		{name: "empty falls back to default", size: "", wantW: 8.5, wantH: 11.0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := getPageSizeFromString(tc.size)
			if assert.NotNil(t, got, "should always return non-nil PageSize") {
				if assert.NotNil(t, got.Width) {
					assert.InDelta(t, tc.wantW, *got.Width, 1e-9, "width of %q", tc.size)
				}
				if assert.NotNil(t, got.Height) {
					assert.InDelta(t, tc.wantH, *got.Height, 1e-9, "height of %q", tc.size)
				}
			}
		})
	}
}

// toDTOCase is one row of the ToDTO matrix: input request plus the
// fully precomputed expected DTO (literals, never prod logic).
type toDTOCase struct {
	name string
	req  GeneratePDFRequest
	want *dto.PDFGenerationDTO
}

func TestToDTO(t *testing.T) {
	cases := []toDTOCase{
		{
			// Mutant: if `var cacheTTL int64` (default 0) or the `if r.Cache != nil` guard is removed, this case fails.
			name: "without cache defaults TTL 0 and empty namespace",
			req: GeneratePDFRequest{
				Items:  []PDFItem{{BodyHTML: "<p>hi</p>"}},
				Output: OutputConfig{Bucket: "b", Key: "k"},
				Cache:  nil,
			},
			want: &dto.PDFGenerationDTO{
				Items:  []dto.PDFItem{{BodyHTML: "<p>hi</p>", Config: nil}},
				Output: dto.OutputConfig{Bucket: "b", Key: "k"},
				Cache:  dto.CacheConfig{TTL: 0, Namespace: ""},
			},
		},
		{
			// Mutant: if `cacheTTL = *r.Cache.TTL` or `strings.TrimSpace` is removed, this case fails.
			name: "explicit TTL and trimmed spaced namespace",
			req: GeneratePDFRequest{
				Items:  []PDFItem{{BodyHTML: "<p>hi</p>"}},
				Output: OutputConfig{Bucket: "b", Key: "k"},
				Cache:  &CacheConfig{TTL: testutils.Int64Ptr(3600), Namespace: "  tenant "},
			},
			want: &dto.PDFGenerationDTO{
				Items:  []dto.PDFItem{{BodyHTML: "<p>hi</p>", Config: nil}},
				Output: dto.OutputConfig{Bucket: "b", Key: "k"},
				Cache:  dto.CacheConfig{TTL: 3600, Namespace: "tenant"},
			},
		},
		{
			// Mutant: if the `if r.Cache.TTL != nil` guard is removed (nil-deref or garbage TTL), this case fails.
			name: "cache without TTL defaults to 0 but keeps namespace",
			req: GeneratePDFRequest{
				Items:  []PDFItem{{BodyHTML: "<p>hi</p>"}},
				Output: OutputConfig{Bucket: "b", Key: "k"},
				Cache:  &CacheConfig{Namespace: "ns"},
			},
			want: &dto.PDFGenerationDTO{
				Items:  []dto.PDFItem{{BodyHTML: "<p>hi</p>", Config: nil}},
				Output: dto.OutputConfig{Bucket: "b", Key: "k"},
				Cache:  dto.CacheConfig{TTL: 0, Namespace: "ns"},
			},
		},
		{
			// Mutant: if the `buildItemConfig` nil-check (`if config == nil`) is removed, this case panics/fails.
			name: "item without config stays nil",
			req: GeneratePDFRequest{
				Items:  []PDFItem{{BodyHTML: "<p>hi</p>", Config: nil}},
				Output: OutputConfig{Bucket: "b", Key: "k"},
			},
			want: &dto.PDFGenerationDTO{
				Items:  []dto.PDFItem{{BodyHTML: "<p>hi</p>", Config: nil}},
				Output: dto.OutputConfig{Bucket: "b", Key: "k"},
				Cache:  dto.CacheConfig{TTL: 0, Namespace: ""},
			},
		},
		{
			// Mutant: if `itemConfig.Size = getPageSizeFromString(*config.Size)` is removed, Size stays nil and this case fails.
			name: "maps size to precomputed dimensions",
			req: GeneratePDFRequest{
				Items: []PDFItem{{
					BodyHTML: "<p>hi</p>",
					Config:   &ItemConfig{Size: testutils.StrPtr("a4")},
				}},
				Output: OutputConfig{Bucket: "b", Key: "k"},
			},
			want: &dto.PDFGenerationDTO{
				Items: []dto.PDFItem{{
					BodyHTML: "<p>hi</p>",
					Config: &dto.ItemConfig{
						Size: &dto.PageSize{Width: testutils.FloatPtr(8.27), Height: testutils.FloatPtr(11.7)},
					},
				}},
				Output: dto.OutputConfig{Bucket: "b", Key: "k"},
				Cache:  dto.CacheConfig{TTL: 0, Namespace: ""},
			},
		},
		{
			// Mutant: if the Start/End copy of PageRange is removed, this case fails.
			name: "maps pageRange",
			req: GeneratePDFRequest{
				Items: []PDFItem{{
					BodyHTML: "<p>hi</p>",
					Config:   &ItemConfig{PageRange: &PageRange{Start: 2, End: 5}},
				}},
				Output: OutputConfig{Bucket: "b", Key: "k"},
			},
			want: &dto.PDFGenerationDTO{
				Items: []dto.PDFItem{{
					BodyHTML: "<p>hi</p>",
					Config:   &dto.ItemConfig{PageRange: &dto.PageRange{Start: 2, End: 5}},
				}},
				Output: dto.OutputConfig{Bucket: "b", Key: "k"},
				Cache:  dto.CacheConfig{TTL: 0, Namespace: ""},
			},
		},
		{
			// Mutant: if the `if config.Size/Margin/PageRange != nil` guards are removed, this case panics (nil-deref).
			name: "partial config leaves missing fields untouched",
			req: GeneratePDFRequest{
				Items: []PDFItem{{
					BodyHTML: "<p>hi</p>",
					Config:   &ItemConfig{Orientation: testutils.StrPtr("landscape")},
				}},
				Output: OutputConfig{Bucket: "b", Key: "k"},
			},
			want: &dto.PDFGenerationDTO{
				Items: []dto.PDFItem{{
					BodyHTML: "<p>hi</p>",
					Config: &dto.ItemConfig{
						Orientation: testutils.StrPtr("landscape"),
						Size:        nil,
						Margin:      nil,
						PageRange:   nil,
					},
				}},
				Output: dto.OutputConfig{Bucket: "b", Key: "k"},
				Cache:  dto.CacheConfig{TTL: 0, Namespace: ""},
			},
		},
		{
			// Mutant: if the Margin/Scale/flags/Header/Footer copy is removed, this case fails.
			name: "maps margins flags and header footer",
			req: GeneratePDFRequest{
				Items: []PDFItem{{
					BodyHTML: "<p>hi</p>",
					Config: &ItemConfig{
						DisplayHeaderFooter: testutils.BoolPtr(true),
						PrintBackground:     testutils.BoolPtr(true),
						Scale:               testutils.FloatPtr(1.5),
						Margin: &PageMargin{
							TopInches:    testutils.FloatPtr(0.5),
							BottomInches: testutils.FloatPtr(0.6),
							LeftInches:   testutils.FloatPtr(0.7),
							RightInches:  testutils.FloatPtr(0.8),
						},
						HeaderHTML: testutils.StrPtr("<h>hdr</h>"),
						FooterHTML: testutils.StrPtr("<f>ftr</f>"),
					},
				}},
				Output: OutputConfig{Bucket: "b", Key: "k"},
			},
			want: &dto.PDFGenerationDTO{
				Items: []dto.PDFItem{{
					BodyHTML: "<p>hi</p>",
					Config: &dto.ItemConfig{
						DisplayHeaderFooter: testutils.BoolPtr(true),
						PrintBackground:     testutils.BoolPtr(true),
						Scale:               testutils.FloatPtr(1.5),
						Margin: &dto.PageMargin{
							TopInches:    testutils.FloatPtr(0.5),
							BottomInches: testutils.FloatPtr(0.6),
							LeftInches:   testutils.FloatPtr(0.7),
							RightInches:  testutils.FloatPtr(0.8),
						},
						HeaderHTML: testutils.StrPtr("<h>hdr</h>"),
						FooterHTML: testutils.StrPtr("<f>ftr</f>"),
					},
				}},
				Output: dto.OutputConfig{Bucket: "b", Key: "k"},
				Cache:  dto.CacheConfig{TTL: 0, Namespace: ""},
			},
		},
		{
			// Mutant: if the `for i, item := range r.Items` loop skipped items or did not preserve order, this case fails.
			name: "multiple items preserve order and body",
			req: GeneratePDFRequest{
				Items: []PDFItem{
					{BodyHTML: "<p>one</p>"},
					{BodyHTML: "<p>two</p>", Config: &ItemConfig{Orientation: testutils.StrPtr("portrait")}},
				},
				Output: OutputConfig{Bucket: "b", Key: "k"},
			},
			want: &dto.PDFGenerationDTO{
				Items: []dto.PDFItem{
					{BodyHTML: "<p>one</p>", Config: nil},
					{BodyHTML: "<p>two</p>", Config: &dto.ItemConfig{Orientation: testutils.StrPtr("portrait")}},
				},
				Output: dto.OutputConfig{Bucket: "b", Key: "k"},
				Cache:  dto.CacheConfig{TTL: 0, Namespace: ""},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.req.ToDTO())
		})
	}
}
