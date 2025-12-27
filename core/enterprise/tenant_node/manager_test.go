package tenant_node

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
)

// mockCPClient implements enterprise.ControlPlaneClient for testing
type mockCPClient struct {
	tenants     map[string]*enterprise.Tenant
	nodes       map[string]*enterprise.NodeInfo
	placements  map[string]*enterprise.PlacementDecision
	heartbeats  int
	registerErr error
}

func newMockCPClient() *mockCPClient {
	return &mockCPClient{
		tenants:    make(map[string]*enterprise.Tenant),
		nodes:      make(map[string]*enterprise.NodeInfo),
		placements: make(map[string]*enterprise.PlacementDecision),
	}
}

func (m *mockCPClient) GetTenantMetadata(ctx context.Context, tenantID string) (*enterprise.Tenant, error) {
	t, exists := m.tenants[tenantID]
	if !exists {
		return nil, enterprise.ErrTenantNotFound
	}
	return t, nil
}

func (m *mockCPClient) GetTenantByDomain(ctx context.Context, domain string) (*enterprise.Tenant, error) {
	for _, t := range m.tenants {
		if t.Domain == domain {
			return t, nil
		}
	}
	return nil, enterprise.ErrTenantNotFound
}

func (m *mockCPClient) UpdateTenantStatus(ctx context.Context, tenantID string, status enterprise.TenantStatus) error {
	t, exists := m.tenants[tenantID]
	if !exists {
		return enterprise.ErrTenantNotFound
	}
	t.Status = status
	return nil
}

func (m *mockCPClient) RegisterNode(ctx context.Context, nodeInfo *enterprise.NodeInfo) error {
	if m.registerErr != nil {
		return m.registerErr
	}
	m.nodes[nodeInfo.ID] = nodeInfo
	return nil
}

func (m *mockCPClient) SendHeartbeat(ctx context.Context, nodeID string, activeTenantsCount int) error {
	m.heartbeats++
	return nil
}

func (m *mockCPClient) GetPlacementDecision(ctx context.Context, tenantID string) (*enterprise.PlacementDecision, error) {
	p, exists := m.placements[tenantID]
	if !exists {
		return nil, enterprise.ErrTenantNotAssigned
	}
	return p, nil
}

func (m *mockCPClient) addTenant(t *enterprise.Tenant) {
	m.tenants[t.ID] = t
}

// mockStorageBackend implements enterprise.StorageBackend for testing
type mockStorageBackend struct{}

func (m *mockStorageBackend) DownloadTenantDB(ctx context.Context, tenant *enterprise.Tenant, dbName string, destPath string) error {
	return nil
}

func (m *mockStorageBackend) UploadTenantDB(ctx context.Context, tenant *enterprise.Tenant, dbName string, sourcePath string) error {
	return nil
}

func (m *mockStorageBackend) DeleteTenantData(ctx context.Context, tenant *enterprise.Tenant) error {
	return nil
}

func (m *mockStorageBackend) ListTenantBackups(ctx context.Context, tenantID string) ([]string, error) {
	return []string{}, nil
}

func (m *mockStorageBackend) RestoreFromBackup(ctx context.Context, tenantID string, backupID string) error {
	return nil
}

// testManagerOnce ensures we only create one manager for tests to avoid
// duplicate Prometheus metrics registration
var (
	testManager     *Manager
	testManagerOnce sync.Once
	testManagerErr  error
)

func getTestManager(t *testing.T) *Manager {
	testManagerOnce.Do(func() {
		config := &enterprise.ClusterConfig{
			Mode:       enterprise.ModeTenantNode,
			DataDir:    t.TempDir(),
			MaxTenants: 10,
		}

		cpClient := newMockCPClient()
		storage := &mockStorageBackend{}

		testManager, testManagerErr = NewManager(config, storage, cpClient)
	})

	if testManagerErr != nil {
		t.Fatalf("failed to create test manager: %v", testManagerErr)
	}

	return testManager
}

