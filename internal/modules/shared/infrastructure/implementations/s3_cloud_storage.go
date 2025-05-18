package implementations

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
	"github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure"
)

// S3CloudStorage implements the CloudStorage interface for AWS S3
type S3CloudStorage struct {
	client *s3.Client
}

var (
	s3CloudStorage *S3CloudStorage
	once           sync.Once
)

// GetS3CloudStorage returns a singleton instance of S3CloudStorage
func GetS3CloudStorage() definitions.CloudStorage {
	once.Do(func() {
		s3CloudStorage = &S3CloudStorage{
			client: createS3Client(),
		}
	})

	return s3CloudStorage
}

// createS3Client creates a shared S3 client instance
func createS3Client() *s3.Client {
	env := infrastructure.GetEnvironment()

	// Load the AWS SDK config
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(env.AwsRegion),
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     env.AwsAccessKeyID,
				SecretAccessKey: env.AwsSecretAccessKey,
			}, nil
		})),
	)
	if err != nil {
		panic("Unable to load AWS SDK config: " + err.Error())
	}

	// Set S3 options for custom endpoint
	s3Options := func(o *s3.Options) {
		o.BaseEndpoint = aws.String(env.AwsS3EndpointURL)
		o.UsePathStyle = true
	}

	// Create the S3 client
	client := s3.NewFromConfig(cfg, s3Options)
	return client
}

// UploadFile uploads a file to S3 and returns the URL
func (s *S3CloudStorage) UploadFile(request definitions.UploadFileRequest) (string, error) {
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(request.FileFolder),
		Key:         aws.String(request.FilePath),
		Body:        request.FileReader,
		ContentType: aws.String(request.ContentType),
	})

	if err != nil {
		return "", err
	}

	// Construct the public URL
	publicURL := fmt.Sprintf("%s/%s/%s", request.PublicURLPrefix, request.FileFolder, request.FilePath)
	return publicURL, nil
}

// FileExists checks if a file exists in the S3 bucket
func (s *S3CloudStorage) FileExists(request definitions.FileExistsRequest) (bool, error) {
	_, err := s.client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(request.FileFolder),
		Key:    aws.String(request.FilePath),
	})

	if err != nil {
		// Check if the error is because the file doesn't exist
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey" {
				return false, nil
			}
		}
		return false, err
	}

	return true, nil
}
