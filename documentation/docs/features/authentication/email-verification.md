# Email Verification

Email verification ensures users own the email addresses they register with.

## How It Works

1. User registers with email
2. Verification email is sent automatically (if configured)
3. User clicks link in email
4. Email is marked as verified

## Request Verification Email

```bash
POST /api/collections/users/request-verification
Content-Type: application/json

{
  "email": "user@example.com"
}
```

Returns `204 No Content` (doesn't reveal if email exists).

## Confirm Verification

```bash
POST /api/collections/users/confirm-verification
Content-Type: application/json

{
  "token": "verification_token_from_email"
}
```

**Response:**

```json
{
  "token": "new_auth_token",
  "record": {
    "id": "user123",
    "email": "user@example.com",
    "verified": true,
    ...
  }
}
```

## Configuration

### SMTP Settings

Configure email delivery in Admin UI > Settings > Mail settings:

| Setting | Description |
|---------|-------------|
| Sender name | Display name in emails |
| Sender address | From email address |
| SMTP host | Mail server hostname |
| SMTP port | Mail server port |
| SMTP username | Auth username |
| SMTP password | Auth password |
| TLS encryption | Enable TLS/SSL |

### Test Email Configuration

```bash
POST /api/settings/test/email
Authorization: Bearer <superuser_token>
Content-Type: application/json

{
  "email": "test@example.com",
  "template": "verification"
}
```

## Email Templates

Customize verification email templates in Admin UI > Settings > Mail templates.

### Available Templates

| Template | Purpose |
|----------|---------|
| Verification | Email address verification |
| Password Reset | Password reset requests |
| Email Change | Email address change confirmation |

### Template Variables

| Variable | Description |
|----------|-------------|
| `{APP_NAME}` | Application name |
| `{APP_URL}` | Application URL |
| `{TOKEN}` | Verification token |
| `{ACTION_URL}` | Complete verification URL |
| `{RECORD:fieldname}` | Record field value |

### Example Template

```html
<p>Hello {RECORD:name},</p>

<p>Click the link below to verify your email address:</p>

<p><a href="{ACTION_URL}">Verify Email</a></p>

<p>If you didn't create an account with {APP_NAME}, you can ignore this email.</p>

<p>Thanks,<br>{APP_NAME} Team</p>
```

## Requiring Verification

### Require for Login

Configure collection to only allow verified users:

```json
{
  "onlyVerified": true
}
```

Unverified users receive 403 error on login attempt.

### Require in API Rules

Use `@request.auth.verified` in rules:

```
# Only verified users can create posts
@request.auth.verified = true

# Only verified users can access sensitive data
@request.auth.id != "" && @request.auth.verified = true
```

## Auto-Send on Registration

Configure automatic verification email:

```javascript
// pb_hooks/send_verification.pb.js
onRecordAfterCreateSuccess((e) => {
    if (e.collection.name === 'users' && !e.record.verified) {
        $app.sendRecordVerification(e.record);
    }
});
```

## Resend Verification

Allow users to request new verification email:

```javascript
// Frontend
async function resendVerification(email) {
    try {
        await pb.collection('users').requestVerification(email);
        alert('Verification email sent!');
    } catch (error) {
        console.error('Error:', error);
    }
}
```

## Handle Verification Callback

```javascript
// On verification callback page
async function handleVerification() {
    const params = new URLSearchParams(window.location.search);
    const token = params.get('token');

    if (!token) {
        alert('Invalid verification link');
        return;
    }

    try {
        await pb.collection('users').confirmVerification(token);
        alert('Email verified successfully!');
        window.location.href = '/login';
    } catch (error) {
        alert('Verification failed: ' + error.message);
    }
}
```

## Check Verification Status

```javascript
// Check if current user is verified
const isVerified = pb.authStore.model?.verified;

if (!isVerified) {
    showVerificationBanner();
}
```

## Re-verification on Email Change

When users change their email, they must verify the new address:

```bash
# Request email change
POST /api/collections/users/request-email-change
Authorization: Bearer <token>
Content-Type: application/json

{
  "newEmail": "newemail@example.com"
}

# Confirm email change (from email link)
POST /api/collections/users/confirm-email-change
Content-Type: application/json

{
  "token": "email_change_token",
  "password": "current_password"
}
```

## Best Practices

1. **Always require verification** - Enable `onlyVerified` for auth collections
2. **Clear messaging** - Tell users to check spam folder
3. **Resend option** - Allow users to request new verification
4. **Token expiration** - Verification tokens expire after a period
5. **Rate limiting** - Limit verification email requests
6. **Secure templates** - Don't include sensitive data in emails

## Troubleshooting

### Emails Not Sending

1. Check SMTP settings are correct
2. Verify sender email is valid
3. Check spam folder
4. Use test email endpoint
5. Check server logs for errors

### Token Expired

Verification tokens expire. User must request new verification email.

### Already Verified

If user is already verified, verification request returns success but no email is sent.

## JavaScript SDK Example

```javascript
import PocketBase from 'pocketbase';

const pb = new PocketBase('http://127.0.0.1:8090');

// Register and send verification
async function register(email, password, name) {
    // Create user
    const user = await pb.collection('users').create({
        email,
        password,
        passwordConfirm: password,
        name
    });

    // Send verification email
    await pb.collection('users').requestVerification(email);

    return user;
}

// Check verification status
function checkVerification() {
    if (pb.authStore.isValid && !pb.authStore.model.verified) {
        return {
            verified: false,
            message: 'Please verify your email address'
        };
    }
    return { verified: true };
}

// Resend verification
async function resendVerification() {
    const email = pb.authStore.model?.email;
    if (email) {
        await pb.collection('users').requestVerification(email);
    }
}

// Confirm verification from token
async function confirmVerification(token) {
    return await pb.collection('users').confirmVerification(token);
}
```

## Next Steps

- [Password Authentication](password-auth.md)
- [OAuth2 Setup](oauth2.md)
- [Email Templates](../email-templates.md)
