package helpers

import (
	"os"
)

// envOr returns the value of the environment variable named key or fallback
// when the variable is unset or empty.
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
