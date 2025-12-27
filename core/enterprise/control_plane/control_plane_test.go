package control_plane

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
)

func TestNewControlPlaneValidMode(t *testing.T) {
	config := &enterprise.ClusterConfig{
		Mode:    enterprise.ModeControlPlane,
		NodeID:  "cp-1",
		DataDir: t.TempDir(),
	}

	cp, err := NewControlPlane(config)
	if err != nil {
		t.Fatalf("failed to create control plane: %v", err)
	}

	if cp == nil {
		t.Fatal("expected non-nil control plane")
	}

	if cp.healthChecker == nil {
		t.Error("expected non-nil health checker")
	}
}

func TestNewControlPlaneAllInOneMode(t *testing.T) {
	config := &enterprise.ClusterConfig{
		Mode:    enterprise.ModeAllInOne,
		NodeID:  "aio-1",
		DataDir: t.TempDir(),
	}

	cp, err := NewControlPlane(config)
	if err != nil {
		t.Fatalf("failed to create control plane for AllInOne mode: %v", err)
	}

	if cp == nil {
		t.Fatal("expected non-nil control plane")
	}
}

func TestNewControlPlaneInvalidMode(t *testing.T) {
	tests := []struct {
		name string
		mode enterprise.Mode
	}{
		{"Gateway mode", enterprise.ModeGateway},
		{"TenantNode mode", enterprise.ModeTenantNode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &enterprise.ClusterConfig{
				Mode:    tt.mode,
				NodeID:  "test-1",
				DataDir: t.TempDir(),
			}

			_, err := NewControlPlane(config)
			if err == nil {
				t.Errorf("expected error for invalid mode %s", tt.mode)
			}
		})
	}
}

func TestControlPlaneGetHealthChecker(t *testing.T) {
	config := &enterprise.ClusterConfig{
		Mode:    enterprise.ModeControlPlane,
		NodeID:  "cp-1",
		DataDir: t.TempDir(),
	}

	cp, _ := NewControlPlane(config)

	checker := cp.GetHealthChecker()
	if checker == nil {
		t.Error("expected non-nil health checker")
	}
}

