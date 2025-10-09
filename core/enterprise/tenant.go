package enterprise

import (
	"context"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// TenantInstance represents a running tenant PocketBase instance
type TenantInstance struct {
	Tenant *Tenant  // Tenant metadata
	App    core.App // PocketBase app instance (interface, not pointer)

	// State
	LoadedAt     time.Time
	LastAccessed time.Time
	RequestCount int64

	// Litestream
	LitestreamRunning bool
	LastReplication   time.Time
}

// TenantManager defines the interface for managing tenant lifecycle
type TenantManager interface {
	// LoadTenant loads a tenant from S3 or cache
	LoadTenant(ctx context.Context, tenantID string) (*TenantInstance, error)

	// UnloadTenant removes a tenant from memory (saves to S3 first)
	UnloadTenant(ctx context.Context, tenantID string) error

	// GetTenant retrieves a cached tenant (does not load from S3)
	GetTenant(tenantID string) (*TenantInstance, error)

	// ListActiveTenants returns all currently loaded tenants
	ListActiveTenants() []*TenantInstance

	// EvictIdleTenants removes tenants that haven't been accessed recently
	EvictIdleTenants(idleThreshold time.Duration) error
}

// ControlPlaneClient defines the interface for communicating with the control plane
type ControlPlaneClient interface {
	// GetTenantMetadata retrieves tenant metadata from control plane
	GetTenantMetadata(ctx context.Context, tenantID string) (*Tenant, error)

	// GetTenantByDomain retrieves tenant by domain name
	GetTenantByDomain(ctx context.Context, domain string) (*Tenant, error)

	// UpdateTenantStatus updates tenant status
	UpdateTenantStatus(ctx context.Context, tenantID string, status TenantStatus) error

	// RegisterNode registers a tenant node with the control plane
	RegisterNode(ctx context.Context, nodeInfo *NodeInfo) error

	// SendHeartbeat sends a heartbeat from this node
	SendHeartbeat(ctx context.Context, nodeID string, activeTenantsCount int) error

	// GetPlacementDecision requests placement decision for a tenant
	GetPlacementDecision(ctx context.Context, tenantID string) (*PlacementDecision, error)
}

// PlacementStrategy defines the interface for tenant placement algorithms
type PlacementStrategy interface {
	// SelectNode chooses the best node for a tenant
	SelectNode(tenant *Tenant, nodes []*NodeInfo) (*NodeInfo, error)

	// ShouldRebalance determines if tenants should be redistributed
	ShouldRebalance(nodes []*NodeInfo) bool

	// GenerateRebalancePlan creates a rebalancing plan
	GenerateRebalancePlan(nodes []*NodeInfo, tenants []*Tenant) ([]*PlacementDecision, error)
}

// StorageBackend defines the interface for S3 operations
type StorageBackend interface {
	// DownloadTenantDB downloads tenant databases from S3
	DownloadTenantDB(ctx context.Context, tenant *Tenant, dbName string, destPath string) error

	// UploadTenantDB uploads tenant database to S3
	UploadTenantDB(ctx context.Context, tenant *Tenant, dbName string, sourcePath string) error

	// DeleteTenantData removes all tenant data from S3
	DeleteTenantData(ctx context.Context, tenant *Tenant) error

	// ListTenantBackups lists available backups for a tenant
	ListTenantBackups(ctx context.Context, tenantID string) ([]string, error)

	// RestoreFromBackup restores tenant data from a specific backup
	RestoreFromBackup(ctx context.Context, tenantID string, backupID string) error
}

// TenantRequest wraps an HTTP request with tenant context
type TenantRequest struct {
	TenantID string
	Domain   string
	Path     string
	Method   string
}

// TenantLifecycleEvent represents events in the tenant lifecycle
type TenantLifecycleEvent struct {
	Type      string    // created, loaded, evicted, deleted
	TenantID  string
	NodeID    string
	Timestamp time.Time
	Metadata  map[string]interface{}
}

// MetricsCollector defines the interface for collecting tenant metrics
type MetricsCollector interface {
	// RecordRequest records an API request for a tenant
	RecordRequest(tenantID string, duration time.Duration, statusCode int)

	// RecordStorageUsage records storage usage
	RecordStorageUsage(tenantID string, usageMB int64)

	// RecordTenantEvent records a lifecycle event
	RecordTenantEvent(event *TenantLifecycleEvent)

	// GetTenantMetrics retrieves metrics for a specific tenant
	GetTenantMetrics(tenantID string, start, end time.Time) (map[string]interface{}, error)
}
