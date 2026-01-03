# 14. Production Deployment

This guide covers deploying PocketBase Enterprise to production environments.

## Deployment Artifacts

All deployment files are in the `deploy/` directory:

```
deploy/
├── systemd/                    # Production service files
│   ├── pocketbase-control-plane.service
│   ├── pocketbase-tenant-node.service
│   ├── pocketbase-gateway.service
│   ├── *.env.example           # Configuration templates
│   └── install.sh              # Installation script
├── docker/                     # Container deployment
│   ├── Dockerfile
│   ├── docker-compose.yml      # Full multi-node cluster
│   └── docker-compose.simple.yml
└── monitoring/                 # Observability
    ├── prometheus/
    │   ├── prometheus.yml
    │   └── alerts.yml
    └── grafana/
        └── dashboards/
```

## Systemd Deployment

### Quick Start

```bash
cd deploy/systemd
sudo ./install.sh
```

This creates:
- `/usr/local/bin/pocketbase` - Binary location
- `/var/lib/pocketbase/` - Data directories
- `/etc/pocketbase/` - Configuration
- Systemd service files

### Configuration

**Control Plane** (`/etc/pocketbase/control-plane.env`):
```bash
NODE_ID=cp-1
RAFT_BIND_ADDR=10.0.0.1:7000
RAFT_ADVERTISE_ADDR=10.0.0.1:7000
RAFT_JOIN=  # Empty for bootstrap, comma-separated for joining
```

**Tenant Node** (`/etc/pocketbase/tenant-node.env`):
```bash
CONTROL_PLANE_ADDRS=10.0.0.1:8090,10.0.0.2:8090,10.0.0.3:8090
NODE_ADDRESS=10.0.1.1:8091
MAX_TENANTS=100
S3_ENDPOINT=https://fsn1.your-objectstorage.com
S3_BUCKET=pocketbase-tenants
S3_ACCESS_KEY=your-key
S3_SECRET_KEY=your-secret
```

**Gateway** (`/etc/pocketbase/gateway.env`):
```bash
CONTROL_PLANE_ADDRS=10.0.0.1:8090,10.0.0.2:8090,10.0.0.3:8090
```

### Starting Services

```bash
# Control Plane (start first, in order for Raft)
sudo systemctl enable --now pocketbase-control-plane

# Tenant Nodes (after control plane is healthy)
sudo systemctl enable --now pocketbase-tenant-node

# Gateway (after tenant nodes)
sudo systemctl enable --now pocketbase-gateway
```

## Docker Compose Deployment

### Full Cluster

```bash
cd deploy/docker
docker-compose up -d
```

Starts:
- 3 Control Plane nodes (Raft consensus)
- 2 Tenant Nodes
- 1 Gateway
- LocalStack (S3)
- Prometheus + Grafana

Access:
- Gateway: http://localhost:8080
- Control Plane: http://localhost:8090
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

### Simple Mode

```bash
docker-compose -f docker-compose.simple.yml up -d
```

Runs all-in-one mode for quick testing.

## Hetzner Cloud Setup

### Recommended Infrastructure

**Starter (100 tenants):**
| Component | Type | Count | Cost |
|-----------|------|-------|------|
| Control Plane | CPX21 | 3 | €27/mo |
| Tenant Node | CPX31 | 2 | €42/mo |
| Gateway | CPX11 | 2 | €12/mo |
| Load Balancer | LB11 | 1 | €6/mo |
| Object Storage | 1TB | 1 | €5/mo |
| **Total** | | | **€92/mo** |

**Growth (1,000 tenants):**
| Component | Type | Count | Cost |
|-----------|------|-------|------|
| Control Plane | CPX31 | 3 | €63/mo |
| Tenant Node | CPX41 | 4 | €120/mo |
| Gateway | CPX21 | 3 | €27/mo |
| Load Balancer | LB11 | 1 | €6/mo |
| Object Storage | 5TB | 1 | €25/mo |
| **Total** | | | **€241/mo** |

### Network Architecture

