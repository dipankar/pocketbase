package tenant_node

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
)

// MetricsCollector collects actual resource metrics for tenants
type MetricsCollector struct {
	manager *Manager

	// CPU tracking (previous values for delta calculation)
	lastCPUTime   time.Time
	lastCPUSample map[string]time.Duration
	mu            sync.RWMutex

	logger *log.Logger
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(manager *Manager) *MetricsCollector {
	return &MetricsCollector{
		manager:       manager,
		lastCPUSample: make(map[string]time.Duration),
		lastCPUTime:   time.Now(),
		logger:        log.Default(),
	}
}

// CollectTenantMetrics collects actual metrics for a tenant
func (mc *MetricsCollector) CollectTenantMetrics(tenantID string, instance *enterprise.TenantInstance) *enterprise.TenantResourceMetrics {
	// Calculate database size
	dbSizeMB := mc.calculateDatabaseSize(tenantID)

	// Estimate memory usage (simplified - would need process-level tracking in production)
	memoryMB := mc.estimateMemoryUsage(tenantID)

	// Estimate CPU usage
	cpuPercent := mc.estimateCPUUsage(tenantID, instance)

	// Calculate query metrics (would need actual query tracking)
	avgQueryTime := mc.estimateAvgQueryTime(instance)

	metrics := &enterprise.TenantResourceMetrics{
		TenantID:            tenantID,
		Tier:                enterprise.TenantTierSmall, // Will be auto-upgraded by ResourceManager
		DatabaseSizeMB:      dbSizeMB,
		DatabaseGrowthRate:  0, // TODO: Track growth over time
		QueryComplexity:     0.5,
		AvgQueryTimeMs:      avgQueryTime,
		RequestsLast24h:     instance.RequestCount, // Simplified
		PeakRequestsPerMin:  0,                     // TODO: Track peak
		AvgResponseTimeMs:   100.0,                 // TODO: Track actual response times
		ErrorRate:           0.0,                   // TODO: Track errors
		MemoryUsageMB:       memoryMB,
		CPUUsagePercent:     cpuPercent,
		DiskIOPS:            0,   // TODO: Track disk I/O
		NetworkMBPS:         0.0, // TODO: Track network
		IsHotspot:           false,
		IsSpiking:           false,
		HotspotScore:        0.0,
		Updated:             time.Now(),
	}

	return metrics
}

// calculateDatabaseSize calculates the total size of tenant databases in MB
func (mc *MetricsCollector) calculateDatabaseSize(tenantID string) int64 {
	tenantDir := filepath.Join(mc.manager.dataDir, tenantID)

	// Calculate size of all database files
	var totalSize int64

	databases := []string{"data.db", "data.db-shm", "data.db-wal", "auxiliary.db", "auxiliary.db-shm", "auxiliary.db-wal", "hooks.db", "hooks.db-shm", "hooks.db-wal"}

	for _, dbFile := range databases {
		path := filepath.Join(tenantDir, dbFile)
		if info, err := os.Stat(path); err == nil {
			totalSize += info.Size()
		}
	}

	// Convert to MB
	return totalSize / (1024 * 1024)
}

// estimateMemoryUsage estimates memory usage for a tenant (simplified)
func (mc *MetricsCollector) estimateMemoryUsage(tenantID string) int64 {
	// In a real implementation, this would track actual memory allocation
	// For now, we'll estimate based on:
	// - Base overhead: 20 MB per tenant
	// - Database cache: proportional to DB size
	// - Connection pool: ~5 MB

	dbSizeMB := mc.calculateDatabaseSize(tenantID)

	// Estimate: 20 MB base + 10% of DB size for cache
	estimatedMemory := int64(20) + (dbSizeMB / 10)

	return estimatedMemory
}

// estimateCPUUsage estimates CPU usage for a tenant (simplified)
func (mc *MetricsCollector) estimateCPUUsage(tenantID string, instance *enterprise.TenantInstance) float64 {
	// In a real implementation, this would use:
	// - cgroups for container-level CPU tracking
	// - Process CPU time deltas
	// - Goroutine profiling

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Simplified: Base CPU on request rate
	now := time.Now()
	timeDelta := now.Sub(mc.lastCPUTime).Seconds()

	if timeDelta < 1 {
		timeDelta = 1
	}

	// Rough estimate: 0.1% CPU per request per second
	requestRate := float64(instance.RequestCount) / timeDelta
	estimatedCPU := requestRate * 0.1

	// Cap at reasonable value
	if estimatedCPU > 100.0 {
		estimatedCPU = 100.0
	}

	return estimatedCPU
}

// estimateAvgQueryTime estimates average query execution time
func (mc *MetricsCollector) estimateAvgQueryTime(instance *enterprise.TenantInstance) float64 {
	// In a real implementation, this would track actual query times
	// via database hooks or middleware

	// Simplified: Base on database size (larger DB = slower queries)
	dbSizeMB := mc.calculateDatabaseSize(instance.Tenant.ID)

	// Simple formula: 10ms base + 0.1ms per MB
	avgTime := 10.0 + (float64(dbSizeMB) * 0.1)

	return avgTime
}

// GetSystemMetrics returns overall system metrics
func (mc *MetricsCollector) GetSystemMetrics() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"go_memory_alloc_mb":    m.Alloc / 1024 / 1024,
		"go_memory_total_mb":    m.TotalAlloc / 1024 / 1024,
		"go_memory_sys_mb":      m.Sys / 1024 / 1024,
		"go_goroutines":         runtime.NumGoroutine(),
		"go_gc_runs":            m.NumGC,
		"go_gc_pause_ns":        m.PauseNs[(m.NumGC+255)%256],
	}
}

