# Password Authentication

Email/password authentication is the most common authentication method.

## Registration

Create a new user account:

```bash
POST /api/collections/users/records
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "securepassword123",
  "passwordConfirm": "securepassword123",
  "name": "John Doe"
}
```

**Response:**

```json
{
  "id": "user123abc456789",
  "collectionId": "users_collection_id",
  "collectionName": "users",
  "email": "user@example.com",
  "emailVisibility": false,
  "verified": false,
  "created": "2024-01-01 12:00:00.000Z",
  "updated": "2024-01-01 12:00:00.000Z",
  "name": "John Doe"
}
```

## Login

Authenticate with email/password:

```bash
POST /api/collections/users/auth-with-password
Content-Type: application/json

{
  "identity": "user@example.com",
  "password": "securepassword123"
}
```

**Response:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "record": {
    "id": "user123abc456789",
    "email": "user@example.com",
    "verified": false,
    "name": "John Doe",
    ...
  }
}
```

## Username Login

If you have a `username` field and configure it as an identity field:

```bash
POST /api/collections/users/auth-with-password
Content-Type: application/json

{
  "identity": "johndoe",
  "password": "securepassword123"
}
```

### Configure Identity Fields

In Admin UI or via API:

```json
{
  "passwordAuth": {
    "enabled": true,
    "identityFields": ["email", "username"]
  }
}
```

## Token Refresh

Refresh the auth token before it expires:

```bash
POST /api/collections/users/auth-refresh
Authorization: Bearer <current_token>
```

**Response:**

```json
{
  "token": "new_token...",
  "record": {...}
}
```

## Password Change

Update password for authenticated user:

```bash
PATCH /api/collections/users/records/{id}
Authorization: Bearer <token>
Content-Type: application/json

{
  "oldPassword": "currentpassword",
  "password": "newpassword123",
  "passwordConfirm": "newpassword123"
}
```

## Password Reset

### Request Reset Email

```bash
POST /api/collections/users/request-password-reset
Content-Type: application/json

{
  "email": "user@example.com"
}
```

Returns `204 No Content` (doesn't reveal if email exists).

### Confirm Reset

Use the token from the reset email:

```bash
POST /api/collections/users/confirm-password-reset
Content-Type: application/json

{
  "token": "reset_token_from_email",
  "password": "newpassword123",
  "passwordConfirm": "newpassword123"
}
```

## Password Requirements

Configure password validation in collection settings:

```json
{
  "schema": [
    {
      "name": "password",
      "type": "password",
      "options": {
        "min": 8,
        "max": 72,
        "pattern": ""
      }
    }
  ]
}
```

### Password Validation Options

| Option | Description |
|--------|-------------|
| `min` | Minimum length (recommended: 8+) |
| `max` | Maximum length (max: 72 for bcrypt) |
| `pattern` | Regex pattern for requirements |

### Pattern Examples

```
# Require uppercase, lowercase, number
^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,}$

# Require special character
^(?=.*[!@#$%^&*]).{8,}$
```

## Restricting Registration

### Disable Public Registration

Set create rule to restrict registration:

```
# No public registration (admin only)
@request.auth.role = "admin"

# Require invite code
@request.data.inviteCode = "valid_code"
```

### Email Domain Restriction

Use email field options:

```json
{
  "name": "email",
  "type": "email",
  "options": {
    "onlyDomains": ["company.com"]
  }
}
```

## Verification Requirement

Require verified email for login:

Configure in collection auth options:

```json
{
  "onlyVerified": true
}
```

Unverified users will receive a 403 error on login.

## Account Lockout

PocketBase doesn't have built-in account lockout. Implement via:

1. **Reverse proxy rate limiting** (recommended)
2. **Custom hook** tracking failed attempts

### Custom Hook Example

```javascript
// pb_hooks/auth_limiter.pb.js
onRecordAuthRequest((e) => {
    const ip = e.httpContext.request().remoteAddr;
    const attempts = getFailedAttempts(ip);

    if (attempts >= 5) {
        throw new ForbiddenError('Too many failed attempts');
    }
});

onRecordAuthFailure((e) => {
    const ip = e.httpContext.request().remoteAddr;
    incrementFailedAttempts(ip);
});
```

## JavaScript SDK Examples

```javascript
import PocketBase from 'pocketbase';

const pb = new PocketBase('http://127.0.0.1:8090');

// Register new user
async function register(email, password, name) {
    return await pb.collection('users').create({
        email,
        password,
        passwordConfirm: password,
        name
    });
}

// Login
async function login(email, password) {
    return await pb.collection('users').authWithPassword(email, password);
}

// Check if logged in
function isLoggedIn() {
    return pb.authStore.isValid;
}

// Get current user
function getCurrentUser() {
    return pb.authStore.model;
}

// Logout
function logout() {
    pb.authStore.clear();
}

// Change password
async function changePassword(oldPassword, newPassword) {
    return await pb.collection('users').update(pb.authStore.model.id, {
        oldPassword,
        password: newPassword,
        passwordConfirm: newPassword
    });
}

// Request password reset
async function requestPasswordReset(email) {
    return await pb.collection('users').requestPasswordReset(email);
}

// Confirm password reset
async function confirmPasswordReset(token, newPassword) {
    return await pb.collection('users').confirmPasswordReset(
        token,
        newPassword,
        newPassword
    );
}

// Persist auth across page reloads
pb.authStore.onChange((token, model) => {
    // Auth state changed
    console.log('Auth changed:', model?.email);
});
```

## Security Checklist

- [ ] Use HTTPS in production
- [ ] Set minimum password length (8+ characters)
- [ ] Enable email verification
- [ ] Implement rate limiting
- [ ] Use secure password storage (automatic with PocketBase)
- [ ] Configure proper CORS origins
- [ ] Set appropriate token duration

## Next Steps

- [OAuth2 Authentication](oauth2.md)
- [Email Verification](email-verification.md)
- [Authentication Overview](overview.md)
