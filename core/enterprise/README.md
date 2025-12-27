# PocketBase Enterprise - Core Implementation

This directory contains the core implementation of PocketBase's multi-tenant enterprise platform.

## 📁 Directory Structure

```
enterprise/
├── auth/                   # Authentication & JWT handling
│   ├── jwt.go             # JWT token generation and validation
│   └── middleware.go      # Authentication middleware
│
├── control_plane/         # Distributed control plane (Raft + BadgerDB)
│   ├── badger/           # BadgerDB storage implementation
│   │   ├── storage.go    # CRUD operations for tenants, users, nodes
│   │   └── disk_manager.go # Disk space management and GC
│   ├── placement/        # Tenant placement and load balancing
│   │   └── placement.go  # Placement algorithms and decisions
│   ├── raft/             # Raft consensus integration
│   │   ├── raft.go       # Raft node setup and peer discovery
│   │   └── fsm.go        # Finite state machine for log application
│   ├── control_plane.go  # Main control plane orchestrator
│   ├── storage.go        # Storage wrapper with Raft integration
│   └── raft_commands.go  # Raft command serialization
│
├── email/                 # Email verification system
│   ├── sender.go         # Email sending (SMTP, SendGrid, etc.)
│   └── templates.go      # Email templates
│
├── gateway/               # HTTP gateway and reverse proxy
│   ├── circuit_breaker/  # Circuit breaker for fault tolerance
│   │   └── breaker.go
│   ├── proxy/            # HTTP proxy implementation
│   │   └── proxy.go
│   └── gateway.go        # Main gateway logic and routing
│
├── health/                # Health check system
│   └── checker.go        # Health monitoring for all components
│
├── metrics/               # Metrics collection (Prometheus-ready)
│   └── collector.go      # Metrics definitions and collectors
│
├── storage/               # S3 storage and Litestream integration
│   ├── s3.go             # S3 backend operations
│   └── litestream.go     # Embedded Litestream for SQLite replication
│
├── tenant_node/          # Tenant node (stateless workers)
│   ├── cache/            # Tenant caching with LRU eviction
│   │   └── lru.go
│   ├── hooks/            # Database-backed hooks system
│   │   └── hooks.go
│   ├── manager.go        # Tenant lifecycle management
│   ├── http_server.go    # HTTP server for tenant requests
│   ├── quota_enforcer.go # Storage and API quota enforcement
│   ├── metrics_collector.go # Real-time metrics collection
│   └── archiver.go       # Tenant archiving and restoration
│
└── types.go              # Shared types and interfaces

```

## 🔑 Key Components

### Control Plane
The distributed brain of the platform:
- **BadgerDB**: Embedded key-value store for tenant metadata
- **Raft**: Consensus protocol for high availability
- **Placement Service**: Assigns tenants to optimal nodes

**Usage**:
```go
cp, err := control_plane.NewControlPlane(config, storage)
cp.Start()
```

### Tenant Node
Stateless workers that run tenant instances:
- **Manager**: Loads/unloads tenants from S3
- **Quota Enforcer**: Enforces storage and API limits
- **Metrics Collector**: Tracks resource usage in real-time
- **Archiver**: Handles tenant backup and restoration

**Usage**:
```go
manager, err := tenant_node.NewManager(config, storage, cpClient)
manager.Start()
```

### Gateway
Reverse proxy that routes requests to tenant nodes:
- **Circuit Breaker**: Prevents cascading failures
- **Proxy**: HTTP request forwarding
- **Load Balancing**: Distributes traffic across nodes

**Usage**:
```go
gw, err := gateway.NewGateway(config, cpClient)
gw.Start()
```

## 🚀 Quick Start

### 1. Control Plane Mode
```go
config := &enterprise.ClusterConfig{
    Mode:         enterprise.ModeControlPlane,
    NodeID:       "cp-1",
    RaftPeers:    []string{"cp-1:7000", "cp-2:7000", "cp-3:7000"},
    RaftBindAddr: "0.0.0.0:7000",
    DataDir:      "/data/control-plane",
}

storage, _ := control_plane.NewBadgerStorage(config.DataDir)
cp, _ := control_plane.NewControlPlane(config, storage)
cp.Start()
```

