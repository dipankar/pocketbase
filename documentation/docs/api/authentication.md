# Authentication API

PocketBase provides multiple authentication methods for auth collections.

## Auth Collections

Auth collections are special collections with built-in authentication fields:

- `email` - User email address
- `password` - Hashed password (write-only)
- `verified` - Email verification status
- `tokenKey` - Internal auth token key

## Password Authentication

### Login

```bash
POST /api/collections/{collection}/auth-with-password
Content-Type: application/json

{
  "identity": "user@example.com",
  "password": "secretpassword"
}
```

**Response:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "record": {
    "id": "abc123def456789",
    "email": "user@example.com",
    "verified": true,
    "created": "2024-01-01 12:00:00.000Z",
    "updated": "2024-01-01 12:00:00.000Z"
  }
}
```

The `identity` field accepts either email or username (if username field exists).

### Register

```bash
POST /api/collections/{collection}/records
Content-Type: application/json

{
  "email": "newuser@example.com",
  "password": "secretpassword",
  "passwordConfirm": "secretpassword",
  "name": "John Doe"
}
```

## Token Refresh

Refresh an auth token before it expires:

```bash
POST /api/collections/{collection}/auth-refresh
Authorization: Bearer <current_token>
```

**Response:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "record": {...}
}
```

## OAuth2 Authentication

### List Providers

```bash
GET /api/collections/{collection}/auth-methods
```

**Response:**

```json
{
  "usernamePassword": true,
  "emailPassword": true,
  "onlyVerified": false,
  "authProviders": [
    {
      "name": "google",
      "displayName": "Google",
      "state": "abc123...",
      "authUrl": "https://accounts.google.com/o/oauth2/auth?..."
    },
    {
      "name": "github",
      "displayName": "GitHub",
      "state": "def456...",
      "authUrl": "https://github.com/login/oauth/authorize?..."
    }
  ]
}
```

### OAuth2 Login

```bash
POST /api/collections/{collection}/auth-with-oauth2
Content-Type: application/json

{
  "provider": "google",
  "code": "oauth_authorization_code",
  "codeVerifier": "pkce_code_verifier",
  "redirectUrl": "https://myapp.com/oauth-callback"
}
```

**Response:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "record": {...},
  "meta": {
    "id": "oauth_user_id",
    "name": "John Doe",
    "email": "john@example.com",
    "avatarUrl": "https://...",
    "isNew": false
  }
}
```

### Supported OAuth2 Providers

- Google
- Facebook
- GitHub
- GitLab
- Discord
- Twitter
- Microsoft
- Spotify
- Kakao
- Twitch
- Strava
- LiveChat
- Apple
- Instagram
- VK
- Yandex
- Patreon
- Mailcow
- Bitbucket
- OIDC (generic OpenID Connect)

## Email Verification

### Request Verification Email

```bash
POST /api/collections/{collection}/request-verification
Content-Type: application/json

{
  "email": "user@example.com"
}
```

### Confirm Verification

```bash
POST /api/collections/{collection}/confirm-verification
Content-Type: application/json

{
  "token": "verification_token_from_email"
}
```

## Password Reset

### Request Reset Email

```bash
POST /api/collections/{collection}/request-password-reset
Content-Type: application/json

{
  "email": "user@example.com"
}
```

### Confirm Reset

```bash
POST /api/collections/{collection}/confirm-password-reset
Content-Type: application/json

{
  "token": "reset_token_from_email",
  "password": "newpassword",
  "passwordConfirm": "newpassword"
}
```

## Email Change

### Request Email Change

```bash
POST /api/collections/{collection}/request-email-change
Authorization: Bearer <token>
Content-Type: application/json

{
  "newEmail": "newemail@example.com"
}
```

### Confirm Email Change

```bash
POST /api/collections/{collection}/confirm-email-change
Content-Type: application/json

{
  "token": "email_change_token",
  "password": "current_password"
}
```

## One-Time Password (OTP)

### Request OTP

```bash
POST /api/collections/{collection}/request-otp
Content-Type: application/json

{
  "email": "user@example.com"
}
```

### Authenticate with OTP

```bash
POST /api/collections/{collection}/auth-with-otp
Content-Type: application/json

{
  "otpId": "otp_request_id",
  "password": "123456"
}
```

## Multi-Factor Authentication (MFA)

When MFA is enabled, additional verification is required after primary authentication.

### Authenticate with MFA

```bash
POST /api/collections/{collection}/auth-with-password
Content-Type: application/json

{
  "identity": "user@example.com",
  "password": "secretpassword",
  "mfaId": "mfa_request_id",
  "mfaCode": "123456"
}
```

## Using Auth Tokens

Include the token in subsequent requests:

```bash
GET /api/collections/posts/records
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Token Structure

PocketBase uses JWT tokens containing:

- `id` - Record ID
- `type` - Token type (auth)
- `collectionId` - Collection ID
- `exp` - Expiration timestamp

### Token Expiration

Default token expiration is configurable per collection. Handle expired tokens by:

1. Catching 401 responses
2. Using refresh endpoint
3. Re-authenticating

## Impersonation (Admin)

Superusers can generate tokens for any user:

```bash
POST /api/collections/{collection}/impersonate/{record_id}
Authorization: Bearer <superuser_token>
```

## Unlinking OAuth2 Provider

```bash
DELETE /api/collections/{collection}/external-auths/{provider}
Authorization: Bearer <token>
```

## Example: Complete Auth Flow

```javascript
import PocketBase from 'pocketbase';

const pb = new PocketBase('http://127.0.0.1:8090');

// Register
const user = await pb.collection('users').create({
    email: 'user@example.com',
    password: 'secretpassword',
    passwordConfirm: 'secretpassword',
    name: 'John Doe'
});

// Request verification
await pb.collection('users').requestVerification('user@example.com');

// Login
const authData = await pb.collection('users').authWithPassword(
    'user@example.com',
    'secretpassword'
);

console.log(pb.authStore.token);
console.log(pb.authStore.model);

// Refresh token
await pb.collection('users').authRefresh();

// Logout
pb.authStore.clear();
```

## Security Considerations

1. **HTTPS** - Always use HTTPS in production
2. **Token Storage** - Store tokens securely (httpOnly cookies preferred)
3. **Password Requirements** - Enforce strong passwords
4. **Rate Limiting** - Implement rate limiting for auth endpoints
5. **Email Verification** - Require verification for sensitive operations

## Next Steps

- [Collections API](collections.md)
- [Records API](records.md)
- [OAuth2 Setup](../features/authentication/oauth2.md)
