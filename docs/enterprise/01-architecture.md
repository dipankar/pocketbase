# Architecture Deep Dive

## System Components

### 1. Control Plane (Distributed Brain)

**Purpose**: Centralized coordination and tenant routing

**Technology Stack**:
- **BadgerDB**: Embedded KV store for metadata
- **Hashicorp Raft**: Consensus and replication
- **Mangos v3**: IPC communication layer

**Data Storage**:
```go
// BadgerDB Schema
type TenantRecord struct {
    ID          string    // tenant_abc123
    Domain      string    // tenant123.platform.com
    Status      string    // created, active, idle, archived
    NodeID      string    // node_47 (current placement)
    Created     time.Time
    Updated     time.Time
    Quotas      TenantQuotas
    Metadata    map[string]interface{}
}

type NodeRecord struct {
    ID           string    // node_47
    Address      string    // 10.0.1.47:8090
    Capacity     int       // 200 (max tenants)
    ActiveCount  int       // 143 (current active)
    Status       string    // healthy, degraded, down
    LastHeartbeat time.Time
    Region       string    // us-east-1a
    Resources    NodeResources
}

type RouteRecord struct {
    TenantID     string
    NodeID       string
    Priority     int       // for multi-node scenarios
    LastAccess   time.Time
}

type TenantQuotas struct {
    MaxStorage      int64  // bytes
    MaxAPIRequests  int    // per hour
    MaxConnections  int
    MaxCPU          int    // millicores
    MaxMemory       int64  // bytes
}

type NodeResources struct {
    CPUUsage      float64  // percentage
    MemoryUsage   int64    // bytes
    DiskUsage     int64    // bytes
    NetworkIn     int64    // bytes/sec
    NetworkOut    int64    // bytes/sec
}
```

**Responsibilities**:
1. Maintain tenant registry
2. Track node health and capacity
3. Make tenant placement decisions
4. Handle tenant migration requests
5. Provide routing table to gateways
6. Coordinate distributed operations

**Raft Configuration**:
```go
// core/control_plane/raft_config.go

type RaftConfig struct {
    // Cluster configuration
    NodeID       string   // cp-1, cp-2, cp-3
    BindAddr     string   // 0.0.0.0:7000 (Raft internal)
    Peers        []string // cp-1:7000, cp-2:7000, cp-3:7000

    // Data directories
    DataDir      string   // /data/raft
    SnapshotDir  string   // /data/snapshots

    // Timing
    HeartbeatTimeout time.Duration  // 1s
    ElectionTimeout  time.Duration  // 1s
    CommitTimeout    time.Duration  // 50ms

    // Snapshots
    SnapshotInterval   time.Duration  // 2 minutes
    SnapshotThreshold  uint64         // 8192 logs
    TrailingLogs       uint64         // 10240
}
```

**API Endpoints** (Control Plane):
```
Internal (Mangos IPC):
- /internal/tenant/assign     # Assign tenant to node
- /internal/tenant/route      # Get tenant's current node
- /internal/node/register     # Node announces itself
- /internal/node/heartbeat    # Node health check
- /internal/node/capacity     # Node reports capacity

External (HTTP for admin):
- POST   /api/cp/tenants             # Create tenant
- GET    /api/cp/tenants             # List tenants
- GET    /api/cp/tenants/{id}        # Get tenant
- PUT    /api/cp/tenants/{id}        # Update tenant
- DELETE /api/cp/tenants/{id}        # Delete tenant
- POST   /api/cp/tenants/{id}/migrate # Migrate tenant to another node
- GET    /api/cp/nodes               # List nodes
- GET    /api/cp/nodes/{id}          # Get node details
- GET    /api/cp/health              # Cluster health
```

---

### 2. Tenant Node (Stateless Worker)

**Purpose**: Execute tenant workloads

**Technology Stack**:
- **PocketBase Core**: Multi-instance tenant containers
- **Litestream**: Continuous S3 replication
- **Mangos v3**: Control plane communication

