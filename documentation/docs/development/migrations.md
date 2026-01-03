# Migrations

Migrations help you version control your database schema changes.

## Overview

PocketBase supports two types of migrations:

1. **Go Migrations** - Compiled with your application
2. **JavaScript Migrations** - Loaded at runtime from files

## Creating Migrations

### Go Migrations

```bash
# Create new Go migration
./pocketbase migrate create "add_user_fields"
```

This creates a file like `migrations/1704067200_add_user_fields.go`:

```go
package migrations

import (
    "github.com/pocketbase/pocketbase/core"
    m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
    m.Register(func(app core.App) error {
        // Up migration
        collection, err := app.FindCollectionByNameOrId("users")
        if err != nil {
            return err
        }

        // Add new field
        collection.Fields.Add(&core.TextField{
            Name:     "bio",
            Required: false,
        })

        return app.Save(collection)
    }, func(app core.App) error {
        // Down migration
        collection, err := app.FindCollectionByNameOrId("users")
        if err != nil {
            return err
        }

        // Remove field
        collection.Fields.RemoveByName("bio")

        return app.Save(collection)
    })
}
```

### JavaScript Migrations

```bash
# Create new JS migration
./pocketbase migrate create "add_user_fields" --js
```

Creates `pb_migrations/1704067200_add_user_fields.js`:

```javascript
migrate((app) => {
    // Up migration
    const collection = app.findCollectionByNameOrId("users");

    collection.fields.add(new TextField({
        name: "bio",
        required: false,
    }));

    app.save(collection);
}, (app) => {
    // Down migration
    const collection = app.findCollectionByNameOrId("users");
    collection.fields.removeByName("bio");
    app.save(collection);
});
```

## Running Migrations

### Run All Pending

```bash
./pocketbase migrate up
```

### Revert Last Migration

```bash
./pocketbase migrate down
```

### Check Status

```bash
./pocketbase migrate status
```

## Migration Examples

### Create Collection

**Go:**

```go
m.Register(func(app core.App) error {
    collection := core.NewBaseCollection("products")

    collection.Fields.Add(&core.TextField{
        Name:     "name",
        Required: true,
    })

    collection.Fields.Add(&core.NumberField{
        Name: "price",
        Min:  types.Pointer(0.0),
    })

    collection.Fields.Add(&core.TextField{
        Name: "sku",
    })

    // Set rules
    collection.ListRule = types.Pointer("")
    collection.ViewRule = types.Pointer("")
    collection.CreateRule = types.Pointer("@request.auth.role = 'admin'")
    collection.UpdateRule = types.Pointer("@request.auth.role = 'admin'")
    collection.DeleteRule = types.Pointer("@request.auth.role = 'admin'")

    return app.Save(collection)
}, func(app core.App) error {
    collection, err := app.FindCollectionByNameOrId("products")
    if err != nil {
        return nil // Already deleted
    }
    return app.Delete(collection)
})
```

**JavaScript:**

```javascript
migrate((app) => {
    const collection = new Collection({
        name: "products",
        type: "base",
        fields: [
            new TextField({ name: "name", required: true }),
            new NumberField({ name: "price", min: 0 }),
            new TextField({ name: "sku" }),
        ],
        listRule: "",
        viewRule: "",
        createRule: "@request.auth.role = 'admin'",
        updateRule: "@request.auth.role = 'admin'",
        deleteRule: "@request.auth.role = 'admin'",
    });

    app.save(collection);
}, (app) => {
    const collection = app.findCollectionByNameOrId("products");
    if (collection) {
        app.delete(collection);
    }
});
```

### Add Field

**Go:**

```go
m.Register(func(app core.App) error {
    collection, err := app.FindCollectionByNameOrId("posts")
    if err != nil {
        return err
    }

    collection.Fields.Add(&core.SelectField{
        Name:      "status",
        Values:    []string{"draft", "published", "archived"},
        MaxSelect: 1,
    })

    return app.Save(collection)
}, func(app core.App) error {
    collection, err := app.FindCollectionByNameOrId("posts")
    if err != nil {
        return err
    }

    collection.Fields.RemoveByName("status")
    return app.Save(collection)
})
```

### Modify Field

**Go:**

```go
m.Register(func(app core.App) error {
    collection, err := app.FindCollectionByNameOrId("posts")
    if err != nil {
        return err
    }

    // Find and modify field
    field := collection.Fields.GetByName("title").(*core.TextField)
    field.Max = 500

    return app.Save(collection)
}, func(app core.App) error {
    collection, err := app.FindCollectionByNameOrId("posts")
    if err != nil {
        return err
    }

    field := collection.Fields.GetByName("title").(*core.TextField)
    field.Max = 200

    return app.Save(collection)
})
```

