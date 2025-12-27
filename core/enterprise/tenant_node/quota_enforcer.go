package tenant_node

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
)

// QuotaEnforcer enforces tenant quotas at the tenant node level
type QuotaEnforcer struct {
	manager *Manager

	// API request tracking (in-memory for performance)
	requestCounts   map[string]*RequestCounter // tenantID -> counter
	requestCountsMu sync.RWMutex

	// Storage tracking
	storageSizes   map[string]int64 // tenantID -> size in bytes
	storageSizesMu sync.RWMutex

	logger *log.Logger
}

// RequestCounter tracks API requests for a tenant
type RequestCounter struct {
	Count       int64
	WindowStart time.Time
	mu          sync.Mutex
}

// NewQuotaEnforcer creates a new quota enforcer
func NewQuotaEnforcer(manager *Manager) *QuotaEnforcer {
	return &QuotaEnforcer{
		manager:       manager,
		requestCounts: make(map[string]*RequestCounter),
		storageSizes:  make(map[string]int64),
		logger:        log.Default(),
	}
}

// Start starts background quota monitoring
func (qe *QuotaEnforcer) Start() {
	go qe.periodicStorageCheck()
	go qe.periodicRequestReset()
}

// CheckStorageQuota checks if tenant has exceeded storage quota
func (qe *QuotaEnforcer) CheckStorageQuota(tenantID string, tenant *enterprise.Tenant) error {
	// Get current storage size
	size, err := qe.getTenantStorageSize(tenantID)
	if err != nil {
		return fmt.Errorf("failed to check storage: %w", err)
	}

	sizeMB := size / (1024 * 1024)

	// Check against quota
	if sizeMB >= tenant.StorageQuotaMB {
		return enterprise.NewQuotaError("storage", sizeMB, tenant.StorageQuotaMB)
	}

	return nil
}

// CheckAPIQuota checks if tenant has exceeded API request quota
func (qe *QuotaEnforcer) CheckAPIQuota(tenantID string, tenant *enterprise.Tenant) error {
	qe.requestCountsMu.RLock()
	counter, exists := qe.requestCounts[tenantID]
	qe.requestCountsMu.RUnlock()

	if !exists {
		// No requests yet, create counter
		qe.requestCountsMu.Lock()
		qe.requestCounts[tenantID] = &RequestCounter{
			Count:       0,
			WindowStart: time.Now(),
		}
		qe.requestCountsMu.Unlock()
		return nil
	}

	counter.mu.Lock()
	currentCount := counter.Count
	counter.mu.Unlock()

	// Check against daily quota
	if currentCount >= tenant.APIRequestsQuota {
		return enterprise.NewQuotaError("api_requests", currentCount, tenant.APIRequestsQuota)
	}

	return nil
}

// RecordAPIRequest records an API request for quota tracking
func (qe *QuotaEnforcer) RecordAPIRequest(tenantID string) {
	qe.requestCountsMu.Lock()
	counter, exists := qe.requestCounts[tenantID]
	if !exists {
		counter = &RequestCounter{
			Count:       0,
			WindowStart: time.Now(),
		}
		qe.requestCounts[tenantID] = counter
	}
	qe.requestCountsMu.Unlock()

	counter.mu.Lock()
	counter.Count++
	counter.mu.Unlock()
}

// getTenantStorageSize calculates total storage size for a tenant
func (qe *QuotaEnforcer) getTenantStorageSize(tenantID string) (int64, error) {
	// Check cache first
	qe.storageSizesMu.RLock()
	if size, exists := qe.storageSizes[tenantID]; exists {
		qe.storageSizesMu.RUnlock()
		return size, nil
	}
	qe.storageSizesMu.RUnlock()

	// Calculate from disk
	tenantDir := filepath.Join(qe.manager.dataDir, tenantID)

	var totalSize int64
	err := filepath.Walk(tenantDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	// Cache the result
	qe.storageSizesMu.Lock()
	qe.storageSizes[tenantID] = totalSize
	qe.storageSizesMu.Unlock()

	return totalSize, nil
}

// periodicStorageCheck periodically checks and updates storage sizes
func (qe *QuotaEnforcer) periodicStorageCheck() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Get all active tenants
		instances := qe.manager.ListActiveTenants()

		for _, instance := range instances {
			// Recalculate storage size
			size, err := qe.getTenantStorageSize(instance.Tenant.ID)
			if err != nil {
				qe.logger.Printf("[QuotaEnforcer] Failed to check storage for tenant %s: %v",
					instance.Tenant.ID, err)
				continue
			}

			sizeMB := size / (1024 * 1024)

			// Update tenant usage in memory
			instance.Tenant.StorageUsedMB = sizeMB

			// Check if exceeded quota
			if sizeMB >= instance.Tenant.StorageQuotaMB {
				qe.logger.Printf("[QuotaEnforcer] Tenant %s exceeded storage quota: %d MB / %d MB",
					instance.Tenant.ID, sizeMB, instance.Tenant.StorageQuotaMB)
			}
		}
	}
}

// periodicRequestReset resets daily request counters
func (qe *QuotaEnforcer) periodicRequestReset() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		qe.requestCountsMu.Lock()

		now := time.Now()
		for tenantID, counter := range qe.requestCounts {
			counter.mu.Lock()

			// Reset if window is older than 24 hours
			if now.Sub(counter.WindowStart) > 24*time.Hour {
				counter.Count = 0
				counter.WindowStart = now
				qe.logger.Printf("[QuotaEnforcer] Reset request counter for tenant %s", tenantID)
			}

			counter.mu.Unlock()
		}

		qe.requestCountsMu.Unlock()
	}
}

// GetRequestCount returns the current request count for a tenant
func (qe *QuotaEnforcer) GetRequestCount(tenantID string) int64 {
	qe.requestCountsMu.RLock()
	counter, exists := qe.requestCounts[tenantID]
	qe.requestCountsMu.RUnlock()

	if !exists {
		return 0
	}

	counter.mu.Lock()
	count := counter.Count
	counter.mu.Unlock()

	return count
}

// GetStorageSize returns the current storage size for a tenant
func (qe *QuotaEnforcer) GetStorageSize(tenantID string) int64 {
	qe.storageSizesMu.RLock()
	size := qe.storageSizes[tenantID]
	qe.storageSizesMu.RUnlock()
	return size
}

// CleanupTenant removes quota data for an unloaded tenant to prevent memory leaks
func (qe *QuotaEnforcer) CleanupTenant(tenantID string) {
	// Remove request counter
	qe.requestCountsMu.Lock()
	delete(qe.requestCounts, tenantID)
	qe.requestCountsMu.Unlock()

	// Remove storage size cache
	qe.storageSizesMu.Lock()
	delete(qe.storageSizes, tenantID)
	qe.storageSizesMu.Unlock()

	qe.logger.Printf("[QuotaEnforcer] Cleaned up quota data for tenant: %s", tenantID)
}
