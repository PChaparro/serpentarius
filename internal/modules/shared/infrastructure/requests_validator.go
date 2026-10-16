package infrastructure

import (
	"regexp"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	validatorInstance *validator.Validate
	validatorOnce     sync.Once
)

// s3BucketNamePattern matches the pragmatic AWS S3 bucket subset:
// lowercase letters, digits, dots and hyphens, starting and ending alphanumerically.
var s3BucketNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

// cacheNamespacePattern matches colon-separated segments of alphanumerics,
// underscores, dots and hyphens (case-sensitive).
var cacheNamespacePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+(?::[A-Za-z0-9_.-]+)*$`)

// validateS3Bucket checks a value against the pragmatic S3 bucket subset
// (3-63 chars, restricted charset, no consecutive dots).
func validateS3Bucket(fl validator.FieldLevel) bool {
	bucket := fl.Field().String()
	if len(bucket) < 3 || len(bucket) > 63 {
		return false
	}
	if strings.Contains(bucket, "..") {
		return false
	}
	return s3BucketNamePattern.MatchString(bucket)
}

// validateS3Key checks a value against the S3 key rules
// (1-1024 bytes, must not start with a slash).
func validateS3Key(fl validator.FieldLevel) bool {
	key := fl.Field().String()
	if len(key) < 1 || len(key) > 1024 {
		return false
	}
	return !strings.HasPrefix(key, "/")
}

// validateCacheNamespace checks a value against the cache namespace rules
// (1-128 chars, charset [A-Za-z0-9_.-] with colon-separated segments, no consecutive dots).
func validateCacheNamespace(fl validator.FieldLevel) bool {
	namespace := fl.Field().String()
	if len(namespace) < 1 || len(namespace) > 128 {
		return false
	}
	if strings.Contains(namespace, "..") {
		return false
	}
	return cacheNamespacePattern.MatchString(namespace)
}

// GetValidatorInstance returns a singleton instance of the validator
func GetValidatorInstance() *validator.Validate {
	validatorOnce.Do(func() {
		validatorInstance = validator.New(validator.WithRequiredStructEnabled())
		// Register custom validators for S3 bucket and key formats
		_ = validatorInstance.RegisterValidation("s3bucket", validateS3Bucket)
		_ = validatorInstance.RegisterValidation("s3key", validateS3Key)
		_ = validatorInstance.RegisterValidation("cachenamespace", validateCacheNamespace)
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
		return "Value must be at most " + err.Param()
	case "oneof":
		return "Value must be one of: " + err.Param()
	case "gtefield":
		return "Value must be greater than or equal to " + err.Param() + " field"
	case "http_url":
		return "Must be a valid http URL"
	case "s3bucket":
		return "Must be a valid S3 bucket name (3-63 lowercase alphanumeric characters, dots or hyphens)"
	case "s3key":
		return "Must be a valid S3 key (1-1024 characters, must not start with '/')"
	case "cachenamespace":
		return "Must be a valid cache namespace (1-128 characters, alphanumeric, '_', '.', '-' and ':' separators)"
	default:
		return "Invalid value"
	}
}
