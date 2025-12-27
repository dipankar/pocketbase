package enterprise

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCircuitBreakerInitialState(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 3,
	})

	if cb.State() != CircuitClosed {
		t.Errorf("expected initial state to be closed, got %s", cb.State())
	}

	if cb.Name() != "test" {
		t.Errorf("expected name to be 'test', got %s", cb.Name())
	}
}

func TestCircuitBreakerOpensAfterMaxFailures(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:         "test",
		MaxFailures:  3,
		ResetTimeout: 100 * time.Millisecond,
	})

	testErr := errors.New("test error")

	// First 3 failures should keep circuit closed
	for i := 0; i < 3; i++ {
		err := cb.Execute(func() error {
			return testErr
		})
		if err != testErr {
			t.Errorf("expected test error, got %v", err)
		}
	}

	// Circuit should now be open
	if cb.State() != CircuitOpen {
		t.Errorf("expected circuit to be open after %d failures, got %s", 3, cb.State())
	}

	// Next request should be rejected
	err := cb.Execute(func() error {
		t.Error("function should not be called when circuit is open")
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreakerResetsOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 3,
	})

	testErr := errors.New("test error")

	// 2 failures
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	// 1 success should reset the failure count
	cb.Execute(func() error {
		return nil
	})

	// Another 2 failures should not open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	if cb.State() != CircuitClosed {
		t.Errorf("expected circuit to remain closed, got %s", cb.State())
	}
}

func TestCircuitBreakerHalfOpenState(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:            "test",
		MaxFailures:     2,
		ResetTimeout:    50 * time.Millisecond,
		HalfOpenMaxReqs: 2,
	})

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	if cb.State() != CircuitOpen {
		t.Errorf("expected circuit to be open, got %s", cb.State())
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Next request should transition to half-open
	callCount := 0
	cb.Execute(func() error {
		callCount++
		return nil
	})

	if callCount != 1 {
		t.Errorf("expected function to be called once, got %d", callCount)
	}

	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected circuit to be half-open, got %s", cb.State())
	}
}

func TestCircuitBreakerClosesAfterSuccessfulHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:            "test",
		MaxFailures:     2,
		ResetTimeout:    50 * time.Millisecond,
		HalfOpenMaxReqs: 2,
	})

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Successful requests in half-open state
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return nil
		})
	}

	if cb.State() != CircuitClosed {
		t.Errorf("expected circuit to be closed after successful half-open, got %s", cb.State())
	}
}

func TestCircuitBreakerReopensOnHalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:            "test",
		MaxFailures:     2,
		ResetTimeout:    50 * time.Millisecond,
		HalfOpenMaxReqs: 3,
	})

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// One success in half-open
	cb.Execute(func() error {
		return nil
	})

	// Then a failure
	cb.Execute(func() error {
		return testErr
	})

	if cb.State() != CircuitOpen {
		t.Errorf("expected circuit to reopen after half-open failure, got %s", cb.State())
	}
}

func TestCircuitBreakerConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 100, // High threshold to avoid opening
	})

	var wg sync.WaitGroup
	iterations := 1000

	// Concurrent successful requests
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.Execute(func() error {
				return nil
			})
		}()
	}

	wg.Wait()

	if cb.State() != CircuitClosed {
		t.Errorf("expected circuit to remain closed, got %s", cb.State())
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 2,
	})

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	if cb.State() != CircuitOpen {
		t.Errorf("expected circuit to be open, got %s", cb.State())
	}

	// Manual reset
	cb.Reset()

	if cb.State() != CircuitClosed {
		t.Errorf("expected circuit to be closed after reset, got %s", cb.State())
	}

	// Should accept requests again
	callCount := 0
	cb.Execute(func() error {
		callCount++
		return nil
	})

	if callCount != 1 {
		t.Errorf("expected function to be called after reset")
	}
}

func TestCircuitBreakerStats(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test-stats",
		MaxFailures: 5,
	})

	stats := cb.Stats()

	if stats["name"] != "test-stats" {
		t.Errorf("expected name in stats, got %v", stats["name"])
	}

	if stats["state"] != CircuitClosed {
		t.Errorf("expected closed state in stats, got %v", stats["state"])
	}
}

func TestCircuitBreakerRegistry(t *testing.T) {
	registry := NewCircuitBreakerRegistry()

	// Get creates a new breaker
	cb1 := registry.Get("service-a")
	if cb1 == nil {
		t.Error("expected circuit breaker to be created")
	}

	// Get same name returns same breaker
	cb2 := registry.Get("service-a")
	if cb1 != cb2 {
		t.Error("expected same circuit breaker instance")
	}

	// GetOrCreate with config
	cb3 := registry.GetOrCreate(CircuitBreakerConfig{
		Name:        "service-b",
		MaxFailures: 10,
	})
	if cb3 == nil {
		t.Error("expected circuit breaker to be created")
	}

	// Stats should include both breakers
	stats := registry.Stats()
	if len(stats) != 2 {
		t.Errorf("expected 2 breakers in stats, got %d", len(stats))
	}
}

func TestCircuitBreakerDefaultConfig(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name: "test-defaults",
		// All other values should use defaults
	})

	// Verify defaults are applied (5 failures, 30s timeout, 3 half-open reqs)
	testErr := errors.New("test error")

	// Should need 5 failures to open
	for i := 0; i < 4; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	if cb.State() != CircuitClosed {
		t.Errorf("expected circuit to still be closed after 4 failures")
	}

	cb.Execute(func() error {
		return testErr
	})

	if cb.State() != CircuitOpen {
		t.Errorf("expected circuit to be open after 5 failures")
	}
}
