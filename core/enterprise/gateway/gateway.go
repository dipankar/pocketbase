package gateway

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/pocketbase/pocketbase/core/enterprise"
	"github.com/pocketbase/pocketbase/core/enterprise/health"
)

// Gateway handles incoming requests and routes them to the appropriate tenant nodes
type Gateway struct {
	config   *enterprise.ClusterConfig
	cpClient enterprise.ControlPlaneClient

	// Proxy cache: domain -> node address
	proxyCache   map[string]*httputil.ReverseProxy
	proxyCacheMu sync.RWMutex

	// Node cache: tenantID -> node address
	nodeCache   map[string]string
	nodeCacheMu sync.RWMutex

	// Quota enforcement
	quotaEnforcer *QuotaEnforcer

	// Health and monitoring
	healthChecker *health.Checker

	ctx    context.Context
	cancel context.CancelFunc

	logger *log.Logger
}

// NewGateway creates a new gateway instance
func NewGateway(config *enterprise.ClusterConfig, cpClient enterprise.ControlPlaneClient) (*Gateway, error) {
	if config.Mode != enterprise.ModeGateway && config.Mode != enterprise.ModeAllInOne {
		return nil, fmt.Errorf("invalid mode for gateway: %s", config.Mode)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize health checker
	healthChecker := health.NewChecker("gateway")
	healthChecker.SetMetadata("mode", config.Mode)

	// Initialize quota enforcer
	quotaEnforcer := NewQuotaEnforcer(cpClient)

	return &Gateway{
		config:        config,
		cpClient:      cpClient,
		proxyCache:    make(map[string]*httputil.ReverseProxy),
		nodeCache:     make(map[string]string),
		quotaEnforcer: quotaEnforcer,
		healthChecker: healthChecker,
		ctx:           ctx,
		cancel:        cancel,
		logger:        log.Default(),
	}, nil
}

// Start starts the gateway HTTP server
func (g *Gateway) Start(addr string) error {
	g.logger.Printf("[Gateway] Starting gateway on %s", addr)

	// Start quota enforcer
	g.quotaEnforcer.Start()

	// Register health checks
	g.healthChecker.Register("control_plane", func(ctx context.Context) error {
		if g.cpClient == nil {
			return fmt.Errorf("control plane client not initialized")
		}
		// Test connectivity
		_, err := g.cpClient.GetTenantByDomain(ctx, "health-check-test.example.com")
		if err != nil && err.Error() != "tenant not found" {
			return fmt.Errorf("control plane unreachable: %w", err)
		}
		return nil
	})

	g.healthChecker.Register("proxy_cache", func(ctx context.Context) error {
		g.proxyCacheMu.RLock()
		defer g.proxyCacheMu.RUnlock()

		cacheSize := len(g.proxyCache)
		g.healthChecker.SetMetadata("proxyCacheSize", cacheSize)
		return nil
	})

	g.healthChecker.Register("quota_enforcer", func(ctx context.Context) error {
		if g.quotaEnforcer == nil {
			return fmt.Errorf("quota enforcer not initialized")
		}
		stats := g.quotaEnforcer.GetStats()
		g.healthChecker.SetMetadata("quotaStats", stats)
		return nil
	})

	// Wrap handleRequest with quota middleware
	quotaHandler := g.quotaEnforcer.QuotaMiddleware(http.HandlerFunc(g.handleRequest))

	http.Handle("/", quotaHandler)
	http.HandleFunc("/health/live", health.LivenessHandler())
	http.HandleFunc("/health/ready", health.ReadinessHandler(g.healthChecker))
	http.HandleFunc("/_health", g.healthChecker.HTTPHandler())

	return http.ListenAndServe(addr, nil)
}

// Stop gracefully stops the gateway
func (g *Gateway) Stop() error {
	g.logger.Printf("[Gateway] Stopping gateway...")

	// Stop quota enforcer
	if g.quotaEnforcer != nil {
		g.quotaEnforcer.Stop()
	}

	g.cancel()
	return nil
}

// handleRequest handles incoming HTTP requests and routes them
func (g *Gateway) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Extract tenant ID from domain
	host := r.Host
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	tenantID := enterprise.ExtractTenantIDFromDomain(host)
	if tenantID == "" {
		http.Error(w, "Invalid domain", http.StatusBadRequest)
		return
	}

	// Get tenant metadata from control plane
	tenant, err := g.cpClient.GetTenantByDomain(r.Context(), host)
	if err != nil {
		if err == enterprise.ErrTenantNotFound {
			http.Error(w, "Tenant not found", http.StatusNotFound)
		} else {
			g.logger.Printf("[Gateway] Failed to get tenant: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Check if tenant has assigned node
	nodeAddr := g.getNodeAddress(tenant.ID)
	if nodeAddr == "" {
		// No assignment yet, request placement decision
		_, err := g.cpClient.GetPlacementDecision(r.Context(), tenant.ID)
		if err != nil {
			g.logger.Printf("[Gateway] Failed to get placement decision: %v", err)
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}

		// Get node info to find address
		// For now, we'll use a default mapping
		// TODO: Store node addresses in control plane
		nodeAddr = fmt.Sprintf("http://localhost:8091") // Default tenant node address
		g.cacheNodeAddress(tenant.ID, nodeAddr)
	}

	// Get or create reverse proxy for this node
	proxy := g.getOrCreateProxy(nodeAddr)

	// Set tenant context in request
	r.Header.Set("X-Tenant-ID", tenant.ID)
	r.Header.Set("X-Tenant-Domain", tenant.Domain)

	// Proxy the request
	proxy.ServeHTTP(w, r)
}

// getOrCreateProxy gets or creates a reverse proxy for a node address
func (g *Gateway) getOrCreateProxy(nodeAddr string) *httputil.ReverseProxy {
	g.proxyCacheMu.RLock()
	proxy, exists := g.proxyCache[nodeAddr]
	g.proxyCacheMu.RUnlock()

	if exists {
		return proxy
	}

	// Create new proxy
	g.proxyCacheMu.Lock()
	defer g.proxyCacheMu.Unlock()

	// Double-check after acquiring write lock
	if proxy, exists := g.proxyCache[nodeAddr]; exists {
		return proxy
	}

	target, _ := url.Parse(nodeAddr)
	proxy = httputil.NewSingleHostReverseProxy(target)

	// Customize proxy behavior
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		g.logger.Printf("[Gateway] Proxy error: %v", err)

		// Invalidate cache on error
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID != "" {
			g.invalidateNodeCache(tenantID)
		}

		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	}

	g.proxyCache[nodeAddr] = proxy
	return proxy
}

// getNodeAddress retrieves the cached node address for a tenant
func (g *Gateway) getNodeAddress(tenantID string) string {
	g.nodeCacheMu.RLock()
	defer g.nodeCacheMu.RUnlock()

	return g.nodeCache[tenantID]
}

// cacheNodeAddress caches the node address for a tenant
func (g *Gateway) cacheNodeAddress(tenantID, nodeAddr string) {
	g.nodeCacheMu.Lock()
	defer g.nodeCacheMu.Unlock()

	g.nodeCache[tenantID] = nodeAddr
}

// invalidateNodeCache removes a tenant from the node cache
func (g *Gateway) invalidateNodeCache(tenantID string) {
	g.nodeCacheMu.Lock()
	defer g.nodeCacheMu.Unlock()

	delete(g.nodeCache, tenantID)
}
