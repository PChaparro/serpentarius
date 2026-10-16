package dto

// PageSize represents the dimensions of the PDF page
type PageSize struct {
	Width  *float64
	Height *float64
}

// PageMargin represents the margins of the PDF page in inches
type PageMargin struct {
	TopInches    *float64
	BottomInches *float64
	LeftInches   *float64
	RightInches  *float64
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
	PageRange           *PageRange
	HeaderHTML          *string
	FooterHTML          *string
}

// PDFItem represents an individual PDF generation item
type PDFItem struct {
	BodyHTML string // Required field
	Config   *ItemConfig
}

// OutputConfig represents the output configuration of the generated PDF
type OutputConfig struct {
	Bucket string // Required field
	Key    string // Required field
}

// CacheConfig represents the cache configuration for the generated PDF
type CacheConfig struct {
	TTL       int64  // Optional field, time in seconds for the cache entry to live (0 = cache without expiry)
	Namespace string // Optional field, namespace segment for the cache key
}

// PDFGenerationDTO represents the complete PDF generation request
type PDFGenerationDTO struct {
	Items  []PDFItem
	Output OutputConfig
	Cache  CacheConfig
}

// PDFGenerationResult represents the file reference of a generated PDF
type PDFGenerationResult struct {
	Bucket string
	Key    string
}
