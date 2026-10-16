package helpers

import (
	"bytes"
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
)

// s3ClientFromEnv builds an S3 client from the AWS_* variables set by TestMain.
// Path-style URLs are required for MinIO compatibility, mirroring prod.
func s3ClientFromEnv(t *testing.T) *s3.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(envOr("AWS_REGION", "us-east-1")),
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(
			func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     envOr("AWS_ACCESS_KEY_ID", ""),
					SecretAccessKey: envOr("AWS_SECRET_ACCESS_KEY", ""),
				}, nil
			},
		)),
	)
	require.NoError(t, err, "loading AWS config for test S3 client")

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(envOr("AWS_S3_ENDPOINT_URL", "http://localhost:9000"))
		o.UsePathStyle = true
	})
}

// EnsureBucketArgs groups the parameters for ensuring a bucket exists.
type EnsureBucketArgs struct {
	Bucket string
}

// EnsureBucket creates the bucket when missing. It is idempotent, so suites
// can call it in SetupSuite to prove MinIO responds and the bucket is ready.
func EnsureBucket(t *testing.T, args EnsureBucketArgs) {
	t.Helper()
	require.NotEmpty(t, args.Bucket, "bucket name must not be empty")

	client := s3ClientFromEnv(t)

	_, err := client.HeadBucket(context.Background(), &s3.HeadBucketInput{
		Bucket: aws.String(args.Bucket),
	})
	if err == nil {
		return
	}

	_, err = client.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(args.Bucket),
	})
	require.NoError(t, err, "creating test bucket %q", args.Bucket)
}

// DownloadObjectArgs groups the parameters for downloading an object.
type DownloadObjectArgs struct {
	Bucket string
	Key    string
}

// DownloadObjectResult carries the downloaded object bytes and its metadata.
type DownloadObjectResult struct {
	Bytes       []byte
	Metadata    map[string]string
	ContentType string
}

// DownloadObject fetches an object from S3-compatible storage. Used in later
// phases to assert magic bytes, page count, extracted text and render-hash
// metadata of the object the API stored.
func DownloadObject(t *testing.T, args DownloadObjectArgs) DownloadObjectResult {
	t.Helper()

	client := s3ClientFromEnv(t)

	head, err := client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(args.Bucket),
		Key:    aws.String(args.Key),
	})
	require.NoError(t, err, "stating object %q in bucket %q", args.Key, args.Bucket)

	get, err := client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(args.Bucket),
		Key:    aws.String(args.Key),
	})
	require.NoError(t, err, "downloading object %q in bucket %q", args.Key, args.Bucket)
	defer func() { _ = get.Body.Close() }()

	var buf bytes.Buffer
	_, err = buf.ReadFrom(get.Body)
	require.NoError(t, err, "reading object body")

	contentType := ""
	if get.ContentType != nil {
		contentType = *get.ContentType
	}

	return DownloadObjectResult{
		Bytes:       buf.Bytes(),
		Metadata:    head.Metadata,
		ContentType: contentType,
	}
}

// CleanupBucketArgs groups the parameters for emptying a bucket.
type CleanupBucketArgs struct {
	Bucket string
}

// CleanupBucket deletes every object in the bucket. Call it in SetupSuite so
// objects from a previous run never leak into the current one.
func CleanupBucket(t *testing.T, args CleanupBucketArgs) {
	t.Helper()

	client := s3ClientFromEnv(t)

	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(args.Bucket),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		require.NoError(t, err, "listing objects in bucket %q", args.Bucket)

		for _, obj := range page.Contents {
			_, err := client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
				Bucket: aws.String(args.Bucket),
				Key:    obj.Key,
			})
			require.NoError(t, err, "deleting object %q", aws.ToString(obj.Key))
		}
	}
}
