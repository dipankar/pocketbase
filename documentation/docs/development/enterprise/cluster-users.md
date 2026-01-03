# Cluster Users: Self-Service Platform

## Overview

Cluster users are **customers** of the platform who can:

- Register and create accounts
- Create tenants (within quota limits)
- Access all their tenant admin UIs without re-authentication (SSO)
- Request quota increases
- Monitor aggregate metrics across tenants

This is a **self-service SaaS model** where users manage their own infrastructure.

---

## Data Model

### ClusterUser

```go
type ClusterUser struct {
    ID           string              `json:"id"`           // user_abc123
    Email        string              `json:"email"`        // user@example.com
    Name         string              `json:"name"`
    PasswordHash string              `json:"password_hash,omitempty"`

    // Quota limits
    Quotas       UserQuotas          `json:"quotas"`

    // Metadata
    Company      string              `json:"company"`
    Verified     bool                `json:"verified"`
    Metadata     map[string]interface{} `json:"metadata"`

    Created      time.Time           `json:"created"`
    Updated      time.Time           `json:"updated"`
    LastLogin    time.Time           `json:"last_login"`
}

type UserQuotas struct {
    // Tenant limits
    MaxTenants          int    `json:"max_tenants"`           // Default: 3
    CurrentTenants      int    `json:"current_tenants"`       // Current usage

    // Per-tenant resource limits (applied to new tenants)
    DefaultStoragePerTenant   int64  `json:"default_storage_per_tenant"`   // bytes (default: 1GB)
    DefaultAPIRequestsPerHour int    `json:"default_api_requests_per_hour"` // default: unlimited
    DefaultMaxUsers           int    `json:"default_max_users"`             // default: unlimited

    // Pending quota request
    PendingQuotaRequest *QuotaRequest `json:"pending_quota_request,omitempty"`
}

type QuotaRequest struct {
    ID           string    `json:"id"`
    Type         string    `json:"type"`         // "max_tenants", "tenant_storage", "tenant_api_requests"
    TenantID     string    `json:"tenant_id,omitempty"` // If requesting for specific tenant
    CurrentValue int64     `json:"current_value"`
    RequestedValue int64   `json:"requested_value"`
    Reason       string    `json:"reason"`
    Status       string    `json:"status"`       // pending, approved, rejected
    RequestedAt  time.Time `json:"requested_at"`
    ReviewedAt   time.Time `json:"reviewed_at,omitempty"`
    ReviewedBy   string    `json:"reviewed_by,omitempty"` // admin_id
}
```

### Default Quotas

```go
var DefaultUserQuotas = UserQuotas{
    MaxTenants:                3,
    CurrentTenants:            0,
    DefaultStoragePerTenant:   1 * 1024 * 1024 * 1024, // 1GB
    DefaultAPIRequestsPerHour: 0,                      // unlimited (fair use)
    DefaultMaxUsers:           0,                      // unlimited
}
```

---

## User Lifecycle

### 1. Registration

**Endpoint**: `POST /api/auth/signup`

```go
type SignupRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
    Name     string `json:"name"`
    Company  string `json:"company,omitempty"`
}

func (api *ClusterUserAPI) Signup(c echo.Context) error {
    var req SignupRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // Validate email uniqueness
    existing, _ := api.cp.GetUserByEmail(req.Email)
    if existing != nil {
        return c.JSON(400, map[string]string{
            "error": "Email already registered",
        })
    }

    // Hash password
    hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), 12)

    // Create user with default quotas
    user := &ClusterUser{
        ID:           generateID("user_"),
        Email:        req.Email,
        Name:         req.Name,
        Company:      req.Company,
        PasswordHash: string(hash),
        Quotas:       DefaultUserQuotas,
        Verified:     false,
        Created:      time.Now(),
    }

    // Save via Raft
    if err := api.cp.CreateUser(user); err != nil {
        return err
    }

    // Send verification email
    api.sendVerificationEmail(user)

    return c.JSON(201, map[string]interface{}{
        "id":    user.ID,
        "email": user.Email,
        "message": "Account created. Please check your email to verify.",
    })
}
```

**User Flow**:

```
1. User visits: app.platform.com/signup
2. Fills form: email, password, name, company
3. Submits form
4. Account created with default quotas
5. Verification email sent
6. User clicks verification link
7. Account verified, can now login
```

### 2. Login

**Endpoint**: `POST /api/auth/login`

```go
type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type LoginResponse struct {
    Token string       `json:"token"`
    User  *ClusterUser `json:"user"`
}

func (api *ClusterUserAPI) Login(c echo.Context) error {
    var req LoginRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // Get user by email
    user, err := api.cp.GetUserByEmail(req.Email)
    if err != nil {
        return c.JSON(401, map[string]string{
            "error": "Invalid credentials",
        })
    }

    // Verify password
    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
        return c.JSON(401, map[string]string{
            "error": "Invalid credentials",
        })
    }

    // Generate JWT
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "user_id": user.ID,
        "email":   user.Email,
        "exp":     time.Now().Add(24 * time.Hour).Unix(),
    })

    tokenString, _ := token.SignedString([]byte(api.config.JWTSecret))

    // Update last login
    user.LastLogin = time.Now()
    api.cp.UpdateUser(user)

    return c.JSON(200, LoginResponse{
        Token: tokenString,
        User:  user,
    })
}
```

