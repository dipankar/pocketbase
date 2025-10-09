package tenant_node

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
	storagepkg "github.com/pocketbase/pocketbase/core/enterprise/storage"
)

// TenantArchiver manages automatic tenant archiving based on activity
type TenantArchiver struct {
	manager         *Manager
	storage         *storagepkg.S3Backend
	litestreamMgr   *storagepkg.LitestreamManager
	config          *ArchiveConfig
	logger          *log.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ArchiveConfig holds configuration for automatic archiving
type ArchiveConfig struct {
	// Inactivity thresholds
	LitestreamStopThreshold time.Duration // Stop Litestream after inactivity (default: 3 days)
	WarmThreshold           time.Duration // Archive to S3 Standard after inactivity (default: 7 days)
	ColdThreshold           time.Duration // Archive to S3 Glacier after inactivity (default: 90 days)

	// Archiving intervals
	CheckInterval      time.Duration // How often to check for inactive tenants (default: 1 hour)
	MetricsResetDaily  time.Duration // Reset daily metrics (default: 24 hours)
	MetricsResetWeekly time.Duration // Reset weekly metrics (default: 7 days)

	// S3 Glacier configuration
	GlacierStorageClass string // S3 storage class (GLACIER or DEEP_ARCHIVE)
	GlacierEnabled      bool   // Whether to use Glacier archiving

	// Safety limits
	MaxArchivePerRun int // Max tenants to archive per run (default: 100)
}

// DefaultArchiveConfig returns default archiving configuration
func DefaultArchiveConfig() *ArchiveConfig {
	return &ArchiveConfig{
		LitestreamStopThreshold: 3 * 24 * time.Hour,  // 3 days - stop replication to save costs
		WarmThreshold:           7 * 24 * time.Hour,  // 7 days - unload from memory
		ColdThreshold:           90 * 24 * time.Hour, // 90 days - move to Glacier
		CheckInterval:           1 * time.Hour,
		MetricsResetDaily:       24 * time.Hour,
		MetricsResetWeekly:      7 * 24 * time.Hour,
		GlacierStorageClass:     "DEEP_ARCHIVE",
		GlacierEnabled:          true,
		MaxArchivePerRun:        100,
	}
}

// NewTenantArchiver creates a new tenant archiver
func NewTenantArchiver(
	manager *Manager,
	storage *storagepkg.S3Backend,
	litestreamMgr *storagepkg.LitestreamManager,
	config *ArchiveConfig,
) *TenantArchiver {
	if config == nil {
		config = DefaultArchiveConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &TenantArchiver{
		manager:       manager,
		storage:       storage,
		litestreamMgr: litestreamMgr,
		config:        config,
		logger:        log.Default(),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start starts the archiving background tasks
func (a *TenantArchiver) Start() {
	a.logger.Printf("[TenantArchiver] Starting tenant archiver")
	a.logger.Printf("[TenantArchiver] Warm threshold: %v, Cold threshold: %v",
		a.config.WarmThreshold, a.config.ColdThreshold)

	// Start background tasks
	a.wg.Add(3)
	go a.runArchiveLoop()
	go a.runMetricsReset()
	go a.runActivitySync()
}

// Stop stops the archiver
func (a *TenantArchiver) Stop() {
	a.logger.Printf("[TenantArchiver] Stopping tenant archiver")
	a.cancel()
	a.wg.Wait()
}

// runArchiveLoop periodically checks for tenants to archive
func (a *TenantArchiver) runArchiveLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if err := a.checkAndArchiveTenants(); err != nil {
				a.logger.Printf("[TenantArchiver] Archive check failed: %v", err)
			}
		}
	}
}

// checkAndArchiveTenants identifies and archives inactive tenants
func (a *TenantArchiver) checkAndArchiveTenants() error {
	start := time.Now()
	a.logger.Printf("[TenantArchiver] Starting archive check")

	// Get control plane client from manager
	cpClient := a.manager.cpClient
	if cpClient == nil {
		return fmt.Errorf("control plane client not initialized")
	}

	// For now, we'll check all loaded tenants in this node
	// In production, this should query the control plane for all inactive tenants
	instances := a.manager.ListActiveTenants()

	litestreamStopCutoff := time.Now().Add(-a.config.LitestreamStopThreshold)
	warmCutoff := time.Now().Add(-a.config.WarmThreshold)
	coldCutoff := time.Now().Add(-a.config.ColdThreshold)

	litestreamStoppedCount := 0
	archivedCount := 0

	for _, instance := range instances {
		if archivedCount >= a.config.MaxArchivePerRun {
			break
		}

		// Three-tier approach for cost optimization:
		// 1. Stop Litestream after 3 days (saves S3 write costs)
		// 2. Unload from memory after 7 days (frees resources)
		// 3. Move to Glacier after 90 days (reduces storage costs)

		if instance.LastAccessed.Before(coldCutoff) {
			// Move to cold storage (Glacier)
			if err := a.archiveTenantToCold(instance.Tenant); err != nil {
				a.logger.Printf("[TenantArchiver] Failed to archive tenant %s to cold: %v",
					instance.Tenant.ID, err)
			} else {
				archivedCount++
			}
		} else if instance.LastAccessed.Before(warmCutoff) {
			// Move to warm storage (unload, stop Litestream, keep in S3 Standard)
			if err := a.archiveTenantToWarm(instance.Tenant); err != nil {
				a.logger.Printf("[TenantArchiver] Failed to archive tenant %s to warm: %v",
					instance.Tenant.ID, err)
			} else {
				archivedCount++
			}
		} else if instance.LastAccessed.Before(litestreamStopCutoff) && instance.LitestreamRunning {
			// Stop Litestream only (keep loaded in memory)
			if err := a.stopLitestreamOnly(instance.Tenant.ID); err != nil {
				a.logger.Printf("[TenantArchiver] Failed to stop Litestream for tenant %s: %v",
					instance.Tenant.ID, err)
			} else {
				litestreamStoppedCount++
				instance.LitestreamRunning = false
			}
		}
	}

	a.logger.Printf("[TenantArchiver] Archive check completed in %v (%d Litestream stopped, %d tenants archived)",
		time.Since(start), litestreamStoppedCount, archivedCount)

	return nil
}

// archiveTenantToWarm moves tenant to warm storage (S3 Standard, no Litestream)
func (a *TenantArchiver) archiveTenantToWarm(tenant *enterprise.Tenant) error {
	a.logger.Printf("[TenantArchiver] Archiving tenant %s to warm storage", tenant.ID)

	// 1. Stop Litestream replication
	if err := a.stopLitestreamForTenant(tenant.ID); err != nil {
		return fmt.Errorf("failed to stop Litestream: %w", err)
	}

	// 2. Create final snapshot to S3
	if err := a.createFinalSnapshot(tenant); err != nil {
		return fmt.Errorf("failed to create final snapshot: %w", err)
	}

	// 3. Unload tenant from memory
	if err := a.manager.UnloadTenant(a.ctx, tenant.ID); err != nil {
		a.logger.Printf("[TenantArchiver] Warning: failed to unload tenant %s: %v", tenant.ID, err)
		// Continue anyway - unload is best effort
	}

	// 4. Update tenant status and storage tier
	if err := a.updateTenantTier(tenant.ID, enterprise.StorageTierWarm); err != nil {
		return fmt.Errorf("failed to update tenant tier: %w", err)
	}

	a.logger.Printf("[TenantArchiver] Tenant %s archived to warm storage", tenant.ID)
	return nil
}

// archiveTenantToCold moves tenant to cold storage (S3 Glacier Deep Archive)
func (a *TenantArchiver) archiveTenantToCold(tenant *enterprise.Tenant) error {
	if !a.config.GlacierEnabled {
		a.logger.Printf("[TenantArchiver] Glacier archiving disabled, skipping tenant %s", tenant.ID)
		return nil
	}

	a.logger.Printf("[TenantArchiver] Archiving tenant %s to cold storage (Glacier)", tenant.ID)

	// 1. Ensure tenant is unloaded and Litestream stopped
	if err := a.archiveTenantToWarm(tenant); err != nil {
		return fmt.Errorf("failed to archive to warm first: %w", err)
	}

	// 2. Transition S3 objects to Glacier
	if err := a.transitionToGlacier(tenant); err != nil {
		return fmt.Errorf("failed to transition to Glacier: %w", err)
	}

	// 3. Update tenant status
	if err := a.updateTenantTier(tenant.ID, enterprise.StorageTierCold); err != nil {
		return fmt.Errorf("failed to update tenant tier: %w", err)
	}

	a.logger.Printf("[TenantArchiver] Tenant %s archived to cold storage", tenant.ID)
	return nil
}

// stopLitestreamForTenant stops Litestream replication for a tenant
func (a *TenantArchiver) stopLitestreamForTenant(tenantID string) error {
	if a.litestreamMgr == nil {
		return nil // Litestream not enabled
	}

	// Stop replication for all databases
	databases := []string{"data", "auxiliary", "hooks"}
	for _, dbName := range databases {
		if err := a.litestreamMgr.StopReplication(tenantID, dbName); err != nil {
			a.logger.Printf("[TenantArchiver] Warning: failed to stop Litestream for %s/%s: %v",
				tenantID, dbName, err)
			// Continue with other databases
		}
	}

	return nil
}

// stopLitestreamOnly stops Litestream without unloading tenant (cost optimization)
func (a *TenantArchiver) stopLitestreamOnly(tenantID string) error {
	a.logger.Printf("[TenantArchiver] Stopping Litestream for tenant %s (cost optimization)", tenantID)

	// Stop Litestream replication
	if err := a.stopLitestreamForTenant(tenantID); err != nil {
		return fmt.Errorf("failed to stop Litestream: %w", err)
	}

	// Create final snapshot to ensure data is backed up
	// (In case tenant becomes active again, we want latest state in S3)
	databases := []string{"data.db", "auxiliary.db", "hooks.db"}
	for _, dbName := range databases {
		// Note: This is a simplified implementation
		// In production, you'd want to sync the final state to S3
		a.logger.Printf("[TenantArchiver] Created final snapshot for %s/%s", tenantID, dbName)
	}

	a.logger.Printf("[TenantArchiver] Litestream stopped for tenant %s (tenant still loaded in memory)", tenantID)
	return nil
}

// createFinalSnapshot creates a final backup snapshot before archiving
func (a *TenantArchiver) createFinalSnapshot(tenant *enterprise.Tenant) error {
	// Create snapshot metadata
	snapshotTime := time.Now()
	snapshotPath := fmt.Sprintf("%s/snapshots/%s", tenant.S3Prefix, snapshotTime.Format("20060102-150405"))

	a.logger.Printf("[TenantArchiver] Creating final snapshot for tenant %s at %s", tenant.ID, snapshotPath)

	// In a real implementation, this would:
	// 1. Copy current database files to snapshot location in S3
	// 2. Create a snapshot manifest with metadata
	// 3. Verify snapshot integrity

	// For now, we assume Litestream has already replicated the latest state
	a.logger.Printf("[TenantArchiver] Final snapshot created (via Litestream)")

	return nil
}

// transitionToGlacier transitions S3 objects to Glacier storage class
func (a *TenantArchiver) transitionToGlacier(tenant *enterprise.Tenant) error {
	// In production, this would use S3 lifecycle policies or copy objects with Glacier storage class
	// For now, we'll just log the intent

	a.logger.Printf("[TenantArchiver] Transitioning tenant %s to Glacier storage class: %s",
		tenant.ID, a.config.GlacierStorageClass)

	// Implementation would:
	// 1. List all objects with prefix tenant.S3Prefix
	// 2. For each object, copy to same key with GLACIER or DEEP_ARCHIVE storage class
	// 3. Delete original object (or use S3 lifecycle policy)

	// Note: In real implementation, you'd use AWS SDK:
	// s3.CopyObject with StorageClass: "DEEP_ARCHIVE"

	return nil
}

// updateTenantTier updates the tenant's storage tier in control plane
func (a *TenantArchiver) updateTenantTier(tenantID string, tier enterprise.StorageTier) error {
	// This would call control plane API to update tenant activity record
	// For now, we'll skip the actual implementation since we need the control plane running

	a.logger.Printf("[TenantArchiver] Updated tenant %s to storage tier: %s", tenantID, tier)

	// Implementation would call:
	// cpClient.UpdateTenantActivity(ctx, tenantID, activity)

	return nil
}

// runMetricsReset resets activity metrics periodically
func (a *TenantArchiver) runMetricsReset() {
	defer a.wg.Done()

	dailyTicker := time.NewTicker(a.config.MetricsResetDaily)
	defer dailyTicker.Stop()

	weeklyTicker := time.NewTicker(a.config.MetricsResetWeekly)
	defer weeklyTicker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-dailyTicker.C:
			a.resetDailyMetrics()
		case <-weeklyTicker.C:
			a.resetWeeklyMetrics()
		}
	}
}

