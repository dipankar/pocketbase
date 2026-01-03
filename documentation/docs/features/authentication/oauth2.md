# OAuth2 Authentication

PocketBase supports OAuth2 authentication with popular identity providers.

## Supported Providers

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

## Setting Up OAuth2

### 1. Create OAuth App with Provider

Each provider requires creating an OAuth application:

**Google:**

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing
3. Navigate to APIs & Services > Credentials
4. Create OAuth 2.0 Client ID
5. Add authorized redirect URI: `https://yourdomain.com/api/oauth2-redirect`

**GitHub:**

1. Go to GitHub Settings > Developer settings > OAuth Apps
2. Create new OAuth App
3. Set Authorization callback URL: `https://yourdomain.com/api/oauth2-redirect`

**Facebook:**

1. Go to [Facebook Developers](https://developers.facebook.com/)
2. Create new app
3. Add Facebook Login product
4. Set Valid OAuth Redirect URIs

### 2. Configure Provider in PocketBase

In Admin UI:

1. Go to Settings > Auth providers
2. Enable desired provider
3. Enter Client ID and Client Secret
4. Save

Or via API:

```bash
PATCH /api/collections/users
Authorization: Bearer <superuser_token>
Content-Type: application/json

{
  "oauth2": {
    "enabled": true,
    "providers": [
      {
        "name": "google",
        "clientId": "your_client_id.apps.googleusercontent.com",
        "clientSecret": "your_client_secret"
      },
      {
        "name": "github",
        "clientId": "your_github_client_id",
        "clientSecret": "your_github_client_secret"
      }
    ]
  }
}
```

## OAuth2 Flow

### 1. Get Auth Methods

```bash
GET /api/collections/users/auth-methods
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
      "state": "random_state_string",
      "codeChallenge": "pkce_challenge",
      "codeChallengeMethod": "S256",
      "authUrl": "https://accounts.google.com/o/oauth2/auth?..."
    },
    {
      "name": "github",
      "displayName": "GitHub",
      "state": "random_state_string",
      "codeChallenge": "pkce_challenge",
      "codeChallengeMethod": "S256",
      "authUrl": "https://github.com/login/oauth/authorize?..."
    }
  ]
}
```

### 2. Redirect User to Provider

Redirect the user to the `authUrl` from the response.

### 3. Handle Callback

After user authorizes, provider redirects back with a code:

```
https://yourdomain.com/callback?code=authorization_code&state=state_string
```

### 4. Exchange Code for Token

```bash
POST /api/collections/users/auth-with-oauth2
Content-Type: application/json

{
  "provider": "google",
  "code": "authorization_code_from_callback",
  "codeVerifier": "pkce_code_verifier_from_step_1",
  "redirectUrl": "https://yourdomain.com/callback"
}
```

**Response:**

```json
{
  "token": "pocketbase_auth_token",
  "record": {
    "id": "user123",
    "email": "user@gmail.com",
    "verified": true,
    "name": "John Doe",
    ...
  },
  "meta": {
    "id": "google_user_id",
    "name": "John Doe",
    "email": "user@gmail.com",
    "avatarUrl": "https://...",
    "isNew": true
  }
}
```

## JavaScript SDK Implementation

```javascript
import PocketBase from 'pocketbase';

const pb = new PocketBase('http://127.0.0.1:8090');

// Get available auth methods
const authMethods = await pb.collection('users').listAuthMethods();

// OAuth2 with popup/redirect
async function loginWithGoogle() {
    const authData = await pb.collection('users').authWithOAuth2({
        provider: 'google',
        // Optional: specify scopes
        scopes: ['email', 'profile'],
        // Optional: create user data if new
        createData: {
            name: 'auto-filled from oauth'
        }
    });

    console.log('Logged in:', authData.record);
    console.log('Is new user:', authData.meta.isNew);
}

// OAuth2 with manual redirect handling
async function loginWithGitHub() {
    // Store verifier for later
    const verifier = pb.collection('users').authWithOAuth2CodeChallenge();

    // Get auth URL
    const authMethods = await pb.collection('users').listAuthMethods();
    const githubProvider = authMethods.authProviders.find(p => p.name === 'github');

    // Redirect user
    window.location.href = githubProvider.authUrl;
}

// Handle OAuth callback (on callback page)
async function handleOAuthCallback() {
    const params = new URLSearchParams(window.location.search);
    const code = params.get('code');
    const state = params.get('state');

    // Exchange code for token
    const authData = await pb.collection('users').authWithOAuth2Code(
        'github',
        code,
        localStorage.getItem('pkce_verifier'),
        window.location.origin + '/callback'
    );

    console.log('Logged in:', authData.record);
}
```

## Provider-Specific Configuration

### Google

```json
{
  "name": "google",
  "clientId": "xxx.apps.googleusercontent.com",
  "clientSecret": "xxx"
}
```

Scopes: `email`, `profile`

### GitHub

```json
{
  "name": "github",
  "clientId": "xxx",
  "clientSecret": "xxx"
}
```

Scopes: `read:user`, `user:email`

### Apple

Apple requires additional configuration:

```json
{
  "name": "apple",
  "clientId": "com.yourapp.id",
  "clientSecret": "generated_jwt_token",
  "teamId": "your_team_id",
  "keyId": "your_key_id"
}
```

### Generic OIDC

For any OpenID Connect compatible provider:

```json
{
  "name": "oidc",
  "displayName": "Corporate SSO",
  "clientId": "xxx",
  "clientSecret": "xxx",
  "authUrl": "https://sso.company.com/authorize",
  "tokenUrl": "https://sso.company.com/token",
  "userApiUrl": "https://sso.company.com/userinfo"
}
```

## Linking OAuth Accounts

Link additional OAuth providers to existing account:

```bash
POST /api/collections/users/auth-with-oauth2
Authorization: Bearer <existing_user_token>
Content-Type: application/json

{
  "provider": "github",
  "code": "authorization_code",
  "codeVerifier": "pkce_verifier",
  "redirectUrl": "https://yourdomain.com/callback"
}
```

## Unlinking OAuth Accounts

```bash
DELETE /api/collections/users/external-auths/{provider}
Authorization: Bearer <token>
```

## List Linked Accounts

```bash
GET /api/collections/users/records/{id}/external-auths
Authorization: Bearer <token>
```

**Response:**

```json
[
  {
    "id": "ext_auth_id",
    "provider": "google",
    "providerId": "google_user_id",
    "created": "2024-01-01 12:00:00.000Z",
    "updated": "2024-01-01 12:00:00.000Z"
  }
]
```

## Handling New vs Existing Users

Check `meta.isNew` in the response:

```javascript
const authData = await pb.collection('users').authWithOAuth2({
    provider: 'google'
});

if (authData.meta.isNew) {
    // New user - redirect to profile setup
    window.location.href = '/setup-profile';
} else {
    // Existing user - redirect to dashboard
    window.location.href = '/dashboard';
}
```

## Pre-populating User Data

Create user with OAuth data:

```javascript
const authData = await pb.collection('users').authWithOAuth2({
    provider: 'google',
    createData: {
        name: '', // Will be filled from OAuth profile
        role: 'user',
        newsletter: true
    }
});
```

## Error Handling

Common OAuth errors:

| Error | Cause |
|-------|-------|
| `invalid_state` | State mismatch (CSRF protection) |
| `invalid_code` | Authorization code expired or invalid |
| `access_denied` | User denied permission |
| `invalid_client` | Wrong client ID/secret |

```javascript
try {
    await pb.collection('users').authWithOAuth2({ provider: 'google' });
} catch (error) {
    if (error.data?.code === 'access_denied') {
        console.log('User cancelled login');
    } else {
        console.error('OAuth error:', error);
    }
}
```

## Next Steps

- [Email Verification](email-verification.md)
- [Password Authentication](password-auth.md)
- [Authentication Overview](overview.md)
