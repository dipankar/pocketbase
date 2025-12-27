package raft

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/pocketbase/pocketbase/core/enterprise"
)

// Node represents a Raft consensus node
type Node struct {
	raft   *raft.Raft
	config *enterprise.ClusterConfig
	fsm    *FSM
}

// NewNode creates a new Raft node
func NewNode(config *enterprise.ClusterConfig, fsm *FSM) (*Node, error) {
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(config.NodeID)

	// Create data directory
	raftDir := filepath.Join(config.DataDir, "raft")
	if err := os.MkdirAll(raftDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create raft directory: %w", err)
	}

	// Create stable store (for Raft metadata)
	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "stable.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create stable store: %w", err)
	}

	// Create log store (for Raft log)
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "logs.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create log store: %w", err)
	}

	// Create snapshot store (for Raft snapshots)
	snapshotStore, err := raft.NewFileSnapshotStore(raftDir, 3, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %w", err)
	}

	// Create TCP transport
	bindAddr := config.RaftBindAddr
	if bindAddr == "" {
		bindAddr = "127.0.0.1:7000"
	}

	addr, err := net.ResolveTCPAddr("tcp", bindAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve bind address: %w", err)
	}

	transport, err := raft.NewTCPTransport(bindAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create TCP transport: %w", err)
	}

	// Create Raft instance
	r, err := raft.NewRaft(raftConfig, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft instance: %w", err)
	}

	node := &Node{
		raft:   r,
		config: config,
		fsm:    fsm,
	}

	// Bootstrap cluster if this is the first node
	if len(config.RaftPeers) > 0 {
		// Start with this node in the configuration
		servers := []raft.Server{
			{
				ID:      raft.ServerID(config.NodeID),
				Address: transport.LocalAddr(),
			},
		}

		// Parse and add peer nodes from config
		// RaftPeers format: ["node1:7000", "node2:7000", "node3:7000"]
		// Each peer address should be formatted as "host:port"
		for i, peerAddr := range config.RaftPeers {
			// Skip if this is our own address
			if peerAddr == bindAddr {
				continue
			}

			// Generate a server ID for the peer (could be specified in config in the future)
			// For now, use a simple numeric suffix
			peerID := fmt.Sprintf("node%d", i+1)

			// If the peer address matches our bind address, use our node ID
			peerTCPAddr, err := net.ResolveTCPAddr("tcp", peerAddr)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve peer address %s: %w", peerAddr, err)
			}

			servers = append(servers, raft.Server{
				ID:      raft.ServerID(peerID),
				Address: raft.ServerAddress(peerTCPAddr.String()),
			})
		}

		configuration := raft.Configuration{
			Servers: servers,
		}

		// Bootstrap cluster with all peers
		// Note: BootstrapCluster should only be called on ONE node in the cluster
		// All nodes should have the same peer list, but only one should actually bootstrap
		f := r.BootstrapCluster(configuration)
		if err := f.Error(); err != nil {
			// Ignore error if already bootstrapped
			// This is expected if another node already bootstrapped the cluster
			if err != raft.ErrCantBootstrap {
				return nil, fmt.Errorf("failed to bootstrap cluster: %w", err)
			}
		}
	}

	return node, nil
}

// IsLeader returns true if this node is the Raft leader
func (n *Node) IsLeader() bool {
	return n.raft.State() == raft.Leader
}

// Apply applies a command to the Raft log
func (n *Node) Apply(cmd []byte, timeout time.Duration) error {
	future := n.raft.Apply(cmd, timeout)
	return future.Error()
}

// Shutdown gracefully shuts down the Raft node
func (n *Node) Shutdown() error {
	future := n.raft.Shutdown()
	return future.Error()
}

// GetLeader returns the current leader address
func (n *Node) GetLeader() string {
	addr, _ := n.raft.LeaderWithID()
	return string(addr)
}

// GetStats returns Raft stats
func (n *Node) GetStats() map[string]string {
	return n.raft.Stats()
}
