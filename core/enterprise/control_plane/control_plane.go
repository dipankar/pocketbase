package control_plane

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
	"github.com/pocketbase/pocketbase/core/enterprise/health"
)

// ControlPlane manages the distributed control plane for the multi-tenant system
type ControlPlane struct {
	config *enterprise.ClusterConfig

	// Core components
	storage   *BadgerStorage   // BadgerDB storage
	raft      *RaftNode        // Raft consensus
	placement *PlacementService // Tenant placement

	// State
	nodes   map[string]*enterprise.NodeInfo // Active tenant nodes
	nodesMu sync.RWMutex

	// Health and monitoring
	healthChecker *health.Checker

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// IPC server for handling requests from gateways and tenant nodes
	ipcServer *IPCServer

	// HTTP server for enterprise APIs
	httpServer *http.Server

	logger *log.Logger
}

// NewControlPlane creates a new control plane instance
func NewControlPlane(config *enterprise.ClusterConfig) (*ControlPlane, error) {
	if config.Mode != enterprise.ModeControlPlane && config.Mode != enterprise.ModeAllInOne {
		return nil, fmt.Errorf("invalid mode for control plane: %s", config.Mode)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize health checker
	healthChecker := health.NewChecker("control-plane")
	healthChecker.SetMetadata("nodeId", config.NodeID)
	healthChecker.SetMetadata("mode", config.Mode)

	cp := &ControlPlane{
		config:        config,
		nodes:         make(map[string]*enterprise.NodeInfo),
		healthChecker: healthChecker,
		ctx:           ctx,
		cancel:        cancel,
		logger:        log.Default(),
	}

	return cp, nil
}

// Start initializes and starts the control plane
func (cp *ControlPlane) Start() error {
	cp.logger.Printf("[ControlPlane] Starting control plane node: %s", cp.config.NodeID)

	// 1. Initialize BadgerDB storage
	storage, err := NewBadgerStorage(cp.config.DataDir)
	if err != nil {
		return fmt.Errorf("failed to initialize BadgerDB: %w", err)
	}
	cp.storage = storage

	// 2. Initialize Raft
	raftNode, err := NewRaftNode(cp.config, cp.storage)
	if err != nil {
		return fmt.Errorf("failed to initialize Raft: %w", err)
	}
	cp.raft = raftNode

	// 3. Initialize placement service
	cp.placement = NewPlacementService(cp.storage, cp.raft)

	// 4. Start IPC server for gateway/tenant node communication
	ipcServer, err := NewIPCServer(cp)
	if err != nil {
		return fmt.Errorf("failed to initialize IPC server: %w", err)
	}
	cp.ipcServer = ipcServer

	if err := cp.ipcServer.Start(); err != nil {
		return fmt.Errorf("failed to start IPC server: %w", err)
	}

	// 5. Register health checks
	cp.healthChecker.Register("raft", func(ctx context.Context) error {
		if cp.raft == nil {
			return fmt.Errorf("raft not initialized")
		}
		return nil
	})

	cp.healthChecker.Register("storage", func(ctx context.Context) error {
		if cp.storage == nil {
			return fmt.Errorf("storage not initialized")
		}
		// Test a simple read operation
		_, err := cp.storage.GetUser("health-check-test")
		if err != nil && err.Error() != "user not found" {
			return fmt.Errorf("storage error: %w", err)
		}
		return nil
	})

	cp.healthChecker.Register("disk", func(ctx context.Context) error {
		if cp.storage == nil || cp.storage.GetDiskManager() == nil {
			return fmt.Errorf("disk manager not initialized")
		}

		// Check disk usage
		_, usagePct := cp.storage.GetDiskManager().GetDiskUsage()
		cp.healthChecker.SetMetadata("diskUsagePercent", usagePct)

		// Mark as unhealthy if over 95%
		if usagePct >= 95.0 {
			return fmt.Errorf("disk usage critical: %.2f%%", usagePct)
		}

		return nil
	})

	cp.healthChecker.Register("ipc", func(ctx context.Context) error {
		if cp.ipcServer == nil {
			return fmt.Errorf("IPC server not initialized")
		}
		return nil
	})

	// 6. Start background tasks
	cp.wg.Add(2)
	go cp.monitorNodes()
	go cp.rebalanceTenants()

	cp.logger.Printf("[ControlPlane] Control plane started successfully")
	return nil
}

// Stop gracefully shuts down the control plane
func (cp *ControlPlane) Stop() error {
	cp.logger.Printf("[ControlPlane] Stopping control plane...")

	cp.cancel()
	cp.wg.Wait()

	if cp.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := cp.httpServer.Shutdown(ctx); err != nil {
			cp.logger.Printf("[ControlPlane] Error stopping HTTP server: %v", err)
		}
	}

	if cp.ipcServer != nil {
		if err := cp.ipcServer.Stop(); err != nil {
			cp.logger.Printf("[ControlPlane] Error stopping IPC server: %v", err)
		}
	}

	if cp.raft != nil {
		if err := cp.raft.Shutdown(); err != nil {
			cp.logger.Printf("[ControlPlane] Error shutting down Raft: %v", err)
		}
	}

	if cp.storage != nil {
		if err := cp.storage.Close(); err != nil {
			cp.logger.Printf("[ControlPlane] Error closing storage: %v", err)
		}
	}

	cp.logger.Printf("[ControlPlane] Control plane stopped")
	return nil
}

