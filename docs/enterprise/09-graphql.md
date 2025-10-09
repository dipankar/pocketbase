# GraphQL Layer

## Overview

Auto-generate a GraphQL API from PocketBase collections, providing a modern alternative to the REST API.

---

## Architecture

```
PocketBase Collections
         ↓
Schema Generator (analyze collection fields)
         ↓
GraphQL Schema (types, queries, mutations)
         ↓
Resolvers (map to PocketBase CRUD)
         ↓
DataLoader (prevent N+1 queries)
         ↓
GraphQL Server (gqlgen)
```

---

## Schema Generation

### Collection to GraphQL Type

**PocketBase Collection**:
```javascript
{
  "name": "posts",
  "type": "base",
  "schema": [
    {"name": "title", "type": "text", "required": true},
    {"name": "content", "type": "editor"},
    {"name": "published", "type": "bool"},
    {"name": "author", "type": "relation", "options": {"collection": "users"}},
    {"name": "tags", "type": "select", "options": {"values": ["tech", "news"]}}
  ]
}
```

**Generated GraphQL Type**:
```graphql
type Post {
  id: ID!
  created: DateTime!
  updated: DateTime!

  # Fields
  title: String!
  content: String
  published: Boolean

  # Relations
  author: User

  # Select field
  tags: [String!]
}

# List wrapper with pagination
type PostConnection {
  items: [Post!]!
  totalCount: Int!
  pageInfo: PageInfo!
}

type PageInfo {
  hasNextPage: Boolean!
  hasPreviousPage: Boolean!
  startCursor: String
  endCursor: String
}
```

---

## Field Type Mapping

| PocketBase Field | GraphQL Type | Notes |
|------------------|--------------|-------|
| `text` | `String` | |
| `editor` | `String` | Rich text stored as HTML |
| `number` | `Float` | |
| `bool` | `Boolean` | |
| `email` | `String` | Validated email |
| `url` | `String` | Validated URL |
| `date` | `DateTime` | ISO 8601 format |
| `select` (single) | `String` | Enum if values defined |
| `select` (multiple) | `[String!]` | |
| `file` (single) | `File` | Custom scalar |
| `file` (multiple) | `[File!]` | |
| `relation` (single) | `Type` | Resolved to related type |
| `relation` (multiple) | `[Type!]` | |
| `json` | `JSON` | Custom scalar |

---

## Queries

### List Query

```graphql
type Query {
  posts(
    # Pagination
    page: Int
    perPage: Int

    # Filtering
    filter: String

    # Sorting
    sort: String

    # Expansion (load relations)
    expand: [String!]
  ): PostConnection!
}
```

**Example Request**:
```graphql
query {
  posts(
    page: 1,
    perPage: 20,
    filter: "published = true",
    sort: "-created",
    expand: ["author"]
  ) {
    items {
      id
      title
      content
      author {
        id
        name
        email
      }
    }
    totalCount
    pageInfo {
      hasNextPage
    }
  }
}
```

**Resolver Implementation**:
```go
// core/graphql/resolvers/query.go

func (r *queryResolver) Posts(ctx context.Context, args PostsArgs) (*PostConnection, error) {
    tenant := getTenantFromContext(ctx)

    // Build PocketBase query
    records, err := tenant.App.FindRecordsByFilter(
        "posts",
        args.Filter,
        args.Sort,
        args.PerPage,
        (args.Page-1)*args.PerPage,
        args.Expand...,
    )
    if err != nil {
        return nil, err
    }

    // Convert to GraphQL types
    items := make([]*Post, len(records))
    for i, record := range records {
        items[i] = recordToPost(record)
    }

    // Count total
    total, _ := tenant.App.CountRecords("posts", args.Filter)

    return &PostConnection{
        Items:      items,
        TotalCount: total,
        PageInfo: &PageInfo{
            HasNextPage: total > args.Page*args.PerPage,
        },
    }, nil
}
```

---

### Get Query (Single Record)

```graphql
type Query {
  post(id: ID!): Post
}
```

**Example**:
```graphql
query {
  post(id: "abc123xyz") {
    id
    title
    author {
      name
    }
  }
}
```

---

## Mutations

### Create

```graphql
type Mutation {
  createPost(input: CreatePostInput!): Post!
}

input CreatePostInput {
  title: String!
  content: String
  published: Boolean
  author: ID
  tags: [String!]
}
```

**Example**:
```graphql
mutation {
  createPost(input: {
    title: "Hello GraphQL",
    content: "This is a test post",
    published: true,
    author: "user123"
  }) {
    id
    title
    created
  }
}
```

