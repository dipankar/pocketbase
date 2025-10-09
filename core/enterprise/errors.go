package enterprise

import (
	"errors"
	"fmt"
)

// Common enterprise errors
var (
	// Tenant errors
	ErrTenantNotFound      = errors.New("tenant not found")
	ErrTenantAlreadyExists = errors.New("tenant already exists")
	ErrTenantNotAssigned   = errors.New("tenant not assigned to any node")
	ErrTenantOffline       = errors.New("tenant is offline")
	ErrTenantOverQuota     = errors.New("tenant over quota")

	// Node errors
	ErrNodeNotFound       = errors.New("node not found")
	ErrNodeAtCapacity     = errors.New("node at capacity")
	ErrNodeOffline        = errors.New("node is offline")
	ErrNoHealthyNodes     = errors.New("no healthy nodes available")

	// Control plane errors
	ErrNotLeader          = errors.New("not the raft leader")
	ErrControlPlaneDown   = errors.New("control plane unavailable")
	ErrPlacementFailed    = errors.New("placement failed")

	// User errors
	ErrUserNotFound       = errors.New("cluster user not found")
	ErrUserAlreadyExists  = errors.New("cluster user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotVerified    = errors.New("user email not verified")
	ErrUserOverQuota      = errors.New("user over quota")

	// Auth errors
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrUnauthorized       = errors.New("unauthorized")

	// Storage errors
	ErrS3DownloadFailed   = errors.New("S3 download failed")
	ErrS3UploadFailed     = errors.New("S3 upload failed")
	ErrS3DeleteFailed     = errors.New("S3 delete failed")
	ErrLitestreamFailed   = errors.New("litestream replication failed")

	// Mode errors
	ErrInvalidMode        = errors.New("invalid mode")
	ErrModeNotSupported   = errors.New("operation not supported in this mode")
)

// TenantError wraps an error with tenant context
type TenantError struct {
	TenantID string
	Err      error
}

func (e *TenantError) Error() string {
	return fmt.Sprintf("tenant %s: %v", e.TenantID, e.Err)
}

func (e *TenantError) Unwrap() error {
	return e.Err
}

// NewTenantError creates a new tenant-specific error
func NewTenantError(tenantID string, err error) *TenantError {
	return &TenantError{
		TenantID: tenantID,
		Err:      err,
	}
}

// NodeError wraps an error with node context
type NodeError struct {
	NodeID string
	Err    error
}

func (e *NodeError) Error() string {
	return fmt.Sprintf("node %s: %v", e.NodeID, e.Err)
}

func (e *NodeError) Unwrap() error {
	return e.Err
}

// NewNodeError creates a new node-specific error
func NewNodeError(nodeID string, err error) *NodeError {
	return &NodeError{
		NodeID: nodeID,
		Err:    err,
	}
}

// QuotaError represents a quota violation error
type QuotaError struct {
	Resource string
	Current  int64
	Limit    int64
}

func (e *QuotaError) Error() string {
	return fmt.Sprintf("quota exceeded for %s: %d/%d", e.Resource, e.Current, e.Limit)
}

// NewQuotaError creates a new quota error
func NewQuotaError(resource string, current, limit int64) *QuotaError {
	return &QuotaError{
		Resource: resource,
		Current:  current,
		Limit:    limit,
	}
}
