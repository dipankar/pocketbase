# Managing Collections

The Admin UI provides a visual interface for creating and managing collections.

## Creating Collections

1. Click **New collection** in the sidebar
2. Choose collection type:
   - **Base** - Standard data collection
   - **Auth** - User authentication collection
   - **View** - Read-only SQL-based collection
3. Enter collection name
4. Add fields
5. Configure API rules
6. Click **Create**

## Collection Types

### Base Collection

Standard collection for storing data:

- Custom fields
- CRUD API
- Realtime subscriptions

### Auth Collection

For user authentication:

- Built-in email, password, verified fields
- Authentication endpoints
- Email verification
- Password reset

### View Collection

Read-only collection from SQL query:

- Aggregate data from other collections
- Custom SQL queries
- No write operations

## Managing Fields

### Adding Fields

1. Click **+ New field**
2. Select field type
3. Configure field options
4. Set as required if needed
5. Save collection

### Field Types

| Type | Description | Use Case |
|------|-------------|----------|
| Text | Plain text | Names, titles |
| Number | Numeric values | Prices, counts |
| Bool | True/false | Flags, toggles |
| Email | Validated email | Contact info |
| URL | Validated URL | Links |
| Date | Date/time | Timestamps |
| Select | Dropdown options | Status, category |
| Relation | Link to other records | Associations |
| File | File uploads | Images, documents |
| JSON | Arbitrary JSON | Complex data |
| Editor | Rich text HTML | Content |

### Editing Fields

1. Click on field name
2. Modify settings
3. Save collection

!!! warning "Data Impact"
    Changing field types or removing fields may affect existing data.

### Deleting Fields

1. Click the field's delete icon
2. Confirm deletion
3. Save collection

!!! danger "Data Loss"
    Deleted fields permanently remove associated data.

## API Rules

### Configuring Rules

1. Go to **API Rules** tab
2. Set rules for each operation:
   - List/Search
   - View
   - Create
   - Update
   - Delete

### Rule Examples

**Public read, authenticated write:**

```
List: (leave empty)
View: (leave empty)
Create: @request.auth.id != ""
Update: @request.auth.id != ""
Delete: @request.auth.id != ""
```

**Owner-based access:**

```
List: @request.auth.id = owner
View: @request.auth.id = owner
Update: @request.auth.id = owner
Delete: @request.auth.id = owner
```

### Rule Editor

The rule editor provides:

- Syntax highlighting
- Auto-complete for fields
- Validation feedback
- Preview of available variables

## Collection Options

### Indexes

Add database indexes for performance:

1. Go to **Options** tab
2. Add index in Indexes section
3. Use SQL syntax: `CREATE INDEX idx_name ON collection (field)`

### Auth Options (Auth Collections)

Configure authentication settings:

- Allowed auth methods
- Password requirements
- Email verification
- OAuth2 providers

## Import/Export

### Export Collection Schema

1. Open collection settings
2. View JSON schema
3. Copy for backup or migration

### Import Collection

Use the API or migrations to import collection definitions.

## Renaming Collections

1. Open collection settings
2. Change the name field
3. Save

!!! note "API Impact"
    Renaming affects API endpoints. Update client code accordingly.

## Deleting Collections

1. Open collection settings
2. Click **Delete collection**
3. Type collection name to confirm
4. Click **Delete**

!!! danger "Irreversible"
    Deleting a collection permanently removes all records.

## Best Practices

### Naming Conventions

- Use lowercase with underscores: `user_profiles`
- Be descriptive: `order_items` not `items`
- Use plural form: `posts` not `post`

### Schema Design

- Normalize data with relations
- Add indexes for frequently queried fields
- Set appropriate field constraints
- Plan for growth

### Rule Design

- Start restrictive, open as needed
- Test rules with different user types
- Document complex rules
- Use comments in rule expressions

## System Collections

PocketBase includes system collections (prefixed with `_`):

| Collection | Purpose |
|------------|---------|
| `_superusers` | Admin accounts |
| `_auth_origins` | OAuth identity mappings |
| `_external_auths` | External auth records |
| `_mfa` | Multi-factor auth |
| `_otp` | One-time passwords |

!!! warning "System Collections"
    Modifying system collections can break core functionality.

## Next Steps

- [Managing Records](records.md)
- [Fields Reference](../features/fields.md)
- [Authorization Rules](../features/authorization.md)
