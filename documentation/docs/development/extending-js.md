# Extending with JavaScript

PocketBase includes a JavaScript VM that allows you to extend functionality without rebuilding the binary.

## Getting Started

### File Location

Place JavaScript files in the `pb_hooks` directory:

```
pb_data/
pb_hooks/
├── main.pb.js
├── routes.pb.js
└── hooks.pb.js
```

All `.pb.js` files are automatically loaded when PocketBase starts.

### Development Mode

Run with `--dev` flag for auto-reload:

```bash
./pocketbase serve --dev
```

## Basic Syntax

```javascript
// Access the app instance
console.log($app.settings().meta.appName);

// Register hooks
onRecordAfterCreateSuccess((e) => {
    console.log("New record created:", e.record.id);
});

// Add routes
routerAdd("GET", "/api/hello", (e) => {
    return e.json(200, { message: "Hello World!" });
});
```

## Custom Routes

### Basic Routes

```javascript
// GET request
routerAdd("GET", "/api/greet/{name}", (e) => {
    const name = e.request.pathValue("name");
    return e.json(200, { message: `Hello, ${name}!` });
});

// POST request
routerAdd("POST", "/api/contact", (e) => {
    const data = $apis.requestInfo(e).data;

    // Validate
    if (!data.email || !data.message) {
        throw new BadRequestError("Email and message required");
    }

    // Process...
    return e.json(200, { success: true });
});

// Multiple methods
routerAdd("GET,POST", "/api/webhook", (e) => {
    const method = e.request.method;
    // Handle both GET and POST
    return e.json(200, { method });
});
```

### Route with Middleware

```javascript
// Require authentication
routerAdd("GET", "/api/profile", (e) => {
    const authRecord = e.auth;

    if (!authRecord) {
        throw new UnauthorizedError("Login required");
    }

    return e.json(200, {
        id: authRecord.id,
        email: authRecord.email(),
        name: authRecord.get("name"),
    });
}, $apis.requireAuth());
```

### Response Types

```javascript
// JSON response
return e.json(200, { data: "value" });

// HTML response
return e.html(200, "<h1>Hello</h1>");

// Plain text
return e.string(200, "Hello World");

// Redirect
return e.redirect(302, "/new-location");

// No content
return e.noContent(204);

// File download
return e.blob(200, "application/pdf", pdfBytes);
```

## Hooks

### Record Hooks

```javascript
// Before create
onRecordCreateRequest((e) => {
    // Modify data before validation
    e.record.set("status", "pending");
});

// After create success
onRecordAfterCreateSuccess((e) => {
    console.log("Created:", e.record.id);

    // Send notification
    $app.newMailClient().send({
        from: { address: "noreply@example.com" },
        to: [{ address: e.record.get("email") }],
        subject: "Welcome!",
        html: "<p>Thanks for signing up!</p>",
    });
});

// Before update
onRecordUpdateRequest((e) => {
    // Prevent changing certain fields
    const old = e.record.original();
    if (e.record.get("role") !== old.get("role")) {
        throw new ForbiddenError("Cannot change role");
    }
});

// After delete
onRecordAfterDeleteSuccess((e) => {
    console.log("Deleted:", e.record.id);
});
```

### Collection-Specific Hooks

```javascript
// Only for "posts" collection
onRecordAfterCreateSuccess((e) => {
    // Increment author's post count
    const author = $app.findRecordById("users", e.record.get("author"));
    author.set("postCount", author.get("postCount") + 1);
    $app.save(author);
}, "posts");

// Multiple collections
onRecordAfterCreateSuccess((e) => {
    console.log("New record in:", e.record.collection().name);
}, "posts", "comments");
```

### Auth Hooks

```javascript
// After successful authentication
onRecordAuthRequest((e) => {
    console.log("User logged in:", e.record.email());

    // Update last login
    e.record.set("lastLogin", new Date().toISOString());
    $app.save(e.record);
});

// Before password reset
onMailerRecordPasswordResetSend((e) => {
    console.log("Password reset requested for:", e.record.email());
});

// Custom verification email
onMailerRecordVerificationSend((e) => {
    e.message.subject = "Please verify your account";
    e.message.html = `
        <h1>Welcome!</h1>
        <p>Click <a href="${e.meta.actionUrl}">here</a> to verify.</p>
    `;
});
```

### Application Hooks

```javascript
// On app bootstrap
onBootstrap((e) => {
    console.log("App starting...");
});

// Before serve
onServe((e) => {
    console.log("Server starting on:", e.server.addr);
});

// On terminate
onTerminate((e) => {
    console.log("App shutting down...");
});
```

## Database Operations

### Find Records

```javascript
// Find by ID
const record = $app.findRecordById("posts", "record_id");

// Find with filter
const records = $app.findRecordsByFilter(
    "posts",
    "published = true && views > 100",
    "-created", // sort
    100,        // limit
    0           // offset
);

// Find first match
const record = $app.findFirstRecordByFilter(
    "users",
    "email = {:email}",
    { email: "user@example.com" }
);

// Count records
const count = $app.countRecords("posts", "published = true");
```

### Create Records

```javascript
const collection = $app.findCollectionByNameOrId("posts");
const record = new Record(collection);

record.set("title", "New Post");
record.set("content", "Hello World");
record.set("author", userId);
record.set("published", false);

$app.save(record);

console.log("Created:", record.id);
```

### Update Records

