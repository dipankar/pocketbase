package badger

import (
	"os"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
)

// createTestStorage creates a temporary storage instance for testing
func createTestStorage(t *testing.T) (*Storage, func()) {
	tempDir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	storage, err := NewStorage(tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to create storage: %v", err)
	}

	cleanup := func() {
		storage.Close()
		os.RemoveAll(tempDir)
	}

	return storage, cleanup
}

// Tenant operation tests

func TestCreateAndGetTenant(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	tenant := &enterprise.Tenant{
		ID:               "tenant-1",
		Domain:           "test.example.com",
		OwnerUserID:      "user-1",
		Status:           enterprise.TenantStatusActive,
		StorageQuotaMB:   1024,
		APIRequestsQuota: 10000,
	}

	// Create tenant
	err := storage.CreateTenant(tenant)
	if err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	// Get tenant by ID
	retrieved, err := storage.GetTenant("tenant-1")
	if err != nil {
		t.Fatalf("failed to get tenant: %v", err)
	}

	if retrieved.ID != tenant.ID {
		t.Errorf("expected ID %s, got %s", tenant.ID, retrieved.ID)
	}
	if retrieved.Domain != tenant.Domain {
		t.Errorf("expected Domain %s, got %s", tenant.Domain, retrieved.Domain)
	}
	if retrieved.OwnerUserID != tenant.OwnerUserID {
		t.Errorf("expected OwnerUserID %s, got %s", tenant.OwnerUserID, retrieved.OwnerUserID)
	}
}

func TestGetTenantByDomain(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	tenant := &enterprise.Tenant{
		ID:     "tenant-1",
		Domain: "test.example.com",
	}

	storage.CreateTenant(tenant)

	// Get by domain
	retrieved, err := storage.GetTenantByDomain("test.example.com")
	if err != nil {
		t.Fatalf("failed to get tenant by domain: %v", err)
	}

	if retrieved.ID != tenant.ID {
		t.Errorf("expected ID %s, got %s", tenant.ID, retrieved.ID)
	}
}

