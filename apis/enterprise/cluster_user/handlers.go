package cluster_user

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
	"github.com/pocketbase/pocketbase/core/enterprise/auth"
	"github.com/pocketbase/pocketbase/core/enterprise/control_plane"
	"github.com/pocketbase/pocketbase/core/enterprise/email"
	"golang.org/x/crypto/bcrypt"
)

// API handles cluster user API requests
type API struct {
	cp         *control_plane.ControlPlane
	jwtManager *auth.JWTManager
	logger     *log.Logger
}

// NewAPI creates a new cluster user API handler
func NewAPI(cp *control_plane.ControlPlane, jwtManager *auth.JWTManager) *API {
	return &API{
		cp:         cp,
		jwtManager: jwtManager,
		logger:     log.Default(),
	}
}

// SignupRequest represents a user signup request
type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// LoginRequest represents a user login request
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// CreateTenantRequest represents a tenant creation request
type CreateTenantRequest struct {
	ID     string `json:"id"`     // Desired tenant ID (e.g., "myapp")
	Domain string `json:"domain"` // Full domain (e.g., "myapp.platform.com")
}

// QuotaIncreaseRequest represents a quota increase request
type QuotaIncreaseRequestData struct {
	TenantID         string `json:"tenantId"`
	RequestedQuotaMB int64  `json:"requestedQuotaMb"`
	Reason           string `json:"reason"`
}

// HandleSignup handles user registration
func (api *API) HandleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" || req.Name == "" {
		http.Error(w, "Email, password, and name are required", http.StatusBadRequest)
		return
	}

	// Check if user already exists
	// Return a generic success message to prevent email enumeration
	existingUser, _ := api.cp.GetUserByEmail(req.Email)
	if existingUser != nil {
		// Log for debugging but don't reveal to user
		api.logger.Printf("[Signup] Attempted registration with existing email: %s", req.Email)

		// Return same response as success to prevent email enumeration
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "If this email is not already registered, an account has been created. Please check your email to verify your account.",
		})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		api.logger.Printf("Failed to hash password: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create user with default quotas
	maxTenants, maxStoragePerTenant, maxAPIRequestsDaily := enterprise.DefaultUserQuotas()

	user := &enterprise.ClusterUser{
		ID:                  enterprise.GenerateUserID(),
		Email:               req.Email,
		Name:                req.Name,
		PasswordHash:        string(hashedPassword),
		Verified:            false, // Will be set to true after email verification
		MaxTenants:          maxTenants,
		MaxStoragePerTenant: maxStoragePerTenant,
		MaxAPIRequestsDaily: maxAPIRequestsDaily,
		Created:             time.Now(),
		Updated:             time.Now(),
	}

	// Save user
	if err := api.cp.CreateUser(user); err != nil {
		api.logger.Printf("Failed to create user: %v", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Generate verification token
	verificationTokenStr, err := email.GenerateToken()
	if err != nil {
		api.logger.Printf("Failed to generate verification token: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Save verification token (expires in 24 hours)
	verificationToken := &enterprise.VerificationToken{
		Token:   verificationTokenStr,
		UserID:  user.ID,
		Email:   user.Email,
		Expires: time.Now().Add(24 * time.Hour),
		Created: time.Now(),
		Used:    false,
	}

	if err := api.cp.SaveVerificationToken(verificationToken); err != nil {
		api.logger.Printf("Failed to save verification token: %v", err)
		// Don't fail user creation if token save fails
	}

	// Log verification link for development (in production, this would be sent via email)
	verificationURL := "http://localhost:8095/api/enterprise/users/verify?token=" + verificationTokenStr
	api.logger.Printf("[Verification] New user registered: %s", user.Email)
	api.logger.Printf("[Verification] Verification URL: %s", verificationURL)

	// Generate JWT token
	token, err := api.jwtManager.GenerateUserToken(user, 24)
	if err != nil {
		api.logger.Printf("Failed to generate token: %v", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": map[string]interface{}{
			"id":      user.ID,
			"email":   user.Email,
			"name":    user.Name,
			"created": user.Created,
		},
		"token":   token,
		"message": "Account created successfully. Please check your email to verify your account.",
	})
}

// HandleLogin handles user login
func (api *API) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user by email
	user, err := api.cp.GetUserByEmail(req.Email)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Update last login
	now := time.Now()
	user.LastLogin = &now
	api.cp.UpdateUser(user)

	// Generate JWT token
	token, err := api.jwtManager.GenerateUserToken(user, 24)
	if err != nil {
		api.logger.Printf("Failed to generate token: %v", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": map[string]interface{}{
			"id":       user.ID,
			"email":    user.Email,
			"name":     user.Name,
			"verified": user.Verified,
		},
		"token": token,
	})
}

// HandleGetProfile returns the current user's profile
func (api *API) HandleGetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := api.cp.GetUser(claims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":                  user.ID,
		"email":               user.Email,
		"name":                user.Name,
		"verified":            user.Verified,
		"maxTenants":          user.MaxTenants,
		"maxStoragePerTenant": user.MaxStoragePerTenant,
		"maxApiRequestsDaily": user.MaxAPIRequestsDaily,
		"created":             user.Created,
		"lastLogin":           user.LastLogin,
	})
}

