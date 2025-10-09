# Cluster Admin: Platform Operations

## Overview

Cluster admins are **platform operators** who manage the entire PocketBase Enterprise infrastructure. They have:
- System-wide visibility and control
- User management capabilities
- Quota approval workflow
- Impersonation for support
- Authentication via long-lived tokens

---

## Data Model

### ClusterAdmin

```go
// core/control_plane/models.go

type ClusterAdmin struct {
    ID           string    `json:"id"`           // admin_xyz789
    Email        string    `json:"email"`
    Name         string    `json:"name"`

    // Authentication (long-lived token)
    Token        string    `json:"token"`        // Cleartext (show once on creation)
    TokenHash    string    `json:"token_hash"`   // bcrypt hash for storage

    // Permissions
    Permissions  []string  `json:"permissions"`  // ["*"] for super admin

    // Audit
    Created      time.Time `json:"created"`
    Updated      time.Time `json:"updated"`
    LastLogin    time.Time `json:"last_login"`
}
```

### Token Format

```
admin_{random_32_chars}

Example: admin_k8x2Jv9pQm4LnZ7wRt3FyB1cH6sA5dG0
```

---

## Admin Lifecycle

### 1. Create Admin

**Only existing admins can create new admins**

```go
// apis/cluster_admin_mgmt.go

type CreateAdminRequest struct {
    Email       string   `json:"email"`
    Name        string   `json:"name"`
    Permissions []string `json:"permissions"` // Default: ["*"]
}

type CreateAdminResponse struct {
    Admin *ClusterAdmin `json:"admin"`
    Token string        `json:"token"` // Show ONCE
}

func (api *ClusterAdminAPI) CreateAdmin(c echo.Context) error {
    // Verify caller is admin
    caller := getAdminFromContext(c)

    var req CreateAdminRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // Generate token
    token := generateAdminToken() // admin_k8x2...

    // Hash for storage
    hash, _ := bcrypt.GenerateFromPassword([]byte(token), 12)

    // Create admin
    admin := &ClusterAdmin{
        ID:          generateID("admin_"),
        Email:       req.Email,
        Name:        req.Name,
        Token:       token, // Only for response
        TokenHash:   string(hash),
        Permissions: req.Permissions,
        Created:     time.Now(),
    }

    if len(admin.Permissions) == 0 {
        admin.Permissions = []string{"*"} // Default: full access
    }

    // Save via Raft
    if err := api.cp.CreateAdmin(admin); err != nil {
        return err
    }

    // IMPORTANT: Show token only once
    return c.JSON(201, CreateAdminResponse{
        Admin: admin,
        Token: token,
    })
}

func generateAdminToken() string {
    randomBytes := make([]byte, 24)
    rand.Read(randomBytes)
    return "admin_" + base64.URLEncoding.EncodeToString(randomBytes)[:32]
}
```

**Bootstrap First Admin**:
```bash
# On control plane startup, if no admins exist:
./pocketbase serve --mode=control-plane --create-admin

# Generates and prints first admin token:
Created first cluster admin:
  Email: admin@platform.com
  Token: admin_k8x2Jv9pQm4LnZ7wRt3FyB1cH6sA5dG0

SAVE THIS TOKEN! It will not be shown again.
```

### 2. Authentication

**All admin API requests use token in Authorization header**:

```bash
curl -H "Authorization: Bearer admin_k8x2Jv9pQm4LnZ7wRt3FyB1cH6sA5dG0" \
  https://admin.platform.com/api/cluster/health
```

**Middleware**:
```go
// core/control_plane/admin_auth.go

func AdminAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // Extract token from Authorization header
        authHeader := c.Request().Header.Get("Authorization")
        if authHeader == "" {
            return c.JSON(401, map[string]string{"error": "Missing authorization"})
        }

        token := strings.TrimPrefix(authHeader, "Bearer ")
        if !strings.HasPrefix(token, "admin_") {
            return c.JSON(401, map[string]string{"error": "Invalid token format"})
        }

        // Get all admins and check token
        admins, _ := api.cp.GetAllAdmins()
        for _, admin := range admins {
            if bcrypt.CompareHashAndPassword([]byte(admin.TokenHash), []byte(token)) == nil {
                // Valid token
                c.Set("admin", admin)

                // Update last login
                admin.LastLogin = time.Now()
                api.cp.UpdateAdmin(admin)

                return next(c)
            }
        }

        return c.JSON(401, map[string]string{"error": "Invalid token"})
    }
}
```

