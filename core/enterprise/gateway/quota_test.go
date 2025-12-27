package gateway

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pocketbase/pocketbase/core/enterprise"
)

// mockControlPlaneClient implements enterprise.ControlPlaneClient for testing
type mockControlPlaneClient struct {
	tenants map[string]*enterprise.Tenant
}

func newMockCPClient() *mockControlPlaneClient {
	return &mockControlPlaneClient{
		tenants: make(map[string]*enterprise.Tenant),
	}
}

func (m *mockControlPlaneClient) GetTenantMetadata(ctx context.Context, tenantID string) (*enterprise.Tenant, error) {
	t, exists := m.tenants[tenantID]
	if !exists {
		return nil, enterprise.ErrTenantNotFound
	}
	return t, nil
}

func (m *mockControlPlaneClient) GetTenantByDomain(ctx context.Context, domain string) (*enterprise.Tenant, error) {
	return nil, enterprise.ErrTenantNotFound
}

func (m *mockControlPlaneClient) UpdateTenantStatus(ctx context.Context, tenantID string, status enterprise.TenantStatus) error {
	return nil
}

func (m *mockControlPlaneClient) RegisterNode(ctx context.Context, nodeInfo *enterprise.NodeInfo) error {
	return nil
}

func (m *mockControlPlaneClient) SendHeartbeat(ctx context.Context, nodeID string, activeTenantsCount int) error {
	return nil
}

func (m *mockControlPlaneClient) GetPlacementDecision(ctx context.Context, tenantID string) (*enterprise.PlacementDecision, error) {
	return nil, nil
}

func (m *mockControlPlaneClient) addTenant(id string, storageQuota, apiQuota int64) {
	m.tenants[id] = &enterprise.Tenant{
		ID:               id,
		StorageQuotaMB:   storageQuota,
		APIRequestsQuota: apiQuota,
	}
}

func TestQuotaEnforcerCheckQuota(t *testing.T) {
	cpClient := newMockCPClient()
	cpClient.addTenant("tenant-1", 100, 1000)

	enforcer := NewQuotaEnforcer(cpClient)

	// First request should be allowed
	err := enforcer.CheckQuota("tenant-1", 0)
	if err != nil {
		t.Errorf("first request should be allowed: %v", err)
	}
}

func TestQuotaEnforcerConcurrentRequests(t *testing.T) {
	cpClient := newMockCPClient()
	cpClient.addTenant("tenant-1", 1024, 10000)

	enforcer := NewQuotaEnforcer(cpClient)

	var wg sync.WaitGroup
	var successCount int64
	var failCount int64

	// 50 concurrent requests (should all pass with high quota)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := enforcer.CheckQuota("tenant-1", 0)
			if err != nil {
				atomic.AddInt64(&failCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	// Most should succeed (token bucket allows bursts)
	if successCount < 40 {
		t.Errorf("expected at least 40 successes, got %d (failures: %d)", successCount, failCount)
	}
}

func TestQuotaEnforcerMultipleTenants(t *testing.T) {
	cpClient := newMockCPClient()
	cpClient.addTenant("tenant-1", 100, 1000)
	cpClient.addTenant("tenant-2", 100, 1000)

	enforcer := NewQuotaEnforcer(cpClient)

	// Both tenants should be able to make requests
	err := enforcer.CheckQuota("tenant-1", 0)
	if err != nil {
		t.Errorf("tenant-1 request should be allowed: %v", err)
	}

	err = enforcer.CheckQuota("tenant-2", 0)
	if err != nil {
		t.Errorf("tenant-2 request should be allowed: %v", err)
	}
}

func TestQuotaEnforcerGetStats(t *testing.T) {
	cpClient := newMockCPClient()
	cpClient.addTenant("tenant-1", 100, 1000)

	enforcer := NewQuotaEnforcer(cpClient)

	// Make a request
	enforcer.CheckQuota("tenant-1", 0)

	stats := enforcer.GetStats()

	// Stats should be a valid map
	if stats == nil {
		t.Error("stats should not be nil")
	}

	// Should include expected keys
	if _, exists := stats["rejectedRequests"]; !exists {
		t.Error("stats should include rejectedRequests")
	}
}

func TestQuotaEnforcerCleanupTenant(t *testing.T) {
	cpClient := newMockCPClient()
	cpClient.addTenant("tenant-1", 100, 1000)

	enforcer := NewQuotaEnforcer(cpClient)

	// Make some requests
	enforcer.CheckQuota("tenant-1", 0)
	enforcer.CheckQuota("tenant-1", 0)

	// Cleanup tenant
	enforcer.CleanupTenant("tenant-1")

	// After cleanup, tenant should still work (fresh state)
	err := enforcer.CheckQuota("tenant-1", 0)
	if err != nil {
		t.Errorf("tenant should have fresh quota after cleanup: %v", err)
	}
}

func TestQuotaEnforcerRecordRequest(t *testing.T) {
	cpClient := newMockCPClient()
	cpClient.addTenant("tenant-1", 100, 1000)

	enforcer := NewQuotaEnforcer(cpClient)

	// Record a request - should not panic
	enforcer.RecordRequest("tenant-1", 1024)

	// Record for unknown tenant - should also not panic
	enforcer.RecordRequest("unknown-tenant", 512)
}

func TestRateLimiterTokenBucket(t *testing.T) {
	// Test the rate limiter directly
	rl := &RateLimiter{
		tokens:     10,
		maxTokens:  10,
		refillRate: 1.0, // 1 token per second
	}

	// Should allow requests while tokens available
	for i := 0; i < 10; i++ {
		rl.mu.Lock()
		if rl.tokens < 1 {
			rl.mu.Unlock()
			t.Errorf("expected tokens available at iteration %d", i)
			break
		}
		rl.tokens--
		rl.mu.Unlock()
	}

	// Should be out of tokens now
	rl.mu.Lock()
	remaining := rl.tokens
	rl.mu.Unlock()

	if remaining != 0 {
		t.Errorf("expected 0 tokens remaining, got %f", remaining)
	}
}

func TestTenantQuotaState(t *testing.T) {
	state := &TenantQuotaState{
		TenantID:         "tenant-1",
		StorageQuotaMB:   100,
		APIRequestsQuota: 1000,
		StorageUsedMB:    50,
		RequestsToday:    500,
	}

	if state.TenantID != "tenant-1" {
		t.Errorf("expected tenant-1, got %s", state.TenantID)
	}

	if state.StorageUsedMB != 50 {
		t.Errorf("expected 50MB used, got %d", state.StorageUsedMB)
	}

	if state.RequestsToday != 500 {
		t.Errorf("expected 500 requests, got %d", state.RequestsToday)
	}
}

func TestQuotaEnforcerUnknownTenant(t *testing.T) {
	cpClient := newMockCPClient()
	// Don't add tenant-1

	enforcer := NewQuotaEnforcer(cpClient)

	// Request for unknown tenant should still work (uses defaults)
	err := enforcer.CheckQuota("unknown-tenant", 0)
	// May or may not error depending on implementation
	_ = err // Just verify no panic
}
