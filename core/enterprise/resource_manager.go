package enterprise

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// TenantTier represents different resource allocation tiers
type TenantTier string

const (
	TenantTierMicro      TenantTier = "micro"      // < 10 MB, < 1k req/day
	TenantTierSmall      TenantTier = "small"      // < 100 MB, < 10k req/day
	TenantTierMedium     TenantTier = "medium"     // < 1 GB, < 100k req/day
	TenantTierLarge      TenantTier = "large"      // < 5 GB, < 1M req/day
	TenantTierEnterprise TenantTier = "enterprise" // Dedicated resources
)

// TenantResourceMetrics tracks actual resource usage
type TenantResourceMetrics struct {
	TenantID string
	Tier     TenantTier

	// Database metrics
	DatabaseSizeMB      int64
	DatabaseGrowthRate  float64 // MB/day
	WALSizeMB           int64
	QueryComplexity     float64 // 0-1 score
	AvgQueryTimeMs      float64

	// Traffic metrics
	RequestsLast24h     int64
	RequestsLast7d      int64
	PeakRequestsPerMin  int64
	AvgResponseTimeMs   float64
	ErrorRate           float64

	// Resource consumption
	MemoryUsageMB       int64
	CPUUsagePercent     float64
	DiskIOPS            int64
	NetworkMBPS         float64

	// Behavior classification
	IsHotspot           bool      // High resource usage
	IsSpiking           bool      // Sudden traffic increase
	LastSpike           time.Time
	HotspotScore        float64   // 0-1, higher = more resources needed

	Updated             time.Time
}

// ResourceQuota defines limits for each tier
type ResourceQuota struct {
	Tier                TenantTier
	MaxDatabaseMB       int64
	MaxRequestsDaily    int64
	MaxConcurrentConns  int
	MaxMemoryMB         int64
	MaxCPUPercent       float64
	MaxQueryTimeMs      int64
	PriorityClass       int // Higher = more priority in scheduling
}

// DefaultResourceQuotas returns standard quotas for each tier
var DefaultResourceQuotas = map[TenantTier]*ResourceQuota{
	TenantTierMicro: {
		Tier:               TenantTierMicro,
		MaxDatabaseMB:      10,
		MaxRequestsDaily:   1_000,
		MaxConcurrentConns: 5,
		MaxMemoryMB:        50,
		MaxCPUPercent:      5.0,
		MaxQueryTimeMs:     1000,
		PriorityClass:      1,
	},
	TenantTierSmall: {
		Tier:               TenantTierSmall,
		MaxDatabaseMB:      100,
		MaxRequestsDaily:   10_000,
		MaxConcurrentConns: 20,
		MaxMemoryMB:        200,
		MaxCPUPercent:      10.0,
		MaxQueryTimeMs:     5000,
		PriorityClass:      2,
	},
	TenantTierMedium: {
		Tier:               TenantTierMedium,
		MaxDatabaseMB:      1_000,
		MaxRequestsDaily:   100_000,
		MaxConcurrentConns: 50,
		MaxMemoryMB:        1_000,
		MaxCPUPercent:      25.0,
		MaxQueryTimeMs:     10000,
		PriorityClass:      3,
	},
	TenantTierLarge: {
		Tier:               TenantTierLarge,
		MaxDatabaseMB:      5_000,
		MaxRequestsDaily:   1_000_000,
		MaxConcurrentConns: 100,
		MaxMemoryMB:        4_000,
		MaxCPUPercent:      50.0,
		MaxQueryTimeMs:     30000,
		PriorityClass:      4,
	},
	TenantTierEnterprise: {
		Tier:               TenantTierEnterprise,
		MaxDatabaseMB:      50_000, // 50 GB
		MaxRequestsDaily:   10_000_000,
		MaxConcurrentConns: 500,
		MaxMemoryMB:        16_000,
		MaxCPUPercent:      100.0, // Dedicated node
		MaxQueryTimeMs:     60000,
		PriorityClass:      5,
	},
}

// ResourceManager monitors and manages tenant resource usage
type ResourceManager struct {
	mu      sync.RWMutex
	metrics map[string]*TenantResourceMetrics // tenantID -> metrics

	quotas map[TenantTier]*ResourceQuota

	// Callbacks for actions
	onHotspotDetected   func(tenantID string, metrics *TenantResourceMetrics)
	onTierUpgrade       func(tenantID string, oldTier, newTier TenantTier)
	onQuotaExceeded     func(tenantID string, quotaType string, current, limit int64)

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	logger *log.Logger
}

// NewResourceManager creates a new resource manager
func NewResourceManager() *ResourceManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &ResourceManager{
		metrics: make(map[string]*TenantResourceMetrics),
		quotas:  DefaultResourceQuotas,
		ctx:     ctx,
		cancel:  cancel,
		logger:  log.Default(),
	}
}

