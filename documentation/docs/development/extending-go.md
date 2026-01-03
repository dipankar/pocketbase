# Extending with Go

PocketBase can be used as a Go framework, allowing you to build custom functionality directly in Go.

## Getting Started

### Installation

```bash
go get github.com/pocketbase/pocketbase
```

### Basic Application

```go
package main

import (
    "log"

    "github.com/pocketbase/pocketbase"
)

func main() {
    app := pocketbase.New()

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
```

### Build and Run

```bash
go build -o myapp
./myapp serve
```

## Application Structure

```go
app := pocketbase.New()

// app.App provides access to:
// - Database operations
// - Collection management
// - Record operations
// - File handling
// - Email sending
// - Settings
// - Hooks
```

## Custom Routes

Add custom API endpoints:

```go
package main

import (
    "net/http"

    "github.com/pocketbase/pocketbase"
    "github.com/pocketbase/pocketbase/core"
)

func main() {
    app := pocketbase.New()

    app.OnServe().BindFunc(func(se *core.ServeEvent) error {
        // Add custom route
        se.Router.GET("/api/hello", func(e *core.RequestEvent) error {
            return e.JSON(http.StatusOK, map[string]string{
                "message": "Hello World!",
            })
        })

        // Route with path parameters
        se.Router.GET("/api/users/{id}", func(e *core.RequestEvent) error {
            id := e.Request.PathValue("id")
            return e.JSON(http.StatusOK, map[string]string{
                "userId": id,
            })
        })

        return se.Next()
    })

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
```

### Route Groups

```go
app.OnServe().BindFunc(func(se *core.ServeEvent) error {
    // Create route group
    api := se.Router.Group("/api/v2")

    api.GET("/items", listItems)
    api.POST("/items", createItem)
    api.GET("/items/{id}", getItem)
    api.PATCH("/items/{id}", updateItem)
    api.DELETE("/items/{id}", deleteItem)

    return se.Next()
})
```

### Middleware

```go
// Authentication middleware
func requireAuth(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        authRecord := c.Get(apis.ContextAuthRecordKey)
        if authRecord == nil {
            return apis.NewForbiddenError("Authentication required", nil)
        }
        return next(c)
    }
}

// Apply middleware
api.GET("/protected", protectedHandler, requireAuth)
```

## Database Operations

### Find Records

```go
// Find single record
record, err := app.FindRecordById("posts", "record_id")

// Find with filter
records, err := app.FindRecordsByFilter(
    "posts",
    "published = true && views > 100",
    "-created",  // sort
    100,         // limit
    0,           // offset
)

// Find first matching
record, err := app.FindFirstRecordByFilter(
    "posts",
    "slug = {:slug}",
    dbx.Params{"slug": "hello-world"},
)
```

### Create Records

```go
collection, err := app.FindCollectionByNameOrId("posts")
if err != nil {
    return err
}

record := core.NewRecord(collection)
record.Set("title", "New Post")
record.Set("content", "Hello World")
record.Set("author", userId)

if err := app.Save(record); err != nil {
    return err
}
```

### Update Records

```go
record, err := app.FindRecordById("posts", recordId)
if err != nil {
    return err
}

record.Set("title", "Updated Title")
record.Set("views", record.GetInt("views") + 1)

if err := app.Save(record); err != nil {
    return err
}
```

### Delete Records

```go
record, err := app.FindRecordById("posts", recordId)
if err != nil {
    return err
}

if err := app.Delete(record); err != nil {
    return err
}
```

### Transactions

```go
err := app.RunInTransaction(func(txApp core.App) error {
    // All operations here use the transaction
    record1, err := txApp.FindRecordById("posts", id1)
    if err != nil {
        return err
    }

    record1.Set("status", "archived")
    if err := txApp.Save(record1); err != nil {
        return err
    }

    record2, err := txApp.FindRecordById("posts", id2)
    if err != nil {
        return err
    }

    record2.Set("featured", true)
    if err := txApp.Save(record2); err != nil {
        return err
    }

    return nil // commit transaction
})
```

## Collections

### Find Collection

```go
collection, err := app.FindCollectionByNameOrId("posts")
```

### Create Collection

```go
collection := core.NewBaseCollection("products")

// Add fields
collection.Fields.Add(&core.TextField{
    Name:     "name",
    Required: true,
})

collection.Fields.Add(&core.NumberField{
    Name: "price",
    Min:  types.Pointer(0.0),
})

collection.Fields.Add(&core.SelectField{
    Name:      "status",
    Values:    []string{"draft", "published", "archived"},
    MaxSelect: 1,
})

// Set API rules
collection.ListRule = types.Pointer("")
collection.ViewRule = types.Pointer("")
collection.CreateRule = types.Pointer("@request.auth.id != ''")

if err := app.Save(collection); err != nil {
    return err
}
```

## Authentication

### Get Current User

```go
func handler(e *core.RequestEvent) error {
    // Get auth record from context
    authRecord := e.Auth

    if authRecord == nil {
        return apis.NewUnauthorizedError("Not authenticated", nil)
    }

    userId := authRecord.Id
    userEmail := authRecord.Email()

    return e.JSON(http.StatusOK, map[string]any{
        "id":    userId,
        "email": userEmail,
    })
}
```

