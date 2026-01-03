# Records API

Records are the data entries within collections. Access is controlled by collection API rules.

## List Records

```bash
GET /api/collections/{collection}/records
```

**Query Parameters:**

| Parameter | Description | Default |
|-----------|-------------|---------|
| `page` | Page number | 1 |
| `perPage` | Items per page (max 500) | 30 |
| `sort` | Sort fields | - |
| `filter` | Filter expression | - |
| `expand` | Relations to expand | - |
| `fields` | Fields to return | * |
| `skipTotal` | Skip total count for performance | false |

**Response:**

```json
{
  "page": 1,
  "perPage": 30,
  "totalItems": 100,
  "totalPages": 4,
  "items": [
    {
      "id": "abc123def456789",
      "collectionId": "xyz789abc123456",
      "collectionName": "posts",
      "created": "2024-01-01 12:00:00.000Z",
      "updated": "2024-01-01 12:00:00.000Z",
      "title": "Hello World",
      "content": "<p>My first post</p>",
      "author": "user123abc456789"
    }
  ]
}
```

### Sorting

Sort by one or more fields. Prefix with `-` for descending order:

```
# Sort by created descending
?sort=-created

# Multiple sort fields
?sort=-created,title

# Sort by relation field
?sort=author.name
```

### Filtering

Use filter expressions to query records:

```
# Basic equality
?filter=(status="active")

# Comparison operators
?filter=(views>100)
?filter=(price>=10 && price<=50)

# String contains (case-insensitive)
?filter=(title~"hello")

# NOT contains
?filter=(title!~"spam")

# NULL checks
?filter=(deletedAt=null)
?filter=(deletedAt!=null)

# OR conditions
?filter=(status="draft" || status="review")

# AND conditions
?filter=(status="active" && featured=true)

# Nested relation filters
?filter=(author.verified=true)
?filter=(author.role="admin")

# Array field contains
?filter=(tags?="javascript")

# Complex expressions
?filter=((status="active" && views>100) || featured=true)
```

### Expanding Relations

Include related records in the response:

```
# Single relation
?expand=author

# Multiple relations
?expand=author,category

# Nested relations (up to 6 levels)
?expand=author.profile,comments.author

# Back-relations
?expand=comments_via_post
```

**Response with expanded relations:**

```json
{
  "id": "post123",
  "title": "Hello World",
  "author": "user123",
  "expand": {
    "author": {
      "id": "user123",
      "name": "John Doe",
      "email": "john@example.com"
    }
  }
}
```

### Field Selection

Return only specific fields:

```
# Specific fields
?fields=id,title,created

# All fields
?fields=*

# Include expanded relation fields
?fields=*,expand.author.name
```

## Get Record

```bash
GET /api/collections/{collection}/records/{id}
```

**Query Parameters:**

| Parameter | Description |
|-----------|-------------|
| `expand` | Relations to expand |
| `fields` | Fields to return |

**Response:**

```json
{
  "id": "abc123def456789",
  "collectionId": "xyz789abc123456",
  "collectionName": "posts",
  "created": "2024-01-01 12:00:00.000Z",
  "updated": "2024-01-01 12:00:00.000Z",
  "title": "Hello World",
  "content": "<p>My first post</p>",
  "author": "user123abc456789"
}
```

## Create Record

```bash
POST /api/collections/{collection}/records
Content-Type: application/json

{
  "title": "New Post",
  "content": "<p>Post content here</p>",
  "author": "user123abc456789",
  "published": true
}
```

### With File Upload

```bash
POST /api/collections/{collection}/records
Content-Type: multipart/form-data

--boundary
Content-Disposition: form-data; name="title"

New Post
--boundary
Content-Disposition: form-data; name="image"; filename="photo.jpg"
Content-Type: image/jpeg

<binary data>
--boundary--
```

**Response:**

```json
{
  "id": "newrecord123456",
  "collectionId": "xyz789abc123456",
  "collectionName": "posts",
  "created": "2024-01-15 10:30:00.000Z",
  "updated": "2024-01-15 10:30:00.000Z",
  "title": "New Post",
  "content": "<p>Post content here</p>",
  "author": "user123abc456789",
  "published": true,
  "image": "photo_abc123.jpg"
}
```

## Update Record

```bash
PATCH /api/collections/{collection}/records/{id}
Content-Type: application/json

{
  "title": "Updated Title",
  "published": false
}
```

