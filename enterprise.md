✦ PocketBase Enterprise: Multi-tenant Platform Specification

  Table of Contents
   1. Overview (#overview)
   2. Architecture (#architecture)
   3. Core Components (#core-components)
   4. Multi-tenancy System (#multi-tenancy-system)
   5. Storage Integration (#storage-integration)
   6. Security & Compliance (#security--compliance)
   7. API Enhancements (#api-enhancements)
   8. Gateway & Load Balancing (#gateway--load-balancing)
   9. Distributed Hook System (#distributed-hook-system)
   10. Monitoring & Operations (#monitoring--operations)
   11. Implementation Roadmap (#implementation-roadmap)
   12. Resource Requirements (#resource-requirements)
   13. Cost Analysis (#cost-analysis)

  Overview

  PocketBase Enterprise is a multi-tenant platform built on top of the standard PocketBase framework. It provides enterprise-grade features including SOC2/HIPAA
  compliance, GraphQL APIs, vector search, distributed hook execution, and scalable multi-tenant architecture with automatic tenant archiving.

  Key Features
   - Multi-tenant Architecture: Complete tenant isolation with separate databases
   - Enterprise Security: MFA, RBAC, audit logging, SOC2/HIPAA compliance
   - Scalable Storage: S3 integration with Litestream replication
   - Advanced APIs: GraphQL, vector search, SSR support
   - Distributed System: Load balancing, gateway, distributed hooks
   - Observability: Comprehensive monitoring, alerting, and metrics

  Architecture

  High-Level Architecture

    1 ┌─────────────────────────────────────────────────────────────────┐
    2 │                    Client Applications                          │
    3 ├─────────────────────────────────────────────────────────────────┤
    4 │                    Load Balancer/Gateway                        │
    5 ├─────────────────────────────────────────────────────────────────┤
    6 │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
    7 │  │   Tenant1   │  │   Tenant2   │  │   TenantN   │              │
    8 │  │  Sandbox    │  │  Sandbox    │  │  Sandbox    │              │
    9 │  └─────────────┘  └─────────────┘  └─────────────┘              │
   10 ├─────────────────────────────────────────────────────────────────┤
   11 │                    Control Plane                               │
   12 │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
   13 │  │  Registry   │  │  Metrics    │  │  Security   │              │
   14 │  │    & DB     │  │  Manager    │  │  Manager    │              │
   15 │  └─────────────┘  └─────────────┘  └─────────────┘              │
   16 └─────────────────────────────────────────────────────────────────┘

  Component Architecture
   - Control Plane: Central tenant registry, system metrics, security management
   - Tenant Nodes: Distributed processing nodes with PocketBase instances
   - Gateway: Load-balanced request routing with caching
   - Storage: S3-backed storage with Litestream replication
   - Monitoring: Prometheus metrics, alerting, health checks

  Core Components

  1. Tenant Manager Plugin
  Manages tenant lifecycle, registration, and configuration.

  Features:
   - Tenant creation, deletion, and updates
   - Tenant status management (active, archived, maintenance)
   - Resource quota enforcement
   - Tenant-specific configuration

  API Endpoints:

   1 POST   /api/tenants               # Create tenant
   2 GET    /api/tenants               # List tenants
   3 GET    /api/tenants/{id}          # Get tenant details
   4 PUT    /api/tenants/{id}          # Update tenant
   5 DELETE /api/tenants/{id}          # Delete tenant
   6 POST   /api/tenants/{id}/archive   # Archive tenant
   7 POST   /api/tenants/{id}/resume    # Resume tenant

  2. Storage Plugin System
  Provides S3-backed storage with local caching and Litestream replication.

  Features:
   - S3 integration for file storage and database backups
   - Local caching for performance optimization
   - Litestream real-time database replication
   - Automatic tenant data archiving to S3 Glacier

  3. Security Plugin
  Implements enterprise security features including MFA and RBAC.

  Features:
   - Multi-factor authentication (TOTP, backup codes)
   - Role-based access control
   - Audit logging with export capabilities
   - SOC2 and HIPAA compliance features

  4. API Enhancement Plugins
  Extends PocketBase APIs with GraphQL and vector search.

  Features:
   - GraphQL API generation from PocketBase collections
   - Vector search with embedding support
   - Server-side rendering (SSR) optimization
   - Custom API endpoints

  Multi-tenancy System

  Tenant Isolation
  Each tenant has complete isolation:
   - Separate Databases: Dedicated data.db and auxiliary.db SQLite files
   - Separate File Storage: Tenant-specific directories in S3
   - Sandboxed Hooks: Tenant-specific JavaScript execution environments
   - Independent Configuration: Tenant-specific settings and features

  Tenant Lifecycle Management
   - Active: Tenant is running and serving requests
   - Archived: Tenant data is compressed and moved to S3 for cost savings
   - Maintenance: Tenant is undergoing maintenance operations
   - Resuming: Archived tenant is being restored to active state

  Resource Management
  Each tenant has configurable quotas:
   - Storage: Maximum database and file storage
   - Memory: Memory usage limits
   - CPU: CPU quota enforcement
   - Connections: Maximum concurrent connections
   - API Requests: Rate limiting per tenant

  Storage Integration

  S3 Storage Plugin
  Uses Amazon S3 (or compatible) for persistent storage:
   - File Storage: Tenant file uploads stored in S3 buckets
   - Database Backups: Automated backups of SQLite databases
   - Archive Storage: Inactive tenant data moved to S3 Glacier

  Litestream Replication
  Real-time SQLite replication to S3:
   - Zero RPO: Continuous replication with write-ahead logs
   - Point-in-Time Recovery: Restore to any point in time
   - Cross-Region Replication: Disaster recovery across regions

  Local Caching
  Performance optimization with local caching:
   - Hot Data: Frequently accessed tenant data cached locally
   - Lazy Loading: Load tenant data on-demand
   - Eviction Policies: LRU and tenant-specific eviction

  Security & Compliance

  Multi-Factor Authentication (MFA)
   - TOTP Support: Time-based one-time passwords
   - Backup Codes: Recovery codes for MFA
   - Device Management: Trusted device registration
   - Session Management: Secure session handling

  Role-Based Access Control (RBAC)
   - Granular Permissions: Fine-grained access control
   - Custom Roles: Tenant-specific role creation
   - Policy Engine: Declarative access policies
   - Audit Trails: Comprehensive access logging

  Compliance Features
   - SOC2 Compliance: Security, availability, processing integrity, confidentiality, privacy
   - HIPAA Compliance: PHI detection, encryption, audit logging
   - GDPR Compliance: Data portability, right to erasure
   - Audit Logging: Comprehensive event logging with export

  Data Protection
   - Encryption: At-rest and in-transit encryption
   - Key Management: Tenant-specific encryption keys
   - Access Controls: Fine-grained data access policies
   - Data Masking: Sensitive data obfuscation

  API Enhancements

  GraphQL API
  Auto-generated GraphQL API from PocketBase collections:
   - Schema Generation: Automatic schema from collection definitions
   - Query Optimization: Efficient query execution
   - Mutations: Create, update, delete operations
   - Subscriptions: Real-time data updates

  Vector Search
  Intelligent search with embedding support:
   - Semantic Search: Meaning-based search rather than keyword matching
   - Similarity Matching: Find similar content
   - Recommendation Engine: Content recommendations
   - Natural Language Processing: Text analysis and categorization

  Server-Side Rendering (SSR)
  Optimized for web frameworks:
   - HTML Responses: Direct HTML generation
   - Template System: Flexible templating engine
   - Asset Optimization: CSS/JS bundling and minification
   - Progressive Enhancement: Graceful degradation support

  Gateway & Load Balancing

  Reverse Proxy
  Smart request routing:
   - Tenant Identification: Automatic tenant detection
   - Load Distribution: Even distribution across nodes
   - Health Checks: Node health monitoring
   - Circuit Breaking: Fail-fast mechanisms

  Load Balancing Algorithms
  Multiple load balancing strategies:
   - Round Robin: Sequential distribution
   - Least Connections: Fewest active connections
   - Weighted Round Robin: Node capacity weighting
   - IP Hash: Session affinity

  Caching Layer
  Performance optimization:
   - HTTP Caching: Standard HTTP cache headers
   - Content Caching: Frequently accessed content
   - Query Caching: Database query results
   - Compression: GZIP and Brotli compression

  Distributed Hook System

  Tenant-Specific Sandboxing
  Secure JavaScript execution:
   - Isolated Environment: Tenant-specific execution contexts
   - Resource Limits: Memory, CPU, and time quotas
   - API Restrictions: Controlled access to system APIs
   - Error Handling: Graceful error recovery

  Hook Distribution
  Intelligent hook routing:
   - Node Selection: Optimal node assignment
   - Load Balancing: Even hook distribution
   - Failover: Automatic retry on node failures
   - Result Aggregation: Combined hook results

  Hook Management
  Comprehensive hook lifecycle:
   - CRUD Operations: Create, read, update, delete hooks
   - Version Control: Hook version management
   - Testing: Hook testing and validation
   - Monitoring: Hook execution metrics

  Event System
  Rich event model:
   - Record Events: Create, update, delete
   - Auth Events: Login, logout, password reset
   - File Events: Upload, download, delete
   - Custom Events: Tenant-defined events

  Monitoring & Operations

  Metrics Collection
  Comprehensive system metrics:
   - System Metrics: CPU, memory, disk, network
   - Tenant Metrics: Resource usage per tenant
   - Hook Metrics: Execution performance
   - API Metrics: Request rates and latencies

  Alerting System
  Proactive issue detection:
   - Threshold Alerts: Metric-based alerts
   - Anomaly Detection: Unusual behavior detection
   - Multi-channel Notifications: Slack, email, webhook
   - Escalation Policies: Alert escalation workflows

  Health Checks
  System reliability monitoring:
   - Node Health: Individual node status
   - Tenant Health: Tenant-specific health
   - Service Health: API and database status
   - Dependency Health: External service status

  Operations Dashboard
  Centralized management interface:
   - Real-time Monitoring: Live system metrics
   - Tenant Management: Tenant lifecycle operations
   - Alert Management: Alert configuration and resolution
   - Performance Analytics: Usage and performance insights

  Implementation Roadmap

  Phase 1: Foundation (Weeks 1-3)
   - Control Plane Development: Tenant registry, basic APIs
   - Tenant Manager Plugin: Core tenant management
   - Storage Integration: S3 and Litestream setup
   - Basic Security: Authentication and authorization

  Phase 2: Multi-tenant Core (Weeks 4-6)
   - Tenant Node System: Distributed node management
   - Tenant Isolation: Complete data and resource isolation
   - Resource Management: Quotas and limits enforcement
   - Basic Gateway: Load balancing and routing

  Phase 3: Enterprise Security (Weeks 7-9)
   - MFA Implementation: TOTP and backup codes
   - RBAC System: Role-based access control
   - Audit Logging: Comprehensive event logging
   - Compliance Features: SOC2/HIPAA support

  Phase 4: Advanced APIs (Weeks 10-12)
   - GraphQL API: Schema generation and execution
   - Vector Search: Embedding and similarity search
   - SSR Support: HTML response generation
   - API Enhancement: Custom endpoints and middleware

  Phase 5: Gateway & Load Balancing (Weeks 13-15)
   - Reverse Proxy: Intelligent request routing
   - Load Balancer: Multiple balancing algorithms
   - Caching Layer: Performance optimization
   - Health Checks: System reliability monitoring

  Phase 6: Distributed Hooks (Weeks 16-18)
   - Sandbox System: Secure JavaScript execution
   - Hook Distribution: Node-based execution
   - Hook Management: Lifecycle and monitoring
   - Event System: Rich event model

  Phase 7: Monitoring & Operations (Weeks 19-21)
   - Metrics Collection: Comprehensive monitoring
   - Alerting System: Proactive notifications
   - Operations Dashboard: Centralized management
   - Health Checks: System reliability

  Phase 8: Testing & Validation (Weeks 22-24)
   - Unit Testing: Component-level testing
   - Integration Testing: System-level testing
   - Performance Testing: Load and stress testing
   - Security Testing: Penetration testing

  Resource Requirements

  Minimum Requirements
   - Control Plane: 2 vCPU, 4GB RAM, 50GB SSD
   - Tenant Nodes: 4 vCPU, 8GB RAM, 100GB SSD (per node)
   - Gateway: 2 vCPU, 4GB RAM, 20GB SSD
   - S3 Storage: 1TB minimum (pay-as-you-go)

  Recommended Configuration
   - Control Plane: 4 vCPU, 8GB RAM, 100GB SSD
   - Tenant Nodes: 8 vCPU, 16GB RAM, 200GB SSD (3+ nodes)
   - Gateway: 4 vCPU, 8GB RAM, 50GB SSD
   - S3 Storage: 10TB+ (includes archival)

  Scalability Guidelines
   - Small Deployment: 100-500 tenants
   - Medium Deployment: 500-2000 tenants
   - Large Deployment: 2000+ tenants
   - High Availability: 3+ control plane nodes, 3+ gateway nodes

  Cost Analysis

  Infrastructure Costs (Monthly)

  ┌───────────────┬───────┬────────┬───────┐
  │ Component     │ Small │ Medium │ Large │
  ├───────────────┼───────┼────────┼───────┤
  │ Control Plane │ $20   │ $40    │ $80   │
  │ Tenant Nodes  │ $80   │ $240   │ $480  │
  │ Gateway       │ $20   │ $40    │ $80   │
  │ S3 Storage    │ $25   │ $100   │ $250  │
  │ Total         │ $145  │ $420   │ $890  │
  └───────────────┴───────┴────────┴───────┘


  Per-Tenant Costs
   - Small Deployment: $0.29-$1.45 per tenant/month
   - Medium Deployment: $0.21-$0.84 per tenant/month
   - Large Deployment: $0.18-$0.45 per tenant/month

  Comparison with Competitors

  ┌──────────────────┬───────────────────────┬────────────────┬─────────────────┐
  │ Feature          │ PocketBase Enterprise │ Supabase Pro   │ Firebase        │
  ├──────────────────┼───────────────────────┼────────────────┼─────────────────┤
  │ Cost per Tenant  │ $0.18-1.45            │ $25-28         │ $25-400         │
  │ Multi-tenancy    │ Native                │ Limited        │ Limited         │
  │ Custom Logic     │ JavaScript Hooks      │ Edge Functions │ Cloud Functions │
  │ Vector Search    │ Included              │ Add-on         │ Add-on          │
  │ SOC2 Compliance  │ Native                │ Enterprise     │ Enterprise      │
  │ HIPAA Compliance │ Native                │ Add-on         │ Business        │
  └──────────────────┴───────────────────────┴────────────────┴─────────────────┘


  Savings Analysis
  Compared to Supabase:
   - Cost Savings: 14x-155x reduction
   - Multi-tenancy: Native vs workaround
   - Customization: More flexible per-tenant logic
   - Performance: Better isolation and lower latency

  This specification provides a comprehensive blueprint for building a scalable, secure, and compliant multi-tenant PocketBase platform with all the enterprise features
  needed for production deployment.



## Disk Space Management

The control plane uses BadgerDB for metadata storage with comprehensive disk space management to prevent disk exhaustion.

### Automatic Maintenance

| Task | Frequency | Action |
|------|-----------|--------|
| Garbage Collection | 5 minutes | Reclaim deleted values |
| Disk Usage Check | 1 minute | Monitor usage, trigger alerts |
| Compaction | 1 hour | Flatten LSM tree |

### Disk Limits

**Default Configuration:**
- Max Disk Usage: 10 GB
- Warning Threshold: 80% (8 GB)
- Critical Threshold: 95% (9.5 GB)

### BadgerDB Production Settings

```go
opts.NumVersionsToKeep = 1           // Keep only latest version
opts.CompactL0OnClose = true         // Compact on close
opts.ValueLogFileSize = 64 << 20     // 64 MB value log files
opts.NumLevelZeroTables = 5          // Trigger compaction after 5 L0 tables
opts.NumLevelZeroTablesStall = 10    // Stall writes after 10 L0 tables
opts.ValueLogMaxEntries = 500000     // Max entries per value log file
```

### Monitoring

**Admin API Endpoint:**
```bash
GET /api/enterprise/admin/disk
```

**Response:**
```json
{
  "disk": {
    "diskUsageBytes": 1234567890,
    "diskUsageGB": 1.15,
    "usagePercent": 11.5,
    "lastGCTime": "2025-10-09T10:30:00Z"
  }
}
```

**Health Checks:**
- Integrated into `/health/ready` endpoint
- Reports unhealthy at 95%+ usage
- Automatic emergency cleanup

### Emergency Procedures

**At Critical Disk Usage (95%+):**
1. Immediate aggressive garbage collection
2. Emergency database compaction
3. Logging of CRITICAL alerts
4. Health check reports unhealthy

**Manual Remediation:**
- Increase disk limits in configuration
- Delete unused tenants/users via API
- Expand storage volume

### Capacity Planning

**Typical Control Plane Data:**
- Tenant metadata: ~1 KB per tenant
- User metadata: ~500 bytes per user
- Node info: ~200 bytes per node

**Example with 10,000 tenants:**
- Raw data: ~23 MB
- With LSM amplification (3x): ~70 MB
- With historical versions: ~100-200 MB

Even with 100,000 tenants, control plane should stay under 2 GB.

**Scaling to 1M+ Tenants:**
- Updated to 100 GB disk limit (supports 1M+ tenants)
- Per-tenant overhead: ~1 KB metadata + activity tracking
- Estimated storage for 1M tenants: ~20-30 GB (with compression)


## Scaling to 1 Million Tenants

### Overview

PocketBase Enterprise is optimized for extreme long-tail usage patterns, supporting 1M+ tenants where 95-99% are inactive at any time. The system uses a three-tier storage architecture to minimize costs while maintaining fast access for active tenants.

### Three-Tier Storage Architecture

#### Tier 1: Hot Storage (Active Tenants)
**Characteristics:**
- In-memory tenant instances
- Active Litestream replication to S3
- Sub-100ms response times
- Typically 5,000-10,000 tenants (0.5-1% of total)

**Storage:**
- Local disk + RAM on tenant nodes
- S3 Standard with continuous WAL replication
- Estimated cost: $0.023/GB/month

#### Tier 2: Warm Storage (Inactive < 90 days)
**Characteristics:**
- Unloaded from memory, databases in S3
- Litestream replication stopped (cost optimization)
- 5-15 second restore time
- Typically 50,000-100,000 tenants (5-10% of total)

**Storage:**
- S3 Standard storage
- Static snapshots (no continuous replication)
- Estimated cost: $0.023/GB/month

#### Tier 3: Cold Storage (Archived > 90 days)
**Characteristics:**
- S3 Glacier Deep Archive
- 3-5 hour restore time (expedited: 1-5 minutes)
- Typically 900,000+ tenants (90%+ of total)

**Storage:**
- S3 Glacier Deep Archive
- Estimated cost: $0.001/GB/month (96% savings vs hot)

### Automatic Archiving System

#### Configuration

```go
type ArchiveConfig struct {
    LitestreamStopThreshold time.Duration // 3 days - stop replication
    WarmThreshold           time.Duration // 7 days - unload from memory
    ColdThreshold           time.Duration // 90 days - move to Glacier
    CheckInterval           time.Duration // 1 hour - archiving check frequency
}
```

#### Archiving Workflow

**Step 1: Litestream Optimization (3 days inactive)**
- Stop Litestream replication (saves S3 PUT costs)
- Create final snapshot backup
- Keep tenant loaded in memory
- **Savings**: ~$0.005/tenant/month in S3 writes

**Step 2: Warm Archiving (7 days inactive)**
- Unload tenant from memory (frees node capacity)
- Keep databases in S3 Standard
- Stop all replication services
- **Savings**: Frees RAM for active tenants

**Step 3: Cold Archiving (90 days inactive)**
- Transition to S3 Glacier Deep Archive
- 96% reduction in storage costs
- Restore on-demand when accessed
- **Savings**: ~$0.022/tenant/month per GB

#### S3 Lifecycle Policies

Automatic transitions configured via AWS S3:

```
Rule: archive-tenants
- Transition to Glacier Deep Archive after 90 days
- Applies to: tenants/* prefix

Rule: wal-intelligent
- Transition Litestream WALs to Intelligent-Tiering after 7 days
- Applies to: tenants/*/litestream/* prefix

Rule: cleanup-snapshots
- Delete old snapshots after 180 days
- Applies to: tenants/*/snapshots/* prefix
```

### Tenant Activity Tracking

The control plane tracks activity for intelligent archiving decisions:

```go
type TenantActivity struct {
    TenantID        string      // Unique tenant identifier
    LastAccess      time.Time   // Last API request timestamp
    AccessCount     int64       // Total access count
    StorageTier     StorageTier // Current tier: hot/warm/cold

    RequestsLast24h int64       // Rolling 24-hour counter
    RequestsLast7d  int64       // Rolling 7-day counter

    ArchiveDate     *time.Time  // When archived
    RestoreCount    int         // Number of restores from cold
}
```

### Admin APIs for Archive Management

**Get Tenant Activity:**
```bash
GET /api/enterprise/admin/archive/activity?tenantId=tenant_123
```

**List Inactive Tenants:**
```bash
GET /api/enterprise/admin/archive/inactive?days=30
```

**Manual Archive:**
```bash
POST /api/enterprise/admin/archive/tenant
{
  "tenantId": "tenant_123",
  "tier": "cold"  // "warm" or "cold"
}
```

**Restore Tenant:**
```bash
POST /api/enterprise/admin/archive/restore
{
  "tenantId": "tenant_123"
}
```

**Archive Statistics:**
```bash
GET /api/enterprise/admin/archive/stats
```

Response:
```json
{
  "storage": {
    "hot": 8543,
    "warm": 67234,
    "cold": 924223,
    "total": 1000000
  }
}
```

### Cost Analysis for 1M Tenants

#### Infrastructure Costs (Monthly)

| Component | Configuration | Cost |
|-----------|--------------|------|
| **Control Plane** | 3 nodes × 4 vCPU, 8GB, 100GB | $240 |
| **Tenant Nodes** | 50 nodes × 4 vCPU, 8GB, 100GB | $2,000 |
| **Gateway** | 3 nodes × 2 vCPU, 4GB | $120 |
| **S3 Storage** | | |
| - Hot (10k × 500MB) | 5 TB Standard | $115 |
| - Warm (90k × 100MB) | 9 TB Standard | $207 |
| - Cold (900k × 50MB) | 45 TB Glacier | $45 |
| - Litestream WALs | 500 GB | $12 |
| **Total** | | **$2,739/month** |

**Per-Tenant Cost:** $0.0027/month = **$0.33/year per tenant**

#### Cost Breakdown by Tier

| Tier | Tenants | Storage | Monthly Cost | Cost/Tenant |
|------|---------|---------|--------------|-------------|
| Hot | 10,000 | 5 TB | $115 + node costs | $0.21 |
| Warm | 90,000 | 9 TB | $207 | $0.0023 |
| Cold | 900,000 | 45 TB | $45 | $0.00005 |

#### Savings vs Alternatives

**Compared to Supabase Pro ($25/tenant/month):**
- PocketBase Enterprise: $0.0027/tenant/month
- **Savings: 9,259x reduction** (or $24.997/tenant/month)
- For 1M tenants: **$24.997M/month savings**

**Compared to Firebase Blaze ($0.20/GB/month + compute):**
- Estimated Firebase cost for 1M tenants: $1M-2M/month
- PocketBase Enterprise: $2,739/month
- **Savings: 365x-730x reduction**

### Performance Characteristics

#### Response Time SLAs

| Scenario | P50 | P99 | Notes |
|----------|-----|-----|-------|
| Hot tenant (cached) | 50ms | 200ms | In-memory, ready to serve |
| Warm tenant (restore) | 5s | 15s | Load from S3 Standard |
| Cold tenant (Glacier) | 4hr | 6hr | Standard Glacier restore |
| Cold tenant (expedited) | 2min | 5min | Expedited restore ($30/restore) |

#### Throughput Capacity

**Per Tenant Node (4 vCPU, 8GB):**
- Max concurrent tenants: 100-200
- Requests/sec per tenant: 100-500
- Total cluster capacity: 50 nodes × 150 tenants = 7,500 active tenants

**Auto-Scaling Triggers:**
- Scale up: Average node utilization > 75%
- Scale down: Average node utilization < 40%
- Min nodes: 10 (1,000 tenant capacity)
- Max nodes: 100 (10,000 tenant capacity)

### Implementation Status

#### ✅ Completed (Phase 1 & 2)

- [x] Control plane with 100 GB disk capacity
- [x] Activity tracking database schema
- [x] Automatic tenant archiving system
- [x] Three-tier storage implementation
- [x] Litestream optimization (stop after 3 days)
- [x] S3 Glacier lifecycle policies
- [x] Admin APIs for archive management
- [x] Health checks and monitoring
- [x] Prometheus metrics integration

#### 🚧 In Progress

- [ ] Gateway read-through caching with Redis
- [ ] Predictive tenant loading based on patterns
- [ ] Auto-scaling for tenant nodes
- [ ] Advanced alerting system

#### 📋 Planned (Phase 3)

- [ ] ML-based tenant access prediction
- [ ] Cross-region Glacier restore
- [ ] Cost optimization dashboard
- [ ] Archive analytics and reporting

### Capacity Planning Guide

#### Small Deployment (< 10k tenants)

```
Control Plane: 1 node, 10 GB disk
Tenant Nodes:  5-10 nodes
Expected Cost: $400-600/month
Active tenants: 500-1,000
```

#### Medium Deployment (10k-100k tenants)

```
Control Plane: 3 nodes (HA), 100 GB disk each
Tenant Nodes:  20-30 nodes
Expected Cost: $1,200-1,800/month
Active tenants: 2,000-5,000
Archive tier:  ~80% of tenants in warm/cold
```

#### Large Deployment (100k-1M tenants)

```
Control Plane: 3-5 nodes (HA), 100 GB disk each
Tenant Nodes:  50-100 nodes (auto-scaling)
Expected Cost: $2,500-4,000/month
Active tenants: 5,000-10,000
Archive tier:  ~95% of tenants in cold storage
Cost per tenant: $0.0025-0.004/month
```

### Monitoring and Observability

#### Key Metrics to Track

**Tenant Distribution:**
```
enterprise_tenants_by_tier{tier="hot"} 8543
enterprise_tenants_by_tier{tier="warm"} 67234
enterprise_tenants_by_tier{tier="cold"} 924223
```

**Archive Operations:**
```
enterprise_archive_transitions_total{from="hot",to="warm"} 1234
enterprise_restore_duration_seconds{tier="cold",percentile="p99"} 3.2
```

**Cost Tracking:**
```
enterprise_storage_cost_monthly{tier="hot"} 115
enterprise_storage_cost_monthly{tier="warm"} 207
enterprise_storage_cost_monthly{tier="cold"} 45
```

**Performance:**
```
enterprise_tenant_load_duration{tier="warm"} histogram
enterprise_request_duration{tenant_tier="hot"} histogram
```

### Operational Procedures

#### Emergency Restore (Cold Tenant)

For business-critical cold tenant needing immediate access:

```bash
# 1. Check current status
GET /api/enterprise/admin/archive/activity?tenantId=tenant_123

# 2. Initiate expedited restore ($30 fee)
POST /api/enterprise/admin/archive/restore
{
  "tenantId": "tenant_123",
  "expedited": true
}

# 3. Monitor restore progress
GET /api/enterprise/admin/archive/activity?tenantId=tenant_123
# Wait 1-5 minutes for Glacier restore to complete

# 4. Verify tenant is accessible
GET /health/ready
```

#### Bulk Archive Operation

For planned maintenance or cost optimization:

```bash
# 1. List inactive tenants
GET /api/enterprise/admin/archive/inactive?days=60

# 2. Review and archive in batches
for tenant in inactive_tenants:
  POST /api/enterprise/admin/archive/tenant
  {
    "tenantId": tenant.id,
    "tier": "cold"
  }

# 3. Verify archive stats
GET /api/enterprise/admin/archive/stats
```

### Best Practices

#### Cost Optimization

1. **Set appropriate thresholds** based on usage patterns
2. **Monitor archive transitions** to identify optimization opportunities
3. **Use expedited restore sparingly** ($30/restore vs $0 standard)
4. **Enable S3 Intelligent-Tiering** for Litestream WALs
5. **Clean up old snapshots** automatically (180-day retention)

#### Performance Optimization

1. **Identify high-value tenants** and keep them hot
2. **Pre-warm tenants** before scheduled events (webinars, launches)
3. **Use Redis gateway cache** for read-heavy workloads
4. **Monitor restore latency** and adjust tier thresholds
5. **Scale tenant nodes proactively** before hitting capacity

#### Reliability

1. **Test restore procedures** regularly (monthly)
2. **Monitor health checks** for all components
3. **Set up alerts** for critical thresholds:
   - Control plane disk > 80 GB
   - Restore failures > 1%
   - Average restore time > 10s (warm tier)
4. **Maintain 3x node capacity** for surge handling
5. **Enable cross-region replication** for disaster recovery

---

This architecture enables PocketBase Enterprise to serve 1 million tenants at a fraction of traditional costs while maintaining excellent performance for active users.


## Handling Hotspotting and Large Tenants

### Overview

While the three-tier storage architecture handles the majority of tenants efficiently, some customers may have exceptional resource needs ("hotspots"):
- **Large databases**: 5-50 GB instead of typical 50-500 MB
- **High traffic**: 100k+ requests/day vs typical 100-1k/day
- **Complex queries**: Heavy CPU usage from analytics or reporting
- **Noisy neighbor risk**: One tenant impacting co-located tenants

PocketBase Enterprise includes a comprehensive **Resource Management System** that automatically detects, classifies, and handles these edge cases.

### Tenant Tier Classification

Tenants are automatically classified into tiers based on actual resource usage:

```go
const (
    TenantTierMicro      // < 10 MB, < 1k req/day
    TenantTierSmall      // < 100 MB, < 10k req/day
    TenantTierMedium     // < 1 GB, < 100k req/day
    TenantTierLarge      // < 5 GB, < 1M req/day
    TenantTierEnterprise // > 5 GB, dedicated resources
)
```

#### Tier Quotas and Limits

| Tier | Database | Requests/Day | Memory | CPU % | Priority |
|------|----------|--------------|--------|-------|----------|
| **Micro** | 10 MB | 1,000 | 50 MB | 5% | 1 |
| **Small** | 100 MB | 10,000 | 200 MB | 10% | 2 |
| **Medium** | 1 GB | 100,000 | 1 GB | 25% | 3 |
| **Large** | 5 GB | 1,000,000 | 4 GB | 50% | 4 |
| **Enterprise** | 50 GB | 10,000,000 | 16 GB | 100% | 5 |

#### Automatic Tier Upgrades

The ResourceManager continuously monitors tenant metrics and automatically upgrades tiers when usage exceeds thresholds:

```go
type TenantResourceMetrics struct {
    TenantID            string
    Tier                TenantTier

    // Database metrics
    DatabaseSizeMB      int64
    DatabaseGrowthRate  float64  // MB/day
    QueryComplexity     float64  // 0-1 score
    AvgQueryTimeMs      float64

    // Traffic metrics
    RequestsLast24h     int64
    PeakRequestsPerMin  int64
    AvgResponseTimeMs   float64
    ErrorRate           float64

    // Resource consumption
    MemoryUsageMB       int64
    CPUUsagePercent     float64
    DiskIOPS            int64
    NetworkMBPS         float64

    // Behavior classification
    IsHotspot           bool     // High resource usage
    IsSpiking           bool     // Sudden traffic increase
    HotspotScore        float64  // 0-1, higher = more resources
}
```

### Hotspot Detection

The system calculates a **Hotspot Score (0-1)** based on resource usage relative to tier quotas:

```
HotspotScore = 0.25 × (DatabaseSize/Quota)
             + 0.25 × (RequestRate/Quota)
             + 0.30 × (CPUUsage/Quota)
             + 0.20 × (MemoryUsage/Quota)
```

**Score Interpretation:**
- **< 0.3**: Normal usage
- **0.3 - 0.7**: Elevated usage (monitor)
- **> 0.7**: Hotspot detected (take action)

#### Spike Detection

Sudden traffic increases are detected by comparing current traffic to historical patterns:

```
IsSpiking = (RequestsLast24h / AvgRequestsLast7d) > 3.0
```

When a spike is detected, the system:
1. Logs the spike event with timestamp
2. Temporarily increases CPU quota
3. Monitors for sustained high traffic (tier upgrade)
4. Tracks spike frequency for pattern analysis

### Resource Isolation Mechanisms

#### 1. Weighted LRU Cache

Unlike traditional LRU caches where each tenant uses one slot, PocketBase uses **weighted slots** based on tier:

| Tier | Weight | Effective Slots |
|------|--------|-----------------|
| Micro | 1 | 1 |
| Small | 2 | 2 |
| Medium | 5 | 5 |
| Large | 10 | 10 |
| Enterprise | 20 | 20 |

**Example**: A node with capacity 1000 can hold:
- 1000 micro tenants, OR
- 500 small tenants, OR
- 100 large tenants, OR
- Mix: 800 micro + 50 small + 10 large = (800×1 + 50×2 + 10×10) = 1000

This prevents large tenants from consuming disproportionate resources while appearing as a single entry.

#### 2. Automatic Eviction

When a hotspot tenant exceeds quotas significantly (2x over limit), it's automatically evicted:

```go
func (rm *ResourceManager) ShouldEvict(tenantID string) (bool, string) {
    metrics := rm.metrics[tenantID]
    quota := rm.quotas[metrics.Tier]

    // Evict if significantly over CPU quota
    if metrics.CPUUsagePercent > quota.MaxCPUPercent*2 {
        return true, "CPU usage exceeds 2x tier limit"
    }

    // Evict if database is much larger than tier allows
    if metrics.DatabaseSizeMB > quota.MaxDatabaseMB*2 {
        return true, "Database size exceeds 2x tier limit"
    }

    return false, ""
}
```

**Eviction protects other tenants** by freeing resources, forcing the large tenant to reload (and potentially be assigned to a different node or upgraded to higher tier).

#### 3. Per-Tenant Monitoring

Every tenant access triggers metric collection:

```go
// After each request
m.recordTenantMetrics(tenantID, instance)

// Metrics include
- Database size
- Query execution time
- Memory footprint
- CPU usage
- Request rate
```

The ResourceManager runs background loops (every 30 seconds) to:
- Detect new hotspots
- Check quota violations
- Classify tenant behavior
- Trigger callbacks for action

### Handling Specific Hotspot Scenarios

#### Scenario 1: Large Database (5 GB+)

**Detection:**
```
DatabaseSizeMB > 5000 → Classify as TenantTierEnterprise
```

**Actions:**
1. **Automatic tier upgrade** to Enterprise
2. **Notify control plane** to mark for dedicated node placement
3. **Increased quotas**: 16 GB RAM, 100% CPU, 50 GB database limit
4. **Priority scheduling**: Higher priority in placement decisions

**Cost Impact:**
- Standard tenant: $0.0027/month
- Enterprise tenant: $40-80/month (dedicated node)
- **Fair pricing** for resource-intensive customers

#### Scenario 2: High Traffic Spike (10x normal)

**Detection:**
```
RequestsLast24h / AvgRequestsLast7d > 3.0
IsSpiking = true
```

**Actions:**
1. **Temporary quota increase**: 2x CPU, 2x memory for 1 hour
2. **Monitor sustained traffic**: If continues > 6 hours, upgrade tier
3. **Log spike event** for pattern analysis
4. **Alert admin** if spike is unusual (possible attack)

**No Eviction:** Spikes are expected (e.g., product launches), so tenant remains loaded with temporary increased quotas.

#### Scenario 3: Heavy CPU Usage (Complex Queries)

**Detection:**
```
CPUUsagePercent > quota.MaxCPUPercent
AvgQueryTimeMs > 5000ms
```

**Actions:**
1. **Query logging** to identify slow queries
2. **Automatic index suggestions** (if applicable)
3. **Tier upgrade** if sustained high CPU usage
4. **Eviction** if CPU usage > 2x quota (protects other tenants)

**Customer notification:**
- Email alert about query performance
- Suggestion to optimize or upgrade tier
- Link to query analysis dashboard

#### Scenario 4: Noisy Neighbor Protection

**Problem:** One tenant using 80% of node CPU, impacting 99 other tenants.

**Solution:**
```go
// In LoadTenant, check weighted capacity
used, total := m.getWeightedCapacity()
if used >= total {
    m.evictLRULocked() // Evict least recently used
}

// Hotspot callback
if shouldEvict, reason := m.resourceMgr.ShouldEvict(tenantID); shouldEvict {
    m.UnloadTenant(ctx, tenantID) // Force eviction
}
```

**Result:**
- Large tenant is unloaded
- Frees resources for other tenants
- Large tenant reloads on next access (potentially on different node)
- If pattern continues, automatic tier upgrade to dedicated node

### Node Pool Architecture (Future Enhancement)

For optimal isolation, PocketBase Enterprise can use **tiered node pools**:

```
┌─────────────────────────────────────────┐
│           Gateway / Load Balancer        │
└─────────────────────────────────────────┘
                     │
        ┌────────────┼────────────┐
        │            │            │
  ┌─────▼────┐ ┌────▼─────┐ ┌────▼─────────┐
  │ Micro/   │ │ Medium   │ │ Enterprise   │
  │ Small    │ │ Nodes    │ │ Dedicated    │
  │ Pool     │ │ Pool     │ │ Nodes        │
  │          │ │          │ │              │
  │ 40 nodes │ │ 8 nodes  │ │ 2 nodes      │
  │ 200/node │ │ 50/node  │ │ 1/node       │
  └──────────┘ └──────────┘ └──────────────┘
```

**Benefits:**
- **Complete isolation** for enterprise tenants
- **Optimized node sizing** per tier
- **Better resource utilization**
- **Predictable performance**

**Placement Decision:**
```go
func (ps *PlacementService) AssignTenant(tenantID string) (*PlacementDecision, error) {
    // Get tenant metrics
    metrics := ps.resourceMgr.GetMetrics(tenantID)

    // Assign to appropriate node pool
    switch metrics.Tier {
    case TenantTierEnterprise:
        return ps.assignToDedicatedNode(tenantID)
    case TenantTierLarge, TenantTierMedium:
        return ps.assignToMediumPool(tenantID)
    default:
        return ps.assignToStandardPool(tenantID)
    }
}
```

### Resource Manager API

#### Monitoring Endpoints

**Get Hotspot Tenants:**
```bash
GET /api/enterprise/admin/resource/hotspots
```

Response:
```json
{
  "hotspots": [
    {
      "tenantId": "tenant_789",
      "tier": "large",
      "hotspotScore": 0.87,
      "metrics": {
        "databaseSizeMB": 4500,
        "cpuUsagePercent": 65.3,
        "requestsLast24h": 850000
      },
      "recommendation": "upgrade to enterprise tier"
    }
  ],
  "totalHotspots": 12,
  "criticalHotspots": 3
}
```

**Get Tenant Metrics:**
```bash
GET /api/enterprise/admin/resource/metrics?tenantId=tenant_123
```

**Update Tenant Quotas:**
```bash
PUT /api/enterprise/admin/resource/quota
{
  "tenantId": "tenant_123",
  "tier": "enterprise",
  "customQuotas": {
    "maxDatabaseMB": 100000,
    "maxRequestsDaily": 50000000
  }
}
```

### Real-World Example Scenarios

#### Example 1: SaaS Product Launch

**Situation:** Tenant normally has 1k requests/day. Product launch day: 500k requests/day.

**System Response:**
1. **00:00**: Spike detected (500x normal traffic)
2. **00:01**: IsSpiking=true, temporary 5x quotas granted
3. **00:05**: Tier upgraded from Small → Large
4. **06:00**: Sustained high traffic confirmed, tier upgrade permanent
5. **Day 7**: Traffic normalizes to 50k/day (still 50x original)
6. **Outcome**: Smooth handling, no downtime, appropriate tier

**Cost**: $0.0027 → $0.20/month (still 125x cheaper than Supabase)

#### Example 2: Analytics Dashboard with Large Dataset

**Situation:** Tenant has 10 GB database with daily aggregation queries.

**System Response:**
1. **Week 1**: Database grows from 500 MB → 5 GB
2. **Week 2**: Tier upgraded from Small → Large automatically
3. **Week 3**: Database reaches 10 GB, queries slow
4. **Week 4**: Tier upgraded to Enterprise, assigned dedicated node
5. **Result**: Query performance improves, no impact on other tenants

**Cost**: $0.0027 → $80/month (dedicated 8vCPU/16GB node)

#### Example 3: Noisy Neighbor Scenario

**Situation:** One tenant running heavy analytics at midnight, consuming 90% CPU.

**System Response:**
1. **00:00**: ResourceManager detects CPUUsagePercent=90%
2. **00:01**: Hotspot score = 0.95 (critical)
3. **00:02**: ShouldEvict returns true (3x over CPU quota)
4. **00:03**: Tenant evicted, other tenants unaffected
5. **00:10**: Large tenant reloads on different node
6. **Next day**: Automatic tier upgrade to Enterprise

**Outcome**:
- Other 99 tenants unaffected
- Large tenant experiences one 10-second reload
- Future runs on dedicated node

### Configuration

#### Enabling Resource Management

```go
// In tenant node initialization
mgr.resourceMgr = enterprise.NewResourceManager()
mgr.setupResourceCallbacks()
mgr.resourceMgr.Start()
```

#### Custom Tier Quotas

```go
// Override default quotas
customQuotas := map[enterprise.TenantTier]*enterprise.ResourceQuota{
    enterprise.TenantTierLarge: {
        MaxDatabaseMB:      10_000,  // 10 GB
        MaxRequestsDaily:   5_000_000,
        MaxMemoryMB:        8_000,
        MaxCPUPercent:      75.0,
    },
}

resourceMgr.SetCustomQuotas(customQuotas)
```

#### Hotspot Callbacks

```go
resourceMgr.SetCallbacks(
    // On hotspot detected
    func(tenantID string, metrics *enterprise.TenantResourceMetrics) {
        logger.Printf("Hotspot: %s (score: %.2f)", tenantID, metrics.HotspotScore)

        // Custom actions
        if metrics.Tier == enterprise.TenantTierEnterprise {
            notifyAdmin("Large tenant needs dedicated node: " + tenantID)
        }
    },

    // On tier upgrade
    func(tenantID string, oldTier, newTier enterprise.TenantTier) {
        logger.Printf("Tier upgrade: %s (%s → %s)", tenantID, oldTier, newTier)

        // Update billing
        updateCustomerBilling(tenantID, newTier)
    },

    // On quota exceeded
    func(tenantID string, quotaType string, current, limit int64) {
        logger.Printf("Quota exceeded: %s %s (%d > %d)", tenantID, quotaType, current, limit)

        // Notify customer
        sendQuotaAlert(tenantID, quotaType, current, limit)
    },
)
```

### Monitoring and Alerts

#### Prometheus Metrics

```
# Hotspot tracking
enterprise_hotspot_tenants 12
enterprise_hotspot_score{tenant_id="tenant_789"} 0.87

# Tier distribution
enterprise_tenants_by_tier{tier="micro"} 850000
enterprise_tenants_by_tier{tier="small"} 140000
enterprise_tenants_by_tier{tier="medium"} 8500
enterprise_tenants_by_tier{tier="large"} 1200
enterprise_tenants_by_tier{tier="enterprise"} 300

# Resource violations
enterprise_quota_violations_total{type="cpu"} 45
enterprise_quota_violations_total{type="database_size"} 23

# Evictions
enterprise_tenant_evictions_total{reason="cpu_exceeded"} 12
enterprise_tenant_evictions_total{reason="database_size"} 8
```

#### Alert Rules

```yaml
groups:
  - name: hotspot_alerts
    rules:
      # Critical: Too many hotspots
      - alert: HighHotspotCount
        expr: enterprise_hotspot_tenants > 50
        for: 10m
        annotations:
          summary: "High number of hotspot tenants detected"

      # Warning: Large tenant on shared node
      - alert: EnterpriseTenantOnSharedNode
        expr: enterprise_tenant_on_shared_node{tier="enterprise"} > 0
        for: 5m
        annotations:
          summary: "Enterprise tenant should be on dedicated node"

      # Critical: Frequent evictions
      - alert: FrequentEvictions
        expr: rate(enterprise_tenant_evictions_total[5m]) > 1
        for: 10m
        annotations:
          summary: "High eviction rate indicates resource pressure"
```

### Cost Impact of Large Tenants

#### Pricing Strategy

| Tier | Recommended Pricing | Cost to Serve | Margin |
|------|---------------------|---------------|--------|
| **Micro** | $0/month (free) | $0.0027 | Loss leader |
| **Small** | $5/month | $0.0027 | 99.9% |
| **Medium** | $25/month | $0.20 | 99.2% |
| **Large** | $100/month | $5 | 95% |
| **Enterprise** | $500/month | $80 | 84% |

**Key Insight:** Even enterprise tenants with dedicated nodes cost only $80/month to serve, allowing healthy margins while being 50x cheaper than competitors.

#### ROI Analysis

**Scenario: 1M tenants with realistic distribution**

```
Micro (850k):     850k × $0 = $0
Small (140k):     140k × $5 = $700k
Medium (8.5k):    8.5k × $25 = $212k
Large (1.2k):     1.2k × $100 = $120k
Enterprise (300): 300 × $500 = $150k

Total Revenue: $1,182,000/month
Total Cost:    $2,739/month (infrastructure)
Gross Profit:  $1,179,261/month (99.8% margin)
```

**At scale, infrastructure becomes negligible** compared to revenue, enabling aggressive free tier and competitive pricing.

---

This comprehensive resource management system ensures PocketBase Enterprise can handle any tenant workload, from tiny hobby projects to enterprise applications with 50 GB+ databases and millions of daily requests, all while maintaining fair resource allocation and protecting against noisy neighbor problems.
