# Admin UI Overview

PocketBase includes a built-in Admin UI for managing your application without writing code.

## Accessing the Admin UI

The Admin UI is available at:

```
http://127.0.0.1:8090/_/
```

Replace with your server address in production.

## First-Time Setup

On first access, you'll be prompted to create a superuser account:

1. Enter your email address
2. Create a strong password
3. Click "Create admin"

Or create via CLI:

```bash
./pocketbase superuser create admin@example.com yourpassword
```

## Main Sections

### Dashboard

The dashboard provides an overview of your application:

- Recent activity
- Collection statistics
- Quick access to common actions

### Collections

Manage your data structure:

- Create, edit, delete collections
- Configure fields and schema
- Set API rules
- Manage indexes

### Records

Browse and manage data:

- View records in table format
- Filter and search
- Create, edit, delete records
- Export data

### Logs

View application logs:

- Request logs
- Error logs
- Filter by date, method, status

### Settings

Configure application settings:

- Application name and URL
- Email/SMTP configuration
- S3 file storage
- Backup settings
- Auth providers

## Navigation

### Sidebar

- **Collections** - List of all collections
- **Logs** - Request and error logs
- **Settings** - Application configuration
- **Profile** - Your account settings

### Collection View

When viewing a collection:

- **Records** tab - Browse data
- **API Rules** tab - Access control
- **Options** tab - Collection settings

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl/Cmd + K` | Quick search |
| `Ctrl/Cmd + N` | New record |
| `Escape` | Close modal |

## Theme

The Admin UI supports light and dark themes:

1. Click your profile icon
2. Select theme preference

## Superuser vs Regular Users

| Feature | Superuser | Regular User |
|---------|-----------|--------------|
| Access Admin UI | Yes | No |
| Manage collections | Yes | No |
| View all records | Yes | Based on rules |
| Configure settings | Yes | No |
| Create backups | Yes | No |

## Security

### Session Management

- Sessions expire after inactivity
- Force logout from settings
- OTP login available

### Access Control

- Admin UI requires superuser credentials
- All actions are logged
- HTTPS recommended in production

## Mobile Support

The Admin UI is responsive and works on mobile devices, though some features are optimized for desktop use.

## Customization

The Admin UI appearance can be customized:

### Hide Controls

Hide certain controls for non-technical users:

Settings > Application > Hide collection controls

### Custom Branding

Set application name displayed in Admin UI:

Settings > Application > Application name

## Troubleshooting

### Can't Access Admin UI

1. Check PocketBase is running
2. Verify correct URL (`/_/`)
3. Check firewall settings
4. Try incognito/private browsing

### Forgot Superuser Password

Reset via CLI:

```bash
./pocketbase superuser update admin@example.com newpassword
```

Or generate OTP:

```bash
./pocketbase superuser otp admin@example.com
```

### Session Expired

Log in again. Consider adjusting token duration in settings.

## Next Steps

- [Managing Collections](collections.md)
- [Managing Records](records.md)
- [Settings](settings.md)
