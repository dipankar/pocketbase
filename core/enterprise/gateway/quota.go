package gateway

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
)

// QuotaEnforcer enforces tenant quotas and rate limits at the gateway
type QuotaEnforcer struct {
	cpClient enterprise.ControlPlaneClient

	// In-memory quota tracking (synced with control plane)
	quotas   map[string]*TenantQuotaState
	quotasMu sync.RWMutex

	// Rate limiting (token bucket per tenant)
	rateLimiters   map[string]*RateLimiter
	rateLimitersMu sync.RWMutex

	// Metrics
	rejectedRequests int64
	rejectedStorage  int64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	logger *log.Logger
}

// TenantQuotaState tracks current quota usage for a tenant
type TenantQuotaState struct {
	TenantID string

	// Quota limits (from control plane)
	StorageQuotaMB   int64
	APIRequestsQuota int64 // Per day

	// Current usage
	StorageUsedMB   int64
	RequestsToday   int64
	RequestsLast1h  int64
	LastRequestTime time.Time

	// Reset tracking
	DayStart time.Time

	// Sync
	LastSync time.Time
	Mu       sync.RWMutex
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	tokens         float64
	maxTokens      float64
	refillRate     float64 // tokens per second
	lastRefillTime time.Time
	mu             sync.Mutex
}

// NewQuotaEnforcer creates a new quota enforcer
func NewQuotaEnforcer(cpClient enterprise.ControlPlaneClient) *QuotaEnforcer {
	ctx, cancel := context.WithCancel(context.Background())

	return &QuotaEnforcer{
		cpClient:       cpClient,
		quotas:         make(map[string]*TenantQuotaState),
		rateLimiters:   make(map[string]*RateLimiter),
		ctx:            ctx,
		cancel:         cancel,
		logger:         log.Default(),
	}
}

// Start begins background quota syncing
func (qe *QuotaEnforcer) Start() {
	qe.logger.Printf("[QuotaEnforcer] Starting quota enforcement")

	qe.wg.Add(2)
	go qe.syncQuotasLoop()
	go qe.resetDailyCountersLoop()
}

// Stop stops the quota enforcer
func (qe *QuotaEnforcer) Stop() {
	qe.logger.Printf("[QuotaEnforcer] Stopping quota enforcer")
	qe.cancel()
	qe.wg.Wait()
}

// CheckQuota checks if a request is allowed based on quotas
func (qe *QuotaEnforcer) CheckQuota(tenantID string, requestSizeBytes int64) error {
	// Get or create quota state
	state := qe.getOrCreateQuotaState(tenantID)

	state.Mu.Lock()
	defer state.Mu.Unlock()

	// Check daily API request quota
	if state.APIRequestsQuota > 0 && state.RequestsToday >= state.APIRequestsQuota {
		qe.rejectedRequests++
		return enterprise.NewQuotaError("api_requests", state.RequestsToday, state.APIRequestsQuota)
	}

	// Check storage quota (if upload)
	if requestSizeBytes > 0 {
		requestSizeMB := requestSizeBytes / (1024 * 1024)
		if state.StorageQuotaMB > 0 && state.StorageUsedMB+requestSizeMB > state.StorageQuotaMB {
			qe.rejectedStorage++
			return enterprise.NewQuotaError("storage", state.StorageUsedMB+requestSizeMB, state.StorageQuotaMB)
		}
	}

	// Check rate limit (requests per second)
	if !qe.checkRateLimit(tenantID) {
		qe.rejectedRequests++
		return enterprise.NewQuotaError("rate_limit", state.RequestsLast1h, 3600)
	}

	// Update counters
	state.RequestsToday++
	state.RequestsLast1h++
	state.LastRequestTime = time.Now()

	return nil
}

// RecordRequest records a successful request (called after processing)
func (qe *QuotaEnforcer) RecordRequest(tenantID string, responseSizeBytes int64) {
	state := qe.getOrCreateQuotaState(tenantID)

	state.Mu.Lock()
	defer state.Mu.Unlock()

	// Storage usage updates are handled by control plane
	// We just track request counts here
}

// getOrCreateQuotaState retrieves or creates quota state for a tenant
func (qe *QuotaEnforcer) getOrCreateQuotaState(tenantID string) *TenantQuotaState {
	qe.quotasMu.RLock()
	state, exists := qe.quotas[tenantID]
	qe.quotasMu.RUnlock()

	if exists {
		return state
	}

	// Create new state with defaults
	qe.quotasMu.Lock()
	defer qe.quotasMu.Unlock()

	// Double-check after acquiring write lock
	if state, exists := qe.quotas[tenantID]; exists {
		return state
	}

	now := time.Now()
	state = &TenantQuotaState{
		TenantID:         tenantID,
		StorageQuotaMB:   1024,   // Default 1 GB
		APIRequestsQuota: 100000, // Default 100k/day
		DayStart:         time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
		LastSync:         time.Time{}, // Force immediate sync
	}

	qe.quotas[tenantID] = state

	// Trigger background sync for this tenant
	go qe.syncTenantQuota(tenantID)

	return state
}

// checkRateLimit checks if request is allowed based on rate limit
func (qe *QuotaEnforcer) checkRateLimit(tenantID string) bool {
	limiter := qe.getOrCreateRateLimiter(tenantID)

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(limiter.lastRefillTime).Seconds()
	limiter.tokens += elapsed * limiter.refillRate
	if limiter.tokens > limiter.maxTokens {
		limiter.tokens = limiter.maxTokens
	}
	limiter.lastRefillTime = now

	// Check if we have tokens
	if limiter.tokens >= 1.0 {
		limiter.tokens -= 1.0
		return true
	}

	return false
}

