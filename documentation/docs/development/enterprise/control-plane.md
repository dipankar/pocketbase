# Control Plane: The Distributed Brain

## Overview

The control plane is the **centralized coordination layer** that manages:

- Tenant registry and metadata
- Node pool and health monitoring
- Tenant placement decisions
- Routing table for gateways
- Distributed state via Raft consensus

## Technology Stack

### BadgerDB (Embedded KV Store)

- **Fast**: LSM tree design, optimized for SSDs
- **Embedded**: No separate process, runs in-process
- **Transactional**: ACID guarantees
- **Efficient**: Low memory footprint

### Hashicorp Raft (Consensus Protocol)

- **Leader election**: Automatic failover
- **Log replication**: Strong consistency
- **Snapshots**: Compact state transfer
- **Membership changes**: Dynamic cluster resizing

### Mangos v3 (IPC Layer)

- **REQ/REP**: Synchronous queries
- **PUB/SUB**: Broadcast updates
- **SURVEYOR/RESPONDENT**: Distributed queries

---

## Architecture

```go
type ControlPlane struct {
    config   *Config
    nodeID   string  // cp-1, cp-2, cp-3

    // Storage
    badger   *badger.DB
    raft     *raft.Raft

    // IPC
    reqSocket    mangos.Socket  // REP for incoming requests
    pubSocket    mangos.Socket  // PUB for broadcasts
    surveySocket mangos.Socket  // SURVEYOR for distributed queries

    // Services
    placement    *PlacementService
    health       *HealthMonitor
    router       *RouterService

    // State
    isLeader     atomic.Bool
    leaderAddr   atomic.Value

    // Shutdown
    shutdown     chan struct{}
    wg           sync.WaitGroup
}

type Config struct {
    // Identity
    NodeID       string   // cp-1, cp-2, cp-3
    DataDir      string   // /data/control-plane

    // Raft
    RaftBindAddr string   // 0.0.0.0:7000 (internal Raft)
    RaftPeers    []string // cp-1:7000, cp-2:7000, cp-3:7000

    // API
    HTTPBindAddr string   // 0.0.0.0:8090 (admin API)
    IPCBindAddr  string   // tcp://0.0.0.0:5555 (Mangos REP)
    PubBindAddr  string   // tcp://0.0.0.0:5556 (Mangos PUB)

    // Timeouts
    HeartbeatTimeout time.Duration  // 1s
    ElectionTimeout  time.Duration  // 1s
    NodeTimeout      time.Duration  // 15s (heartbeat timeout)
}
```

---

## BadgerDB Schema

### Key Prefixes

```
/tenants/{tenant_id}        -> TenantRecord
/nodes/{node_id}            -> NodeRecord
/routes/{tenant_id}         -> RouteRecord
/quotas/{tenant_id}         -> QuotaRecord
/config/placement           -> PlacementConfig
/metrics/{tenant_id}/{ts}   -> MetricsRecord
```

### Data Structures

```go
type TenantRecord struct {
    ID          string                 `json:"id"`           // tenant_abc123
    Domain      string                 `json:"domain"`       // tenant123.platform.com
    Status      TenantStatus           `json:"status"`       // created, active, idle, archived
    NodeID      string                 `json:"node_id"`      // node_47 (current placement)
    Created     time.Time              `json:"created"`
    Updated     time.Time              `json:"updated"`
    Owner       string                 `json:"owner"`        // user/org who owns tenant
    Plan        string                 `json:"plan"`         // free, pro, enterprise
    Metadata    map[string]interface{} `json:"metadata"`
}

type TenantStatus string

const (
    TenantStatusCreated   TenantStatus = "created"   // Metadata exists
    TenantStatusAssigning TenantStatus = "assigning" // Selecting node
    TenantStatusDeploying TenantStatus = "deploying" // Node loading tenant
    TenantStatusActive    TenantStatus = "active"    // Serving traffic
    TenantStatusIdle      TenantStatus = "idle"      // No recent activity
    TenantStatusMigrating TenantStatus = "migrating" // Moving to new node
    TenantStatusArchived  TenantStatus = "archived"  // S3 only
    TenantStatusDeleted   TenantStatus = "deleted"   // Soft delete
)

type NodeRecord struct {
    ID              string       `json:"id"`               // node_47
    Address         string       `json:"address"`          // http://10.0.1.47:8090
    IPCAddress      string       `json:"ipc_address"`      // tcp://10.0.1.47:5557
    Status          NodeStatus   `json:"status"`
    Capacity        int          `json:"capacity"`         // 200
    ActiveTenants   int          `json:"active_tenants"`   // 143
    CachedTenants   int          `json:"cached_tenants"`   // 187
    Region          string       `json:"region"`           // us-east-1a
    Zone            string       `json:"zone"`             // zone-a
    Resources       NodeResources `json:"resources"`
    LastHeartbeat   time.Time    `json:"last_heartbeat"`
    Registered      time.Time    `json:"registered"`
}

type NodeStatus string

const (
    NodeStatusHealthy  NodeStatus = "healthy"
    NodeStatusDegraded NodeStatus = "degraded"
    NodeStatusDown     NodeStatus = "down"
    NodeStatusDraining NodeStatus = "draining"  // Gracefully removing tenants
)
```

