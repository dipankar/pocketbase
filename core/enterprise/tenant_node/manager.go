package tenant_node

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/core/enterprise"
	"github.com/pocketbase/pocketbase/core/enterprise/health"
	"github.com/pocketbase/pocketbase/core/enterprise/metrics"
	storagepkg "github.com/pocketbase/pocketbase/core/enterprise/storage"
)

// Manager manages tenant instances on a tenant node
type Manager struct {
	config *enterprise.ClusterConfig
	nodeID string

	// Storage
	storage           enterprise.StorageBackend
	cpClient          enterprise.ControlPlaneClient
	litestreamManager *storagepkg.LitestreamManager
	dataDir           string

	// Cache
	tenants   map[string]*enterprise.TenantInstance
	tenantsMu sync.RWMutex
	capacity  int // Max number of tenants

	// LRU tracking
	accessOrder []string // Tenant IDs in access order (most recent last)

	// Archiving
	archiver *TenantArchiver

	// Resource management
	resourceMgr       *enterprise.ResourceManager
	metricsCollector  *MetricsCollector

	// Health and monitoring
	healthChecker *health.Checker
	metrics       *metrics.Collector

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	logger *log.Logger
}

// NewManager creates a new tenant node manager
func NewManager(config *enterprise.ClusterConfig, storage enterprise.StorageBackend, cpClient enterprise.ControlPlaneClient) (*Manager, error) {
	if config.Mode != enterprise.ModeTenantNode && config.Mode != enterprise.ModeAllInOne {
		return nil, fmt.Errorf("invalid mode for tenant node: %s", config.Mode)
	}

	ctx, cancel := context.WithCancel(context.Background())

	nodeID := enterprise.GenerateNodeID()
	dataDir := filepath.Join(config.DataDir, "tenants")

	// Initialize health checker
	healthChecker := health.NewChecker("tenant-node")
	healthChecker.SetMetadata("nodeId", nodeID)
	healthChecker.SetMetadata("mode", config.Mode)

	// Initialize metrics collector
	metricsCollector := metrics.NewCollector("tenant_node")

	mgr := &Manager{
		config:            config,
		nodeID:            nodeID,
		storage:           storage,
		cpClient:          cpClient,
		litestreamManager: nil, // Will be initialized after struct is created
		dataDir:           dataDir,
		tenants:           make(map[string]*enterprise.TenantInstance),
		accessOrder:       make([]string, 0),
		capacity:          config.MaxTenants,
		healthChecker:     healthChecker,
		metrics:           metricsCollector,
		ctx:               ctx,
		cancel:            cancel,
		logger:            log.Default(),
	}

	// Initialize Litestream manager (after mgr is created to avoid package name collision)
	mgr.litestreamManager = storagepkg.NewLitestreamManager(config)

	// Initialize resource manager
	mgr.resourceMgr = enterprise.NewResourceManager()
	mgr.setupResourceCallbacks()

	// Initialize metrics collector
	mgr.metricsCollector = NewMetricsCollector(mgr)

	// Initialize tenant archiver (if storage is S3Backend)
	if s3Backend, ok := storage.(*storagepkg.S3Backend); ok {
		mgr.archiver = NewTenantArchiver(mgr, s3Backend, mgr.litestreamManager, nil)
	}

	return mgr, nil
}

