package control_plane

import (
	"encoding/json"
	"fmt"

	"github.com/pocketbase/pocketbase/core/enterprise"
)

// CommandType represents the type of Raft command
type CommandType string

const (
	CommandCreateTenant       CommandType = "create_tenant"
	CommandUpdateTenant       CommandType = "update_tenant"
	CommandUpdateTenantStatus CommandType = "update_tenant_status"
	CommandCreateUser         CommandType = "create_user"
	CommandUpdateUser         CommandType = "update_user"
	CommandSaveNode           CommandType = "save_node"
	CommandSavePlacement      CommandType = "save_placement"
	CommandSaveActivity       CommandType = "save_activity"
	CommandSaveToken          CommandType = "save_token"
	CommandMarkTokenUsed      CommandType = "mark_token_used"
)

// RaftCommand represents a command to be replicated via Raft
type RaftCommand struct {
	Type    CommandType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// CreateTenantPayload is the payload for creating a tenant
type CreateTenantPayload struct {
	Tenant *enterprise.Tenant `json:"tenant"`
}

// UpdateTenantPayload is the payload for updating a tenant
type UpdateTenantPayload struct {
	Tenant *enterprise.Tenant `json:"tenant"`
}

// UpdateTenantStatusPayload is the payload for updating tenant status
type UpdateTenantStatusPayload struct {
	TenantID string                  `json:"tenantId"`
	Status   enterprise.TenantStatus `json:"status"`
}

// CreateUserPayload is the payload for creating a user
type CreateUserPayload struct {
	User *enterprise.ClusterUser `json:"user"`
}

// UpdateUserPayload is the payload for updating a user
type UpdateUserPayload struct {
	User *enterprise.ClusterUser `json:"user"`
}

// SaveNodePayload is the payload for saving node info
type SaveNodePayload struct {
	Node *enterprise.NodeInfo `json:"node"`
}

// SavePlacementPayload is the payload for saving placement decision
type SavePlacementPayload struct {
	Placement *enterprise.PlacementDecision `json:"placement"`
}

// SaveActivityPayload is the payload for saving tenant activity
type SaveActivityPayload struct {
	Activity *enterprise.TenantActivity `json:"activity"`
}

// SaveTokenPayload is the payload for saving a verification token
type SaveTokenPayload struct {
	Token *enterprise.VerificationToken `json:"token"`
}

// MarkTokenUsedPayload is the payload for marking a token as used
type MarkTokenUsedPayload struct {
	Token string `json:"token"`
}

// NewRaftCommand creates a new Raft command with the given type and payload
func NewRaftCommand(cmdType CommandType, payload interface{}) (*RaftCommand, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	return &RaftCommand{
		Type:    cmdType,
		Payload: data,
	}, nil
}

// Encode encodes the command to JSON bytes
func (c *RaftCommand) Encode() ([]byte, error) {
	return json.Marshal(c)
}

// DecodeRaftCommand decodes a Raft command from JSON bytes
func DecodeRaftCommand(data []byte) (*RaftCommand, error) {
	var cmd RaftCommand
	if err := json.Unmarshal(data, &cmd); err != nil {
		return nil, fmt.Errorf("failed to unmarshal command: %w", err)
	}
	return &cmd, nil
}
