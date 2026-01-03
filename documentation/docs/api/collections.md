# Collections API

Collections define the structure of your data. This API is primarily for superusers/admins.

## Collection Types

| Type | Description |
|------|-------------|
| `base` | Standard data collection |
| `auth` | Authentication-enabled collection |
| `view` | Read-only collection based on SQL query |

## List Collections

```bash
GET /api/collections
Authorization: Bearer <superuser_token>
```

**Query Parameters:**

| Parameter | Description |
|-----------|-------------|
| `page` | Page number |
| `perPage` | Items per page |
| `sort` | Sort field |
| `filter` | Filter expression |

**Response:**

```json
{
  "page": 1,
  "perPage": 30,
  "totalItems": 5,
  "totalPages": 1,
  "items": [
    {
      "id": "abc123def456789",
      "name": "posts",
      "type": "base",
      "system": false,
      "schema": [...],
      "listRule": "",
      "viewRule": "",
      "createRule": "@request.auth.id != ''",
      "updateRule": "@request.auth.id = author",
      "deleteRule": "@request.auth.id = author",
      "created": "2024-01-01 12:00:00.000Z",
      "updated": "2024-01-01 12:00:00.000Z"
    }
  ]
}
```

## Get Collection

```bash
GET /api/collections/{collectionIdOrName}
Authorization: Bearer <superuser_token>
```

**Response:**

```json
{
  "id": "abc123def456789",
  "name": "posts",
  "type": "base",
  "system": false,
  "schema": [
    {
      "id": "field1id",
      "name": "title",
      "type": "text",
      "required": true,
      "options": {
        "min": null,
        "max": 200,
        "pattern": ""
      }
    },
    {
      "id": "field2id",
      "name": "content",
      "type": "editor",
      "required": false,
      "options": {}
    }
  ],
  "listRule": "",
  "viewRule": "",
  "createRule": "@request.auth.id != ''",
  "updateRule": "@request.auth.id = author",
  "deleteRule": "@request.auth.id = author",
  "indexes": [
    "CREATE INDEX idx_posts_author ON posts (author)"
  ],
  "created": "2024-01-01 12:00:00.000Z",
  "updated": "2024-01-01 12:00:00.000Z"
}
```

## Create Collection

```bash
POST /api/collections
Authorization: Bearer <superuser_token>
Content-Type: application/json

{
  "name": "posts",
  "type": "base",
  "schema": [
    {
      "name": "title",
      "type": "text",
      "required": true,
      "options": {
        "max": 200
      }
    },
    {
      "name": "content",
      "type": "editor"
    },
    {
      "name": "author",
      "type": "relation",
      "options": {
        "collectionId": "users_collection_id",
        "maxSelect": 1,
        "cascadeDelete": false
      }
    },
    {
      "name": "published",
      "type": "bool"
    }
  ],
  "listRule": "published = true || @request.auth.id = author",
  "viewRule": "published = true || @request.auth.id = author",
  "createRule": "@request.auth.id != ''",
  "updateRule": "@request.auth.id = author",
  "deleteRule": "@request.auth.id = author"
}
```

## Update Collection

```bash
PATCH /api/collections/{collectionIdOrName}
Authorization: Bearer <superuser_token>
Content-Type: application/json

{
  "name": "articles",
  "schema": [
    {
      "id": "existing_field_id",
      "name": "title",
      "type": "text",
      "required": true
    },
    {
      "name": "subtitle",
      "type": "text"
    }
  ],
  "updateRule": "@request.auth.id = author && @request.auth.verified = true"
}
```

!!! warning "Schema Updates"
    Removing or renaming fields will delete the associated data. Always backup before making schema changes.

## Delete Collection

```bash
DELETE /api/collections/{collectionIdOrName}
Authorization: Bearer <superuser_token>
```

Returns `204 No Content` on success.

!!! danger "Irreversible"
    Deleting a collection permanently removes all its records and cannot be undone.

