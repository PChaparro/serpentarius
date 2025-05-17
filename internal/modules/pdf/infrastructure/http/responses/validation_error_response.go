package responses

// ValidationError represents a field validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrorResponse represents the response for validation errors
type ValidationErrorResponse struct {
	Message string            `json:"message"`
	Errors  []ValidationError `json:"errors"`
}
