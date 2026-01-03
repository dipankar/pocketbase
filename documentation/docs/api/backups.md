# Backups API

PocketBase provides built-in backup functionality for your database and uploaded files.

## List Backups

```bash
GET /api/backups
Authorization: Bearer <superuser_token>
```

**Response:**

```json
[
  {
    "key": "pb_backup_20240115_120000.zip",
    "size": 1048576,
    "modified": "2024-01-15 12:00:00.000Z"
  },
  {
    "key": "pb_backup_20240114_120000.zip",
    "size": 1024000,
    "modified": "2024-01-14 12:00:00.000Z"
  }
]
```

## Create Backup

```bash
POST /api/backups
Authorization: Bearer <superuser_token>
Content-Type: application/json

{
  "name": "manual_backup"
}
```

If `name` is not provided, an auto-generated name with timestamp is used.

**Response:**

```json
{
  "key": "manual_backup.zip"
}
```

## Download Backup

```bash
GET /api/backups/{key}
Authorization: Bearer <superuser_token>
```

Returns the backup file as a download.

### Generate Download Token

For browser downloads, generate a temporary token:

```bash
POST /api/backups/{key}/download-token
Authorization: Bearer <superuser_token>
```

**Response:**

```json
{
  "token": "temporary_download_token"
}
```

Then download:

```
GET /api/backups/{key}?token={download_token}
```

## Restore Backup

```bash
POST /api/backups/{key}/restore
Authorization: Bearer <superuser_token>
```

!!! danger "Warning"
    Restoring a backup replaces all current data. The operation cannot be undone.

**Response:**

```json
{
  "message": "Backup restored successfully."
}
```

## Delete Backup

```bash
DELETE /api/backups/{key}
Authorization: Bearer <superuser_token>
```

Returns `204 No Content` on success.

## Upload Backup

```bash
POST /api/backups/upload
Authorization: Bearer <superuser_token>
Content-Type: multipart/form-data

--boundary
Content-Disposition: form-data; name="file"; filename="external_backup.zip"
Content-Type: application/zip

<binary data>
--boundary--
```

## Backup Configuration

Configure automatic backups in the Admin UI or via settings API:

```bash
PATCH /api/settings
Authorization: Bearer <superuser_token>
Content-Type: application/json

{
  "backups": {
    "cron": "0 0 * * *",
    "cronMaxKeep": 7,
    "s3": {
      "enabled": true,
      "bucket": "my-backups",
      "region": "us-east-1",
      "endpoint": "https://s3.amazonaws.com",
      "accessKey": "...",
      "secret": "...",
      "forcePathStyle": false
    }
  }
}
```

### Configuration Options

| Option | Description |
|--------|-------------|
| `cron` | Cron expression for automatic backups |
| `cronMaxKeep` | Maximum number of auto-backups to retain |
| `s3.enabled` | Store backups in S3 |
| `s3.bucket` | S3 bucket name |
| `s3.region` | S3 region |
| `s3.endpoint` | S3 endpoint URL |
| `s3.accessKey` | S3 access key |
| `s3.secret` | S3 secret key |
| `s3.forcePathStyle` | Use path-style URLs |

### Cron Expression Examples

| Expression | Description |
|------------|-------------|
| `0 0 * * *` | Daily at midnight |
| `0 */6 * * *` | Every 6 hours |
| `0 0 * * 0` | Weekly on Sunday |
| `0 0 1 * *` | Monthly on the 1st |

## Backup Contents

Backups include:

- `data.db` - Main SQLite database
- `aux.db` - Auxiliary database (logs)
- `storage/` - All uploaded files

## JavaScript SDK Usage

```javascript
import PocketBase from 'pocketbase';

const pb = new PocketBase('http://127.0.0.1:8090');

// Authenticate as superuser
await pb.collection('_superusers').authWithPassword('admin@example.com', 'password');

// List backups
const backups = await pb.backups.getFullList();

// Create backup
const backup = await pb.backups.create('my_backup');

// Download backup
const blob = await pb.backups.download('my_backup.zip');

// Restore backup
await pb.backups.restore('my_backup.zip');

// Delete backup
await pb.backups.delete('my_backup.zip');

// Upload backup
const file = document.getElementById('backup-file').files[0];
await pb.backups.upload({ file });
```

## CLI Backup

Create backups via command line by copying the data directory:

```bash
# Stop PocketBase first for consistency
systemctl stop pocketbase

# Copy data directory
cp -r pb_data pb_data_backup_$(date +%Y%m%d)

# Restart PocketBase
systemctl start pocketbase
```

Or use SQLite's backup command:

```bash
sqlite3 pb_data/data.db ".backup 'backup.db'"
```

## Best Practices

1. **Regular backups** - Set up automatic daily or hourly backups
2. **Off-site storage** - Use S3 to store backups externally
3. **Test restores** - Periodically test backup restoration
4. **Retention policy** - Configure `cronMaxKeep` to manage storage
5. **Pre-migration backups** - Always backup before schema changes
6. **Monitor backup size** - Large files can impact backup time

## Disaster Recovery

### Recovery Steps

1. Stop the current PocketBase instance
2. Clear or backup the current `pb_data` directory
3. Start PocketBase with empty `pb_data`
4. Upload the backup file via API or copy directly
5. Restore the backup
6. Verify data integrity

### Manual Restoration

```bash
# Stop PocketBase
systemctl stop pocketbase

# Backup current data (just in case)
mv pb_data pb_data_old

# Create new data directory
mkdir pb_data

# Extract backup
unzip backup.zip -d pb_data

# Start PocketBase
systemctl start pocketbase
```

## Security Considerations

1. **Encrypt backups** - Backups contain all data including passwords (hashed)
2. **Secure S3 credentials** - Use IAM roles or environment variables
3. **Limit backup access** - Only superusers can access backup API
4. **Secure download tokens** - Tokens are temporary but should not be shared

## Next Steps

- [Deployment Guide](../guides/deployment.md)
- [Production Setup](../guides/production.md)
- [Settings API](../api/overview.md)