// Start initializes and starts the tenant node manager
func (m *Manager) Start() error {
	m.logger.Printf("[TenantNode] Starting tenant node: %s", m.nodeID)

	// Register with control plane
	nodeAddress := m.config.NodeAddress
	if nodeAddress == "" {
		nodeAddress = "localhost:8091" // Default address
	}

	nodeInfo := &enterprise.NodeInfo{
		ID:       m.nodeID,
		Address:  nodeAddress,
		Status:   "online",
		Capacity: m.capacity,
	}

	if err := m.cpClient.RegisterNode(m.ctx, nodeInfo); err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	// Register health checks
	m.healthChecker.Register("control_plane", func(ctx context.Context) error {
		if m.cpClient == nil {
			return fmt.Errorf("control plane client not initialized")
		}
		// Test connectivity by getting metadata (using a dummy tenant ID)
		_, err := m.cpClient.GetTenantMetadata(ctx, "health-check-test")
		if err != nil && err.Error() != "tenant not found" {
			return fmt.Errorf("control plane unreachable: %w", err)
		}
		return nil
	})

	m.healthChecker.Register("storage", func(ctx context.Context) error {
		if m.storage == nil {
			return fmt.Errorf("storage backend not initialized")
		}
		return nil
	})

	m.healthChecker.Register("litestream", func(ctx context.Context) error {
		if m.litestreamManager == nil {
			return fmt.Errorf("litestream manager not initialized")
		}
		return nil
	})

	m.healthChecker.Register("capacity", func(ctx context.Context) error {
		m.tenantsMu.RLock()
		defer m.tenantsMu.RUnlock()

		activeCount := len(m.tenants)
		m.healthChecker.SetMetadata("activeTenants", activeCount)
		m.healthChecker.SetMetadata("capacity", m.capacity)
		m.healthChecker.SetMetadata("utilizationPercent", float64(activeCount)/float64(m.capacity)*100)

		if activeCount >= m.capacity {
			return fmt.Errorf("at capacity: %d/%d", activeCount, m.capacity)
		}
		return nil
	})

	// Start background tasks
	m.wg.Add(2)
	go m.sendHeartbeats()
	go m.evictIdleTenants()

	// Start resource manager
	if m.resourceMgr != nil {
		m.resourceMgr.Start()
	}

	// Start tenant archiver if available
	if m.archiver != nil {
		m.archiver.Start()
	}

	m.logger.Printf("[TenantNode] Tenant node started successfully")
	return nil
}

// Stop gracefully shuts down the tenant node manager
func (m *Manager) Stop() error {
	m.logger.Printf("[TenantNode] Stopping tenant node...")

	m.cancel()
	m.wg.Wait()

	// Stop resource manager
	if m.resourceMgr != nil {
		m.resourceMgr.Stop()
	}

	// Stop tenant archiver if available
	if m.archiver != nil {
		m.archiver.Stop()
	}

	// Stop all Litestream replications first
	if m.litestreamManager != nil {
		if err := m.litestreamManager.StopAllReplications(); err != nil {
			m.logger.Printf("[TenantNode] Error stopping Litestream replications: %v", err)
		}
	}

	// Unload all tenants
	m.tenantsMu.Lock()
	defer m.tenantsMu.Unlock()

	for tenantID := range m.tenants {
		if err := m.unloadTenantLocked(tenantID); err != nil {
			m.logger.Printf("[TenantNode] Error unloading tenant %s: %v", tenantID, err)
		}
	}

	m.logger.Printf("[TenantNode] Tenant node stopped")
	return nil
}