---

## Raft Integration

### Raft FSM (Finite State Machine)

```go
type RaftFSM struct {
    badger *badger.DB
    mu     sync.RWMutex
}

// Apply applies a Raft log entry
func (f *RaftFSM) Apply(log *raft.Log) interface{} {
    var cmd Command
    if err := json.Unmarshal(log.Data, &cmd); err != nil {
        return err
    }

    switch cmd.Type {
    case "tenant.create":
        return f.applyCreateTenant(cmd.Payload)
    case "tenant.update":
        return f.applyUpdateTenant(cmd.Payload)
    case "tenant.delete":
        return f.applyDeleteTenant(cmd.Payload)
    case "node.register":
        return f.applyRegisterNode(cmd.Payload)
    case "node.heartbeat":
        return f.applyHeartbeat(cmd.Payload)
    case "route.assign":
        return f.applyAssignRoute(cmd.Payload)
    default:
        return fmt.Errorf("unknown command: %s", cmd.Type)
    }
}

type Command struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}
```

### Snapshot Store (S3-backed)

```go
type S3SnapshotStore struct {
    s3Client *s3.Client
    bucket   string
    prefix   string  // control-plane/snapshots/
}

func (s *S3SnapshotStore) Create(
    version raft.SnapshotVersion,
    index uint64,
    term uint64,
    configuration raft.Configuration,
    configurationIndex uint64,
    trans raft.Transport,
) (raft.SnapshotSink, error) {
    // Create S3 multipart upload
    key := fmt.Sprintf("%s/snapshot-%d-%d.snap", s.prefix, term, index)

    sink := &S3SnapshotSink{
        s3:     s.s3Client,
        bucket: s.bucket,
        key:    key,
        meta: raft.SnapshotMeta{
            Version:            version,
            Index:              index,
            Term:               term,
            Configuration:      configuration,
            ConfigurationIndex: configurationIndex,
        },
    }

    return sink, nil
}
```

---

## Placement Service

The placement service decides which node should host a tenant.

```go
type PlacementService struct {
    badger *badger.DB
    raft   *raft.Raft
    config *PlacementConfig
}

type PlacementConfig struct {
    Strategy         string  // "least-loaded", "random", "zone-aware"
    MaxTenantPerNode int     // 200
    MinFreeCapacity  int     // 20 (keep 20 slots free)

    // Zone awareness
    PreferSameZone   bool

    // Affinity rules
    AntiAffinity     []AffinityRule
}

// AssignTenant assigns a tenant to a node
func (ps *PlacementService) AssignTenant(tenantID string) (string, error) {
    switch ps.config.Strategy {
    case "least-loaded":
        return ps.leastLoadedPlacement(tenantID)
    case "zone-aware":
        return ps.zoneAwarePlacement(tenantID)
    default:
        return ps.leastLoadedPlacement(tenantID)
    }
}

func (ps *PlacementService) leastLoadedPlacement(tenantID string) (string, error) {
    // Get all healthy nodes
    nodes, err := ps.getHealthyNodes()
    if err != nil {
        return "", err
    }

    if len(nodes) == 0 {
        return "", errors.New("no healthy nodes available")
    }

    // Find node with lowest load
    var selectedNode *NodeRecord
    var minLoad float64 = 1.0

    for _, node := range nodes {
        // Check capacity
        if node.ActiveTenants >= node.Capacity - ps.config.MinFreeCapacity {
            continue  // Skip full nodes
        }

        // Calculate load score
        load := float64(node.ActiveTenants) / float64(node.Capacity)

        // Consider CPU and memory
        load += node.Resources.CPUUsage * 0.3
        load += (float64(node.Resources.MemoryUsage) / float64(node.Resources.MemoryTotal)) * 0.3

        if load < minLoad {
            minLoad = load
            selectedNode = &node
        }
    }

    if selectedNode == nil {
        return "", errors.New("no available capacity")
    }

    // Create route via Raft
    route := RouteRecord{
        TenantID: tenantID,
        NodeID:   selectedNode.ID,
        Priority: 1,
        Created:  time.Now(),
    }

    if err := ps.applyRouteAssignment(route); err != nil {
        return "", err
    }

    return selectedNode.ID, nil
}
```

---

## Health Monitor

Monitors node health via heartbeats.