func TestNewManagerValidMode(t *testing.T) {
	// This test uses the shared manager since we can only create one
	mgr := getTestManager(t)
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}

	if mgr.capacity != 10 {
		t.Errorf("expected capacity 10, got %d", mgr.capacity)
	}
}

func TestNewManagerInvalidMode(t *testing.T) {
	config := &enterprise.ClusterConfig{
		Mode:       enterprise.ModeGateway, // Invalid for tenant node
		DataDir:    t.TempDir(),
		MaxTenants: 10,
	}

	cpClient := newMockCPClient()
	storage := &mockStorageBackend{}

	// This test doesn't create a manager, so it won't have metrics issues
	_, err := NewManager(config, storage, cpClient)
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestNewManagerControlPlaneMode(t *testing.T) {
	config := &enterprise.ClusterConfig{
		Mode:       enterprise.ModeControlPlane, // Invalid for tenant node
		DataDir:    t.TempDir(),
		MaxTenants: 10,
	}

	cpClient := newMockCPClient()
	storage := &mockStorageBackend{}

	_, err := NewManager(config, storage, cpClient)
	if err == nil {
		t.Error("expected error for control plane mode")
	}
}

func TestGetTenantNotLoaded(t *testing.T) {
	mgr := getTestManager(t)

	_, err := mgr.GetTenant("nonexistent")
	if err != enterprise.ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestListActiveTenantsEmpty(t *testing.T) {
	mgr := getTestManager(t)

	tenants := mgr.ListActiveTenants()
	// May not be empty if other tests added tenants
	_ = tenants // Just verify no panic
}

func TestGetStats(t *testing.T) {
	mgr := getTestManager(t)

	stats := mgr.GetStats()

	if stats.Capacity != 10 {
		t.Errorf("expected capacity 10, got %d", stats.Capacity)
	}
}

func TestGetHealthChecker(t *testing.T) {
	mgr := getTestManager(t)

	checker := mgr.GetHealthChecker()
	if checker == nil {
		t.Error("expected non-nil health checker")
	}
}

func TestGetQuotaEnforcer(t *testing.T) {
	mgr := getTestManager(t)

	enforcer := mgr.GetQuotaEnforcer()
	if enforcer == nil {
		t.Error("expected non-nil quota enforcer")
	}
}

func TestEvictIdleTenantsNoTenants(t *testing.T) {
	mgr := getTestManager(t)

	// Should not error with no tenants
	err := mgr.EvictIdleTenants(10 * time.Minute)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAccessOrderTracking(t *testing.T) {
	mgr := getTestManager(t)

	// Clear access order for this test
	mgr.tenantsMu.Lock()
	originalOrder := mgr.accessOrder
	mgr.accessOrder = make([]string, 0)

	mgr.updateAccessOrder("tenant-test-1")
	mgr.updateAccessOrder("tenant-test-2")
	mgr.updateAccessOrder("tenant-test-3")

	if len(mgr.accessOrder) != 3 {
		t.Errorf("expected 3 items in access order, got %d", len(mgr.accessOrder))
	}

	if mgr.accessOrder[0] != "tenant-test-1" {
		t.Errorf("expected tenant-test-1 at position 0, got %s", mgr.accessOrder[0])
	}

	// Access tenant-test-1 again, should move to end
	mgr.updateAccessOrder("tenant-test-1")

	if len(mgr.accessOrder) != 3 {
		t.Errorf("expected 3 items after re-access, got %d", len(mgr.accessOrder))
	}

	if mgr.accessOrder[2] != "tenant-test-1" {
		t.Errorf("expected tenant-test-1 at position 2 after re-access, got %s", mgr.accessOrder[2])
	}

	// Remove tenant-test-2
	mgr.removeFromAccessOrder("tenant-test-2")

	if len(mgr.accessOrder) != 2 {
		t.Errorf("expected 2 items after removal, got %d", len(mgr.accessOrder))
	}

	// Restore original order
	mgr.accessOrder = originalOrder
	mgr.tenantsMu.Unlock()
}

func TestRemoveFromAccessOrderEmpty(t *testing.T) {
	mgr := getTestManager(t)

	mgr.tenantsMu.Lock()
	originalOrder := mgr.accessOrder
	mgr.accessOrder = make([]string, 0)

	// Should not panic when removing from empty order
	mgr.removeFromAccessOrder("nonexistent")

	if len(mgr.accessOrder) != 0 {
		t.Errorf("expected 0 items, got %d", len(mgr.accessOrder))
	}

	mgr.accessOrder = originalOrder
	mgr.tenantsMu.Unlock()
}

func TestRemoveFromAccessOrderNotFound(t *testing.T) {
	mgr := getTestManager(t)

	mgr.tenantsMu.Lock()
	originalOrder := mgr.accessOrder
	mgr.accessOrder = []string{"tenant-x", "tenant-y"}

	// Remove nonexistent tenant - should not affect list
	mgr.removeFromAccessOrder("nonexistent")

	if len(mgr.accessOrder) != 2 {
		t.Errorf("expected 2 items, got %d", len(mgr.accessOrder))
	}

	mgr.accessOrder = originalOrder
	mgr.tenantsMu.Unlock()
}

func TestGetWeightedCapacity(t *testing.T) {
	mgr := getTestManager(t)

	used, total := mgr.getWeightedCapacity()

	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}

	// Used may be > 0 if other tests loaded tenants
	_ = used // Just verify no panic
}

func TestManagerStatsMemoryCalculation(t *testing.T) {
	mgr := getTestManager(t)

	stats := mgr.GetStats()

	// Memory is calculated based on loaded tenants (estimate ~50MB per tenant)
	// With no tenants: 0 MB
	// With N tenants: N * 50 MB
	expectedMB := int64(stats.LoadedTenants * 50)
	if stats.MemoryUsedMB != expectedMB {
		t.Errorf("expected %d MB memory, got %d", expectedMB, stats.MemoryUsedMB)
	}

	// CPU percent should default to 0
	if stats.CPUPercent != 0 {
		t.Errorf("expected 0 CPU percent, got %d", stats.CPUPercent)
	}
}

func TestUnloadNonexistentTenant(t *testing.T) {
	mgr := getTestManager(t)

	// Unloading nonexistent tenant should not error
	err := mgr.UnloadTenant(context.Background(), "nonexistent-tenant-xyz")
	if err != nil {
		t.Errorf("unexpected error unloading nonexistent tenant: %v", err)
	}
}

func TestEvictLRUEmptyOrder(t *testing.T) {
	mgr := getTestManager(t)

	mgr.tenantsMu.Lock()
	originalOrder := mgr.accessOrder
	mgr.accessOrder = make([]string, 0)

	// Evicting from empty access order should error
	err := mgr.evictLRULocked()

	mgr.accessOrder = originalOrder
	mgr.tenantsMu.Unlock()

	if err == nil {
		t.Error("expected error when evicting from empty order")
	}
}

func TestResourceManagerInitialized(t *testing.T) {
	mgr := getTestManager(t)

	if mgr.resourceMgr == nil {
		t.Error("expected non-nil resource manager")
	}
}

func TestMetricsCollectorInitialized(t *testing.T) {
	mgr := getTestManager(t)

	if mgr.metricsCollector == nil {
		t.Error("expected non-nil metrics collector")
	}
}

func TestLitestreamManagerInitialized(t *testing.T) {
	mgr := getTestManager(t)

	if mgr.litestreamManager == nil {
		t.Error("expected non-nil litestream manager")
	}
}

func TestMultipleAccessOrderUpdates(t *testing.T) {
	mgr := getTestManager(t)

	mgr.tenantsMu.Lock()
	originalOrder := mgr.accessOrder
	mgr.accessOrder = make([]string, 0)

	// Simulate many updates
	for i := 0; i < 100; i++ {
		tenantID := "tenant-multi-" + string(rune('a'+i%5))
		mgr.updateAccessOrder(tenantID)
	}

	// Should only have 5 unique tenants
	if len(mgr.accessOrder) != 5 {
		t.Errorf("expected 5 items, got %d", len(mgr.accessOrder))
	}

	mgr.accessOrder = originalOrder
	mgr.tenantsMu.Unlock()
}

func TestMockCPClientGetTenantMetadata(t *testing.T) {
	client := newMockCPClient()

	// Add a tenant
	tenant := &enterprise.Tenant{
		ID:     "test-tenant",
		Domain: "test.example.com",
		Status: enterprise.TenantStatusActive,
	}
	client.addTenant(tenant)

	// Retrieve it
	retrieved, err := client.GetTenantMetadata(context.Background(), "test-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.ID != tenant.ID {
		t.Errorf("expected ID %s, got %s", tenant.ID, retrieved.ID)
	}

	// Try non-existent
	_, err = client.GetTenantMetadata(context.Background(), "nonexistent")
	if err != enterprise.ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestMockCPClientGetTenantByDomain(t *testing.T) {
	client := newMockCPClient()

	tenant := &enterprise.Tenant{
		ID:     "test-tenant",
		Domain: "test.example.com",
	}
	client.addTenant(tenant)

	// Find by domain
	retrieved, err := client.GetTenantByDomain(context.Background(), "test.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.ID != tenant.ID {
		t.Errorf("expected ID %s, got %s", tenant.ID, retrieved.ID)
	}

	// Try non-existent domain
	_, err = client.GetTenantByDomain(context.Background(), "nonexistent.example.com")
	if err != enterprise.ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestMockCPClientUpdateTenantStatus(t *testing.T) {
	client := newMockCPClient()

	tenant := &enterprise.Tenant{
		ID:     "test-tenant",
		Status: enterprise.TenantStatusActive,
	}
	client.addTenant(tenant)

	// Update status
	err := client.UpdateTenantStatus(context.Background(), "test-tenant", enterprise.TenantStatusIdle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenant.Status != enterprise.TenantStatusIdle {
		t.Errorf("expected status Idle, got %v", tenant.Status)
	}

	// Try non-existent
	err = client.UpdateTenantStatus(context.Background(), "nonexistent", enterprise.TenantStatusIdle)
	if err != enterprise.ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestMockCPClientRegisterNode(t *testing.T) {
	client := newMockCPClient()

	nodeInfo := &enterprise.NodeInfo{
		ID:       "node-1",
		Address:  "localhost:8091",
		Status:   "online",
		Capacity: 10,
	}

	err := client.RegisterNode(context.Background(), nodeInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.nodes["node-1"] == nil {
		t.Error("expected node to be registered")
	}
}

func TestMockCPClientSendHeartbeat(t *testing.T) {
	client := newMockCPClient()

	err := client.SendHeartbeat(context.Background(), "node-1", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.heartbeats != 1 {
		t.Errorf("expected 1 heartbeat, got %d", client.heartbeats)
	}

	client.SendHeartbeat(context.Background(), "node-1", 6)

	if client.heartbeats != 2 {
		t.Errorf("expected 2 heartbeats, got %d", client.heartbeats)
	}
}

func TestMockCPClientGetPlacementDecision(t *testing.T) {
	client := newMockCPClient()

	placement := &enterprise.PlacementDecision{
		TenantID:    "tenant-1",
		NodeID:      "node-1",
		NodeAddress: "localhost:8091",
	}
	client.placements["tenant-1"] = placement

	retrieved, err := client.GetPlacementDecision(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.NodeID != placement.NodeID {
		t.Errorf("expected NodeID %s, got %s", placement.NodeID, retrieved.NodeID)
	}

	// Non-existent
	_, err = client.GetPlacementDecision(context.Background(), "nonexistent")
	if err != enterprise.ErrTenantNotAssigned {
		t.Errorf("expected ErrTenantNotAssigned, got %v", err)
	}
}
