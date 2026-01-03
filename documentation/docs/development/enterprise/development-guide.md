# Enterprise Development Guide

This guide covers day-to-day development workflows for the PocketBase Enterprise platform.

## Quick Start

### Prerequisites

```bash
# Go 1.24 or later
go version

# Git
git --version

# Optional: Docker for local S3
docker --version
```

### Initial Setup

```bash
# Clone repository
git clone https://github.com/pocketbase/pocketbase.git
cd pocketbase

# Install dependencies
go mod download

# Build
go build

# Verify build
./pocketbase --version
```

## Development Workflows

### Running in All-In-One Mode (Development)

The easiest way to develop and test:

```bash
# Build
go build

# Run all-in-one mode (control plane + tenant node + gateway in one process)
./pocketbase serve --mode=all-in-one \
  --data-dir=./data \
  --s3-endpoint=http://localhost:4566 \
  --s3-bucket=pocketbase-dev

# Access:
# - Control Plane API: http://localhost:8090
# - Gateway: http://localhost:8080
# - Tenant Nodes: running internally
```

### Running Multi-Node Locally

For testing distributed features:

#### Terminal 1: Control Plane

```bash
./pocketbase serve --mode=control-plane \
  --node-id=cp-1 \
  --raft-bind-addr=127.0.0.1:7000 \
  --data-dir=./data/cp-1 \
  --http-addr=127.0.0.1:8090
```

#### Terminal 2: Tenant Node 1

```bash
./pocketbase serve --mode=tenant-node \
  --control-plane-addrs=127.0.0.1:8090 \
  --node-address=127.0.0.1:8091 \
  --max-tenants=50 \
  --data-dir=./data/tn-1 \
  --http-addr=127.0.0.1:8091
```

#### Terminal 3: Tenant Node 2

```bash
./pocketbase serve --mode=tenant-node \
  --control-plane-addrs=127.0.0.1:8090 \
  --node-address=127.0.0.1:8092 \
  --max-tenants=50 \
  --data-dir=./data/tn-2 \
  --http-addr=127.0.0.1:8092
```

#### Terminal 4: Gateway

```bash
./pocketbase serve --mode=gateway \
  --control-plane-addrs=127.0.0.1:8090 \
  --http-addr=127.0.0.1:8080
```

### Using LocalStack for S3

```bash
# Start LocalStack
docker run -d \
  --name pocketbase-s3 \
  -p 4566:4566 \
  -e SERVICES=s3 \
  -e DEBUG=1 \
  localstack/localstack

# Create bucket
aws --endpoint-url=http://localhost:4566 s3 mb s3://pocketbase-dev

# List buckets
aws --endpoint-url=http://localhost:4566 s3 ls
```

## Testing

### Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./core/enterprise/tenant_node/...

# With coverage
go test -cover ./core/enterprise/...

# Verbose output
go test -v ./core/enterprise/control_plane/...

# Run specific test
go test -v ./core/enterprise/tenant_node/ -run TestQuotaEnforcer
```

### Writing Tests

```go
package tenant_node_test

import (
    "testing"
    "github.com/pocketbase/pocketbase/core/enterprise/tenant_node"
)

func TestQuotaEnforcement(t *testing.T) {
    // Setup
    manager := setupTestManager(t)
    defer manager.Stop()

    // Test
    err := manager.CheckStorageQuota("tenant-123")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    // Verify
    // ...
}

func setupTestManager(t *testing.T) *tenant_node.Manager {
    // Helper to create test manager
    // ...
}
```

## Code Style

### File Organization

```go
package mypackage

// Imports - grouped and sorted
import (
    // Standard library
    "context"
    "fmt"
    "sync"

    // Third-party
    "github.com/hashicorp/raft"

    // Local
    "github.com/pocketbase/pocketbase/core/enterprise"
)

// Constants
const (
    DefaultTimeout = 10 * time.Second
    MaxRetries     = 3
)

// Types
type Manager struct {
    config *Config
    mu     sync.RWMutex
}

// Constructors
func NewManager(config *Config) *Manager {
    // ...
}

// Public methods
func (m *Manager) Start() error {
    // ...
}

// Private methods
func (m *Manager) privateHelper() {
    // ...
}
```

### Naming Conventions

```go
// Use descriptive names
tenantManager    // Good
mgr              // Bad

// Interfaces end with -er
StorageBackend   // Good
QuotaEnforcer    // Good
Storage          // Bad

// Avoid stuttering
manager.Start()                 // Good
tenantManager.StartTenantManager()  // Bad

