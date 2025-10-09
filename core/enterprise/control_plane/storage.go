package control_plane

import (
	"github.com/pocketbase/pocketbase/core/enterprise"
	"github.com/pocketbase/pocketbase/core/enterprise/control_plane/badger"
)

// BadgerStorage wraps the BadgerDB storage implementation
type BadgerStorage struct {
	*badger.Storage
}

// NewBadgerStorage creates a new BadgerDB storage wrapper
func NewBadgerStorage(dataDir string) (*BadgerStorage, error) {
	storage, err := badger.NewStorage(dataDir)
	if err != nil {
		return nil, err
	}

	return &BadgerStorage{Storage: storage}, nil
}

// ApplyRaftLog applies a Raft log entry to the storage
// This is called by the Raft FSM when a log is committed
func (s *BadgerStorage) ApplyRaftLog(logData []byte) error {
	// TODO: Parse log entry and apply the appropriate operation
	// For now, this is a placeholder
	return nil
}

// Snapshot creates a snapshot of the current state for Raft
func (s *BadgerStorage) Snapshot() ([]byte, error) {
	// TODO: Create a snapshot of the BadgerDB state
	// For now, return empty
	return []byte{}, nil
}

// Restore restores state from a Raft snapshot
func (s *BadgerStorage) Restore(snapshot []byte) error {
	// TODO: Restore BadgerDB state from snapshot
	// For now, this is a placeholder
	return nil
}

// These methods wrap the underlying badger.Storage methods
// and will be extended to work with Raft replication

func (s *BadgerStorage) CreateTenant(tenant *enterprise.Tenant) error {
	// TODO: Wrap in Raft log proposal
	return s.Storage.CreateTenant(tenant)
}

func (s *BadgerStorage) UpdateTenant(tenant *enterprise.Tenant) error {
	// TODO: Wrap in Raft log proposal
	return s.Storage.UpdateTenant(tenant)
}

func (s *BadgerStorage) UpdateTenantStatus(tenantID string, status enterprise.TenantStatus) error {
	// TODO: Wrap in Raft log proposal
	return s.Storage.UpdateTenantStatus(tenantID, status)
}

func (s *BadgerStorage) CreateUser(user *enterprise.ClusterUser) error {
	// TODO: Wrap in Raft log proposal
	return s.Storage.CreateUser(user)
}

func (s *BadgerStorage) UpdateUser(user *enterprise.ClusterUser) error {
	// TODO: Wrap in Raft log proposal
	return s.Storage.UpdateUser(user)
}

func (s *BadgerStorage) SaveNode(node *enterprise.NodeInfo) error {
	// TODO: Wrap in Raft log proposal
	return s.Storage.SaveNode(node)
}

func (s *BadgerStorage) SavePlacement(placement *enterprise.PlacementDecision) error {
	// TODO: Wrap in Raft log proposal
	return s.Storage.SavePlacement(placement)
}