// Start begins monitoring
func (rm *ResourceManager) Start() {
	rm.logger.Printf("[ResourceManager] Starting resource monitoring")

	rm.wg.Add(2)
	go rm.monitorLoop()
	go rm.classifyLoop()
}

// Stop stops monitoring
func (rm *ResourceManager) Stop() {
	rm.logger.Printf("[ResourceManager] Stopping resource manager")
	rm.cancel()
	rm.wg.Wait()
}

// RecordMetrics updates metrics for a tenant
func (rm *ResourceManager) RecordMetrics(metrics *TenantResourceMetrics) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	metrics.Updated = time.Now()
	rm.metrics[metrics.TenantID] = metrics

	// Check for quota violations
	rm.checkQuotas(metrics)

	// Update tier if needed
	newTier := rm.calculateTier(metrics)
	if newTier != metrics.Tier {
		rm.logger.Printf("[ResourceManager] Tenant %s tier change: %s -> %s",
			metrics.TenantID, metrics.Tier, newTier)

		if rm.onTierUpgrade != nil {
			rm.onTierUpgrade(metrics.TenantID, metrics.Tier, newTier)
		}

		metrics.Tier = newTier
	}
}

// GetMetrics retrieves metrics for a tenant
func (rm *ResourceManager) GetMetrics(tenantID string) *TenantResourceMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.metrics[tenantID]
}

// GetQuota returns quota for a tier
func (rm *ResourceManager) GetQuota(tier TenantTier) *ResourceQuota {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.quotas[tier]
}

// calculateTier determines appropriate tier based on usage
func (rm *ResourceManager) calculateTier(metrics *TenantResourceMetrics) TenantTier {
	// Check enterprise threshold
	if metrics.DatabaseSizeMB > 5000 || metrics.RequestsLast24h > 1_000_000 {
		return TenantTierEnterprise
	}

	// Check large threshold
	if metrics.DatabaseSizeMB > 1000 || metrics.RequestsLast24h > 100_000 {
		return TenantTierLarge
	}

	// Check medium threshold
	if metrics.DatabaseSizeMB > 100 || metrics.RequestsLast24h > 10_000 {
		return TenantTierMedium
	}

	// Check small threshold
	if metrics.DatabaseSizeMB > 10 || metrics.RequestsLast24h > 1_000 {
		return TenantTierSmall
	}

	return TenantTierMicro
}

// checkQuotas verifies tenant is within quota limits
func (rm *ResourceManager) checkQuotas(metrics *TenantResourceMetrics) {
	quota := rm.quotas[metrics.Tier]
	if quota == nil {
		return
	}

	// Check database size
	if metrics.DatabaseSizeMB > quota.MaxDatabaseMB {
		rm.logger.Printf("[ResourceManager] Tenant %s exceeded database quota: %d MB > %d MB",
			metrics.TenantID, metrics.DatabaseSizeMB, quota.MaxDatabaseMB)

		if rm.onQuotaExceeded != nil {
			rm.onQuotaExceeded(metrics.TenantID, "database_size", metrics.DatabaseSizeMB, quota.MaxDatabaseMB)
		}
	}

	// Check request quota
	if metrics.RequestsLast24h > quota.MaxRequestsDaily {
		rm.logger.Printf("[ResourceManager] Tenant %s exceeded request quota: %d > %d",
			metrics.TenantID, metrics.RequestsLast24h, quota.MaxRequestsDaily)

		if rm.onQuotaExceeded != nil {
			rm.onQuotaExceeded(metrics.TenantID, "daily_requests", metrics.RequestsLast24h, quota.MaxRequestsDaily)
		}
	}

	// Check CPU usage
	if metrics.CPUUsagePercent > quota.MaxCPUPercent {
		rm.logger.Printf("[ResourceManager] Tenant %s exceeded CPU quota: %.2f%% > %.2f%%",
			metrics.TenantID, metrics.CPUUsagePercent, quota.MaxCPUPercent)

		if rm.onQuotaExceeded != nil {
			rm.onQuotaExceeded(metrics.TenantID, "cpu_usage", int64(metrics.CPUUsagePercent), int64(quota.MaxCPUPercent))
		}
	}
}

// monitorLoop periodically checks for hotspots
func (rm *ResourceManager) monitorLoop() {
	defer rm.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.detectHotspots()
		}
	}
}

