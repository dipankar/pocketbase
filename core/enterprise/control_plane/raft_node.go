package control_plane

import (
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
	"github.com/pocketbase/pocketbase/core/enterprise/control_plane/raft"
)

// RaftNode wraps the Raft consensus implementation
type RaftNode struct {
	node    *raft.Node
	storage *BadgerStorage
}

// NewRaftNode creates a new Raft node for the control plane
func NewRaftNode(config *enterprise.ClusterConfig, storage *BadgerStorage) (*RaftNode, error) {
	// Create FSM with callbacks to storage
	fsm := raft.NewFSM(
		storage.ApplyRaftLog,
		storage.Snapshot,
		storage.Restore,
	)

	// Create Raft node
	node, err := raft.NewNode(config, fsm)
	if err != nil {
		return nil, err
	}

	return &RaftNode{
		node:    node,
		storage: storage,
	}, nil
}

// IsLeader returns true if this node is the Raft leader
func (rn *RaftNode) IsLeader() bool {
	return rn.node.IsLeader()
}

// Apply applies a command to the Raft log
// This should be used for all state-changing operations
func (rn *RaftNode) Apply(cmd []byte) error {
	return rn.node.Apply(cmd, 10*time.Second)
}

// Shutdown gracefully shuts down the Raft node
func (rn *RaftNode) Shutdown() error {
	return rn.node.Shutdown()
}

// GetLeader returns the current leader address
func (rn *RaftNode) GetLeader() string {
	return rn.node.GetLeader()
}

// GetStats returns Raft statistics
func (rn *RaftNode) GetStats() map[string]string {
	return rn.node.GetStats()
}
