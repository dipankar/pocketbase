package control_plane

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
	"github.com/pocketbase/pocketbase/core/enterprise/control_plane/badger"
)

// BadgerStorage wraps the BadgerDB storage implementation
type BadgerStorage struct {
	*badger.Storage
	raftNode *RaftNode // Reference to Raft node for log proposals
}

// NewBadgerStorage creates a new BadgerDB storage wrapper
func NewBadgerStorage(dataDir string) (*BadgerStorage, error) {
	storage, err := badger.NewStorage(dataDir)
	if err != nil {
		return nil, err
	}

	return &BadgerStorage{
		Storage:  storage,
		raftNode: nil, // Set later via SetRaftNode
	}, nil
}

// SetRaftNode sets the Raft node reference
func (s *BadgerStorage) SetRaftNode(raftNode *RaftNode) {
	s.raftNode = raftNode
}

// ErrNotLeader is returned when a write operation is attempted on a non-leader node
var ErrNotLeader = fmt.Errorf("not the Raft leader")

// proposeCommand proposes a command via Raft consensus
func (s *BadgerStorage) proposeCommand(cmd *RaftCommand) error {
	// Single-node mode (no Raft configured) - apply directly
	if s.raftNode == nil {
		return s.ApplyRaftLog(cmd)
	}

	// Check if we are the leader
	if !s.raftNode.IsLeader() {
		// Return error with leader address for client to redirect
		leaderAddr := s.raftNode.GetLeader()
		if leaderAddr != "" {
			return fmt.Errorf("%w: leader is %s", ErrNotLeader, leaderAddr)
		}
		return ErrNotLeader
	}

	// Encode command
	data, err := cmd.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode command: %w", err)
	}

	// Propose via Raft with 10 second timeout
	return s.raftNode.Apply(data, 10*time.Second)
}

// ApplyRaftLog applies a Raft log entry to the storage
// This is called by the Raft FSM when a log is committed
func (s *BadgerStorage) ApplyRaftLog(cmd *RaftCommand) error {
	// Check if we received a valid command
	if cmd == nil {
		return fmt.Errorf("nil command")
	}

	// Apply the command based on type
	switch cmd.Type {
	case CommandCreateTenant:
		var payload CreateTenantPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal tenant payload: %w", err)
		}
		return s.Storage.CreateTenant(payload.Tenant)

	case CommandUpdateTenant:
		var payload UpdateTenantPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal tenant payload: %w", err)
		}
		return s.Storage.UpdateTenant(payload.Tenant)

	case CommandUpdateTenantStatus:
		var payload UpdateTenantStatusPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal status payload: %w", err)
		}
		return s.Storage.UpdateTenantStatus(payload.TenantID, payload.Status)

	case CommandCreateUser:
		var payload CreateUserPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal user payload: %w", err)
		}
		return s.Storage.CreateUser(payload.User)

	case CommandUpdateUser:
		var payload UpdateUserPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal user payload: %w", err)
		}
		return s.Storage.UpdateUser(payload.User)

	case CommandSaveNode:
		var payload SaveNodePayload
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal node payload: %w", err)
		}
		return s.Storage.SaveNode(payload.Node)

	case CommandSavePlacement:
		var payload SavePlacementPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal placement payload: %w", err)
		}
		return s.Storage.SavePlacement(payload.Placement)

	case CommandSaveActivity:
		var payload SaveActivityPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal activity payload: %w", err)
		}
		return s.Storage.SaveActivity(payload.Activity)

	case CommandSaveToken:
		var payload SaveTokenPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal token payload: %w", err)
		}
		return s.Storage.SaveVerificationToken(payload.Token)

	case CommandMarkTokenUsed:
		var payload MarkTokenUsedPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal token payload: %w", err)
		}
		return s.Storage.MarkVerificationTokenUsed(payload.Token)

	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// Current snapshot format version
const SnapshotVersion = 1

// SnapshotData represents the complete snapshot data
type SnapshotData struct {
	Version   int                    `json:"version"`   // Snapshot format version
	CreatedAt time.Time              `json:"createdAt"` // When snapshot was created
	NodeID    string                 `json:"nodeId"`    // Node that created the snapshot
	Checksum  string                 `json:"checksum"`  // CRC32 checksum of entries
	Entries   []badger.SnapshotEntry `json:"entries"`
}

// SnapshotMigrator handles migration between snapshot versions
type SnapshotMigrator struct {
	migrations map[int]func(*SnapshotData) error
}

// NewSnapshotMigrator creates a new migrator with all version migrations
func NewSnapshotMigrator() *SnapshotMigrator {
	m := &SnapshotMigrator{
		migrations: make(map[int]func(*SnapshotData) error),
	}

	// Register migrations (from version N to N+1)
	// Example: m.migrations[1] = migrateV1ToV2

	return m
}

