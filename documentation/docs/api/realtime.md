# Realtime API

PocketBase provides realtime subscriptions via Server-Sent Events (SSE) for receiving database changes as they happen.

## Connection

Connect to the realtime endpoint:

```
GET /api/realtime
```

Or via POST for initial subscription:

```bash
POST /api/realtime
Content-Type: application/json

{
  "clientId": "optional_client_id"
}
```

**Response:**

```json
{
  "clientId": "abc123xyz789"
}
```

## Subscribing to Changes

After establishing connection, send subscription messages:

```json
{
  "clientId": "abc123xyz789",
  "subscriptions": ["posts", "comments"]
}
```

### Subscription Targets

| Target | Description |
|--------|-------------|
| `collection_name` | All records in collection |
| `collection_name/record_id` | Specific record |
| `collection_name/*` | All records (explicit) |

### Examples

```javascript
// Subscribe to all posts
"posts"

// Subscribe to specific post
"posts/abc123def456789"

// Subscribe to multiple collections
["posts", "comments", "users"]

// Subscribe to specific records
["posts/id1", "posts/id2", "comments/id3"]
```

## Event Format

Events are delivered as SSE messages:

```
event: PB_CONNECT
data: {"clientId":"abc123xyz789"}

event: posts
data: {"action":"create","record":{"id":"new123","title":"Hello",...}}

event: posts
data: {"action":"update","record":{"id":"new123","title":"Updated",...}}

event: posts
data: {"action":"delete","record":{"id":"new123",...}}
```

### Event Actions

| Action | Description |
|--------|-------------|
| `create` | New record created |
| `update` | Existing record updated |
| `delete` | Record deleted |

### Event Data Structure

```json
{
  "action": "create",
  "record": {
    "id": "abc123def456789",
    "collectionId": "xyz789abc123456",
    "collectionName": "posts",
    "created": "2024-01-01 12:00:00.000Z",
    "updated": "2024-01-01 12:00:00.000Z",
    "title": "New Post",
    "content": "..."
  }
}
```

## JavaScript SDK Usage

```javascript
import PocketBase from 'pocketbase';

const pb = new PocketBase('http://127.0.0.1:8090');

// Subscribe to collection changes
const unsubscribe = await pb.collection('posts').subscribe('*', (e) => {
    console.log('Action:', e.action);  // create, update, delete
    console.log('Record:', e.record);
});

// Subscribe to specific record
await pb.collection('posts').subscribe('RECORD_ID', (e) => {
    console.log('Record updated:', e.record);
});

// Subscribe with options
await pb.collection('posts').subscribe('*', (e) => {
    console.log(e);
}, {
    expand: 'author',
    filter: 'published = true'
});

// Unsubscribe from specific subscription
await pb.collection('posts').unsubscribe('*');

// Unsubscribe from all
await pb.collection('posts').unsubscribe();
```

## Vanilla JavaScript

```javascript
// Create EventSource connection
const eventSource = new EventSource('http://127.0.0.1:8090/api/realtime');

eventSource.addEventListener('PB_CONNECT', (e) => {
    const data = JSON.parse(e.data);
    console.log('Connected with client ID:', data.clientId);

    // Send subscription request
    fetch('http://127.0.0.1:8090/api/realtime', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            clientId: data.clientId,
            subscriptions: ['posts', 'comments']
        })
    });
});

eventSource.addEventListener('posts', (e) => {
    const data = JSON.parse(e.data);
    console.log('Posts event:', data.action, data.record);
});

eventSource.addEventListener('comments', (e) => {
    const data = JSON.parse(e.data);
    console.log('Comments event:', data.action, data.record);
});

eventSource.onerror = (e) => {
    console.error('SSE error:', e);
};

// Close connection
eventSource.close();
```

## Authentication

For authenticated subscriptions, include the auth token:

```javascript
// With SDK (automatic)
await pb.collection('users').authWithPassword('email', 'password');
await pb.collection('private_posts').subscribe('*', handler);

// Manual with headers
const eventSource = new EventSource(
    'http://127.0.0.1:8090/api/realtime',
    {
        headers: {
            'Authorization': 'Bearer ' + authToken
        }
    }
);
```

## Subscription Rules

Realtime subscriptions respect collection API rules:

- **List rule** applies to `*` (all records) subscriptions
- **View rule** applies to specific record subscriptions

If a user doesn't have permission to view a record, they won't receive events for it.

## Advanced Usage

### Filtered Subscriptions

Subscribe only to records matching specific criteria:

```javascript
// SDK
await pb.collection('posts').subscribe('*', handler, {
    filter: 'published = true && author = "user123"'
});
```

### Expanded Relations

Include related records in events:

```javascript
await pb.collection('posts').subscribe('*', handler, {
    expand: 'author,category'
});
```

### Multiple Collections

```javascript
// Subscribe to multiple collections
await pb.collection('posts').subscribe('*', postsHandler);
await pb.collection('comments').subscribe('*', commentsHandler);
await pb.collection('users').subscribe('*', usersHandler);
```

## Reconnection

The SDK handles reconnection automatically. For manual implementations:

```javascript
let eventSource;

function connect() {
    eventSource = new EventSource('http://127.0.0.1:8090/api/realtime');

    eventSource.onerror = () => {
        eventSource.close();
        setTimeout(connect, 1000); // Reconnect after 1 second
    };

    // Set up listeners...
}

connect();
```

## Best Practices

1. **Unsubscribe when done** - Clean up subscriptions to prevent memory leaks
2. **Handle reconnection** - Network issues will disconnect the SSE connection
3. **Use filters** - Only subscribe to data you need
4. **Consider pagination** - Use REST API for initial data, realtime for updates
5. **Debounce updates** - Multiple rapid updates may need client-side debouncing

## Limitations

- SSE is one-directional (server to client)
- Maximum connections per client may be limited by browser
- Large payloads should be avoided in frequent updates
- File changes don't trigger realtime events (only record metadata)

## Example: Live Chat

```javascript
import PocketBase from 'pocketbase';

const pb = new PocketBase('http://127.0.0.1:8090');

// Authenticate user
await pb.collection('users').authWithPassword('user@example.com', 'password');

// Load initial messages
const messages = await pb.collection('messages').getList(1, 50, {
    sort: '-created',
    expand: 'author'
});

// Subscribe to new messages
await pb.collection('messages').subscribe('*', (e) => {
    if (e.action === 'create') {
        // Add new message to UI
        addMessageToChat(e.record);
    } else if (e.action === 'update') {
        // Update existing message
        updateMessageInChat(e.record);
    } else if (e.action === 'delete') {
        // Remove message from UI
        removeMessageFromChat(e.record.id);
    }
}, {
    expand: 'author',
    filter: `room = "${currentRoomId}"`
});

// Send message
async function sendMessage(text) {
    await pb.collection('messages').create({
        text,
        author: pb.authStore.model.id,
        room: currentRoomId
    });
}

// Cleanup on page unload
window.addEventListener('beforeunload', () => {
    pb.collection('messages').unsubscribe();
});
```

## Next Steps

- [Records API](records.md)
- [Authentication](authentication.md)
- [Files API](files.md)