// StartHTTP starts the HTTP server for enterprise APIs
func (cp *ControlPlane) StartHTTP(addr string, handler http.Handler) error {
	cp.httpServer = &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	cp.logger.Printf("[ControlPlane] Starting HTTP API server on %s", addr)

	go func() {
		if err := cp.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			cp.logger.Printf("[ControlPlane] HTTP server error: %v", err)
		}
	}()

	return nil
}

// GetTenant retrieves tenant metadata
func (cp *ControlPlane) GetTenant(tenantID string) (*enterprise.Tenant, error) {
	return cp.storage.GetTenant(tenantID)
}

// GetTenantByDomain retrieves tenant by domain
func (cp *ControlPlane) GetTenantByDomain(domain string) (*enterprise.Tenant, error) {
	return cp.storage.GetTenantByDomain(domain)
}

// CreateTenant creates a new tenant
func (cp *ControlPlane) CreateTenant(tenant *enterprise.Tenant) error {
	// Verify user exists and is under quota
	user, err := cp.storage.GetUser(tenant.OwnerUserID)
	if err != nil {
		return fmt.Errorf("owner user not found: %w", err)
	}

	// Check user quota
	tenantCount, err := cp.storage.CountUserTenants(user.ID)
	if err != nil {
		return fmt.Errorf("failed to count user tenants: %w", err)
	}

	if tenantCount >= user.MaxTenants {
		return enterprise.NewQuotaError("tenants", int64(tenantCount), int64(user.MaxTenants))
	}

	// Set defaults
	if tenant.StorageQuotaMB == 0 {
		tenant.StorageQuotaMB = user.MaxStoragePerTenant
	}
	if tenant.APIRequestsQuota == 0 {
		tenant.APIRequestsQuota = user.MaxAPIRequestsDaily
	}

	tenant.Status = enterprise.TenantStatusCreated
	tenant.Created = time.Now()
	tenant.Updated = time.Now()

	// S3 paths
	tenant.S3Bucket = cp.config.S3Bucket
	tenant.S3Prefix = enterprise.GetS3TenantPrefix(tenant.ID)

	// Store via Raft
	return cp.storage.CreateTenant(tenant)
}

// UpdateTenantStatus updates tenant status
func (cp *ControlPlane) UpdateTenantStatus(tenantID string, status enterprise.TenantStatus) error {
	return cp.storage.UpdateTenantStatus(tenantID, status)
}

// AssignTenant assigns a tenant to a node
func (cp *ControlPlane) AssignTenant(tenantID string) (*enterprise.PlacementDecision, error) {
	return cp.placement.AssignTenant(tenantID)
}

// RegisterNode registers a new tenant node
func (cp *ControlPlane) RegisterNode(node *enterprise.NodeInfo) error {
	cp.nodesMu.Lock()
	defer cp.nodesMu.Unlock()

	node.Registered = time.Now()
	node.LastHeartbeat = time.Now()

	cp.nodes[node.ID] = node
	return cp.storage.SaveNode(node)
}

// UpdateNodeHeartbeat updates node heartbeat
func (cp *ControlPlane) UpdateNodeHeartbeat(nodeID string, activeTenantsCount int) error {
	cp.nodesMu.Lock()
	defer cp.nodesMu.Unlock()

	node, exists := cp.nodes[nodeID]
	if !exists {
		return enterprise.ErrNodeNotFound
	}

	node.LastHeartbeat = time.Now()
	node.ActiveTenants = activeTenantsCount

	return cp.storage.SaveNode(node)
}

// GetNodes returns all registered nodes
func (cp *ControlPlane) GetNodes() []*enterprise.NodeInfo {
	cp.nodesMu.RLock()
	defer cp.nodesMu.RUnlock()

	nodes := make([]*enterprise.NodeInfo, 0, len(cp.nodes))
	for _, node := range cp.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// monitorNodes monitors node health and marks unhealthy nodes as offline
func (cp *ControlPlane) monitorNodes() {
	defer cp.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cp.ctx.Done():
			return
		case <-ticker.C:
			cp.checkNodeHealth()
		}
	}
}

// checkNodeHealth checks all nodes for health
func (cp *ControlPlane) checkNodeHealth() {
	cp.nodesMu.Lock()
	defer cp.nodesMu.Unlock()

	heartbeatTimeout := 30 * time.Second
	now := time.Now()

	for nodeID, node := range cp.nodes {
		if now.Sub(node.LastHeartbeat) > heartbeatTimeout {
			if node.Status != "offline" {
				cp.logger.Printf("[ControlPlane] Node %s marked offline (no heartbeat)", nodeID)
				node.Status = "offline"
				cp.storage.SaveNode(node)
			}
		}
	}
}

