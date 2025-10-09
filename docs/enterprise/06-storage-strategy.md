# Storage Strategy: S3 + Litestream

## Core Principle

**S3 is the single source of truth.** All tenant data ultimately lives in S3. Tenant nodes are ephemeral caches that can be destroyed and recreated at any time without data loss.

---

## Architecture

```
┌────────────────────────────────────────────────────────────┐
│                     Tenant Node                            │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐ │
│  │  Tenant Instance (tenant_abc123)                     │ │
│  │                                                       │ │
│  │  ┌──────────────┐         ┌──────────────┐          │ │
│  │  │   data.db    │         │ auxiliary.db │          │ │
│  │  │  (SQLite)    │         │   (SQLite)   │          │ │
│  │  └──────┬───────┘         └──────┬───────┘          │ │
│  │         │                        │                   │ │
│  │         │  WAL Write             │  WAL Write        │ │
│  │         ▼                        ▼                   │ │
│  │  ┌──────────────────────────────────────────────┐   │ │
│  │  │   Litestream (Embedded Process)              │   │ │
│  │  │   - Watches WAL files                        │   │ │
│  │  │   - Streams changes to S3 continuously       │   │ │
│  │  │   - Sub-second replication lag               │   │ │
│  │  └──────────────┬───────────────────────────────┘   │ │
│  └─────────────────┼───────────────────────────────────┘ │
└────────────────────┼─────────────────────────────────────┘
                     │
                     │ Continuous Replication
                     │ (WAL Segments)
                     ▼
┌────────────────────────────────────────────────────────────┐
│                    S3 Bucket                               │
│                                                             │
│  s3://bucket/tenants/tenant_abc123/                        │
│  ├── litestream/                                            │
│  │   ├── data.db/                                          │
│  │   │   ├── generations/                                  │
│  │   │   │   ├── 00000001/                                │
│  │   │   │   │   ├── snapshot.db (periodic full snapshot) │
│  │   │   │   │   ├── wal/000001.wal                       │
│  │   │   │   │   ├── wal/000002.wal                       │
│  │   │   │   │   └── wal/...                              │
│  │   │   │   └── 00000002/                                │
│  │   │   └── manifest.json                                │
│  │   └── auxiliary.db/                                     │
│  │       └── (same structure)                              │
│  ├── hooks/                                                 │
│  │   ├── main.pb.js                                        │
│  │   └── routes.pb.js                                      │
│  └── metadata.json                                          │
└────────────────────────────────────────────────────────────┘
```

---

## Litestream Integration

### Embedded Litestream

Rather than spawning external processes, we embed Litestream as a Go library.

```go
// core/storage/litestream.go

import (
    "github.com/benbjohnson/litestream"
)

type LitestreamManager struct {
    replications map[string]*litestream.DB
    mu           sync.RWMutex
}

// StartReplication starts continuous replication for a tenant database
func (lm *LitestreamManager) StartReplication(
    tenantID string,
    dbPath string,
    s3Bucket string,
    s3Path string,
) error {
    // Create Litestream DB instance
    db := litestream.NewDB(dbPath)

    // Configure S3 replica
    replica := litestream.NewReplica(db, "s3")
    replica.URL = fmt.Sprintf("s3://%s/%s", s3Bucket, s3Path)

    // Sync interval (how often to check for WAL changes)
    replica.SyncInterval = 1 * time.Second

    // Snapshot interval (full database snapshot)
    replica.SnapshotInterval = 24 * time.Hour

    // Retention (keep snapshots for 7 days)
    replica.Retention = 7 * 24 * time.Hour

    // Add replica to DB
    db.Replicas = append(db.Replicas, replica)

    // Start replication
    if err := db.Open(); err != nil {
        return fmt.Errorf("failed to open litestream db: %w", err)
    }

    // Store reference
    lm.mu.Lock()
    lm.replications[tenantID] = db
    lm.mu.Unlock()

    return nil
}

// StopReplication gracefully stops replication (ensures all data synced)
func (lm *LitestreamManager) StopReplication(tenantID string) error {
    lm.mu.Lock()
    db, exists := lm.replications[tenantID]
    if !exists {
        lm.mu.Unlock()
        return nil
    }
    delete(lm.replications, tenantID)
    lm.mu.Unlock()

    // Close DB (blocks until all WAL segments synced)
    return db.Close()
}

// RestoreFromS3 restores a database from S3
func (lm *LitestreamManager) RestoreFromS3(
    s3Bucket string,
    s3Path string,
    localPath string,
    timestamp time.Time, // point-in-time restore
) error {
    db := litestream.NewDB(localPath)

    replica := litestream.NewReplica(db, "s3")
    replica.URL = fmt.Sprintf("s3://%s/%s", s3Bucket, s3Path)

    // Restore to specific timestamp (or latest if zero)
    if err := db.Restore(context.Background(), replica, timestamp); err != nil {
        return fmt.Errorf("failed to restore from s3: %w", err)
    }

    return nil
}
```