Only include fields you want to update. Omitted fields remain unchanged.

### Append to Array Fields

For multi-select or multi-file fields, use `+` to append:

```json
{
  "tags+": ["newtag"]
}
```

### Remove from Array Fields

Use `-` to remove values:

```json
{
  "tags-": ["oldtag"]
}
```

### Delete Files

Set file fields to empty or use `-`:

```json
{
  "image": ""
}
```

Or remove specific files:

```json
{
  "images-": ["file1.jpg"]
}
```

## Delete Record

```bash
DELETE /api/collections/{collection}/records/{id}
```

Returns `204 No Content` on success.

## Batch Operations

Execute multiple operations in a transaction:

```bash
POST /api/batch
Content-Type: application/json

{
  "requests": [
    {
      "method": "POST",
      "url": "/api/collections/posts/records",
      "body": {
        "title": "Post 1"
      }
    },
    {
      "method": "POST",
      "url": "/api/collections/posts/records",
      "body": {
        "title": "Post 2"
      }
    },
    {
      "method": "PATCH",
      "url": "/api/collections/posts/records/existingid",
      "body": {
        "published": true
      }
    }
  ]
}
```

**Response:**

```json
[
  {
    "status": 200,
    "body": {"id": "new1", "title": "Post 1", ...}
  },
  {
    "status": 200,
    "body": {"id": "new2", "title": "Post 2", ...}
  },
  {
    "status": 200,
    "body": {"id": "existingid", "published": true, ...}
  }
]
```

## Auth Records

Auth collection records have additional fields and endpoints.

### Create Auth Record

```bash
POST /api/collections/users/records
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "secretpassword",
  "passwordConfirm": "secretpassword",
  "name": "John Doe"
}
```

### Update Auth Record

```bash
PATCH /api/collections/users/records/{id}
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Jane Doe",
  "oldPassword": "currentpassword",
  "password": "newpassword",
  "passwordConfirm": "newpassword"
}
```

Password changes require `oldPassword` (for self-update) or superuser token.

## Error Responses

### Validation Error

```json
{
  "code": 400,
  "message": "Failed to create record.",
  "data": {
    "title": {
      "code": "validation_required",
      "message": "Missing required value."
    },
    "email": {
      "code": "validation_invalid_email",
      "message": "Must be a valid email address."
    }
  }
}
```

### Not Found

```json
{
  "code": 404,
  "message": "The requested resource wasn't found.",
  "data": {}
}
```

### Forbidden

```json
{
  "code": 403,
  "message": "You are not allowed to perform this request.",
  "data": {}
}
```

## Performance Tips

1. **Use `skipTotal`** - Skip total count when you don't need pagination info
2. **Limit fields** - Only request fields you need with `?fields=`
3. **Index frequently filtered fields** - Add database indexes
4. **Limit expand depth** - Deep expansions can be slow
5. **Use batch operations** - Reduce HTTP overhead for multiple operations

## Examples

### JavaScript SDK

```javascript
import PocketBase from 'pocketbase';

const pb = new PocketBase('http://127.0.0.1:8090');

// List with filters
const posts = await pb.collection('posts').getList(1, 20, {
    filter: 'published = true && created >= "2024-01-01"',
    sort: '-created',
    expand: 'author'
});

// Get single record
const post = await pb.collection('posts').getOne('recordid', {
    expand: 'author,comments'
});

// Create record
const newPost = await pb.collection('posts').create({
    title: 'Hello',
    content: 'World',
    published: true
});

// Update record
await pb.collection('posts').update('recordid', {
    title: 'Updated Title'
});

// Delete record
await pb.collection('posts').delete('recordid');
```

### cURL

```bash
# List records
curl "http://127.0.0.1:8090/api/collections/posts/records?filter=(published=true)&sort=-created"

# Create record
curl -X POST http://127.0.0.1:8090/api/collections/posts/records \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"title": "Hello", "published": true}'

# Update record
curl -X PATCH http://127.0.0.1:8090/api/collections/posts/records/recordid \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"title": "Updated"}'

# Delete record
curl -X DELETE http://127.0.0.1:8090/api/collections/posts/records/recordid \
  -H "Authorization: Bearer <token>"
```

## Next Steps

- [Realtime API](realtime.md)
- [Files API](files.md)
- [Authentication](authentication.md)
