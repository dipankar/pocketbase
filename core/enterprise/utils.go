package enterprise

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// GenerateID generates a unique ID with a prefix
// Examples: tenant_abc123, user_xyz789, node_def456
func GenerateID(prefix string) string {
	randomBytes := make([]byte, 12)
	rand.Read(randomBytes)
	encoded := base64.URLEncoding.EncodeToString(randomBytes)
	// Remove padding and make URL-safe
	encoded = strings.TrimRight(encoded, "=")
	encoded = strings.ReplaceAll(encoded, "+", "")
	encoded = strings.ReplaceAll(encoded, "/", "")
	return fmt.Sprintf("%s_%s", prefix, encoded[:12])
}

// GenerateTenantID generates a tenant ID
func GenerateTenantID() string {
	return GenerateID("tenant")
}

// GenerateUserID generates a cluster user ID
func GenerateUserID() string {
	return GenerateID("user")
}

// GenerateNodeID generates a node ID
func GenerateNodeID() string {
	return GenerateID("node")
}

// GenerateAdminToken generates a long-lived admin token
func GenerateAdminToken() string {
	randomBytes := make([]byte, 32)
	rand.Read(randomBytes)
	encoded := base64.URLEncoding.EncodeToString(randomBytes)
	encoded = strings.TrimRight(encoded, "=")
	return fmt.Sprintf("admin_%s", encoded[:40])
}

// GenerateSessionToken generates a session token for cluster users
func GenerateSessionToken() string {
	randomBytes := make([]byte, 24)
	rand.Read(randomBytes)
	encoded := base64.URLEncoding.EncodeToString(randomBytes)
	encoded = strings.TrimRight(encoded, "=")
	return encoded
}

// ExtractTenantIDFromDomain extracts tenant ID from domain
// Example: tenant123.platform.com -> tenant123
func ExtractTenantIDFromDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// IsValidMode checks if a mode string is valid
func IsValidMode(mode string) bool {
	m := Mode(mode)
	return m == ModeControlPlane || m == ModeTenantNode || m == ModeGateway || m == ModeAllInOne
}

// GetS3TenantPrefix returns the S3 prefix for a tenant
// Example: tenants/tenant_abc123/
func GetS3TenantPrefix(tenantID string) string {
	return fmt.Sprintf("tenants/%s/", tenantID)
}

// GetS3DatabasePath returns the S3 path for a tenant database
// Example: tenants/tenant_abc123/litestream/data.db/
func GetS3DatabasePath(tenantID, dbName string) string {
	return fmt.Sprintf("tenants/%s/litestream/%s/", tenantID, dbName)
}

// IsNodeHealthy checks if a node is healthy based on last heartbeat
func IsNodeHealthy(node *NodeInfo, heartbeatTimeout time.Duration) bool {
	if node == nil {
		return false
	}
	if node.Status != "online" {
		return false
	}
	return time.Since(node.LastHeartbeat) <= heartbeatTimeout
}

// CalculateNodeLoad calculates the load percentage of a node
func CalculateNodeLoad(node *NodeInfo) float64 {
	if node == nil || node.Capacity == 0 {
		return 0
	}
	return float64(node.ActiveTenants) / float64(node.Capacity) * 100
}

// SelectLeastLoadedNode selects the node with the lowest load
func SelectLeastLoadedNode(nodes []*NodeInfo) *NodeInfo {
	if len(nodes) == 0 {
		return nil
	}

	var bestNode *NodeInfo
	var lowestLoad float64 = 100

	for _, node := range nodes {
		if !IsNodeHealthy(node, 30*time.Second) {
			continue
		}
		if node.ActiveTenants >= node.Capacity {
			continue // Skip full nodes
		}

		load := CalculateNodeLoad(node)
		if bestNode == nil || load < lowestLoad {
			bestNode = node
			lowestLoad = load
		}
	}

	return bestNode
}

// ParseMode parses a mode string into a Mode type
func ParseMode(modeStr string) (Mode, error) {
	mode := Mode(modeStr)
	if !IsValidMode(modeStr) {
		return "", ErrInvalidMode
	}
	return mode, nil
}

// MustParseMode parses a mode string and panics on error
func MustParseMode(modeStr string) Mode {
	mode, err := ParseMode(modeStr)
	if err != nil {
		panic(err)
	}
	return mode
}
