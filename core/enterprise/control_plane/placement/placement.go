package placement

import (
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
)

// Storage interface for placement service
type Storage interface {
	GetTenant(tenantID string) (*enterprise.Tenant, error)
	UpdateTenant(tenant *enterprise.Tenant) error
	ListNodes() ([]*enterprise.NodeInfo, error)
	SavePlacement(placement *enterprise.PlacementDecision) error
	GetPlacement(tenantID string) (*enterprise.PlacementDecision, error)
}

// Service handles tenant placement decisions
type Service struct {
	storage  Storage
	strategy enterprise.PlacementStrategy
}

// NewService creates a new placement service
func NewService(storage Storage, strategy enterprise.PlacementStrategy) *Service {
	if strategy == nil {
		// Use default strategy
		strategy = &LeastLoadedStrategy{}
	}

	return &Service{
		storage:  storage,
		strategy: strategy,
	}
}

// AssignTenant assigns a tenant to a node
func (s *Service) AssignTenant(tenantID string) (*enterprise.PlacementDecision, error) {
	// Check if tenant already has a placement
	existing, err := s.storage.GetPlacement(tenantID)
	if err == nil && existing != nil {
		// Already placed
		return existing, nil
	}

	// Get tenant metadata
	tenant, err := s.storage.GetTenant(tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	// Get available nodes
	nodes, err := s.storage.ListNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Filter to only healthy nodes
	healthyNodes := make([]*enterprise.NodeInfo, 0)
	for _, node := range nodes {
		if enterprise.IsNodeHealthy(node, 30*time.Second) {
			healthyNodes = append(healthyNodes, node)
		}
	}

	if len(healthyNodes) == 0 {
		return nil, enterprise.ErrNoHealthyNodes
	}

	// Use strategy to select node
	selectedNode, err := s.strategy.SelectNode(tenant, healthyNodes)
	if err != nil {
		return nil, fmt.Errorf("failed to select node: %w", err)
	}

	// Create placement decision
	decision := &enterprise.PlacementDecision{
		TenantID:  tenantID,
		NodeID:    selectedNode.ID,
		Reason:    "Initial placement",
		DecidedAt: time.Now(),
	}

	// Save placement
	if err := s.storage.SavePlacement(decision); err != nil {
		return nil, fmt.Errorf("failed to save placement: %w", err)
	}

	// Update tenant with assignment
	tenant.AssignedNodeID = selectedNode.ID
	tenant.AssignedAt = time.Now()
	tenant.Status = enterprise.TenantStatusAssigning

	if err := s.storage.UpdateTenant(tenant); err != nil {
		return nil, fmt.Errorf("failed to update tenant: %w", err)
	}

	return decision, nil
}

// CheckRebalance checks if rebalancing is needed
func (s *Service) CheckRebalance() error {
	nodes, err := s.storage.ListNodes()
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	if !s.strategy.ShouldRebalance(nodes) {
		return nil
	}

	// TODO: Implement rebalancing logic
	// This would involve generating a rebalance plan and migrating tenants

	return nil
}

// LeastLoadedStrategy selects the node with the lowest load
type LeastLoadedStrategy struct{}

// SelectNode selects the least loaded node
func (s *LeastLoadedStrategy) SelectNode(tenant *enterprise.Tenant, nodes []*enterprise.NodeInfo) (*enterprise.NodeInfo, error) {
	node := enterprise.SelectLeastLoadedNode(nodes)
	if node == nil {
		return nil, enterprise.ErrNoHealthyNodes
	}
	return node, nil
}

// ShouldRebalance determines if rebalancing is needed
func (s *LeastLoadedStrategy) ShouldRebalance(nodes []*enterprise.NodeInfo) bool {
	if len(nodes) < 2 {
		return false
	}

	// Calculate load variance
	var maxLoad, minLoad float64 = 0, 100
	for _, node := range nodes {
		if !enterprise.IsNodeHealthy(node, 30*time.Second) {
			continue
		}

		load := enterprise.CalculateNodeLoad(node)
		if load > maxLoad {
			maxLoad = load
		}
		if load < minLoad {
			minLoad = load
		}
	}

	// Rebalance if variance is greater than 30%
	variance := maxLoad - minLoad
	return variance > 30
}

// GenerateRebalancePlan creates a rebalancing plan
func (s *LeastLoadedStrategy) GenerateRebalancePlan(nodes []*enterprise.NodeInfo, tenants []*enterprise.Tenant) ([]*enterprise.PlacementDecision, error) {
	// TODO: Implement rebalancing algorithm
	return nil, nil
}
