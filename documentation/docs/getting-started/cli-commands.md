# CLI Commands

PocketBase provides several command-line commands for managing your instance.

## serve

Start the web server.

```bash
./pocketbase serve [domain(s)] [flags]
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--http` | HTTP server address | `127.0.0.1:8090` |
| `--https` | HTTPS server address (enables auto TLS) | - |
| `--origins` | CORS allowed origins | `*` |
| `--dir` | Data directory path | `./pb_data` |
| `--dev` | Enable development mode | `false` |
| `--queryTimeout` | Default SELECT query timeout | `30s` |
| `--encryptionEnv` | Env variable for settings encryption | - |

### Examples

```bash
# Basic usage
./pocketbase serve

# Custom port
./pocketbase serve --http=0.0.0.0:8080

# HTTPS with auto-cert
./pocketbase serve --https=example.com

# Multiple domains
./pocketbase serve --https=example.com,www.example.com

# Development mode
./pocketbase serve --dev

# Custom data directory
./pocketbase serve --dir=/var/lib/pocketbase
```

## superuser

Manage superuser accounts.

### Create Superuser

```bash
./pocketbase superuser create <email> <password>
```

### Update Superuser

```bash
./pocketbase superuser update <email> <new_password>
```

### Upsert Superuser

Create or update a superuser:

```bash
./pocketbase superuser upsert <email> <password>
```

### Delete Superuser

```bash
./pocketbase superuser delete <email>
```

### Generate OTP

Generate a one-time password for superuser login:

```bash
./pocketbase superuser otp <email>
```

### Examples

```bash
# Create new superuser
./pocketbase superuser create admin@example.com mysecretpassword

# Update existing superuser password
./pocketbase superuser update admin@example.com newpassword

# Create or update
./pocketbase superuser upsert admin@example.com password123

# Delete superuser
./pocketbase superuser delete admin@example.com

# Get OTP for login
./pocketbase superuser otp admin@example.com
```

## migrate

Run database migrations.

```bash
./pocketbase migrate [flags]
```

### Subcommands

| Command | Description |
|---------|-------------|
| `up` | Run all pending migrations |
| `down` | Revert the last migration |
| `create` | Create a new migration file |
| `collections` | Generate migrations from collections |

### Examples

```bash
# Run pending migrations
./pocketbase migrate up

# Revert last migration
./pocketbase migrate down

# Create new Go migration
./pocketbase migrate create "add_user_fields"

# Create JavaScript migration
./pocketbase migrate create "add_user_fields" --js

# Generate migrations from current collections
./pocketbase migrate collections
```

## version

Display the current PocketBase version.

```bash
./pocketbase version
```

Output:

```
PocketBase v0.x.x
```

## help

Display help information.

```bash
./pocketbase help
./pocketbase help serve
./pocketbase help superuser
```

## Global Flags

These flags are available for all commands:

| Flag | Description |
|------|-------------|
| `--dir` | Data directory path |
| `--encryptionEnv` | Environment variable for encryption key |
| `--help` | Show help |

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |

## Running as a Service

### systemd (Linux)

Create `/etc/systemd/system/pocketbase.service`:

```ini
[Unit]
Description=PocketBase
After=network.target

[Service]
Type=simple
User=pocketbase
Group=pocketbase
WorkingDirectory=/opt/pocketbase
ExecStart=/opt/pocketbase/pocketbase serve --http=0.0.0.0:8090
Restart=on-failure
RestartSec=5

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/pocketbase/pb_data

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable pocketbase
sudo systemctl start pocketbase
```

### Docker

```bash
docker run -d \
  --name pocketbase \
  -p 8090:8090 \
  -v /path/to/data:/pb/pb_data \
  pocketbase/pocketbase:latest \
  serve --http=0.0.0.0:8090
```

### Supervisor

Create `/etc/supervisor/conf.d/pocketbase.conf`:

```ini
[program:pocketbase]
command=/opt/pocketbase/pocketbase serve --http=0.0.0.0:8090
directory=/opt/pocketbase
user=pocketbase
autostart=true
autorestart=true
stderr_logfile=/var/log/pocketbase/error.log
stdout_logfile=/var/log/pocketbase/output.log
```

## Next Steps

- [Configuration Guide](configuration.md)
- [Deployment Guide](../guides/deployment.md)
- [API Overview](../api/overview.md)