// detectHotspots identifies tenants using excessive resources
func (rm *ResourceManager) detectHotspots() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for tenantID, metrics := range rm.metrics {
		hotspotScore := rm.calculateHotspotScore(metrics)
		metrics.HotspotScore = hotspotScore

		wasHotspot := metrics.IsHotspot
		metrics.IsHotspot = hotspotScore > 0.7

		// Detect spikes (sudden increases)
		if metrics.RequestsLast24h > 0 {
			expectedRequests := metrics.RequestsLast7d / 7
			if expectedRequests > 0 {
				ratio := float64(metrics.RequestsLast24h) / float64(expectedRequests)
				metrics.IsSpiking = ratio > 3.0 // 3x normal traffic
				if metrics.IsSpiking {
					metrics.LastSpike = time.Now()
				}
			}
		}

		// Notify if newly detected hotspot
		if metrics.IsHotspot && !wasHotspot {
			rm.logger.Printf("[ResourceManager] Hotspot detected: %s (score: %.2f)", tenantID, hotspotScore)

			if rm.onHotspotDetected != nil {
				rm.onHotspotDetected(tenantID, metrics)
			}
		}
	}
}

// calculateHotspotScore returns 0-1 score for resource usage
func (rm *ResourceManager) calculateHotspotScore(metrics *TenantResourceMetrics) float64 {
	quota := rm.quotas[metrics.Tier]
	if quota == nil {
		return 0.0
	}

	// Weight different factors
	var score float64

	// Database size (25% weight)
	if quota.MaxDatabaseMB > 0 {
		score += 0.25 * (float64(metrics.DatabaseSizeMB) / float64(quota.MaxDatabaseMB))
	}

	// Request rate (25% weight)
	if quota.MaxRequestsDaily > 0 {
		score += 0.25 * (float64(metrics.RequestsLast24h) / float64(quota.MaxRequestsDaily))
	}

	// CPU usage (30% weight)
	if quota.MaxCPUPercent > 0 {
		score += 0.30 * (metrics.CPUUsagePercent / quota.MaxCPUPercent)
	}

	// Memory usage (20% weight)
	if quota.MaxMemoryMB > 0 {
		score += 0.20 * (float64(metrics.MemoryUsageMB) / float64(quota.MaxMemoryMB))
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// classifyLoop periodically classifies tenant behavior
func (rm *ResourceManager) classifyLoop() {
	defer rm.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.classifyTenants()
		}
	}
}

// classifyTenants analyzes usage patterns
func (rm *ResourceManager) classifyTenants() {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	hotspots := 0
	spikes := 0

	for _, metrics := range rm.metrics {
		if metrics.IsHotspot {
			hotspots++
		}
		if metrics.IsSpiking {
			spikes++
		}
	}

	rm.logger.Printf("[ResourceManager] Classification: %d tenants tracked, %d hotspots, %d spikes",
		len(rm.metrics), hotspots, spikes)
}

// SetCallbacks configures action callbacks
func (rm *ResourceManager) SetCallbacks(
	onHotspot func(string, *TenantResourceMetrics),
	onTierUpgrade func(string, TenantTier, TenantTier),
	onQuotaExceeded func(string, string, int64, int64),
) {
	rm.onHotspotDetected = onHotspot
	rm.onTierUpgrade = onTierUpgrade
	rm.onQuotaExceeded = onQuotaExceeded
}

// GetHotspots returns all current hotspot tenants
func (rm *ResourceManager) GetHotspots() []*TenantResourceMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var hotspots []*TenantResourceMetrics
	for _, metrics := range rm.metrics {
		if metrics.IsHotspot {
			hotspots = append(hotspots, metrics)
		}
	}

	return hotspots
}

// ShouldEvict determines if tenant should be evicted from cache
// based on resource usage and tier
func (rm *ResourceManager) ShouldEvict(tenantID string) (bool, string) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	metrics := rm.metrics[tenantID]
	if metrics == nil {
		return false, ""
	}

	// Never evict enterprise tier (they have dedicated resources)
	if metrics.Tier == TenantTierEnterprise {
		return false, ""
	}

	// Evict if significantly over quota
	quota := rm.quotas[metrics.Tier]
	if quota == nil {
		return false, ""
	}

	// Check if grossly over CPU quota (2x over limit)
	if metrics.CPUUsagePercent > quota.MaxCPUPercent*2 {
		return true, fmt.Sprintf("CPU usage %.2f%% exceeds 2x tier limit", metrics.CPUUsagePercent)
	}

	// Check if database is much larger than tier allows
	if metrics.DatabaseSizeMB > quota.MaxDatabaseMB*2 {
		return true, fmt.Sprintf("Database size %d MB exceeds 2x tier limit", metrics.DatabaseSizeMB)
	}

	return false, ""
}

// GetTenantWeight returns a weight for LRU cache (larger = uses more slots)
func (rm *ResourceManager) GetTenantWeight(tenantID string) int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	metrics := rm.metrics[tenantID]
	if metrics == nil {
		return 1 // Default weight
	}

	// Weight based on tier (larger tenants use more "slots" in cache)
	switch metrics.Tier {
	case TenantTierMicro:
		return 1
	case TenantTierSmall:
		return 2
	case TenantTierMedium:
		return 5
	case TenantTierLarge:
		return 10
	case TenantTierEnterprise:
		return 20
	default:
		return 1
	}
}
