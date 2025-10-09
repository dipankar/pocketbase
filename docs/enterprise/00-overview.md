# PocketBase Enterprise: Multi-Tenant Platform

## Vision

Transform PocketBase into a horizontally scalable, multi-tenant platform capable of serving **100,000+ tenants** while maintaining the simplicity of a **single binary deployment**.

## Core Principles

### 1. Self-Service SaaS Platform

**Three-Tier Access Model**:
```
┌─────────────────────────────────────────────┐
│  CLUSTER ADMIN (Platform Operators)         │
│  - System monitoring                        │
│  - User management, quota approval          │
│  - Impersonation for support                │
│  - Authentication: Long-lived tokens        │
│  URL: admin.platform.com                    │
└─────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────┐
│  CLUSTER USER (SaaS Customers)              │
│  - Self-service registration                │
│  - Create tenants (within quota)            │
│  - SSO access to all tenant admins          │
│  - Request quota increases                  │
│  URL: app.platform.com                      │
└─────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────┐
│  TENANT ADMIN (PocketBase Admin UI)         │
│  - Manage collections, users, hooks         │
│  - Automatic SSO login (no re-auth)         │
│  URL: tenant123.platform.com/_/             │
└─────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────┐
│  END USERS (Application Users)              │
│  - CRUD operations, GraphQL                 │
│  URL: tenant123.platform.com/api/           │
└─────────────────────────────────────────────┘
```

### 2. Single Binary, Multiple Modes
```bash
# Control Plane (Raft cluster)
./pocketbase serve --mode=control-plane --raft-peers=cp1:7000,cp2:7000,cp3:7000

# Tenant Node (stateless worker)
./pocketbase serve --mode=tenant-node --control-plane=cp1:8090,cp2:8090,cp3:8090

# Gateway (load balancer)
./pocketbase serve --mode=gateway --control-plane=cp1:8090,cp2:8090,cp3:8090

# All-in-One (development/testing)
./pocketbase serve --mode=all-in-one
```

### 2. S3 as Source of Truth
- All tenant data lives in S3
- Nodes are ephemeral caches
- Litestream continuously replicates SQLite → S3
- Any node can serve any tenant
- Node failure = no data loss

### 3. Horizontal Scaling
- Target: **200 active tenants per node**
- Total capacity: **100,000+ tenants**
- Dynamic tenant placement via Raft consensus
- Elastic scaling: add nodes, control plane rebalances

### 4. Zero-Downtime Operations
- Tenants can migrate between nodes transparently
- Rolling updates without service interruption
- Graceful node shutdown with tenant handoff

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                         Internet                            │
└────────────────────────┬────────────────────────────────────┘
                         │
                    DNS Resolution
              (tenant123.platform.com)
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                    Gateway Cluster                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │Gateway-1 │  │Gateway-2 │  │Gateway-N │                  │
│  └──────────┘  └──────────┘  └──────────┘                  │
│         (Tenant ID extraction & routing)                    │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              Control Plane (Raft Cluster)                   │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  BadgerDB (Replicated via Raft)                      │  │
│  │  - Tenant Registry (id, domain, status)              │  │
│  │  - Routing Table (tenant_id → node_id)               │  │
│  │  - Node Pool (node_id, capacity, health)             │  │
│  │  - Placement Decisions                               │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐            │
│  │   CP-1     │  │   CP-2     │  │   CP-3     │            │
│  │  (Leader)  │  │(Follower)  │  │(Follower)  │            │
│  └────────────┘  └────────────┘  └────────────┘            │
│         Mangos v3 IPC (REQ/REP, PUB/SUB)                    │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                   Tenant Node Pool                          │
│  ┌─────────────┐  ┌─────────────┐       ┌─────────────┐    │
│  │   Node-1    │  │   Node-2    │  ...  │  Node-500   │    │
│  │ 200 tenants │  │ 200 tenants │       │ 200 tenants │    │
│  └─────────────┘  └─────────────┘       └─────────────┘    │
│         ▲                 ▲                      ▲           │
│         │ Litestream      │ Litestream           │           │
│         │ Replication     │ Replication          │           │
│         ▼                 ▼                      ▼           │
└─────────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  S3 (Source of Truth)                       │
│                                                              │
│  s3://bucket/                                                │
│  ├── tenants/                                                │
│  │   ├── tenant_001/                                         │
│  │   │   ├── litestream/                                     │
│  │   │   │   ├── data.db (WAL segments)                     │
│  │   │   │   ├── auxiliary.db (WAL segments)                │
│  │   │   │   └── hooks.db (hooks database)                  │
│  │   │   └── metadata.json                                   │
│  │   ├── tenant_002/                                         │
│  │   └── ...                                                 │
│  └── control-plane/                                          │
│      └── badger-snapshots/                                   │
└─────────────────────────────────────────────────────────────┘
```

## Key Technologies

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **Control Plane DB** | BadgerDB | Fast embedded KV store with Raft replication |
| **Consensus** | Hashicorp Raft | Leader election, distributed state |
| **IPC** | Mangos v3 | Scalable IPC patterns (REQ/REP, PUB/SUB) |
| **Replication** | Litestream (embedded) | Continuous SQLite → S3 replication |
| **Storage** | S3 | Source of truth for all tenant data |
| **Tenant DB** | SQLite | Per-tenant databases (data.db, auxiliary.db) |
| **GraphQL** | gqlgen | Auto-generated GraphQL from collections |

## Scale Targets

| Metric | Target |
|--------|--------|
| Tenants per node | 200 active |
| Total tenants | 100,000+ |
| Nodes required (at capacity) | ~500 |
| Control plane cluster | 3-5 nodes (HA) |
| Gateway instances | 3+ (load balanced) |
| Tenant DB size (avg) | 100MB - 1GB |
| S3 storage (100K tenants @ 500MB avg) | ~50TB |

## Request Flow

1. **DNS Resolution**: `tenant123.platform.com` → Gateway IP
2. **Tenant Extraction**: Gateway extracts tenant ID from domain
3. **Routing Lookup**: Gateway queries control plane via Mangos IPC
4. **Node Assignment**: Control plane returns node address (or assigns if new)
5. **Tenant Loading**: Node checks cache, loads from S3 if needed
6. **Request Processing**: Tenant's PocketBase instance handles request
7. **Continuous Sync**: Litestream replicates changes to S3
8. **Response**: Proxied back through gateway to client

## Tenant Lifecycle States

```
┌─────────┐
│ Created │ (Metadata in control plane)
└────┬────┘
     │
     ▼
