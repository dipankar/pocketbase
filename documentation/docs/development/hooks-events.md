# Hooks & Events

PocketBase provides an extensive hook system for extending functionality at various points in the application lifecycle.

## Hook Types

### Application Lifecycle

| Hook | Trigger |
|------|---------|
| `OnBootstrap` | App initialization |
| `OnServe` | HTTP server starting |
| `OnTerminate` | App shutdown |

### Record Lifecycle

| Hook | Trigger |
|------|---------|
| `OnRecordCreateRequest` | Before create validation |
| `OnRecordAfterCreateSuccess` | After successful create |
| `OnRecordUpdateRequest` | Before update validation |
| `OnRecordAfterUpdateSuccess` | After successful update |
| `OnRecordDeleteRequest` | Before delete |
| `OnRecordAfterDeleteSuccess` | After successful delete |
| `OnRecordValidate` | During validation |
| `OnRecordEnrich` | Before returning to client |

### Authentication

| Hook | Trigger |
|------|---------|
| `OnRecordAuthRequest` | Auth attempt |
| `OnRecordAuthRefreshRequest` | Token refresh |

### Email

| Hook | Trigger |
|------|---------|
| `OnMailerRecordVerificationSend` | Verification email |
| `OnMailerRecordPasswordResetSend` | Password reset email |
| `OnMailerRecordEmailChangeSend` | Email change confirmation |

## Go Hook Examples

### Application Hooks

```go
// On bootstrap
app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
    log.Println("App starting...")
    // Initialize services, load config, etc.
    return e.Next()
})

// On serve
app.OnServe().BindFunc(func(e *core.ServeEvent) error {
    log.Println("Server starting at:", e.Server.Addr)
    // Add custom routes
    return e.Next()
})

// On terminate
app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
    log.Println("Shutting down...")
    // Cleanup resources
    return e.Next()
})
```

### Record Hooks

```go
// Before create
app.OnRecordCreateRequest("posts").BindFunc(func(e *core.RecordRequestEvent) error {
    // Set default values
    e.Record.Set("status", "draft")
    e.Record.Set("views", 0)

    // Validate custom logic
    if e.Record.GetString("title") == "" {
        return apis.NewBadRequestError("Title is required", nil)
    }

    return e.Next()
})

// After create success
app.OnRecordAfterCreateSuccess("posts").BindFunc(func(e *core.RecordEvent) error {
    log.Printf("New post created: %s", e.Record.Id)

    // Send notification
    go sendNewPostNotification(e.Record)

    return e.Next()
})

// Before update
app.OnRecordUpdateRequest("posts").BindFunc(func(e *core.RecordRequestEvent) error {
    original := e.Record.Original()

    // Prevent changing author
    if e.Record.GetString("author") != original.GetString("author") {
        return apis.NewForbiddenError("Cannot change author", nil)
    }

    return e.Next()
})

// After delete
app.OnRecordAfterDeleteSuccess("posts").BindFunc(func(e *core.RecordEvent) error {
    log.Printf("Post deleted: %s", e.Record.Id)

    // Cleanup related data
    go cleanupPostComments(e.Record.Id)

    return e.Next()
})
```

### Validation Hooks

```go
app.OnRecordValidate("users").BindFunc(func(e *core.RecordEvent) error {
    // Custom validation
    username := e.Record.GetString("username")

    if len(username) < 3 {
        return validation.NewError("username", "Must be at least 3 characters")
    }

    // Check uniqueness
    existing, _ := app.FindFirstRecordByFilter(
        "users",
        "username = {:username} && id != {:id}",
        dbx.Params{
            "username": username,
            "id":       e.Record.Id,
        },
    )

    if existing != nil {
        return validation.NewError("username", "Already taken")
    }

    return e.Next()
})
```

### Enrichment Hooks

```go
// Add computed fields before returning to client
app.OnRecordEnrich("posts").BindFunc(func(e *core.RecordEnrichEvent) error {
    // Add comment count
    count, _ := app.CountRecords("comments", "post = {:postId}",
        dbx.Params{"postId": e.Record.Id},
    )
    e.Record.Set("commentCount", count)

    // Add author name (expand alternative)
    author, _ := app.FindRecordById("users", e.Record.GetString("author"))
    if author != nil {
        e.Record.Set("authorName", author.GetString("name"))
    }

    return e.Next()
})
```

### Authentication Hooks

```go
// On auth request
app.OnRecordAuthRequest("users").BindFunc(func(e *core.RecordAuthRequestEvent) error {
    // Log login
    log.Printf("User %s logged in from %s",
        e.Record.Email(),
        e.RequestInfo().RemoteAddr,
    )

    // Update last login
    e.Record.Set("lastLogin", time.Now())
    if err := app.Save(e.Record); err != nil {
        log.Printf("Failed to update last login: %v", err)
    }

    return e.Next()
})

// Block certain users
app.OnRecordAuthRequest("users").BindFunc(func(e *core.RecordAuthRequestEvent) error {
    if e.Record.GetBool("banned") {
        return apis.NewForbiddenError("Account is banned", nil)
    }
    return e.Next()
})
```

