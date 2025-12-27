package raft

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/pocketbase/pocketbase/core/enterprise"
)

// ===== FSM Tests =====

func TestNewFSM(t *testing.T) {
	applyFunc := func(data []byte) error { return nil }
	snapshotFunc := func() ([]byte, error) { return []byte("snapshot"), nil }
	restoreFunc := func(data []byte) error { return nil }

	fsm := NewFSM(applyFunc, snapshotFunc, restoreFunc)

	if fsm == nil {
		t.Fatal("expected non-nil FSM")
	}

	if fsm.applyFunc == nil {
		t.Error("expected non-nil applyFunc")
	}

	if fsm.snapshotFunc == nil {
		t.Error("expected non-nil snapshotFunc")
	}

	if fsm.restoreFunc == nil {
		t.Error("expected non-nil restoreFunc")
	}
}

func TestNewFSMWithNilFunctions(t *testing.T) {
	fsm := NewFSM(nil, nil, nil)

	if fsm == nil {
		t.Fatal("expected non-nil FSM even with nil functions")
	}
}

func TestFSMApply(t *testing.T) {
	var appliedData []byte
	applyFunc := func(data []byte) error {
		appliedData = data
		return nil
	}

	fsm := NewFSM(applyFunc, nil, nil)

	log := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  []byte("test command"),
	}

	result := fsm.Apply(log)

	// Should return nil (no error)
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}

	if !bytes.Equal(appliedData, []byte("test command")) {
		t.Errorf("expected applied data 'test command', got '%s'", appliedData)
	}
}

func TestFSMApplyWithError(t *testing.T) {
	expectedErr := errors.New("apply error")
	applyFunc := func(data []byte) error {
		return expectedErr
	}

	fsm := NewFSM(applyFunc, nil, nil)

	log := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  []byte("test"),
	}

	result := fsm.Apply(log)

	if result != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, result)
	}
}

func TestFSMApplyWithNilApplyFunc(t *testing.T) {
	fsm := NewFSM(nil, nil, nil)

	log := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  []byte("test"),
	}

	result := fsm.Apply(log)

	if result != nil {
		t.Errorf("expected nil result when applyFunc is nil, got %v", result)
	}
}

func TestFSMSnapshot(t *testing.T) {
	snapshotData := []byte("snapshot data")
	snapshotFunc := func() ([]byte, error) {
		return snapshotData, nil
	}

	fsm := NewFSM(nil, snapshotFunc, nil)

	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}

	// Verify the snapshot is of the correct type
	fsmSnap, ok := snapshot.(*fsmSnapshot)
	if !ok {
		t.Fatal("expected *fsmSnapshot type")
	}

	if !bytes.Equal(fsmSnap.data, snapshotData) {
		t.Errorf("expected snapshot data '%s', got '%s'", snapshotData, fsmSnap.data)
	}
}

func TestFSMSnapshotWithError(t *testing.T) {
	expectedErr := errors.New("snapshot error")
	snapshotFunc := func() ([]byte, error) {
		return nil, expectedErr
	}

	fsm := NewFSM(nil, snapshotFunc, nil)

	_, err := fsm.Snapshot()

	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestFSMSnapshotWithNilSnapshotFunc(t *testing.T) {
	fsm := NewFSM(nil, nil, nil)

	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot even with nil snapshotFunc")
	}

	fsmSnap, ok := snapshot.(*fsmSnapshot)
	if !ok {
		t.Fatal("expected *fsmSnapshot type")
	}

	if len(fsmSnap.data) != 0 {
		t.Errorf("expected empty snapshot data, got %d bytes", len(fsmSnap.data))
	}
}