**Architecture**:
```go
// core/tenant_node/node.go

type TenantNode struct {
    nodeID       string
    controlPlane *ControlPlaneClient

    // Tenant cache (LRU)
    cache        *TenantCache
    maxTenants   int  // 200

    // Litestream manager
    litestream   *LitestreamManager

    // Resource monitor
    monitor      *ResourceMonitor

    // Mangos sockets
    reqSocket    mangos.Socket  // REQ/REP to control plane
    subSocket    mangos.Socket  // SUB for broadcasts
}

type TenantCache struct {
    mu       sync.RWMutex
    tenants  map[string]*TenantInstance
    lru      *lru.Cache

    // Metrics
    hits     uint64
    misses   uint64
    evictions uint64
}

type TenantInstance struct {
    ID         string
    App        *core.BaseApp  // Separate PocketBase instance
    DataDir    string         // /data/tenants/tenant_abc123

    // State
    Status     string         // loading, active, idle
    LoadedAt   time.Time
    LastAccess time.Time
    RequestCount uint64

    // Resources
    DBConnections []*dbx.DB
    HooksVM       *goja.Runtime

    // Litestream
    Replications  []*exec.Cmd  // litestream processes
}
```

**Tenant Loading Process**:
```
1. Request arrives for tenant_abc123
2. Check cache:
   - Cache hit: Return immediately
   - Cache miss: Load tenant
3. Loading:
   a. Check local disk: /data/tenants/tenant_abc123/
   b. If not exists:
      - Download from S3: s3://bucket/tenants/tenant_abc123/litestream/
      - Restore using Litestream: litestream restore
   c. Bootstrap PocketBase instance:
      - Open data.db and auxiliary.db
      - Load hooks from pb_hooks/
      - Initialize tenant-specific settings
   d. Start Litestream replication:
      - litestream replicate data.db → s3://bucket/tenants/tenant_abc123/litestream/data.db
      - litestream replicate auxiliary.db → s3://...
4. Add to cache (evict LRU if at capacity)
5. Return tenant instance
```

**Cache Eviction Strategy**:
```go
// LRU eviction when cache is full

type EvictionPolicy struct {
    MaxTenants    int           // 200
    IdleTimeout   time.Duration // 30 minutes

    // Weights for eviction scoring
    WeightAccess  float64  // 0.5 (recent access)
    WeightRequests float64 // 0.3 (request count)
    WeightSize    float64  // 0.2 (memory footprint)
}

func (tc *TenantCache) Evict() error {
    // Find tenant with lowest score
    var evictID string
    var lowestScore float64 = math.MaxFloat64

    for id, inst := range tc.tenants {
        score := tc.calculateScore(inst)
        if score < lowestScore {
            lowestScore = score
            evictID = id
        }
    }

    // Graceful eviction
    return tc.EvictTenant(evictID)
}

func (tc *TenantCache) EvictTenant(tenantID string) error {
    inst := tc.tenants[tenantID]

    // 1. Stop accepting new requests
    inst.Status = "evicting"

    // 2. Wait for in-flight requests to complete (with timeout)
    tc.drainRequests(inst, 30*time.Second)

    // 3. Stop Litestream replication (ensures all data synced to S3)
    for _, proc := range inst.Replications {
        proc.Process.Signal(syscall.SIGTERM)
        proc.Wait()
    }

    // 4. Close database connections
    for _, db := range inst.DBConnections {
        db.Close()
    }

    // 5. Shutdown PocketBase instance
    inst.App.ResetBootstrapState()

    // 6. Delete local files (optional, can keep for faster reload)
    // os.RemoveAll(inst.DataDir)

    // 7. Remove from cache
    delete(tc.tenants, tenantID)
    tc.lru.Remove(tenantID)

    tc.evictions++
    return nil
}
```

**Node Heartbeat** (to Control Plane):
```go
type Heartbeat struct {
    NodeID      string
    Timestamp   time.Time
    Status      string  // healthy, degraded, down

    // Capacity
    MaxTenants    int
    ActiveTenants int
    CachedTenants int

    // Resources
    CPUUsage     float64  // 0.0 - 1.0
    MemoryUsage  int64    // bytes
    DiskUsage    int64

    // Tenants currently hosted
    Tenants      []string
}

// Send every 5 seconds
func (tn *TenantNode) sendHeartbeat() {
    ticker := time.NewTicker(5 * time.Second)
    for range ticker.C {
        hb := tn.buildHeartbeat()
        tn.controlPlane.SendHeartbeat(hb)
    }
}
```

---

### 3. Gateway (Reverse Proxy)

**Purpose**: Route incoming requests to appropriate tenant nodes

**Technology Stack**:
- **HTTP Reverse Proxy**: Custom or Caddy/Traefik
- **Mangos v3**: Control plane queries
- **Local Cache**: Routing table cache

