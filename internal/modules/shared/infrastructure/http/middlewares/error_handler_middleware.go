package middlewares

import (
	"net/http"

	sharedErrors "github.com/PChaparro/serpentarius/internal/modules/shared/domain/errors"
	"github.com/gin-gonic/gin"
)

// domainErrorCodeToHTTPStatusCode maps error codes to HTTP status codes
var domainErrorCodeToHTTPStatusCode = map[string]int{
	"ERROR": http.StatusInternalServerError,
}

// ErrorHandlerMiddleware is a Gin middleware that handles errors returned by the application
// and sends appropriate HTTP responses to the client.
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors[0]

			switch e := err.Err.(type) {
			// Handle domain errors
			case sharedErrors.DomainError:
				domainErrorCode := e.Code()

				statusCode, isErrorCodeMapped := domainErrorCodeToHTTPStatusCode[domainErrorCode]
				if !isErrorCodeMapped {
					// If the error code is not mapped, use the default error code
					statusCode = http.StatusInternalServerError
				}

				c.JSON(statusCode, gin.H{
					"message":  e.Message(),
					"metadata": e.Metadata(),
				})
			// Handle unexpected errors
			default:
				c.JSON(500, gin.H{
					"message": "There was an error processing your request",
					// "error":   e.Error(),
				})
			}
		}
	}
}
