package cluster_admin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
	"github.com/pocketbase/pocketbase/core/enterprise/auth"
	"github.com/pocketbase/pocketbase/core/enterprise/control_plane"
)

// API handles cluster admin API requests
type API struct {
	cp         *control_plane.ControlPlane
	jwtManager *auth.JWTManager
	adminTokens map[string]*enterprise.AdminToken // In-memory admin token storage
	logger     *log.Logger
}

// NewAPI creates a new cluster admin API handler
func NewAPI(cp *control_plane.ControlPlane, jwtManager *auth.JWTManager) *API {
	return &API{
		cp:          cp,
		jwtManager:  jwtManager,
		adminTokens: make(map[string]*enterprise.AdminToken),
		logger:      log.Default(),
	}
}

// GenerateAdminTokenRequest represents a request to generate an admin token
type GenerateAdminTokenRequest struct {
	Name string `json:"name"`
}

// UpdateUserQuotaRequest represents a request to update user quotas
type UpdateUserQuotaRequest struct {
	MaxTenants          *int   `json:"maxTenants,omitempty"`
	MaxStoragePerTenant *int64 `json:"maxStoragePerTenant,omitempty"`
	MaxAPIRequestsDaily *int64 `json:"maxApiRequestsDaily,omitempty"`
}

// ValidateAdminToken checks if an admin token is valid
func (api *API) ValidateAdminToken(token string) bool {
	adminToken, exists := api.adminTokens[token]
	if !exists {
		return false
	}

	// Update last used
	now := time.Now()
	adminToken.LastUsed = &now
	return true
}

// HandleGenerateAdminToken generates a new admin token
func (api *API) HandleGenerateAdminToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GenerateAdminTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Token name is required", http.StatusBadRequest)
		return
	}

	// Generate token
	tokenString := api.jwtManager.GenerateAdminToken(req.Name)

	adminToken := &enterprise.AdminToken{
		Token:   tokenString,
		Name:    req.Name,
		Created: time.Now(),
	}

	// Store token
	api.adminTokens[tokenString] = adminToken

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":   tokenString,
		"name":    req.Name,
		"created": adminToken.Created,
		"message": "Admin token generated. Keep this secure!",
	})
}

// HandleListUsers lists all cluster users
func (api *API) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse pagination parameters
	limit := 50 // default limit
	offset := 0 // default offset

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil {
			http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
			return
		}
		// Enforce maximum limit
		if limit > 1000 {
			limit = 1000
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if _, err := fmt.Sscanf(offsetStr, "%d", &offset); err != nil {
			http.Error(w, "Invalid offset parameter", http.StatusBadRequest)
			return
		}
	}

	// Get users from control plane
	users, total, err := api.cp.ListUsers(limit, offset)
	if err != nil {
		api.logger.Printf("Failed to list users: %v", err)
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users":  users,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// HandleGetUser retrieves a specific user
func (api *API) HandleGetUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, "userId parameter required", http.StatusBadRequest)
		return
	}

	user, err := api.cp.GetUser(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": user,
	})
}

// HandleUpdateUserQuota updates user quotas
func (api *API) HandleUpdateUserQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, "userId parameter required", http.StatusBadRequest)
		return
	}

	var req UpdateUserQuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user
	user, err := api.cp.GetUser(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Update quotas
	if req.MaxTenants != nil {
		user.MaxTenants = *req.MaxTenants
	}
	if req.MaxStoragePerTenant != nil {
		user.MaxStoragePerTenant = *req.MaxStoragePerTenant
	}
	if req.MaxAPIRequestsDaily != nil {
		user.MaxAPIRequestsDaily = *req.MaxAPIRequestsDaily
	}

	user.Updated = time.Now()

	// Save user
	if err := api.cp.UpdateUser(user); err != nil {
		api.logger.Printf("Failed to update user: %v", err)
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":    user,
		"message": "User quotas updated successfully",
	})
}

// HandleImpersonateUser generates a user token for impersonation
func (api *API) HandleImpersonateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user
	user, err := api.cp.GetUser(req.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Generate a short-lived impersonation token (1 hour)
	token, err := api.jwtManager.GenerateUserToken(user, 1)
	if err != nil {
		api.logger.Printf("Failed to generate impersonation token: %v", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":     token,
		"expiresIn": 3600,
		"user": map[string]interface{}{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		},
		"message": "Impersonation token generated. Valid for 1 hour.",
	})
}