func TestGetTenantNotFound(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	_, err := storage.GetTenant("nonexistent")
	if err != enterprise.ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestGetTenantByDomainNotFound(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	_, err := storage.GetTenantByDomain("nonexistent.example.com")
	if err != enterprise.ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestCreateTenantDuplicateID(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	tenant := &enterprise.Tenant{
		ID:     "tenant-1",
		Domain: "test1.example.com",
	}

	storage.CreateTenant(tenant)

	// Try to create with same ID but different domain
	duplicate := &enterprise.Tenant{
		ID:     "tenant-1",
		Domain: "test2.example.com",
	}

	err := storage.CreateTenant(duplicate)
	if err != enterprise.ErrTenantAlreadyExists {
		t.Errorf("expected ErrTenantAlreadyExists, got %v", err)
	}
}

func TestCreateTenantDuplicateDomain(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	tenant := &enterprise.Tenant{
		ID:     "tenant-1",
		Domain: "test.example.com",
	}

	storage.CreateTenant(tenant)

	// Try to create with different ID but same domain
	duplicate := &enterprise.Tenant{
		ID:     "tenant-2",
		Domain: "test.example.com",
	}

	err := storage.CreateTenant(duplicate)
	if err == nil {
		t.Error("expected error for duplicate domain")
	}
}

func TestUpdateTenant(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	tenant := &enterprise.Tenant{
		ID:             "tenant-1",
		Domain:         "test.example.com",
		StorageQuotaMB: 100,
	}

	storage.CreateTenant(tenant)

	// Update tenant
	tenant.StorageQuotaMB = 200
	err := storage.UpdateTenant(tenant)
	if err != nil {
		t.Fatalf("failed to update tenant: %v", err)
	}

	// Verify update
	retrieved, _ := storage.GetTenant("tenant-1")
	if retrieved.StorageQuotaMB != 200 {
		t.Errorf("expected StorageQuotaMB 200, got %d", retrieved.StorageQuotaMB)
	}
}

func TestUpdateTenantNotFound(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	tenant := &enterprise.Tenant{
		ID:     "nonexistent",
		Domain: "test.example.com",
	}

	err := storage.UpdateTenant(tenant)
	if err != enterprise.ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestUpdateTenantStatus(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	tenant := &enterprise.Tenant{
		ID:     "tenant-1",
		Domain: "test.example.com",
		Status: enterprise.TenantStatusActive,
	}

	storage.CreateTenant(tenant)

	// Update status
	err := storage.UpdateTenantStatus("tenant-1", enterprise.TenantStatusIdle)
	if err != nil {
		t.Fatalf("failed to update tenant status: %v", err)
	}

	// Verify update
	retrieved, _ := storage.GetTenant("tenant-1")
	if retrieved.Status != enterprise.TenantStatusIdle {
		t.Errorf("expected Idle status, got %v", retrieved.Status)
	}
}

func TestDeleteTenant(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	tenant := &enterprise.Tenant{
		ID:     "tenant-1",
		Domain: "test.example.com",
	}

	storage.CreateTenant(tenant)

	// Delete tenant
	err := storage.DeleteTenant("tenant-1")
	if err != nil {
		t.Fatalf("failed to delete tenant: %v", err)
	}

	// Verify deletion
	_, err = storage.GetTenant("tenant-1")
	if err != enterprise.ErrTenantNotFound {
		t.Error("expected tenant to be deleted")
	}

	// Verify domain mapping is also deleted
	_, err = storage.GetTenantByDomain("test.example.com")
	if err != enterprise.ErrTenantNotFound {
		t.Error("expected domain mapping to be deleted")
	}
}

func TestDeleteTenantNotFound(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	err := storage.DeleteTenant("nonexistent")
	if err != enterprise.ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
}

// User operation tests

func TestCreateAndGetUser(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	user := &enterprise.ClusterUser{
		ID:       "user-1",
		Email:    "test@example.com",
		Name:     "Test User",
		Verified: true,
	}

	// Create user
	err := storage.CreateUser(user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Get user by ID
	retrieved, err := storage.GetUser("user-1")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	if retrieved.ID != user.ID {
		t.Errorf("expected ID %s, got %s", user.ID, retrieved.ID)
	}
	if retrieved.Email != user.Email {
		t.Errorf("expected Email %s, got %s", user.Email, retrieved.Email)
	}
}

func TestGetUserByEmail(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	user := &enterprise.ClusterUser{
		ID:    "user-1",
		Email: "test@example.com",
	}

	storage.CreateUser(user)

	// Get by email
	retrieved, err := storage.GetUserByEmail("test@example.com")
	if err != nil {
		t.Fatalf("failed to get user by email: %v", err)
	}

	if retrieved.ID != user.ID {
		t.Errorf("expected ID %s, got %s", user.ID, retrieved.ID)
	}
}

func TestGetUserNotFound(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	_, err := storage.GetUser("nonexistent")
	if err != enterprise.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestGetUserByEmailNotFound(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	_, err := storage.GetUserByEmail("nonexistent@example.com")
	if err != enterprise.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestCreateUserDuplicateID(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	user := &enterprise.ClusterUser{
		ID:    "user-1",
		Email: "test1@example.com",
	}

	storage.CreateUser(user)

	duplicate := &enterprise.ClusterUser{
		ID:    "user-1",
		Email: "test2@example.com",
	}

	err := storage.CreateUser(duplicate)
	if err != enterprise.ErrUserAlreadyExists {
		t.Errorf("expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	user := &enterprise.ClusterUser{
		ID:    "user-1",
		Email: "test@example.com",
	}

	storage.CreateUser(user)

	duplicate := &enterprise.ClusterUser{
		ID:    "user-2",
		Email: "test@example.com",
	}

	err := storage.CreateUser(duplicate)
	if err == nil {
		t.Error("expected error for duplicate email")
	}
}

func TestUpdateUser(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	user := &enterprise.ClusterUser{
		ID:       "user-1",
		Email:    "test@example.com",
		Name:     "Original Name",
		Verified: false,
	}

	storage.CreateUser(user)

	// Update user
	user.Name = "Updated Name"
	user.Verified = true
	err := storage.UpdateUser(user)
	if err != nil {
		t.Fatalf("failed to update user: %v", err)
	}

	// Verify update
	retrieved, _ := storage.GetUser("user-1")
	if retrieved.Name != "Updated Name" {
		t.Errorf("expected Updated Name, got %s", retrieved.Name)
	}
	if !retrieved.Verified {
		t.Error("expected Verified to be true")
	}
}

func TestUpdateUserNotFound(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	user := &enterprise.ClusterUser{
		ID:    "nonexistent",
		Email: "test@example.com",
	}

	err := storage.UpdateUser(user)
	if err != enterprise.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestCountUserTenants(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	// Create tenants for user-1
	for i := 0; i < 3; i++ {
		storage.CreateTenant(&enterprise.Tenant{
			ID:          "tenant-" + string(rune('a'+i)),
			Domain:      "test" + string(rune('a'+i)) + ".example.com",
			OwnerUserID: "user-1",
			Status:      enterprise.TenantStatusActive,
		})
	}

	// Create a deleted tenant (should not be counted)
	storage.CreateTenant(&enterprise.Tenant{
		ID:          "tenant-deleted",
		Domain:      "deleted.example.com",
		OwnerUserID: "user-1",
		Status:      enterprise.TenantStatusDeleted,
	})

	// Create a tenant for another user
	storage.CreateTenant(&enterprise.Tenant{
		ID:          "tenant-other",
		Domain:      "other.example.com",
		OwnerUserID: "user-2",
		Status:      enterprise.TenantStatusActive,
	})

	count, err := storage.CountUserTenants("user-1")
	if err != nil {
		t.Fatalf("failed to count tenants: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 tenants, got %d", count)
	}
}

func TestListUsers(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	// Create users
	for i := 0; i < 5; i++ {
		storage.CreateUser(&enterprise.ClusterUser{
			ID:    "user-" + string(rune('a'+i)),
			Email: "user" + string(rune('a'+i)) + "@example.com",
		})
	}

	// List all users
	users, total, err := storage.ListUsers(0, 0)
	if err != nil {
		t.Fatalf("failed to list users: %v", err)
	}

	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(users) != 5 {
		t.Errorf("expected 5 users, got %d", len(users))
	}

	// Test pagination
	users, total, err = storage.ListUsers(2, 1)
	if err != nil {
		t.Fatalf("failed to list users with pagination: %v", err)
	}

	if total != 5 {
		t.Errorf("expected total 5 with pagination, got %d", total)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users with limit, got %d", len(users))
	}
}

func TestListTenants(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	// Create tenants
	for i := 0; i < 5; i++ {
		storage.CreateTenant(&enterprise.Tenant{
			ID:          "tenant-" + string(rune('a'+i)),
			Domain:      "test" + string(rune('a'+i)) + ".example.com",
			OwnerUserID: "user-1",
			Status:      enterprise.TenantStatusActive,
		})
	}

	// Create a deleted tenant (should not appear in list)
	storage.CreateTenant(&enterprise.Tenant{
		ID:          "tenant-deleted",
		Domain:      "deleted.example.com",
		OwnerUserID: "user-1",
		Status:      enterprise.TenantStatusDeleted,
	})

	// List all tenants
	tenants, total, err := storage.ListTenants(0, 0, "")
	if err != nil {
		t.Fatalf("failed to list tenants: %v", err)
	}

	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(tenants) != 5 {
		t.Errorf("expected 5 tenants, got %d", len(tenants))
	}
}

func TestListTenantsWithOwnerFilter(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	// Create tenants for different owners
	storage.CreateTenant(&enterprise.Tenant{
		ID:          "tenant-1",
		Domain:      "test1.example.com",
		OwnerUserID: "user-1",
		Status:      enterprise.TenantStatusActive,
	})
	storage.CreateTenant(&enterprise.Tenant{
		ID:          "tenant-2",
		Domain:      "test2.example.com",
		OwnerUserID: "user-1",
		Status:      enterprise.TenantStatusActive,
	})
	storage.CreateTenant(&enterprise.Tenant{
		ID:          "tenant-3",
		Domain:      "test3.example.com",
		OwnerUserID: "user-2",
		Status:      enterprise.TenantStatusActive,
	})

	// Filter by owner
	tenants, total, err := storage.ListTenants(0, 0, "user-1")
	if err != nil {
		t.Fatalf("failed to list tenants: %v", err)
	}

	if total != 2 {
		t.Errorf("expected total 2 for user-1, got %d", total)
	}
	if len(tenants) != 2 {
		t.Errorf("expected 2 tenants for user-1, got %d", len(tenants))
	}
}

// Node operation tests

func TestSaveAndGetNode(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	node := &enterprise.NodeInfo{
		ID:            "node-1",
		Address:       "localhost:8091",
		Status:        "online",
		Capacity:      10,
		ActiveTenants: 5,
		LastHeartbeat: time.Now(),
	}

	// Save node
	err := storage.SaveNode(node)
	if err != nil {
		t.Fatalf("failed to save node: %v", err)
	}

	// Get node
	retrieved, err := storage.GetNode("node-1")
	if err != nil {
		t.Fatalf("failed to get node: %v", err)
	}

	if retrieved.ID != node.ID {
		t.Errorf("expected ID %s, got %s", node.ID, retrieved.ID)
	}
	if retrieved.Address != node.Address {
		t.Errorf("expected Address %s, got %s", node.Address, retrieved.Address)
	}
}

func TestGetNodeNotFound(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	_, err := storage.GetNode("nonexistent")
	if err != enterprise.ErrNodeNotFound {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestListNodes(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	// Save nodes
	for i := 0; i < 3; i++ {
		storage.SaveNode(&enterprise.NodeInfo{
			ID:      "node-" + string(rune('a'+i)),
			Address: "localhost:" + string(rune('0'+i)),
		})
	}

	nodes, err := storage.ListNodes()
	if err != nil {
		t.Fatalf("failed to list nodes: %v", err)
	}

	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}
}

func TestListTenantsByNode(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	// Create tenants assigned to different nodes
	storage.CreateTenant(&enterprise.Tenant{
		ID:             "tenant-1",
		Domain:         "test1.example.com",
		AssignedNodeID: "node-1",
		Status:         enterprise.TenantStatusActive,
	})
	storage.CreateTenant(&enterprise.Tenant{
		ID:             "tenant-2",
		Domain:         "test2.example.com",
		AssignedNodeID: "node-1",
		Status:         enterprise.TenantStatusActive,
	})
	storage.CreateTenant(&enterprise.Tenant{
		ID:             "tenant-3",
		Domain:         "test3.example.com",
		AssignedNodeID: "node-2",
		Status:         enterprise.TenantStatusActive,
	})
	storage.CreateTenant(&enterprise.Tenant{
		ID:             "tenant-deleted",
		Domain:         "deleted.example.com",
		AssignedNodeID: "node-1",
		Status:         enterprise.TenantStatusDeleted, // Should be excluded
	})

	tenants, err := storage.ListTenantsByNode("node-1")
	if err != nil {
		t.Fatalf("failed to list tenants by node: %v", err)
	}

	if len(tenants) != 2 {
		t.Errorf("expected 2 tenants for node-1, got %d", len(tenants))
	}
}

// Placement operation tests

func TestSaveAndGetPlacement(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	placement := &enterprise.PlacementDecision{
		TenantID:    "tenant-1",
		NodeID:      "node-1",
		NodeAddress: "localhost:8091",
		Reason:      "least-loaded",
		DecidedAt:   time.Now(),
	}

	// Save placement
	err := storage.SavePlacement(placement)
	if err != nil {
		t.Fatalf("failed to save placement: %v", err)
	}

	// Get placement
	retrieved, err := storage.GetPlacement("tenant-1")
	if err != nil {
		t.Fatalf("failed to get placement: %v", err)
	}

	if retrieved.TenantID != placement.TenantID {
		t.Errorf("expected TenantID %s, got %s", placement.TenantID, retrieved.TenantID)
	}
	if retrieved.NodeID != placement.NodeID {
		t.Errorf("expected NodeID %s, got %s", placement.NodeID, retrieved.NodeID)
	}
}

func TestGetPlacementNotFound(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	_, err := storage.GetPlacement("nonexistent")
	if err != enterprise.ErrTenantNotAssigned {
		t.Errorf("expected ErrTenantNotAssigned, got %v", err)
	}
}

// Activity tracking tests

func TestSaveAndGetActivity(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	now := time.Now()
	activity := &enterprise.TenantActivity{
		TenantID:    "tenant-1",
		LastAccess:  now,
		AccessCount: 100,
		StorageTier: enterprise.StorageTierHot,
		Created:     now,
		Updated:     now,
	}

	// Save activity
	err := storage.SaveActivity(activity)
	if err != nil {
		t.Fatalf("failed to save activity: %v", err)
	}

	// Get activity
	retrieved, err := storage.GetActivity("tenant-1")
	if err != nil {
		t.Fatalf("failed to get activity: %v", err)
	}

	if retrieved.TenantID != activity.TenantID {
		t.Errorf("expected TenantID %s, got %s", activity.TenantID, retrieved.TenantID)
	}
	if retrieved.AccessCount != activity.AccessCount {
		t.Errorf("expected AccessCount %d, got %d", activity.AccessCount, retrieved.AccessCount)
	}
}

func TestGetActivityDefaultForMissing(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	// Get activity for non-existent tenant should return defaults
	activity, err := storage.GetActivity("nonexistent")
	if err != nil {
		t.Fatalf("failed to get default activity: %v", err)
	}

	if activity.TenantID != "nonexistent" {
		t.Errorf("expected TenantID nonexistent, got %s", activity.TenantID)
	}
	if activity.AccessCount != 0 {
		t.Errorf("expected AccessCount 0, got %d", activity.AccessCount)
	}
	if activity.StorageTier != enterprise.StorageTierHot {
		t.Errorf("expected StorageTierHot, got %s", activity.StorageTier)
	}
}

func TestRecordTenantAccess(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	// Record first access
	err := storage.RecordTenantAccess("tenant-1")
	if err != nil {
		t.Fatalf("failed to record access: %v", err)
	}

	activity, _ := storage.GetActivity("tenant-1")
	if activity.AccessCount != 1 {
		t.Errorf("expected AccessCount 1, got %d", activity.AccessCount)
	}

	// Record second access
	storage.RecordTenantAccess("tenant-1")
	activity, _ = storage.GetActivity("tenant-1")
	if activity.AccessCount != 2 {
		t.Errorf("expected AccessCount 2, got %d", activity.AccessCount)
	}
}

func TestListInactiveTenants(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	now := time.Now()

	// Create active tenant (accessed recently)
	storage.SaveActivity(&enterprise.TenantActivity{
		TenantID:   "active-tenant",
		LastAccess: now,
	})

	// Create inactive tenant (accessed long ago)
	storage.SaveActivity(&enterprise.TenantActivity{
		TenantID:   "inactive-tenant",
		LastAccess: now.Add(-48 * time.Hour),
	})

	// List tenants inactive for more than 24 hours
	inactive, err := storage.ListInactiveTenants(now.Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("failed to list inactive tenants: %v", err)
	}

	if len(inactive) != 1 {
		t.Errorf("expected 1 inactive tenant, got %d", len(inactive))
	}
	if len(inactive) > 0 && inactive[0].TenantID != "inactive-tenant" {
		t.Errorf("expected inactive-tenant, got %s", inactive[0].TenantID)
	}
}

func TestListActivitiesByTier(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	now := time.Now()

	storage.SaveActivity(&enterprise.TenantActivity{
		TenantID:    "hot-tenant-1",
		StorageTier: enterprise.StorageTierHot,
		LastAccess:  now,
	})
	storage.SaveActivity(&enterprise.TenantActivity{
		TenantID:    "hot-tenant-2",
		StorageTier: enterprise.StorageTierHot,
		LastAccess:  now,
	})
	storage.SaveActivity(&enterprise.TenantActivity{
		TenantID:    "cold-tenant",
		StorageTier: enterprise.StorageTierCold,
		LastAccess:  now,
	})

	// List hot tenants
	hotTenants, err := storage.ListActivitiesByTier(enterprise.StorageTierHot)
	if err != nil {
		t.Fatalf("failed to list hot tenants: %v", err)
	}

	if len(hotTenants) != 2 {
		t.Errorf("expected 2 hot tenants, got %d", len(hotTenants))
	}

	// List cold tenants
	coldTenants, err := storage.ListActivitiesByTier(enterprise.StorageTierCold)
	if err != nil {
		t.Fatalf("failed to list cold tenants: %v", err)
	}

	if len(coldTenants) != 1 {
		t.Errorf("expected 1 cold tenant, got %d", len(coldTenants))
	}
}

// Access pattern tests

func TestSaveAndGetAccessPattern(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	now := time.Now()
	pattern := &enterprise.TenantAccessPattern{
		TenantID:          "tenant-1",
		DayOfWeek:         []int{1, 2, 3, 4, 5},
		HourOfDay:         []int{9, 10, 11, 14, 15, 16},
		PatternConfidence: 0.85,
		Created:           now,
		Updated:           now,
	}

	// Save pattern
	err := storage.SaveAccessPattern(pattern)
	if err != nil {
		t.Fatalf("failed to save pattern: %v", err)
	}

	// Get pattern
	retrieved, err := storage.GetAccessPattern("tenant-1")
	if err != nil {
		t.Fatalf("failed to get pattern: %v", err)
	}

	if retrieved.TenantID != pattern.TenantID {
		t.Errorf("expected TenantID %s, got %s", pattern.TenantID, retrieved.TenantID)
	}
	if retrieved.PatternConfidence != pattern.PatternConfidence {
		t.Errorf("expected PatternConfidence %f, got %f", pattern.PatternConfidence, retrieved.PatternConfidence)
	}
}

func TestGetAccessPatternDefaultForMissing(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	// Get pattern for non-existent tenant should return empty pattern
	pattern, err := storage.GetAccessPattern("nonexistent")
	if err != nil {
		t.Fatalf("failed to get default pattern: %v", err)
	}

	if pattern.TenantID != "nonexistent" {
		t.Errorf("expected TenantID nonexistent, got %s", pattern.TenantID)
	}
	if pattern.PatternConfidence != 0.0 {
		t.Errorf("expected PatternConfidence 0.0, got %f", pattern.PatternConfidence)
	}
}

// Verification token tests

func TestSaveAndGetVerificationToken(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	token := &enterprise.VerificationToken{
		Token:   "abc123",
		UserID:  "user-1",
		Email:   "test@example.com",
		Expires: time.Now().Add(24 * time.Hour),
		Used:    false,
	}

	// Save token
	err := storage.SaveVerificationToken(token)
	if err != nil {
		t.Fatalf("failed to save token: %v", err)
	}

	// Get token
	retrieved, err := storage.GetVerificationToken("abc123")
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}

	if retrieved.Token != token.Token {
		t.Errorf("expected Token %s, got %s", token.Token, retrieved.Token)
	}
	if retrieved.UserID != token.UserID {
		t.Errorf("expected UserID %s, got %s", token.UserID, retrieved.UserID)
	}
}

func TestGetVerificationTokenNotFound(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	_, err := storage.GetVerificationToken("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent token")
	}
}

func TestGetVerificationTokenExpired(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	token := &enterprise.VerificationToken{
		Token:   "expired123",
		UserID:  "user-1",
		Email:   "test@example.com",
		Expires: time.Now().Add(-1 * time.Hour), // Already expired
		Used:    false,
	}

	storage.SaveVerificationToken(token)

	_, err := storage.GetVerificationToken("expired123")
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestGetVerificationTokenAlreadyUsed(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	token := &enterprise.VerificationToken{
		Token:   "used123",
		UserID:  "user-1",
		Email:   "test@example.com",
		Expires: time.Now().Add(24 * time.Hour),
		Used:    true, // Already used
	}

	storage.SaveVerificationToken(token)

	_, err := storage.GetVerificationToken("used123")
	if err == nil {
		t.Error("expected error for used token")
	}
}

func TestMarkVerificationTokenUsed(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	token := &enterprise.VerificationToken{
		Token:   "mark123",
		UserID:  "user-1",
		Email:   "test@example.com",
		Expires: time.Now().Add(24 * time.Hour),
		Used:    false,
	}

	storage.SaveVerificationToken(token)

	// Mark as used
	err := storage.MarkVerificationTokenUsed("mark123")
	if err != nil {
		t.Fatalf("failed to mark token as used: %v", err)
	}

	// Should now fail to get
	_, err = storage.GetVerificationToken("mark123")
	if err == nil {
		t.Error("expected error for used token")
	}
}

func TestUseVerificationTokenAtomically(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	token := &enterprise.VerificationToken{
		Token:   "atomic123",
		UserID:  "user-1",
		Email:   "test@example.com",
		Expires: time.Now().Add(24 * time.Hour),
		Used:    false,
	}

	storage.SaveVerificationToken(token)

	// Use token atomically
	retrieved, err := storage.UseVerificationTokenAtomically("atomic123")
	if err != nil {
		t.Fatalf("failed to use token atomically: %v", err)
	}

	if retrieved.UserID != "user-1" {
		t.Errorf("expected UserID user-1, got %s", retrieved.UserID)
	}

	// Second attempt should fail
	_, err = storage.UseVerificationTokenAtomically("atomic123")
	if err == nil {
		t.Error("expected error for already used token")
	}
}

func TestUseVerificationTokenAtomicallyExpired(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	token := &enterprise.VerificationToken{
		Token:   "expired-atomic",
		UserID:  "user-1",
		Email:   "test@example.com",
		Expires: time.Now().Add(-1 * time.Hour), // Already expired
		Used:    false,
	}

	storage.SaveVerificationToken(token)

	_, err := storage.UseVerificationTokenAtomically("expired-atomic")
	if err == nil {
		t.Error("expected error for expired token")
	}
}

// Export/Import tests

func TestExportAndImportData(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	// Create some data
	storage.CreateTenant(&enterprise.Tenant{
		ID:             "tenant-1",
		Domain:         "test.example.com",
		StorageQuotaMB: 512,
	})
	storage.CreateUser(&enterprise.ClusterUser{
		ID:    "user-1",
		Email: "test@example.com",
	})

	// Export data
	var entries []SnapshotEntry
	err := storage.ExportData(func(key, value []byte) error {
		entries = append(entries, SnapshotEntry{Key: key, Value: value})
		return nil
	})
	if err != nil {
		t.Fatalf("failed to export data: %v", err)
	}

	if len(entries) == 0 {
		t.Error("expected some entries to be exported")
	}

	// Create new storage and import
	storage2, cleanup2 := createTestStorage(t)
	defer cleanup2()

	err = storage2.ImportData(entries)
	if err != nil {
		t.Fatalf("failed to import data: %v", err)
	}

	// Verify data was imported
	tenant, err := storage2.GetTenant("tenant-1")
	if err != nil {
		t.Fatalf("failed to get imported tenant: %v", err)
	}

	if tenant.StorageQuotaMB != 512 {
		t.Errorf("expected StorageQuotaMB 512, got %d", tenant.StorageQuotaMB)
	}

	user, err := storage2.GetUser("user-1")
	if err != nil {
		t.Fatalf("failed to get imported user: %v", err)
	}

	if user.Email != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", user.Email)
	}
}

func TestGetDiskManager(t *testing.T) {
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	dm := storage.GetDiskManager()
	if dm == nil {
		t.Error("expected non-nil disk manager")
	}
}
