package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status of a component
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// Check represents a single health check
type Check struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Error  string `json:"error,omitempty"`
	Took   int64  `json:"took_ms"`
}

// Response represents the overall health check response
type Response struct {
	Status    Status           `json:"status"`
	Timestamp time.Time        `json:"timestamp"`
	Version   string           `json:"version,omitempty"`
	Checks    []Check          `json:"checks"`
	Metadata  map[string]any   `json:"metadata,omitempty"`
}

// CheckFunc is a function that performs a health check
type CheckFunc func(ctx context.Context) error

// Checker manages health checks for a component
type Checker struct {
	checks   map[string]CheckFunc
	mu       sync.RWMutex
	version  string
	metadata map[string]any
}

// NewChecker creates a new health checker
func NewChecker(version string) *Checker {
	return &Checker{
		checks:   make(map[string]CheckFunc),
		version:  version,
		metadata: make(map[string]any),
	}
}

// Register registers a new health check
func (c *Checker) Register(name string, check CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
}

// SetMetadata sets metadata to include in health responses
func (c *Checker) SetMetadata(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// Check performs all registered health checks
func (c *Checker) Check(ctx context.Context) *Response {
	c.mu.RLock()
	checks := make(map[string]CheckFunc, len(c.checks))
	for k, v := range c.checks {
		checks[k] = v
	}
	metadata := make(map[string]any, len(c.metadata))
	for k, v := range c.metadata {
		metadata[k] = v
	}
	version := c.version
	c.mu.RUnlock()

	// Run all checks with timeout
	checkResults := make([]Check, 0, len(checks))
	overallStatus := StatusHealthy

	for name, checkFunc := range checks {
		start := time.Now()

		// Create timeout context for individual check
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := checkFunc(checkCtx)
		cancel()

		took := time.Since(start).Milliseconds()

		result := Check{
			Name: name,
			Took: took,
		}

		if err != nil {
			result.Status = StatusUnhealthy
			result.Error = err.Error()
			overallStatus = StatusUnhealthy
		} else {
			result.Status = StatusHealthy
		}

		checkResults = append(checkResults, result)
	}

	return &Response{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Version:   version,
		Checks:    checkResults,
		Metadata:  metadata,
	}
}

// HTTPHandler returns an HTTP handler for health checks
func (c *Checker) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		response := c.Check(ctx)

		// Set appropriate status code
		statusCode := http.StatusOK
		if response.Status == StatusUnhealthy {
			statusCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}
}

// LivenessHandler returns a simple liveness check (always returns 200 if process is running)
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "alive",
		})
	}
}

// ReadinessHandler returns a readiness check using the provided checker
func ReadinessHandler(checker *Checker) http.HandlerFunc {
	return checker.HTTPHandler()
}