func TestFSMRestore(t *testing.T) {
	var restoredData []byte
	restoreFunc := func(data []byte) error {
		restoredData = data
		return nil
	}

	fsm := NewFSM(nil, nil, restoreFunc)

	snapshotData := []byte("restore this data")
	reader := io.NopCloser(bytes.NewReader(snapshotData))

	err := fsm.Restore(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(restoredData, snapshotData) {
		t.Errorf("expected restored data '%s', got '%s'", snapshotData, restoredData)
	}
}

func TestFSMRestoreWithError(t *testing.T) {
	expectedErr := errors.New("restore error")
	restoreFunc := func(data []byte) error {
		return expectedErr
	}

	fsm := NewFSM(nil, nil, restoreFunc)

	reader := io.NopCloser(bytes.NewReader([]byte("data")))

	err := fsm.Restore(reader)

	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestFSMRestoreWithNilRestoreFunc(t *testing.T) {
	fsm := NewFSM(nil, nil, nil)

	reader := io.NopCloser(bytes.NewReader([]byte("data")))

	err := fsm.Restore(reader)
	if err != nil {
		t.Errorf("expected nil error when restoreFunc is nil, got %v", err)
	}
}

func TestFSMRestoreWithReadError(t *testing.T) {
	restoreFunc := func(data []byte) error {
		return nil
	}

	fsm := NewFSM(nil, nil, restoreFunc)

	// Create a reader that returns an error
	reader := io.NopCloser(&errorReader{})

	err := fsm.Restore(reader)
	if err == nil {
		t.Error("expected error from reader")
	}
}

// errorReader is a helper that always returns an error
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

// ===== fsmSnapshot Tests =====

func TestFsmSnapshotPersist(t *testing.T) {
	snapshot := &fsmSnapshot{
		data: []byte("snapshot content"),
	}

	sink := &mockSnapshotSink{
		buf: &bytes.Buffer{},
	}

	err := snapshot.Persist(sink)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(sink.buf.Bytes(), []byte("snapshot content")) {
		t.Errorf("expected 'snapshot content', got '%s'", sink.buf.Bytes())
	}

	if !sink.closed {
		t.Error("expected sink to be closed")
	}
}

func TestFsmSnapshotPersistWriteError(t *testing.T) {
	snapshot := &fsmSnapshot{
		data: []byte("snapshot content"),
	}

	sink := &mockSnapshotSink{
		writeErr: errors.New("write error"),
	}

	err := snapshot.Persist(sink)
	if err == nil {
		t.Fatal("expected error")
	}

	if !sink.cancelled {
		t.Error("expected sink to be cancelled on error")
	}
}

func TestFsmSnapshotRelease(t *testing.T) {
	snapshot := &fsmSnapshot{
		data: []byte("snapshot content"),
	}

	// Release should not panic
	snapshot.Release()
}

func TestFsmSnapshotEmptyData(t *testing.T) {
	snapshot := &fsmSnapshot{
		data: []byte{},
	}

	sink := &mockSnapshotSink{
		buf: &bytes.Buffer{},
	}

	err := snapshot.Persist(sink)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sink.buf.Len() != 0 {
		t.Errorf("expected empty buffer, got %d bytes", sink.buf.Len())
	}
}

// mockSnapshotSink implements raft.SnapshotSink for testing
type mockSnapshotSink struct {
	buf       *bytes.Buffer
	writeErr  error
	closed    bool
	cancelled bool
}

func (m *mockSnapshotSink) Write(p []byte) (n int, err error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	if m.buf == nil {
		m.buf = &bytes.Buffer{}
	}
	return m.buf.Write(p)
}

func (m *mockSnapshotSink) Close() error {
	m.closed = true
	return nil
}

func (m *mockSnapshotSink) Cancel() error {
	m.cancelled = true
	return nil
}

func (m *mockSnapshotSink) ID() string {
	return "mock-snapshot-id"
}

// ===== Node configuration tests =====

func TestNodeConfigDefaults(t *testing.T) {
	// Test that default bind address is set when not provided
	config := &enterprise.ClusterConfig{
		Mode:    enterprise.ModeControlPlane,
		NodeID:  "test-node",
		DataDir: t.TempDir(),
	}

	// RaftBindAddr should default to empty, which means 127.0.0.1:7000 in NewNode
	if config.RaftBindAddr != "" {
		t.Errorf("expected empty RaftBindAddr by default, got %s", config.RaftBindAddr)
	}
}

func TestNodeConfigWithPeers(t *testing.T) {
	config := &enterprise.ClusterConfig{
		Mode:         enterprise.ModeControlPlane,
		NodeID:       "node1",
		DataDir:      t.TempDir(),
		RaftBindAddr: "127.0.0.1:7001",
		RaftPeers:    []string{"127.0.0.1:7001", "127.0.0.1:7002", "127.0.0.1:7003"},
	}

	// Validate peer list
	if len(config.RaftPeers) != 3 {
		t.Errorf("expected 3 peers, got %d", len(config.RaftPeers))
	}
}

// ===== Integration-style tests (FSM with real operations) =====

func TestFSMFullCycle(t *testing.T) {
	// Simulate a key-value store FSM
	store := make(map[string]string)

	applyFunc := func(data []byte) error {
		// Simple format: "key=value"
		parts := bytes.SplitN(data, []byte("="), 2)
		if len(parts) == 2 {
			store[string(parts[0])] = string(parts[1])
		}
		return nil
	}

	snapshotFunc := func() ([]byte, error) {
		var buf bytes.Buffer
		for k, v := range store {
			buf.WriteString(k)
			buf.WriteByte('=')
			buf.WriteString(v)
			buf.WriteByte('\n')
		}
		return buf.Bytes(), nil
	}

	restoreFunc := func(data []byte) error {
		store = make(map[string]string)
		lines := bytes.Split(data, []byte("\n"))
		for _, line := range lines {
			if len(line) == 0 {
				continue
			}
			parts := bytes.SplitN(line, []byte("="), 2)
			if len(parts) == 2 {
				store[string(parts[0])] = string(parts[1])
			}
		}
		return nil
	}

	fsm := NewFSM(applyFunc, snapshotFunc, restoreFunc)

	// Apply some commands
	commands := []string{"key1=value1", "key2=value2", "key3=value3"}
	for i, cmd := range commands {
		log := &raft.Log{
			Index: uint64(i + 1),
			Term:  1,
			Type:  raft.LogCommand,
			Data:  []byte(cmd),
		}
		fsm.Apply(log)
	}

	// Verify store state
	if store["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %s", store["key1"])
	}
	if store["key2"] != "value2" {
		t.Errorf("expected key2=value2, got %s", store["key2"])
	}
	if store["key3"] != "value3" {
		t.Errorf("expected key3=value3, got %s", store["key3"])
	}

	// Create snapshot
	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}

	// Persist snapshot
	sink := &mockSnapshotSink{buf: &bytes.Buffer{}}
	err = snapshot.Persist(sink)
	if err != nil {
		t.Fatalf("failed to persist snapshot: %v", err)
	}

	// Clear store and restore
	store = make(map[string]string)
	if len(store) != 0 {
		t.Fatal("store should be empty after clear")
	}

	reader := io.NopCloser(bytes.NewReader(sink.buf.Bytes()))
	err = fsm.Restore(reader)
	if err != nil {
		t.Fatalf("failed to restore: %v", err)
	}

	// Verify restored state
	if store["key1"] != "value1" {
		t.Errorf("after restore: expected key1=value1, got %s", store["key1"])
	}
	if store["key2"] != "value2" {
		t.Errorf("after restore: expected key2=value2, got %s", store["key2"])
	}
	if store["key3"] != "value3" {
		t.Errorf("after restore: expected key3=value3, got %s", store["key3"])
	}
}