---

## Admin Dashboard

### URL Structure

```
https://admin.platform.com/          → Admin dashboard (static files)
https://admin.platform.com/api/      → Admin API (token-protected)
```

### Dashboard UI

```
┌──────────────────────────────────────────────────────────────┐
│  PocketBase Cluster Admin            admin@platform.com      │
├──────────────────────────────────────────────────────────────┤
│  Dashboard  │  Users  │  Tenants  │  Nodes  │  Quota Requests│
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│  System Overview                                              │
├──────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │   Users      │  │   Tenants    │  │    Nodes     │       │
│  │    1,234     │  │    3,456     │  │   47 / 50    │       │
│  │   +12 today  │  │   +45 today  │  │   94% cap    │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
│                                                               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Cluster Health: 🟢 Healthy                          │   │
│  │  Control Plane: 3/3 nodes healthy                    │   │
│  │  Tenant Nodes: 47/50 healthy, 2 degraded, 1 down    │   │
│  │  Total API Requests (24h): 125M                      │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                               │
│  Recent Activity:                                            │
│  • 10:34 AM - user_456 created tenant staging-app            │
│  • 10:28 AM - node_47 health degraded (high CPU 85%)        │
│  • 10:15 AM - admin_001 approved quota request qreq_789     │
│  • 09:45 AM - user_123 deleted tenant old-project            │
│                                                               │
│  Active Alerts:                                              │
│  ⚠️  Node node_47 CPU > 80%                    [Investigate] │
│  ⚠️  Node node_12 disk usage > 85%             [Investigate] │
│  📊 12 pending quota requests                  [Review]      │
└──────────────────────────────────────────────────────────────┘
```

---

## User Management

### List Users

**Endpoint**: `GET /api/cluster/users`

```go
func (api *ClusterAdminAPI) ListUsers(c echo.Context) error {
    // Pagination
    page := getIntParam(c, "page", 1)
    perPage := getIntParam(c, "perPage", 50)

    // Filters
    search := c.QueryParam("search") // Search email/name

    users, total := api.cp.GetUsers(page, perPage, search)

    return c.JSON(200, map[string]interface{}{
        "users": users,
        "total": total,
        "page":  page,
        "perPage": perPage,
    })
}
```

**UI**:
```
┌──────────────────────────────────────────────────────────────┐
│  Users                               🔍 [Search users...]     │
├──────────────────────────────────────────────────────────────┤
│  Email              Name        Tenants   Created    Actions │
│  ──────────────────────────────────────────────────────────  │
│  john@example.com   John Doe    2/3       2025-01-15 [View]  │
│  jane@startup.com   Jane Smith  5/10      2025-01-14 [View]  │
│  demo@test.com      Demo User   1/3       2025-01-13 [View]  │
│                                                               │
│  Showing 1-50 of 1,234 users                        [Next →] │
└──────────────────────────────────────────────────────────────┘
```

### View User Details

**Endpoint**: `GET /api/cluster/users/{id}`

```go
func (api *ClusterAdminAPI) GetUser(c echo.Context) error {
    userID := c.Param("id")

    user, err := api.cp.GetUser(userID)
    if err != nil {
        return c.JSON(404, map[string]string{"error": "User not found"})
    }

    // Get user's tenants
    tenants, _ := api.cp.GetTenantsByUserID(userID)

    // Get quota requests
    quotaRequests, _ := api.cp.GetQuotaRequestsByUserID(userID)

    return c.JSON(200, map[string]interface{}{
        "user":           user,
        "tenants":        tenants,
        "quota_requests": quotaRequests,
    })
}
```

