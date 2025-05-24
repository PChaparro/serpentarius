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
	Environment string `split_words:"true" default:"development"`

	// Binaries
	ChromiumBinaryPath string `split_words:"true" default:"/usr/bin/chromium-browser"`

	// Concurrency
	MaxChromiumBrowsers       int `split_words:"true" default:"1"`
	MaxChromiumTabsPerBrowser int `split_words:"true" default:"4"`
	MaxIdleSeconds            int `split_words:"true" default:"30"`

	// AWS S3
	AwsS3EndpointURL   string `split_words:"true" default:"https://s3.amazonaws.com"`
	AwsAccessKeyID     string `required:"true" split_words:"true"`
	AwsSecretAccessKey string `required:"true" split_words:"true"`
	AwsRegion          string `split_words:"true" default:"us-east-1"`

	// Redis
	RedisHost     string `required:"true" split_words:"true"`
	RedisPort     string `split_words:"true" default:"6379"`
	RedisPassword string `required:"true" split_words:"true"`
	RedisDB       int    `split_words:"true" default:"0"`

	// Authentication
	AuthSecret string `required:"true" split_words:"true"`
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