### Generate Auth Token

```go
record, err := app.FindRecordById("users", userId)
if err != nil {
    return err
}

token, err := record.NewAuthToken()
if err != nil {
    return err
}
```

### Custom Authentication

```go
app.OnServe().BindFunc(func(se *core.ServeEvent) error {
    se.Router.POST("/api/custom-login", func(e *core.RequestEvent) error {
        data := struct {
            ApiKey string `json:"apiKey"`
        }{}

        if err := e.BindBody(&data); err != nil {
            return apis.NewBadRequestError("Invalid request", err)
        }

        // Validate API key
        record, err := app.FindFirstRecordByFilter(
            "api_keys",
            "key = {:key} && active = true",
            dbx.Params{"key": data.ApiKey},
        )
        if err != nil {
            return apis.NewUnauthorizedError("Invalid API key", err)
        }

        // Get associated user
        user, err := app.FindRecordById("users", record.GetString("user"))
        if err != nil {
            return err
        }

        token, err := user.NewAuthToken()
        if err != nil {
            return err
        }

        return e.JSON(http.StatusOK, map[string]any{
            "token":  token,
            "record": user,
        })
    })

    return se.Next()
})
```

## File Handling

### Upload Files

```go
func uploadHandler(e *core.RequestEvent) error {
    // Get uploaded file
    file, err := e.Request.FormFile("document")
    if err != nil {
        return apis.NewBadRequestError("Missing file", err)
    }

    // Find or create record
    collection, _ := app.FindCollectionByNameOrId("documents")
    record := core.NewRecord(collection)

    // Set file field
    record.Set("file", file)
    record.Set("name", file.Filename)

    if err := app.Save(record); err != nil {
        return err
    }

    return e.JSON(http.StatusOK, record)
}
```

### Get File URL

```go
fileKey := record.GetString("document")
url := app.Settings().Meta.AppURL + "/api/files/" +
    record.Collection().Id + "/" +
    record.Id + "/" +
    fileKey
```

## Sending Emails

```go
err := app.NewMailClient().Send(&mailer.Message{
    From: mail.Address{
        Name:    "My App",
        Address: "noreply@example.com",
    },
    To:      []mail.Address{{Address: "user@example.com"}},
    Subject: "Welcome!",
    HTML:    "<p>Welcome to our app!</p>",
})
```

## Scheduled Tasks

```go
app.Cron().Add("cleanup", "0 0 * * *", func() {
    // Run daily at midnight
    log.Println("Running cleanup...")

    // Delete old records
    records, _ := app.FindRecordsByFilter(
        "logs",
        "created < {:date}",
        "",
        0,
        0,
        dbx.Params{"date": time.Now().AddDate(0, 0, -30)},
    )

    for _, record := range records {
        app.Delete(record)
    }
})
```

## Error Handling

```go
// Return API errors
return apis.NewBadRequestError("Invalid input", err)
return apis.NewUnauthorizedError("Not logged in", nil)
return apis.NewForbiddenError("Access denied", nil)
return apis.NewNotFoundError("Record not found", err)

// Custom error with data
return apis.NewApiError(422, "Validation failed", map[string]any{
    "errors": validationErrors,
})
```

## Building for Production

```bash
# Build for current OS
go build -o pocketbase

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o pocketbase

# Build with optimizations
go build -ldflags="-s -w" -o pocketbase

# Build with version info
go build -ldflags="-X main.Version=1.0.0" -o pocketbase
```

## Example: Complete Custom API

```go
package main

import (
    "log"
    "net/http"

    "github.com/pocketbase/pocketbase"
    "github.com/pocketbase/pocketbase/apis"
    "github.com/pocketbase/pocketbase/core"
)

func main() {
    app := pocketbase.New()

    // Add hooks
    app.OnRecordAfterCreateSuccess("orders").BindFunc(func(e *core.RecordEvent) error {
        // Send notification email
        order := e.Record
        log.Printf("New order: %s", order.Id)
        return e.Next()
    })

    // Add custom routes
    app.OnServe().BindFunc(func(se *core.ServeEvent) error {
        // Public endpoint
        se.Router.GET("/api/stats", func(e *core.RequestEvent) error {
            posts, _ := app.CountRecords("posts", "published = true")
            users, _ := app.CountRecords("users", "")

            return e.JSON(http.StatusOK, map[string]int{
                "posts": posts,
                "users": users,
            })
        })

        // Protected endpoint
        se.Router.GET("/api/me/orders", func(e *core.RequestEvent) error {
            if e.Auth == nil {
                return apis.NewUnauthorizedError("Login required", nil)
            }

            orders, err := app.FindRecordsByFilter(
                "orders",
                "customer = {:userId}",
                "-created",
                50,
                0,
                dbx.Params{"userId": e.Auth.Id},
            )
            if err != nil {
                return err
            }

            return e.JSON(http.StatusOK, orders)
        })

        return se.Next()
    })

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
```

## Next Steps

- [Extending with JavaScript](extending-js.md)
- [Hooks & Events](hooks-events.md)
- [Migrations](migrations.md)