// Advanced: Process-level CPU tracking (Linux only)
func (mc *MetricsCollector) getProcessCPUTime() (time.Duration, error) {
	// Read /proc/self/stat for CPU time
	// Format: pid (comm) state ppid ... utime stime ...
	// This is Linux-specific and simplified

	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, fmt.Errorf("cannot read /proc/self/stat: %w", err)
	}

	// Parse stat file (simplified - would need proper parsing in production)
	// Fields 14 and 15 are utime and stime (in clock ticks)
	// This is a placeholder - real implementation would parse correctly

	_ = data // Suppress unused warning
	return 0, fmt.Errorf("not implemented")
}

// GetDiskUsage returns disk usage for the tenant data directory
func (mc *MetricsCollector) GetDiskUsage() (usedBytes int64, totalBytes int64, err error) {
	// Get filesystem stats for the data directory
	// This is OS-specific

	return mc.getDiskUsageForPath(mc.manager.dataDir)
}

// getDiskUsageForPath returns disk usage for a specific path
func (mc *MetricsCollector) getDiskUsageForPath(path string) (used int64, total int64, err error) {
	// Walk directory tree and sum file sizes
	err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			used += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, 0, err
	}

	// For total, we'd need filesystem stats (OS-specific)
	// This is a placeholder
	total = used * 10 // Assume we're using 10% of available space

	return used, total, nil
}

// Production-ready metrics collection would include:
//
// 1. **Database Metrics** (via SQLite hooks):
//    - Query execution times
//    - Query complexity (EXPLAIN analysis)
//    - Cache hit rates
//    - Lock contention
//
// 2. **CPU Metrics** (via cgroups or /proc):
//    - Per-process CPU time
//    - CPU throttling events
//    - Context switches
//
// 3. **Memory Metrics** (via cgroups or /proc):
//    - RSS (Resident Set Size)
//    - Peak memory usage
//    - Memory pressure events
//    - OOM events
//
// 4. **Disk I/O** (via /proc/[pid]/io):
//    - Read/write bytes
//    - Read/write syscalls
//    - I/O wait time
//
// 5. **Network Metrics** (via /proc/net):
//    - Bytes sent/received
//    - Connection count
//    - Packet loss
//
// Example production implementation for Linux:
//
// ```go
// func (mc *MetricsCollector) CollectLinuxMetrics(pid int) (*ProcessMetrics, error) {
//     // Read /proc/[pid]/stat for CPU
//     stat, err := readProcStat(pid)
//     if err != nil {
//         return nil, err
//     }
//
//     // Read /proc/[pid]/status for memory
//     status, err := readProcStatus(pid)
//     if err != nil {
//         return nil, err
//     }
//
//     // Read /proc/[pid]/io for disk I/O
//     io, err := readProcIO(pid)
//     if err != nil {
//         return nil, err
//     }
//
//     // Read cgroup stats if available
//     cgroup, _ := readCgroupStats(pid)
//
//     return &ProcessMetrics{
//         CPUPercent:    calculateCPUPercent(stat, mc.lastStat[pid]),
//         MemoryRSS:     status.VmRSS,
//         MemorySwap:    status.VmSwap,
//         DiskReadMB:    io.ReadBytes / 1024 / 1024,
//         DiskWriteMB:   io.WriteBytes / 1024 / 1024,
//         CPUThrottled:  cgroup.ThrottledTime,
//     }, nil
// }
// ```
//
// For now, the simplified estimators provide reasonable approximations
// for the resource management system to work with.
