package requests

import (
	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"
	"github.com/ysmood/gson"
)

// PageMargin represents the margins of the PDF page in inches
type PageMargin struct {
	TopInches    *float64 `json:"topInches,omitempty" validate:"omitempty,min=0"`
	BottomInches *float64 `json:"bottomInches,omitempty" validate:"omitempty,min=0"`
	LeftInches   *float64 `json:"leftInches,omitempty" validate:"omitempty,min=0"`
	RightInches  *float64 `json:"rightInches,omitempty" validate:"omitempty,min=0"`
}

// PageRange represents the range of pages to print
type PageRange struct {
	Start int `json:"start" validate:"required,min=1"`
	End   int `json:"end" validate:"required,min=1,gtefield=Start"`
}

// ItemConfig represents the configuration for each PDF element
type ItemConfig struct {
	Orientation         *string     `json:"orientation,omitempty" validate:"omitempty,oneof=landscape portrait"`
	DisplayHeaderFooter *bool       `json:"displayHeaderFooter,omitempty"`
	PrintBackground     *bool       `json:"printBackground,omitempty"`
	Scale               *float64    `json:"scale,omitempty" validate:"omitempty,min=0.1,max=2"`
	Size                *string     `json:"size,omitempty" validate:"omitempty,oneof=letter legal tabloid ledger a0 a1 a2 a3 a4 a5 a6"`
	Margin              *PageMargin `json:"margin,omitempty" validate:"omitempty"`
	PageRange           *PageRange  `json:"pageRange,omitempty" validate:"omitempty"`
	HeaderHTML          *string     `json:"headerHTML,omitempty"`
	FooterHTML          *string     `json:"footerHTML,omitempty"`
}

// PDFItem represents an individual PDF generation item
type PDFItem struct {
	BodyHTML string      `json:"bodyHTML" validate:"required"`
	Config   *ItemConfig `json:"config,omitempty" validate:"omitempty"`
}

// OutputConfig represents the output configuration of the generated PDF
type OutputConfig struct {
	Bucket   string `json:"bucket" validate:"required,s3bucket"`
	Key      string `json:"key" validate:"required,s3key"`
	CacheTTL *int64 `json:"cacheTTL,omitempty" validate:"omitempty,min=0"` // Cache TTL in seconds (0 = cache without expiry)
}

// GeneratePDFRequest represents the complete PDF generation request
type GeneratePDFRequest struct {
	Items  []PDFItem    `json:"items" validate:"required,dive"`
	Output OutputConfig `json:"output" validate:"required"`
}

// getPageSizeFromString converts a string representation of a page size to a PageSize struct
func getPageSizeFromString(size string) *dto.PageSize {
	// Default values
	width := gson.Num(8.5)
	height := gson.Num(11.0)

	switch size {
	case "letter":
		width = gson.Num(8.5)
		height = gson.Num(11.0)
	case "legal":
		width = gson.Num(8.5)
		height = gson.Num(14.0)
	case "tabloid":
		width = gson.Num(11.0)
		height = gson.Num(17.0)
	case "ledger":
		width = gson.Num(17.0)
		height = gson.Num(11.0)
	case "a0":
		width = gson.Num(33.1)
		height = gson.Num(46.8)
	case "a1":
		width = gson.Num(23.4)
		height = gson.Num(33.1)
	case "a2":
		width = gson.Num(16.5)
		height = gson.Num(23.4)
	case "a3":
		width = gson.Num(11.7)
		height = gson.Num(16.5)
	case "a4":
		width = gson.Num(8.27)
		height = gson.Num(11.7)
	case "a5":
		width = gson.Num(5.875)
		height = gson.Num(8.25)
	case "a6":
		width = gson.Num(4.125)
		height = gson.Num(5.875)
	}

	return &dto.PageSize{
		Width:  width,
		Height: height,
	}
}

// buildItemConfig safely converts a request ItemConfig to a domain ItemConfig, handling nil pointers
func buildItemConfig(config *ItemConfig) *dto.ItemConfig {
	if config == nil {
		return nil
	}

	itemConfig := &dto.ItemConfig{
		Orientation:         config.Orientation,
		DisplayHeaderFooter: config.DisplayHeaderFooter,
		HeaderHTML:          config.HeaderHTML,
		FooterHTML:          config.FooterHTML,
		PrintBackground:     config.PrintBackground,
		Scale:               config.Scale,
	}

	// Handle Size safely
	if config.Size != nil {
		itemConfig.Size = getPageSizeFromString(*config.Size)
	}

	// Handle Margin safely
	if config.Margin != nil {
		itemConfig.Margin = &dto.PageMargin{
			TopInches:    config.Margin.TopInches,
			BottomInches: config.Margin.BottomInches,
			LeftInches:   config.Margin.LeftInches,
			RightInches:  config.Margin.RightInches,
		}
	}

	// Handle PageRange safely
	if config.PageRange != nil {
		itemConfig.PageRange = &dto.PageRange{
			Start: config.PageRange.Start,
			End:   config.PageRange.End,
		}
	}

	return itemConfig
}

// ToDTO converts the request to a PDFGenerationDTO that can be used by the use case
func (r *GeneratePDFRequest) ToDTO() *dto.PDFGenerationDTO {
	// Default to 0 (cache without expiry) when no cache TTL is provided
	var cacheTTL int64
	if r.Output.CacheTTL != nil {
		cacheTTL = *r.Output.CacheTTL
	}

	output := dto.OutputConfig{
		Bucket:   r.Output.Bucket,
		Key:      r.Output.Key,
		CacheTTL: cacheTTL,
	}

	items := make([]dto.PDFItem, len(r.Items))

	for i, item := range r.Items {
		items[i] = dto.PDFItem{
			BodyHTML: item.BodyHTML,
			Config:   buildItemConfig(item.Config),
		}
	}

	return &dto.PDFGenerationDTO{
		Items:  items,
		Output: output,
	}
}