// Use acronyms consistently
HTTPServer   // Good
JSONConfig   // Good
HttpServer   // Bad
JsonConfig   // Bad
```

### Error Handling

```go
// Always wrap errors with context
return err                                    // Bad
return fmt.Errorf("failed to load tenant: %w", err)  // Good

// Check errors immediately
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// Use named return values sparingly
func GetTenant(id string) (*Tenant, error)              // Good
func GetTenant(id string) (tenant *Tenant, err error)   // Bad
```

### Logging

```go
// Use structured logging with component prefix
logger.Printf("[TenantNode] Loading tenant: %s", tenantID)
logger.Printf("[ControlPlane] Raft leader elected: %s", nodeID)
logger.Printf("[Gateway] Request routed to node: %s", nodeAddr)

// Log levels (use appropriate methods)
logger.Printf("INFO: Operation completed")
logger.Printf("WARN: Retry attempt %d/%d", attempt, maxRetries)
logger.Printf("ERROR: Failed to connect: %v", err)
```

### Thread Safety

```go
// Always document thread-safety requirements
// Thread-safe for concurrent use
type Manager struct {
    tenants map[string]*Tenant
    mu      sync.RWMutex  // Protects tenants
}

// Use RWMutex for read-heavy workloads
func (m *Manager) GetTenant(id string) *Tenant {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.tenants[id]
}

func (m *Manager) SetTenant(id string, t *Tenant) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.tenants[id] = t
}
```

## Debugging

### Enable Verbose Logging

```bash
# Set environment variable
export PB_LOG_LEVEL=DEBUG

# Run with debug output
./pocketbase serve --mode=all-in-one --debug
```

### Common Issues

#### "Failed to connect to control plane"

```bash
# Check control plane is running
curl http://localhost:8090/api/health

# Check firewall rules
# Verify addresses in config
```

#### "Tenant not found"

```bash
# List tenants in control plane
curl http://localhost:8090/api/cp/tenants

# Check S3 bucket
aws --endpoint-url=http://localhost:4566 s3 ls s3://pocketbase-dev/
```

#### "Quota exceeded"

```bash
# Check tenant quotas
curl http://localhost:8090/api/cp/tenants/tenant-123

# Adjust quotas
curl -X PATCH http://localhost:8090/api/cp/tenants/tenant-123 \
  -d '{"storageQuotaMB": 1000, "apiRequestsQuota": 100000}'
```

## Performance Profiling

### CPU Profiling

```go
import _ "net/http/pprof"

// In main.go
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

```bash
# Collect profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Analyze
(pprof) top10
(pprof) list FunctionName
```

### Memory Profiling

```bash
# Collect heap profile
curl http://localhost:6060/debug/pprof/heap > heap.prof

# Analyze
go tool pprof heap.prof
(pprof) top10
(pprof) list FunctionName
```

### Benchmarking

```go
func BenchmarkTenantLoad(b *testing.B) {
    manager := setupTestManager()
    defer manager.Stop()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := manager.LoadTenant(context.Background(), "test-tenant")
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

```bash
# Run benchmarks
go test -bench=. ./core/enterprise/tenant_node/

# With memory profiling
go test -bench=. -benchmem ./core/enterprise/tenant_node/
```

## Git Workflow

### Branching Strategy

```bash
# Create feature branch
git checkout -b feature/quota-enforcement

# Make changes
git add .
git commit -m "feat(tenant-node): add storage quota enforcement"

# Push
git push origin feature/quota-enforcement
```

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(control-plane): implement raft snapshots
fix(tenant-node): resolve memory leak in cache
docs(enterprise): update architecture diagram
test(gateway): add circuit breaker tests
refactor(storage): simplify litestream integration
perf(metrics): optimize collector performance
```

### Before Committing

```bash
# Format code
go fmt ./...

# Run linter
golangci-lint run

# Run tests
go test ./...

# Build
go build
```

## Code Navigation

- **Control Plane**: `core/enterprise/control_plane/`
- **Tenant Node**: `core/enterprise/tenant_node/`
- **Gateway**: `core/enterprise/gateway/`
- **Types**: `core/enterprise/types.go`
- **Tests**: Look for `*_test.go` files

## Additional Resources

### Documentation

- [Architecture Overview](architecture.md)
- [Control Plane](control-plane.md)
- [Storage Strategy](storage.md)

### External Resources

- [Go Documentation](https://go.dev/doc/)
- [Raft Consensus](https://raft.github.io/)
- [BadgerDB](https://dgraph.io/docs/badger/)
- [Litestream](https://litestream.io/)