```go
type HealthMonitor struct {
    badger      *badger.DB
    raft        *raft.Raft
    nodeTimeout time.Duration  // 15s

    // Callbacks
    onNodeDown  func(nodeID string)
    onNodeUp    func(nodeID string)
}

// Start monitoring
func (hm *HealthMonitor) Start() {
    go hm.checkNodesHealth()
}

func (hm *HealthMonitor) checkNodesHealth() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        hm.scanNodes()
    }
}

func (hm *HealthMonitor) scanNodes() {
    now := time.Now()

    hm.badger.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        opts.Prefix = []byte("/nodes/")

        it := txn.NewIterator(opts)
        defer it.Close()

        for it.Rewind(); it.Valid(); it.Next() {
            item := it.Item()
            item.Value(func(val []byte) error {
                var node NodeRecord
                if err := json.Unmarshal(val, &node); err != nil {
                    return err
                }

                // Check if heartbeat timed out
                if now.Sub(node.LastHeartbeat) > hm.nodeTimeout {
                    if node.Status != NodeStatusDown {
                        hm.markNodeDown(node.ID)
                    }
                } else {
                    if node.Status == NodeStatusDown {
                        hm.markNodeUp(node.ID)
                    }
                }

                return nil
            })
        }

        return nil
    })
}
```

---

## Router Service

Handles routing queries from gateways.

```go
type RouterService struct {
    badger    *badger.DB
    placement *PlacementService
}

// GetTenantRoute returns the node address for a tenant
func (rs *RouterService) GetTenantRoute(tenantID string) (string, error) {
    // Check if route exists
    key := "/routes/" + tenantID

    var route RouteRecord
    err := rs.badger.View(func(txn *badger.Txn) error {
        item, err := txn.Get([]byte(key))
        if err != nil {
            return err
        }

        return item.Value(func(val []byte) error {
            return json.Unmarshal(val, &route)
        })
    })

    if err == badger.ErrKeyNotFound {
        // No route exists, assign tenant to node
        nodeID, err := rs.placement.AssignTenant(tenantID)
        if err != nil {
            return "", err
        }

        // Get node address
        return rs.getNodeAddress(nodeID)
    }

    if err != nil {
        return "", err
    }

    // Get node address from route
    return rs.getNodeAddress(route.NodeID)
}

func (rs *RouterService) getNodeAddress(nodeID string) (string, error) {
    key := "/nodes/" + nodeID

    var node NodeRecord
    err := rs.badger.View(func(txn *badger.Txn) error {
        item, err := txn.Get([]byte(key))
        if err != nil {
            return err
        }

        return item.Value(func(val []byte) error {
            return json.Unmarshal(val, &node)
        })
    })

    if err != nil {
        return "", err
    }

    if node.Status != NodeStatusHealthy {
        return "", fmt.Errorf("node %s is not healthy", nodeID)
    }

    return node.Address, nil
}
```

---

## Mangos IPC Handlers

```go
type IPCHandler struct {
    router    *RouterService
    placement *PlacementService
    badger    *badger.DB
}

// Start IPC server
func (ipc *IPCHandler) Start(bindAddr string) error {
    socket, err := rep.NewSocket()
    if err != nil {
        return err
    }

    if err := socket.Listen(bindAddr); err != nil {
        return err
    }

    go ipc.handleRequests(socket)

    return nil
}

func (ipc *IPCHandler) handleRequests(socket mangos.Socket) {
    for {
        msg, err := socket.Recv()
        if err != nil {
            continue
        }

        // Parse request
        var req Request
        if err := json.Unmarshal(msg, &req); err != nil {
            ipc.sendError(socket, err)
            continue
        }

        // Handle request
        resp := ipc.handleRequest(req)

        // Send response
        data, _ := json.Marshal(resp)
        socket.Send(data)
    }
}

func (ipc *IPCHandler) handleRequest(req Request) Response {
    switch req.Type {
    case "route.get":
        var query RouteQuery
        json.Unmarshal(req.Payload, &query)

        nodeAddr, err := ipc.router.GetTenantRoute(query.TenantID)
        if err != nil {
            return Response{Error: err.Error()}
        }

        return Response{
            Data: map[string]interface{}{
                "tenant_id": query.TenantID,
                "node_addr": nodeAddr,
            },
        }

    case "node.register":
        var node NodeRecord
        json.Unmarshal(req.Payload, &node)

        // Register node (via Raft)
        return ipc.registerNode(node)

    case "node.heartbeat":
        var hb Heartbeat
        json.Unmarshal(req.Payload, &hb)

        // Update heartbeat (via Raft)
        return ipc.updateHeartbeat(hb)

    default:
        return Response{Error: "unknown request type"}
    }
}
```

## Next Steps

- [Storage Strategy](storage.md) - S3 + Litestream details
- [Cluster Users](cluster-users.md) - Self-service customers