// LoadTenant loads a tenant from S3 or returns from cache
func (m *Manager) LoadTenant(ctx context.Context, tenantID string) (*enterprise.TenantInstance, error) {
	m.tenantsMu.Lock()
	defer m.tenantsMu.Unlock()

	// Check cache first
	if instance, exists := m.tenants[tenantID]; exists {
		m.updateAccessOrder(tenantID)
		instance.LastAccessed = time.Now()
		instance.RequestCount++

		// Update resource metrics on access
		m.recordTenantMetrics(tenantID, instance)

		return instance, nil
	}

	// Track load duration
	start := time.Now()
	defer func() {
		m.metrics.TenantLoadDuration.Observe(time.Since(start).Seconds())
	}()

	// Check weighted capacity (large tenants count as multiple slots)
	used, total := m.getWeightedCapacity()
	if used >= total {
		// Evict least recently used tenant
		if err := m.evictLRULocked(); err != nil {
			return nil, fmt.Errorf("failed to evict tenant: %w", err)
		}
	}

	// Get tenant metadata from control plane
	tenant, err := m.cpClient.GetTenantMetadata(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant metadata: %w", err)
	}

	// Restore tenant databases from S3 using Litestream
	tenantDir := filepath.Join(m.dataDir, tenantID)

	// Restore each database using Litestream (handles both existing and new databases)
	if err := m.litestreamManager.RestoreDatabase(ctx, tenantID, "data.db", filepath.Join(tenantDir, "data.db")); err != nil {
		return nil, fmt.Errorf("failed to restore data.db: %w", err)
	}

	if err := m.litestreamManager.RestoreDatabase(ctx, tenantID, "auxiliary.db", filepath.Join(tenantDir, "auxiliary.db")); err != nil {
		return nil, fmt.Errorf("failed to restore auxiliary.db: %w", err)
	}

	if err := m.litestreamManager.RestoreDatabase(ctx, tenantID, "hooks.db", filepath.Join(tenantDir, "hooks.db")); err != nil {
		return nil, fmt.Errorf("failed to restore hooks.db: %w", err)
	}

	// Create PocketBase app instance for this tenant
	app := core.NewBaseApp(core.BaseAppConfig{
		DataDir:       tenantDir,
		EncryptionEnv: fmt.Sprintf("PB_ENCRYPTION_%s", tenantID),
		IsDev:         false,
	})

	// Bootstrap the app
	if err := app.Bootstrap(); err != nil {
		return nil, fmt.Errorf("failed to bootstrap tenant app: %w", err)
	}

	// Start Litestream replication for all databases
	litestreamRunning := true

	if err := m.litestreamManager.StartReplication(tenantID, filepath.Join(tenantDir, "data.db"), "data.db"); err != nil {
		m.logger.Printf("[TenantNode] Failed to start Litestream for data.db: %v", err)
		litestreamRunning = false
	}

	if err := m.litestreamManager.StartReplication(tenantID, filepath.Join(tenantDir, "auxiliary.db"), "auxiliary.db"); err != nil {
		m.logger.Printf("[TenantNode] Failed to start Litestream for auxiliary.db: %v", err)
		litestreamRunning = false
	}

	if err := m.litestreamManager.StartReplication(tenantID, filepath.Join(tenantDir, "hooks.db"), "hooks.db"); err != nil {
		m.logger.Printf("[TenantNode] Failed to start Litestream for hooks.db: %v", err)
		litestreamRunning = false
	}

	// Create tenant instance
	instance := &enterprise.TenantInstance{
		Tenant:            tenant,
		App:               app, // app is *BaseApp which implements App interface
		LoadedAt:          time.Now(),
		LastAccessed:      time.Now(),
		RequestCount:      1,
		LitestreamRunning: litestreamRunning,
	}

	// Cache the instance
	m.tenants[tenantID] = instance
	m.updateAccessOrder(tenantID)

	// Update metrics
	m.metrics.TenantsLoaded.Inc()
	m.metrics.TenantsActive.Set(float64(len(m.tenants)))
	m.metrics.CacheUtilization.Set(float64(len(m.tenants)) / float64(m.capacity) * 100)

	// Record resource metrics
	m.recordTenantMetrics(tenantID, instance)

	m.logger.Printf("[TenantNode] Loaded tenant: %s", tenantID)

	// Notify control plane
	if err := m.cpClient.UpdateTenantStatus(ctx, tenantID, enterprise.TenantStatusActive); err != nil {
		m.logger.Printf("[TenantNode] Failed to update tenant status: %v", err)
	}

	return instance, nil
}

// UnloadTenant removes a tenant from memory and syncs to S3
func (m *Manager) UnloadTenant(ctx context.Context, tenantID string) error {
	m.tenantsMu.Lock()
	defer m.tenantsMu.Unlock()

	return m.unloadTenantLocked(tenantID)
}