┌─────────┐
│Assigning│ (Control plane selects node)
└────┬────┘
     │
     ▼
┌─────────┐
│Deploying│ (Node downloads from S3, bootstraps)
└────┬────┘
     │
     ▼
┌─────────┐
│ Active  │ ◄──┐ (Serving requests)
└────┬────┘    │
     │         │
     ▼         │
┌─────────┐    │
│  Idle   │────┘ (No recent activity, but still in memory)
└────┬────┘
     │
     ▼
┌─────────┐
│ Evicted │ (Removed from node cache, data in S3)
└────┬────┘
     │
     ▼
┌─────────┐
│Archived │ (Long-term inactive, S3 Glacier)
└────┬────┘
     │
     ▼
┌─────────┐
│ Deleted │ (Soft delete, can be recovered)
└─────────┘
```

## Core Features

### Multi-Tenancy
- ✅ Complete tenant isolation (separate databases)
- ✅ Dynamic tenant placement across nodes
- ✅ Tenant migration without downtime
- ✅ Per-tenant resource quotas (storage, API requests, CPU)

### High Availability
- ✅ Control plane Raft cluster (3-5 nodes)
- ✅ Tenant node auto-scaling
- ✅ Automatic failover on node failure
- ✅ Zero data loss (S3 + Litestream)

### Scalability
- ✅ Horizontal scaling (add nodes to handle more tenants)
- ✅ 200 active tenants per node
- ✅ 100,000+ total tenant capacity
- ✅ Stateless nodes (ephemeral infrastructure)

### Security & Compliance
- ✅ Enhanced MFA (TOTP, backup codes, WebAuthn)
- ✅ RBAC with custom roles
- ✅ Comprehensive audit logging
- ✅ SOC2/HIPAA compliance features
- ✅ Per-tenant encryption keys

### Developer Experience
- ✅ GraphQL API (auto-generated from collections)
- ✅ Tenant-scoped hooks (JavaScript VM per tenant)
- ✅ Multi-tenant admin UI
- ✅ Tenant CLI for management

### Advanced Features
- ✅ Vector search (embeddings, similarity)
- ✅ Real-time subscriptions (per-tenant channels)
- ✅ Backup/restore per tenant
- ✅ Usage analytics and billing

## Deployment Models

### 1. Small Deployment (< 1,000 tenants)
```
- 1x All-in-One mode (development)
- OR: 3x Control Plane + 5x Tenant Nodes + 2x Gateway
```

### 2. Medium Deployment (1,000 - 10,000 tenants)
```
- 3x Control Plane (Raft cluster)
- 50x Tenant Nodes (~200 tenants each)
- 3x Gateway (load balanced)
```

### 3. Large Deployment (10,000 - 100,000 tenants)
```
- 5x Control Plane (Raft cluster, geographically distributed)
- 500x Tenant Nodes (auto-scaled)
- 10x Gateway (multi-region)
```

## Next Steps

1. [Architecture Deep Dive](01-architecture.md) - Detailed component design
2. [Control Plane](02-control-plane.md) - Raft + BadgerDB implementation
3. [Cluster Users](03-cluster-users.md) - Self-service SaaS customers
4. [Cluster Admin](04-cluster-admin.md) - Platform operations dashboard
5. [Storage Strategy](06-storage-strategy.md) - S3 + Litestream details
6. [Hooks Database](07-hooks-database.md) - Database-backed hooks system
7. [GraphQL](09-graphql.md) - Auto-generated API layer
8. [Implementation Phases](11-implementation-phases.md) - Development roadmap
