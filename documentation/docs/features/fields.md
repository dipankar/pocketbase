# Fields

Fields define the structure and validation rules for your collection data.

## Built-in System Fields

All records automatically include these fields:

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique 15-character identifier |
| `created` | datetime | Creation timestamp |
| `updated` | datetime | Last modification timestamp |
| `collectionId` | string | Parent collection ID |
| `collectionName` | string | Parent collection name (virtual) |

Auth collections include additional system fields:

| Field | Type | Description |
|-------|------|-------------|
| `email` | string | User email address |
| `emailVisibility` | bool | Whether email is public |
| `verified` | bool | Email verification status |
| `password` | string | Hashed password (write-only) |
| `tokenKey` | string | Auth token key (internal) |

## Field Types

### Text

Single or multi-line text content.

```json
{
  "name": "title",
  "type": "text",
  "required": true,
  "options": {
    "min": 1,
    "max": 200,
    "pattern": ""
  }
}
```

**Options:**

| Option | Type | Description |
|--------|------|-------------|
| `min` | int | Minimum character length |
| `max` | int | Maximum character length |
| `pattern` | string | Regex pattern for validation |

**Examples:**

```json
// Simple required text
{ "name": "name", "type": "text", "required": true }

// Text with length limits
{ "name": "bio", "type": "text", "options": { "max": 500 } }

// Text with pattern (alphanumeric only)
{ "name": "code", "type": "text", "options": { "pattern": "^[a-zA-Z0-9]+$" } }
```

### Number

Integer or decimal numbers.

```json
{
  "name": "price",
  "type": "number",
  "options": {
    "min": 0,
    "max": 999999,
    "noDecimal": false
  }
}
```

**Options:**

| Option | Type | Description |
|--------|------|-------------|
| `min` | number | Minimum value |
| `max` | number | Maximum value |
| `noDecimal` | bool | Allow only integers |

### Bool

True/false values.

```json
{
  "name": "active",
  "type": "bool"
}
```

Default value is `false` if not provided.

### Email

Validated email addresses.

```json
{
  "name": "contactEmail",
  "type": "email",
  "options": {
    "exceptDomains": ["spam.com", "temp-mail.org"],
    "onlyDomains": []
  }
}
```

**Options:**

| Option | Type | Description |
|--------|------|-------------|
| `exceptDomains` | array | Blocked domains |
| `onlyDomains` | array | Allowed domains only |

### URL

Validated URLs.

```json
{
  "name": "website",
  "type": "url",
  "options": {
    "exceptDomains": [],
    "onlyDomains": ["example.com"]
  }
}
```

### Date / DateTime

Date and time values.

```json
{
  "name": "publishDate",
  "type": "date",
  "options": {
    "min": "",
    "max": ""
  }
}
```

**Options:**

| Option | Type | Description |
|--------|------|-------------|
| `min` | string | Minimum date (YYYY-MM-DD) |
| `max` | string | Maximum date (YYYY-MM-DD) |

### AutoDate

Automatically set date on create/update.

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

**Options:**

| Option | Type | Description |
|--------|------|-------------|
| `onCreate` | bool | Set on record creation |
| `onUpdate` | bool | Update on record modification |

### Select

Single or multiple choice options.

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

**Options:**

| Option | Type | Description |
|--------|------|-------------|
| `maxSelect` | int | Max selections (1 = single select) |
| `values` | array | Available options |

**Examples:**

```json
// Single select (returns string)
{
  "name": "category",
  "type": "select",
  "options": { "maxSelect": 1, "values": ["tech", "news", "sports"] }
}

// Multi-select (returns array)
{
  "name": "tags",
  "type": "select",
  "options": { "maxSelect": 5, "values": ["go", "javascript", "python", "rust"] }
}
```

### Relation

Links to records in other collections.

```json
{
  "name": "author",
  "type": "relation",
  "options": {
    "collectionId": "users_collection_id",
    "cascadeDelete": false,
    "minSelect": null,
    "maxSelect": 1,
    "displayFields": ["name", "email"]
  }
}
```

**Options:**

| Option | Type | Description |
|--------|------|-------------|
| `collectionId` | string | Target collection ID |
| `cascadeDelete` | bool | Delete related records |
| `minSelect` | int | Minimum required relations |
| `maxSelect` | int | Max relations (1 = single, null = unlimited) |
| `displayFields` | array | Fields shown in admin UI |

**Examples:**

