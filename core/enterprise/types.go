package enterprise

import (
	"time"
)

// Mode represents the operational mode of the PocketBase instance
type Mode string

const (
	ModeControlPlane Mode = "control-plane" // Control plane node (Raft cluster)
	ModeTenantNode   Mode = "tenant-node"   // Stateless tenant worker
	ModeGateway      Mode = "gateway"       // Reverse proxy/load balancer
	ModeAllInOne     Mode = "all-in-one"    // Development/testing mode (all components)
)

// TenantStatus represents the lifecycle state of a tenant
type TenantStatus string

const (
	TenantStatusCreated   TenantStatus = "created"   // Metadata created
	TenantStatusAssigning TenantStatus = "assigning" // Being assigned to a node
	TenantStatusDeploying TenantStatus = "deploying" // Downloading from S3, bootstrapping
	TenantStatusActive    TenantStatus = "active"    // Serving requests
	TenantStatusIdle      TenantStatus = "idle"      // No recent activity
	TenantStatusEvicted   TenantStatus = "evicted"   // Removed from node cache
	TenantStatusArchived  TenantStatus = "archived"  // Long-term inactive (S3 Glacier)
	TenantStatusDeleted   TenantStatus = "deleted"   // Soft deleted
)

// Tenant represents a single tenant in the multi-tenant system
type Tenant struct {
	ID          string       `json:"id"`          // Unique tenant identifier (tenant_xxx)
	Domain      string       `json:"domain"`      // Custom domain (tenant123.platform.com)
	OwnerUserID string       `json:"ownerUserId"` // Cluster user who owns this tenant
	Status      TenantStatus `json:"status"`

	// Resource quotas
	StorageQuotaMB   int64 `json:"storageQuotaMb"`   // Storage limit in MB
	APIRequestsQuota int64 `json:"apiRequestsQuota"` // API requests per day

	// Current usage
	StorageUsedMB   int64 `json:"storageUsedMb"`
	APIRequestsUsed int64 `json:"apiRequestsUsed"`

	// Node assignment
	AssignedNodeID string    `json:"assignedNodeId,omitempty"` // Current node hosting this tenant
	AssignedAt     time.Time `json:"assignedAt,omitempty"`

	// Timestamps
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`

	// S3 metadata
	S3Bucket string `json:"s3Bucket"` // S3 bucket for tenant data
	S3Prefix string `json:"s3Prefix"` // S3 prefix (e.g., tenants/tenant_001/)
}

// ClusterUser represents a self-service SaaS customer
type ClusterUser struct {
	ID           string    `json:"id"`           // Unique user identifier (user_xxx)
	Email        string    `json:"email"`        // Login email
	Name         string    `json:"name"`         // Display name
	PasswordHash string    `json:"passwordHash"` // Bcrypt hash
	Verified     bool      `json:"verified"`     // Email verification status

	// Quotas
	MaxTenants          int   `json:"maxTenants"`          // Maximum number of tenants
	MaxStoragePerTenant int64 `json:"maxStoragePerTenant"` // Storage per tenant in MB
	MaxAPIRequestsDaily int64 `json:"maxApiRequestsDaily"` // API requests per tenant per day

	// Timestamps
	Created      time.Time  `json:"created"`
	Updated      time.Time  `json:"updated"`
	LastLogin    *time.Time `json:"lastLogin,omitempty"`

	// Session
	SessionToken string    `json:"sessionToken,omitempty"` // Current JWT token
	TokenExpiry  time.Time `json:"tokenExpiry,omitempty"`
}

// DefaultUserQuotas returns the default quota limits for new cluster users
func DefaultUserQuotas() (maxTenants int, maxStoragePerTenant int64, maxAPIRequestsDaily int64) {
	return 3, 1024, 100000 // 3 tenants, 1GB each, 100k requests/day
}

// NodeInfo represents a tenant node in the cluster
type NodeInfo struct {
	ID       string    `json:"id"`       // Unique node identifier
	Address  string    `json:"address"`  // Network address (host:port)
	Status   string    `json:"status"`   // online, offline, draining
	Capacity int       `json:"capacity"` // Max tenants this node can handle

	// Current load
	ActiveTenants int   `json:"activeTenants"` // Number of currently loaded tenants
	MemoryUsedMB  int64 `json:"memoryUsedMb"`  // Memory usage
	CPUPercent    int   `json:"cpuPercent"`    // CPU usage percentage

	// Health
	LastHeartbeat time.Time `json:"lastHeartbeat"`
	Version       string    `json:"version"` // PocketBase version

	// Timestamps
	Registered time.Time `json:"registered"`
}

// PlacementDecision represents a decision to place a tenant on a node
type PlacementDecision struct {
	TenantID    string    `json:"tenantId"`
	NodeID      string    `json:"nodeId"`
	NodeAddress string    `json:"nodeAddress"` // HTTP address of the tenant node (e.g., http://node1:8091)
	Reason      string    `json:"reason"`      // Why this node was chosen
	DecidedAt   time.Time `json:"decidedAt"`
}

// ClusterConfig holds the configuration for the enterprise cluster
type ClusterConfig struct {
	// Mode
	Mode Mode `json:"mode"`

	// Control Plane settings (for control-plane mode)
	NodeID       string   `json:"nodeId,omitempty"`       // This control plane node's ID
	RaftPeers    []string `json:"raftPeers,omitempty"`    // Raft peer addresses (cp1:7000,cp2:7000,cp3:7000)
	RaftBindAddr string   `json:"raftBindAddr,omitempty"` // Raft bind address
	DataDir      string   `json:"dataDir,omitempty"`      // BadgerDB data directory

	// Tenant Node settings (for tenant-node mode)
	ControlPlaneAddrs []string `json:"controlPlaneAddrs,omitempty"` // Control plane addresses
	MaxTenants        int      `json:"maxTenants,omitempty"`        // Max tenants this node can handle
	NodeAddress       string   `json:"nodeAddress,omitempty"`       // This node's advertised address (host:port)

	// Gateway settings (for gateway mode)
	GatewayControlPlaneAddrs []string `json:"gatewayControlPlaneAddrs,omitempty"`

	// S3 settings (all modes)
	S3Endpoint        string `json:"s3Endpoint"`
	S3Region          string `json:"s3Region"`
	S3Bucket          string `json:"s3Bucket"`
	S3AccessKeyID     string `json:"s3AccessKeyId"`
	S3SecretAccessKey string `json:"s3SecretAccessKey"`

	// Litestream settings
	LitestreamEnabled       bool   `json:"litestreamEnabled"`
	LitestreamReplicateSync bool   `json:"litestreamReplicateSync"` // Sync replication (slower but safer)
	LitestreamRetention     string `json:"litestreamRetention"`     // Retention period (e.g., "72h")

	// Security settings
	JWTSecret string `json:"jwtSecret,omitempty"` // Secret key for JWT signing (env: POCKETBASE_JWT_SECRET)
}

// QuotaIncreaseRequest represents a request to increase tenant quotas
type QuotaIncreaseRequest struct {
	ID               string    `json:"id"`
	UserID           string    `json:"userId"`
	TenantID         string    `json:"tenantId"`
	RequestedQuotaMB int64     `json:"requestedQuotaMb"`
	Reason           string    `json:"reason"`
	Status           string    `json:"status"` // pending, approved, rejected
	AdminNotes       string    `json:"adminNotes,omitempty"`
	Created          time.Time `json:"created"`
	Updated          time.Time `json:"updated"`
}

// AdminToken represents a long-lived cluster admin token
type AdminToken struct {
	Token   string    `json:"token"`   // Token string (admin_xxx...)
	Name    string    `json:"name"`    // Token name/description
	Created time.Time `json:"created"`
	LastUsed *time.Time `json:"lastUsed,omitempty"`
}

// VerificationToken represents an email verification token
type VerificationToken struct {
	Token   string    `json:"token"`   // Verification token
	UserID  string    `json:"userId"`  // User this token belongs to
	Email   string    `json:"email"`   // Email being verified
	Expires time.Time `json:"expires"` // Token expiration time
	Created time.Time `json:"created"` // Token creation time
	Used    bool      `json:"used"`    // Whether token has been used
}

// StorageTier represents the storage tier for a tenant
type StorageTier string

const (
	StorageTierHot  StorageTier = "hot"  // In-memory, active, Litestream replication
	StorageTierWarm StorageTier = "warm" // S3 Standard, inactive < 90 days, fast restore
	StorageTierCold StorageTier = "cold" // S3 Glacier Deep Archive, inactive > 90 days
)

// TenantActivity tracks tenant access patterns and storage tier
type TenantActivity struct {
	TenantID     string      `json:"tenantId"`
	LastAccess   time.Time   `json:"lastAccess"`   // Last API request
	AccessCount  int64       `json:"accessCount"`  // Total access count
	StorageTier  StorageTier `json:"storageTier"`  // Current storage tier

	// Archiving metadata
	ArchiveDate  *time.Time  `json:"archiveDate,omitempty"`  // When tenant was archived
	RestoreCount int         `json:"restoreCount"`           // Number of times restored
	LastRestore  *time.Time  `json:"lastRestore,omitempty"`  // Last restore time

	// Usage metrics
	RequestsLast24h int64     `json:"requestsLast24h"` // Requests in last 24 hours
	RequestsLast7d  int64     `json:"requestsLast7d"`  // Requests in last 7 days

	// Timestamps
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`
}

// TenantAccessPattern tracks predictable access patterns for a tenant
type TenantAccessPattern struct {
	TenantID      string    `json:"tenantId"`

	// Access schedule
	DayOfWeek     []int     `json:"dayOfWeek"`     // [0-6] where 0=Sunday, bitmap of active days
	HourOfDay     []int     `json:"hourOfDay"`     // [0-23] bitmap of active hours

	// Performance metrics
	AvgSessionDuration time.Duration `json:"avgSessionDuration"` // Average active session length
	AvgRequestsPerSession int        `json:"avgRequestsPerSession"` // Average requests per session

	// Prediction confidence
	PatternConfidence float64   `json:"patternConfidence"` // 0-1, how predictable
	LastPatternUpdate time.Time `json:"lastPatternUpdate"`

	// Timestamps
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}
