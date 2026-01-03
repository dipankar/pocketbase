# Production Deployment

This guide covers deploying PocketBase Enterprise to production environments, with specific guidance for Hetzner Cloud.

---

## Deployment Options

### Option 1: Systemd Services (Recommended for Production)

Production-ready systemd service files with security hardening.

### Option 2: Docker Compose

Container-based deployment for local testing and container orchestration platforms.

### Option 3: Kubernetes

For large-scale deployments (see Kubernetes section below).

---

## Architecture Overview

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

---

## Hetzner Cloud Deployment

### Infrastructure Requirements

#### Starter Setup (up to 100 tenants)

| Component | Instance Type | Count | Monthly Cost |
|-----------|---------------|-------|--------------|
| Control Plane | CPX21 (3 vCPU, 4GB) | 3 | €27 |
| Tenant Node | CPX31 (4 vCPU, 8GB) | 2 | €42 |
| Gateway | CPX11 (2 vCPU, 2GB) | 2 | €12 |
| Load Balancer | LB11 | 1 | €6 |
| Object Storage | 1TB | 1 | €5 |
| Private Network | - | 1 | Free |
| **Total** | | | **~€92/mo** |

#### Growth Setup (up to 1,000 tenants)

| Component | Instance Type | Count | Monthly Cost |
|-----------|---------------|-------|--------------|
| Control Plane | CPX31 (4 vCPU, 8GB) | 3 | €63 |
| Tenant Node | CPX41 (8 vCPU, 16GB) | 4 | €120 |
| Gateway | CPX21 (3 vCPU, 4GB) | 3 | €27 |
| Load Balancer | LB11 | 1 | €6 |
| Object Storage | 5TB | 1 | €25 |
| **Total** | | | **~€241/mo** |

### Step 1: Create Infrastructure

```bash
# Install hcloud CLI
brew install hcloud  # macOS
# or
apt install hcloud-cli  # Debian/Ubuntu

# Create private network
hcloud network create --name pocketbase-net --ip-range 10.0.0.0/8

# Create subnet
hcloud network add-subnet pocketbase-net \
  --type cloud \
  --network-zone eu-central \
  --ip-range 10.0.0.0/24

# Create Control Plane servers
for i in 1 2 3; do
  hcloud server create \
    --name cp-$i \
    --type cpx21 \
    --image ubuntu-22.04 \
    --location fsn1 \
    --network pocketbase-net \
    --ssh-key your-key
done

# Create Tenant Node servers
for i in 1 2; do
  hcloud server create \
    --name tn-$i \
    --type cpx31 \
    --image ubuntu-22.04 \
    --location fsn1 \
    --network pocketbase-net \
    --ssh-key your-key
done

# Create Gateway servers
for i in 1 2; do
  hcloud server create \
    --name gw-$i \
    --type cpx11 \
    --image ubuntu-22.04 \
    --location fsn1 \
    --network pocketbase-net \
    --ssh-key your-key
done

# Create Load Balancer
hcloud load-balancer create \
  --name pocketbase-lb \
  --type lb11 \
  --location fsn1 \
  --network-zone eu-central
```

### Step 2: Configure Object Storage

1. Create a bucket in Hetzner Cloud Console
2. Generate access credentials
3. Note the endpoint URL (e.g., `https://fsn1.your-objectstorage.com`)

### Step 3: Install PocketBase

On each server:

```bash
# Clone the deployment files
git clone https://github.com/pocketbase/pocketbase.git
cd pocketbase/deploy/systemd

# Run installer
sudo ./install.sh

# Copy pocketbase binary
sudo cp /path/to/pocketbase /usr/local/bin/pocketbase
sudo chmod +x /usr/local/bin/pocketbase
```

### Step 4: Configure Control Plane

On each control plane server, create `/etc/pocketbase/control-plane.env`:

**cp-1 (Bootstrap node):**
```bash
NODE_ID=cp-1
RAFT_BIND_ADDR=10.0.0.1:7000
RAFT_ADVERTISE_ADDR=10.0.0.1:7000
RAFT_JOIN=
```

