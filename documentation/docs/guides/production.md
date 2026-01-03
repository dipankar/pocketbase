# Production Setup

Best practices and configuration for running PocketBase in production.

## Pre-Production Checklist

### Security

- [ ] HTTPS enabled and enforced
- [ ] Strong superuser password set
- [ ] CORS origins restricted
- [ ] File upload limits configured
- [ ] API rules properly configured
- [ ] Email verification enabled (if using auth)

### Infrastructure

- [ ] Reverse proxy configured (nginx/Caddy)
- [ ] Process manager running (systemd)
- [ ] Automatic restarts enabled
- [ ] Health checks configured
- [ ] Firewall rules set

### Data

- [ ] Backup strategy implemented
- [ ] S3 configured for file storage
- [ ] Data directory permissions correct
- [ ] Encryption key set for sensitive settings

### Monitoring

- [ ] Log aggregation set up
- [ ] Uptime monitoring active
- [ ] Disk space alerts configured
- [ ] Error alerting enabled

## HTTPS Configuration

### Option 1: PocketBase Auto TLS

Best for simple deployments:

```bash
./pocketbase serve --https=example.com
```

### Option 2: Reverse Proxy (Recommended)

Better for complex deployments:

**Nginx example:**

```nginx
server {
    listen 443 ssl http2;
    server_name example.com;

    ssl_certificate /etc/ssl/certs/example.com.crt;
    ssl_certificate_key /etc/ssl/private/example.com.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
    ssl_prefer_server_ciphers off;

    # HSTS
    add_header Strict-Transport-Security "max-age=63072000" always;

    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## CORS Configuration

Restrict origins to your domains:

```bash
./pocketbase serve --origins=https://myapp.com,https://admin.myapp.com
```

## Rate Limiting

PocketBase doesn't include built-in rate limiting. Use your reverse proxy:

**Nginx:**

```nginx
http {
    limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;

    server {
        location /api/ {
            limit_req zone=api burst=20 nodelay;
            proxy_pass http://127.0.0.1:8090;
        }
    }
}
```

**Caddy:**

```
example.com {
    rate_limit {
        zone api {
            key {remote_host}
            events 10
            window 1s
        }
    }

    reverse_proxy localhost:8090
}
```

## File Upload Limits

Configure in your reverse proxy:

**Nginx:**

```nginx
client_max_body_size 100M;
```

**Caddy:**

```
request_body {
    max_size 100MB
}
```

Also set limits in collection field options.

## Settings Encryption

Encrypt sensitive settings at rest:

```bash
# Generate a 32-character key
openssl rand -hex 16

# Set environment variable
export PB_ENCRYPTION_KEY=your32characterencryptionkey12

# Start with encryption
./pocketbase serve --encryptionEnv=PB_ENCRYPTION_KEY
```

!!! danger "Key Management"
    Store the encryption key securely. Without it, encrypted settings are unrecoverable.

## S3 File Storage

Use S3 for production file storage:

1. Create S3 bucket
2. Configure in Admin UI > Settings > Files storage
3. Set appropriate bucket policies
4. Enable versioning (optional)

### Bucket Policy Example

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "PocketBaseAccess",
            "Effect": "Allow",
            "Principal": {
                "AWS": "arn:aws:iam::ACCOUNT_ID:user/pocketbase"
            },
            "Action": [
                "s3:GetObject",
                "s3:PutObject",
                "s3:DeleteObject",
                "s3:ListBucket"
            ],
            "Resource": [
                "arn:aws:s3:::your-bucket",
                "arn:aws:s3:::your-bucket/*"
            ]
        }
    ]
}
```

## Backup Configuration

### Automatic Backups

```bash
# Via CLI
./pocketbase backup create daily_backup

# Via API
curl -X POST http://localhost:8090/api/backups \
  -H "Authorization: Bearer <superuser_token>" \
  -H "Content-Type: application/json" \
  -d '{"name": "daily_backup"}'
```

### Backup to S3

Configure S3 for backups in Admin UI > Settings > Backups.

### Backup Schedule

```
# Daily at 2 AM
0 2 * * *

# Every 6 hours
0 */6 * * *
```

### Backup Retention

Set `cronMaxKeep` to control how many backups to retain.

## Database Optimization

### Query Timeout

Set reasonable query timeouts:

```bash
./pocketbase serve --queryTimeout=30s
```

### Indexes

Add indexes for frequently queried fields:

```sql
CREATE INDEX idx_posts_author ON posts (author);
CREATE INDEX idx_posts_status_created ON posts (status, created);
```

### Vacuum

SQLite automatically handles vacuuming, but you can trigger it:

```bash
sqlite3 pb_data/data.db "VACUUM;"
```

## Memory Configuration

PocketBase uses SQLite with reasonable defaults. For high-traffic:

```go
// In custom Go build
app := pocketbase.New()

// Configure connection pool
db := app.DB()
db.SetMaxOpenConns(100)
db.SetMaxIdleConns(10)
db.SetConnMaxLifetime(time.Hour)
```

## Logging

### Production Logging

Don't use `--dev` in production. Configure structured logging:

```bash
./pocketbase serve 2>&1 | logger -t pocketbase
```

### Log Rotation

With logrotate:

```
/var/log/pocketbase/*.log {
    daily
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 pocketbase pocketbase
    sharedscripts
}
```

## Monitoring

### Health Check Endpoint

```bash
# Simple health check
curl -f http://localhost:8090/api/health || exit 1
```

### Prometheus Metrics

Use reverse proxy metrics or implement custom metrics via hooks.

### External Monitoring

Configure uptime monitoring:

```yaml
# Example healthcheck.io
checks:
  - name: PocketBase
    url: https://example.com/api/health
    interval: 60
    expected_status: 200
```

## Disaster Recovery

### Recovery Time Objective (RTO)

- Restore from backup: ~5-30 minutes
- Redeploy: ~10-60 minutes

### Recovery Point Objective (RPO)

- With hourly backups: 1 hour max data loss
- With realtime S3 replication: minutes

### Recovery Procedure

1. Stop failed instance
2. Deploy new instance
3. Restore from latest backup
4. Verify data integrity
5. Update DNS if needed
6. Test functionality

## Security Hardening

### Linux Server

```bash
# Firewall
ufw allow 22
ufw allow 443
ufw enable

# Disable root login
# In /etc/ssh/sshd_config
PermitRootLogin no

# Automatic updates
apt install unattended-upgrades
```

### Application

- Use strong superuser passwords
- Enable MFA if available
- Review API rules regularly
- Audit logs periodically

## Performance Tuning

### Reverse Proxy Caching

```nginx
# Cache static assets
location /api/files/ {
    proxy_pass http://127.0.0.1:8090;
    proxy_cache_valid 200 1d;
    add_header Cache-Control "public, max-age=86400";
}
```

### CDN Integration

Use CDN for file serving:

1. Configure CDN with PocketBase as origin
2. Set appropriate cache headers
3. Update file URLs in frontend

### Connection Pooling

Use connection pooling in reverse proxy for better performance.

## Maintenance

### Updates

1. Backup data
2. Download new version
3. Stop current instance
4. Replace binary
5. Run migrations (if any)
6. Start new instance
7. Verify functionality

### Monitoring Disk Space

```bash
# Alert when disk usage > 80%
df -h /opt/pocketbase/pb_data | awk 'NR==2 {if($5+0 > 80) print "Disk usage high: "$5}'
```

### Log Cleanup

Configure log retention in Admin UI > Settings > Logs.

## Next Steps

- [Troubleshooting](troubleshooting.md)
- [Deployment Guide](deployment.md)
- [Backups](../api/backups.md)
