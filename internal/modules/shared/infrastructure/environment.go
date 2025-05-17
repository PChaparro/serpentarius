package infrastructure

import (
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// EnvironmentSpec holds the configuration for the application environment.
type EnvironmentSpec struct {
	// AWS S3
	AwsS3EndpointURL   string `split_words:"true" default:"https://s3.amazonaws.com"`
	AwsAccessKeyID     string `required:"true" split_words:"true"`
	AwsSecretAccessKey string `required:"true" split_words:"true"`
	AwsRegion          string `split_words:"true" default:"us-east-1"`
}

var (
	environment     *EnvironmentSpec
	environmentOnce sync.Once
)

// GetEnvironment returns a singleton instance of the EnvironmentSpec.
func GetEnvironment() *EnvironmentSpec {
	environmentOnce.Do(func() {
		loadFromEnvFile()
		initializeEnvironmentInstance()
	})

	return environment
}

// loadFromEnvFile loads environment variables from a .env file if not in production.
func loadFromEnvFile() {
	execEnvironment := os.Getenv("ENVIRONMENT")

	if execEnvironment != "production" {
		err := godotenv.Load(".env")
		if err != nil {
			log.Fatal("[ERROR] ", err.Error())
		}
	}
}

// initializeEnvironmentInstance initializes the EnvironmentSpec instance with environment variables.
func initializeEnvironmentInstance() {
	environment = &EnvironmentSpec{}

	err := envconfig.Process("", environment)

	if err != nil {
		log.Fatal("[ERROR] ", err.Error())
	}
}
