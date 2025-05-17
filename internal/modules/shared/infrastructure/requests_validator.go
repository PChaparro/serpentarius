package infrastructure

import (
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	validatorInstance *validator.Validate
	validatorOnce     sync.Once
)

// GetValidatorInstance returns a singleton instance of the validator
func GetValidatorInstance() *validator.Validate {
	validatorOnce.Do(func() {
		validatorInstance = validator.New(validator.WithRequiredStructEnabled())
	})
	return validatorInstance
}

// GetUserFriendlyValidationErrorMessage returns a human-readable error message for validation errors
func GetUserFriendlyValidationErrorMessage(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return "This field is required"
	case "min":
		return "Value must be greater than " + err.Param()
	case "max":
		return "Value must be less than " + err.Param()
	case "oneof":
		return "Value must be one of: " + err.Param()
	case "gtefield":
		return "Value must be greater than or equal to " + err.Param() + " field"
	case "filename":
		return "Must be a valid file name"
	case "url":
		return "Must be a valid URL"
	default:
		return "Invalid value"
	}
}