---

## Tenant Data Layout

### S3 Structure

```
s3://pocketbase-enterprise/
├── tenants/
│   ├── tenant_001/
│   │   ├── litestream/
│   │   │   ├── data.db/
│   │   │   │   ├── generations/
│   │   │   │   │   └── 00000001/
│   │   │   │   │       ├── snapshot.db        # Full DB snapshot
│   │   │   │   │       └── wal/
│   │   │   │   │           ├── 000001.wal     # WAL segments
│   │   │   │   │           ├── 000002.wal
│   │   │   │   │           └── ...
│   │   │   │   └── manifest.json
│   │   │   └── auxiliary.db/
│   │   │       └── (same structure)
│   │   ├── hooks/
│   │   │   ├── main.pb.js
│   │   │   └── routes.pb.js
│   │   ├── storage/                           # Uploaded files
│   │   │   └── (tenant file uploads)
│   │   └── metadata.json                      # Tenant metadata
│   ├── tenant_002/
│   └── ...
├── control-plane/
│   └── badger-snapshots/
│       └── snapshot-{term}-{index}.snap
└── backups/                                    # Manual backups
    └── tenant_001/
        └── backup-2025-01-15.zip
```

### Local Node Structure

```
/data/
├── control-plane/           # (Control plane nodes only)
│   ├── raft/
│   └── badger/
├── tenants/                 # (Tenant nodes only)
│   ├── tenant_001/
│   │   ├── data.db
│   │   ├── data.db-wal
│   │   ├── data.db-shm
│   │   ├── auxiliary.db
│   │   ├── auxiliary.db-wal
│   │   ├── auxiliary.db-shm
│   │   ├── pb_hooks/
│   │   └── pb_storage/
│   ├── tenant_002/
│   └── ...
└── cache/                   # Metadata cache
    └── route_cache.db
```

---

## Tenant Lifecycle Operations

### 1. Tenant Creation

```go
// When a new tenant is created

func (tn *TenantNode) CreateTenant(tenantID, domain string) error {
    // 1. Create local directory
    tenantDir := filepath.Join(tn.config.DataDir, "tenants", tenantID)
    os.MkdirAll(tenantDir, 0755)

    // 2. Initialize empty PocketBase databases
    dataDB := filepath.Join(tenantDir, "data.db")
    auxDB := filepath.Join(tenantDir, "auxiliary.db")

    app := core.NewBaseApp(core.BaseAppConfig{
        DataDir: tenantDir,
    })

    if err := app.Bootstrap(); err != nil {
        return err
    }

    // 3. Start Litestream replication
    s3Path := fmt.Sprintf("tenants/%s/litestream/data.db", tenantID)
    err := tn.litestream.StartReplication(
        tenantID,
        dataDB,
        tn.config.S3Bucket,
        s3Path,
    )
    if err != nil {
        return err
    }

    // Auxiliary DB too
    s3PathAux := fmt.Sprintf("tenants/%s/litestream/auxiliary.db", tenantID)
    err = tn.litestream.StartReplication(
        tenantID+"-aux",
        auxDB,
        tn.config.S3Bucket,
        s3PathAux,
    )

    // 4. Upload default hooks to S3 (if any)
    // ...

    return nil
}
```

