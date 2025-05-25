package requests

import (
	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"
	"github.com/ysmood/gson"
)

// PageMargin represents the margins of the PDF page
type PageMargin struct {
	Top    *float64 `json:"top,omitempty" validate:"omitempty,min=0"`
	Bottom *float64 `json:"bottom,omitempty" validate:"omitempty,min=0"`
	Left   *float64 `json:"left,omitempty" validate:"omitempty,min=0"`
	Right  *float64 `json:"right,omitempty" validate:"omitempty,min=0"`
}

// PageRange represents the range of pages to print
type PageRange struct {
	Start int `json:"start,omitempty" validate:"omitempty,min=1"`
	End   int `json:"end,omitempty" validate:"omitempty,min=1,gtefield=Start"`
}

// ItemConfig represents the configuration for each PDF element
type ItemConfig struct {
	Orientation         *string     `json:"orientation,omitempty" validate:"omitempty,oneof=landscape portrait"`
	DisplayHeaderFooter *bool       `json:"displayHeaderFooter,omitempty"`
	PrintBackground     *bool       `json:"printBackground,omitempty"`
	Scale               *float64    `json:"scale,omitempty" validate:"omitempty,min=0.1,max=2"`
	Size                *string     `json:"size,omitempty" validate:"omitempty,oneof=letter legal tabloid ledger a0 a1 a2 a3 a4 a5 a6"`
	Margin              *PageMargin `json:"margin,omitempty" validate:"omitempty"`
	PageRanges          *PageRange  `json:"pageRanges,omitempty" validate:"omitempty"`
	HeaderHTML          *string     `json:"headerHTML,omitempty"`
	FooterHTML          *string     `json:"footerHTML,omitempty"`
}

// PDFItem represents an individual PDF generation item
type PDFItem struct {
	BodyHTML string      `json:"bodyHTML" validate:"required"`
	Config   *ItemConfig `json:"config,omitempty" validate:"omitempty"`
}

// GeneralConfig represents the general PDF configuration
type GeneralConfig struct {
	Directory       string `json:"directory" validate:"required"`
	FileName        string `json:"fileName" validate:"required"`
	PublicURLPrefix string `json:"publicURLPrefix,omitempty" validate:"required,http_url"`
	Expiration      *int64 `json:"expiration,omitempty" validate:"omitempty,min=0"` // Expiration time in seconds
}

// GeneratePDFReturningURLRequest represents the complete PDF generation request
type GeneratePDFReturningURLRequest struct {
	Items  []PDFItem     `json:"items" validate:"required,dive"`
	Config GeneralConfig `json:"config" validate:"required"`
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
			Top:    config.Margin.Top,
			Bottom: config.Margin.Bottom,
			Left:   config.Margin.Left,
			Right:  config.Margin.Right,
		}
	}

	// Handle PageRanges safely
	if config.PageRanges != nil {
		itemConfig.PageRanges = &dto.PageRange{
			Start: config.PageRanges.Start,
			End:   config.PageRanges.End,
		}
	}

	return itemConfig
}

// ToDTO converts the request to a PDFGenerationDTO that can be used by the use case
func (r *GeneratePDFReturningURLRequest) ToDTO() *dto.PDFGenerationDTO {
	config := dto.GeneralConfig{
		Directory:       r.Config.Directory,
		FileName:        r.Config.FileName,
		PublicURLPrefix: r.Config.PublicURLPrefix,
		Expiration:      r.Config.Expiration,
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
		Config: config,
	}
}
