# PocketBase Enterprise Deployment

This directory contains deployment artifacts for running PocketBase Enterprise in production.

## Directory Structure

```
deploy/
├── systemd/              # Production systemd service files
│   ├── *.service         # Service unit files
│   ├── *.env.example     # Environment configuration templates
│   └── install.sh        # Installation script
├── docker/               # Docker/Container deployment
│   ├── Dockerfile        # Multi-stage build
│   ├── docker-compose.yml         # Full multi-node cluster
│   └── docker-compose.simple.yml  # Simple all-in-one mode
└── monitoring/           # Observability stack
    ├── prometheus/       # Prometheus configuration
    │   ├── prometheus.yml
    │   └── alerts.yml
    └── grafana/          # Grafana dashboards
        ├── provisioning/
        └── dashboards/
```

## Quick Start

### Local Development (Docker Compose)

```bash
# Full multi-node cluster (3 CP + 2 TN + Gateway + Monitoring)
cd deploy/docker
docker-compose up -d

# Simple all-in-one mode
docker-compose -f docker-compose.simple.yml up -d
```

**Access Points:**
- Gateway: http://localhost:8080
- Control Plane API: http://localhost:8090
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

### Production (Hetzner/Bare Metal)

1. **Install on each server:**
   ```bash
   cd deploy/systemd
   sudo ./install.sh
   ```

2. **Configure:**
   ```bash
   # Control Plane nodes
   sudo cp /etc/pocketbase/control-plane.env.example /etc/pocketbase/control-plane.env
   sudo vim /etc/pocketbase/control-plane.env

   # Tenant Nodes
   sudo cp /etc/pocketbase/tenant-node.env.example /etc/pocketbase/tenant-node.env
   sudo vim /etc/pocketbase/tenant-node.env

   # Gateway
   sudo cp /etc/pocketbase/gateway.env.example /etc/pocketbase/gateway.env
   sudo vim /etc/pocketbase/gateway.env
   ```

3. **Start services:**
   ```bash
   # On Control Plane servers (start first node, then others)
   sudo systemctl enable --now pocketbase-control-plane

   # On Tenant Node servers
   sudo systemctl enable --now pocketbase-tenant-node

   # On Gateway servers
   sudo systemctl enable --now pocketbase-gateway
   ```

4. **View logs:**
   ```bash
   journalctl -u pocketbase-control-plane -f
   journalctl -u pocketbase-tenant-node -f
   journalctl -u pocketbase-gateway -f
   ```

## Architecture Reference

```
                    ┌─────────────────┐
                    │  Load Balancer  │
                    │   (Hetzner LB)  │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
        ┌──────────┐   ┌──────────┐   ┌──────────┐
        │ Gateway  │   │ Gateway  │   │ Gateway  │
        │  :8080   │   │  :8080   │   │  :8080   │
        └────┬─────┘   └────┬─────┘   └────┬─────┘
             │              │              │
             └──────────────┼──────────────┘
                            │
              ┌─────────────┼─────────────┐
              ▼             ▼             ▼
        ┌───────────┐ ┌───────────┐ ┌───────────┐
        │ Control   │ │ Control   │ │ Control   │
        │ Plane 1   │◄│ Plane 2   │◄│ Plane 3   │
        │  :8090    │ │  :8090    │ │  :8090    │
        │  :7000    │ │  :7000    │ │  :7000    │
        └───────────┘ └───────────┘ └───────────┘
              │             │             │
              │       Raft Consensus      │
              │             │             │
        ┌─────┴─────────────┴─────────────┴─────┐
        │                                        │
        ▼                                        ▼
  ┌───────────┐                           ┌───────────┐
  │ Tenant    │                           │ Tenant    │
  │ Node 1    │                           │ Node 2    │
  │  :8091    │                           │  :8091    │
  └─────┬─────┘                           └─────┬─────┘
        │                                       │
        └───────────────┬───────────────────────┘
                        │
                        ▼
              ┌──────────────────┐
              │ Hetzner Object   │
              │ Storage (S3)     │
              └──────────────────┘
```

## Hetzner Recommended Configuration

### Starter (up to 100 tenants)
| Component | Instance | Count | Monthly Cost |
|-----------|----------|-------|--------------|
| Control Plane | CPX21 | 3 | €27 |
| Tenant Node | CPX31 | 2 | €42 |
| Gateway | CPX11 | 2 | €12 |
| Load Balancer | LB11 | 1 | €6 |
| Object Storage | - | 1TB | €5 |
| Private Network | - | 1 | Free |
| **Total** | | | **~€92/mo** |

### Growth (up to 1,000 tenants)
| Component | Instance | Count | Monthly Cost |
|-----------|----------|-------|--------------|
| Control Plane | CPX31 | 3 | €63 |
| Tenant Node | CPX41 | 4 | €120 |
| Gateway | CPX21 | 3 | €27 |
| Load Balancer | LB11 | 1 | €6 |
| Object Storage | - | 5TB | €25 |
| **Total** | | | **~€241/mo** |

## Monitoring

### Prometheus Metrics

PocketBase exposes metrics at `/api/metrics`:

- `pocketbase_tenant_node_active_tenants` - Active tenant count
- `pocketbase_gateway_requests_total` - Request count by status
- `pocketbase_gateway_request_duration_seconds` - Request latency histogram
- `pocketbase_tenant_load_duration_seconds` - Tenant load time
- `pocketbase_litestream_replication_lag_seconds` - S3 replication lag

### Alerts

Pre-configured alerts in `monitoring/prometheus/alerts.yml`:

- `ControlPlaneDown` - Control plane node unreachable
- `RaftNoLeader` - No Raft leader elected
- `TenantNodeHighCPU` - CPU > 80% for 10 minutes
- `GatewayHighErrorRate` - Error rate > 5%
- `S3ReplicationLag` - Litestream lag > 60 seconds

### Grafana Dashboard

The cluster overview dashboard shows:

- Component health status
- Request rate and latency
- Active tenants per node
- CPU and memory usage
- Error rates