---

## Tenant Management

### 1. Create Tenant

**Endpoint**: `POST /api/tenants`

```go
type CreateTenantRequest struct {
    Name   string `json:"name"`   // e.g., "my-app"
    Region string `json:"region"` // e.g., "us-east-1"
}

func (api *ClusterUserAPI) CreateTenant(c echo.Context) error {
    user := getUserFromContext(c) // Extract from JWT

    var req CreateTenantRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // Check quota
    if user.Quotas.CurrentTenants >= user.Quotas.MaxTenants {
        return c.JSON(403, map[string]interface{}{
            "error": "Tenant quota exceeded",
            "current": user.Quotas.CurrentTenants,
            "max": user.Quotas.MaxTenants,
            "message": "Please request a quota increase",
        })
    }

    // Generate domain
    domain := fmt.Sprintf("%s.platform.com", sanitizeName(req.Name))

    // Check domain availability
    existing, _ := api.cp.GetTenantByDomain(domain)
    if existing != nil {
        return c.JSON(400, map[string]string{
            "error": "Domain already taken",
        })
    }

    // Create tenant
    tenant := &TenantRecord{
        ID:      generateID("tenant_"),
        Domain:  domain,
        Status:  TenantStatusCreated,
        UserID:  user.ID,
        Quotas: TenantQuotas{
            MaxStorage:     user.Quotas.DefaultStoragePerTenant,
            MaxAPIRequests: user.Quotas.DefaultAPIRequestsPerHour,
            MaxUsers:       user.Quotas.DefaultMaxUsers,
        },
        Created: time.Now(),
    }

    // Save via Raft
    if err := api.cp.CreateTenant(tenant); err != nil {
        return err
    }

    // Assign to node (via placement service)
    nodeID, err := api.cp.PlacementService.AssignTenant(tenant.ID)
    if err != nil {
        return err
    }

    tenant.Status = TenantStatusAssigning
    tenant.NodeID = nodeID
    api.cp.UpdateTenant(tenant)

    // Increment user's tenant count
    user.Quotas.CurrentTenants++
    api.cp.UpdateUser(user)

    // Notify node to deploy tenant (via Mangos PUB)
    api.cp.NotifyNodeDeployTenant(nodeID, tenant.ID)

    return c.JSON(201, tenant)
}
```

**User Flow**:

```
1. User clicks "Create Tenant" in dashboard
2. Fills form: name, region
3. System checks quota (2/3 -> OK)
4. Tenant created in control plane
5. Assigned to node via placement service
6. Node loads tenant from S3 (or creates new)
7. User can now access tenant admin UI
```

### 2. List Tenants

**Endpoint**: `GET /api/tenants`

```go
func (api *ClusterUserAPI) ListTenants(c echo.Context) error {
    user := getUserFromContext(c)

    // Get all tenants owned by user
    tenants, err := api.cp.GetTenantsByUserID(user.ID)
    if err != nil {
        return err
    }

    // Enrich with live metrics
    for i, tenant := range tenants {
        metrics := api.cp.GetTenantMetrics(tenant.ID)
        tenants[i].Metrics = metrics
    }

    return c.JSON(200, map[string]interface{}{
        "tenants": tenants,
        "quota": map[string]int{
            "current": user.Quotas.CurrentTenants,
            "max":     user.Quotas.MaxTenants,
        },
    })
}
```

### 3. Delete Tenant

**Endpoint**: `DELETE /api/tenants/{id}`

```go
func (api *ClusterUserAPI) DeleteTenant(c echo.Context) error {
    user := getUserFromContext(c)
    tenantID := c.Param("id")

    // Get tenant
    tenant, err := api.cp.GetTenant(tenantID)
    if err != nil {
        return c.JSON(404, map[string]string{"error": "Tenant not found"})
    }

    // Check ownership
    if tenant.UserID != user.ID {
        return c.JSON(403, map[string]string{"error": "Not authorized"})
    }

    // Mark as deleted
    tenant.Status = TenantStatusDeleted
    api.cp.UpdateTenant(tenant)

    // Notify node to stop tenant
    api.cp.NotifyNodeStopTenant(tenant.NodeID, tenant.ID)

    // Decrement user's tenant count
    user.Quotas.CurrentTenants--
    api.cp.UpdateUser(user)

    // Archive data to S3 (optional: can delete immediately)
    api.cp.ArchiveTenant(tenant.ID)

    return c.JSON(200, map[string]string{
        "message": "Tenant deleted successfully",
    })
}
```

---

## SSO: Accessing Tenant Admin

### Generate SSO Token

**Endpoint**: `POST /api/tenants/{id}/sso`