### 2. Tenant Node Mode
```go
config := &enterprise.ClusterConfig{
    Mode:              enterprise.ModeTenantNode,
    ControlPlaneAddrs: []string{"cp-1:8090", "cp-2:8090"},
    MaxTenants:        200,
    DataDir:          "/data/tenants",
}

cpClient := control_plane.NewHTTPClient(config.ControlPlaneAddrs)
manager, _ := tenant_node.NewManager(config, s3Backend, cpClient)
manager.Start()
```

### 3. Gateway Mode
```go
config := &enterprise.ClusterConfig{
    Mode:                     enterprise.ModeGateway,
    GatewayControlPlaneAddrs: []string{"cp-1:8090"},
}

cpClient := control_plane.NewHTTPClient(config.GatewayControlPlaneAddrs)
gw, _ := gateway.NewGateway(config, cpClient)
gw.Start()
```

## 📊 Implemented Features

### ✅ Phase 1: Foundation
- [x] Control plane with Raft consensus
- [x] BadgerDB for metadata storage
- [x] Tenant node lifecycle management
- [x] S3 integration with Litestream
- [x] Gateway reverse proxy
- [x] JWT authentication
- [x] Email verification system

### ✅ Phase 2: Production Hardening (Partial)
- [x] Storage quota enforcement
- [x] API rate limiting (24-hour windows)
- [x] Database growth rate tracking
- [x] Peak request monitoring
- [x] Response time tracking
- [x] Error rate tracking
- [x] Raft snapshots for backup/restore
- [x] Control plane notifications
- [ ] Tenant placement optimization
- [ ] Prometheus metrics export
- [ ] Distributed tracing

## 🔧 Development

### Building
```bash
cd /path/to/pocketbase
go build ./...
```

### Testing
```bash
# Unit tests
go test ./core/enterprise/...

# Specific component
go test ./core/enterprise/tenant_node/...
go test ./core/enterprise/control_plane/...
```

### Code Quality
All code follows these standards:
- Thread-safe with proper mutex usage
- Comprehensive error handling with context
- Structured logging with component prefixes
- Performance optimizations (caching, batching)

## 📖 Documentation

Full documentation is available in `/docs/enterprise/`:
- [00-overview.md](../../docs/enterprise/00-overview.md) - Architecture overview
- [01-architecture.md](../../docs/enterprise/01-architecture.md) - Component design
- [02-control-plane.md](../../docs/enterprise/02-control-plane.md) - Control plane details
- [12-implementation-status.md](../../docs/enterprise/12-implementation-status.md) - Current status

## 🔒 Security Considerations

### Authentication
- JWT tokens with configurable expiry
- Email verification for user registration
- Secure token storage and validation

### Quota Enforcement
- Storage limits enforced at HTTP layer
- API rate limiting with 24-hour windows
- Returns appropriate HTTP status codes (429, 507)

### Isolation
- Each tenant runs in isolated PocketBase instance
- Separate SQLite databases per tenant
- S3 path isolation with tenant prefixes

## 🎯 Performance Targets

- **200 active tenants** per tenant node
- **100,000+ total tenants** platform-wide
- **< 100ms** tenant load time (from cache)
- **< 5s** tenant cold start (from S3)
- **99.9%** uptime with 3+ control plane nodes

## 🤝 Contributing

When contributing to enterprise features:

1. **Follow the architecture** - Respect component boundaries
2. **Add tests** - Unit tests for all new functionality
3. **Document** - Update relevant docs in `/docs/enterprise/`
4. **Thread-safe** - Use proper synchronization primitives
5. **Error handling** - Wrap errors with context

## 📝 License

MIT License - Same as PocketBase core

---

**For questions or support, see the main PocketBase repository.**
