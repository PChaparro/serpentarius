package infrastructure

import (
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

const (
	ENVIRONMENT_PRODUCTION  = "production"
	ENVIRONMENT_DEVELOPMENT = "development"
)

// EnvironmentSpec holds the configuration for the application environment.
type EnvironmentSpec struct {
	Environment string `split_words:"true" default:"development"` // App environment (development/production)

	// Binaries
	ChromiumBinaryPath string `split_words:"true" default:"/usr/bin/chromium-browser"` // Path to Chromium binary

	// Concurrency
	MaxChromiumBrowsers       int `split_words:"true" default:"1"`  // Max concurrent Chromium browsers
	MaxChromiumTabsPerBrowser int `split_words:"true" default:"4"`  // Max tabs per browser
	MaxChromiumTabIdleSeconds int `split_words:"true" default:"30"` // Max seconds a tab can be idle

	// AWS S3
	AwsS3EndpointURL   string `split_words:"true" default:"https://s3.amazonaws.com"` // S3 endpoint URL
	AwsAccessKeyID     string `required:"true" split_words:"true"`                    // S3 access key
	AwsSecretAccessKey string `required:"true" split_words:"true"`                    // S3 secret key
	AwsRegion          string `split_words:"true" default:"us-east-1"`                // S3 region

	// Redis
	RedisHost     string `required:"true" split_words:"true"` // Redis host
	RedisPort     string `split_words:"true" default:"6379"`  // Redis port
	RedisPassword string `required:"true" split_words:"true"` // Redis password
	RedisDB       int    `split_words:"true" default:"0"`     // Redis DB number

	// Authentication
	AuthSecret string `required:"true" split_words:"true"` // Secret for JWT auth
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

	if execEnvironment != ENVIRONMENT_PRODUCTION {
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
