package badger

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	badger "github.com/dgraph-io/badger/v4"
)

// DiskManager handles disk space monitoring and management for BadgerDB
type DiskManager struct {
	db     *badger.DB
	config *DiskConfig
	logger *log.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	lastGCTime     time.Time
	lastCheckTime  time.Time
	diskUsageBytes int64
	mu             sync.RWMutex
}

// DiskConfig holds configuration for disk management
type DiskConfig struct {
	// Disk space thresholds
	MaxDiskUsageBytes   int64         // Maximum allowed disk usage (0 = unlimited)
	WarningThresholdPct float64       // Warning threshold percentage (default: 80%)
	CriticalThresholdPct float64      // Critical threshold percentage (default: 95%)

	// Garbage collection settings
	GCInterval          time.Duration // How often to run GC (default: 5 minutes)
	GCDiscardRatio      float64       // GC discard ratio (default: 0.5)

	// Compaction settings
	CompactionInterval  time.Duration // How often to compact (default: 1 hour)

	// Data retention
	RetentionPeriod     time.Duration // How long to keep old data (0 = forever)

	// Monitoring
	CheckInterval       time.Duration // How often to check disk usage (default: 1 minute)
}

// DefaultDiskConfig returns default disk management configuration
func DefaultDiskConfig() *DiskConfig {
	return &DiskConfig{
		MaxDiskUsageBytes:    100 * 1024 * 1024 * 1024, // 100 GB default (supports 1M+ tenants)
		WarningThresholdPct:  80.0,
		CriticalThresholdPct: 95.0,
		GCInterval:           5 * time.Minute,
		GCDiscardRatio:       0.5,
		CompactionInterval:   1 * time.Hour,
		RetentionPeriod:      0, // Keep everything by default
		CheckInterval:        1 * time.Minute,
	}
}

