package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type contextKey string

const (
	UserClaimsKey  contextKey = "user_claims"
	AdminTokenKey  contextKey = "admin_token"
)

// RateLimiter implements IP-based rate limiting for auth endpoints
type RateLimiter struct {
	requests map[string]*requestTracker
	mu       sync.RWMutex

	// Configuration
	maxRequests int           // Max requests per window
	window      time.Duration // Time window
	blockTime   time.Duration // Block duration after exceeding limit
}

type requestTracker struct {
	count       int
	windowStart time.Time
	blockedAt   *time.Time
}

// NewRateLimiter creates a new rate limiter
// Default: 10 requests per minute, 5-minute block on exceed
func NewRateLimiter(maxRequests int, window, blockTime time.Duration) *RateLimiter {
	if maxRequests <= 0 {
		maxRequests = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	if blockTime <= 0 {
		blockTime = 5 * time.Minute
	}

	rl := &RateLimiter{
		requests:    make(map[string]*requestTracker),
		maxRequests: maxRequests,
		window:      window,
		blockTime:   blockTime,
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	tracker, exists := rl.requests[ip]

	if !exists {
		rl.requests[ip] = &requestTracker{
			count:       1,
			windowStart: now,
		}
		return nil
	}

	// Check if blocked
	if tracker.blockedAt != nil {
		if now.Sub(*tracker.blockedAt) < rl.blockTime {
			remaining := rl.blockTime - now.Sub(*tracker.blockedAt)
			return fmt.Errorf("too many requests, try again in %v", remaining.Round(time.Second))
		}
		// Block expired, reset tracker
		tracker.blockedAt = nil
		tracker.count = 1
		tracker.windowStart = now
		return nil
	}

	// Check if window expired
	if now.Sub(tracker.windowStart) > rl.window {
		tracker.count = 1
		tracker.windowStart = now
		return nil
	}

	// Increment and check limit
	tracker.count++
	if tracker.count > rl.maxRequests {
		tracker.blockedAt = &now
		return fmt.Errorf("too many requests, try again in %v", rl.blockTime)
	}

	return nil
}

// cleanupLoop periodically cleans up old entries
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes expired entries
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := rl.window + rl.blockTime

	for ip, tracker := range rl.requests {
		// Remove if window expired and not blocked
		if tracker.blockedAt == nil && now.Sub(tracker.windowStart) > cutoff {
			delete(rl.requests, ip)
			continue
		}
		// Remove if block expired
		if tracker.blockedAt != nil && now.Sub(*tracker.blockedAt) > rl.blockTime {
			delete(rl.requests, ip)
		}
	}
}

// RateLimitMiddleware returns middleware that rate-limits requests
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			if err := limiter.Allow(ip); err != nil {
				w.Header().Set("Retry-After", "300") // 5 minutes
				http.Error(w, err.Error(), http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxied requests)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the chain
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// RequireUserAuth is middleware that requires cluster user authentication
func RequireUserAuth(jwtManager *JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Expecting "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]

			// Validate token
			claims, err := jwtManager.ValidateUserToken(tokenString)
			if err != nil {
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Check if user is verified
			if !claims.Verified {
				http.Error(w, "Email not verified", http.StatusForbidden)
				return
			}

			// Add claims to context
			ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdminAuth is middleware that requires cluster admin authentication
func RequireAdminAuth(validateAdminToken func(string) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header or X-Admin-Token header
			token := r.Header.Get("X-Admin-Token")
			if token == "" {
				authHeader := r.Header.Get("Authorization")
				if authHeader != "" {
					parts := strings.Split(authHeader, " ")
					if len(parts) == 2 && parts[0] == "Bearer" {
						token = parts[1]
					}
				}
			}

			if token == "" {
				http.Error(w, "Admin token required", http.StatusUnauthorized)
				return
			}

			// Validate admin token
			if !validateAdminToken(token) {
				http.Error(w, "Invalid admin token", http.StatusUnauthorized)
				return
			}

			// Add token to context
			ctx := context.WithValue(r.Context(), AdminTokenKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserClaims retrieves user claims from context
func GetUserClaims(ctx context.Context) (*ClusterUserClaims, bool) {
	claims, ok := ctx.Value(UserClaimsKey).(*ClusterUserClaims)
	return claims, ok
}

// GetAdminToken retrieves admin token from context
func GetAdminToken(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(AdminTokenKey).(string)
	return token, ok
}