### Add Relation

**Go:**

```go
m.Register(func(app core.App) error {
    posts, err := app.FindCollectionByNameOrId("posts")
    if err != nil {
        return err
    }

    categories, err := app.FindCollectionByNameOrId("categories")
    if err != nil {
        return err
    }

    posts.Fields.Add(&core.RelationField{
        Name:          "categories",
        CollectionId:  categories.Id,
        MaxSelect:     types.Pointer(5),
        CascadeDelete: false,
    })

    return app.Save(posts)
}, func(app core.App) error {
    posts, err := app.FindCollectionByNameOrId("posts")
    if err != nil {
        return err
    }

    posts.Fields.RemoveByName("categories")
    return app.Save(posts)
})
```

### Add Index

**Go:**

```go
m.Register(func(app core.App) error {
    collection, err := app.FindCollectionByNameOrId("posts")
    if err != nil {
        return err
    }

    collection.Indexes = append(collection.Indexes,
        "CREATE INDEX idx_posts_slug ON posts (slug)",
        "CREATE INDEX idx_posts_author_created ON posts (author, created)",
    )

    return app.Save(collection)
}, func(app core.App) error {
    collection, err := app.FindCollectionByNameOrId("posts")
    if err != nil {
        return err
    }

    // Remove indexes
    newIndexes := []string{}
    for _, idx := range collection.Indexes {
        if !strings.Contains(idx, "idx_posts_slug") &&
           !strings.Contains(idx, "idx_posts_author_created") {
            newIndexes = append(newIndexes, idx)
        }
    }
    collection.Indexes = newIndexes

    return app.Save(collection)
})
```

### Data Migration

**Go:**

```go
m.Register(func(app core.App) error {
    // Update all existing records
    records, err := app.FindRecordsByFilter(
        "posts",
        "status = ''",
        "",
        0,
        0,
    )
    if err != nil {
        return err
    }

    for _, record := range records {
        record.Set("status", "draft")
        if err := app.Save(record); err != nil {
            return err
        }
    }

    return nil
}, func(app core.App) error {
    // Revert data changes if needed
    return nil
})
```

### Update API Rules

**Go:**

```go
m.Register(func(app core.App) error {
    collection, err := app.FindCollectionByNameOrId("posts")
    if err != nil {
        return err
    }

    collection.ListRule = types.Pointer("published = true || @request.auth.id = author")
    collection.ViewRule = types.Pointer("published = true || @request.auth.id = author")
    collection.CreateRule = types.Pointer("@request.auth.verified = true")
    collection.UpdateRule = types.Pointer("@request.auth.id = author")
    collection.DeleteRule = types.Pointer("@request.auth.id = author")

    return app.Save(collection)
}, func(app core.App) error {
    collection, err := app.FindCollectionByNameOrId("posts")
    if err != nil {
        return err
    }

    collection.ListRule = types.Pointer("")
    collection.ViewRule = types.Pointer("")
    collection.CreateRule = types.Pointer("")
    collection.UpdateRule = types.Pointer("")
    collection.DeleteRule = types.Pointer("")

    return app.Save(collection)
})
```

## Auto-Generate Migrations

Generate migrations from current database state:

```bash
./pocketbase migrate collections
```

This creates a migration that recreates all collections as they currently exist.

## Embedding Migrations

For Go applications, embed migrations in the binary:

```go
package main

import (
    "github.com/pocketbase/pocketbase"
    _ "myapp/migrations" // Import migrations
)

func main() {
    app := pocketbase.New()

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
```

## Migration Best Practices

1. **Always provide down migrations** - Enable rollback
2. **Test migrations on copy of production data** - Catch issues early
3. **Keep migrations atomic** - One logical change per migration
4. **Backup before migrating** - Safety first
5. **Use transactions** - Ensure consistency
6. **Version control migrations** - Track changes
7. **Don't modify old migrations** - Create new ones instead

## Troubleshooting

### Migration Fails

1. Check error message
2. Verify collection/field exists
3. Check for data conflicts
4. Review down migration

### Can't Rollback

1. Check down migration is defined
2. Verify migration was recorded
3. Check for dependent data

### Duplicate Migration

Migrations are tracked by filename timestamp. Ensure unique timestamps.

## Next Steps

- [Extending with Go](extending-go.md)
- [Extending with JavaScript](extending-js.md)
- [Deployment Guide](../guides/deployment.md)
