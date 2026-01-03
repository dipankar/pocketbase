# Configuration

PocketBase can be configured through command-line flags, environment variables, and the Admin UI.

## Command Line Flags

### Server Configuration

```bash
./pocketbase serve [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--http` | HTTP server address | `127.0.0.1:8090` |
| `--https` | HTTPS server address (enables auto TLS) | - |
| `--origins` | CORS allowed origins (comma-separated) | `*` |
| `--dir` | Data directory path | `./pb_data` |
| `--dev` | Enable development mode | `false` |
| `--queryTimeout` | Default SELECT query timeout | `30s` |
| `--encryptionEnv` | Environment variable for settings encryption | - |

### Examples

```bash
# Run on a different port
./pocketbase serve --http=0.0.0.0:8080

# Enable HTTPS with automatic certificates
./pocketbase serve --https=example.com:443

# Custom data directory
./pocketbase serve --dir=/var/lib/pocketbase

# Development mode (verbose logging)
./pocketbase serve --dev

# Restrict CORS origins
./pocketbase serve --origins=https://myapp.com,https://admin.myapp.com
```

## Environment Variables

You can also use environment variables by prefixing the flag name with `PB_`:

```bash
export PB_HTTP=0.0.0.0:8080
export PB_DIR=/var/lib/pocketbase
./pocketbase serve
```

## Data Directory Structure

```
pb_data/
├── data.db              # Main SQLite database
├── aux.db               # Auxiliary database (logs, etc.)
├── storage/             # File uploads organized by collection
│   └── <collection_id>/
│       └── <record_id>/
│           └── <filename>
├── backups/             # Database backups
└── .pb_temp_to_delete/  # Temporary files
```

## Application Settings

Settings are stored in the database and managed via the Admin UI or API.

### General Settings

- **Application name** - Displayed in emails and admin UI
- **Application URL** - Base URL for your application
- **Hide controls** - Hide collection controls in admin UI

### Email Settings (SMTP)

Configure email delivery for verification, password reset, etc.

| Setting | Description |
|---------|-------------|
| Sender name | Name shown in sent emails |
| Sender address | Email address for outgoing mail |
| SMTP host | Mail server hostname |
| SMTP port | Mail server port (typically 587 or 465) |
| SMTP username | Authentication username |
| SMTP password | Authentication password |
| TLS encryption | Enable TLS/SSL |

### S3 Storage

Configure S3-compatible storage for file uploads:

| Setting | Description |
|---------|-------------|
| Endpoint | S3 endpoint URL |
| Bucket | Bucket name |
| Region | AWS region |
| Access Key | AWS access key ID |
| Secret | AWS secret access key |
| Force path style | Use path-style URLs |

### Backups

| Setting | Description |
|---------|-------------|
| Cron expression | Schedule for automatic backups |
| Max backups | Number of backups to retain |
| S3 storage | Store backups in S3 |

## Settings via API

Retrieve settings:

```bash
curl http://127.0.0.1:8090/api/settings \
  -H "Authorization: Bearer <superuser_token>"
```

Update settings:

```bash
curl -X PATCH http://127.0.0.1:8090/api/settings \
  -H "Authorization: Bearer <superuser_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "meta": {
      "appName": "My App",
      "appURL": "https://myapp.com"
    }
  }'
```

## Settings Encryption

Sensitive settings (passwords, API keys) can be encrypted at rest:

```bash
# Set the encryption key via environment variable
export PB_ENCRYPTION_KEY=your-32-character-secret-key

# Start with encryption enabled
./pocketbase serve --encryptionEnv=PB_ENCRYPTION_KEY
```

!!! warning "Encryption Key"
    Store your encryption key securely. If lost, encrypted settings cannot be recovered.

## Development Mode

Development mode (`--dev`) enables:

- Verbose request logging
- SQL query logging
- Auto-reload of JavaScript hooks
- Detailed error messages

```bash
./pocketbase serve --dev
```

!!! danger "Production Warning"
    Never use `--dev` in production as it exposes sensitive information.

## Query Timeout

Set the default timeout for SELECT queries:

```bash
./pocketbase serve --queryTimeout=60s
```

This helps prevent long-running queries from blocking the database.

## Logs Configuration

Logs are stored in the auxiliary database and can be configured in the Admin UI:

- **Max days** - How long to retain logs
- **Log IP** - Whether to log client IP addresses
- **Min level** - Minimum log level to record

## Collection-Level Settings

Each collection has its own settings:

### API Rules

Define who can access the API:

```
# Allow anyone
(empty rule)

# Authenticated users only
@request.auth.id != ""

# Owner only
@request.auth.id = user.id

# Custom conditions
@request.auth.verified = true && @request.auth.role = "admin"
```

### Indexes

Create database indexes for performance:

```sql
CREATE INDEX idx_posts_published ON posts (published)
CREATE INDEX idx_posts_author ON posts (author)
CREATE UNIQUE INDEX idx_users_username ON users (username)
```

### Options

- **List rule** - Filter for list/search operations
- **View rule** - Filter for viewing single records
- **Create rule** - Filter for creating records
- **Update rule** - Filter for updating records
- **Delete rule** - Filter for deleting records

## Next Steps

- [CLI Commands Reference](cli-commands.md)
- [API Overview](../api/overview.md)
- [Deployment Guide](../guides/deployment.md)