**Request Flow**:
```
1. DNS: tenant123.platform.com → Gateway IP
2. Extract tenant ID from domain
3. Check local routing cache:
   - Hit: Use cached node address
   - Miss: Query control plane via Mangos
4. Proxy request to tenant node
5. Update local cache with route
6. Return response to client
```

**Architecture**:
```go
// core/gateway/gateway.go

type Gateway struct {
    controlPlane *ControlPlaneClient

    // Routing cache (local)
    routeCache   *RouteCache

    // HTTP reverse proxy
    proxy        *httputil.ReverseProxy

    // Circuit breaker (per node)
    breakers     map[string]*CircuitBreaker

    // Metrics
    metrics      *GatewayMetrics
}

type RouteCache struct {
    mu      sync.RWMutex
    routes  map[string]*Route  // tenantID -> Route
    ttl     time.Duration      // 5 minutes
}

type Route struct {
    TenantID   string
    NodeAddr   string  // http://10.0.1.47:8090
    CachedAt   time.Time
    Hits       uint64
}

type CircuitBreaker struct {
    state         string  // closed, open, half-open
    failures      int
    threshold     int     // 5 consecutive failures
    timeout       time.Duration
    lastFailure   time.Time
}
```

**Tenant Extraction**:
```go
// Extract tenant ID from request

func (g *Gateway) extractTenantID(r *http.Request) (string, error) {
    // Method 1: Subdomain
    // tenant123.platform.com → tenant123
    host := r.Host
    parts := strings.Split(host, ".")
    if len(parts) >= 3 {
        return parts[0], nil
    }

    // Method 2: X-Tenant-ID header
    if tenantID := r.Header.Get("X-Tenant-ID"); tenantID != "" {
        return tenantID, nil
    }

    // Method 3: API Key (extract tenant from JWT)
    if auth := r.Header.Get("Authorization"); auth != "" {
        token := strings.TrimPrefix(auth, "Bearer ")
        tenantID, err := extractTenantFromToken(token)
        if err == nil {
            return tenantID, nil
        }
    }

    return "", errors.New("unable to extract tenant ID")
}
```

**Proxy Logic**:
```go
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Extract tenant ID
    tenantID, err := g.extractTenantID(r)
    if err != nil {
        http.Error(w, "Invalid tenant", 400)
        return
    }

    // Get route (cached or from control plane)
    route, err := g.getRoute(tenantID)
    if err != nil {
        http.Error(w, "Tenant not found", 404)
        return
    }

    // Check circuit breaker
    if breaker := g.breakers[route.NodeAddr]; breaker != nil {
        if breaker.state == "open" {
            http.Error(w, "Service unavailable", 503)
            return
        }
    }

    // Proxy to tenant node
    target, _ := url.Parse(route.NodeAddr)
    proxy := httputil.NewSingleHostReverseProxy(target)

    // Add tenant context to request
    r.Header.Set("X-PB-Tenant-ID", tenantID)

    // Forward request
    proxy.ServeHTTP(w, r)

    // Update metrics
    route.Hits++
}

func (g *Gateway) getRoute(tenantID string) (*Route, error) {
    // Check cache
    g.routeCache.mu.RLock()
    if route, ok := g.routeCache.routes[tenantID]; ok {
        if time.Since(route.CachedAt) < g.routeCache.ttl {
            g.routeCache.mu.RUnlock()
            return route, nil
        }
    }
    g.routeCache.mu.RUnlock()

    // Query control plane via Mangos
    nodeAddr, err := g.controlPlane.GetTenantNode(tenantID)
    if err != nil {
        return nil, err
    }

    // Update cache
    route := &Route{
        TenantID: tenantID,
        NodeAddr: nodeAddr,
        CachedAt: time.Now(),
    }

    g.routeCache.mu.Lock()
    g.routeCache.routes[tenantID] = route
    g.routeCache.mu.Unlock()

    return route, nil
}
```

---

## Inter-Component Communication

### Mangos v3 Patterns

**1. REQ/REP (Request/Reply)**
Used for synchronous queries:
```go
// Gateway → Control Plane: Get tenant route
// Node → Control Plane: Register node

socket, _ := rep.NewSocket()
socket.Listen("tcp://0.0.0.0:5555")

// Server
for {
    msg, _ := socket.Recv()
    // Process request
    reply := processRequest(msg)
    socket.Send(reply)
}
```

