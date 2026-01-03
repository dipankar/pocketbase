# Hooks Database: Database-Backed Hooks

## Overview

Instead of file-based hooks (`pb_hooks/*.pb.js`), PocketBase Enterprise stores hooks in a **SQLite database** (`hooks.db`) for each tenant.

**Benefits**:

- No file system dependencies
- Hooks included in tenant backup/restore automatically (via Litestream)
- Manage hooks via admin UI (no deployment process)
- Version control built-in (hook versioning)
- Execution history and debugging
- Migration-friendly (hooks move with tenant)

---

## Database Schema

### hooks.db Structure

Each tenant has a `hooks.db` alongside `data.db` and `auxiliary.db`:

```sql
CREATE TABLE hooks (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL UNIQUE,
  type        TEXT NOT NULL,    -- 'record.create', 'record.update', 'route', etc.
  collection  TEXT,              -- NULL for routes, collection name for record hooks
  event       TEXT,              -- 'before', 'after'
  code        TEXT NOT NULL,     -- JavaScript source
  enabled     BOOLEAN DEFAULT 1,
  priority    INTEGER DEFAULT 0, -- Execution order (higher = first)

  -- Metadata
  description TEXT,
  version     INTEGER DEFAULT 1,

  -- Execution stats
  last_executed    DATETIME,
  execution_count  INTEGER DEFAULT 0,
  error_count      INTEGER DEFAULT 0,
  last_error       TEXT,

  -- Audit
  created     DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated     DATETIME DEFAULT CURRENT_TIMESTAMP,
  created_by  TEXT,              -- user_id or admin_id
  updated_by  TEXT
);

CREATE INDEX idx_hooks_type ON hooks(type);
CREATE INDEX idx_hooks_collection ON hooks(collection);
CREATE INDEX idx_hooks_enabled ON hooks(enabled);
CREATE INDEX idx_hooks_priority ON hooks(priority DESC);

-- Hook execution history (for debugging)
CREATE TABLE hook_executions (
  id           TEXT PRIMARY KEY,
  hook_id      TEXT NOT NULL,
  tenant_id    TEXT NOT NULL,
  started_at   DATETIME,
  completed_at DATETIME,
  duration_ms  INTEGER,
  success      BOOLEAN,
  error        TEXT,
  context      TEXT,            -- JSON: {record_id, user_id, etc.}

  FOREIGN KEY (hook_id) REFERENCES hooks(id) ON DELETE CASCADE
);

CREATE INDEX idx_executions_hook ON hook_executions(hook_id);
CREATE INDEX idx_executions_started ON hook_executions(started_at);
CREATE INDEX idx_executions_success ON hook_executions(success);
```

---

## Hook Types

### Record Hooks

```sql
-- Example: Validate post title before creation
INSERT INTO hooks (id, name, type, collection, event, code, enabled) VALUES (
  'hook_001',
  'Validate Post Title',
  'record.create',
  'posts',
  'before',
  'onRecordCreate("posts", (e) => {
    if (e.record.get("title").length < 5) {
      throw new BadRequestError("Title too short")
    }
  })',
  1
);

-- Example: Send email after user registration
INSERT INTO hooks (id, name, type, collection, event, code, enabled) VALUES (
  'hook_002',
  'Send Welcome Email',
  'record.create',
  'users',
  'after',
  'onRecordCreate("users", async (e) => {
    await $email.send({
      to: e.record.get("email"),
      subject: "Welcome!",
      body: "Thanks for signing up"
    })
  })',
  1
);
```

**Supported Record Hook Types**:

- `record.create` (before/after)
- `record.update` (before/after)
- `record.delete` (before/after)

### Route Hooks

```sql
-- Example: Custom API endpoint
INSERT INTO hooks (id, name, type, event, code, enabled) VALUES (
  'hook_003',
  'Custom Publish Endpoint',
  'route',
  'after', -- For routes, this is unused but required
  '$app.router.POST("/api/custom/publish", async (e) => {
    const postId = e.request.body.postId

    // Load record
    const post = await $app.findRecordById("posts", postId)

    // Business logic
    post.set("published", true)
    post.set("publishedAt", new Date())

    await $app.save(post)

    return e.json({ success: true })
  }, {
    auth: "required",
    roles: ["admin", "editor"]
  })',
  1
);
```

---

## Hook Loading (Tenant Bootstrap)

When a tenant is loaded, hooks from `hooks.db` are dynamically registered:

