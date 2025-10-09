# PocketBase Enterprise: Multi-Tenant Platform Documentation

**Complete technical documentation for building a horizontally scalable, multi-tenant PocketBase platform.**

---

## 📚 Documentation Index

### Getting Started
1. **[00-overview.md](00-overview.md)** - Vision, architecture overview, and core principles
   - Self-service SaaS platform model
   - Three-tier access (Cluster Admin, Cluster User, Tenant Admin)
   - Single binary, multiple modes
   - S3 as source of truth
   - Scale targets: 200 tenants/node, 100K total

### Architecture Deep Dive
2. **[01-architecture.md](01-architecture.md)** - Detailed component design
   - Control Plane (BadgerDB + Raft)
   - Tenant Nodes (stateless workers)
   - Gateway (reverse proxy)
   - Inter-component communication (Mangos v3)
   - Data flow examples

3. **[02-control-plane.md](02-control-plane.md)** - Distributed brain
   - BadgerDB schema and operations
   - Raft consensus integration
   - Placement service (tenant assignment)
   - Health monitoring
   - Mangos IPC handlers

### User Management & Platform Operations
4. **[03-cluster-users.md](03-cluster-users.md)** - Self-service SaaS customers
   - User registration and authentication
   - Tenant creation (within quotas)
   - SSO access to tenant admins
   - Quota management and requests
   - User dashboard UI

5. **[04-cluster-admin.md](04-cluster-admin.md)** - Platform operators
   - Admin authentication (long-lived tokens)
   - User management and quota approval
   - Impersonation for support
   - System monitoring dashboard
   - Admin API specifications

### Storage & Data
6. **[06-storage-strategy.md](06-storage-strategy.md)** - S3 + Litestream
   - Embedded Litestream integration
   - Tenant data layout (data.db, auxiliary.db, hooks.db)
   - Tenant lifecycle operations
   - Point-in-time recovery
   - Disaster recovery scenarios
   - Cost analysis

7. **[07-hooks-database.md](07-hooks-database.md)** - Database-backed hooks
   - hooks.db schema
   - Hook types (record, route)
   - Hook loading and execution
   - Admin UI for hook management
   - Migration from file-based hooks

### Developer Experience
8. **[09-graphql.md](09-graphql.md)** - Auto-generated GraphQL API
   - Schema generation from collections
   - Queries, mutations, subscriptions
   - DataLoader for N+1 prevention
   - Authentication and permissions
   - GraphQL Playground

### Implementation
9. **[11-implementation-phases.md](11-implementation-phases.md)** - Development roadmap
   - 32-week implementation plan (8 months)
   - 5 phases: Foundation → Hardening → Security → DX → Advanced
   - Week-by-week tasks and deliverables
   - Testing strategy
   - Success metrics

---

## 🎯 Quick Reference

### Single Binary Modes

```bash
# Control Plane (3-5 nodes for HA)
./pocketbase serve --mode=control-plane \
  --node-id=cp-1 \
  --raft-peers=cp-1:7000,cp-2:7000,cp-3:7000 \
  --dir=/data/control-plane

# Tenant Node (stateless workers, scale horizontally)
./pocketbase serve --mode=tenant-node \
  --control-plane=cp-1:8090,cp-2:8090,cp-3:8090 \
  --dir=/data/tenants

# Gateway (reverse proxy)
./pocketbase serve --mode=gateway \
  --control-plane=cp-1:8090,cp-2:8090,cp-3:8090

# All-in-One (development/testing)
./pocketbase serve --mode=all-in-one \
  --dir=/data
```

### Key Technologies

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **Control Plane DB** | BadgerDB | Embedded KV store |
| **Consensus** | Hashicorp Raft | Leader election, replication |
| **IPC** | Mangos v3 | REQ/REP, PUB/SUB patterns |
| **Replication** | Litestream (embedded) | SQLite → S3 continuous sync |
| **Storage** | S3 | Single source of truth |
| **GraphQL** | gqlgen | Auto-generated API |

### Architecture at a Glance

```
Client Request
     ↓
DNS (tenant123.platform.com)
     ↓
Gateway (extract tenant ID, route lookup)
     ↓
Control Plane (assign to node via Raft)
     ↓
Tenant Node (load from S3 if needed)
     ↓
PocketBase Instance (process request)
     ↓
Litestream (replicate to S3)
     ↓
Response
```

### Scale Targets

- **200 active tenants** per node
- **100,000+ total tenants**
- **~500 nodes** at full capacity
- **3-5 control plane** nodes (HA)
- **3+ gateway** instances (load balanced)

### Storage Cost (100K tenants @ 500MB avg)

```
S3 Standard (20% active):        $230/month
S3 Standard-IA (60% idle):       $375/month
S3 Glacier Deep (20% archived):  $10/month
Total:                           ~$665/month
Per tenant:                      $0.00665/month
```