// unloadTenantLocked unloads a tenant (must be called with lock held)
func (m *Manager) unloadTenantLocked(tenantID string) error {
	instance, exists := m.tenants[tenantID]
	if !exists {
		return nil // Already unloaded
	}

	// Track unload duration
	start := time.Now()
	defer func() {
		m.metrics.TenantUnloadDuration.Observe(time.Since(start).Seconds())
	}()

	m.logger.Printf("[TenantNode] Unloading tenant: %s", tenantID)

	// Stop Litestream replication for all databases (with final sync)
	if instance.LitestreamRunning {
		if err := m.litestreamManager.StopReplication(tenantID, "data.db"); err != nil {
			m.logger.Printf("[TenantNode] Error stopping Litestream for data.db: %v", err)
		}

		if err := m.litestreamManager.StopReplication(tenantID, "auxiliary.db"); err != nil {
			m.logger.Printf("[TenantNode] Error stopping Litestream for auxiliary.db: %v", err)
		}

		if err := m.litestreamManager.StopReplication(tenantID, "hooks.db"); err != nil {
			m.logger.Printf("[TenantNode] Error stopping Litestream for hooks.db: %v", err)
		}
	}

	// Close the PocketBase app
	// Note: We can't easily trigger the terminate event from here as it requires
	// the full app lifecycle. For now, we'll just close the databases.
	// TODO: Implement proper app shutdown

	// Remove from cache
	delete(m.tenants, tenantID)
	m.removeFromAccessOrder(tenantID)

	// Update metrics
	m.metrics.TenantsUnloaded.Inc()
	m.metrics.TenantsActive.Set(float64(len(m.tenants)))
	m.metrics.CacheUtilization.Set(float64(len(m.tenants)) / float64(m.capacity) * 100)

	m.logger.Printf("[TenantNode] Unloaded tenant: %s (requests: %d)", tenantID, instance.RequestCount)
	return nil
}

// GetTenant retrieves a cached tenant (does not load from S3)
func (m *Manager) GetTenant(tenantID string) (*enterprise.TenantInstance, error) {
	m.tenantsMu.RLock()
	defer m.tenantsMu.RUnlock()

	instance, exists := m.tenants[tenantID]
	if !exists {
		return nil, enterprise.ErrTenantNotFound
	}

	return instance, nil
}

// ListActiveTenants returns all currently loaded tenants
func (m *Manager) ListActiveTenants() []*enterprise.TenantInstance {
	m.tenantsMu.RLock()
	defer m.tenantsMu.RUnlock()

	instances := make([]*enterprise.TenantInstance, 0, len(m.tenants))
	for _, instance := range m.tenants {
		instances = append(instances, instance)
	}

	return instances
}

// EvictIdleTenants removes tenants that haven't been accessed recently
func (m *Manager) EvictIdleTenants(idleThreshold time.Duration) error {
	m.tenantsMu.Lock()
	defer m.tenantsMu.Unlock()

	now := time.Now()
	toEvict := make([]string, 0)

	for tenantID, instance := range m.tenants {
		if now.Sub(instance.LastAccessed) > idleThreshold {
			toEvict = append(toEvict, tenantID)
		}
	}

	for _, tenantID := range toEvict {
		if err := m.unloadTenantLocked(tenantID); err != nil {
			m.logger.Printf("[TenantNode] Failed to evict tenant %s: %v", tenantID, err)
		} else {
			m.metrics.TenantsEvicted.Inc()
		}
	}

	if len(toEvict) > 0 {
		m.logger.Printf("[TenantNode] Evicted %d idle tenants", len(toEvict))
	}

	return nil
}

// evictLRULocked evicts the least recently used tenant (must be called with lock held)
func (m *Manager) evictLRULocked() error {
	if len(m.accessOrder) == 0 {
		return fmt.Errorf("no tenants to evict")
	}

	// First tenant in access order is least recently used
	lruTenantID := m.accessOrder[0]
	err := m.unloadTenantLocked(lruTenantID)
	if err == nil {
		m.metrics.TenantsEvicted.Inc()
	}
	return err
}

// updateAccessOrder updates the access order for LRU tracking
func (m *Manager) updateAccessOrder(tenantID string) {
	// Remove from current position
	m.removeFromAccessOrder(tenantID)

	// Add to end (most recently used)
	m.accessOrder = append(m.accessOrder, tenantID)
}

