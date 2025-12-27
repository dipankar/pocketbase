package placement

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
)

// mockStorage implements the Storage interface for testing
type mockStorage struct {
	tenants    map[string]*enterprise.Tenant
	nodes      map[string]*enterprise.NodeInfo
	placements map[string]*enterprise.PlacementDecision
	nodeTenants map[string][]*enterprise.Tenant
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		tenants:     make(map[string]*enterprise.Tenant),
		nodes:       make(map[string]*enterprise.NodeInfo),
		placements:  make(map[string]*enterprise.PlacementDecision),
		nodeTenants: make(map[string][]*enterprise.Tenant),
	}
}

func (m *mockStorage) GetTenant(tenantID string) (*enterprise.Tenant, error) {
	t, exists := m.tenants[tenantID]
	if !exists {
		return nil, enterprise.ErrTenantNotFound
	}
	return t, nil
}

func (m *mockStorage) UpdateTenant(tenant *enterprise.Tenant) error {
	m.tenants[tenant.ID] = tenant
	return nil
}

func (m *mockStorage) ListNodes() ([]*enterprise.NodeInfo, error) {
	nodes := make([]*enterprise.NodeInfo, 0, len(m.nodes))
	for _, n := range m.nodes {
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (m *mockStorage) SavePlacement(placement *enterprise.PlacementDecision) error {
	m.placements[placement.TenantID] = placement
	return nil
}

func (m *mockStorage) GetPlacement(tenantID string) (*enterprise.PlacementDecision, error) {
	p, exists := m.placements[tenantID]
	if !exists {
		return nil, enterprise.ErrTenantNotAssigned
	}
	return p, nil
}

func (m *mockStorage) ListTenantsByNode(nodeID string) ([]*enterprise.Tenant, error) {
	tenants, exists := m.nodeTenants[nodeID]
	if !exists {
		return []*enterprise.Tenant{}, nil
	}
	return tenants, nil
}

func (m *mockStorage) addNode(id, address string, capacity, activeTenants int) {
	m.nodes[id] = &enterprise.NodeInfo{
		ID:            id,
		Address:       address,
		Status:        "online",
		Capacity:      capacity,
		ActiveTenants: activeTenants,
		LastHeartbeat: time.Now(),
	}
}

func (m *mockStorage) addTenant(id, nodeID string) {
	m.tenants[id] = &enterprise.Tenant{
		ID:             id,
		AssignedNodeID: nodeID,
	}
	if nodeID != "" {
		m.nodeTenants[nodeID] = append(m.nodeTenants[nodeID], m.tenants[id])
	}
}

func TestLeastLoadedStrategySelectNode(t *testing.T) {
	storage := newMockStorage()
	storage.addNode("node-1", "localhost:8091", 10, 5)
	storage.addNode("node-2", "localhost:8092", 10, 2) // Least loaded
	storage.addNode("node-3", "localhost:8093", 10, 8)

	strategy := &LeastLoadedStrategy{}
	nodes, _ := storage.ListNodes()

	tenant := &enterprise.Tenant{ID: "tenant-1"}

	selected, err := strategy.SelectNode(tenant, nodes)
	if err != nil {
		t.Fatalf("failed to select node: %v", err)
	}

	if selected.ID != "node-2" {
		t.Errorf("expected node-2 (least loaded), got %s", selected.ID)
	}
}

func TestLeastLoadedStrategySkipsFullNodes(t *testing.T) {
	storage := newMockStorage()
	storage.addNode("node-1", "localhost:8091", 10, 10) // Full
	storage.addNode("node-2", "localhost:8092", 10, 5)  // Has capacity

	strategy := &LeastLoadedStrategy{}
	nodes, _ := storage.ListNodes()

	tenant := &enterprise.Tenant{ID: "tenant-1"}

	selected, err := strategy.SelectNode(tenant, nodes)
	if err != nil {
		t.Fatalf("failed to select node: %v", err)
	}

	if selected.ID != "node-2" {
		t.Errorf("expected node-2 (not full), got %s", selected.ID)
	}
}

func TestLeastLoadedStrategySkipsOfflineNodes(t *testing.T) {
	storage := newMockStorage()
	storage.addNode("node-1", "localhost:8091", 10, 2)
	storage.nodes["node-1"].Status = "offline"
	storage.addNode("node-2", "localhost:8092", 10, 5)

	strategy := &LeastLoadedStrategy{}
	nodes, _ := storage.ListNodes()

	tenant := &enterprise.Tenant{ID: "tenant-1"}

	selected, err := strategy.SelectNode(tenant, nodes)
	if err != nil {
		t.Fatalf("failed to select node: %v", err)
	}

	if selected.ID != "node-2" {
		t.Errorf("expected node-2 (online), got %s", selected.ID)
	}
}

func TestLeastLoadedStrategyNoHealthyNodes(t *testing.T) {
	storage := newMockStorage()
	storage.addNode("node-1", "localhost:8091", 10, 10)
	storage.nodes["node-1"].Status = "offline"

	strategy := &LeastLoadedStrategy{}
	nodes, _ := storage.ListNodes()

	tenant := &enterprise.Tenant{ID: "tenant-1"}

	_, err := strategy.SelectNode(tenant, nodes)
	if err != enterprise.ErrNoHealthyNodes {
		t.Errorf("expected ErrNoHealthyNodes, got %v", err)
	}
}

func TestShouldRebalance(t *testing.T) {
	strategy := &LeastLoadedStrategy{}

	tests := []struct {
		name     string
		nodes    []*enterprise.NodeInfo
		expected bool
	}{
		{
			name:     "single node - no rebalance",
			nodes:    []*enterprise.NodeInfo{{ID: "n1", Status: "online", LastHeartbeat: time.Now(), Capacity: 10, ActiveTenants: 5}},
			expected: false,
		},
		{
			name: "balanced load - no rebalance",
			nodes: []*enterprise.NodeInfo{
				{ID: "n1", Status: "online", LastHeartbeat: time.Now(), Capacity: 10, ActiveTenants: 5},
				{ID: "n2", Status: "online", LastHeartbeat: time.Now(), Capacity: 10, ActiveTenants: 5},
			},
			expected: false,
		},
		{
			name: "imbalanced load - needs rebalance",
			nodes: []*enterprise.NodeInfo{
				{ID: "n1", Status: "online", LastHeartbeat: time.Now(), Capacity: 10, ActiveTenants: 9},
				{ID: "n2", Status: "online", LastHeartbeat: time.Now(), Capacity: 10, ActiveTenants: 1},
			},
			expected: true, // 90% - 10% = 80% variance > 30%
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.ShouldRebalance(tt.nodes)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGenerateRebalancePlan(t *testing.T) {
	storage := newMockStorage()

	// Node 1: overloaded with 8 tenants
	storage.addNode("node-1", "localhost:8091", 10, 8)
	for i := 0; i < 8; i++ {
		storage.addTenant("tenant-1-"+string(rune('a'+i)), "node-1")
	}

	// Node 2: underloaded with 2 tenants
	storage.addNode("node-2", "localhost:8092", 10, 2)
	for i := 0; i < 2; i++ {
		storage.addTenant("tenant-2-"+string(rune('a'+i)), "node-2")
	}

	strategy := &LeastLoadedStrategy{}
	nodes, _ := storage.ListNodes()

	nodeTenants := make(map[string][]*enterprise.Tenant)
	for _, node := range nodes {
		tenants, _ := storage.ListTenantsByNode(node.ID)
		nodeTenants[node.ID] = tenants
	}

	plan, err := strategy.GenerateRebalancePlan(nodes, nodeTenants)
	if err != nil {
		t.Fatalf("failed to generate rebalance plan: %v", err)
	}

	// Should move some tenants from node-1 to node-2
	if len(plan) == 0 {
		t.Error("expected at least one rebalance decision")
	}

	for _, decision := range plan {
		if decision.NodeID != "node-2" {
			t.Errorf("expected tenants to move to node-2, got %s", decision.NodeID)
		}
	}
}

func TestGenerateRebalancePlanNoRebalanceNeeded(t *testing.T) {
	storage := newMockStorage()

	// Both nodes have equal load
	storage.addNode("node-1", "localhost:8091", 10, 5)
	for i := 0; i < 5; i++ {
		storage.addTenant("tenant-1-"+string(rune('a'+i)), "node-1")
	}

	storage.addNode("node-2", "localhost:8092", 10, 5)
	for i := 0; i < 5; i++ {
		storage.addTenant("tenant-2-"+string(rune('a'+i)), "node-2")
	}

	strategy := &LeastLoadedStrategy{}
	nodes, _ := storage.ListNodes()

	nodeTenants := make(map[string][]*enterprise.Tenant)
	for _, node := range nodes {
		tenants, _ := storage.ListTenantsByNode(node.ID)
		nodeTenants[node.ID] = tenants
	}

	plan, err := strategy.GenerateRebalancePlan(nodes, nodeTenants)
	if err != nil {
		t.Fatalf("failed to generate rebalance plan: %v", err)
	}

	// No rebalance needed when load is balanced
	if len(plan) != 0 {
		t.Errorf("expected no rebalance decisions, got %d", len(plan))
	}
}

func TestAssignTenantReturnsExistingPlacement(t *testing.T) {
	storage := newMockStorage()
	storage.addNode("node-1", "localhost:8091", 10, 0)
	storage.addTenant("tenant-1", "node-1")
	storage.placements["tenant-1"] = &enterprise.PlacementDecision{
		TenantID:    "tenant-1",
		NodeID:      "node-1",
		NodeAddress: "localhost:8091",
	}

	service := NewService(storage, nil)

	decision, err := service.AssignTenant("tenant-1")
	if err != nil {
		t.Fatalf("failed to assign tenant: %v", err)
	}

	if decision.NodeID != "node-1" {
		t.Errorf("expected existing placement node-1, got %s", decision.NodeID)
	}
}

func TestAssignTenantCreatesNewPlacement(t *testing.T) {
	storage := newMockStorage()
	storage.addNode("node-1", "localhost:8091", 10, 0)
	storage.addTenant("tenant-1", "")

	service := NewService(storage, nil)

	decision, err := service.AssignTenant("tenant-1")
	if err != nil {
		t.Fatalf("failed to assign tenant: %v", err)
	}

	if decision.NodeID != "node-1" {
		t.Errorf("expected node-1, got %s", decision.NodeID)
	}

	// Verify placement was saved
	saved, _ := storage.GetPlacement("tenant-1")
	if saved == nil {
		t.Error("expected placement to be saved")
	}
}
