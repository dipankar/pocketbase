# API Overview

PocketBase provides a REST-ish API for interacting with your data. All responses are in JSON format.

## Base URL

```
http://127.0.0.1:8090/api/
```

## Authentication

Most endpoints require authentication. Include the auth token in the `Authorization` header:

```
Authorization: Bearer <token>
```

Tokens are obtained through the [authentication endpoints](authentication.md).

## Response Format

### Success Response

```json
{
  "page": 1,
  "perPage": 30,
  "totalItems": 100,
  "totalPages": 4,
  "items": [...]
}
```

### Single Item Response

```json
{
  "id": "abc123def456789",
  "collectionId": "xyz789abc123456",
  "collectionName": "posts",
  "created": "2024-01-01 12:00:00.000Z",
  "updated": "2024-01-01 12:00:00.000Z",
  "field1": "value1",
  "field2": "value2"
}
```

### Error Response

```json
{
  "code": 400,
  "message": "Something went wrong.",
  "data": {
    "field": {
      "code": "validation_required",
      "message": "Missing required value."
    }
  }
}
```

## HTTP Status Codes

| Code | Description |
|------|-------------|
| 200 | Success |
| 201 | Created |
| 204 | No Content (successful delete) |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 500 | Internal Server Error |

## Endpoint Categories

### Records

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/collections/{collection}/records` | List records |
| GET | `/api/collections/{collection}/records/{id}` | Get record |
| POST | `/api/collections/{collection}/records` | Create record |
| PATCH | `/api/collections/{collection}/records/{id}` | Update record |
| DELETE | `/api/collections/{collection}/records/{id}` | Delete record |

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/collections/{collection}/auth-with-password` | Login |
| POST | `/api/collections/{collection}/auth-refresh` | Refresh token |
| POST | `/api/collections/{collection}/auth-with-oauth2` | OAuth2 login |
| POST | `/api/collections/{collection}/request-verification` | Request email verification |
| POST | `/api/collections/{collection}/confirm-verification` | Confirm email |
| POST | `/api/collections/{collection}/request-password-reset` | Request password reset |
| POST | `/api/collections/{collection}/confirm-password-reset` | Confirm reset |

### Collections (Admin)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/collections` | List collections |
| GET | `/api/collections/{collection}` | Get collection |
| POST | `/api/collections` | Create collection |
| PATCH | `/api/collections/{collection}` | Update collection |
| DELETE | `/api/collections/{collection}` | Delete collection |

### Files

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/files/{collection}/{record}/{filename}` | Get file |

### Realtime

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST | `/api/realtime` | WebSocket connection |

### System

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/health` | Health check |
| GET | `/api/settings` | Get settings (admin) |
| PATCH | `/api/settings` | Update settings (admin) |
| GET | `/api/logs` | Get logs (admin) |
| GET | `/api/backups` | List backups (admin) |
| POST | `/api/backups` | Create backup (admin) |

## Query Parameters

### Pagination

| Parameter | Description | Default |
|-----------|-------------|---------|
| `page` | Page number | 1 |
| `perPage` | Items per page | 30 |

```
GET /api/collections/posts/records?page=2&perPage=50
```

### Sorting

| Parameter | Description |
|-----------|-------------|
| `sort` | Comma-separated fields (prefix with `-` for descending) |

```
GET /api/collections/posts/records?sort=-created,title
```

### Filtering

| Parameter | Description |
|-----------|-------------|
| `filter` | Filter expression |

```
GET /api/collections/posts/records?filter=(published=true && views>100)
```

### Filter Operators

| Operator | Description |
|----------|-------------|
| `=` | Equal |
| `!=` | Not equal |
| `>` | Greater than |
| `>=` | Greater or equal |
| `<` | Less than |
| `<=` | Less or equal |
| `~` | Contains (case-insensitive) |
| `!~` | Not contains |
| `?=` | Any equal (for arrays) |
| `?!=` | Any not equal |
| `?>` | Any greater than |
| `?<` | Any less than |

### Filter Examples

```
# Simple equality
filter=(status="active")

# Multiple conditions
filter=(status="active" && created>"2024-01-01")

# OR conditions
filter=(role="admin" || role="moderator")

# Contains (case-insensitive)
filter=(title~"hello")

# Nested (relation) filters
filter=(author.verified=true)

# Null checks
filter=(deletedAt=null)
```

### Field Selection

| Parameter | Description |
|-----------|-------------|
| `fields` | Comma-separated fields to return |

```
GET /api/collections/posts/records?fields=id,title,created
```

### Expanding Relations

| Parameter | Description |
|-----------|-------------|
| `expand` | Comma-separated relations to expand |

```
GET /api/collections/posts/records?expand=author,comments
```

## Request Headers

| Header | Description |
|--------|-------------|
| `Authorization` | Bearer token for authentication |
| `Content-Type` | `application/json` for JSON bodies |

## Rate Limiting

PocketBase doesn't have built-in rate limiting. Consider using a reverse proxy (nginx, Caddy) for production deployments.

## CORS

By default, PocketBase allows all origins. Configure with the `--origins` flag:

```bash
./pocketbase serve --origins=https://myapp.com
```

## Batch Operations

Execute multiple operations in a single request:

```bash
POST /api/batch
Content-Type: application/json

{
  "requests": [
    {
      "method": "POST",
      "url": "/api/collections/posts/records",
      "body": {"title": "Post 1"}
    },
    {
      "method": "POST",
      "url": "/api/collections/posts/records",
      "body": {"title": "Post 2"}
    }
  ]
}
```

## Next Steps

- [Authentication](authentication.md)
- [Collections](collections.md)
- [Records](records.md)
- [Realtime](realtime.md)