```json
// One-to-one relation
{
  "name": "profile",
  "type": "relation",
  "options": { "collectionId": "profiles", "maxSelect": 1 }
}

// One-to-many relation
{
  "name": "comments",
  "type": "relation",
  "options": { "collectionId": "comments", "maxSelect": null }
}

// Many-to-many relation
{
  "name": "tags",
  "type": "relation",
  "options": { "collectionId": "tags", "maxSelect": null }
}
```

### File

File uploads.

```json
{
  "name": "documents",
  "type": "file",
  "options": {
    "maxSelect": 5,
    "maxSize": 10485760,
    "mimeTypes": ["application/pdf", "image/*"],
    "thumbs": ["100x100", "200x0"],
    "protected": false
  }
}
```

**Options:**

| Option | Type | Description |
|--------|------|-------------|
| `maxSelect` | int | Max files |
| `maxSize` | int | Max file size in bytes |
| `mimeTypes` | array | Allowed MIME types |
| `thumbs` | array | Thumbnail sizes for images |
| `protected` | bool | Require token for access |

### JSON

Arbitrary JSON data.

```json
{
  "name": "metadata",
  "type": "json",
  "options": {
    "maxSize": 2000000
  }
}
```

**Options:**

| Option | Type | Description |
|--------|------|-------------|
| `maxSize` | int | Max JSON size in bytes |

**Example values:**

```json
{"key": "value", "nested": {"array": [1, 2, 3]}}
```

### Editor

Rich text (HTML) content.

```json
{
  "name": "content",
  "type": "editor",
  "options": {
    "convertUrls": false
  }
}
```

**Options:**

| Option | Type | Description |
|--------|------|-------------|
| `convertUrls` | bool | Convert relative URLs to absolute |

### GeoPoint

Geographic coordinates.

```json
{
  "name": "location",
  "type": "geopoint"
}
```

**Value format:**

```json
{
  "lon": -73.935242,
  "lat": 40.730610
}
```

### Password

Hashed password field (for custom auth scenarios).

```json
{
  "name": "secret",
  "type": "password",
  "options": {
    "min": 8,
    "max": 72,
    "pattern": ""
  }
}
```

**Options:**

| Option | Type | Description |
|--------|------|-------------|
| `min` | int | Minimum length |
| `max` | int | Maximum length |
| `pattern` | string | Regex pattern |

!!! note
    Password fields are write-only. The value is never returned in API responses.

## Field Configuration

### Common Properties

All fields share these properties:

| Property | Type | Description |
|----------|------|-------------|
| `id` | string | Unique field ID (auto-generated) |
| `name` | string | Field name (alphanumeric + underscore) |
| `type` | string | Field type |
| `required` | bool | Is field required |
| `presentable` | bool | Show in admin list view |
| `options` | object | Type-specific options |

### Naming Conventions

- Use lowercase letters and underscores
- Start with a letter
- Avoid reserved names: `id`, `created`, `updated`, `collectionId`, `collectionName`, `expand`

```
Good: title, user_name, created_at, post_count
Bad: Title, user-name, 1st_field, id
```

## Validation

### Built-in Validation

Each field type has built-in validation:

- **email**: Valid email format
- **url**: Valid URL format
- **number**: Numeric value within range
- **select**: Value in allowed options
- **relation**: Valid record ID in target collection

### Required Fields

```json
{
  "name": "title",
  "type": "text",
  "required": true
}
```

Required fields must have a non-empty value.

### Pattern Validation

Use regex patterns for text fields:

```json
{
  "name": "phone",
  "type": "text",
  "options": {
    "pattern": "^\\+?[1-9]\\d{1,14}$"
  }
}
```

## Modifying Fields

### Adding Fields

Add via Admin UI or API. Existing records get null/default values.

### Renaming Fields

Renaming a field creates a new field. You must migrate data manually.

### Removing Fields

!!! danger "Data Loss"
    Removing a field permanently deletes all data in that field.

### Changing Field Types

Not supported directly. Create new field, migrate data, remove old field.

## Best Practices

1. **Plan schema early** - Changing fields affects existing data
2. **Use appropriate types** - Use `number` for calculations, `text` for display
3. **Set reasonable limits** - Configure min/max to prevent abuse
4. **Index frequently queried fields** - Add database indexes
5. **Use relations** - Normalize data with relations instead of duplicating

## Next Steps

- [Collections API](../api/collections.md)
- [Records API](../api/records.md)
- [Authorization Rules](authorization.md)
