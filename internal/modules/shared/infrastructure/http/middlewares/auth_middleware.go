package middlewares

import (
	"net/http"
	"strings"

	sharedInfrastructure "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates the Authorization header against the AUTH_SECRET environment variable
// The header must be in the format "Bearer {token}" where {token} matches the AUTH_SECRET
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Authorization header is required",
			})
			return
		}

		// Check if it starts with "Bearer "
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Authorization header must start with 'Bearer'",
			})
			return
		}

		// Extract the token
		token := strings.TrimPrefix(authHeader, prefix)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Token cannot be empty",
			})
			return
		}

		// Compare the token with the environment variable
		envSecret := sharedInfrastructure.GetEnvironment().AuthSecret
		if token != envSecret {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Authorization token is wrong",
			})
			return
		}

		// If token is valid, proceed with the request
		c.Next()
	}
}