**UI**:
```
┌──────────────────────────────────────────────────────────────┐
│  User: john@example.com                      [Edit] [Delete] │
├──────────────────────────────────────────────────────────────┤
│  ID: user_abc123                                              │
│  Name: John Doe                                               │
│  Company: ACME Corp                                           │
│  Email: john@example.com ✓ Verified                          │
│  Registered: 2025-01-15 10:30 AM                             │
│  Last Login: 2025-10-09 08:45 AM                             │
│  ──────────────────────────────────────────────────────────  │
│  Quotas:                                       [Edit Quotas] │
│    Max Tenants: 3 (2 used)                                   │
│    Default Storage/Tenant: 1 GB                              │
│    Default API Requests: Unlimited                           │
│  ──────────────────────────────────────────────────────────  │
│  Tenants (2):                                                │
│    • my-app.platform.com      (🟢, 245MB, node_12)           │
│    • staging.platform.com     (🟢, 89MB, node_12)            │
│  ──────────────────────────────────────────────────────────  │
│  Quota Requests:                                             │
│    ⏳ Pending: Increase max tenants 3 → 5                    │
│       Reason: "Growing customer base"                        │
│       [Approve] [Reject]                                     │
│  ──────────────────────────────────────────────────────────  │
│  Actions:                                                    │
│    [Impersonate User]  [Adjust Quotas]  [View Audit Logs]   │
└──────────────────────────────────────────────────────────────┘
```

### Update User Quotas

**Endpoint**: `PUT /api/cluster/users/{id}/quotas`

```go
type UpdateQuotasRequest struct {
    MaxTenants                int   `json:"max_tenants"`
    DefaultStoragePerTenant   int64 `json:"default_storage_per_tenant"`
    DefaultAPIRequestsPerHour int   `json:"default_api_requests_per_hour"`
}

func (api *ClusterAdminAPI) UpdateUserQuotas(c echo.Context) error {
    admin := getAdminFromContext(c)
    userID := c.Param("id")

    var req UpdateQuotasRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // Get user
    user, err := api.cp.GetUser(userID)
    if err != nil {
        return c.JSON(404, map[string]string{"error": "User not found"})
    }

    // Update quotas
    user.Quotas.MaxTenants = req.MaxTenants
    user.Quotas.DefaultStoragePerTenant = req.DefaultStoragePerTenant
    user.Quotas.DefaultAPIRequestsPerHour = req.DefaultAPIRequestsPerHour

    // Save via Raft
    api.cp.UpdateUser(user)

    // Audit log
    api.cp.AuditLog("quota_update", map[string]interface{}{
        "admin_id": admin.ID,
        "user_id":  userID,
        "quotas":   req,
    })

    return c.JSON(200, user)
}
```

---

## Impersonation

### Generate Impersonation Token

**Endpoint**: `POST /api/cluster/users/{id}/impersonate`

```go
func (api *ClusterAdminAPI) ImpersonateUser(c echo.Context) error {
    admin := getAdminFromContext(c)
    userID := c.Param("id")

    // Get user
    user, err := api.cp.GetUser(userID)
    if err != nil {
        return c.JSON(404, map[string]string{"error": "User not found"})
    }

    // Generate impersonation JWT
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "admin_id":           admin.ID,
        "impersonate_user_id": user.ID,
        "permissions":        []string{"*"},
        "exp":                time.Now().Add(2 * time.Hour).Unix(),
    })

    tokenString, _ := token.SignedString([]byte(api.config.JWTSecret))

    // Audit log
    api.cp.AuditLog("impersonation_start", map[string]interface{}{
        "admin_id": admin.ID,
        "user_id":  userID,
    })

    // Return URL for user dashboard
    impersonateURL := fmt.Sprintf("https://app.platform.com/?impersonate=%s", tokenString)

    return c.JSON(200, map[string]interface{}{
        "token": tokenString,
        "url":   impersonateURL,
        "message": "Impersonation token valid for 2 hours",
    })
}
```