## Collection Schema

### Field Properties

| Property | Description |
|----------|-------------|
| `id` | Field ID (generated, use when updating) |
| `name` | Field name (alphanumeric + underscore) |
| `type` | Field type |
| `required` | Whether field is required |
| `options` | Type-specific options |

### Field Types and Options

#### text

```json
{
  "name": "title",
  "type": "text",
  "options": {
    "min": 1,
    "max": 200,
    "pattern": "^[a-zA-Z]+$"
  }
}
```

#### number

```json
{
  "name": "price",
  "type": "number",
  "options": {
    "min": 0,
    "max": 99999,
    "noDecimal": false
  }
}
```

#### bool

```json
{
  "name": "active",
  "type": "bool"
}
```

#### email

```json
{
  "name": "contactEmail",
  "type": "email",
  "options": {
    "exceptDomains": ["spam.com"],
    "onlyDomains": []
  }
}
```

#### url

```json
{
  "name": "website",
  "type": "url",
  "options": {
    "exceptDomains": [],
    "onlyDomains": []
  }
}
```

#### date

```json
{
  "name": "publishDate",
  "type": "date",
  "options": {
    "min": "2024-01-01",
    "max": "2030-12-31"
  }
}
```

#### select

```json
{
  "name": "status",
  "type": "select",
  "options": {
    "maxSelect": 1,
    "values": ["draft", "published", "archived"]
  }
}
```

#### relation

```json
{
  "name": "author",
  "type": "relation",
  "options": {
    "collectionId": "users_collection_id",
    "maxSelect": 1,
    "cascadeDelete": false,
    "minSelect": null
  }
}
```

#### file

```json
{
  "name": "images",
  "type": "file",
  "options": {
    "maxSelect": 5,
    "maxSize": 5242880,
    "mimeTypes": ["image/jpeg", "image/png", "image/gif"],
    "thumbs": ["100x100", "400x0"]
  }
}
```

#### json

```json
{
  "name": "metadata",
  "type": "json",
  "options": {
    "maxSize": 2000000
  }
}
```

#### editor

```json
{
  "name": "content",
  "type": "editor",
  "options": {
    "convertUrls": false
  }
}
```

#### autodate

```json
{
  "name": "lastModified",
  "type": "autodate",
  "options": {
    "onCreate": true,
    "onUpdate": true
  }
}
```

## API Rules

Rules use a filter-like syntax to control access:

### Rule Variables

| Variable | Description |
|----------|-------------|
| `@request.auth` | Current authenticated user record |
| `@request.auth.id` | Current user ID |
| `@request.data` | Submitted form data |
| `@collection` | Access other collections |

### Rule Examples

```
# Public access
(leave empty)

# Authenticated users only
@request.auth.id != ""

# Verified users only
@request.auth.verified = true

# Owner only
@request.auth.id = user

# Owner or admin
@request.auth.id = user || @request.auth.role = "admin"

# Check submitted data
@request.data.status != "published" || @request.auth.role = "editor"

# Check against other collections
@request.auth.id = @collection.teams.members.id
```

## View Collections

Create read-only collections from SQL queries:

```bash
POST /api/collections
Authorization: Bearer <superuser_token>
Content-Type: application/json

{
  "name": "posts_stats",
  "type": "view",
  "options": {
    "query": "SELECT posts.id, posts.title, COUNT(comments.id) as comment_count FROM posts LEFT JOIN comments ON comments.post = posts.id GROUP BY posts.id"
  },
  "listRule": "",
  "viewRule": ""
}
```

## Import/Export

### Export Collections

```bash
GET /api/collections?fields=*
Authorization: Bearer <superuser_token>
```

### Import Collections

Use the create/update endpoints to recreate collections.

## Next Steps

- [Records API](records.md)
- [Fields Reference](../features/fields.md)
- [Authorization Rules](../features/authorization.md)