```go
func (tn *TenantNode) LoadTenant(tenantID string) (*TenantInstance, error) {
    tenantDir := filepath.Join(tn.config.DataDir, "tenants", tenantID)

    // 1. Restore databases from S3 (including hooks.db)
    databases := []string{"data.db", "auxiliary.db", "hooks.db"}
    for _, db := range databases {
        dbPath := filepath.Join(tenantDir, db)
        if !exists(dbPath) {
            s3Path := fmt.Sprintf("tenants/%s/litestream/%s", tenantID, db)
            err := tn.litestream.RestoreFromS3(tn.config.S3Bucket, s3Path, dbPath, time.Time{})

            // hooks.db might not exist for new tenants
            if err != nil && db == "hooks.db" {
                tn.createEmptyHooksDB(dbPath)
            }
        }
    }

    // 2. Bootstrap PocketBase instance
    app := core.NewBaseApp(core.BaseAppConfig{
        DataDir: tenantDir,
    })

    if err := app.Bootstrap(); err != nil {
        return nil, err
    }

    // 3. Load hooks from hooks.db
    if err := tn.loadHooksFromDB(app, tenantID); err != nil {
        return nil, err
    }

    // 4. Start Litestream replication (including hooks.db)
    for _, db := range databases {
        dbPath := filepath.Join(tenantDir, db)
        s3Path := fmt.Sprintf("tenants/%s/litestream/%s", tenantID, db)
        tn.litestream.StartReplication(
            fmt.Sprintf("%s-%s", tenantID, db),
            dbPath,
            tn.config.S3Bucket,
            s3Path,
        )
    }

    return instance, nil
}

func (tn *TenantNode) loadHooksFromDB(app *core.BaseApp, tenantID string) error {
    // Open hooks.db
    hooksDBPath := filepath.Join(app.DataDir(), "hooks.db")
    hooksDB, err := sql.Open("sqlite", hooksDBPath)
    if err != nil {
        return err
    }
    defer hooksDB.Close()

    // Query enabled hooks, ordered by priority
    rows, err := hooksDB.Query(`
        SELECT id, name, type, collection, event, code, priority
        FROM hooks
        WHERE enabled = 1
        ORDER BY priority DESC
    `)
    if err != nil {
        return err
    }
    defer rows.Close()

    // Create JSVM instance for this tenant
    jsvm := goja.New()

    // Bind PocketBase APIs to JSVM
    tn.bindPocketBaseAPIs(jsvm, app, tenantID)

    // Load each hook
    for rows.Next() {
        var hook HookRecord
        err := rows.Scan(&hook.ID, &hook.Name, &hook.Type, &hook.Collection, &hook.Event, &hook.Code, &hook.Priority)
        if err != nil {
            continue
        }

        // Register hook based on type
        switch hook.Type {
        case "record.create":
            tn.registerRecordHook(app, jsvm, hook, "create")
        case "record.update":
            tn.registerRecordHook(app, jsvm, hook, "update")
        case "record.delete":
            tn.registerRecordHook(app, jsvm, hook, "delete")
        case "route":
            tn.registerRouteHook(app, jsvm, hook)
        }
    }

    return nil
}
```

---

## Hook Execution

```go
func (tn *TenantNode) executeHookCode(
    hook HookRecord,
    jsvm *goja.Runtime,
    context map[string]interface{},
) error {
    startTime := time.Now()

    // Set event context
    jsvm.Set("e", context)

    // Execute hook code with timeout
    done := make(chan error, 1)
    go func() {
        _, err := jsvm.RunString(hook.Code)
        done <- err
    }()

    select {
    case err := <-done:
        duration := time.Since(startTime)

        // Update execution stats
        tn.updateHookStats(hook.ID, duration, err)

        // Log execution
        tn.logHookExecution(hook.ID, context, duration, err)

        return err

    case <-time.After(30 * time.Second):
        // Timeout
        err := errors.New("hook execution timeout (30s)")
        tn.updateHookStats(hook.ID, 30*time.Second, err)
        return err
    }
}

func (tn *TenantNode) updateHookStats(hookID string, duration time.Duration, err error) {
    hooksDB := tn.getHooksDB()

    if err == nil {
        // Success
        hooksDB.Exec(`
            UPDATE hooks
            SET execution_count = execution_count + 1,
                last_executed = CURRENT_TIMESTAMP
            WHERE id = ?
        `, hookID)
    } else {
        // Error
        hooksDB.Exec(`
            UPDATE hooks
            SET execution_count = execution_count + 1,
                error_count = error_count + 1,
                last_executed = CURRENT_TIMESTAMP,
                last_error = ?
            WHERE id = ?
        `, err.Error(), hookID)
    }
}
```

---

## Admin UI for Hooks

### List Hooks