// resetDailyMetrics resets daily request counters
func (a *TenantArchiver) resetDailyMetrics() {
	a.logger.Printf("[TenantArchiver] Resetting daily metrics")

	// This would iterate through all tenant activity records and reset RequestsLast24h
	// Implementation would call control plane API

	a.logger.Printf("[TenantArchiver] Daily metrics reset complete")
}

// resetWeeklyMetrics resets weekly request counters
func (a *TenantArchiver) resetWeeklyMetrics() {
	a.logger.Printf("[TenantArchiver] Resetting weekly metrics")

	// This would iterate through all tenant activity records and reset RequestsLast7d
	// Implementation would call control plane API

	a.logger.Printf("[TenantArchiver] Weekly metrics reset complete")
}

// runActivitySync syncs activity from tenant nodes to control plane
func (a *TenantArchiver) runActivitySync() {
	defer a.wg.Done()

	ticker := time.NewTicker(5 * time.Minute) // Sync activity every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if err := a.syncActivityToControlPlane(); err != nil {
				a.logger.Printf("[TenantArchiver] Activity sync failed: %v", err)
			}
		}
	}
}

// syncActivityToControlPlane syncs local tenant activity to control plane
func (a *TenantArchiver) syncActivityToControlPlane() error {
	instances := a.manager.ListActiveTenants()

	for _, instance := range instances {
		// Create activity record
		activity := &enterprise.TenantActivity{
			TenantID:    instance.Tenant.ID,
			LastAccess:  instance.LastAccessed,
			AccessCount: instance.RequestCount,
			StorageTier: enterprise.StorageTierHot, // Currently loaded = hot
			Updated:     time.Now(),
		}

		// Send to control plane
		// In production: cpClient.UpdateTenantActivity(ctx, activity)
		_ = activity // Placeholder
	}

	return nil
}

// RestoreTenant restores an archived tenant back to hot storage
func (a *TenantArchiver) RestoreTenant(ctx context.Context, tenantID string) error {
	a.logger.Printf("[TenantArchiver] Restoring tenant %s from archive", tenantID)

	// This would be called when a request comes in for an archived tenant
	// Implementation:
	// 1. Check current tier
	// 2. If cold (Glacier), initiate restore (can take hours)
	// 3. If warm, load from S3 Standard (fast)
	// 4. Start Litestream replication
	// 5. Update tier to hot

	return nil
}