// rebalanceTenants periodically checks if tenant rebalancing is needed
func (cp *ControlPlane) rebalanceTenants() {
	defer cp.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-cp.ctx.Done():
			return
		case <-ticker.C:
			// Only leader should rebalance
			if !cp.raft.IsLeader() {
				continue
			}

			if err := cp.placement.CheckRebalance(); err != nil {
				cp.logger.Printf("[ControlPlane] Rebalance check failed: %v", err)
			}
		}
	}
}

// CreateUser creates a new cluster user
func (cp *ControlPlane) CreateUser(user *enterprise.ClusterUser) error {
	return cp.storage.CreateUser(user)
}

// GetUser retrieves a cluster user
func (cp *ControlPlane) GetUser(userID string) (*enterprise.ClusterUser, error) {
	return cp.storage.GetUser(userID)
}

// GetUserByEmail retrieves a user by email
func (cp *ControlPlane) GetUserByEmail(email string) (*enterprise.ClusterUser, error) {
	return cp.storage.GetUserByEmail(email)
}

// UpdateUser updates user information
func (cp *ControlPlane) UpdateUser(user *enterprise.ClusterUser) error {
	return cp.storage.UpdateUser(user)
}

// ListUsers lists all cluster users with pagination
func (cp *ControlPlane) ListUsers(limit, offset int) ([]*enterprise.ClusterUser, int, error) {
	return cp.storage.ListUsers(limit, offset)
}

// ListTenants lists all tenants with optional filtering and pagination
func (cp *ControlPlane) ListTenants(limit, offset int, ownerUserID string) ([]*enterprise.Tenant, int, error) {
	return cp.storage.ListTenants(limit, offset, ownerUserID)
}

// GetHealthChecker returns the health checker for exposing health endpoints
func (cp *ControlPlane) GetHealthChecker() *health.Checker {
	return cp.healthChecker
}

// GetDiskStats returns disk usage statistics
func (cp *ControlPlane) GetDiskStats() map[string]interface{} {
	if cp.storage != nil {
		return cp.storage.GetDiskManager().GetStats()
	}
	return map[string]interface{}{}
}

// GetTenantActivity retrieves tenant activity
func (cp *ControlPlane) GetTenantActivity(tenantID string) (*enterprise.TenantActivity, error) {
	return cp.storage.GetActivity(tenantID)
}

// ListInactiveTenants returns tenants inactive since the given time
func (cp *ControlPlane) ListInactiveTenants(since time.Time) ([]*enterprise.TenantActivity, error) {
	return cp.storage.ListInactiveTenants(since)
}

// CountTenantsByTier returns the number of tenants in a given storage tier
func (cp *ControlPlane) CountTenantsByTier(tier enterprise.StorageTier) (int, error) {
	activities, err := cp.storage.ListActivitiesByTier(tier)
	if err != nil {
		return 0, err
	}
	return len(activities), nil
}

// ArchiveTenant archives a tenant to the specified tier
func (cp *ControlPlane) ArchiveTenant(tenantID string, tier enterprise.StorageTier) error {
	// Get current activity
	activity, err := cp.storage.GetActivity(tenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant activity: %w", err)
	}

	// Update tier
	now := time.Now()
	activity.StorageTier = tier
	activity.ArchiveDate = &now
	activity.Updated = now

	// Save activity
	return cp.storage.SaveActivity(activity)
}

// RestoreTenant restores an archived tenant
func (cp *ControlPlane) RestoreTenant(tenantID string) error {
	// Get current activity
	activity, err := cp.storage.GetActivity(tenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant activity: %w", err)
	}

	// Update tier to hot
	now := time.Now()
	activity.StorageTier = enterprise.StorageTierHot
	activity.RestoreCount++
	activity.LastRestore = &now
	activity.Updated = now

	// Save activity
	return cp.storage.SaveActivity(activity)
}

// SaveVerificationToken saves a verification token
func (cp *ControlPlane) SaveVerificationToken(token *enterprise.VerificationToken) error {
	return cp.storage.SaveVerificationToken(token)
}

// GetVerificationToken retrieves a verification token
func (cp *ControlPlane) GetVerificationToken(token string) (*enterprise.VerificationToken, error) {
	return cp.storage.GetVerificationToken(token)
}

// MarkVerificationTokenUsed marks a verification token as used
func (cp *ControlPlane) MarkVerificationTokenUsed(token string) error {
	return cp.storage.MarkVerificationTokenUsed(token)
}

// UseVerificationTokenAtomically validates and marks a token as used in a single atomic operation
// This prevents double-use of tokens due to race conditions
func (cp *ControlPlane) UseVerificationTokenAtomically(token string) (*enterprise.VerificationToken, error) {
	return cp.storage.Storage.UseVerificationTokenAtomically(token)
}
