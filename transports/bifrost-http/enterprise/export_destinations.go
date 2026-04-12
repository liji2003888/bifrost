package enterprise

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ExportDestination is the interface that all export destinations implement.
type ExportDestination interface {
	// Upload uploads a local file to the destination storage.
	Upload(ctx context.Context, filePath string, objectKey string) error
	// Name returns a human-readable identifier for this destination.
	Name() string
}

// NewDestination creates an ExportDestination from a destination type and configuration map.
func NewDestination(destType string, cfg map[string]any) (ExportDestination, error) {
	switch destType {
	case "local", "":
		return &LocalDestination{}, nil
	case "s3":
		return newS3Destination(cfg)
	case "gcs":
		return newGCSDestination(cfg)
	case "azure_blob":
		return newAzureBlobDestination(cfg)
	default:
		return nil, fmt.Errorf("unsupported destination type %q: must be one of: local, s3, gcs, azure_blob", destType)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// LocalDestination — keeps files on disk (existing behavior, no upload needed)
// ─────────────────────────────────────────────────────────────────────────────

// LocalDestination is a no-op destination that keeps files on local disk.
type LocalDestination struct{}

func (d *LocalDestination) Name() string { return "local" }

// Upload is a no-op for local destination since the file is already on disk.
func (d *LocalDestination) Upload(_ context.Context, _ string, _ string) error {
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// S3Destination — uploads to Amazon S3
// ─────────────────────────────────────────────────────────────────────────────

// S3Destination uploads export files to Amazon S3.
type S3Destination struct {
	client   *s3.Client
	bucket   string
	prefix   string
	endpoint string // optional custom endpoint (e.g., for MinIO)
}

func newS3Destination(cfg map[string]any) (*S3Destination, error) {
	bucket, _ := cfg["bucket"].(string)
	if bucket == "" {
		return nil, fmt.Errorf("s3 destination requires 'bucket' in destination_config")
	}
	region, _ := cfg["region"].(string)
	if region == "" {
		region = "us-east-1"
	}
	prefix, _ := cfg["prefix"].(string)
	endpoint, _ := cfg["endpoint"].(string)

	// Build AWS config options
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	// Support explicit credentials (fallback to IAM role / environment)
	accessKeyID, _ := cfg["access_key_id"].(string)
	secretAccessKey, _ := cfg["secret_access_key"].(string)
	sessionToken, _ := cfg["session_token"].(string)
	if accessKeyID != "" && secretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, sessionToken),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for S3 destination: %w", err)
	}

	s3Opts := []func(*s3.Options){}
	if endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	return &S3Destination{
		client:   client,
		bucket:   bucket,
		prefix:   prefix,
		endpoint: endpoint,
	}, nil
}

func (d *S3Destination) Name() string { return "s3" }

// Upload streams the local file to the configured S3 bucket.
func (d *S3Destination) Upload(ctx context.Context, filePath string, objectKey string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("s3 upload: failed to open file %s: %w", filePath, err)
	}
	defer f.Close()

	key := objectKey
	if d.prefix != "" {
		key = d.prefix + "/" + objectKey
	}

	_, err = d.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(key),
		Body:   f,
	})
	if err != nil {
		return fmt.Errorf("s3 upload: failed to put object %q in bucket %q: %w", key, d.bucket, err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// GCSDestination — stub (Google Cloud Storage SDK not in go.mod as direct dep)
// ─────────────────────────────────────────────────────────────────────────────

// GCSDestination is a stub implementation for Google Cloud Storage.
// To enable GCS uploads, add cloud.google.com/go/storage as a direct dependency
// and replace this implementation with the real GCS client.
type GCSDestination struct {
	bucket string
	prefix string
}

func newGCSDestination(cfg map[string]any) (*GCSDestination, error) {
	bucket, _ := cfg["bucket"].(string)
	if bucket == "" {
		return nil, fmt.Errorf("gcs destination requires 'bucket' in destination_config")
	}
	prefix, _ := cfg["prefix"].(string)
	return &GCSDestination{bucket: bucket, prefix: prefix}, nil
}

func (d *GCSDestination) Name() string { return "gcs" }

// Upload returns an error explaining that the GCS SDK is not directly included.
func (d *GCSDestination) Upload(_ context.Context, _ string, _ string) error {
	return fmt.Errorf("GCS destination is not yet available: add cloud.google.com/go/storage as a direct dependency to enable GCS uploads")
}

// ─────────────────────────────────────────────────────────────────────────────
// AzureBlobDestination — stub (Azure Blob Storage SDK not in go.mod as direct dep)
// ─────────────────────────────────────────────────────────────────────────────

// AzureBlobDestination is a stub implementation for Azure Blob Storage.
// To enable Azure Blob uploads, add github.com/Azure/azure-sdk-for-go/sdk/storage/azblob
// as a direct dependency and replace this implementation.
type AzureBlobDestination struct {
	container string
	prefix    string
}

func newAzureBlobDestination(cfg map[string]any) (*AzureBlobDestination, error) {
	container, _ := cfg["container"].(string)
	if container == "" {
		return nil, fmt.Errorf("azure_blob destination requires 'container' in destination_config")
	}
	prefix, _ := cfg["prefix"].(string)
	return &AzureBlobDestination{container: container, prefix: prefix}, nil
}

func (d *AzureBlobDestination) Name() string { return "azure_blob" }

// Upload returns an error explaining that the Azure Blob SDK is not directly included.
func (d *AzureBlobDestination) Upload(_ context.Context, _ string, _ string) error {
	return fmt.Errorf("Azure Blob destination is not yet available: add github.com/Azure/azure-sdk-for-go/sdk/storage/azblob as a direct dependency to enable Azure Blob uploads")
}
