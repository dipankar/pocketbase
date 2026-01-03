# Authorization & Rules

PocketBase uses a flexible rule system to control access to your data.

## API Rules

Each collection has five configurable rules:

| Rule | Description |
|------|-------------|
| **List** | Who can list/search multiple records |
| **View** | Who can view a single record |
| **Create** | Who can create new records |
| **Update** | Who can modify existing records |
| **Delete** | Who can delete records |

## Rule Syntax

Rules use a filter expression syntax. An empty rule means **public access**.

### Basic Examples

```
# Public access (anyone)
(leave empty)

# No access (admin only)
null

# Authenticated users only
@request.auth.id != ""

# Specific user
@request.auth.id = "user123"

# Owner only
@request.auth.id = user

# Verified users only
@request.auth.verified = true
```

## Rule Variables

### @request.auth

The currently authenticated user:

| Variable | Description |
|----------|-------------|
| `@request.auth.id` | User ID |
| `@request.auth.collectionId` | Auth collection ID |
| `@request.auth.collectionName` | Auth collection name |
| `@request.auth.verified` | Email verification status |
| `@request.auth.email` | User email |
| `@request.auth.<field>` | Any custom auth field |

### @request.data

The submitted request data (create/update):

| Variable | Description |
|----------|-------------|
| `@request.data.field` | Field value being submitted |
| `@request.data.<field>:isset` | Check if field is present |
| `@request.data.<field>:length` | Length of array/string |

### @request.headers

Request headers:

| Variable | Description |
|----------|-------------|
| `@request.headers.x_custom` | Custom header value |

### @collection

Reference other collections:

| Variable | Description |
|----------|-------------|
| `@collection.collectionName` | Reference another collection |

## Operators

| Operator | Description |
|----------|-------------|
| `=` | Equal |
| `!=` | Not equal |
| `>` | Greater than |
| `>=` | Greater than or equal |
| `<` | Less than |
| `<=` | Less than or equal |
| `~` | Contains (case-insensitive) |
| `!~` | Does not contain |
| `?=` | Any equals (for arrays) |
| `?!=` | All not equal |
| `?>` | Any greater than |
| `?<` | Any less than |

## Logical Operators

| Operator | Description |
|----------|-------------|
| `&&` | AND |
| `\|\|` | OR |
| `()` | Grouping |

## Common Patterns

### Owner-Based Access

```
# User can only access their own records
@request.auth.id = user

# User can only access their own records (relation field)
@request.auth.id = author.id
```

### Role-Based Access

```
# Admin only
@request.auth.role = "admin"

# Admin or moderator
@request.auth.role = "admin" || @request.auth.role = "moderator"

# Multiple roles
@request.auth.role ?= ["admin", "editor", "moderator"]
```

### Conditional Access

```
# Public read, authenticated write
List/View: (empty)
Create/Update/Delete: @request.auth.id != ""

# Published content is public, drafts are owner-only
published = true || @request.auth.id = author

# Owner can edit, anyone can view
View: (empty)
Update: @request.auth.id = author
```

### Field-Level Control

```
# Prevent changing certain fields
@request.data.role:isset = false || @request.auth.role = "admin"

# Only allow specific status values
@request.data.status = "draft" || @request.data.status = "pending"
```

### Cross-Collection Rules

```
# User must be team member
@request.auth.id = @collection.teams.members.id

# User must belong to organization
@collection.organizations.members.id ?= @request.auth.id

# Check user has specific permission
@collection.permissions.user = @request.auth.id &&
@collection.permissions.resource = id &&
@collection.permissions.action = "edit"
```

### Time-Based Rules

```
# Cannot modify after 24 hours
created > @now - 86400

# Only during business hours (use with caution)
@request.auth.id != "" && @now > "2024-01-01"
```

## Field-Level Rules

Control access to specific fields:

### Hide Fields

Use API rules to filter response:

```
# In select/expand, only return specific fields
View: published = true
```

### Prevent Field Modification

```
# Prevent changing owner
Update: @request.data.owner:isset = false || @request.data.owner = owner

# Admin can change any field, users can't change role
Update: @request.auth.role = "admin" || @request.data.role:isset = false
```

## Examples by Use Case

### Blog Posts

```
# List: Published posts are public, drafts for owner
List: published = true || @request.auth.id = author

# View: Same as list
View: published = true || @request.auth.id = author

# Create: Authenticated users
Create: @request.auth.id != ""

# Update: Owner only
Update: @request.auth.id = author

# Delete: Owner only
Delete: @request.auth.id = author
```

### Comments

```
# List: Public
List: (empty)

# View: Public
View: (empty)

# Create: Authenticated and verified
Create: @request.auth.id != "" && @request.auth.verified = true

# Update: Owner within 1 hour
Update: @request.auth.id = author && created > @now - 3600

# Delete: Owner or admin
Delete: @request.auth.id = author || @request.auth.role = "admin"
```

### Private Messages

```
# List: Participant only
List: @request.auth.id = sender || @request.auth.id = recipient

# View: Participant only
View: @request.auth.id = sender || @request.auth.id = recipient

# Create: Authenticated users
Create: @request.auth.id != ""

# Update: None (messages are immutable)
Update: null

# Delete: Sender only
Delete: @request.auth.id = sender
```

### Team Resources

```
# List: Team members
List: team.members.id ?= @request.auth.id

# View: Team members
View: team.members.id ?= @request.auth.id

# Create: Team admins
Create: @collection.team_members.team = @request.data.team &&
        @collection.team_members.user = @request.auth.id &&
        @collection.team_members.role = "admin"

# Update: Team admins
Update: team.members.id ?= @request.auth.id &&
        @collection.team_members.user = @request.auth.id &&
        @collection.team_members.role = "admin"

# Delete: Team owners
Delete: team.owner = @request.auth.id
```

## Testing Rules

### Via Admin UI

1. Go to collection settings
2. Click "API Rules" tab
3. Use the rule editor with autocomplete
4. Test with preview

### Via API

Test rules by making requests with different auth states:

```bash
# Unauthenticated
curl http://127.0.0.1:8090/api/collections/posts/records

# Authenticated
curl http://127.0.0.1:8090/api/collections/posts/records \
  -H "Authorization: Bearer <token>"
```

## Best Practices

1. **Default deny** - Start restrictive, open as needed
2. **Test thoroughly** - Test with different user types
3. **Keep simple** - Complex rules are hard to maintain
4. **Document rules** - Comment complex logic
5. **Use roles** - Create role fields instead of complex conditions
6. **Audit regularly** - Review rules periodically

## Debugging Rules

If access is denied unexpectedly:

1. Check user is authenticated (`@request.auth.id`)
2. Verify field values match expected
3. Test simpler rule to isolate issue
4. Check relation fields are correct
5. Review server logs for errors

## Next Steps

- [Collections API](../api/collections.md)
- [Records API](../api/records.md)
- [Authentication](authentication/overview.md)