**cp-2:**
```bash
NODE_ID=cp-2
RAFT_BIND_ADDR=10.0.0.2:7000
RAFT_ADVERTISE_ADDR=10.0.0.2:7000
RAFT_JOIN=10.0.0.1:7000
```

**cp-3:**
```bash
NODE_ID=cp-3
RAFT_BIND_ADDR=10.0.0.3:7000
RAFT_ADVERTISE_ADDR=10.0.0.3:7000
RAFT_JOIN=10.0.0.1:7000,10.0.0.2:7000
```

Start in order:

```bash
# On cp-1 first
sudo systemctl enable --now pocketbase-control-plane

# Wait for it to become leader, then on cp-2
sudo systemctl enable --now pocketbase-control-plane

# Then on cp-3
sudo systemctl enable --now pocketbase-control-plane
```

### Step 5: Configure Tenant Nodes

On each tenant node, create `/etc/pocketbase/tenant-node.env`:

```bash
CONTROL_PLANE_ADDRS=10.0.0.1:8090,10.0.0.2:8090,10.0.0.3:8090
NODE_ADDRESS=10.0.1.1:8091  # Use this node's IP
MAX_TENANTS=100

# Hetzner Object Storage
S3_ENDPOINT=https://fsn1.your-objectstorage.com
S3_BUCKET=pocketbase-tenants
S3_ACCESS_KEY=your-access-key
S3_SECRET_KEY=your-secret-key
```

Start the service:

```bash
sudo systemctl enable --now pocketbase-tenant-node
```

### Step 6: Configure Gateways

On each gateway, create `/etc/pocketbase/gateway.env`:

```bash
CONTROL_PLANE_ADDRS=10.0.0.1:8090,10.0.0.2:8090,10.0.0.3:8090
```

Start the service:

```bash
sudo systemctl enable --now pocketbase-gateway
```

### Step 7: Configure Load Balancer

```bash
# Add gateway targets
hcloud load-balancer add-target pocketbase-lb \
  --server gw-1 \
  --use-private-ip

hcloud load-balancer add-target pocketbase-lb \
  --server gw-2 \
  --use-private-ip

# Configure HTTP service
hcloud load-balancer add-service pocketbase-lb \
  --protocol http \
  --listen-port 80 \
  --destination-port 8080

# Configure HTTPS service (with managed certificate)
hcloud load-balancer add-service pocketbase-lb \
  --protocol https \
  --listen-port 443 \
  --destination-port 8080 \
  --http-certificates your-cert-id
```

---

## Docker Compose Deployment

For local testing before production:

```bash
cd deploy/docker

# Full multi-node cluster
docker-compose up -d

# Access points:
# - Gateway: http://localhost:8080
# - Control Plane: http://localhost:8090
# - Prometheus: http://localhost:9090
# - Grafana: http://localhost:3000 (admin/admin)
```

For simple all-in-one mode:

```bash
docker-compose -f docker-compose.simple.yml up -d
```

---

## Monitoring Setup

### Prometheus

The deployment includes pre-configured Prometheus scrape configs:

```yaml
# deploy/monitoring/prometheus/prometheus.yml
scrape_configs:
  - job_name: 'pocketbase-control-plane'
    static_configs:
      - targets: ['cp-1:8090', 'cp-2:8090', 'cp-3:8090']

  - job_name: 'pocketbase-tenant-node'
    static_configs:
      - targets: ['tn-1:8091', 'tn-2:8091']

  - job_name: 'pocketbase-gateway'
    static_configs:
      - targets: ['gw-1:8080', 'gw-2:8080']
```

### Grafana Dashboard

Import the pre-built dashboard from `deploy/monitoring/grafana/dashboards/cluster-overview.json`.

Dashboard panels include:

- Cluster health status
- Request rate and latency
- Active tenants per node
- CPU and memory usage
- Error rates
- Replication lag

### Alerts

Pre-configured alerts in `deploy/monitoring/prometheus/alerts.yml`:

| Alert | Severity | Description |
|-------|----------|-------------|
| `ControlPlaneDown` | Critical | Control plane node unreachable |
| `RaftNoLeader` | Critical | No Raft leader for 5 minutes |
| `RaftQuorumLost` | Critical | Less than 2 CP nodes healthy |
| `TenantNodeDown` | Warning | Tenant node unreachable |
| `TenantNodeHighCPU` | Warning | CPU > 80% for 10 minutes |
| `GatewayHighErrorRate` | Warning | Error rate > 5% |
| `S3ReplicationLag` | Warning | Litestream lag > 60 seconds |

---

## Security Considerations

### Network Security

1. **Private Network**: All inter-component communication over private network
2. **Firewall Rules**:
   ```bash
   # Control Plane (internal only)
   ufw allow from 10.0.0.0/8 to any port 7000  # Raft
   ufw allow from 10.0.0.0/8 to any port 8090  # API

   # Tenant Node (internal only)
   ufw allow from 10.0.0.0/8 to any port 8091

   # Gateway (public via LB only)
   ufw allow from 10.0.0.0/8 to any port 8080
   ```

### TLS Configuration

For production, configure TLS on the load balancer or directly on gateways:

```bash
# Gateway with TLS
./pocketbase serve --mode=gateway \
  --https-addr=0.0.0.0:443 \
  --tls-cert=/etc/pocketbase/certs/fullchain.pem \
  --tls-key=/etc/pocketbase/certs/privkey.pem
```

### Secrets Management

Never commit secrets to version control. Use:

- Environment files with restricted permissions (`chmod 600`)
- Hetzner Cloud secrets or external secret managers
- Encrypted environment variables

---

## Backup and Recovery

### Automatic Backups

Litestream continuously replicates tenant databases to S3. Recovery is automatic on tenant load.

### Control Plane Backup

Raft snapshots are stored in the data directory. For disaster recovery:

```bash
# Backup BadgerDB
tar -czf cp-backup.tar.gz /var/lib/pocketbase/control-plane

# Restore
tar -xzf cp-backup.tar.gz -C /var/lib/pocketbase/
```

### Point-in-Time Recovery

Litestream supports point-in-time recovery:

```bash
# Restore tenant to specific time
litestream restore -o /tmp/restored.db \
  -timestamp "2024-01-15T10:30:00Z" \
  s3://bucket/tenants/tenant-123/litestream/data.db
```

---

## Scaling

### Horizontal Scaling

**Add Tenant Nodes:**

```bash
# Create new server
hcloud server create --name tn-3 --type cpx31 ...

# Install and configure
sudo ./install.sh
# Edit /etc/pocketbase/tenant-node.env
sudo systemctl enable --now pocketbase-tenant-node
```

**Add Gateways:**

```bash
# Create and configure gateway
hcloud server create --name gw-3 --type cpx11 ...

# Add to load balancer
hcloud load-balancer add-target pocketbase-lb --server gw-3
```

### Vertical Scaling

Upgrade instance types as needed:

```bash
hcloud server change-type tn-1 cpx41 --keep-disk
```

---

## Troubleshooting

### View Logs

```bash
# Control Plane
journalctl -u pocketbase-control-plane -f

# Tenant Node
journalctl -u pocketbase-tenant-node -f

# Gateway
journalctl -u pocketbase-gateway -f
```

### Health Checks

```bash
# Control Plane health
curl http://10.0.0.1:8090/api/health

# Raft status
curl http://10.0.0.1:8090/api/cp/raft/status

# Tenant Node health
curl http://10.0.1.1:8091/api/health

# Gateway health
curl http://10.0.2.1:8080/api/health
```

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| Raft leader election fails | Network partition | Check firewall rules for port 7000 |
| Tenant load timeout | S3 connectivity | Verify S3 credentials and endpoint |
| High memory on tenant node | Too many active tenants | Reduce `MAX_TENANTS` or add nodes |
| Gateway 502 errors | All tenant nodes down | Check tenant node health |

---

## Next Steps

- [Architecture](architecture.md) - Deep dive into system design
- [Control Plane](control-plane.md) - Raft consensus details
- [Storage Strategy](storage.md) - S3 and Litestream configuration
- [Development Guide](development-guide.md) - Local development setup