// HandleCreateTenant creates a new tenant for the user
func (api *API) HandleCreateTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.ID == "" || req.Domain == "" {
		http.Error(w, "ID and domain are required", http.StatusBadRequest)
		return
	}

	// Generate tenant ID if not provided
	tenantID := enterprise.GenerateTenantID()
	if req.ID != "" {
		tenantID = "tenant_" + req.ID
	}

	// Create tenant
	tenant := &enterprise.Tenant{
		ID:          tenantID,
		Domain:      req.Domain,
		OwnerUserID: claims.UserID,
		Status:      enterprise.TenantStatusCreated,
		Created:     time.Now(),
		Updated:     time.Now(),
	}

	if err := api.cp.CreateTenant(tenant); err != nil {
		api.logger.Printf("Failed to create tenant: %v", err)

		// Check if it's a quota error
		if _, ok := err.(*enterprise.QuotaError); ok {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		http.Error(w, "Failed to create tenant", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tenant": tenant,
		"message": "Tenant created successfully",
	})
}

// HandleListTenants lists all tenants owned by the user
func (api *API) HandleListTenants(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// List all tenants owned by this user
	tenants, total, err := api.cp.ListTenants(0, 0, claims.UserID)
	if err != nil {
		api.logger.Printf("Failed to list tenants: %v", err)
		http.Error(w, "Failed to list tenants", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tenants": tenants,
		"total":   total,
	})
}

// HandleGenerateTenantSSO generates a SSO token for accessing tenant admin
func (api *API) HandleGenerateTenantSSO(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get tenant ID from request
	var req struct {
		TenantID string `json:"tenantId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Verify user owns this tenant
	tenant, err := api.cp.GetTenant(req.TenantID)
	if err != nil {
		http.Error(w, "Tenant not found", http.StatusNotFound)
		return
	}

	if tenant.OwnerUserID != claims.UserID {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Get user
	user, err := api.cp.GetUser(claims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Generate SSO token
	ssoToken, err := api.jwtManager.GenerateTenantAdminToken(user, req.TenantID)
	if err != nil {
		api.logger.Printf("Failed to generate SSO token: %v", err)
		http.Error(w, "Failed to generate SSO token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ssoToken":  ssoToken,
		"tenantUrl": "https://" + tenant.Domain + "/_/",
		"expiresIn": 3600, // 1 hour
	})
}

// HandleVerifyEmail handles email verification
func (api *API) HandleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "Verification token required", http.StatusBadRequest)
		return
	}

	// Atomically validate and mark token as used (prevents double-use race condition)
	verificationToken, err := api.cp.UseVerificationTokenAtomically(tokenStr)
	if err != nil {
		api.logger.Printf("[Verification] Invalid token: %v", err)
		http.Error(w, "Invalid or expired verification token", http.StatusBadRequest)
		return
	}

	// Get user
	user, err := api.cp.GetUser(verificationToken.UserID)
	if err != nil {
		api.logger.Printf("[Verification] User not found: %v", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Check if already verified (could happen if same user verifies with different token)
	if user.Verified {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Email already verified. You can log in.",
		})
		return
	}

	// Mark user as verified
	user.Verified = true
	user.Updated = time.Now()

	if err := api.cp.UpdateUser(user); err != nil {
		api.logger.Printf("[Verification] Failed to update user: %v", err)
		http.Error(w, "Failed to verify email", http.StatusInternalServerError)
		return
	}

	// Token is already marked as used by UseVerificationTokenAtomically

	api.logger.Printf("[Verification] Email verified for user: %s", user.Email)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Email verification successful! You can now log in.",
	})
}

// HandleResendVerification resends the verification email
func (api *API) HandleResendVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user
	user, err := api.cp.GetUserByEmail(req.Email)
	if err != nil {
		// Don't reveal whether email exists
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "If the email exists, a verification link has been sent.",
		})
		return
	}

	// If already verified, return same generic message to prevent enumeration
	if user.Verified {
		api.logger.Printf("[Verification] Resend attempted for already verified email: %s", req.Email)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "If the email exists and is not yet verified, a verification link has been sent.",
		})
		return
	}

	// Generate new verification token
	verificationTokenStr, err := email.GenerateToken()
	if err != nil {
		api.logger.Printf("Failed to generate token: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Save verification token (expires in 24 hours)
	verificationToken := &enterprise.VerificationToken{
		Token:   verificationTokenStr,
		UserID:  user.ID,
		Email:   user.Email,
		Expires: time.Now().Add(24 * time.Hour),
		Created: time.Now(),
		Used:    false,
	}

	if err := api.cp.SaveVerificationToken(verificationToken); err != nil {
		api.logger.Printf("Failed to save verification token: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Log verification link for development (in production, this would be sent via email)
	verificationURL := "http://localhost:8095/api/enterprise/users/verify?token=" + verificationTokenStr
	api.logger.Printf("[Verification] Resending verification for %s", user.Email)
	api.logger.Printf("[Verification] Verification URL: %s", verificationURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Verification email sent. Please check your inbox.",
	})
}