// NewDiskManager creates a new disk manager
func NewDiskManager(db *badger.DB, config *DiskConfig) *DiskManager {
	if config == nil {
		config = DefaultDiskConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &DiskManager{
		db:     db,
		config: config,
		logger: log.Default(),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the disk manager background tasks
func (dm *DiskManager) Start() {
	dm.logger.Printf("[DiskManager] Starting disk manager")
	dm.logger.Printf("[DiskManager] Max disk usage: %d bytes (%.2f GB)",
		dm.config.MaxDiskUsageBytes,
		float64(dm.config.MaxDiskUsageBytes)/(1024*1024*1024))

	// Start background tasks
	dm.wg.Add(3)
	go dm.runGarbageCollection()
	go dm.runDiskMonitoring()
	go dm.runCompaction()
}

// Stop stops the disk manager
func (dm *DiskManager) Stop() {
	dm.logger.Printf("[DiskManager] Stopping disk manager")
	dm.cancel()
	dm.wg.Wait()
}

// runGarbageCollection periodically runs BadgerDB garbage collection
func (dm *DiskManager) runGarbageCollection() {
	defer dm.wg.Done()

	ticker := time.NewTicker(dm.config.GCInterval)
	defer ticker.Stop()

	for {
		select {
		case <-dm.ctx.Done():
			return
		case <-ticker.C:
			dm.runGC()
		}
	}
}

// runGC executes garbage collection
func (dm *DiskManager) runGC() {
	start := time.Now()
	dm.logger.Printf("[DiskManager] Starting garbage collection")

	// Run GC until no more rewriting is possible
	var err error
	gcCount := 0
	for {
		err = dm.db.RunValueLogGC(dm.config.GCDiscardRatio)
		if err != nil {
			break
		}
		gcCount++
	}

	// ErrNoRewrite is expected when GC is complete
	if err != nil && err != badger.ErrNoRewrite {
		dm.logger.Printf("[DiskManager] GC error: %v", err)
	} else {
		dm.logger.Printf("[DiskManager] GC completed in %v (%d rewrites)", time.Since(start), gcCount)
	}

	dm.mu.Lock()
	dm.lastGCTime = time.Now()
	dm.mu.Unlock()
}

// runDiskMonitoring monitors disk usage
func (dm *DiskManager) runDiskMonitoring() {
	defer dm.wg.Done()

	ticker := time.NewTicker(dm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-dm.ctx.Done():
			return
		case <-ticker.C:
			dm.checkDiskUsage()
		}
	}
}

// checkDiskUsage checks current disk usage and triggers alerts
func (dm *DiskManager) checkDiskUsage() {
	lsm, vlog := dm.db.Size()
	totalSize := lsm + vlog

	dm.mu.Lock()
	dm.diskUsageBytes = totalSize
	dm.lastCheckTime = time.Now()
	dm.mu.Unlock()

	// Calculate usage percentage
	if dm.config.MaxDiskUsageBytes > 0 {
		usagePct := (float64(totalSize) / float64(dm.config.MaxDiskUsageBytes)) * 100

		if usagePct >= dm.config.CriticalThresholdPct {
			dm.logger.Printf("[DiskManager] CRITICAL: Disk usage at %.2f%% (%d bytes / %d bytes)",
				usagePct, totalSize, dm.config.MaxDiskUsageBytes)
			dm.handleCriticalDiskUsage()
		} else if usagePct >= dm.config.WarningThresholdPct {
			dm.logger.Printf("[DiskManager] WARNING: Disk usage at %.2f%% (%d bytes / %d bytes)",
				usagePct, totalSize, dm.config.MaxDiskUsageBytes)
		}
	}

	// Log disk usage periodically
	dm.logger.Printf("[DiskManager] Disk usage: LSM=%d bytes, VLog=%d bytes, Total=%d bytes",
		lsm, vlog, totalSize)
}

// handleCriticalDiskUsage handles critical disk usage scenarios
func (dm *DiskManager) handleCriticalDiskUsage() {
	dm.logger.Printf("[DiskManager] Attempting emergency cleanup")

	// Run aggressive GC
	dm.logger.Printf("[DiskManager] Running emergency GC")
	dm.runGC()

	// Force compaction
	dm.logger.Printf("[DiskManager] Running emergency compaction")
	dm.runCompactionNow()

	// Re-check usage
	lsm, vlog := dm.db.Size()
	totalSize := lsm + vlog

	if dm.config.MaxDiskUsageBytes > 0 {
		usagePct := (float64(totalSize) / float64(dm.config.MaxDiskUsageBytes)) * 100

		if usagePct >= dm.config.CriticalThresholdPct {
			dm.logger.Printf("[DiskManager] ALERT: Disk usage still critical after cleanup: %.2f%%", usagePct)
			// In production, this should trigger alerts (PagerDuty, Slack, etc.)
		} else {
			dm.logger.Printf("[DiskManager] Emergency cleanup successful. Usage now at %.2f%%", usagePct)
		}
	}
}

// runCompaction periodically triggers database compaction
func (dm *DiskManager) runCompaction() {
	defer dm.wg.Done()

	ticker := time.NewTicker(dm.config.CompactionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-dm.ctx.Done():
			return
		case <-ticker.C:
			dm.runCompactionNow()
		}
	}
}

// runCompactionNow triggers immediate compaction
func (dm *DiskManager) runCompactionNow() {
	start := time.Now()
	dm.logger.Printf("[DiskManager] Starting database compaction")

	// Flatten compacts the LSM tree
	if err := dm.db.Flatten(4); err != nil {
		dm.logger.Printf("[DiskManager] Compaction error: %v", err)
	} else {
		dm.logger.Printf("[DiskManager] Compaction completed in %v", time.Since(start))
	}
}

// GetDiskUsage returns current disk usage statistics
func (dm *DiskManager) GetDiskUsage() (int64, float64) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	usagePct := float64(0)
	if dm.config.MaxDiskUsageBytes > 0 {
		usagePct = (float64(dm.diskUsageBytes) / float64(dm.config.MaxDiskUsageBytes)) * 100
	}

	return dm.diskUsageBytes, usagePct
}

// GetStats returns disk manager statistics
func (dm *DiskManager) GetStats() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	lsm, vlog := dm.db.Size()

	stats := map[string]interface{}{
		"diskUsageBytes":     dm.diskUsageBytes,
		"diskUsageGB":        float64(dm.diskUsageBytes) / (1024 * 1024 * 1024),
		"lsmSizeBytes":       lsm,
		"vlogSizeBytes":      vlog,
		"maxDiskUsageBytes":  dm.config.MaxDiskUsageBytes,
		"maxDiskUsageGB":     float64(dm.config.MaxDiskUsageBytes) / (1024 * 1024 * 1024),
		"lastGCTime":         dm.lastGCTime,
		"lastCheckTime":      dm.lastCheckTime,
	}

	if dm.config.MaxDiskUsageBytes > 0 {
		stats["usagePercent"] = (float64(dm.diskUsageBytes) / float64(dm.config.MaxDiskUsageBytes)) * 100
	}

	return stats
}

// GetDataDirSize returns the size of the data directory
func GetDataDirSize(path string) (int64, error) {
	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// EstimateDiskUsage estimates future disk usage based on growth rate
func (dm *DiskManager) EstimateDiskUsage(duration time.Duration) (int64, error) {
	// This is a simple linear extrapolation
	// In production, you'd use more sophisticated forecasting

	dm.mu.RLock()
	currentUsage := dm.diskUsageBytes
	dm.mu.RUnlock()

	// For now, return current usage
	// TODO: Implement growth rate tracking and forecasting
	return currentUsage, nil
}