// Migrate applies all necessary migrations to bring snapshot to current version
func (m *SnapshotMigrator) Migrate(snapshot *SnapshotData) error {
	for snapshot.Version < SnapshotVersion {
		migrateFn, exists := m.migrations[snapshot.Version]
		if !exists {
			return fmt.Errorf("no migration path from version %d to %d", snapshot.Version, snapshot.Version+1)
		}

		if err := migrateFn(snapshot); err != nil {
			return fmt.Errorf("migration from version %d failed: %w", snapshot.Version, err)
		}

		snapshot.Version++
	}

	return nil
}

// Snapshot creates a snapshot of the current state for Raft
func (s *BadgerStorage) Snapshot() ([]byte, error) {
	snapshot := SnapshotData{
		Version:   SnapshotVersion,
		CreatedAt: time.Now(),
		Entries:   make([]badger.SnapshotEntry, 0),
	}

	// Export all key-value pairs from BadgerDB
	err := s.Storage.ExportData(func(key, value []byte) error {
		snapshot.Entries = append(snapshot.Entries, badger.SnapshotEntry{
			Key:   append([]byte(nil), key...),   // Copy key
			Value: append([]byte(nil), value...), // Copy value
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to export data for snapshot: %w", err)
	}

	// Calculate checksum of entries
	snapshot.Checksum = calculateEntriesChecksum(snapshot.Entries)

	// Serialize snapshot to JSON
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	return data, nil
}

// calculateEntriesChecksum computes a CRC32 checksum of all entries
func calculateEntriesChecksum(entries []badger.SnapshotEntry) string {
	h := crc32.NewIEEE()
	for _, entry := range entries {
		h.Write(entry.Key)
		h.Write(entry.Value)
	}
	return fmt.Sprintf("%08x", h.Sum32())
}

// Restore restores state from a Raft snapshot
func (s *BadgerStorage) Restore(snapshotData []byte) error {
	// Deserialize snapshot
	var snapshot SnapshotData
	if err := json.Unmarshal(snapshotData, &snapshot); err != nil {
		return fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	// Check and migrate snapshot version if needed
	if snapshot.Version > SnapshotVersion {
		return fmt.Errorf("snapshot version %d is newer than supported version %d", snapshot.Version, SnapshotVersion)
	}

	if snapshot.Version < SnapshotVersion {
		migrator := NewSnapshotMigrator()
		if err := migrator.Migrate(&snapshot); err != nil {
			return fmt.Errorf("failed to migrate snapshot: %w", err)
		}
	}

	// Verify checksum if present (older snapshots may not have it)
	if snapshot.Checksum != "" {
		calculatedChecksum := calculateEntriesChecksum(snapshot.Entries)
		if calculatedChecksum != snapshot.Checksum {
			return fmt.Errorf("snapshot checksum mismatch: expected %s, got %s", snapshot.Checksum, calculatedChecksum)
		}
	}

	// Clear existing data and restore from snapshot
	return s.Storage.ImportData(snapshot.Entries)
}

// These methods wrap the underlying badger.Storage methods
// and use Raft replication for consistency

func (s *BadgerStorage) CreateTenant(tenant *enterprise.Tenant) error {
	cmd, err := NewRaftCommand(CommandCreateTenant, CreateTenantPayload{Tenant: tenant})
	if err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}
	return s.proposeCommand(cmd)
}

func (s *BadgerStorage) UpdateTenant(tenant *enterprise.Tenant) error {
	cmd, err := NewRaftCommand(CommandUpdateTenant, UpdateTenantPayload{Tenant: tenant})
	if err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}
	return s.proposeCommand(cmd)
}

func (s *BadgerStorage) UpdateTenantStatus(tenantID string, status enterprise.TenantStatus) error {
	cmd, err := NewRaftCommand(CommandUpdateTenantStatus, UpdateTenantStatusPayload{
		TenantID: tenantID,
		Status:   status,
	})
	if err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}
	return s.proposeCommand(cmd)
}

func (s *BadgerStorage) CreateUser(user *enterprise.ClusterUser) error {
	cmd, err := NewRaftCommand(CommandCreateUser, CreateUserPayload{User: user})
	if err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}
	return s.proposeCommand(cmd)
}

func (s *BadgerStorage) UpdateUser(user *enterprise.ClusterUser) error {
	cmd, err := NewRaftCommand(CommandUpdateUser, UpdateUserPayload{User: user})
	if err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}
	return s.proposeCommand(cmd)
}

func (s *BadgerStorage) SaveNode(node *enterprise.NodeInfo) error {
	cmd, err := NewRaftCommand(CommandSaveNode, SaveNodePayload{Node: node})
	if err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}
	return s.proposeCommand(cmd)
}

func (s *BadgerStorage) SavePlacement(placement *enterprise.PlacementDecision) error {
	cmd, err := NewRaftCommand(CommandSavePlacement, SavePlacementPayload{Placement: placement})
	if err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}
	return s.proposeCommand(cmd)
}