// removeFromAccessOrder removes a tenant from the access order
func (m *Manager) removeFromAccessOrder(tenantID string) {
	for i, id := range m.accessOrder {
		if id == tenantID {
			m.accessOrder = append(m.accessOrder[:i], m.accessOrder[i+1:]...)
			break
		}
	}
}

// sendHeartbeats periodically sends heartbeats to the control plane
func (m *Manager) sendHeartbeats() {
	defer m.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.tenantsMu.RLock()
			activeCount := len(m.tenants)
			m.tenantsMu.RUnlock()

			if err := m.cpClient.SendHeartbeat(m.ctx, m.nodeID, activeCount); err != nil {
				m.logger.Printf("[TenantNode] Failed to send heartbeat: %v", err)
			}
		}
	}
}

// evictIdleTenants periodically evicts idle tenants
func (m *Manager) evictIdleTenants() {
	defer m.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			// Evict tenants idle for more than 10 minutes
			if err := m.EvictIdleTenants(10 * time.Minute); err != nil {
				m.logger.Printf("[TenantNode] Failed to evict idle tenants: %v", err)
			}
		}
	}
}

// GetHealthChecker returns the health checker for exposing health endpoints
func (m *Manager) GetHealthChecker() *health.Checker {
	return m.healthChecker
}

// setupResourceCallbacks configures resource manager callbacks
func (m *Manager) setupResourceCallbacks() {
	m.resourceMgr.SetCallbacks(
		// On hotspot detected
		func(tenantID string, metrics *enterprise.TenantResourceMetrics) {
			m.logger.Printf("[TenantNode] Hotspot detected: %s (score: %.2f, tier: %s)",
				tenantID, metrics.HotspotScore, metrics.Tier)

			// For enterprise tier, consider moving to dedicated node
			if metrics.Tier == enterprise.TenantTierEnterprise {
				m.logger.Printf("[TenantNode] Enterprise tenant %s should be on dedicated node", tenantID)
				// TODO: Notify control plane to assign to dedicated node pool
			}

			// For other tiers, check if should evict to free resources
			if shouldEvict, reason := m.resourceMgr.ShouldEvict(tenantID); shouldEvict {
				m.logger.Printf("[TenantNode] Evicting hotspot tenant %s: %s", tenantID, reason)
				if err := m.UnloadTenant(m.ctx, tenantID); err != nil {
					m.logger.Printf("[TenantNode] Failed to evict tenant %s: %v", tenantID, err)
				}
			}
		},
		// On tier upgrade
		func(tenantID string, oldTier, newTier enterprise.TenantTier) {
			m.logger.Printf("[TenantNode] Tenant %s tier upgraded: %s -> %s", tenantID, oldTier, newTier)

			// Notify control plane about tier change
			// TODO: Update tenant metadata in control plane
		},
		// On quota exceeded
		func(tenantID string, quotaType string, current, limit int64) {
			m.logger.Printf("[TenantNode] Tenant %s exceeded %s quota: %d > %d",
				tenantID, quotaType, current, limit)

			// For database size, stop accepting writes
			// For requests, rate limit
			// TODO: Implement quota enforcement
		},
	)
}

// recordTenantMetrics records resource metrics for a tenant
func (m *Manager) recordTenantMetrics(tenantID string, instance *enterprise.TenantInstance) {
	// Collect actual metrics using the metrics collector
	metrics := m.metricsCollector.CollectTenantMetrics(tenantID, instance)

	// Record to resource manager for hotspot detection
	m.resourceMgr.RecordMetrics(metrics)
}

// getWeightedCapacity calculates effective capacity considering tenant weights
func (m *Manager) getWeightedCapacity() (used int, total int) {
	m.tenantsMu.RLock()
	defer m.tenantsMu.RUnlock()

	total = m.capacity
	used = 0

	for tenantID := range m.tenants {
		weight := m.resourceMgr.GetTenantWeight(tenantID)
		used += weight
	}

	return used, total
}