### Email Hooks

```go
// Customize verification email
app.OnMailerRecordVerificationSend("users").BindFunc(func(e *core.MailerRecordEvent) error {
    e.Message.Subject = "Please verify your account"
    e.Message.HTML = fmt.Sprintf(`
        <h1>Welcome, %s!</h1>
        <p>Click <a href="%s">here</a> to verify your email.</p>
    `, e.Record.GetString("name"), e.Meta["actionUrl"])

    return e.Next()
})

// Log password resets
app.OnMailerRecordPasswordResetSend("users").BindFunc(func(e *core.MailerRecordEvent) error {
    log.Printf("Password reset requested for: %s", e.Record.Email())
    return e.Next()
})
```

## JavaScript Hook Examples

### Record Hooks

```javascript
// Before create
onRecordCreateRequest((e) => {
    e.record.set("status", "pending");
    e.record.set("createdBy", e.auth?.id || null);
}, "orders");

// After create
onRecordAfterCreateSuccess((e) => {
    console.log("Order created:", e.record.id);

    // Send notification
    $http.send({
        url: "https://hooks.slack.com/...",
        method: "POST",
        body: JSON.stringify({
            text: `New order: ${e.record.id}`
        })
    });
}, "orders");

// Before update
onRecordUpdateRequest((e) => {
    const original = e.record.original();

    // Track status changes
    if (e.record.get("status") !== original.get("status")) {
        e.record.set("statusChangedAt", new Date().toISOString());
    }
}, "orders");

// Validation
onRecordValidate((e) => {
    const price = e.record.get("price");

    if (price < 0) {
        throw new ValidationError("price", "Must be positive");
    }
}, "products");
```

### Auth Hooks

```javascript
// After login
onRecordAuthRequest((e) => {
    e.record.set("lastLogin", new Date().toISOString());
    $app.save(e.record);
}, "users");

// Block banned users
onRecordAuthRequest((e) => {
    if (e.record.get("banned")) {
        throw new ForbiddenError("Account is banned");
    }
}, "users");
```

### Email Hooks

```javascript
// Custom verification email
onMailerRecordVerificationSend((e) => {
    const name = e.record.get("name") || "there";

    e.message.subject = "Verify your email";
    e.message.html = `
        <h1>Hi ${name}!</h1>
        <p>Please <a href="${e.meta.actionUrl}">click here</a> to verify.</p>
    `;
}, "users");
```

## Hook Priority

Multiple hooks can be registered for the same event. They execute in order of registration:

```go
// First hook
app.OnRecordAfterCreateSuccess("posts").BindFunc(func(e *core.RecordEvent) error {
    log.Println("First hook")
    return e.Next()
})

// Second hook
app.OnRecordAfterCreateSuccess("posts").BindFunc(func(e *core.RecordEvent) error {
    log.Println("Second hook")
    return e.Next()
})
```

## Stopping Hook Chain

Return an error to stop further hook execution:

```go
app.OnRecordCreateRequest("posts").BindFunc(func(e *core.RecordRequestEvent) error {
    if someCondition {
        // Stop processing and return error
        return apis.NewBadRequestError("Not allowed", nil)
    }
    return e.Next()
})
```

## Async Operations

For long-running tasks, use goroutines:

```go
app.OnRecordAfterCreateSuccess("orders").BindFunc(func(e *core.RecordEvent) error {
    // Process async to not block response
    go func() {
        // Send email
        sendOrderConfirmation(e.Record)

        // Update inventory
        updateInventory(e.Record)

        // Notify warehouse
        notifyWarehouse(e.Record)
    }()

    return e.Next()
})
```

## Available Event Data

### RecordEvent

```go
e.App      // Application instance
e.Record   // The record being processed
```

### RecordRequestEvent

```go
e.App             // Application instance
e.Record          // The record
e.HttpContext     // HTTP request context
e.Auth            // Authenticated user (if any)
```

## Best Practices

1. **Keep hooks fast** - Use async for slow operations
2. **Handle errors gracefully** - Log errors, don't crash
3. **Use specific collections** - Don't hook all collections unnecessarily
4. **Test thoroughly** - Hooks can have subtle effects
5. **Document hooks** - Make hook behavior clear
6. **Avoid infinite loops** - Don't trigger hooks from hooks without guards

## Next Steps

- [Extending with Go](extending-go.md)
- [Extending with JavaScript](extending-js.md)
- [Migrations](migrations.md)