// HandleListTenants lists all tenants in the system
func (api *API) HandleListTenants(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse pagination parameters
	limit := 50 // default limit
	offset := 0 // default offset

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil {
			http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
			return
		}
		// Enforce maximum limit
		if limit > 1000 {
			limit = 1000
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if _, err := fmt.Sscanf(offsetStr, "%d", &offset); err != nil {
			http.Error(w, "Invalid offset parameter", http.StatusBadRequest)
			return
		}
	}

	// Parse optional owner filter
	ownerUserID := r.URL.Query().Get("ownerId")

	// Get tenants from control plane
	tenants, total, err := api.cp.ListTenants(limit, offset, ownerUserID)
	if err != nil {
		api.logger.Printf("Failed to list tenants: %v", err)
		http.Error(w, "Failed to list tenants", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"tenants": tenants,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	}

	if ownerUserID != "" {
		response["ownerId"] = ownerUserID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGetTenant retrieves a specific tenant
func (api *API) HandleGetTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tenantID := r.URL.Query().Get("tenantId")
	if tenantID == "" {
		http.Error(w, "tenantId parameter required", http.StatusBadRequest)
		return
	}

	tenant, err := api.cp.GetTenant(tenantID)
	if err != nil {
		http.Error(w, "Tenant not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tenant": tenant,
	})
}

// HandleGetSystemStats returns system-wide statistics
func (api *API) HandleGetSystemStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodes := api.cp.GetNodes()

	// Calculate stats
	totalNodes := len(nodes)
	onlineNodes := 0
	totalTenants := 0

	for _, node := range nodes {
		if node.Status == "online" {
			onlineNodes++
		}
		totalTenants += node.ActiveTenants
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": map[string]interface{}{
			"total":  totalNodes,
			"online": onlineNodes,
		},
		"tenants": map[string]interface{}{
			"active": totalTenants,
		},
		"timestamp": time.Now(),
	})
}

// HandleListNodes lists all tenant nodes
func (api *API) HandleListNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodes := api.cp.GetNodes()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodes,
		"total": len(nodes),
	})
}

// HandleGetDiskStats returns disk usage statistics
func (api *API) HandleGetDiskStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	diskStats := api.cp.GetDiskStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"disk": diskStats,
		"timestamp": time.Now(),
	})
}

// HandleGetTenantActivity retrieves tenant activity and archiving status
func (api *API) HandleGetTenantActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tenantID := r.URL.Query().Get("tenantId")
	if tenantID == "" {
		http.Error(w, "tenantId parameter required", http.StatusBadRequest)
		return
	}

	// Get tenant activity from control plane
	activity, err := api.cp.GetTenantActivity(tenantID)
	if err != nil {
		http.Error(w, "Failed to get tenant activity", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"activity": activity,
	})
}

// HandleListInactiveTenants lists inactive tenants for archiving
func (api *API) HandleListInactiveTenants(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get days parameter (default 30 days)
	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		if _, err := fmt.Sscanf(daysStr, "%d", &days); err != nil {
			http.Error(w, "Invalid days parameter", http.StatusBadRequest)
			return
		}
	}

	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

	// Get inactive tenants from control plane
	activities, err := api.cp.ListInactiveTenants(since)
	if err != nil {
		http.Error(w, "Failed to list inactive tenants", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"inactiveTenants": activities,
		"total":           len(activities),
		"inactiveSince":   since,
	})
}

// HandleArchiveTenant manually archives a tenant
func (api *API) HandleArchiveTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TenantID string `json:"tenantId"`
		Tier     string `json:"tier"` // "warm" or "cold"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TenantID == "" {
		http.Error(w, "tenantId is required", http.StatusBadRequest)
		return
	}

	if req.Tier != "warm" && req.Tier != "cold" {
		http.Error(w, "tier must be 'warm' or 'cold'", http.StatusBadRequest)
		return
	}

	// Archive the tenant
	var tier enterprise.StorageTier
	if req.Tier == "warm" {
		tier = enterprise.StorageTierWarm
	} else {
		tier = enterprise.StorageTierCold
	}

	if err := api.cp.ArchiveTenant(req.TenantID, tier); err != nil {
		api.logger.Printf("Failed to archive tenant %s: %v", req.TenantID, err)
		http.Error(w, "Failed to archive tenant", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tenantId": req.TenantID,
		"tier":     req.Tier,
		"message":  fmt.Sprintf("Tenant archived to %s storage", req.Tier),
	})
}

// HandleRestoreTenant manually restores an archived tenant
func (api *API) HandleRestoreTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TenantID string `json:"tenantId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TenantID == "" {
		http.Error(w, "tenantId is required", http.StatusBadRequest)
		return
	}

	// Restore the tenant
	if err := api.cp.RestoreTenant(req.TenantID); err != nil {
		api.logger.Printf("Failed to restore tenant %s: %v", req.TenantID, err)
		http.Error(w, "Failed to restore tenant", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tenantId": req.TenantID,
		"message":  "Tenant restore initiated. This may take several minutes for cold storage.",
	})
}

// HandleGetArchiveStats returns archiving statistics
func (api *API) HandleGetArchiveStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get statistics from control plane
	hotCount, err := api.cp.CountTenantsByTier(enterprise.StorageTierHot)
	if err != nil {
		http.Error(w, "Failed to get statistics", http.StatusInternalServerError)
		return
	}

	warmCount, err := api.cp.CountTenantsByTier(enterprise.StorageTierWarm)
	if err != nil {
		http.Error(w, "Failed to get statistics", http.StatusInternalServerError)
		return
	}

	coldCount, err := api.cp.CountTenantsByTier(enterprise.StorageTierCold)
	if err != nil {
		http.Error(w, "Failed to get statistics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"storage": map[string]interface{}{
			"hot":  hotCount,
			"warm": warmCount,
			"cold": coldCount,
			"total": hotCount + warmCount + coldCount,
		},
		"timestamp": time.Now(),
	})
}