```
Internet
    │
    ▼
┌─────────────────┐
│  Load Balancer  │ (Public IP, HTTPS termination)
└────────┬────────┘
         │
    Private Network (10.0.0.0/8)
         │
    ┌────┴────┐
    │ Gateway │ (:8080)
    └────┬────┘
         │
    ┌────┴────┐
    │ Control │ (:8090, :7000 Raft)
    │ Plane   │
    └────┬────┘
         │
    ┌────┴────┐
    │ Tenant  │ (:8091)
    │ Nodes   │
    └────┬────┘
         │
    ┌────┴────┐
    │ Object  │ (S3)
    │ Storage │
    └─────────┘
```

### Create Infrastructure

```bash
# Create private network
hcloud network create --name pocketbase-net --ip-range 10.0.0.0/8

# Create servers
for i in 1 2 3; do
  hcloud server create --name cp-$i --type cpx21 --image ubuntu-22.04 \
    --network pocketbase-net --location fsn1
done

for i in 1 2; do
  hcloud server create --name tn-$i --type cpx31 --image ubuntu-22.04 \
    --network pocketbase-net --location fsn1
done

for i in 1 2; do
  hcloud server create --name gw-$i --type cpx11 --image ubuntu-22.04 \
    --network pocketbase-net --location fsn1
done

# Create load balancer
hcloud load-balancer create --name pocketbase-lb --type lb11 --location fsn1
```

## Monitoring

### Prometheus Metrics

PocketBase exposes metrics at `/api/metrics`:

- `pocketbase_tenant_node_active_tenants`
- `pocketbase_gateway_requests_total`
- `pocketbase_gateway_request_duration_seconds`
- `pocketbase_tenant_load_duration_seconds`
- `pocketbase_litestream_replication_lag_seconds`

### Pre-configured Alerts

| Alert | Severity | Trigger |
|-------|----------|---------|
| ControlPlaneDown | Critical | Node unreachable > 1m |
| RaftNoLeader | Critical | No leader > 5m |
| RaftQuorumLost | Critical | < 2 nodes healthy |
| TenantNodeDown | Warning | Node unreachable > 2m |
| GatewayHighErrorRate | Warning | Error rate > 5% |
| S3ReplicationLag | Warning | Lag > 60s |

### Grafana Dashboard

Import `deploy/monitoring/grafana/dashboards/cluster-overview.json` for:

- Cluster health status
- Request rate and latency
- Active tenants per node
- Resource usage
- Error rates

## Security

### Firewall Rules

```bash
# Control Plane (internal only)
ufw allow from 10.0.0.0/8 to any port 7000  # Raft
ufw allow from 10.0.0.0/8 to any port 8090  # API

# Tenant Node (internal only)
ufw allow from 10.0.0.0/8 to any port 8091

# Gateway (via load balancer only)
ufw allow from 10.0.0.0/8 to any port 8080
```

### TLS

Configure on load balancer or gateway:

```bash
./pocketbase serve --mode=gateway \
  --https-addr=0.0.0.0:443 \
  --tls-cert=/etc/pocketbase/certs/fullchain.pem \
  --tls-key=/etc/pocketbase/certs/privkey.pem
```

## Backup & Recovery

### Automatic (Litestream)

Tenant databases are continuously replicated to S3. Recovery is automatic on tenant load.

### Control Plane

```bash
# Backup
tar -czf cp-backup.tar.gz /var/lib/pocketbase/control-plane

# Restore
tar -xzf cp-backup.tar.gz -C /var/lib/pocketbase/
```

### Point-in-Time Recovery

```bash
litestream restore -o /tmp/restored.db \
  -timestamp "2024-01-15T10:30:00Z" \
  s3://bucket/tenants/tenant-123/litestream/data.db
```

## Scaling

### Add Tenant Node

```bash
hcloud server create --name tn-3 --type cpx31 ...
# Install and configure
sudo systemctl enable --now pocketbase-tenant-node
```

### Add Gateway

```bash
hcloud server create --name gw-3 --type cpx11 ...
hcloud load-balancer add-target pocketbase-lb --server gw-3
```

## Troubleshooting

### View Logs

```bash
journalctl -u pocketbase-control-plane -f
journalctl -u pocketbase-tenant-node -f
journalctl -u pocketbase-gateway -f
```

### Health Checks

```bash
curl http://localhost:8090/api/health        # Control Plane
curl http://localhost:8091/api/health        # Tenant Node
curl http://localhost:8080/api/health        # Gateway
curl http://localhost:8090/api/cp/raft/status # Raft status
```

---

Next: [Development Guide](13-development-guide.md)