**10x cheaper than Supabase!**

---

## 🚀 Getting Started (Development)

### 1. Prerequisites

```bash
# Go 1.24+
go version

# AWS CLI (for S3)
aws --version

# Docker (for local testing)
docker --version
```

### 2. Clone and Setup

```bash
# Clone repository
git clone https://github.com/pocketbase/pocketbase.git
cd pocketbase

# Checkout enterprise branch (when available)
git checkout enterprise

# Install dependencies
go mod download
```

### 3. Local Development Setup

```bash
# Start LocalStack (S3 emulator)
docker run -d -p 4566:4566 localstack/localstack

# Build PocketBase
go build

# Run all-in-one mode
./pocketbase serve --mode=all-in-one

# Create test tenant
curl -X POST http://localhost:8090/api/cp/tenants \
  -d '{"id":"test123","domain":"test123.localhost"}'

# Test request
curl -H "X-Tenant-ID: test123" http://localhost:8090/api/collections/users
```

### 4. Multi-Node Local Cluster

See `docker-compose.yml` for local multi-node setup:
- 3 control plane nodes
- 2 tenant nodes
- 1 gateway

```bash
docker-compose up
```

---

## 📖 Implementation Roadmap

### Phase 1: Foundation (Weeks 1-8)
✅ Control Plane (BadgerDB + Raft)
✅ Tenant Node (stateless worker)
✅ Gateway (reverse proxy)
✅ Litestream integration

### Phase 2: Production Hardening (Weeks 9-14)
⬜ Placement & migration
⬜ Resource quotas
⬜ Observability (Prometheus, tracing)

### Phase 3: Security (Weeks 15-20)
⬜ Enhanced MFA (TOTP, WebAuthn)
⬜ RBAC system
⬜ Audit logging

### Phase 4: Developer Experience (Weeks 21-26)
⬜ GraphQL layer
⬜ Tenant management UI
⬜ Developer documentation

### Phase 5: Advanced Features (Weeks 27-32)
⬜ Vector search
⬜ SSR support
⬜ Load testing & optimization

**Total: 32 weeks (~8 months)**

---

## 🔧 Development Guidelines

### Code Structure

```
pocketbase/
├── core/
│   ├── control_plane/      # Raft + BadgerDB
│   ├── tenant_node/        # Stateless workers
│   ├── gateway/            # Reverse proxy
│   ├── storage/            # Litestream integration
│   └── graphql/            # GraphQL layer
├── cmd/
│   ├── serve.go            # Multi-mode serve command
│   └── tenant.go           # Tenant CLI
├── apis/
│   ├── control_plane.go    # CP admin APIs
│   └── tenant_mgmt.go      # Tenant management
└── docs/
    └── enterprise/         # This documentation
```

### Testing Strategy

```bash
# Unit tests
go test ./core/control_plane/...
go test ./core/tenant_node/...

# Integration tests (requires Docker)
go test ./tests/integration/...

# Load tests
go test ./tests/load/... -tags=load
```

### Git Workflow

```bash
# Feature branches
git checkout -b feature/control-plane-raft
git commit -m "feat(control-plane): implement raft consensus"
git push origin feature/control-plane-raft

# Conventional commits
feat: new feature
fix: bug fix
docs: documentation
test: tests
refactor: code refactoring
perf: performance improvement
```

---

## 🎓 Learning Resources

### Raft Consensus
- [The Raft Consensus Algorithm](https://raft.github.io/)
- [Hashicorp Raft Library](https://github.com/hashicorp/raft)

### Litestream
- [Litestream Documentation](https://litestream.io/)
- [Litestream Go Library](https://github.com/benbjohnson/litestream)

### BadgerDB
- [BadgerDB Documentation](https://dgraph.io/docs/badger/)
- [BadgerDB GitHub](https://github.com/dgraph-io/badger)

### Mangos (Nanomsg)
- [Mangos Documentation](https://nanomsg.github.io/mangos/)
- [Scalability Protocols](https://nanomsg.org/documentation.html)

---

## 🤝 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for contribution guidelines.

Key areas for contribution:
- Core platform development
- Testing and quality assurance
- Documentation improvements
- Performance optimization
- Security auditing

---

## 📝 License

PocketBase Enterprise follows the same MIT license as PocketBase.

See [LICENSE.md](../../LICENSE.md) for details.

---

## 🆘 Support

- **Issues**: [GitHub Issues](https://github.com/pocketbase/pocketbase/issues)
- **Discussions**: [GitHub Discussions](https://github.com/pocketbase/pocketbase/discussions)
- **Discord**: [PocketBase Discord](https://discord.gg/pocketbase)

---

## 📊 Project Status

**Current Phase**: Planning & Documentation
**Target Launch**: Q3 2025
**Current Version**: Enterprise Preview

---

**Built with ❤️ by the PocketBase community**
