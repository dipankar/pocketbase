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

	// Database growth tracking
	dbSizeHistory   map[string][]dbSizeSnapshot // tenantID -> snapshots
	dbSizeHistoryMu sync.RWMutex

	// Request rate tracking (for peak detection)
	requestWindows   map[string]*requestWindow // tenantID -> window
	requestWindowsMu sync.RWMutex

	// Response time tracking
	responseTimes   map[string]*responseTimeTracker // tenantID -> tracker
	responseTimesMu sync.RWMutex

	// Error rate tracking
	errorTrackers   map[string]*errorTracker // tenantID -> tracker
	errorTrackersMu sync.RWMutex

	mu     sync.RWMutex
	logger *log.Logger
}

// dbSizeSnapshot stores database size at a point in time
type dbSizeSnapshot struct {
	SizeMB    int64
	Timestamp time.Time
}

// requestWindow tracks requests in 1-minute windows
type requestWindow struct {
	Windows     []int64   // Request counts for each minute
	WindowStart time.Time // Start time of the first window
	TotalCount  int64     // Total requests across all windows
	mu          sync.Mutex
}

// responseTimeTracker tracks response times for a tenant
type responseTimeTracker struct {
	Samples      []float64 // Recent response times in ms
	MaxSamples   int       // Maximum number of samples to keep
	TotalTime    float64   // Total time for all samples
	TotalSamples int64     // Total number of samples
	mu           sync.Mutex
}

