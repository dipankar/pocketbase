package tenant_node

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
)

// HTTPServer handles incoming HTTP requests and routes them to tenant instances
type HTTPServer struct {
	manager *Manager
	server  *http.Server

	// Metrics
	totalRequests   int64
	failedRequests  int64
	tenantLoadTime  map[string]time.Duration
	tenantLoadMu    sync.RWMutex

	logger *log.Logger
}

// NewHTTPServer creates a new HTTP server for the tenant node
func NewHTTPServer(manager *Manager) *HTTPServer {
	return &HTTPServer{
		manager:        manager,
		tenantLoadTime: make(map[string]time.Duration),
		logger:         log.Default(),
	}
}

// Start starts the HTTP server
func (s *HTTPServer) Start(addr string) error {
	s.logger.Printf("[TenantNode HTTP] Starting HTTP server on %s", addr)

	mux := http.NewServeMux()

	// Main handler for all tenant requests
	mux.HandleFunc("/", s.handleTenantRequest)

	// Health check endpoint
	mux.HandleFunc("/_health", s.handleHealth)

	// Metrics endpoint
	mux.HandleFunc("/_metrics", s.handleMetrics)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s.server.ListenAndServe()
}

// Stop gracefully stops the HTTP server
func (s *HTTPServer) Stop() error {
	if s.server == nil {
		return nil
	}

	s.logger.Printf("[TenantNode HTTP] Stopping HTTP server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}

// handleTenantRequest routes requests to the appropriate tenant instance
func (s *HTTPServer) handleTenantRequest(w http.ResponseWriter, r *http.Request) {
	s.totalRequests++

	// Extract tenant ID from header (set by gateway)
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		s.logger.Printf("[TenantNode HTTP] Request missing X-Tenant-ID header")
		s.failedRequests++
		http.Error(w, "Missing tenant identifier", http.StatusBadRequest)
		return
	}

	// Get or load tenant instance
	startTime := time.Now()
	instance, err := s.manager.GetOrLoadTenant(tenantID)
	if err != nil {
		s.logger.Printf("[TenantNode HTTP] Failed to load tenant %s: %v", tenantID, err)
		s.failedRequests++

		// Return appropriate error based on the type
		if err == enterprise.ErrTenantNotFound {
			http.Error(w, "Tenant not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to load tenant", http.StatusServiceUnavailable)
		}
		return
	}
	loadDuration := time.Since(startTime)

	// Check API quota before processing request
	if quotaEnforcer := s.manager.GetQuotaEnforcer(); quotaEnforcer != nil {
		if err := quotaEnforcer.CheckAPIQuota(tenantID, instance.Tenant); err != nil {
			s.logger.Printf("[TenantNode HTTP] Tenant %s API quota exceeded", tenantID)
			s.failedRequests++
			http.Error(w, "API quota exceeded. Please upgrade your plan or wait for quota reset.", http.StatusTooManyRequests)
			return
		}

		// Check storage quota for write requests
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if err := quotaEnforcer.CheckStorageQuota(tenantID, instance.Tenant); err != nil {
				s.logger.Printf("[TenantNode HTTP] Tenant %s storage quota exceeded", tenantID)
				s.failedRequests++
				http.Error(w, "Storage quota exceeded. Please upgrade your plan.", http.StatusInsufficientStorage)
				return
			}
		}

		// Record the API request
		quotaEnforcer.RecordAPIRequest(tenantID)
	}

	// Track load time if it was actually loaded (not cached)
	if loadDuration > 100*time.Millisecond {
		s.tenantLoadMu.Lock()
		s.tenantLoadTime[tenantID] = loadDuration
		s.tenantLoadMu.Unlock()
		s.logger.Printf("[TenantNode HTTP] Loaded tenant %s in %v", tenantID, loadDuration)
	}

	// Update tenant last access time
	instance.LastAccessed = time.Now()
	instance.RequestCount++

	// Get the PocketBase app HTTP handler
	if instance.HTTPHandler == nil {
		s.logger.Printf("[TenantNode HTTP] Tenant %s has no HTTP handler", tenantID)
		s.failedRequests++
		http.Error(w, "Tenant HTTP handler not initialized", http.StatusServiceUnavailable)
		return
	}

	// Set tenant context for the request
	w.Header().Set("X-Tenant-ID", tenantID)

	// Log the request
	s.logger.Printf("[TenantNode HTTP] Serving request for tenant %s: %s %s", tenantID, r.Method, r.URL.Path)

	// Record request for peak tracking
	if metricsCollector := s.manager.metricsCollector; metricsCollector != nil {
		metricsCollector.RecordRequest(tenantID)
	}

	// Wrap response writer to capture status code and measure response time
	requestStart := time.Now()
	wrapper := &responseWriterWrapper{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // Default to 200
	}

	// Proxy the request to the tenant's PocketBase app HTTP handler
	instance.HTTPHandler.ServeHTTP(wrapper, r)

	// Calculate response time
	responseTime := time.Since(requestStart).Milliseconds()

	// Record metrics
	if metricsCollector := s.manager.metricsCollector; metricsCollector != nil {
		metricsCollector.RecordResponseTime(tenantID, float64(responseTime))

		// Record success or error based on status code
		if wrapper.statusCode >= 500 {
			metricsCollector.RecordError(tenantID)
		} else {
			metricsCollector.RecordSuccess(tenantID)
		}
	}
}

// responseWriterWrapper wraps http.ResponseWriter to capture status code
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	if !w.written {
		w.statusCode = statusCode
		w.written = true
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// handleHealth returns health status
func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	stats := s.manager.GetStats()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{
		"status": "healthy",
		"loadedTenants": %d,
		"totalRequests": %d,
		"failedRequests": %d,
		"nodeCapacity": %d,
		"memoryUsedMB": %d
	}`, stats.LoadedTenants, s.totalRequests, s.failedRequests, stats.Capacity, stats.MemoryUsedMB)
}

// handleMetrics returns detailed metrics
func (s *HTTPServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	stats := s.manager.GetStats()

	s.tenantLoadMu.RLock()
	loadTimeCount := len(s.tenantLoadTime)
	s.tenantLoadMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{
		"totalRequests": %d,
		"failedRequests": %d,
		"loadedTenants": %d,
		"capacity": %d,
		"memoryUsedMB": %d,
		"cpuPercent": %d,
		"tenantsLoadedCount": %d,
		"cacheHitRate": %.2f
	}`, s.totalRequests, s.failedRequests, stats.LoadedTenants, stats.Capacity,
		stats.MemoryUsedMB, stats.CPUPercent, loadTimeCount,
		float64(s.totalRequests-s.failedRequests)/float64(s.totalRequests)*100)
}