**Resolver**:
```go
func (r *mutationResolver) CreatePost(ctx context.Context, input CreatePostInput) (*Post, error) {
    tenant := getTenantFromContext(ctx)
    collection, _ := tenant.App.FindCachedCollectionByNameOrId("posts")

    // Create record
    record := core.NewRecord(collection)
    record.Set("title", input.Title)
    record.Set("content", input.Content)
    record.Set("published", input.Published)
    record.Set("author", input.Author)
    record.Set("tags", input.Tags)

    // Validate and save
    if err := tenant.App.Save(record); err != nil {
        return nil, err
    }

    return recordToPost(record), nil
}
```

---

### Update

```graphql
type Mutation {
  updatePost(id: ID!, input: UpdatePostInput!): Post!
}

input UpdatePostInput {
  title: String
  content: String
  published: Boolean
  author: ID
  tags: [String!]
}
```

---

### Delete

```graphql
type Mutation {
  deletePost(id: ID!): Boolean!
}
```

---

## Subscriptions (Real-time)

```graphql
type Subscription {
  postCreated(filter: String): Post!
  postUpdated(filter: String): Post!
  postDeleted(filter: String): ID!
}
```

**Example**:
```graphql
subscription {
  postCreated(filter: "published = true") {
    id
    title
    author {
      name
    }
  }
}
```

**Resolver using PocketBase subscriptions**:
```go
func (r *subscriptionResolver) PostCreated(ctx context.Context, filter *string) (<-chan *Post, error) {
    tenant := getTenantFromContext(ctx)
    ch := make(chan *Post)

    // Subscribe to PocketBase events
    unsubscribe := tenant.App.OnRecordCreate("posts").BindFunc(func(e *core.RecordEvent) error {
        // Check filter
        if filter != nil {
            match, _ := tenant.App.CanAccessRecord(e.Record, *filter)
            if !match {
                return nil
            }
        }

        // Send to channel
        select {
        case ch <- recordToPost(e.Record):
        case <-ctx.Done():
            return ctx.Err()
        }

        return nil
    })

    // Cleanup on context cancel
    go func() {
        <-ctx.Done()
        unsubscribe()
        close(ch)
    }()

    return ch, nil
}
```

---

## DataLoader (N+1 Prevention)

**Problem**: Without DataLoader, loading posts with authors causes N+1 queries:
```
1 query for posts
+ N queries for each author (one per post)
= N+1 queries
```

**Solution**: Batch and cache author queries

```go
// core/graphql/dataloader/user_loader.go

type UserLoader struct {
    tenant *TenantInstance
    cache  map[string]*User
    mu     sync.RWMutex
}

func (ul *UserLoader) Load(ctx context.Context, id string) (*User, error) {
    // Check cache
    ul.mu.RLock()
    if user, ok := ul.cache[id]; ok {
        ul.mu.RUnlock()
        return user, nil
    }
    ul.mu.RUnlock()

    // Load from database
    record, err := ul.tenant.App.FindRecordById("users", id)
    if err != nil {
        return nil, err
    }

    user := recordToUser(record)

    // Cache
    ul.mu.Lock()
    ul.cache[id] = user
    ul.mu.Unlock()

    return user, nil
}

func (ul *UserLoader) LoadMany(ctx context.Context, ids []string) ([]*User, error) {
    // Batch load from database
    records, err := ul.tenant.App.FindRecordsByIds("users", ids)
    if err != nil {
        return nil, err
    }

    users := make([]*User, len(records))
    for i, record := range records {
        user := recordToUser(record)
        users[i] = user

        // Cache
        ul.mu.Lock()
        ul.cache[record.Id] = user
        ul.mu.Unlock()
    }

    return users, nil
}
```

**Usage in Resolver**:
```go
func (r *postResolver) Author(ctx context.Context, obj *Post) (*User, error) {
    loader := getDataLoader(ctx)
    return loader.UserLoader.Load(ctx, obj.AuthorID)
}
```

---

## Authentication

### Header-based

```
Authorization: Bearer {jwt_token}
```

### Cookie-based

```
Cookie: pb_auth={jwt_token}
```

**Resolver with Auth**:
```go
func (r *mutationResolver) CreatePost(ctx context.Context, input CreatePostInput) (*Post, error) {
    tenant := getTenantFromContext(ctx)

    // Get authenticated user
    authRecord := getAuthRecordFromContext(ctx)
    if authRecord == nil {
        return nil, errors.New("unauthorized")
    }

    // Check permissions
    canCreate := tenant.App.CanAccessRecord(authRecord, "posts:create")
    if !canCreate {
        return nil, errors.New("forbidden")
    }

    // ... create post
}
```

