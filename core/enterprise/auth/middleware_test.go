package auth

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
)

func TestRateLimiterAllowsRequestsUnderLimit(t *testing.T) {
	limiter := NewRateLimiter(5, time.Minute, time.Minute)

	for i := 0; i < 5; i++ {
		if err := limiter.Allow("192.168.1.1"); err != nil {
			t.Errorf("request %d should be allowed, got error: %v", i+1, err)
		}
	}
}

func TestRateLimiterBlocksExcessRequests(t *testing.T) {
	limiter := NewRateLimiter(3, time.Minute, time.Minute)

	// First 3 requests should pass
	for i := 0; i < 3; i++ {
		if err := limiter.Allow("192.168.1.1"); err != nil {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 4th request should be blocked
	if err := limiter.Allow("192.168.1.1"); err == nil {
		t.Error("4th request should be blocked")
	}
}

func TestRateLimiterTracksIPsSeparately(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute, time.Minute)

	// 2 requests from IP1
	limiter.Allow("192.168.1.1")
	limiter.Allow("192.168.1.1")

	// IP1 should be blocked
	if err := limiter.Allow("192.168.1.1"); err == nil {
		t.Error("IP1 should be blocked after 2 requests")
	}

	// IP2 should still be allowed
	if err := limiter.Allow("192.168.1.2"); err != nil {
		t.Error("IP2 should be allowed")
	}
}

func TestRateLimiterWindowReset(t *testing.T) {
	limiter := NewRateLimiter(2, 50*time.Millisecond, time.Minute)

	// Use up the limit (but don't exceed it)
	limiter.Allow("192.168.1.1")
	limiter.Allow("192.168.1.1")

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again after window reset (counter resets)
	if err := limiter.Allow("192.168.1.1"); err != nil {
		t.Errorf("should be allowed after window reset: %v", err)
	}

	// Can make another request in new window
	if err := limiter.Allow("192.168.1.1"); err != nil {
		t.Errorf("should be allowed in new window: %v", err)
	}
}

func TestRateLimiterBlockExpiry(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute, 50*time.Millisecond)

	// Exceed limit to trigger block
	limiter.Allow("192.168.1.1")
	limiter.Allow("192.168.1.1")
	limiter.Allow("192.168.1.1") // This triggers the block

	// Should be blocked
	if err := limiter.Allow("192.168.1.1"); err == nil {
		t.Error("should be blocked")
	}

	// Wait for block to expire
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	if err := limiter.Allow("192.168.1.1"); err != nil {
		t.Errorf("should be allowed after block expires: %v", err)
	}
}

func TestRateLimiterConcurrency(t *testing.T) {
	limiter := NewRateLimiter(100, time.Minute, time.Minute)

	var wg sync.WaitGroup
	var successCount int
	var mu sync.Mutex

	// 50 concurrent requests from same IP
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := limiter.Allow("192.168.1.1"); err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if successCount != 50 {
		t.Errorf("expected all 50 requests to succeed, got %d", successCount)
	}
}

func TestRateLimiterDefaultValues(t *testing.T) {
	// Test with zero values - should use defaults
	limiter := NewRateLimiter(0, 0, 0)

	// Default is 10 requests per minute
	for i := 0; i < 10; i++ {
		if err := limiter.Allow("192.168.1.1"); err != nil {
			t.Errorf("request %d should be allowed with defaults", i+1)
		}
	}

	// 11th should be blocked
	if err := limiter.Allow("192.168.1.1"); err == nil {
		t.Error("11th request should be blocked with default limit of 10")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := RateLimitMiddleware(limiter)(handler)

	// First 2 requests should pass
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("request %d should return 200, got %d", i+1, rr.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("3rd request should return 429, got %d", rr.Code)
	}

	// Check Retry-After header
	if rr.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on rate limited response")
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xForwardedFor string
		xRealIP    string
		expected   string
	}{
		{
			name:       "RemoteAddr only",
			remoteAddr: "192.168.1.1:12345",
			expected:   "192.168.1.1",
		},
		{
			name:          "X-Forwarded-For single",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "192.168.1.1",
			expected:      "192.168.1.1",
		},
		{
			name:          "X-Forwarded-For chain",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "192.168.1.1, 10.0.0.2, 10.0.0.3",
			expected:      "192.168.1.1",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			xRealIP:    "192.168.1.1",
			expected:   "192.168.1.1",
		},
		{
			name:          "X-Forwarded-For takes precedence over X-Real-IP",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "192.168.1.1",
			xRealIP:       "192.168.1.2",
			expected:      "192.168.1.1",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "192.168.1.1",
			expected:   "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			ip := getClientIP(req)
			if ip != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, ip)
			}
		})
	}
}

func TestRequireUserAuthMiddleware(t *testing.T) {
	jwtManager, err := NewJWTManager("test-secret-key-32-bytes-long!!")
	if err != nil {
		t.Fatalf("failed to create JWT manager: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetUserClaims(r.Context())
		if !ok {
			t.Error("expected claims in context")
			return
		}
		if claims.UserID != "user_123" {
			t.Errorf("expected user_123, got %s", claims.UserID)
		}
		w.WriteHeader(http.StatusOK)
	})

	wrapped := RequireUserAuth(jwtManager)(handler)

	// Test without token
	t.Run("no token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	// Test with invalid token format
	t.Run("invalid format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	// Test with valid token
	t.Run("valid token", func(t *testing.T) {
		user := &enterprise.ClusterUser{
			ID:       "user_123",
			Email:    "test@example.com",
			Verified: true,
		}
		token, err := jwtManager.GenerateUserToken(user, 24)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	// Test with unverified user token
	t.Run("unverified user", func(t *testing.T) {
		user := &enterprise.ClusterUser{
			ID:       "user_123",
			Email:    "test@example.com",
			Verified: false,
		}
		token, err := jwtManager.GenerateUserToken(user, 24)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403 for unverified user, got %d", rr.Code)
		}
	})
}

func TestRequireAdminAuthMiddleware(t *testing.T) {
	validToken := "admin_test_token_12345"

	validateFunc := func(token string) bool {
		return token == validToken
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := GetAdminToken(r.Context())
		if !ok {
			t.Error("expected admin token in context")
			return
		}
		if token != validToken {
			t.Errorf("expected %s, got %s", validToken, token)
		}
		w.WriteHeader(http.StatusOK)
	})

	wrapped := RequireAdminAuth(validateFunc)(handler)

	// Test without token
	t.Run("no token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	// Test with X-Admin-Token header
	t.Run("valid X-Admin-Token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Admin-Token", validToken)
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	// Test with Bearer token
	t.Run("valid Bearer token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	// Test with invalid token
	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Admin-Token", "invalid_token")
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})
}