```go
func (api *ClusterUserAPI) GenerateSSOToken(c echo.Context) error {
    user := getUserFromContext(c)
    tenantID := c.Param("id")

    // Get tenant
    tenant, err := api.cp.GetTenant(tenantID)
    if err != nil {
        return c.JSON(404, map[string]string{"error": "Tenant not found"})
    }

    // Check ownership
    if tenant.UserID != user.ID {
        return c.JSON(403, map[string]string{"error": "Not authorized"})
    }

    // Generate delegated JWT (valid for 1 hour)
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "user_id":      user.ID,
        "tenant_id":    tenant.ID,
        "admin_access": true,
        "exp":          time.Now().Add(1 * time.Hour).Unix(),
    })

    tokenString, _ := token.SignedString([]byte(api.config.JWTSecret))

    // Return SSO URL
    ssoURL := fmt.Sprintf("https://%s/_/?sso_token=%s", tenant.Domain, tokenString)

    return c.JSON(200, map[string]string{
        "token": tokenString,
        "url":   ssoURL,
    })
}
```

**User Flow**:

```
1. User clicks "Open Admin" on tenant in dashboard
2. Frontend calls: POST /api/tenants/{id}/sso
3. Backend generates JWT with tenant context
4. Frontend redirects to: tenant.platform.com/_/?sso_token={jwt}
5. Tenant admin UI validates JWT:
   - Check signature (signed by control plane)
   - Check tenant_id matches current tenant
   - Check admin_access flag
6. Create admin session in tenant (no password needed)
7. User is now logged into tenant admin UI
```

---

## Quota Management

### Request Quota Increase

**Endpoint**: `POST /api/quotas/request`

```go
type QuotaIncreaseRequest struct {
    Type           string `json:"type"`            // "max_tenants", "tenant_storage"
    TenantID       string `json:"tenant_id,omitempty"`
    RequestedValue int64  `json:"requested_value"`
    Reason         string `json:"reason"`
}

func (api *ClusterUserAPI) RequestQuotaIncrease(c echo.Context) error {
    user := getUserFromContext(c)

    var req QuotaIncreaseRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // Check if user already has pending request
    if user.Quotas.PendingQuotaRequest != nil {
        return c.JSON(400, map[string]string{
            "error": "You already have a pending quota request",
        })
    }

    // Determine current value
    var currentValue int64
    switch req.Type {
    case "max_tenants":
        currentValue = int64(user.Quotas.MaxTenants)
    case "tenant_storage":
        if req.TenantID != "" {
            tenant, _ := api.cp.GetTenant(req.TenantID)
            currentValue = tenant.Quotas.MaxStorage
        } else {
            currentValue = user.Quotas.DefaultStoragePerTenant
        }
    }

    // Create quota request
    quotaReq := &QuotaRequest{
        ID:             generateID("qreq_"),
        Type:           req.Type,
        TenantID:       req.TenantID,
        CurrentValue:   currentValue,
        RequestedValue: req.RequestedValue,
        Reason:         req.Reason,
        Status:         "pending",
        RequestedAt:    time.Now(),
    }

    // Save via Raft
    api.cp.CreateQuotaRequest(quotaReq)

    // Link to user
    user.Quotas.PendingQuotaRequest = quotaReq
    api.cp.UpdateUser(user)

    // Notify admins (email, Slack, etc.)
    api.notifyAdminsOfQuotaRequest(user, quotaReq)

    return c.JSON(201, quotaReq)
}
```

### Get Quotas

**Endpoint**: `GET /api/quotas`

```go
func (api *ClusterUserAPI) GetQuotas(c echo.Context) error {
    user := getUserFromContext(c)

    return c.JSON(200, map[string]interface{}{
        "tenants": map[string]interface{}{
            "current": user.Quotas.CurrentTenants,
            "max":     user.Quotas.MaxTenants,
        },
        "defaults": map[string]interface{}{
            "storage_per_tenant":    user.Quotas.DefaultStoragePerTenant,
            "api_requests_per_hour": user.Quotas.DefaultAPIRequestsPerHour,
            "max_users":             user.Quotas.DefaultMaxUsers,
        },
        "pending_request": user.Quotas.PendingQuotaRequest,
    })
}
```

---

## Metrics & Monitoring

### Aggregate Metrics

**Endpoint**: `GET /api/metrics`

```go
func (api *ClusterUserAPI) GetAggregateMetrics(c echo.Context) error {
    user := getUserFromContext(c)

    // Get all user's tenants
    tenants, _ := api.cp.GetTenantsByUserID(user.ID)

    // Aggregate metrics
    var totalStorage int64
    var totalAPIRequests int64
    var totalUsers int

    for _, tenant := range tenants {
        metrics := api.cp.GetTenantMetrics(tenant.ID)
        totalStorage += metrics.StorageUsed
        totalAPIRequests += metrics.APIRequests24h
        totalUsers += metrics.TotalUsers
    }

    return c.JSON(200, map[string]interface{}{
        "total_storage":      totalStorage,
        "total_api_requests": totalAPIRequests,
        "total_users":        totalUsers,
        "tenants_count":      len(tenants),
    })
}
```

## Next Steps

- [Cluster Admin](cluster-admin.md) - Platform operator dashboard
- [Hooks Database](hooks-database.md) - Database-backed hooks