```javascript
const record = $app.findRecordById("posts", recordId);

record.set("title", "Updated Title");
record.set("views", record.get("views") + 1);

$app.save(record);
```

### Delete Records

```javascript
const record = $app.findRecordById("posts", recordId);
$app.delete(record);
```

### Transactions

```javascript
$app.runInTransaction((txApp) => {
    const record1 = txApp.findRecordById("accounts", id1);
    const record2 = txApp.findRecordById("accounts", id2);

    record1.set("balance", record1.get("balance") - 100);
    record2.set("balance", record2.get("balance") + 100);

    txApp.save(record1);
    txApp.save(record2);
});
```

## Sending Emails

```javascript
$app.newMailClient().send({
    from: {
        name: "My App",
        address: "noreply@example.com"
    },
    to: [{ address: "user@example.com" }],
    subject: "Hello!",
    html: "<p>This is an email.</p>",
});
```

## HTTP Requests

```javascript
// GET request
const res = $http.send({
    url: "https://api.example.com/data",
    method: "GET",
    headers: {
        "Authorization": "Bearer token123"
    }
});

console.log(res.statusCode);
console.log(res.json);

// POST request
const res = $http.send({
    url: "https://api.example.com/webhook",
    method: "POST",
    headers: {
        "Content-Type": "application/json"
    },
    body: JSON.stringify({ event: "new_user" })
});
```

## Scheduled Tasks (Cron)

```javascript
// Run every hour
cronAdd("hourly_cleanup", "0 * * * *", () => {
    console.log("Running hourly cleanup...");

    const oldRecords = $app.findRecordsByFilter(
        "temp_data",
        "created < {:date}",
        "",
        0,
        0,
        { date: new Date(Date.now() - 86400000).toISOString() }
    );

    for (const record of oldRecords) {
        $app.delete(record);
    }
});

// Run daily at midnight
cronAdd("daily_report", "0 0 * * *", () => {
    console.log("Generating daily report...");
});
```

## Error Handling

```javascript
// Throw API errors
throw new BadRequestError("Invalid input");
throw new UnauthorizedError("Login required");
throw new ForbiddenError("Access denied");
throw new NotFoundError("Record not found");

// Custom error with data
throw new ApiError(422, "Validation failed", {
    errors: { email: "Invalid email format" }
});

// Try-catch
try {
    const record = $app.findRecordById("posts", "invalid_id");
} catch (err) {
    console.log("Error:", err.message);
}
```

## Security Utilities

```javascript
// Generate random string
const token = $security.randomString(32);

// Hash password
const hash = $security.hashPassword("password123");

// Verify password
const valid = $security.compareHashAndPassword(hash, "password123");

// Generate JWT
const jwt = $security.jwt.sign({ userId: "123" }, "secret", 3600);

// Verify JWT
const payload = $security.jwt.verify(jwt, "secret");
```

## File Operations

```javascript
// Read file
const content = $os.readFile("path/to/file.txt");

// Write file
$os.writeFile("path/to/output.txt", "Hello World");

// Check if exists
const exists = $os.exists("path/to/file.txt");
```

## Complete Example

```javascript
// pb_hooks/main.pb.js

// Custom analytics endpoint
routerAdd("POST", "/api/analytics/track", (e) => {
    const data = $apis.requestInfo(e).data;

    // Validate required fields
    if (!data.event || !data.properties) {
        throw new BadRequestError("Missing event or properties");
    }

    // Create analytics record
    const collection = $app.findCollectionByNameOrId("analytics");
    const record = new Record(collection);

    record.set("event", data.event);
    record.set("properties", JSON.stringify(data.properties));
    record.set("timestamp", new Date().toISOString());
    record.set("userAgent", e.request.header.get("User-Agent"));

    // Get user if authenticated
    if (e.auth) {
        record.set("user", e.auth.id);
    }

    $app.save(record);

    return e.json(200, { tracked: true });
});

// Send welcome email on registration
onRecordAfterCreateSuccess((e) => {
    try {
        $app.newMailClient().send({
            from: { address: $app.settings().meta.senderAddress },
            to: [{ address: e.record.email() }],
            subject: `Welcome to ${$app.settings().meta.appName}!`,
            html: `
                <h1>Welcome, ${e.record.get("name")}!</h1>
                <p>Thanks for joining. Get started by exploring our features.</p>
            `,
        });
    } catch (err) {
        console.error("Failed to send welcome email:", err);
    }
}, "users");

// Auto-set slug on post creation
onRecordCreateRequest((e) => {
    if (!e.record.get("slug")) {
        const title = e.record.get("title") || "";
        const slug = title
            .toLowerCase()
            .replace(/[^a-z0-9]+/g, "-")
            .replace(/^-|-$/g, "");
        e.record.set("slug", slug + "-" + Date.now());
    }
}, "posts");

// Clean up old sessions daily
cronAdd("cleanup_sessions", "0 3 * * *", () => {
    const expiredSessions = $app.findRecordsByFilter(
        "sessions",
        "expiresAt < {:now}",
        "",
        0,
        0,
        { now: new Date().toISOString() }
    );

    console.log(`Cleaning up ${expiredSessions.length} expired sessions`);

    for (const session of expiredSessions) {
        $app.delete(session);
    }
});
```

## Next Steps

- [Hooks & Events](hooks-events.md)
- [Extending with Go](extending-go.md)
- [Migrations](migrations.md)
