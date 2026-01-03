# Admin UI Settings

Configure your PocketBase application through the Settings page.

## Application Settings

### Meta

| Setting | Description |
|---------|-------------|
| Application name | Displayed in emails and admin UI |
| Application URL | Base URL for your application |
| Sender name | Name for outgoing emails |
| Sender address | Email address for outgoing emails |

### Hide Controls

Hide certain UI elements for non-technical users:

- Collection create/edit controls
- Certain admin features

## Mail Settings (SMTP)

Configure email delivery for verification, password reset, etc.

### SMTP Configuration

| Setting | Description |
|---------|-------------|
| Enabled | Turn email sending on/off |
| SMTP host | Mail server hostname |
| SMTP port | Mail server port (usually 587 or 465) |
| Username | SMTP authentication username |
| Password | SMTP authentication password |
| TLS | Enable TLS encryption |
| Auth method | Authentication method |

### Common SMTP Providers

**Gmail:**
```
Host: smtp.gmail.com
Port: 587
TLS: true
```

**SendGrid:**
```
Host: smtp.sendgrid.net
Port: 587
Username: apikey
Password: your_api_key
```

**Mailgun:**
```
Host: smtp.mailgun.org
Port: 587
```

### Test Email

Click **Send test email** to verify your configuration.

## File Storage (S3)

Configure S3-compatible storage for file uploads.

### S3 Configuration

| Setting | Description |
|---------|-------------|
| Enabled | Use S3 instead of local storage |
| Bucket | S3 bucket name |
| Region | AWS region |
| Endpoint | S3 endpoint URL |
| Access key | AWS access key ID |
| Secret | AWS secret access key |
| Force path style | Use path-style URLs |

### Compatible Providers

- Amazon S3
- DigitalOcean Spaces
- Backblaze B2
- MinIO
- Cloudflare R2
- Wasabi

### Test Connection

Click **Test connection** to verify S3 configuration.

## Backups

Configure automatic database backups.

### Backup Settings

| Setting | Description |
|---------|-------------|
| Cron schedule | When to run automatic backups |
| Max backups | Number of backups to retain |
| S3 storage | Store backups in S3 |

### Cron Examples

```
0 0 * * *     # Daily at midnight
0 */6 * * *   # Every 6 hours
0 0 * * 0     # Weekly on Sunday
0 0 1 * *     # Monthly on 1st
```

### Manual Backup

Click **Create backup** to manually create a backup.

### Restore Backup

1. Select backup from list
2. Click **Restore**
3. Confirm restoration

!!! danger "Warning"
    Restoring replaces all current data.

## Auth Providers (OAuth2)

Configure OAuth2/social login providers.

### Adding Providers

1. Go to Settings > Auth providers
2. Click on a provider
3. Enter Client ID and Secret
4. Enable the provider
5. Save

### Available Providers

- Google
- Facebook
- GitHub
- GitLab
- Discord
- Twitter
- Microsoft
- Apple
- And more...

### Provider Configuration

Each provider requires:

1. Create an OAuth app in the provider's console
2. Set redirect URI to: `https://yourdomain.com/api/oauth2-redirect`
3. Copy Client ID and Secret to PocketBase

## Token Settings

Configure authentication token behavior.

| Setting | Description |
|---------|-------------|
| Token duration | How long auth tokens are valid |

Default is 14 days (1209600 seconds).

## Logs Settings

Configure request logging.

| Setting | Description |
|---------|-------------|
| Max days | How long to retain logs |
| Log IP | Whether to log client IP addresses |
| Min level | Minimum log level to record |

## Export/Import Settings

### Export Settings

Settings can be exported for backup:

```bash
GET /api/settings
Authorization: Bearer <superuser_token>
```

### Import Settings

Update settings via API:

```bash
PATCH /api/settings
Authorization: Bearer <superuser_token>
Content-Type: application/json

{
  "meta": {
    "appName": "My App"
  }
}
```

## Settings Security

### Sensitive Fields

Some settings are encrypted at rest:

- SMTP password
- S3 secret key
- OAuth client secrets

### Encryption Key

Configure encryption for sensitive settings:

```bash
export PB_ENCRYPTION_KEY=your-32-char-key
./pocketbase serve --encryptionEnv=PB_ENCRYPTION_KEY
```

## Recommended Production Settings

### Email

- Use a transactional email service (SendGrid, Mailgun)
- Set proper sender address
- Test email delivery

### Storage

- Use S3 for file storage
- Configure backup to S3
- Set up automatic backups

### Security

- Enable HTTPS
- Restrict CORS origins
- Set appropriate token duration
- Enable email verification

## Troubleshooting

### Emails Not Sending

1. Verify SMTP credentials
2. Check sender address is valid
3. Test with a simple provider first
4. Review server logs

### S3 Connection Failed

1. Verify credentials
2. Check bucket exists
3. Verify endpoint URL format
4. Check network connectivity

### Settings Not Saving

1. Ensure you're a superuser
2. Check for validation errors
3. Verify encryption key if set

## Next Steps

- [Email Templates](../features/email-templates.md)
- [File Management](../features/file-management.md)
- [Production Setup](../guides/production.md)
