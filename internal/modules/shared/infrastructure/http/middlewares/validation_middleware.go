package middlewares

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	sharedInfrastructure "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// RequestValidationMiddleware validates the request body against the provided struct type
func RequestValidationMiddleware(structType any) gin.HandlerFunc {
	return func(c *gin.Context) {
		defaultBadRequestResponse := map[string]any{
			"message": "Could not validate request. Please, make sure all fields are of the correct type (E.g, ints are not strings) and that the request body is a valid JSON and try again.",
			"errors":  []string{},
		}

		// Create a new instance of the struct
		val := reflect.ValueOf(structType)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		requestStructPtr := reflect.New(val.Type()).Interface()

		// Decode request body
		if err := c.ShouldBindJSON(requestStructPtr); err != nil {
			c.JSON(http.StatusBadRequest, defaultBadRequestResponse)
			c.Abort()
			return
		}

		// Validate request
		if err := sharedInfrastructure.GetValidatorInstance().Struct(requestStructPtr); err != nil {
			validationErrors, ok := err.(validator.ValidationErrors)
			if !ok {
				c.JSON(http.StatusBadRequest, defaultBadRequestResponse)
				c.Abort()
				return
			}

			// Format validation errors for user-friendly output
			errors := make([]string, 0)

			for _, err := range validationErrors {
				// Get the full path of nested fields
				fieldPath := err.Namespace()
				// Remove the struct name from the beginning
				if idx := strings.Index(fieldPath, "."); idx != -1 {
					fieldPath = fieldPath[idx+1:]
				}
				errors = append(errors, fmt.Sprintf("Field '%s': %s", fieldPath, sharedInfrastructure.GetUserFriendlyValidationErrorMessage(err)))
			}

			c.JSON(http.StatusBadRequest, map[string]any{
				"message": "Validation failed",
				"errors":  errors,
			})
			c.Abort()
			return
		}

		// Store the validated struct in the context
		c.Set("validated_request_body", requestStructPtr)
		c.Next()
	}
}

// GetValidatedRequest extracts the validated request from the context
func GetValidatedRequest(c *gin.Context) any {
	return c.MustGet("validated_request_body")
}