func TestFSMApplyMultipleCommands(t *testing.T) {
	var commands []string
	applyFunc := func(data []byte) error {
		commands = append(commands, string(data))
		return nil
	}

	fsm := NewFSM(applyFunc, nil, nil)

	for i := 0; i < 100; i++ {
		log := &raft.Log{
			Index: uint64(i + 1),
			Term:  1,
			Type:  raft.LogCommand,
			Data:  []byte("command"),
		}
		fsm.Apply(log)
	}

	if len(commands) != 100 {
		t.Errorf("expected 100 commands applied, got %d", len(commands))
	}
}

func TestFSMSnapshotLargeData(t *testing.T) {
	// Create 1MB of snapshot data
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	snapshotFunc := func() ([]byte, error) {
		return largeData, nil
	}

	fsm := NewFSM(nil, snapshotFunc, nil)

	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fsmSnap := snapshot.(*fsmSnapshot)
	if len(fsmSnap.data) != 1024*1024 {
		t.Errorf("expected 1MB of data, got %d bytes", len(fsmSnap.data))
	}
}

// ===== Log type tests =====

func TestFSMApplyDifferentLogTypes(t *testing.T) {
	applyCount := 0
	applyFunc := func(data []byte) error {
		applyCount++
		return nil
	}

	fsm := NewFSM(applyFunc, nil, nil)

	// Command log should apply
	cmdLog := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  []byte("command"),
	}
	fsm.Apply(cmdLog)

	// Noop log should also apply (the FSM receives all logs)
	noopLog := &raft.Log{
		Index: 2,
		Term:  1,
		Type:  raft.LogNoop,
		Data:  nil,
	}
	fsm.Apply(noopLog)

	if applyCount != 2 {
		t.Errorf("expected 2 applies, got %d", applyCount)
	}
}

// ===== Concurrent access tests =====

func TestFSMConcurrentApply(t *testing.T) {
	var count int
	applyFunc := func(data []byte) error {
		count++
		return nil
	}

	fsm := NewFSM(applyFunc, nil, nil)

	// Note: In real Raft, applies are serialized by the Raft library
	// This test just verifies the FSM doesn't crash with concurrent calls
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			log := &raft.Log{
				Index: uint64(idx + 1),
				Term:  1,
				Type:  raft.LogCommand,
				Data:  []byte("concurrent"),
			}
			fsm.Apply(log)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Count may vary due to race conditions without mutex, but should be > 0
	if count == 0 {
		t.Error("expected some applies to succeed")
	}
}