---

## Permissions

GraphQL respects PocketBase collection rules:

```javascript
// Collection rule
{
  "create": "@request.auth.id != ''",  // Authenticated users only
  "update": "@request.auth.id = author", // Only author can update
  "delete": "@request.auth.verified = true" // Only verified users
}
```

These translate to GraphQL field-level permissions:

```go
func (r *mutationResolver) UpdatePost(ctx context.Context, id string, input UpdatePostInput) (*Post, error) {
    tenant := getTenantFromContext(ctx)
    authRecord := getAuthRecordFromContext(ctx)

    // Load existing post
    record, _ := tenant.App.FindRecordById("posts", id)

    // Check update rule
    canUpdate := tenant.App.CanAccessRecord(record, authRecord, "update")
    if !canUpdate {
        return nil, errors.New("forbidden: not authorized to update this post")
    }

    // ... perform update
}
```

---

## Error Handling

```json
{
  "errors": [
    {
      "message": "Record not found",
      "path": ["post"],
      "extensions": {
        "code": "NOT_FOUND",
        "collection": "posts",
        "id": "invalid_id"
      }
    }
  ],
  "data": {
    "post": null
  }
}
```

---

## GraphQL Playground

Embed GraphiQL for API exploration:

```
GET /api/graphql/playground
```

Features:
- Schema explorer
- Query autocomplete
- Documentation
- Query history

---

## Performance Optimizations

### 1. Query Complexity Limit

Prevent expensive queries:

```go
complexity := calculateComplexity(query)
if complexity > 1000 {
    return errors.New("query too complex")
}
```

### 2. Depth Limit

Prevent deeply nested queries:

```go
depth := calculateDepth(query)
if depth > 5 {
    return errors.New("query too deep")
}
```

### 3. Pagination Limits

```go
if args.PerPage > 100 {
    args.PerPage = 100
}
```

### 4. Field-level Caching

```go
// Cache expensive field calculations
@cacheControl(maxAge: 60)
```

---

## Implementation Checklist

- [ ] Schema generator from collections
- [ ] Query resolvers (list, get)
- [ ] Mutation resolvers (create, update, delete)
- [ ] Subscription resolvers (real-time)
- [ ] DataLoader (N+1 prevention)
- [ ] Authentication integration
- [ ] Permission enforcement
- [ ] Error handling
- [ ] GraphQL Playground
- [ ] Performance limits (complexity, depth)
- [ ] Field-level caching
- [ ] Documentation generation

---

## Example: Complete Schema

```graphql
# Generated schema for a blog

scalar DateTime
scalar JSON
scalar Upload

type User {
  id: ID!
  created: DateTime!
  updated: DateTime!
  name: String!
  email: String!
  avatar: String
  verified: Boolean!

  # Relations
  posts: [Post!]!
}

type Post {
  id: ID!
  created: DateTime!
  updated: DateTime!
  title: String!
  content: String
  published: Boolean!

  # Relations
  author: User!
  comments: [Comment!]!

  # Computed
  commentCount: Int!
}

type Comment {
  id: ID!
  created: DateTime!
  updated: DateTime!
  message: String!

  # Relations
  post: Post!
  author: User!
}

type Query {
  # Users
  users(page: Int, perPage: Int, filter: String, sort: String): UserConnection!
  user(id: ID!): User

  # Posts
  posts(page: Int, perPage: Int, filter: String, sort: String): PostConnection!
  post(id: ID!): Post

  # Comments
  comments(page: Int, perPage: Int, filter: String, sort: String): CommentConnection!
  comment(id: ID!): Comment
}

type Mutation {
  # Users
  createUser(input: CreateUserInput!): User!
  updateUser(id: ID!, input: UpdateUserInput!): User!
  deleteUser(id: ID!): Boolean!

  # Posts
  createPost(input: CreatePostInput!): Post!
  updatePost(id: ID!, input: UpdatePostInput!): Post!
  deletePost(id: ID!): Boolean!

  # Comments
  createComment(input: CreateCommentInput!): Comment!
  updateComment(id: ID!, input: UpdateCommentInput!): Comment!
  deleteComment(id: ID!): Boolean!
}

type Subscription {
  postCreated(filter: String): Post!
  postUpdated(filter: String): Post!
  postDeleted(filter: String): ID!
}

# ... input types, connections, etc.
```

---

## Next: Security

See [10-security.md](10-security.md) for MFA, RBAC, and compliance features.
