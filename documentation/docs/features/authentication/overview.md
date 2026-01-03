# Authentication Overview

PocketBase provides a complete authentication system for auth collections.

## Auth Collections

Auth collections are special collections with built-in authentication capabilities. They include:

- Email/password authentication
- OAuth2 integration
- Email verification
- Password reset
- Multi-factor authentication (MFA)
- One-time passwords (OTP)

## Creating an Auth Collection

1. In the Admin UI, click **New collection**
2. Select **Auth collection** type
3. Name your collection (e.g., `users`)
4. Add any custom fields
5. Configure auth options

Or via API:

```bash
POST /api/collections
Content-Type: application/json

{
  "name": "users",
  "type": "auth",
  "schema": [
    {"name": "name", "type": "text"},
    {"name": "avatar", "type": "file", "options": {"maxSelect": 1}}
  ]
}
```

## Auth Collection Fields

Auth collections automatically include:

| Field | Type | Description |
|-------|------|-------------|
| `email` | string | User email address |
| `emailVisibility` | bool | Whether email is publicly visible |
| `verified` | bool | Email verification status |
| `password` | string | Hashed password (write-only) |
| `tokenKey` | string | Token generation key (internal) |

## Authentication Methods

### Password Authentication

Traditional email/password login:

```bash
POST /api/collections/users/auth-with-password
Content-Type: application/json

{
  "identity": "user@example.com",
  "password": "securepassword"
}
```

### OAuth2

Social login via providers like Google, GitHub, etc.:

```bash
POST /api/collections/users/auth-with-oauth2
Content-Type: application/json

{
  "provider": "google",
  "code": "authorization_code",
  "codeVerifier": "pkce_verifier",
  "redirectUrl": "https://myapp.com/callback"
}
```

### OTP (One-Time Password)

Email-based one-time passwords:

```bash
# Request OTP
POST /api/collections/users/request-otp
Content-Type: application/json

{"email": "user@example.com"}

# Authenticate with OTP
POST /api/collections/users/auth-with-otp
Content-Type: application/json

{"otpId": "otp_request_id", "password": "123456"}
```

## Auth Configuration

Configure auth options per collection:

### Auth Methods

```json
{
  "authToken": {
    "duration": 1209600
  },
  "passwordAuth": {
    "enabled": true,
    "identityFields": ["email", "username"]
  },
  "oauth2": {
    "enabled": true,
    "providers": [...]
  },
  "mfa": {
    "enabled": false,
    "duration": 1800
  },
  "otp": {
    "enabled": true,
    "duration": 180
  }
}
```

### Token Duration

Configure how long auth tokens remain valid:

- Default: 14 days (1209600 seconds)
- Set to 0 for no expiration
- Shorter durations for higher security

### Identity Fields

Fields that can be used for login (default: email):

```json
{
  "passwordAuth": {
    "identityFields": ["email", "username"]
  }
}
```

## Auth Response

Successful authentication returns:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "record": {
    "id": "user123abc456789",
    "collectionId": "users_collection_id",
    "collectionName": "users",
    "email": "user@example.com",
    "verified": true,
    "created": "2024-01-01 12:00:00.000Z",
    "updated": "2024-01-01 12:00:00.000Z"
  }
}
```

## Using Auth Tokens

Include the token in the `Authorization` header:

```bash
GET /api/collections/posts/records
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

## Auth Rules

Use `@request.auth` in API rules to reference the authenticated user:

```
# Only authenticated users
@request.auth.id != ""

# Only verified users
@request.auth.verified = true

# Only the record owner
@request.auth.id = user

# Specific role
@request.auth.role = "admin"

# Combination
@request.auth.verified = true && @request.auth.role = "editor"
```

## Multiple Auth Collections

You can have multiple auth collections for different user types:

- `users` - Regular users
- `admins` - Admin users
- `customers` - Customer accounts

Each collection has independent:
- Auth settings
- OAuth2 providers
- Email templates
- Token duration

## Security Best Practices

1. **Use HTTPS** - Always use HTTPS in production
2. **Require verification** - Enable email verification for sensitive operations
3. **Strong passwords** - Set minimum password requirements
4. **Token refresh** - Implement token refresh before expiration
5. **Secure storage** - Store tokens in httpOnly cookies when possible
6. **Rate limiting** - Add rate limiting via reverse proxy
7. **Enable MFA** - Enable MFA for high-security applications

## JavaScript SDK Example

```javascript
import PocketBase from 'pocketbase';

const pb = new PocketBase('http://127.0.0.1:8090');

// Register
const user = await pb.collection('users').create({
    email: 'user@example.com',
    password: 'securepassword',
    passwordConfirm: 'securepassword',
    name: 'John Doe'
});

// Login
const authData = await pb.collection('users').authWithPassword(
    'user@example.com',
    'securepassword'
);

// Check auth state
console.log(pb.authStore.isValid);
console.log(pb.authStore.token);
console.log(pb.authStore.model);

// Refresh token
await pb.collection('users').authRefresh();

// Logout
pb.authStore.clear();

// Auth state change listener
pb.authStore.onChange((token, model) => {
    console.log('Auth changed:', token, model);
});
```

## Next Steps

- [Password Authentication](password-auth.md)
- [OAuth2 Setup](oauth2.md)
- [Email Verification](email-verification.md)
