# Quick Start

Get PocketBase up and running in 5 minutes.

## 1. Start the Server

```bash
./pocketbase serve
```

The server starts at `http://127.0.0.1:8090` by default.

## 2. Create Superuser Account

On first visit to the Admin UI (`http://127.0.0.1:8090/_/`), you'll be prompted to create a superuser account.

Alternatively, create one via CLI:

```bash
./pocketbase superuser create admin@example.com yourpassword
```

## 3. Create Your First Collection

1. Open the Admin UI at `http://127.0.0.1:8090/_/`
2. Log in with your superuser credentials
3. Click **New collection**
4. Name it `posts`
5. Add fields:
   - `title` (Text, required)
   - `content` (Editor)
   - `published` (Bool)
6. Click **Create**

## 4. Add API Rules

By default, collections have no API access. Configure rules to allow access:

1. Select your `posts` collection
2. Go to **API Rules** tab
3. Set rules:
   - **List/Search**: Leave empty for public access, or use `@request.auth.id != ""`
   - **View**: Same as above
   - **Create**: `@request.auth.id != ""` (authenticated users only)
   - **Update**: `@request.auth.id != ""` (authenticated users only)
   - **Delete**: `@request.auth.id != ""` (authenticated users only)

## 5. Create Records

### Via Admin UI

1. Click on your collection
2. Click **New record**
3. Fill in the fields
4. Click **Create**

### Via API

```bash
curl -X POST http://127.0.0.1:8090/api/collections/posts/records \
  -H "Content-Type: application/json" \
  -d '{"title": "Hello World", "content": "<p>My first post!</p>", "published": true}'
```

## 6. Query Records

### List All Records

```bash
curl http://127.0.0.1:8090/api/collections/posts/records
```

Response:

```json
{
  "page": 1,
  "perPage": 30,
  "totalItems": 1,
  "totalPages": 1,
  "items": [
    {
      "id": "abc123def456789",
      "collectionId": "xyz789abc123456",
      "collectionName": "posts",
      "created": "2024-01-01 12:00:00.000Z",
      "updated": "2024-01-01 12:00:00.000Z",
      "title": "Hello World",
      "content": "<p>My first post!</p>",
      "published": true
    }
  ]
}
```

### Filter Records

```bash
curl "http://127.0.0.1:8090/api/collections/posts/records?filter=(published=true)"
```

### Get Single Record

```bash
curl http://127.0.0.1:8090/api/collections/posts/records/abc123def456789
```

## 7. Add Authentication

Create an auth collection for users:

1. Click **New collection**
2. Select **Auth collection** type
3. Name it `users`
4. Add any additional fields (e.g., `name`)
5. Click **Create**

### Register a User

```bash
curl -X POST http://127.0.0.1:8090/api/collections/users/records \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "securepassword123", "passwordConfirm": "securepassword123"}'
```

### Authenticate

```bash
curl -X POST http://127.0.0.1:8090/api/collections/users/auth-with-password \
  -H "Content-Type: application/json" \
  -d '{"identity": "user@example.com", "password": "securepassword123"}'
```

Response includes an auth token:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "record": {
    "id": "user123abc456789",
    "email": "user@example.com",
    ...
  }
}
```

### Make Authenticated Requests

```bash
curl http://127.0.0.1:8090/api/collections/posts/records \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

## 8. Realtime Subscriptions

Subscribe to record changes via WebSocket:

```javascript
import PocketBase from 'pocketbase';

const pb = new PocketBase('http://127.0.0.1:8090');

// Subscribe to changes in the posts collection
pb.collection('posts').subscribe('*', function (e) {
    console.log(e.action); // create, update, delete
    console.log(e.record);
});

// Subscribe to a specific record
pb.collection('posts').subscribe('RECORD_ID', function (e) {
    console.log(e.record);
});

// Unsubscribe
pb.collection('posts').unsubscribe('*');
```

## Example: Simple Blog API

Here's a complete example of a blog API setup:

### Collections

1. **users** (Auth collection)
   - `name` (Text)

2. **posts** (Base collection)
   - `title` (Text, required)
   - `content` (Editor)
   - `author` (Relation to users)
   - `published` (Bool)
   - `publishedAt` (DateTime)

3. **comments** (Base collection)
   - `post` (Relation to posts)
   - `author` (Relation to users)
   - `content` (Text)

### API Rules for Posts

- **List**: `published = true || @request.auth.id = author.id`
- **View**: `published = true || @request.auth.id = author.id`
- **Create**: `@request.auth.id != ""`
- **Update**: `@request.auth.id = author.id`
- **Delete**: `@request.auth.id = author.id`

## Next Steps

- [Configuration Options](configuration.md)
- [API Reference](../api/overview.md)
- [Authentication Guide](../features/authentication/overview.md)
- [Extending PocketBase](../development/extending-go.md)
