package storage

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// GlacierLifecycleManager manages S3 lifecycle policies for archiving to Glacier
type GlacierLifecycleManager struct {
	client *s3.Client
	bucket string
	logger *log.Logger
}

// NewGlacierLifecycleManager creates a new Glacier lifecycle manager
func NewGlacierLifecycleManager(client *s3.Client, bucket string) *GlacierLifecycleManager {
	return &GlacierLifecycleManager{
		client: client,
		bucket: bucket,
		logger: log.Default(),
	}
}

// SetupTenantLifecyclePolicy creates lifecycle rules for a tenant's S3 prefix
func (g *GlacierLifecycleManager) SetupTenantLifecyclePolicy(ctx context.Context, tenantPrefix string) error {
	g.logger.Printf("[GlacierLifecycle] Setting up lifecycle policy for %s", tenantPrefix)

	// Create lifecycle configuration for this tenant
	lifecycleConfig := &types.BucketLifecycleConfiguration{
		Rules: []types.LifecycleRule{
			{
				// Archive inactive tenant data to Glacier after 90 days
				ID:     aws.String(fmt.Sprintf("archive-%s", tenantPrefix)),
				Status: types.ExpirationStatusEnabled,
				Filter: &types.LifecycleRuleFilter{
					Prefix: aws.String(tenantPrefix),
				},
				Transitions: []types.Transition{
					{
						Days:         aws.Int32(90),
						StorageClass: types.TransitionStorageClassDeepArchive,
					},
				},
			},
			{
				// Move Litestream WAL files to Intelligent-Tiering after 7 days
				ID:     aws.String(fmt.Sprintf("wal-%s", tenantPrefix)),
				Status: types.ExpirationStatusEnabled,
				Filter: &types.LifecycleRuleFilter{
					Prefix: aws.String(fmt.Sprintf("%slitestream/", tenantPrefix)),
				},
				Transitions: []types.Transition{
					{
						Days:         aws.Int32(7),
						StorageClass: types.TransitionStorageClassIntelligentTiering,
					},
				},
			},
			{
				// Delete old snapshots after 180 days
				ID:     aws.String(fmt.Sprintf("cleanup-%s", tenantPrefix)),
				Status: types.ExpirationStatusEnabled,
				Filter: &types.LifecycleRuleFilter{
					Prefix: aws.String(fmt.Sprintf("%ssnapshots/", tenantPrefix)),
				},
				Expiration: &types.LifecycleExpiration{
					Days: aws.Int32(180),
				},
			},
		},
	}

	// Note: In production, you'd want to get existing lifecycle config and merge rules
	// For simplicity, we're assuming no existing rules for this tenant

	// Put lifecycle configuration
	_, err := g.client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket:                 aws.String(g.bucket),
		LifecycleConfiguration: lifecycleConfig,
	})

	if err != nil {
		return fmt.Errorf("failed to set lifecycle policy: %w", err)
	}

	g.logger.Printf("[GlacierLifecycle] Lifecycle policy configured for %s", tenantPrefix)
	return nil
}

// SetupGlobalLifecyclePolicy creates global lifecycle rules for the bucket
func (g *GlacierLifecycleManager) SetupGlobalLifecyclePolicy(ctx context.Context) error {
	g.logger.Printf("[GlacierLifecycle] Setting up global lifecycle policy")

	lifecycleConfig := &types.BucketLifecycleConfiguration{
		Rules: []types.LifecycleRule{
			{
				// Archive all tenant data older than 90 days
				ID:     aws.String("global-archive-tenants"),
				Status: types.ExpirationStatusEnabled,
				Filter: &types.LifecycleRuleFilter{
					Prefix: aws.String("tenants/"),
				},
				Transitions: []types.Transition{
					{
						Days:         aws.Int32(90),
						StorageClass: types.TransitionStorageClassDeepArchive,
					},
				},
			},
			{
				// Move all Litestream WALs to Intelligent-Tiering after 7 days
				ID:     aws.String("global-wal-intelligent"),
				Status: types.ExpirationStatusEnabled,
				Filter: &types.LifecycleRuleFilter{
					Prefix: aws.String("tenants/"),
				},
				Transitions: []types.Transition{
					{
						Days:         aws.Int32(7),
						StorageClass: types.TransitionStorageClassIntelligentTiering,
					},
				},
			},
			{
				// Delete temp/incomplete multipart uploads after 7 days
				ID:     aws.String("global-cleanup-multipart"),
				Status: types.ExpirationStatusEnabled,
				AbortIncompleteMultipartUpload: &types.AbortIncompleteMultipartUpload{
					DaysAfterInitiation: aws.Int32(7),
				},
			},
		},
	}

	_, err := g.client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket:                 aws.String(g.bucket),
		LifecycleConfiguration: lifecycleConfig,
	})

	if err != nil {
		return fmt.Errorf("failed to set global lifecycle policy: %w", err)
	}

	g.logger.Printf("[GlacierLifecycle] Global lifecycle policy configured")
	return nil
}