### 2. Tenant Loading (from S3)

```go
// When a tenant needs to be loaded on a node

func (tn *TenantNode) LoadTenant(tenantID string) (*TenantInstance, error) {
    // 1. Check if already in cache
    if inst, ok := tn.cache.Get(tenantID); ok {
        inst.LastAccess = time.Now()
        return inst, nil
    }

    tenantDir := filepath.Join(tn.config.DataDir, "tenants", tenantID)

    // 2. Check if exists locally
    dataDB := filepath.Join(tenantDir, "data.db")
    exists, _ := fileExists(dataDB)

    if !exists {
        // 3. Restore from S3 using Litestream
        s3Path := fmt.Sprintf("tenants/%s/litestream/data.db", tenantID)

        os.MkdirAll(tenantDir, 0755)

        err := tn.litestream.RestoreFromS3(
            tn.config.S3Bucket,
            s3Path,
            dataDB,
            time.Time{}, // latest
        )
        if err != nil {
            return nil, fmt.Errorf("failed to restore data.db: %w", err)
        }

        // Restore auxiliary.db
        s3PathAux := fmt.Sprintf("tenants/%s/litestream/auxiliary.db", tenantID)
        auxDB := filepath.Join(tenantDir, "auxiliary.db")

        err = tn.litestream.RestoreFromS3(
            tn.config.S3Bucket,
            s3PathAux,
            auxDB,
            time.Time{},
        )
        if err != nil {
            return nil, fmt.Errorf("failed to restore auxiliary.db: %w", err)
        }

        // Download hooks from S3
        err = tn.downloadHooks(tenantID, tenantDir)
        if err != nil {
            return nil, err
        }
    }

    // 4. Bootstrap PocketBase instance
    app := core.NewBaseApp(core.BaseAppConfig{
        DataDir: tenantDir,
    })

    if err := app.Bootstrap(); err != nil {
        return nil, err
    }

    // 5. Start Litestream replication
    s3Path := fmt.Sprintf("tenants/%s/litestream/data.db", tenantID)
    tn.litestream.StartReplication(tenantID, dataDB, tn.config.S3Bucket, s3Path)

    s3PathAux := fmt.Sprintf("tenants/%s/litestream/auxiliary.db", tenantID)
    auxDB := filepath.Join(tenantDir, "auxiliary.db")
    tn.litestream.StartReplication(tenantID+"-aux", auxDB, tn.config.S3Bucket, s3PathAux)

    // 6. Create tenant instance
    instance := &TenantInstance{
        ID:         tenantID,
        App:        app,
        DataDir:    tenantDir,
        Status:     "active",
        LoadedAt:   time.Now(),
        LastAccess: time.Now(),
    }

    // 7. Add to cache
    tn.cache.Add(tenantID, instance)

    return instance, nil
}
```

### 3. Tenant Eviction (cache cleanup)

```go
// When cache is full and tenant needs to be evicted

func (tn *TenantNode) EvictTenant(tenantID string) error {
    inst, ok := tn.cache.Get(tenantID)
    if !ok {
        return nil // Already evicted
    }

    // 1. Mark as evicting (stop accepting new requests)
    inst.Status = "evicting"

    // 2. Wait for in-flight requests
    tn.drainRequests(inst, 30*time.Second)

    // 3. Stop Litestream (ensures all WAL segments synced to S3)
    tn.litestream.StopReplication(tenantID)
    tn.litestream.StopReplication(tenantID + "-aux")

    // 4. Close PocketBase app
    inst.App.ResetBootstrapState()

    // 5. Delete local files (optional - can keep for faster reload)
    if tn.config.DeleteOnEvict {
        os.RemoveAll(inst.DataDir)
    }

    // 6. Remove from cache
    tn.cache.Remove(tenantID)

    return nil
}
```