func TestControlPlaneGetNodesEmpty(t *testing.T) {
	config := &enterprise.ClusterConfig{
		Mode:    enterprise.ModeControlPlane,
		NodeID:  "cp-1",
		DataDir: t.TempDir(),
	}

	cp, _ := NewControlPlane(config)

	nodes := cp.GetNodes()
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestControlPlaneGetDiskStatsNotStarted(t *testing.T) {
	config := &enterprise.ClusterConfig{
		Mode:    enterprise.ModeControlPlane,
		NodeID:  "cp-1",
		DataDir: t.TempDir(),
	}

	cp, _ := NewControlPlane(config)

	// Without Start(), storage is nil
	stats := cp.GetDiskStats()
	if len(stats) != 0 {
		t.Errorf("expected empty stats when not started, got %v", stats)
	}
}

func TestControlPlaneStopWithoutStart(t *testing.T) {
	config := &enterprise.ClusterConfig{
		Mode:    enterprise.ModeControlPlane,
		NodeID:  "cp-1",
		DataDir: t.TempDir(),
	}

	cp, _ := NewControlPlane(config)

	// Stop without Start should not panic
	err := cp.Stop()
	if err != nil {
		t.Errorf("unexpected error stopping unstarted control plane: %v", err)
	}
}

// IPCRequest and IPCResponse tests

func TestIPCRequestSerialization(t *testing.T) {
	req := IPCRequest{
		Type: "getTenant",
		Data: map[string]interface{}{
			"tenantId": "tenant-123",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var decoded IPCRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if decoded.Type != req.Type {
		t.Errorf("expected type %s, got %s", req.Type, decoded.Type)
	}

	tenantID, ok := decoded.Data["tenantId"].(string)
	if !ok || tenantID != "tenant-123" {
		t.Errorf("expected tenantId tenant-123, got %v", decoded.Data["tenantId"])
	}
}

func TestIPCResponseSerialization(t *testing.T) {
	resp := IPCResponse{
		Success: true,
		Data: map[string]interface{}{
			"tenant": map[string]interface{}{
				"id":     "tenant-123",
				"domain": "test.example.com",
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded IPCResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if decoded.Success != resp.Success {
		t.Errorf("expected success %v, got %v", resp.Success, decoded.Success)
	}
}

func TestIPCResponseErrorSerialization(t *testing.T) {
	resp := IPCResponse{
		Success: false,
		Error:   "tenant not found",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal error response: %v", err)
	}

	var decoded IPCResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if decoded.Success != false {
		t.Error("expected success false")
	}

	if decoded.Error != "tenant not found" {
		t.Errorf("expected error 'tenant not found', got %s", decoded.Error)
	}
}

// RaftCommand tests - extending what's in storage_test.go

func TestRaftCommandAllTypes(t *testing.T) {
	tests := []struct {
		name    string
		cmdType CommandType
		payload interface{}
	}{
		{
			name:    "CreateTenant",
			cmdType: CommandCreateTenant,
			payload: CreateTenantPayload{
				Tenant: &enterprise.Tenant{
					ID:     "tenant-1",
					Domain: "test.example.com",
				},
			},
		},
		{
			name:    "UpdateTenant",
			cmdType: CommandUpdateTenant,
			payload: UpdateTenantPayload{
				Tenant: &enterprise.Tenant{
					ID:     "tenant-1",
					Domain: "updated.example.com",
				},
			},
		},
		{
			name:    "UpdateTenantStatus",
			cmdType: CommandUpdateTenantStatus,
			payload: UpdateTenantStatusPayload{
				TenantID: "tenant-1",
				Status:   enterprise.TenantStatusActive,
			},
		},
		{
			name:    "CreateUser",
			cmdType: CommandCreateUser,
			payload: CreateUserPayload{
				User: &enterprise.ClusterUser{
					ID:    "user-1",
					Email: "test@example.com",
				},
			},
		},
		{
			name:    "UpdateUser",
			cmdType: CommandUpdateUser,
			payload: UpdateUserPayload{
				User: &enterprise.ClusterUser{
					ID:       "user-1",
					Email:    "test@example.com",
					Verified: true,
				},
			},
		},
		{
			name:    "SaveNode",
			cmdType: CommandSaveNode,
			payload: SaveNodePayload{
				Node: &enterprise.NodeInfo{
					ID:       "node-1",
					Address:  "localhost:8091",
					Status:   "online",
					Capacity: 10,
				},
			},
		},
		{
			name:    "SavePlacement",
			cmdType: CommandSavePlacement,
			payload: SavePlacementPayload{
				Placement: &enterprise.PlacementDecision{
					TenantID:    "tenant-1",
					NodeID:      "node-1",
					NodeAddress: "localhost:8091",
					Reason:      "least-loaded",
					DecidedAt:   time.Now(),
				},
			},
		},
		{
			name:    "SaveActivity",
			cmdType: CommandSaveActivity,
			payload: SaveActivityPayload{
				Activity: &enterprise.TenantActivity{
					TenantID:    "tenant-1",
					LastAccess:  time.Now(),
					AccessCount: 100,
					StorageTier: enterprise.StorageTierHot,
				},
			},
		},
		{
			name:    "SaveToken",
			cmdType: CommandSaveToken,
			payload: SaveTokenPayload{
				Token: &enterprise.VerificationToken{
					Token:   "abc123",
					UserID:  "user-1",
					Email:   "test@example.com",
					Expires: time.Now().Add(24 * time.Hour),
				},
			},
		},
		{
			name:    "MarkTokenUsed",
			cmdType: CommandMarkTokenUsed,
			payload: MarkTokenUsedPayload{
				Token: "abc123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewRaftCommand(tt.cmdType, tt.payload)
			if err != nil {
				t.Fatalf("failed to create command: %v", err)
			}

			if cmd.Type != tt.cmdType {
				t.Errorf("expected type %s, got %s", tt.cmdType, cmd.Type)
			}

			// Encode and decode
			encoded, err := cmd.Encode()
			if err != nil {
				t.Fatalf("failed to encode command: %v", err)
			}

			decoded, err := DecodeRaftCommand(encoded)
			if err != nil {
				t.Fatalf("failed to decode command: %v", err)
			}

			if decoded.Type != tt.cmdType {
				t.Errorf("after decode: expected type %s, got %s", tt.cmdType, decoded.Type)
			}
		})
	}
}

func TestDecodeRaftCommandInvalidJSON(t *testing.T) {
	_, err := DecodeRaftCommand([]byte("not valid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDecodeRaftCommandEmptyPayload(t *testing.T) {
	cmd := &RaftCommand{
		Type:    CommandCreateTenant,
		Payload: json.RawMessage(`{}`),
	}

	encoded, err := cmd.Encode()
	if err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	decoded, err := DecodeRaftCommand(encoded)
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if decoded.Type != CommandCreateTenant {
		t.Errorf("expected CommandCreateTenant, got %s", decoded.Type)
	}
}

// CommandType validation tests

func TestCommandTypeConstants(t *testing.T) {
	// Verify all command type constants are unique
	types := map[CommandType]bool{
		CommandCreateTenant:       true,
		CommandUpdateTenant:       true,
		CommandUpdateTenantStatus: true,
		CommandCreateUser:         true,
		CommandUpdateUser:         true,
		CommandSaveNode:           true,
		CommandSavePlacement:      true,
		CommandSaveActivity:       true,
		CommandSaveToken:          true,
		CommandMarkTokenUsed:      true,
	}

	if len(types) != 10 {
		t.Error("expected 10 unique command types")
	}
}

// Payload serialization tests

func TestCreateTenantPayloadSerialization(t *testing.T) {
	payload := CreateTenantPayload{
		Tenant: &enterprise.Tenant{
			ID:               "tenant-1",
			Domain:           "test.example.com",
			OwnerUserID:      "user-1",
			Status:           enterprise.TenantStatusActive,
			StorageQuotaMB:   1024,
			APIRequestsQuota: 10000,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded CreateTenantPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Tenant.ID != payload.Tenant.ID {
		t.Errorf("expected ID %s, got %s", payload.Tenant.ID, decoded.Tenant.ID)
	}

	if decoded.Tenant.Domain != payload.Tenant.Domain {
		t.Errorf("expected Domain %s, got %s", payload.Tenant.Domain, decoded.Tenant.Domain)
	}
}

func TestUpdateTenantStatusPayloadSerialization(t *testing.T) {
	payload := UpdateTenantStatusPayload{
		TenantID: "tenant-1",
		Status:   enterprise.TenantStatusIdle,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded UpdateTenantStatusPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.TenantID != payload.TenantID {
		t.Errorf("expected TenantID %s, got %s", payload.TenantID, decoded.TenantID)
	}

	if decoded.Status != payload.Status {
		t.Errorf("expected Status %s, got %s", payload.Status, decoded.Status)
	}
}

func TestSaveNodePayloadSerialization(t *testing.T) {
	now := time.Now()
	payload := SaveNodePayload{
		Node: &enterprise.NodeInfo{
			ID:            "node-1",
			Address:       "localhost:8091",
			Status:        "online",
			Capacity:      10,
			ActiveTenants: 5,
			LastHeartbeat: now,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded SaveNodePayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Node.ID != payload.Node.ID {
		t.Errorf("expected ID %s, got %s", payload.Node.ID, decoded.Node.ID)
	}

	if decoded.Node.Capacity != payload.Node.Capacity {
		t.Errorf("expected Capacity %d, got %d", payload.Node.Capacity, decoded.Node.Capacity)
	}
}

func TestSavePlacementPayloadSerialization(t *testing.T) {
	now := time.Now()
	payload := SavePlacementPayload{
		Placement: &enterprise.PlacementDecision{
			TenantID:    "tenant-1",
			NodeID:      "node-1",
			NodeAddress: "localhost:8091",
			Reason:      "least-loaded",
			DecidedAt:   now,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded SavePlacementPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Placement.TenantID != payload.Placement.TenantID {
		t.Errorf("expected TenantID %s, got %s", payload.Placement.TenantID, decoded.Placement.TenantID)
	}

	if decoded.Placement.Reason != payload.Placement.Reason {
		t.Errorf("expected Reason %s, got %s", payload.Placement.Reason, decoded.Placement.Reason)
	}
}

func TestSaveActivityPayloadSerialization(t *testing.T) {
	now := time.Now()
	payload := SaveActivityPayload{
		Activity: &enterprise.TenantActivity{
			TenantID:        "tenant-1",
			LastAccess:      now,
			AccessCount:     1000,
			StorageTier:     enterprise.StorageTierWarm,
			RequestsLast24h: 500,
			RequestsLast7d:  3000,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded SaveActivityPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Activity.TenantID != payload.Activity.TenantID {
		t.Errorf("expected TenantID %s, got %s", payload.Activity.TenantID, decoded.Activity.TenantID)
	}

	if decoded.Activity.StorageTier != payload.Activity.StorageTier {
		t.Errorf("expected StorageTier %s, got %s", payload.Activity.StorageTier, decoded.Activity.StorageTier)
	}
}

func TestCreateUserPayloadSerialization(t *testing.T) {
	payload := CreateUserPayload{
		User: &enterprise.ClusterUser{
			ID:         "user-1",
			Email:      "test@example.com",
			Name:       "Test User",
			Verified:   false,
			MaxTenants: 5,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded CreateUserPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.User.ID != payload.User.ID {
		t.Errorf("expected ID %s, got %s", payload.User.ID, decoded.User.ID)
	}

	if decoded.User.MaxTenants != payload.User.MaxTenants {
		t.Errorf("expected MaxTenants %d, got %d", payload.User.MaxTenants, decoded.User.MaxTenants)
	}
}

func TestSaveTokenPayloadSerialization(t *testing.T) {
	expires := time.Now().Add(24 * time.Hour)
	created := time.Now()
	payload := SaveTokenPayload{
		Token: &enterprise.VerificationToken{
			Token:   "verification-token-123",
			UserID:  "user-1",
			Email:   "test@example.com",
			Expires: expires,
			Created: created,
			Used:    false,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded SaveTokenPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Token.Token != payload.Token.Token {
		t.Errorf("expected Token %s, got %s", payload.Token.Token, decoded.Token.Token)
	}

	if decoded.Token.UserID != payload.Token.UserID {
		t.Errorf("expected UserID %s, got %s", payload.Token.UserID, decoded.Token.UserID)
	}

	if decoded.Token.Used != payload.Token.Used {
		t.Errorf("expected Used %v, got %v", payload.Token.Used, decoded.Token.Used)
	}
}

func TestMarkTokenUsedPayloadSerialization(t *testing.T) {
	payload := MarkTokenUsedPayload{
		Token: "verification-token-123",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded MarkTokenUsedPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Token != payload.Token {
		t.Errorf("expected Token %s, got %s", payload.Token, decoded.Token)
	}
}

// IPC request type tests

func TestIPCRequestTypes(t *testing.T) {
	tests := []struct {
		reqType  string
		dataKeys []string
	}{
		{"getTenant", []string{"tenantId"}},
		{"getTenantByDomain", []string{"domain"}},
		{"assignTenant", []string{"tenantId"}},
		{"registerNode", []string{"nodeId", "address", "capacity"}},
		{"heartbeat", []string{"nodeId", "activeTenantsCount"}},
	}

	for _, tt := range tests {
		t.Run(tt.reqType, func(t *testing.T) {
			data := make(map[string]interface{})
			for _, key := range tt.dataKeys {
				data[key] = "test-value"
			}

			req := IPCRequest{
				Type: tt.reqType,
				Data: data,
			}

			encoded, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			var decoded IPCRequest
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if decoded.Type != tt.reqType {
				t.Errorf("expected type %s, got %s", tt.reqType, decoded.Type)
			}

			for _, key := range tt.dataKeys {
				if _, ok := decoded.Data[key]; !ok {
					t.Errorf("expected key %s in data", key)
				}
			}
		})
	}
}

// Edge cases

func TestNewRaftCommandNilPayload(t *testing.T) {
	// nil struct should marshal to null
	_, err := NewRaftCommand(CommandCreateTenant, CreateTenantPayload{})
	if err != nil {
		t.Errorf("unexpected error for empty payload: %v", err)
	}
}

func TestIPCResponseWithNilData(t *testing.T) {
	resp := IPCResponse{
		Success: true,
		Data:    nil,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded IPCResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !decoded.Success {
		t.Error("expected success true")
	}
}

func TestControlPlaneConfigValidation(t *testing.T) {
	// Empty DataDir should still work (will use current directory)
	config := &enterprise.ClusterConfig{
		Mode:   enterprise.ModeControlPlane,
		NodeID: "cp-1",
	}

	cp, err := NewControlPlane(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cp == nil {
		t.Fatal("expected non-nil control plane")
	}
}
