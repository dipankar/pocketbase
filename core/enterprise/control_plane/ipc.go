package control_plane

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/pocketbase/pocketbase/core/enterprise"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	_ "go.nanomsg.org/mangos/v3/transport/all" // Import all transports
)

// IPCServer handles IPC requests from gateways and tenant nodes
type IPCServer struct {
	cp     *ControlPlane
	socket mangos.Socket
	logger *log.Logger

	ctx    context.Context
	cancel context.CancelFunc
}

// NewIPCServer creates a new IPC server
func NewIPCServer(cp *ControlPlane) (*IPCServer, error) {
	socket, err := rep.NewSocket()
	if err != nil {
		return nil, fmt.Errorf("failed to create REP socket: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &IPCServer{
		cp:     cp,
		socket: socket,
		logger: log.Default(),
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start starts the IPC server
func (s *IPCServer) Start() error {
	// Listen on TCP
	url := fmt.Sprintf("tcp://0.0.0.0:8090")
	if err := s.socket.Listen(url); err != nil {
		return fmt.Errorf("failed to listen on %s: %w", url, err)
	}

	s.logger.Printf("[IPCServer] Listening on %s", url)

	// Start request handler
	go s.handleRequests()

	return nil
}

// Stop stops the IPC server
func (s *IPCServer) Stop() error {
	s.cancel()
	return s.socket.Close()
}

// handleRequests processes incoming IPC requests
func (s *IPCServer) handleRequests() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			msg, err := s.socket.Recv()
			if err != nil {
				if s.ctx.Err() != nil {
					return
				}
				s.logger.Printf("[IPCServer] Error receiving message: %v", err)
				continue
			}

			// Handle request in goroutine
			go s.handleRequest(msg)
		}
	}
}

// IPCRequest represents an IPC request
type IPCRequest struct {
	Type string                 `json:"type"` // getTenant, assignTenant, registerNode, heartbeat
	Data map[string]interface{} `json:"data"`
}

// IPCResponse represents an IPC response
type IPCResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// handleRequest processes a single IPC request
func (s *IPCServer) handleRequest(msg []byte) {
	var req IPCRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		s.sendError(fmt.Sprintf("invalid request: %v", err))
		return
	}

	var resp IPCResponse

	switch req.Type {
	case "getTenant":
		resp = s.handleGetTenant(req.Data)
	case "getTenantByDomain":
		resp = s.handleGetTenantByDomain(req.Data)
	case "assignTenant":
		resp = s.handleAssignTenant(req.Data)
	case "registerNode":
		resp = s.handleRegisterNode(req.Data)
	case "heartbeat":
		resp = s.handleHeartbeat(req.Data)
	default:
		resp = IPCResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown request type: %s", req.Type),
		}
	}

	// Send response
	respJSON, _ := json.Marshal(resp)
	if err := s.socket.Send(respJSON); err != nil {
		s.logger.Printf("[IPCServer] Error sending response: %v", err)
	}
}

func (s *IPCServer) handleGetTenant(data map[string]interface{}) IPCResponse {
	tenantID, ok := data["tenantId"].(string)
	if !ok {
		return IPCResponse{Success: false, Error: "tenantId required"}
	}

	tenant, err := s.cp.GetTenant(tenantID)
	if err != nil {
		return IPCResponse{Success: false, Error: err.Error()}
	}

	return IPCResponse{
		Success: true,
		Data: map[string]interface{}{
			"tenant": tenant,
		},
	}
}

func (s *IPCServer) handleGetTenantByDomain(data map[string]interface{}) IPCResponse {
	domain, ok := data["domain"].(string)
	if !ok {
		return IPCResponse{Success: false, Error: "domain required"}
	}

	tenant, err := s.cp.GetTenantByDomain(domain)
	if err != nil {
		return IPCResponse{Success: false, Error: err.Error()}
	}

	return IPCResponse{
		Success: true,
		Data: map[string]interface{}{
			"tenant": tenant,
		},
	}
}

func (s *IPCServer) handleAssignTenant(data map[string]interface{}) IPCResponse {
	tenantID, ok := data["tenantId"].(string)
	if !ok {
		return IPCResponse{Success: false, Error: "tenantId required"}
	}

	decision, err := s.cp.AssignTenant(tenantID)
	if err != nil {
		return IPCResponse{Success: false, Error: err.Error()}
	}

	return IPCResponse{
		Success: true,
		Data: map[string]interface{}{
			"decision": decision,
		},
	}
}

func (s *IPCServer) handleRegisterNode(data map[string]interface{}) IPCResponse {
	// Parse node info
	nodeID, _ := data["nodeId"].(string)
	address, _ := data["address"].(string)
	capacity, _ := data["capacity"].(float64)

	if nodeID == "" || address == "" {
		return IPCResponse{Success: false, Error: "nodeId and address required"}
	}

	node := &enterprise.NodeInfo{
		ID:       nodeID,
		Address:  address,
		Capacity: int(capacity),
		Status:   "online",
	}

	if err := s.cp.RegisterNode(node); err != nil {
		return IPCResponse{Success: false, Error: err.Error()}
	}

	return IPCResponse{Success: true}
}

func (s *IPCServer) handleHeartbeat(data map[string]interface{}) IPCResponse {
	nodeID, _ := data["nodeId"].(string)
	activeTenantsCount, _ := data["activeTenantsCount"].(float64)

	if nodeID == "" {
		return IPCResponse{Success: false, Error: "nodeId required"}
	}

	if err := s.cp.UpdateNodeHeartbeat(nodeID, int(activeTenantsCount)); err != nil {
		return IPCResponse{Success: false, Error: err.Error()}
	}

	return IPCResponse{Success: true}
}

func (s *IPCServer) sendError(errMsg string) {
	resp := IPCResponse{
		Success: false,
		Error:   errMsg,
	}
	respJSON, _ := json.Marshal(resp)
	s.socket.Send(respJSON)
}