### 4. Tenant Migration (to another node)

```go
// Migration is seamless because S3 is source of truth

func (tn *TenantNode) MigrateTenant(tenantID, targetNodeID string) error {
    // Old node (current):
    // 1. Stop accepting new requests
    // 2. Wait for in-flight requests
    // 3. Stop Litestream (sync all data to S3)
    // 4. Evict from cache

    // New node (target):
    // 1. Load tenant from S3 (standard LoadTenant)
    // 2. Start serving requests

    // Control plane:
    // 1. Update routing table
    // 2. Broadcast route change to gateways

    // No downtime: Gateways route new requests to new node
    // S3 ensures data consistency
}
```

---

## Point-in-Time Recovery

Litestream enables restoring to any point in time:

```go
// Restore tenant to specific timestamp

func (tn *TenantNode) RestoreTenantToTimestamp(
    tenantID string,
    timestamp time.Time,
) error {
    tenantDir := filepath.Join(tn.config.DataDir, "tenants", tenantID)
    dataDB := filepath.Join(tenantDir, "data.db")

    // Stop current replication
    tn.litestream.StopReplication(tenantID)

    // Restore to timestamp
    s3Path := fmt.Sprintf("tenants/%s/litestream/data.db", tenantID)
    err := tn.litestream.RestoreFromS3(
        tn.config.S3Bucket,
        s3Path,
        dataDB,
        timestamp, // Specific point in time
    )

    if err != nil {
        return err
    }

    // Restart replication from this point
    tn.litestream.StartReplication(tenantID, dataDB, tn.config.S3Bucket, s3Path)

    return nil
}
```

---

## Disaster Recovery

### Scenario 1: Node Failure

```
1. Node crashes (hardware failure)
2. Control plane detects missing heartbeat
3. Marks node as "down"
4. Gateways query control plane for affected tenants
5. Control plane assigns tenants to new nodes
6. New nodes load tenants from S3
7. Zero data loss (all data in S3)
8. Downtime: ~2-5 seconds (time to restore + bootstrap)
```

### Scenario 2: S3 Outage

```
1. S3 becomes unavailable
2. Litestream replication queues WAL segments locally
3. Existing loaded tenants continue serving (no impact)
4. New tenant loads fail (503 error)
5. When S3 recovers:
   - Litestream syncs buffered WAL segments
   - New tenant loads resume
```

### Scenario 3: Region Failure

```
1. Entire AWS region goes down
2. Multi-region S3 replication ensures data available in other region
3. Spin up new cluster in backup region
4. Load tenants from S3 (in backup region)
5. Update DNS to point to new region
6. Service restored
```

---

## Storage Costs

### S3 Storage Classes

```
Active tenants:     S3 Standard (frequent access)
Idle tenants:       S3 Standard-IA (infrequent access)
Archived tenants:   S3 Glacier Deep Archive (long-term)
```

### Cost Estimation (100,000 tenants)

```
Assumptions:
- Average tenant DB size: 500MB
- Total data: 50TB
- 20% active (frequent access): 10TB @ S3 Standard
- 60% idle (infrequent): 30TB @ S3 Standard-IA
- 20% archived: 10TB @ S3 Glacier Deep Archive

Monthly costs:
- S3 Standard (10TB):          $230/month
- S3 Standard-IA (30TB):       $375/month
- S3 Glacier Deep (10TB):      $10/month
- Litestream operations:       ~$50/month
Total:                         ~$665/month

Per tenant: $0.00665/month storage cost
```

Much cheaper than competitors!

---

## Next: Implementation Phases

See [11-implementation-phases.md](11-implementation-phases.md) for development roadmap.
