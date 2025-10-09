package control_plane

import (
	"github.com/pocketbase/pocketbase/core/enterprise/control_plane/placement"
)

// PlacementService wraps the placement service
type PlacementService struct {
	*placement.Service
}

// NewPlacementService creates a new placement service
func NewPlacementService(storage *BadgerStorage, raftNode *RaftNode) *PlacementService {
	// Use default least-loaded strategy
	strategy := &placement.LeastLoadedStrategy{}

	service := placement.NewService(storage, strategy)

	return &PlacementService{
		Service: service,
	}
}
