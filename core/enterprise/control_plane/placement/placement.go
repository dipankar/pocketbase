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
	// ListTenantsByNode returns all tenants assigned to a specific node
	ListTenantsByNode(nodeID string) ([]*enterprise.Tenant, error)
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
		TenantID:    tenantID,
		NodeID:      selectedNode.ID,
		NodeAddress: selectedNode.Address, // Use the node's registered address
		Reason:      "Initial placement",
		DecidedAt:   time.Now(),
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

// CheckRebalance checks if rebalancing is needed and executes the plan
func (s *Service) CheckRebalance() error {
	nodes, err := s.storage.ListNodes()
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	if !s.strategy.ShouldRebalance(nodes) {
		return nil
	}

	// Get all tenants grouped by node
	nodeTenants := make(map[string][]*enterprise.Tenant)
	for _, node := range nodes {
		tenants, err := s.storage.ListTenantsByNode(node.ID)
		if err != nil {
			return fmt.Errorf("failed to list tenants for node %s: %w", node.ID, err)
		}
		nodeTenants[node.ID] = tenants
	}

	// Generate rebalancing plan
	plan, err := s.strategy.GenerateRebalancePlan(nodes, nodeTenants)
	if err != nil {
		return fmt.Errorf("failed to generate rebalance plan: %w", err)
	}

	if len(plan) == 0 {
		return nil // Nothing to rebalance
	}

	// Execute the rebalance plan
	for _, decision := range plan {
		// Update placement
		if err := s.storage.SavePlacement(decision); err != nil {
			return fmt.Errorf("failed to save placement for tenant %s: %w", decision.TenantID, err)
		}

		// Update tenant assignment
		tenant, err := s.storage.GetTenant(decision.TenantID)
		if err != nil {
			return fmt.Errorf("failed to get tenant %s: %w", decision.TenantID, err)
		}

		tenant.AssignedNodeID = decision.NodeID
		tenant.AssignedAt = time.Now()
		tenant.Status = enterprise.TenantStatusAssigning // Mark as re-assigning

		if err := s.storage.UpdateTenant(tenant); err != nil {
			return fmt.Errorf("failed to update tenant %s: %w", decision.TenantID, err)
		}
	}

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

// GenerateRebalancePlan creates a rebalancing plan by moving tenants from
// overloaded nodes to underloaded nodes until load is balanced
func (s *LeastLoadedStrategy) GenerateRebalancePlan(nodes []*enterprise.NodeInfo, nodeTenants map[string][]*enterprise.Tenant) ([]*enterprise.PlacementDecision, error) {
	if len(nodes) < 2 {
		return nil, nil // Nothing to rebalance with fewer than 2 nodes
	}

	// Filter healthy nodes
	healthyNodes := make([]*enterprise.NodeInfo, 0)
	for _, node := range nodes {
		if enterprise.IsNodeHealthy(node, 30*time.Second) {
			healthyNodes = append(healthyNodes, node)
		}
	}

	if len(healthyNodes) < 2 {
		return nil, nil
	}

	// Calculate average load target
	totalTenants := 0
	for _, tenants := range nodeTenants {
		totalTenants += len(tenants)
	}
	avgTenants := float64(totalTenants) / float64(len(healthyNodes))

	// Find overloaded and underloaded nodes
	type nodeLoad struct {
		node    *enterprise.NodeInfo
		tenants []*enterprise.Tenant
		load    float64
	}

	overloaded := make([]nodeLoad, 0)
	underloaded := make([]nodeLoad, 0)

	for _, node := range healthyNodes {
		tenants := nodeTenants[node.ID]
		load := float64(len(tenants))

		nl := nodeLoad{node: node, tenants: tenants, load: load}

		// Consider overloaded if more than 20% above average
		if load > avgTenants*1.2 {
			overloaded = append(overloaded, nl)
		} else if load < avgTenants*0.8 && node.ActiveTenants < node.Capacity {
			// Consider underloaded if more than 20% below average and has capacity
			underloaded = append(underloaded, nl)
		}
	}

	if len(overloaded) == 0 || len(underloaded) == 0 {
		return nil, nil
	}

	// Generate migration decisions
	decisions := make([]*enterprise.PlacementDecision, 0)

	for i := 0; i < len(overloaded); i++ {
		srcNode := &overloaded[i]
		tenantsToMove := int(srcNode.load - avgTenants)

		if tenantsToMove <= 0 {
			continue
		}

		// Move tenants to underloaded nodes
		for j := 0; j < len(underloaded) && tenantsToMove > 0 && len(srcNode.tenants) > 0; j++ {
			dstNode := &underloaded[j]

			// Calculate how many tenants this node can accept
			capacity := int(avgTenants - dstNode.load)
			if capacity <= 0 {
				continue
			}
			if capacity > tenantsToMove {
				capacity = tenantsToMove
			}

			// Move tenants
			for k := 0; k < capacity && len(srcNode.tenants) > 0; k++ {
				tenant := srcNode.tenants[0]
				srcNode.tenants = srcNode.tenants[1:]
				srcNode.load--

				decision := &enterprise.PlacementDecision{
					TenantID:    tenant.ID,
					NodeID:      dstNode.node.ID,
					NodeAddress: dstNode.node.Address,
					Reason:      fmt.Sprintf("Rebalancing from node %s (load: %.0f%%) to node %s (load: %.0f%%)",
						srcNode.node.ID, enterprise.CalculateNodeLoad(srcNode.node),
						dstNode.node.ID, enterprise.CalculateNodeLoad(dstNode.node)),
					DecidedAt: time.Now(),
				}
				decisions = append(decisions, decision)

				dstNode.load++
				tenantsToMove--
			}
		}
	}

	return decisions, nil
}
