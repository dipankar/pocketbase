package enterprise

import (
	"log"
	"net/http"

	"github.com/pocketbase/pocketbase/apis/enterprise/cluster_admin"
	"github.com/pocketbase/pocketbase/apis/enterprise/cluster_user"
	"github.com/pocketbase/pocketbase/core/enterprise/auth"
	"github.com/pocketbase/pocketbase/core/enterprise/control_plane"
	"github.com/pocketbase/pocketbase/core/enterprise/health"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Router handles routing for enterprise APIs
type Router struct {
	mux        *http.ServeMux
	cp         *control_plane.ControlPlane
	userAPI    *cluster_user.API
	adminAPI   *cluster_admin.API
	jwtManager *auth.JWTManager
	logger     *log.Logger
}

// NewRouter creates a new enterprise API router
// jwtSecret should be loaded from config or environment variable (POCKETBASE_JWT_SECRET)
func NewRouter(cp *control_plane.ControlPlane, jwtSecret string) *Router {
	jwtManager := auth.NewJWTManager(jwtSecret)

	userAPI := cluster_user.NewAPI(cp, jwtManager)
	adminAPI := cluster_admin.NewAPI(cp, jwtManager)

	router := &Router{
		mux:        http.NewServeMux(),
		cp:         cp,
		userAPI:    userAPI,
		adminAPI:   adminAPI,
		jwtManager: jwtManager,
		logger:     log.Default(),
	}

	router.setupRoutes()
	return router
}

// ServeHTTP implements http.Handler
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Token")

	if req.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	r.mux.ServeHTTP(w, req)
}

// setupRoutes configures all API routes
func (r *Router) setupRoutes() {
	// Public cluster user routes (no auth required)
	r.mux.HandleFunc("/api/enterprise/users/signup", r.userAPI.HandleSignup)
	r.mux.HandleFunc("/api/enterprise/users/login", r.userAPI.HandleLogin)
	r.mux.HandleFunc("/api/enterprise/users/verify", r.userAPI.HandleVerifyEmail)
	r.mux.HandleFunc("/api/enterprise/users/resend-verification", r.userAPI.HandleResendVerification)

	// Protected cluster user routes (require user JWT)
	r.mux.Handle("/api/enterprise/users/profile", auth.RequireUserAuth(r.jwtManager)(http.HandlerFunc(r.userAPI.HandleGetProfile)))
	r.mux.Handle("/api/enterprise/users/tenants", r.handleUserTenants())
	r.mux.Handle("/api/enterprise/users/tenants/sso", auth.RequireUserAuth(r.jwtManager)(http.HandlerFunc(r.userAPI.HandleGenerateTenantSSO)))

	// Admin routes (require admin token)
	r.mux.HandleFunc("/api/enterprise/admin/tokens/generate", r.adminAPI.HandleGenerateAdminToken) // Bootstrap endpoint
	r.mux.Handle("/api/enterprise/admin/users", auth.RequireAdminAuth(r.adminAPI.ValidateAdminToken)(http.HandlerFunc(r.handleAdminUsers())))
	r.mux.Handle("/api/enterprise/admin/users/quota", auth.RequireAdminAuth(r.adminAPI.ValidateAdminToken)(http.HandlerFunc(r.adminAPI.HandleUpdateUserQuota)))
	r.mux.Handle("/api/enterprise/admin/users/impersonate", auth.RequireAdminAuth(r.adminAPI.ValidateAdminToken)(http.HandlerFunc(r.adminAPI.HandleImpersonateUser)))
	r.mux.Handle("/api/enterprise/admin/tenants", auth.RequireAdminAuth(r.adminAPI.ValidateAdminToken)(http.HandlerFunc(r.handleAdminTenants())))
	r.mux.Handle("/api/enterprise/admin/nodes", auth.RequireAdminAuth(r.adminAPI.ValidateAdminToken)(http.HandlerFunc(r.adminAPI.HandleListNodes)))
	r.mux.Handle("/api/enterprise/admin/stats", auth.RequireAdminAuth(r.adminAPI.ValidateAdminToken)(http.HandlerFunc(r.adminAPI.HandleGetSystemStats)))
	r.mux.Handle("/api/enterprise/admin/disk", auth.RequireAdminAuth(r.adminAPI.ValidateAdminToken)(http.HandlerFunc(r.adminAPI.HandleGetDiskStats)))

	// Admin archiving routes
	r.mux.Handle("/api/enterprise/admin/archive/activity", auth.RequireAdminAuth(r.adminAPI.ValidateAdminToken)(http.HandlerFunc(r.adminAPI.HandleGetTenantActivity)))
	r.mux.Handle("/api/enterprise/admin/archive/inactive", auth.RequireAdminAuth(r.adminAPI.ValidateAdminToken)(http.HandlerFunc(r.adminAPI.HandleListInactiveTenants)))
	r.mux.Handle("/api/enterprise/admin/archive/tenant", auth.RequireAdminAuth(r.adminAPI.ValidateAdminToken)(http.HandlerFunc(r.adminAPI.HandleArchiveTenant)))
	r.mux.Handle("/api/enterprise/admin/archive/restore", auth.RequireAdminAuth(r.adminAPI.ValidateAdminToken)(http.HandlerFunc(r.adminAPI.HandleRestoreTenant)))
	r.mux.Handle("/api/enterprise/admin/archive/stats", auth.RequireAdminAuth(r.adminAPI.ValidateAdminToken)(http.HandlerFunc(r.adminAPI.HandleGetArchiveStats)))

	// Health check endpoints
	r.mux.HandleFunc("/health/live", health.LivenessHandler())
	r.mux.HandleFunc("/health/ready", health.ReadinessHandler(r.cp.GetHealthChecker()))
	r.mux.HandleFunc("/api/enterprise/health", r.cp.GetHealthChecker().HTTPHandler())

	// Prometheus metrics endpoint
	r.mux.Handle("/metrics", promhttp.Handler())
}

// handleUserTenants handles tenant-related requests for users
func (r *Router) handleUserTenants() http.Handler {
	return auth.RequireUserAuth(r.jwtManager)(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.userAPI.HandleListTenants(w, req)
		case http.MethodPost:
			r.userAPI.HandleCreateTenant(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
}

// handleAdminUsers handles user-related requests for admins
func (r *Router) handleAdminUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			// Check if getting specific user
			if req.URL.Query().Get("userId") != "" {
				r.adminAPI.HandleGetUser(w, req)
			} else {
				r.adminAPI.HandleListUsers(w, req)
			}
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleAdminTenants handles tenant-related requests for admins
func (r *Router) handleAdminTenants() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			// Check if getting specific tenant
			if req.URL.Query().Get("tenantId") != "" {
				r.adminAPI.HandleGetTenant(w, req)
			} else {
				r.adminAPI.HandleListTenants(w, req)
			}
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
