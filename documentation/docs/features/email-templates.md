# Email Templates

PocketBase provides customizable email templates for authentication-related emails.

## Available Templates

| Template | Trigger |
|----------|---------|
| Verification | `requestVerification()` |
| Password Reset | `requestPasswordReset()` |
| Email Change | `requestEmailChange()` |

## Configuring Templates

### Via Admin UI

1. Go to **Settings** > **Mail settings**
2. Configure SMTP settings
3. Scroll to **Mail templates**
4. Edit each template as needed

### Via API

```bash
PATCH /api/settings
Authorization: Bearer <superuser_token>
Content-Type: application/json

{
  "smtp": {
    "enabled": true,
    "host": "smtp.example.com",
    "port": 587,
    "username": "user@example.com",
    "password": "password",
    "tls": true
  },
  "meta": {
    "senderName": "My App",
    "senderAddress": "noreply@example.com",
    "verificationTemplate": {
      "subject": "Verify your email",
      "body": "<p>Click to verify: {ACTION_URL}</p>"
    },
    "resetPasswordTemplate": {
      "subject": "Reset your password",
      "body": "<p>Click to reset: {ACTION_URL}</p>"
    },
    "confirmEmailChangeTemplate": {
      "subject": "Confirm email change",
      "body": "<p>Click to confirm: {ACTION_URL}</p>"
    }
  }
}
```

## Template Variables

### Common Variables

| Variable | Description |
|----------|-------------|
| `{APP_NAME}` | Application name |
| `{APP_URL}` | Application URL |
| `{TOKEN}` | Raw verification/reset token |
| `{ACTION_URL}` | Complete action URL with token |

### Record Variables

Access any field from the user record:

| Variable | Description |
|----------|-------------|
| `{RECORD:id}` | User ID |
| `{RECORD:email}` | User email |
| `{RECORD:name}` | Custom name field |
| `{RECORD:fieldname}` | Any custom field |

## Example Templates

### Verification Email

**Subject:**
```
Verify your {APP_NAME} account
```

**Body:**
```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Email Verification</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
    <div style="max-width: 600px; margin: 0 auto; padding: 20px;">
        <h2 style="color: #2563eb;">Welcome to {APP_NAME}!</h2>

        <p>Hi {RECORD:name},</p>

        <p>Thank you for signing up. Please verify your email address by clicking the button below:</p>

        <p style="text-align: center; margin: 30px 0;">
            <a href="{ACTION_URL}"
               style="background-color: #2563eb; color: white; padding: 12px 24px;
                      text-decoration: none; border-radius: 4px; display: inline-block;">
                Verify Email Address
            </a>
        </p>

        <p style="color: #666; font-size: 14px;">
            Or copy and paste this link into your browser:<br>
            <a href="{ACTION_URL}" style="color: #2563eb;">{ACTION_URL}</a>
        </p>

        <p>If you didn't create an account with {APP_NAME}, you can safely ignore this email.</p>

        <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">

        <p style="color: #999; font-size: 12px;">
            This email was sent by {APP_NAME}.<br>
            <a href="{APP_URL}" style="color: #999;">{APP_URL}</a>
        </p>
    </div>
</body>
</html>
```

### Password Reset Email

**Subject:**
```
Reset your {APP_NAME} password
```

**Body:**
```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Password Reset</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
    <div style="max-width: 600px; margin: 0 auto; padding: 20px;">
        <h2 style="color: #dc2626;">Password Reset Request</h2>

        <p>Hi {RECORD:name},</p>

        <p>We received a request to reset the password for your {APP_NAME} account.</p>

        <p style="text-align: center; margin: 30px 0;">
            <a href="{ACTION_URL}"
               style="background-color: #dc2626; color: white; padding: 12px 24px;
                      text-decoration: none; border-radius: 4px; display: inline-block;">
                Reset Password
            </a>
        </p>

        <p style="color: #666; font-size: 14px;">
            This link will expire in 1 hour.
        </p>

        <p><strong>Didn't request this?</strong></p>
        <p>If you didn't request a password reset, you can ignore this email.
           Your password won't be changed.</p>

        <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">

        <p style="color: #999; font-size: 12px;">
            This email was sent by {APP_NAME}.<br>
            <a href="{APP_URL}" style="color: #999;">{APP_URL}</a>
        </p>
    </div>
</body>
</html>
```

### Email Change Confirmation

