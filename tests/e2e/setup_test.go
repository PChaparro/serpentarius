package e2e

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	pdfImplementations "github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/implementations"
	sharedHttp "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/http"
	sharedImplementations "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/implementations"
	"github.com/PChaparro/serpentarius/tests/e2e/stubs"
)

// containerEndpoint carries the reachable host/port of a testcontainer service.
type containerEndpoint struct {
	Container testcontainers.Container
	Host      string
	Port      string
}

// redisContainerConfig groups the parameters for starting the Redis container.
type redisContainerConfig struct {
	Ctx      context.Context
	Image    string
	Password string
}

// minioContainerConfig groups the parameters for starting the MinIO container.
type minioContainerConfig struct {
	Ctx      context.Context
	Image    string
	User     string
	Password string
}

var (
	testServer       *httptest.Server
	ServerURL        string
	TestBucket       string
	PDFGeneratorStub *stubs.StubPDFGenerator
	CloudStorageStub *stubs.StubCloudStorage
	CacheStorageStub *stubs.StubCacheStorage
)

func TestMain(m *testing.M) {
	// Production containers run in UTC, so the whole test binary must too.
	time.Local = time.UTC

	ctx := context.Background()

	redisInfo, err := startRedisContainer(redisContainerConfig{
		Ctx:      ctx,
		Image:    "redis:7-alpine",
		Password: "test-redis-password",
	})
	if err != nil {
		fmt.Printf("FATAL: Failed to start Redis container: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = redisInfo.Container.Terminate(ctx) }()

	minioCreds := minioCredentials{User: "e2e-minio-user", Password: "e2e-minio-password"}
	minioInfo, err := startMinIOContainer(minioContainerConfig{
		Ctx:      ctx,
		Image:    "minio/minio:latest",
		User:     minioCreds.User,
		Password: minioCreds.Password,
	})
	if err != nil {
		fmt.Printf("FATAL: Failed to start MinIO container: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = minioInfo.Container.Terminate(ctx) }()

	setEnvVars(envVarsArgs{
		RedisHost:     redisInfo.Host,
		RedisPort:     redisInfo.Port,
		RedisPassword: "test-redis-password",
		MinIOHost:     minioInfo.Host,
		MinIOPort:     minioInfo.Port,
		MinIOUser:     minioCreds.User,
		MinIOPassword: minioCreds.Password,
	})

	// The bucket is ephemeral per TestMain run so objects never leak across runs.
	TestBucket = fmt.Sprintf("e2e-%d", time.Now().UTC().UnixNano())
	if err := createBucket(ctx, minioCreds, TestBucket); err != nil {
		fmt.Printf("FATAL: Failed to create test bucket: %v\n", err)
		os.Exit(1)
	}

	ensureOutputDir()

	// Stubs must be injected before the router is built: the PDF routes
	// resolve every singleton while registering.
	injectStubs()

	testServer = httptest.NewServer(sharedHttp.RegisterRoutes())
	defer testServer.Close()
	ServerURL = testServer.URL

	fmt.Printf("Test server ready at %s (bucket %s)\n", ServerURL, TestBucket)

	code := m.Run()
	closeSharedRealServer()
	os.Exit(code)
}

func startRedisContainer(cfg redisContainerConfig) (containerEndpoint, error) {
	fmt.Printf("Starting Redis container (image: %s)...\n", cfg.Image)

	redisContainer, err := testcontainers.GenericContainer(cfg.Ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        cfg.Image,
			ExposedPorts: []string{"6379/tcp"},
			Cmd:          []string{"redis-server", "--requirepass", cfg.Password},
			WaitingFor: wait.ForLog("Ready to accept connections").
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return containerEndpoint{}, fmt.Errorf("starting container: %w", err)
	}

	host, err := redisContainer.Host(cfg.Ctx)
	if err != nil {
		_ = redisContainer.Terminate(cfg.Ctx)
		return containerEndpoint{}, fmt.Errorf("getting container host: %w", err)
	}

	mappedPort, err := redisContainer.MappedPort(cfg.Ctx, "6379/tcp")
	if err != nil {
		_ = redisContainer.Terminate(cfg.Ctx)
		return containerEndpoint{}, fmt.Errorf("getting mapped port: %w", err)
	}

	fmt.Printf("Redis container ready at %s:%s\n", host, mappedPort.Port())

	return containerEndpoint{Container: redisContainer, Host: host, Port: mappedPort.Port()}, nil
}

func startMinIOContainer(cfg minioContainerConfig) (containerEndpoint, error) {
	fmt.Printf("Starting MinIO container (image: %s)...\n", cfg.Image)

	minioContainer, err := testcontainers.GenericContainer(cfg.Ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: cfg.Image,
			Env: map[string]string{
				"MINIO_ROOT_USER":     cfg.User,
				"MINIO_ROOT_PASSWORD": cfg.Password,
			},
			ExposedPorts: []string{"9000/tcp"},
			Cmd:          []string{"server", "/data"},
			WaitingFor: wait.ForHTTP("/minio/health/live").
				WithPort("9000/tcp").
				WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return containerEndpoint{}, fmt.Errorf("starting container: %w", err)
	}

	host, err := minioContainer.Host(cfg.Ctx)
	if err != nil {
		_ = minioContainer.Terminate(cfg.Ctx)
		return containerEndpoint{}, fmt.Errorf("getting container host: %w", err)
	}

	mappedPort, err := minioContainer.MappedPort(cfg.Ctx, "9000/tcp")
	if err != nil {
		_ = minioContainer.Terminate(cfg.Ctx)
		return containerEndpoint{}, fmt.Errorf("getting mapped port: %w", err)
	}

	fmt.Printf("MinIO container ready at %s:%s\n", host, mappedPort.Port())

	return containerEndpoint{Container: minioContainer, Host: host, Port: mappedPort.Port()}, nil
}

// envVarsArgs groups every dynamic value TestMain exports to the environment.
type envVarsArgs struct {
	RedisHost     string
	RedisPort     string
	RedisPassword string
	MinIOHost     string
	MinIOPort     string
	MinIOUser     string
	MinIOPassword string
}

func setEnvVars(args envVarsArgs) {
	os.Setenv("ENVIRONMENT", "test")

	// S3-compatible storage (MinIO container)
	os.Setenv("AWS_S3_ENDPOINT_URL", fmt.Sprintf("http://%s:%s", args.MinIOHost, args.MinIOPort))
	os.Setenv("AWS_ACCESS_KEY_ID", args.MinIOUser)
	os.Setenv("AWS_SECRET_ACCESS_KEY", args.MinIOPassword)
	os.Setenv("AWS_REGION", "us-east-1")

	// Cache (Redis container)
	os.Setenv("REDIS_HOST", args.RedisHost)
	os.Setenv("REDIS_PORT", args.RedisPort)
	os.Setenv("REDIS_PASSWORD", args.RedisPassword)
	os.Setenv("REDIS_DB", "0")

	// Auth
	os.Setenv("AUTH_SECRET", "test-auth-secret")

	// Chromium is the real host binary (never containerized); stubbed suites
	// replace the generator, while real-render suites use this binary directly.
	if os.Getenv("CHROMIUM_BINARY_PATH") == "" {
		os.Setenv("CHROMIUM_BINARY_PATH", "/usr/bin/chromium")
	}
}

// minioCredentials carries the root credentials of the MinIO container.
type minioCredentials struct {
	User     string
	Password string
}

func createBucket(ctx context.Context, creds minioCredentials, bucket string) error {
	endpoint := os.Getenv("AWS_S3_ENDPOINT_URL")

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(aws.CredentialsProviderFunc(
			func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     creds.User,
					SecretAccessKey: creds.Password,
				}, nil
			},
		)),
	)
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		return fmt.Errorf("creating bucket: %w", err)
	}

	fmt.Printf("Test bucket %q created\n", bucket)

	return nil
}

func ensureOutputDir() {
	outputDir := filepath.Join("testdata", "output")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Printf("FATAL: Failed to create PDF output dir: %v\n", err)
		os.Exit(1)
	}
}

func injectStubs() {
	PDFGeneratorStub = &stubs.StubPDFGenerator{}
	pdfImplementations.SetPDFGeneratorForTesting(PDFGeneratorStub)

	CloudStorageStub = &stubs.StubCloudStorage{}
	sharedImplementations.SetCloudStorageForTesting(CloudStorageStub)

	// All three singletons are resolved when the routes are built, so this
	// must run before httptest.NewServer calls RegisterRoutes().
	CacheStorageStub = &stubs.StubCacheStorage{}
	sharedImplementations.SetCacheStorageForTesting(CacheStorageStub)
}
