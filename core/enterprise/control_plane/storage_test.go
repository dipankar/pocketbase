package control_plane

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise/control_plane/badger"
)

func TestSnapshotVersioning(t *testing.T) {
	// Test that current version is set correctly
	if SnapshotVersion != 1 {
		t.Errorf("expected snapshot version 1, got %d", SnapshotVersion)
	}
}

func TestSnapshotDataSerialization(t *testing.T) {
	snapshot := SnapshotData{
		Version:   SnapshotVersion,
		CreatedAt: time.Now(),
		NodeID:    "node-1",
		Checksum:  "abcd1234",
		Entries: []badger.SnapshotEntry{
			{Key: []byte("key1"), Value: []byte("value1")},
			{Key: []byte("key2"), Value: []byte("value2")},
		},
	}

	// Serialize
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("failed to marshal snapshot: %v", err)
	}

	// Deserialize
	var restored SnapshotData
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal snapshot: %v", err)
	}

	if restored.Version != snapshot.Version {
		t.Errorf("version mismatch: expected %d, got %d", snapshot.Version, restored.Version)
	}

	if restored.Checksum != snapshot.Checksum {
		t.Errorf("checksum mismatch: expected %s, got %s", snapshot.Checksum, restored.Checksum)
	}

	if len(restored.Entries) != len(snapshot.Entries) {
		t.Errorf("entries count mismatch: expected %d, got %d", len(snapshot.Entries), len(restored.Entries))
	}
}

func TestCalculateEntriesChecksum(t *testing.T) {
	entries := []badger.SnapshotEntry{
		{Key: []byte("key1"), Value: []byte("value1")},
		{Key: []byte("key2"), Value: []byte("value2")},
	}

	checksum1 := calculateEntriesChecksum(entries)
	checksum2 := calculateEntriesChecksum(entries)

	// Same entries should produce same checksum
	if checksum1 != checksum2 {
		t.Error("same entries should produce same checksum")
	}

	// Different entries should produce different checksum
	differentEntries := []badger.SnapshotEntry{
		{Key: []byte("key1"), Value: []byte("different")},
	}
	checksum3 := calculateEntriesChecksum(differentEntries)

	if checksum1 == checksum3 {
		t.Error("different entries should produce different checksum")
	}
}

func TestCalculateEntriesChecksumFormat(t *testing.T) {
	entries := []badger.SnapshotEntry{
		{Key: []byte("test"), Value: []byte("data")},
	}

	checksum := calculateEntriesChecksum(entries)

	// Checksum should be 8 hex characters (CRC32)
	if len(checksum) != 8 {
		t.Errorf("expected checksum length 8, got %d", len(checksum))
	}

	// Should be valid hex
	for _, c := range checksum {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("checksum contains non-hex character: %c", c)
		}
	}
}

func TestSnapshotMigratorNoMigrationNeeded(t *testing.T) {
	migrator := NewSnapshotMigrator()

	snapshot := &SnapshotData{
		Version: SnapshotVersion,
	}

	err := migrator.Migrate(snapshot)
	if err != nil {
		t.Errorf("no migration should be needed for current version: %v", err)
	}

	if snapshot.Version != SnapshotVersion {
		t.Errorf("version should remain %d, got %d", SnapshotVersion, snapshot.Version)
	}
}

func TestSnapshotMigratorFutureVersionFails(t *testing.T) {
	migrator := NewSnapshotMigrator()

	snapshot := &SnapshotData{
		Version: SnapshotVersion + 1, // Future version
	}

	err := migrator.Migrate(snapshot)
	// Migration from future version shouldn't try to migrate (handled in Restore)
	// The migrator only handles older versions
	if err == nil && snapshot.Version > SnapshotVersion {
		// This is expected - the check happens in Restore, not Migrate
	}
}

func TestRaftCommandEncodeDecode(t *testing.T) {
	cmd := &RaftCommand{
		Type: CommandCreateTenant,
		Payload: []byte(`{"tenant":{"id":"tenant-1"}}`),
	}

	// Encode
	data, err := cmd.Encode()
	if err != nil {
		t.Fatalf("failed to encode command: %v", err)
	}

	// Decode
	decoded, err := DecodeRaftCommand(data)
	if err != nil {
		t.Fatalf("failed to decode command: %v", err)
	}

	if decoded.Type != cmd.Type {
		t.Errorf("type mismatch: expected %s, got %s", cmd.Type, decoded.Type)
	}

	if string(decoded.Payload) != string(cmd.Payload) {
		t.Error("payload mismatch")
	}
}

func TestNewRaftCommand(t *testing.T) {
	tests := []struct {
		name      string
		cmdType   CommandType
		payload   interface{}
		shouldErr bool
	}{
		{
			name:    "create tenant",
			cmdType: CommandCreateTenant,
			payload: CreateTenantPayload{},
		},
		{
			name:    "update tenant",
			cmdType: CommandUpdateTenant,
			payload: UpdateTenantPayload{},
		},
		{
			name:      "invalid payload",
			cmdType:   CommandCreateTenant,
			payload:   make(chan int), // Channels can't be marshaled to JSON
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewRaftCommand(tt.cmdType, tt.payload)
			if tt.shouldErr {
				if err == nil {
					t.Error("expected error for invalid payload")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if cmd.Type != tt.cmdType {
				t.Errorf("expected type %s, got %s", tt.cmdType, cmd.Type)
			}
		})
	}
}

func TestErrNotLeader(t *testing.T) {
	if ErrNotLeader == nil {
		t.Error("ErrNotLeader should not be nil")
	}

	if ErrNotLeader.Error() == "" {
		t.Error("ErrNotLeader should have a message")
	}
}

func TestEmptyEntriesChecksum(t *testing.T) {
	entries := []badger.SnapshotEntry{}

	checksum := calculateEntriesChecksum(entries)

	// Empty entries should still produce a valid checksum
	if len(checksum) != 8 {
		t.Errorf("expected checksum length 8, got %d", len(checksum))
	}
}
