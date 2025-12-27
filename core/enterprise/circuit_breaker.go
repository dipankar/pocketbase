package enterprise

import (
	"errors"
	"sync"
	"time"
)

// Circuit breaker states
const (
	CircuitClosed   = "closed"   // Normal operation, requests pass through
	CircuitOpen     = "open"     // Failing, requests are rejected
	CircuitHalfOpen = "half-open" // Testing if service recovered
)

var (
	// ErrCircuitOpen is returned when the circuit breaker is open
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// CircuitBreaker implements the circuit breaker pattern for external service calls
type CircuitBreaker struct {
	name string

	// Configuration
	maxFailures     int           // Failures before opening circuit
	resetTimeout    time.Duration // Time to wait before trying half-open
	halfOpenMaxReqs int           // Max requests in half-open state

	// State
	state           string
	failures        int
	successes       int
	halfOpenReqs    int
	lastStateChange time.Time

	mu sync.RWMutex
}

// CircuitBreakerConfig holds configuration for a circuit breaker
type CircuitBreakerConfig struct {
	Name            string        // Identifier for the circuit breaker
	MaxFailures     int           // Number of failures before opening (default: 5)
	ResetTimeout    time.Duration // Time before attempting half-open (default: 30s)
	HalfOpenMaxReqs int           // Max requests in half-open state (default: 3)
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.MaxFailures <= 0 {
		config.MaxFailures = 5
	}
	if config.ResetTimeout <= 0 {
		config.ResetTimeout = 30 * time.Second
	}
	if config.HalfOpenMaxReqs <= 0 {
		config.HalfOpenMaxReqs = 3
	}

	return &CircuitBreaker{
		name:            config.Name,
		maxFailures:     config.MaxFailures,
		resetTimeout:    config.ResetTimeout,
		halfOpenMaxReqs: config.HalfOpenMaxReqs,
		state:           CircuitClosed,
		lastStateChange: time.Now(),
	}
}

// Execute runs the given function through the circuit breaker
// Returns ErrCircuitOpen if the circuit is open
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.allowRequest() {
		return ErrCircuitOpen
	}

	err := fn()

	cb.recordResult(err == nil)
	return err
}

// allowRequest checks if a request should be allowed through
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true

	case CircuitOpen:
		// Check if reset timeout has elapsed
		if time.Since(cb.lastStateChange) >= cb.resetTimeout {
			cb.toHalfOpen()
			return true
		}
		return false

	case CircuitHalfOpen:
		// Allow limited requests in half-open state
		if cb.halfOpenReqs < cb.halfOpenMaxReqs {
			cb.halfOpenReqs++
			return true
		}
		return false
	}

	return false
}

// recordResult records the success or failure of a request
func (cb *CircuitBreaker) recordResult(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		if success {
			cb.failures = 0
		} else {
			cb.failures++
			if cb.failures >= cb.maxFailures {
				cb.toOpen()
			}
		}

	case CircuitHalfOpen:
		if success {
			cb.successes++
			// If we've had enough successes, close the circuit
			if cb.successes >= cb.halfOpenMaxReqs {
				cb.toClosed()
			}
		} else {
			// Any failure in half-open reopens the circuit
			cb.toOpen()
		}
	}
}

// State transitions
func (cb *CircuitBreaker) toOpen() {
	cb.state = CircuitOpen
	cb.lastStateChange = time.Now()
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenReqs = 0
}

func (cb *CircuitBreaker) toHalfOpen() {
	cb.state = CircuitHalfOpen
	cb.lastStateChange = time.Now()
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenReqs = 0
}

func (cb *CircuitBreaker) toClosed() {
	cb.state = CircuitClosed
	cb.lastStateChange = time.Now()
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenReqs = 0
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Name returns the name of the circuit breaker
func (cb *CircuitBreaker) Name() string {
	return cb.name
}

// Stats returns current circuit breaker statistics
func (cb *CircuitBreaker) Stats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"name":            cb.name,
		"state":           cb.state,
		"failures":        cb.failures,
		"successes":       cb.successes,
		"halfOpenReqs":    cb.halfOpenReqs,
		"lastStateChange": cb.lastStateChange,
	}
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.toClosed()
}

// CircuitBreakerRegistry manages multiple circuit breakers
type CircuitBreakerRegistry struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

// NewCircuitBreakerRegistry creates a new registry for circuit breakers
func NewCircuitBreakerRegistry() *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// Get retrieves or creates a circuit breaker by name
func (r *CircuitBreakerRegistry) Get(name string) *CircuitBreaker {
	r.mu.RLock()
	if cb, exists := r.breakers[name]; exists {
		r.mu.RUnlock()
		return cb
	}
	r.mu.RUnlock()

	// Create with default config
	return r.GetOrCreate(CircuitBreakerConfig{Name: name})
}

// GetOrCreate retrieves or creates a circuit breaker with the given config
func (r *CircuitBreakerRegistry) GetOrCreate(config CircuitBreakerConfig) *CircuitBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cb, exists := r.breakers[config.Name]; exists {
		return cb
	}

	cb := NewCircuitBreaker(config)
	r.breakers[config.Name] = cb
	return cb
}

// Stats returns statistics for all circuit breakers
func (r *CircuitBreakerRegistry) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]interface{})
	for name, cb := range r.breakers {
		stats[name] = cb.Stats()
	}
	return stats
}
