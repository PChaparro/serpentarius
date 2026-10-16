package implementations

import (
	"context"
	"errors"
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
	s3CloudStorage definitions.CloudStorage
	once           sync.Once
)

// GetS3CloudStorage returns a singleton instance of S3CloudStorage
func GetS3CloudStorage() definitions.CloudStorage {
	once.Do(func() {
		if s3CloudStorage == nil {
			s3CloudStorage = &S3CloudStorage{
				client: createS3Client(),
			}
		}
	})

	return s3CloudStorage
}

// SetCloudStorageForTesting replaces the singleton with a test double (must be called before GetS3CloudStorage).
func SetCloudStorageForTesting(storage definitions.CloudStorage) {
	s3CloudStorage = storage
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

// UploadFile uploads a file to S3 and returns its object key
func (s *S3CloudStorage) UploadFile(request definitions.UploadFileRequest) (string, error) {
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(request.Bucket),
		Key:         aws.String(request.Key),
		Body:        request.FileReader,
		ContentType: aws.String(request.ContentType),
		Metadata:    request.Metadata,
	})

	if err != nil {
		return "", err
	}

	// Return the object key
	return request.Key, nil
}

// StatFile returns whether a file exists in the S3 bucket along with its metadata
func (s *S3CloudStorage) StatFile(request definitions.FileExistsRequest) (definitions.StatFileResult, error) {
	output, err := s.client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(request.Bucket),
		Key:    aws.String(request.Key),
	})

	if err != nil {
		// Check if the error is because the file doesn't exist
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey" {
				return definitions.StatFileResult{
					Exists: false,
				}, nil
			}
		}

		return definitions.StatFileResult{
			Exists: false,
		}, err
	}

	return definitions.StatFileResult{
		Exists:   true,
		Metadata: output.Metadata,
	}, nil
}