**2. PUB/SUB (Publish/Subscribe)**
Used for broadcasts:
```go
// Control Plane → All Nodes: Tenant migration notice
// Control Plane → All Gateways: Routing table update

socket, _ := sub.NewSocket()
socket.Dial("tcp://control-plane:5556")
socket.SetOption(mangos.OptionSubscribe, []byte("tenant."))

// Subscriber
for {
    msg, _ := socket.Recv()
    handleBroadcast(msg)
}
```

**3. SURVEYOR/RESPONDENT**
Used for distributed queries:
```go
// Control Plane → All Nodes: Which node has tenant X?

socket, _ := surveyor.NewSocket()
socket.SetOption(mangos.OptionSurveyTime, time.Second)
socket.Broadcast([]byte("find:tenant_abc123"))

responses := socket.RecvAll()
```

### Message Protocol

```go
type Message struct {
    Type      string    // "route", "heartbeat", "migrate", etc.
    RequestID string    // for correlation
    Timestamp time.Time
    Payload   []byte    // JSON-encoded data
}

// Example: Route query
type RouteQuery struct {
    TenantID string
}

type RouteResponse struct {
    TenantID  string
    NodeAddr  string
    Status    string  // active, migrating, archived
}
```

---

## Data Flow Examples

### Example 1: First Request to New Tenant

```
1. Client: POST tenant123.platform.com/api/collections/users
2. Gateway extracts tenant ID: "tenant123"
3. Gateway checks cache: MISS
4. Gateway → Control Plane (Mangos REQ):
   {type: "route", tenantID: "tenant123"}
5. Control Plane checks BadgerDB:
   - Tenant exists but no node assignment
6. Control Plane selects node (node-47, least loaded)
7. Control Plane → Gateway (Mangos REP):
   {tenantID: "tenant123", nodeAddr: "http://10.0.1.47:8090"}
8. Gateway caches route
9. Gateway proxies request to node-47
10. Node-47 receives request, checks cache: MISS
11. Node-47 loads tenant from S3:
    - Litestream restore s3://bucket/tenants/tenant123/
12. Node-47 bootstraps PocketBase instance
13. Node-47 starts Litestream replication
14. Node-47 processes request
15. Response flows back: Node → Gateway → Client
```

### Example 2: Subsequent Request (Cache Hit)

```
1. Client: GET tenant123.platform.com/api/collections/users
2. Gateway extracts tenant ID: "tenant123"
3. Gateway checks cache: HIT (node-47)
4. Gateway proxies directly to node-47
5. Node-47 checks cache: HIT
6. Node-47 processes request (fast path)
7. Response: Node → Gateway → Client
```

### Example 3: Tenant Migration

```
1. Admin: POST /api/cp/tenants/tenant123/migrate
   {targetNode: "node-89"}
2. Control Plane:
   a. Update routing: tenant123 → node-89 (priority 1)
   b. Keep old route: tenant123 → node-47 (priority 0)
3. Control Plane broadcasts (Mangos PUB):
   {type: "tenant.migrating", tenantID: "tenant123", newNode: "node-89"}
4. Gateways receive broadcast, invalidate cache
5. Next request → Gateway queries control plane → node-89
6. Node-89 loads tenant from S3 (latest state)
7. After 5 minutes of no traffic to node-47:
   - Node-47 evicts tenant123 from cache
8. Control Plane removes old route
```

---

## Fault Tolerance

### Node Failure

```
1. Node-47 crashes or loses network
2. Control Plane detects: No heartbeat for 15 seconds
3. Control Plane marks node-47 as "down"
4. Control Plane broadcasts (Mangos PUB):
   {type: "node.down", nodeID: "node-47"}
5. Gateways receive broadcast, invalidate routes to node-47
6. Next requests to tenants on node-47:
   a. Gateway queries control plane
   b. Control Plane assigns new node (node-89)
   c. Node-89 loads tenant from S3 (zero data loss!)
   d. Service restored
```

### Control Plane Failover

```
1. CP Leader (cp-1) crashes
2. Raft election triggered (within 1 second)
3. Follower (cp-2) becomes new leader
4. BadgerDB is replicated, no data loss
5. Gateways and nodes reconnect to new leader
6. Service continues with <1s disruption
```

### S3 Outage

```
1. S3 becomes unavailable
2. Litestream replication fails (buffered locally)
3. Nodes continue serving cached tenants (no impact)
4. New tenant loads fail (503 error)
5. When S3 recovers:
   - Litestream syncs buffered WAL segments
   - New tenant loads resume
```

---

## Next: Control Plane Implementation

See [02-control-plane.md](02-control-plane.md) for detailed control plane design with BadgerDB and Raft.
