package dto

// PageSize represents the dimensions of the PDF page
type PageSize struct {
	Width  *float64
	Height *float64
}

// PageMargin represents the margins of the PDF page
type PageMargin struct {
	Top    *float64
	Bottom *float64
	Left   *float64
	Right  *float64
}

// PageRange represents the range of pages to print
type PageRange struct {
	Start int
	End   int
}

// ItemConfig represents the configuration for each PDF element
type ItemConfig struct {
	Orientation         *string
	DisplayHeaderFooter *bool
	PrintBackground     *bool
	Scale               *float64
	Size                *PageSize
	Margin              *PageMargin
	PageRanges          *PageRange
	HeaderHTML          *string
	FooterHTML          *string
}

// PDFItem represents an individual PDF generation item
type PDFItem struct {
	BodyHTML string // Required field
	Config   *ItemConfig
}

// GeneralConfig represents the general PDF configuration
type GeneralConfig struct {
	Directory  string // Required field
	FileName   string // Required field
	Expiration *int64
}

// PDFGenerationDTO represents the complete PDF generation request
type PDFGenerationDTO struct {
	Items  []PDFItem
	Config GeneralConfig
}