**Subject:**
```
Confirm your new email address
```

**Body:**
```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Confirm Email Change</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
    <div style="max-width: 600px; margin: 0 auto; padding: 20px;">
        <h2 style="color: #059669;">Confirm Email Change</h2>

        <p>Hi {RECORD:name},</p>

        <p>You requested to change your email address. Click the button below to confirm this change:</p>

        <p style="text-align: center; margin: 30px 0;">
            <a href="{ACTION_URL}"
               style="background-color: #059669; color: white; padding: 12px 24px;
                      text-decoration: none; border-radius: 4px; display: inline-block;">
                Confirm New Email
            </a>
        </p>

        <p style="color: #666; font-size: 14px;">
            If you didn't request this change, please contact support immediately.
        </p>

        <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">

        <p style="color: #999; font-size: 12px;">
            This email was sent by {APP_NAME}.<br>
            <a href="{APP_URL}" style="color: #999;">{APP_URL}</a>
        </p>
    </div>
</body>
</html>
```

## SMTP Configuration

### Common SMTP Providers

**Gmail:**
```json
{
  "host": "smtp.gmail.com",
  "port": 587,
  "tls": true
}
```

Note: Gmail requires an "App Password" for SMTP access.

**SendGrid:**
```json
{
  "host": "smtp.sendgrid.net",
  "port": 587,
  "username": "apikey",
  "password": "your_sendgrid_api_key",
  "tls": true
}
```

**Mailgun:**
```json
{
  "host": "smtp.mailgun.org",
  "port": 587,
  "tls": true
}
```

**Amazon SES:**
```json
{
  "host": "email-smtp.us-east-1.amazonaws.com",
  "port": 587,
  "tls": true
}
```

### Testing Email Configuration

```bash
POST /api/settings/test/email
Authorization: Bearer <superuser_token>
Content-Type: application/json

{
  "email": "test@example.com",
  "template": "verification"
}
```

## Custom Action URLs

By default, `{ACTION_URL}` points to:
```
{APP_URL}/_/#/auth/confirm-verification?token={TOKEN}
```

To use custom frontend URLs, you can:

1. Set custom `APP_URL` to your frontend
2. Handle the token parameter in your frontend
3. Call the confirmation API endpoint

Example frontend handler:

```javascript
// Frontend route: /verify-email?token=xxx
const token = new URLSearchParams(window.location.search).get('token');

async function verifyEmail() {
    await pb.collection('users').confirmVerification(token);
    // Redirect to success page
}
```

## Plain Text Fallback

For email clients that don't support HTML, include plain text:

```html
<!-- HTML version -->
<p>Click to verify: <a href="{ACTION_URL}">Verify Email</a></p>

<!-- The link URL will be visible for plain text rendering -->
```

## Localization

For multi-language support, you can:

1. Store user language preference
2. Use hooks to send custom templates
3. Or create multiple apps for different regions

### Using Hooks

```javascript
// pb_hooks/custom_emails.pb.js
onMailerRecordVerificationSend((e) => {
    const record = e.record;
    const lang = record.get('language') || 'en';

    if (lang === 'es') {
        e.message.subject = 'Verifica tu correo';
        e.message.html = `<p>Hola ${record.get('name')}, haz clic aquí: ${e.meta.actionUrl}</p>`;
    }
});
```

## Best Practices

1. **Keep it simple** - Don't overdesign email templates
2. **Test across clients** - Test in Gmail, Outlook, Apple Mail
3. **Mobile-friendly** - Use responsive design
4. **Clear CTAs** - Make action buttons prominent
5. **Include plain text** - For accessibility
6. **Avoid spam triggers** - Don't use all caps, excessive punctuation
7. **Brand consistently** - Match your app's look and feel

## Troubleshooting

### Emails Not Sending

1. Check SMTP credentials
2. Verify sender address is valid
3. Check spam folder
4. Review server logs
5. Test with `/api/settings/test/email`

### Emails Going to Spam

- Use authenticated SMTP
- Set up SPF, DKIM, DMARC records
- Use consistent sender address
- Avoid spam trigger words
- Include unsubscribe option

### Template Not Rendering

- Check variable syntax: `{VARIABLE_NAME}`
- Ensure record fields exist
- Test with simple template first

## Next Steps

- [Email Verification](authentication/email-verification.md)
- [SMTP Setup](../guides/production.md)
- [Hooks & Events](../development/hooks-events.md)