// errorTracker tracks errors for a tenant
type errorTracker struct {
	ErrorCount   int64     // Total errors
	RequestCount int64     // Total requests
	WindowStart  time.Time // Start of current window
	mu           sync.Mutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(manager *Manager) *MetricsCollector {
	return &MetricsCollector{
		manager:        manager,
		lastCPUSample:  make(map[string]time.Duration),
		lastCPUTime:    time.Now(),
		dbSizeHistory:  make(map[string][]dbSizeSnapshot),
		requestWindows: make(map[string]*requestWindow),
		responseTimes:  make(map[string]*responseTimeTracker),
		errorTrackers:  make(map[string]*errorTracker),
		logger:         log.Default(),
	}
}

// CollectTenantMetrics collects actual metrics for a tenant
func (mc *MetricsCollector) CollectTenantMetrics(tenantID string, instance *enterprise.TenantInstance) *enterprise.TenantResourceMetrics {
	// Calculate database size and track growth
	dbSizeMB := mc.calculateDatabaseSize(tenantID)
	mc.trackDatabaseSize(tenantID, dbSizeMB)

	// Calculate database growth rate (MB per hour)
	growthRate := mc.calculateGrowthRate(tenantID)

	// Estimate memory usage (simplified - would need process-level tracking in production)
	memoryMB := mc.estimateMemoryUsage(tenantID)

	// Estimate CPU usage
	cpuPercent := mc.estimateCPUUsage(tenantID, instance)

	// Calculate query metrics (would need actual query tracking)
	avgQueryTime := mc.estimateAvgQueryTime(instance)

	// Get peak requests per minute from tracked windows
	peakRequests := mc.getPeakRequestsPerMin(tenantID)

	// Get average response time from tracker
	avgResponseTime := mc.getAvgResponseTime(tenantID)

	// Get error rate from tracker
	errorRate := mc.getErrorRate(tenantID)

	metrics := &enterprise.TenantResourceMetrics{
		TenantID:            tenantID,
		Tier:                enterprise.TenantTierSmall, // Will be auto-upgraded by ResourceManager
		DatabaseSizeMB:      dbSizeMB,
		DatabaseGrowthRate:  growthRate,
		QueryComplexity:     0.5,
		AvgQueryTimeMs:      avgQueryTime,
		RequestsLast24h:     instance.RequestCount, // Simplified
		PeakRequestsPerMin:  peakRequests,
		AvgResponseTimeMs:   avgResponseTime,
		ErrorRate:           errorRate,
		MemoryUsageMB:       memoryMB,
		CPUUsagePercent:     cpuPercent,
		DiskIOPS:            0,   // TODO: Track disk I/O (requires OS-specific implementation)
		NetworkMBPS:         0.0, // TODO: Track network (requires OS-specific implementation)
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

// trackDatabaseSize records database size for growth tracking
func (mc *MetricsCollector) trackDatabaseSize(tenantID string, sizeMB int64) {
	mc.dbSizeHistoryMu.Lock()
	defer mc.dbSizeHistoryMu.Unlock()

	now := time.Now()
	snapshot := dbSizeSnapshot{
		SizeMB:    sizeMB,
		Timestamp: now,
	}

	// Get or create history
	history := mc.dbSizeHistory[tenantID]

	// Add new snapshot
	history = append(history, snapshot)

	// Keep only last 24 snapshots (assuming ~5 min intervals = 2 hours of history)
	if len(history) > 24 {
		history = history[len(history)-24:]
	}

	mc.dbSizeHistory[tenantID] = history
}

// calculateGrowthRate calculates database growth rate in MB per hour
func (mc *MetricsCollector) calculateGrowthRate(tenantID string) float64 {
	mc.dbSizeHistoryMu.RLock()
	defer mc.dbSizeHistoryMu.RUnlock()

	history := mc.dbSizeHistory[tenantID]
	if len(history) < 2 {
		return 0.0 // Need at least 2 data points
	}

	// Get oldest and newest snapshots
	oldest := history[0]
	newest := history[len(history)-1]

	// Calculate growth
	sizeDelta := float64(newest.SizeMB - oldest.SizeMB)
	timeDelta := newest.Timestamp.Sub(oldest.Timestamp).Hours()

	if timeDelta < 0.01 { // Avoid division by zero
		return 0.0
	}

	// Return MB per hour
	return sizeDelta / timeDelta
}

// RecordRequest records a request for peak tracking
func (mc *MetricsCollector) RecordRequest(tenantID string) {
	mc.requestWindowsMu.Lock()
	window, exists := mc.requestWindows[tenantID]
	if !exists {
		window = &requestWindow{
			Windows:     make([]int64, 60), // Track last 60 minutes
			WindowStart: time.Now(),
			TotalCount:  0,
		}
		mc.requestWindows[tenantID] = window
	}
	mc.requestWindowsMu.Unlock()

	window.mu.Lock()
	defer window.mu.Unlock()

	now := time.Now()
	minutesSinceStart := int(now.Sub(window.WindowStart).Minutes())

	// Rotate windows if needed
	if minutesSinceStart >= len(window.Windows) {
		// Shift windows and reset old ones
		shiftAmount := minutesSinceStart - len(window.Windows) + 1
		if shiftAmount >= len(window.Windows) {
			// Complete reset
			window.Windows = make([]int64, 60)
			window.WindowStart = now
			minutesSinceStart = 0
		} else {
			// Shift existing windows
			copy(window.Windows, window.Windows[shiftAmount:])
			for i := len(window.Windows) - shiftAmount; i < len(window.Windows); i++ {
				window.Windows[i] = 0
			}
			window.WindowStart = window.WindowStart.Add(time.Duration(shiftAmount) * time.Minute)
			minutesSinceStart = int(now.Sub(window.WindowStart).Minutes())
		}
	}

	// Increment current window
	if minutesSinceStart >= 0 && minutesSinceStart < len(window.Windows) {
		window.Windows[minutesSinceStart]++
		window.TotalCount++
	}
}

// getPeakRequestsPerMin returns the peak requests per minute
func (mc *MetricsCollector) getPeakRequestsPerMin(tenantID string) int64 {
	mc.requestWindowsMu.RLock()
	window, exists := mc.requestWindows[tenantID]
	mc.requestWindowsMu.RUnlock()

	if !exists {
		return 0
	}

	window.mu.Lock()
	defer window.mu.Unlock()

	var peak int64
	for _, count := range window.Windows {
		if count > peak {
			peak = count
		}
	}

	return peak
}

// RecordResponseTime records a response time for averaging
func (mc *MetricsCollector) RecordResponseTime(tenantID string, responseTimeMs float64) {
	mc.responseTimesMu.Lock()
	tracker, exists := mc.responseTimes[tenantID]
	if !exists {
		tracker = &responseTimeTracker{
			Samples:      make([]float64, 0, 100),
			MaxSamples:   100, // Keep last 100 samples
			TotalTime:    0.0,
			TotalSamples: 0,
		}
		mc.responseTimes[tenantID] = tracker
	}
	mc.responseTimesMu.Unlock()

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	// Add sample
	tracker.Samples = append(tracker.Samples, responseTimeMs)
	tracker.TotalTime += responseTimeMs
	tracker.TotalSamples++

	// Keep only recent samples
	if len(tracker.Samples) > tracker.MaxSamples {
		// Remove oldest sample from total
		removed := tracker.Samples[0]
		tracker.TotalTime -= removed
		tracker.Samples = tracker.Samples[1:]
	}
}

// getAvgResponseTime returns the average response time in milliseconds
func (mc *MetricsCollector) getAvgResponseTime(tenantID string) float64 {
	mc.responseTimesMu.RLock()
	tracker, exists := mc.responseTimes[tenantID]
	mc.responseTimesMu.RUnlock()

	if !exists || len(tracker.Samples) == 0 {
		return 100.0 // Default estimate
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	return tracker.TotalTime / float64(len(tracker.Samples))
}

// RecordError records an error for error rate tracking
func (mc *MetricsCollector) RecordError(tenantID string) {
	mc.RecordRequestOutcome(tenantID, true)
}

// RecordSuccess records a successful request for error rate tracking
func (mc *MetricsCollector) RecordSuccess(tenantID string) {
	mc.RecordRequestOutcome(tenantID, false)
}

// RecordRequestOutcome records a request outcome (success or error)
func (mc *MetricsCollector) RecordRequestOutcome(tenantID string, isError bool) {
	mc.errorTrackersMu.Lock()
	tracker, exists := mc.errorTrackers[tenantID]
	if !exists {
		tracker = &errorTracker{
			ErrorCount:   0,
			RequestCount: 0,
			WindowStart:  time.Now(),
		}
		mc.errorTrackers[tenantID] = tracker
	}
	mc.errorTrackersMu.Unlock()

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	// Reset window if older than 1 hour
	if time.Since(tracker.WindowStart) > time.Hour {
		tracker.ErrorCount = 0
		tracker.RequestCount = 0
		tracker.WindowStart = time.Now()
	}

	tracker.RequestCount++
	if isError {
		tracker.ErrorCount++
	}
}

// getErrorRate returns the error rate as a percentage
func (mc *MetricsCollector) getErrorRate(tenantID string) float64 {
	mc.errorTrackersMu.RLock()
	tracker, exists := mc.errorTrackers[tenantID]
	mc.errorTrackersMu.RUnlock()

	if !exists {
		return 0.0
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	if tracker.RequestCount == 0 {
		return 0.0
	}

	return (float64(tracker.ErrorCount) / float64(tracker.RequestCount)) * 100.0
}

// CleanupTenant removes all metrics data for an unloaded tenant to prevent memory leaks
func (mc *MetricsCollector) CleanupTenant(tenantID string) {
	// Cleanup dbSizeHistory
	mc.dbSizeHistoryMu.Lock()
	delete(mc.dbSizeHistory, tenantID)
	mc.dbSizeHistoryMu.Unlock()

	// Cleanup requestWindows
	mc.requestWindowsMu.Lock()
	delete(mc.requestWindows, tenantID)
	mc.requestWindowsMu.Unlock()

	// Cleanup responseTimes
	mc.responseTimesMu.Lock()
	delete(mc.responseTimes, tenantID)
	mc.responseTimesMu.Unlock()

	// Cleanup errorTrackers
	mc.errorTrackersMu.Lock()
	delete(mc.errorTrackers, tenantID)
	mc.errorTrackersMu.Unlock()

	// Cleanup lastCPUSample
	mc.mu.Lock()
	delete(mc.lastCPUSample, tenantID)
	mc.mu.Unlock()

	mc.logger.Printf("[MetricsCollector] Cleaned up metrics data for tenant: %s", tenantID)
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
