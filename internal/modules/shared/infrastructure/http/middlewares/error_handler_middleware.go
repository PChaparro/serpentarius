package middlewares

import (
	"errors"
	"net/http"

	sharedErrors "github.com/PChaparro/serpentarius/internal/modules/shared/domain/errors"
	sharedUtilities "github.com/PChaparro/serpentarius/internal/modules/shared/utilities"
	"github.com/gin-gonic/gin"
)

// domainErrorCodeToHTTPStatusCode maps error codes to HTTP status codes.
// Add entries here as domain errors are introduced, e.g.:
// "ENTITY_NOT_FOUND": http.StatusNotFound,
var domainErrorCodeToHTTPStatusCode = map[string]int{}

// ErrorHandlerMiddleware is a Gin middleware that handles errors returned by the application
// and sends appropriate HTTP responses to the client.
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors[0]

			sharedUtilities.
				GetLogger().
				WithError(err.Err).
				Error("An error occurred while processing the request")

			var domainErr sharedErrors.DomainError
			if errors.As(err.Err, &domainErr) {
				// Handle domain errors
				domainErrorCode := domainErr.Code()

				statusCode, isErrorCodeMapped := domainErrorCodeToHTTPStatusCode[domainErrorCode]
				if !isErrorCodeMapped {
					// If the error code is not mapped, use the default error code
					statusCode = http.StatusInternalServerError
				}

				c.JSON(statusCode, gin.H{
					"message":  domainErr.Message(),
					"metadata": domainErr.Metadata(),
				})
			} else {
				// Handle unexpected errors
				c.JSON(500, gin.H{
					"message": "There was an error processing your request",
					// "error":   err.Err.Error(),
				})
			}
		}
	}
}