```
+--------------------------------------------------------------+
|  Hooks                                         [+ New Hook]   |
+--------------------------------------------------------------+
|  Name                Type             Collection   Enabled    |
|  ------------------------------------------------------------|
|  Validate Post       record.create   posts        [x] [Edit]  |
|  Send Email          record.create   users        [x] [Edit]  |
|  Custom API          route           -            [x] [Edit]  |
|  Audit Update        record.update   *            [ ] [Edit]  |
+--------------------------------------------------------------+
```

### Create/Edit Hook

```
+--------------------------------------------------------------+
|  Hook: Validate Post                              [Save]      |
+--------------------------------------------------------------+
|  Name: [Validate Post_____________________________]           |
|  Description: [Ensure post title is at least 5 chars_____]    |
|                                                               |
|  Type: [record.create v]                                      |
|  Collection: [posts v]                                        |
|  Event: [before v]                                            |
|  Priority: [0___] (higher = execute first)                    |
|  Enabled: [x]                                                 |
|  -------------------------------------------------------------|
|  Code:                                                        |
|  +----------------------------------------------------------+ |
|  | onRecordCreate("posts", (e) => {                         | |
|  |   if (e.record.get("title").length < 5) {                | |
|  |     throw new BadRequestError("Title too short")         | |
|  |   }                                                      | |
|  | })                                                       | |
|  |                                                          | |
|  | // Available APIs:                                       | |
|  | // - e.record (current record)                           | |
|  | // - e.auth (authenticated user)                         | |
|  | // - $app (PocketBase instance)                          | |
|  | // - $http (HTTP client)                                 | |
|  | // - $email (Email sender)                               | |
|  +----------------------------------------------------------+ |
|                                                               |
|  Stats:                                                       |
|  - Executions: 1,234                                          |
|  - Errors: 3 (0.24%)                                          |
|  - Last executed: 2 minutes ago                               |
|  - Avg duration: 45ms                                         |
|  -------------------------------------------------------------|
|  [Test Hook]  [View Execution History]  [Delete]              |
+--------------------------------------------------------------+
```

### Test Hook

```
Click [Test Hook] -> Modal opens:

+--------------------------------------------------------------+
|  Test Hook: Validate Post                        [x Close]    |
+--------------------------------------------------------------+
|  Mock Event Data (JSON):                                      |
|  +----------------------------------------------------------+ |
|  | {                                                        | |
|  |   "record": {                                            | |
|  |     "title": "Test",                                     | |
|  |     "content": "Test post content"                       | |
|  |   },                                                     | |
|  |   "auth": null                                           | |
|  | }                                                        | |
|  +----------------------------------------------------------+ |
|                                                               |
|  [Run Test]                                                   |
|                                                               |
|  Result:                                                      |
|  Error: BadRequestError: Title too short                      |
|  Duration: 12ms                                               |
+--------------------------------------------------------------+
```

### Execution History

```
+--------------------------------------------------------------+
|  Hook Executions: Validate Post                              |
+--------------------------------------------------------------+
|  Time             Record           Duration   Result          |
|  ------------------------------------------------------------|
|  10:34:12 AM      post_abc123      45ms       Success        |
|  10:32:45 AM      post_def456      38ms       Success        |
|  10:28:19 AM      post_ghi789      52ms       Error          |
|    Error: Title too short                                    |
|  10:15:03 AM      post_jkl012      41ms       Success        |
|                                                               |
|  Showing last 50 executions                       [Load More] |
+--------------------------------------------------------------+
```

---

## Migration from File-Based Hooks

For users with existing `pb_hooks/*.pb.js` files:

```go
// One-time migration

func MigrateFileHooksToDB(tenantDir string) error {
    hooksDir := filepath.Join(tenantDir, "pb_hooks")
    hooksDBPath := filepath.Join(tenantDir, "hooks.db")

    // Create hooks.db if not exists
    if !exists(hooksDBPath) {
        createEmptyHooksDB(hooksDBPath)
    }

    db, _ := sql.Open("sqlite", hooksDBPath)
    defer db.Close()

    // Scan pb_hooks/ directory
    files, _ := filepath.Glob(filepath.Join(hooksDir, "*.pb.js"))

    for _, file := range files {
        code, _ := os.ReadFile(file)

        // Parse filename to determine hook type
        name := filepath.Base(file)
        hookType, collection := parseHookFilename(name)

        _, err := db.Exec(`
            INSERT INTO hooks (id, name, type, collection, code, enabled)
            VALUES (?, ?, ?, ?, ?, 1)
        `, generateID("hook_"), name, hookType, collection, string(code))

        if err != nil {
            log.Printf("Failed to migrate hook %s: %v", name, err)
        }
    }

    return nil
}
```

## Next Steps

- [GraphQL](graphql.md) - Auto-generated GraphQL API
- [Development Guide](development-guide.md) - Development workflows