// getOrCreateRateLimiter retrieves or creates rate limiter for a tenant
func (qe *QuotaEnforcer) getOrCreateRateLimiter(tenantID string) *RateLimiter {
	qe.rateLimitersMu.RLock()
	limiter, exists := qe.rateLimiters[tenantID]
	qe.rateLimitersMu.RUnlock()

	if exists {
		return limiter
	}

	qe.rateLimitersMu.Lock()
	defer qe.rateLimitersMu.Unlock()

	// Double-check
	if limiter, exists := qe.rateLimiters[tenantID]; exists {
		return limiter
	}

	// Token bucket: 100 tokens, refill at 10/second
	// Allows bursts up to 100 req/sec, sustained 10 req/sec
	limiter = &RateLimiter{
		tokens:         100.0,
		maxTokens:      100.0,
		refillRate:     10.0, // 10 requests per second sustained
		lastRefillTime: time.Now(),
	}

	qe.rateLimiters[tenantID] = limiter
	return limiter
}

// syncQuotasLoop periodically syncs quotas from control plane
func (qe *QuotaEnforcer) syncQuotasLoop() {
	defer qe.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-qe.ctx.Done():
			return
		case <-ticker.C:
			qe.syncAllQuotas()
		}
	}
}

// syncAllQuotas syncs quotas for all tracked tenants
func (qe *QuotaEnforcer) syncAllQuotas() {
	qe.quotasMu.RLock()
	tenantIDs := make([]string, 0, len(qe.quotas))
	for tenantID := range qe.quotas {
		tenantIDs = append(tenantIDs, tenantID)
	}
	qe.quotasMu.RUnlock()

	for _, tenantID := range tenantIDs {
		qe.syncTenantQuota(tenantID)
	}
}

// syncTenantQuota syncs quota data for a single tenant from control plane
func (qe *QuotaEnforcer) syncTenantQuota(tenantID string) {
	ctx, cancel := context.WithTimeout(qe.ctx, 5*time.Second)
	defer cancel()

	// Get tenant metadata from control plane
	tenant, err := qe.cpClient.GetTenantMetadata(ctx, tenantID)
	if err != nil {
		qe.logger.Printf("[QuotaEnforcer] Failed to sync quota for tenant %s: %v", tenantID, err)
		return
	}

	state := qe.getOrCreateQuotaState(tenantID)

	state.Mu.Lock()
	defer state.Mu.Unlock()

	// Update quotas from control plane
	state.StorageQuotaMB = tenant.StorageQuotaMB
	state.APIRequestsQuota = tenant.APIRequestsQuota
	state.StorageUsedMB = tenant.StorageUsedMB
	state.LastSync = time.Now()
}

// resetDailyCountersLoop resets daily counters at midnight
func (qe *QuotaEnforcer) resetDailyCountersLoop() {
	defer qe.wg.Done()

	// Calculate time until next midnight
	now := time.Now()
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	duration := nextMidnight.Sub(now)

	timer := time.NewTimer(duration)
	defer timer.Stop()

	for {
		select {
		case <-qe.ctx.Done():
			return
		case <-timer.C:
			qe.resetDailyCounters()

			// Schedule next reset for tomorrow
			now := time.Now()
			nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			timer.Reset(nextMidnight.Sub(now))
		}
	}
}

// resetDailyCounters resets daily request counters for all tenants
func (qe *QuotaEnforcer) resetDailyCounters() {
	qe.logger.Printf("[QuotaEnforcer] Resetting daily request counters")

	qe.quotasMu.RLock()
	defer qe.quotasMu.RUnlock()

	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, state := range qe.quotas {
		state.Mu.Lock()
		state.RequestsToday = 0
		state.DayStart = dayStart
		state.Mu.Unlock()
	}

	qe.logger.Printf("[QuotaEnforcer] Daily counters reset for %d tenants", len(qe.quotas))
}

// GetStats returns quota enforcement statistics
func (qe *QuotaEnforcer) GetStats() map[string]interface{} {
	qe.quotasMu.RLock()
	trackedTenants := len(qe.quotas)
	qe.quotasMu.RUnlock()

	return map[string]interface{}{
		"trackedTenants":    trackedTenants,
		"rejectedRequests":  qe.rejectedRequests,
		"rejectedStorage":   qe.rejectedStorage,
	}
}

// QuotaMiddleware returns HTTP middleware for quota enforcement
func (qe *QuotaEnforcer) QuotaMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract tenant ID from request (should be set by previous middleware)
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			// Tenant not identified yet, let it pass to tenant resolution
			next.ServeHTTP(w, r)
			return
		}

		// Estimate request size (for uploads)
		requestSize := r.ContentLength
		if requestSize < 0 {
			requestSize = 0
		}

		// Check quota
		if err := qe.CheckQuota(tenantID, requestSize); err != nil {
			qe.logger.Printf("[QuotaEnforcer] Quota check failed for tenant %s: %v", tenantID, err)

			// Return 429 Too Many Requests or 507 Insufficient Storage
			statusCode := http.StatusTooManyRequests
			if quotaErr, ok := err.(*enterprise.QuotaError); ok {
				if quotaErr.Resource == "storage" {
					statusCode = http.StatusInsufficientStorage // 507
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "3600") // Retry after 1 hour
			w.WriteHeader(statusCode)
			fmt.Fprintf(w, `{"error": "%s"}`, err.Error())
			return
		}

		// Allow request
		next.ServeHTTP(w, r)

		// Record successful request
		qe.RecordRequest(tenantID, 0) // Response size tracking could be added
	})
}