**User Dashboard when impersonating**:
```
┌──────────────────────────────────────────────────────────────┐
│  ⚠️  IMPERSONATING: john@example.com      [Exit Impersonation]│
│  Admin: admin@platform.com                                   │
├──────────────────────────────────────────────────────────────┤
│  (User's normal dashboard appears here)                      │
│  Admin can see everything the user sees                      │
│  Admin can create tenants on user's behalf                   │
└──────────────────────────────────────────────────────────────┘
```

---

## Quota Request Management

### List Pending Requests

**Endpoint**: `GET /api/cluster/quota-requests?status=pending`

```go
func (api *ClusterAdminAPI) ListQuotaRequests(c echo.Context) error {
    status := c.QueryParam("status") // pending, approved, rejected, all

    requests, _ := api.cp.GetQuotaRequests(status)

    return c.JSON(200, map[string]interface{}{
        "requests": requests,
        "count":    len(requests),
    })
}
```

**UI**:
```
┌──────────────────────────────────────────────────────────────┐
│  Quota Requests                     [Pending: 12] [All: 245] │
├──────────────────────────────────────────────────────────────┤
│  User              Request         Current → Requested       │
│  ──────────────────────────────────────────────────────────  │
│  john@example.com  Max Tenants     3       → 5               │
│  Reason: Growing customer base, need staging + production    │
│  Requested: 2025-10-08 2:34 PM                               │
│  [Approve] [Reject]                                          │
│  ──────────────────────────────────────────────────────────  │
│  jane@startup.com  Storage/Tenant  1GB     → 5GB            │
│  Tenant: my-app.platform.com                                 │
│  Reason: Large media files for e-commerce site               │
│  Requested: 2025-10-08 11:22 AM                              │
│  [Approve] [Reject]                                          │
└──────────────────────────────────────────────────────────────┘
```

### Approve Request

**Endpoint**: `PUT /api/cluster/quota-requests/{id}/approve`

```go
func (api *ClusterAdminAPI) ApproveQuotaRequest(c echo.Context) error {
    admin := getAdminFromContext(c)
    requestID := c.Param("id")

    // Get request
    req, err := api.cp.GetQuotaRequest(requestID)
    if err != nil {
        return c.JSON(404, map[string]string{"error": "Request not found"})
    }

    // Get user
    user, _ := api.cp.GetUserByQuotaRequest(requestID)

    // Apply quota change
    switch req.Type {
    case "max_tenants":
        user.Quotas.MaxTenants = int(req.RequestedValue)
    case "tenant_storage":
        if req.TenantID != "" {
            // Update specific tenant
            tenant, _ := api.cp.GetTenant(req.TenantID)
            tenant.Quotas.MaxStorage = req.RequestedValue
            api.cp.UpdateTenant(tenant)
        } else {
            // Update default
            user.Quotas.DefaultStoragePerTenant = req.RequestedValue
        }
    }

    // Update request status
    req.Status = "approved"
    req.ReviewedAt = time.Now()
    req.ReviewedBy = admin.ID

    // Save via Raft
    api.cp.UpdateUser(user)
    api.cp.UpdateQuotaRequest(req)

    // Clear pending request from user
    user.Quotas.PendingQuotaRequest = nil
    api.cp.UpdateUser(user)

    // Notify user (email)
    api.notifyQuotaRequestApproved(user, req)

    // Audit log
    api.cp.AuditLog("quota_request_approved", map[string]interface{}{
        "admin_id":   admin.ID,
        "request_id": requestID,
        "user_id":    user.ID,
    })

    return c.JSON(200, map[string]string{
        "message": "Quota request approved",
    })
}
```

### Reject Request

**Endpoint**: `PUT /api/cluster/quota-requests/{id}/reject`

