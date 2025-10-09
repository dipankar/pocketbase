package raft

import (
	"io"

	"github.com/hashicorp/raft"
)

// FSM implements the Raft Finite State Machine
type FSM struct {
	// applyFunc is called when a log entry is committed
	// It receives the log data and should apply it to the state
	applyFunc func([]byte) error

	// snapshotFunc creates a snapshot of the current state
	snapshotFunc func() ([]byte, error)

	// restoreFunc restores state from a snapshot
	restoreFunc func([]byte) error
}

// NewFSM creates a new Raft FSM
func NewFSM(applyFunc func([]byte) error, snapshotFunc func() ([]byte, error), restoreFunc func([]byte) error) *FSM {
	return &FSM{
		applyFunc:    applyFunc,
		snapshotFunc: snapshotFunc,
		restoreFunc:  restoreFunc,
	}
}

// Apply applies a Raft log entry to the FSM
func (f *FSM) Apply(log *raft.Log) interface{} {
	if f.applyFunc == nil {
		return nil
	}

	err := f.applyFunc(log.Data)
	return err
}

// Snapshot returns a snapshot of the FSM state
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	if f.snapshotFunc == nil {
		return &fsmSnapshot{data: []byte{}}, nil
	}

	data, err := f.snapshotFunc()
	if err != nil {
		return nil, err
	}

	return &fsmSnapshot{data: data}, nil
}

// Restore restores the FSM state from a snapshot
func (f *FSM) Restore(snapshot io.ReadCloser) error {
	defer snapshot.Close()

	if f.restoreFunc == nil {
		return nil
	}

	data, err := io.ReadAll(snapshot)
	if err != nil {
		return err
	}

	return f.restoreFunc(data)
}

// fsmSnapshot represents a point-in-time snapshot of the FSM
type fsmSnapshot struct {
	data []byte
}

// Persist writes the snapshot to the given sink
func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	if _, err := sink.Write(s.data); err != nil {
		sink.Cancel()
		return err
	}

	return sink.Close()
}

// Release is called when the snapshot is no longer needed
func (s *fsmSnapshot) Release() {
	// Nothing to release
}
