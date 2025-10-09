package tenant_node

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/req"
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

// ControlPlaneClient implements the enterprise.ControlPlaneClient interface
type ControlPlaneClient struct {
	controlPlaneAddrs []string
	socket            mangos.Socket
	logger            *log.Logger
}

// NewControlPlaneClient creates a new control plane client
func NewControlPlaneClient(controlPlaneAddrs []string) (*ControlPlaneClient, error) {
	if len(controlPlaneAddrs) == 0 {
		return nil, fmt.Errorf("at least one control plane address required")
	}

	socket, err := req.NewSocket()
	if err != nil {
		return nil, fmt.Errorf("failed to create REQ socket: %w", err)
	}

	// Set socket options
	socket.SetOption(mangos.OptionRecvDeadline, 5*time.Second)
	socket.SetOption(mangos.OptionSendDeadline, 5*time.Second)

	// Connect to all control plane nodes (REQ socket will round-robin)
	for _, addr := range controlPlaneAddrs {
		url := fmt.Sprintf("tcp://%s", addr)
		if err := socket.Dial(url); err != nil {
			log.Printf("[CPClient] Warning: failed to dial %s: %v", url, err)
			// Continue to try other addresses
		}
	}

	return &ControlPlaneClient{
		controlPlaneAddrs: controlPlaneAddrs,
		socket:            socket,
		logger:            log.Default(),
	}, nil
}

// Close closes the control plane client
func (c *ControlPlaneClient) Close() error {
	return c.socket.Close()
}

// request sends a request to the control plane and returns the response
func (c *ControlPlaneClient) request(reqType string, data map[string]interface{}) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"type": reqType,
		"data": data,
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request
	if err := c.socket.Send(reqJSON); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Receive response
	respJSON, err := c.socket.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	var resp struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
		Error   string                 `json:"error"`
	}

	if err := json.Unmarshal(respJSON, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("control plane error: %s", resp.Error)
	}

	return resp.Data, nil
}

// GetTenantMetadata retrieves tenant metadata from control plane
func (c *ControlPlaneClient) GetTenantMetadata(ctx context.Context, tenantID string) (*enterprise.Tenant, error) {
	data, err := c.request("getTenant", map[string]interface{}{
		"tenantId": tenantID,
	})
	if err != nil {
		return nil, err
	}

	tenantData, ok := data["tenant"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tenant data in response")
	}

	// Convert map to Tenant struct
	tenantJSON, _ := json.Marshal(tenantData)
	var tenant enterprise.Tenant
	if err := json.Unmarshal(tenantJSON, &tenant); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tenant: %w", err)
	}

	return &tenant, nil
}

// GetTenantByDomain retrieves tenant by domain name
func (c *ControlPlaneClient) GetTenantByDomain(ctx context.Context, domain string) (*enterprise.Tenant, error) {
	data, err := c.request("getTenantByDomain", map[string]interface{}{
		"domain": domain,
	})
	if err != nil {
		return nil, err
	}

	tenantData, ok := data["tenant"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tenant data in response")
	}

	// Convert map to Tenant struct
	tenantJSON, _ := json.Marshal(tenantData)
	var tenant enterprise.Tenant
	if err := json.Unmarshal(tenantJSON, &tenant); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tenant: %w", err)
	}

	return &tenant, nil
}

// UpdateTenantStatus updates tenant status
func (c *ControlPlaneClient) UpdateTenantStatus(ctx context.Context, tenantID string, status enterprise.TenantStatus) error {
	_, err := c.request("updateTenantStatus", map[string]interface{}{
		"tenantId": tenantID,
		"status":   string(status),
	})
	return err
}

// RegisterNode registers a tenant node with the control plane
func (c *ControlPlaneClient) RegisterNode(ctx context.Context, nodeInfo *enterprise.NodeInfo) error {
	_, err := c.request("registerNode", map[string]interface{}{
		"nodeId":   nodeInfo.ID,
		"address":  nodeInfo.Address,
		"capacity": nodeInfo.Capacity,
	})
	return err
}

// SendHeartbeat sends a heartbeat from this node
func (c *ControlPlaneClient) SendHeartbeat(ctx context.Context, nodeID string, activeTenantsCount int) error {
	_, err := c.request("heartbeat", map[string]interface{}{
		"nodeId":             nodeID,
		"activeTenantsCount": activeTenantsCount,
	})
	return err
}

// GetPlacementDecision requests placement decision for a tenant
func (c *ControlPlaneClient) GetPlacementDecision(ctx context.Context, tenantID string) (*enterprise.PlacementDecision, error) {
	data, err := c.request("assignTenant", map[string]interface{}{
		"tenantId": tenantID,
	})
	if err != nil {
		return nil, err
	}

	decisionData, ok := data["decision"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid decision data in response")
	}

	// Convert map to PlacementDecision struct
	decisionJSON, _ := json.Marshal(decisionData)
	var decision enterprise.PlacementDecision
	if err := json.Unmarshal(decisionJSON, &decision); err != nil {
		return nil, fmt.Errorf("failed to unmarshal decision: %w", err)
	}

	return &decision, nil
}