```go
type RejectQuotaRequestRequest struct {
    Reason string `json:"reason"` // Optional reason for rejection
}

func (api *ClusterAdminAPI) RejectQuotaRequest(c echo.Context) error {
    admin := getAdminFromContext(c)
    requestID := c.Param("id")

    var body RejectQuotaRequestRequest
    c.Bind(&body)

    // Get request
    req, err := api.cp.GetQuotaRequest(requestID)
    if err != nil {
        return c.JSON(404, map[string]string{"error": "Request not found"})
    }

    // Update request status
    req.Status = "rejected"
    req.ReviewedAt = time.Now()
    req.ReviewedBy = admin.ID

    api.cp.UpdateQuotaRequest(req)

    // Clear pending request from user
    user, _ := api.cp.GetUserByQuotaRequest(requestID)
    user.Quotas.PendingQuotaRequest = nil
    api.cp.UpdateUser(user)

    // Notify user
    api.notifyQuotaRequestRejected(user, req, body.Reason)

    return c.JSON(200, map[string]string{
        "message": "Quota request rejected",
    })
}
```

---

## System Monitoring

### Cluster Health

**Endpoint**: `GET /api/cluster/health`

```go
func (api *ClusterAdminAPI) GetClusterHealth(c echo.Context) error {
    // Get all nodes
    nodes, _ := api.cp.GetAllNodes()

    var healthyNodes, degradedNodes, downNodes int
    for _, node := range nodes {
        switch node.Status {
        case NodeStatusHealthy:
            healthyNodes++
        case NodeStatusDegraded:
            degradedNodes++
        case NodeStatusDown:
            downNodes++
        }
    }

    // Get control plane status
    cpStatus := api.cp.GetRaftStatus()

    return c.JSON(200, map[string]interface{}{
        "status": "healthy",
        "control_plane": map[string]interface{}{
            "leader": cpStatus.Leader,
            "nodes":  cpStatus.Nodes,
        },
        "tenant_nodes": map[string]interface{}{
            "total":    len(nodes),
            "healthy":  healthyNodes,
            "degraded": degradedNodes,
            "down":     downNodes,
        },
    })
}
```

### System Metrics

**Endpoint**: `GET /api/cluster/metrics`

```go
func (api *ClusterAdminAPI) GetSystemMetrics(c echo.Context) error {
    // Get aggregate metrics
    totalUsers, _ := api.cp.CountUsers()
    totalTenants, _ := api.cp.CountTenants()

    // API requests (last 24h)
    apiRequests := api.cp.GetAPIRequestCount(24 * time.Hour)

    // Storage usage
    totalStorage := api.cp.GetTotalStorageUsage()

    return c.JSON(200, map[string]interface{}{
        "users":         totalUsers,
        "tenants":       totalTenants,
        "api_requests":  apiRequests,
        "storage_used":  totalStorage,
    })
}
```

---

## Admin Management

### List Admins

**Endpoint**: `GET /api/cluster/admins`

```go
func (api *ClusterAdminAPI) ListAdmins(c echo.Context) error {
    admins, _ := api.cp.GetAllAdmins()

    // Don't expose token hashes
    for i := range admins {
        admins[i].TokenHash = ""
    }

    return c.JSON(200, map[string]interface{}{
        "admins": admins,
    })
}
```

### Delete Admin

**Endpoint**: `DELETE /api/cluster/admins/{id}`

```go
func (api *ClusterAdminAPI) DeleteAdmin(c echo.Context) error {
    caller := getAdminFromContext(c)
    adminID := c.Param("id")

    // Cannot delete self
    if caller.ID == adminID {
        return c.JSON(400, map[string]string{
            "error": "Cannot delete yourself",
        })
    }

    // Delete
    api.cp.DeleteAdmin(adminID)

    // Audit log
    api.cp.AuditLog("admin_deleted", map[string]interface{}{
        "deleted_by": caller.ID,
        "deleted_admin": adminID,
    })

    return c.JSON(200, map[string]string{
        "message": "Admin deleted successfully",
    })
}
```

---

## Next: SSO & Hooks

See:
- [05-sso-tenant-access.md](05-sso-tenant-access.md) - SSO implementation details
- [07-hooks-database.md](07-hooks-database.md) - Database-backed hooks system