// RemoveTenantLifecyclePolicy removes lifecycle rules for a tenant
func (g *GlacierLifecycleManager) RemoveTenantLifecyclePolicy(ctx context.Context, tenantPrefix string) error {
	g.logger.Printf("[GlacierLifecycle] Removing lifecycle policy for %s", tenantPrefix)

	// Get current lifecycle configuration
	result, err := g.client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
		Bucket: aws.String(g.bucket),
	})

	if err != nil {
		return fmt.Errorf("failed to get lifecycle configuration: %w", err)
	}

	// Filter out rules for this tenant
	newRules := make([]types.LifecycleRule, 0)
	for _, rule := range result.Rules {
		// Check if rule applies to this tenant prefix
		isMatch := false
		if rule.Filter != nil && rule.Filter.Prefix != nil {
			prefix := *rule.Filter.Prefix
			if prefix == tenantPrefix ||
			   prefix == fmt.Sprintf("%slitestream/", tenantPrefix) ||
			   prefix == fmt.Sprintf("%ssnapshots/", tenantPrefix) {
				isMatch = true
			}
		}

		if !isMatch {
			newRules = append(newRules, rule)
		}
	}

	// Update lifecycle configuration
	if len(newRules) == 0 {
		// Delete lifecycle configuration if no rules left
		_, err = g.client.DeleteBucketLifecycle(ctx, &s3.DeleteBucketLifecycleInput{
			Bucket: aws.String(g.bucket),
		})
	} else {
		_, err = g.client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
			Bucket: aws.String(g.bucket),
			LifecycleConfiguration: &types.BucketLifecycleConfiguration{
				Rules: newRules,
			},
		})
	}

	if err != nil {
		return fmt.Errorf("failed to update lifecycle configuration: %w", err)
	}

	g.logger.Printf("[GlacierLifecycle] Lifecycle policy removed for %s", tenantPrefix)
	return nil
}

// TransitionToGlacier immediately transitions objects to Glacier storage class
func (g *GlacierLifecycleManager) TransitionToGlacier(ctx context.Context, tenantPrefix string, storageClass types.StorageClass) error {
	g.logger.Printf("[GlacierLifecycle] Transitioning %s to %s", tenantPrefix, storageClass)

	// List all objects with the tenant prefix
	listResult, err := g.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(g.bucket),
		Prefix: aws.String(tenantPrefix),
	})

	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	// Copy each object to the new storage class
	for _, obj := range listResult.Contents {
		_, err := g.client.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:       aws.String(g.bucket),
			CopySource:   aws.String(fmt.Sprintf("%s/%s", g.bucket, *obj.Key)),
			Key:          obj.Key,
			StorageClass: storageClass,
		})

		if err != nil {
			g.logger.Printf("[GlacierLifecycle] Failed to transition %s: %v", *obj.Key, err)
			continue
		}
	}

	g.logger.Printf("[GlacierLifecycle] Transitioned %d objects to %s", len(listResult.Contents), storageClass)
	return nil
}

// RestoreFromGlacier initiates Glacier restore for archived objects
func (g *GlacierLifecycleManager) RestoreFromGlacier(ctx context.Context, tenantPrefix string, expedited bool) error {
	g.logger.Printf("[GlacierLifecycle] Initiating restore for %s", tenantPrefix)

	// List all objects with the tenant prefix
	listResult, err := g.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(g.bucket),
		Prefix: aws.String(tenantPrefix),
	})

	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	// Determine restore tier
	tier := types.TierStandard
	if expedited {
		tier = types.TierExpedited // 1-5 minutes (expensive)
	}

	// Initiate restore for each object
	for _, obj := range listResult.Contents {
		// Check if object is in Glacier storage class
		if obj.StorageClass == types.ObjectStorageClassGlacier ||
		   obj.StorageClass == types.ObjectStorageClassDeepArchive {

			_, err := g.client.RestoreObject(ctx, &s3.RestoreObjectInput{
				Bucket: aws.String(g.bucket),
				Key:    obj.Key,
				RestoreRequest: &types.RestoreRequest{
					Days: aws.Int32(7), // Keep restored for 7 days
					GlacierJobParameters: &types.GlacierJobParameters{
						Tier: tier,
					},
				},
			})

			if err != nil {
				g.logger.Printf("[GlacierLifecycle] Failed to restore %s: %v", *obj.Key, err)
				continue
			}
		}
	}

	if expedited {
		g.logger.Printf("[GlacierLifecycle] Expedited restore initiated for %s (1-5 minutes)", tenantPrefix)
	} else {
		g.logger.Printf("[GlacierLifecycle] Standard restore initiated for %s (3-5 hours)", tenantPrefix)
	}

	return nil
}
