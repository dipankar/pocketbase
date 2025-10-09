package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pocketbase/pocketbase/core/enterprise"
)

// S3Backend implements the enterprise.StorageBackend interface using AWS S3
type S3Backend struct {
	client *s3.Client
	bucket string
}

// NewS3Backend creates a new S3 storage backend
func NewS3Backend(ctx context.Context, endpoint, region, bucket, accessKeyID, secretAccessKey string) (*S3Backend, error) {
	var cfg aws.Config
	var err error

	if accessKeyID != "" && secretAccessKey != "" {
		// Use explicit credentials
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				accessKeyID,
				secretAccessKey,
				"",
			)),
		)
	} else {
		// Use default credential chain (IAM role, env vars, etc.)
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	var client *s3.Client
	if endpoint != "" {
		// Custom endpoint (e.g., MinIO, LocalStack)
		client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		})
	} else {
		client = s3.NewFromConfig(cfg)
	}

	return &S3Backend{
		client: client,
		bucket: bucket,
	}, nil
}

// DownloadTenantDB downloads a tenant database from S3
func (s *S3Backend) DownloadTenantDB(ctx context.Context, tenant *enterprise.Tenant, dbName string, destPath string) error {
	// S3 key: tenants/tenant_xxx/litestream/data.db/generations/[hash]/snapshots/[hash]/snapshot.db
	// For initial download, we'll get the latest snapshot
	// For now, we'll use a simplified key structure

	key := fmt.Sprintf("%s%s", tenant.S3Prefix, dbName)

	// Check if file exists in S3
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		// Database doesn't exist yet (new tenant)
		// Create an empty database file
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		file, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create empty db file: %w", err)
		}
		file.Close()

		return nil
	}

	// Download from S3
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer result.Body.Close()

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create destination file
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer file.Close()

	// Copy from S3 to file
	_, err = io.Copy(file, result.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// UploadTenantDB uploads a tenant database to S3
func (s *S3Backend) UploadTenantDB(ctx context.Context, tenant *enterprise.Tenant, dbName string, sourcePath string) error {
	key := fmt.Sprintf("%s%s", tenant.S3Prefix, dbName)

	// Open source file
	file, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer file.Close()

	// Upload to S3
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

// DeleteTenantData removes all tenant data from S3
func (s *S3Backend) DeleteTenantData(ctx context.Context, tenant *enterprise.Tenant) error {
	// List all objects with the tenant prefix
	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(tenant.S3Prefix),
	})
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	// Delete all objects
	for _, obj := range result.Contents {
		_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    obj.Key,
		})
		if err != nil {
			return fmt.Errorf("failed to delete object %s: %w", *obj.Key, err)
		}
	}

	return nil
}

// ListTenantBackups lists available backups for a tenant
func (s *S3Backend) ListTenantBackups(ctx context.Context, tenantID string) ([]string, error) {
	prefix := fmt.Sprintf("tenants/%s/backups/", tenantID)

	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	backups := make([]string, 0, len(result.Contents))
	for _, obj := range result.Contents {
		backups = append(backups, *obj.Key)
	}

	return backups, nil
}

// RestoreFromBackup restores tenant data from a specific backup
func (s *S3Backend) RestoreFromBackup(ctx context.Context, tenantID string, backupID string) error {
	// TODO: Implement backup restoration
	// This would involve copying backup files to the active tenant location
	return fmt.Errorf("backup restoration not yet implemented")
}
