# Deployment Guide

Deploy PocketBase to various environments and platforms.

## Basic Deployment

### Direct Binary

1. Download the release for your OS
2. Upload to your server
3. Run with systemd or similar

```bash
# On server
chmod +x pocketbase
./pocketbase serve --http=0.0.0.0:8080
```

### Recommended Directory Structure

```
/opt/pocketbase/
├── pocketbase          # The executable
├── pb_data/            # Data directory
│   ├── data.db
│   ├── aux.db
│   └── storage/
├── pb_hooks/           # JavaScript hooks (optional)
└── pb_migrations/      # Migrations (optional)
```

## Linux (systemd)

Create `/etc/systemd/system/pocketbase.service`:

```ini
[Unit]
Description=PocketBase
After=network.target

[Service]
Type=simple
User=pocketbase
Group=pocketbase
LimitNOFILE=4096

WorkingDirectory=/opt/pocketbase
ExecStart=/opt/pocketbase/pocketbase serve --http=127.0.0.1:8090

Restart=always
RestartSec=5

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/pocketbase/pb_data
ReadOnlyPaths=/opt/pocketbase

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
# Create user
sudo useradd -r -s /bin/false pocketbase
sudo chown -R pocketbase:pocketbase /opt/pocketbase

# Enable service
sudo systemctl daemon-reload
sudo systemctl enable pocketbase
sudo systemctl start pocketbase

# Check status
sudo systemctl status pocketbase
```

## Docker

### Dockerfile

```dockerfile
FROM alpine:latest

ARG PB_VERSION=0.23.0

RUN apk add --no-cache \
    unzip \
    ca-certificates

# Download PocketBase
ADD https://github.com/pocketbase/pocketbase/releases/download/v${PB_VERSION}/pocketbase_${PB_VERSION}_linux_amd64.zip /tmp/pb.zip
RUN unzip /tmp/pb.zip -d /pb/ && rm /tmp/pb.zip

# Create data directory
RUN mkdir -p /pb/pb_data

EXPOSE 8080

WORKDIR /pb

CMD ["/pb/pocketbase", "serve", "--http=0.0.0.0:8080"]
```

### Docker Compose

```yaml
version: '3.8'

services:
  pocketbase:
    build: .
    container_name: pocketbase
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./pb_data:/pb/pb_data
      - ./pb_hooks:/pb/pb_hooks
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

Build and run:

```bash
docker-compose up -d
```

### Docker with Custom Go Build

```dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o pocketbase .

FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/pocketbase /pb/pocketbase

WORKDIR /pb
EXPOSE 8080

CMD ["/pb/pocketbase", "serve", "--http=0.0.0.0:8080"]
```

## Reverse Proxy

### Nginx

```nginx
server {
    listen 80;
    server_name example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name example.com;

    ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

    client_max_body_size 100M;

    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}
```

### Caddy

```
example.com {
    reverse_proxy localhost:8090

    # Enable file uploads
    request_body {
        max_size 100MB
    }
}
```

### Traefik

```yaml
# docker-compose.yml
services:
  pocketbase:
    image: pocketbase
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.pocketbase.rule=Host(`example.com`)"
      - "traefik.http.routers.pocketbase.tls=true"
      - "traefik.http.routers.pocketbase.tls.certresolver=letsencrypt"
      - "traefik.http.services.pocketbase.loadbalancer.server.port=8080"
```

## Cloud Platforms

### DigitalOcean App Platform

1. Create a Dockerfile in your repo
2. Connect repo to App Platform
3. Configure:
   - HTTP Port: 8080
   - Mount volume to `/pb/pb_data`

### Fly.io

Create `fly.toml`:

```toml
app = "my-pocketbase"
primary_region = "iad"

[build]
  dockerfile = "Dockerfile"

[env]
  PORT = "8080"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = false
  auto_start_machines = true

[mounts]
  source = "pb_data"
  destination = "/pb/pb_data"
```

Deploy:

```bash
fly launch
fly volumes create pb_data --size 1
fly deploy
```

### Railway

1. Connect GitHub repository
2. Add Dockerfile
3. Configure persistent volume for `/pb/pb_data`
4. Deploy

### Render

1. Create new Web Service
2. Connect repository
3. Set build command: `docker build`
4. Add disk for `/pb/pb_data`

## HTTPS Configuration

### With PocketBase (Auto TLS)

```bash
./pocketbase serve --https=example.com
```

PocketBase will automatically obtain Let's Encrypt certificates.

### With Reverse Proxy

Use nginx/Caddy/Traefik for TLS termination (recommended for production).

### Certificate Requirements

- Valid domain name pointing to server
- Ports 80 and 443 accessible
- DNS properly configured

## Environment Variables

```bash
# Data directory
export PB_DATA_DIR=/path/to/pb_data

# Encryption key for sensitive settings
export PB_ENCRYPTION_KEY=your-32-character-secret

# Run
./pocketbase serve \
  --dir=$PB_DATA_DIR \
  --encryptionEnv=PB_ENCRYPTION_KEY
```

## Health Checks

PocketBase provides a health endpoint:

```bash
curl http://localhost:8090/api/health
```

Returns:
```json
{"code":200,"message":"API is healthy."}
```

## Logging

### Access Logs

Enable verbose logging with `--dev` flag (not for production).

### Log to File

```bash
./pocketbase serve 2>&1 | tee -a pocketbase.log
```

Or use systemd journaling:

```bash
journalctl -u pocketbase -f
```

## Backup Strategy

### Automated Backups

Configure in Admin UI or via API:

```bash
PATCH /api/settings
{
  "backups": {
    "cron": "0 0 * * *",
    "cronMaxKeep": 7
  }
}
```

### Manual Backup

```bash
# Stop PocketBase (for consistency)
systemctl stop pocketbase

# Copy data directory
cp -r pb_data pb_data_backup_$(date +%Y%m%d)

# Restart
systemctl start pocketbase
```

### S3 Backup

Configure S3 in settings for off-site backups.

## Monitoring

### Process Monitoring

Use systemd, supervisord, or PM2 to ensure PocketBase restarts on failure.

### Resource Monitoring

Monitor:
- CPU usage
- Memory usage
- Disk space (especially pb_data)
- Network connections

### External Monitoring

Use uptime monitoring services:
- UptimeRobot
- Pingdom
- Healthchecks.io

## Scaling Considerations

PocketBase is designed for single-instance deployments. For high-traffic scenarios:

1. **Optimize SQLite** - Configure appropriate cache sizes
2. **Use CDN** - Serve static files and file uploads via CDN
3. **Read replicas** - Use Litestream for SQLite replication
4. **Enterprise Edition** - For horizontal scaling needs

## Security Checklist

- [ ] HTTPS enabled
- [ ] Firewall configured (only 443, 22)
- [ ] Strong superuser password
- [ ] Email verification enabled
- [ ] CORS restricted to your domains
- [ ] Regular backups configured
- [ ] File upload limits set
- [ ] Rate limiting via reverse proxy

## Next Steps

- [Production Setup](production.md)
- [Troubleshooting](troubleshooting.md)
- [Backups API](../api/backups.md)
