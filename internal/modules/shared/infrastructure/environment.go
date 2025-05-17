package infrastructure

import (
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

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

func GetEnvironment() *EnvironmentSpec {
	environmentOnce.Do(func() {
		loadFromEnvFile()
		initializeEnvironmentInstance()
	})

	return environment
}

func loadFromEnvFile() {
	execEnvironment := os.Getenv("ENVIRONMENT")

	if execEnvironment != "production" {
		err := godotenv.Load(".env")
		if err != nil {
			log.Fatal("[ERROR] ", err.Error())
		}
	}
}

func initializeEnvironmentInstance() {
	environment = &EnvironmentSpec{}

	err := envconfig.Process("", environment)

	if err != nil {
		log.Fatal("[ERROR] ", err.Error())
	}
}
