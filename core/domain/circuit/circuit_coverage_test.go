package circuit

import (
	"sync"
	"testing"
	"time"
)

// TestBreaker_RecordSuccessInOpenState verifies no state change
func TestBreaker_RecordSuccessInOpenState(t *testing.T) {
	b := New(Config{Failures: 2, Cooldown: 10 * time.Millisecond}, nil)
	
	// Open the breaker
	b.RecordFailure()
	b.RecordFailure()
	if b.State() != StateOpen {
		t.Fatal("expected open state")
	}
	
	// Record success in open state (should do nothing)
	b.RecordSuccess()
	
	if b.State() != StateOpen {
		t.Error("expected open state after success in open")
	}
}

// TestBreaker_AllowInHalfOpenRespectsMaxProbes tests the half-open probe limit
func TestBreaker_AllowInHalfOpenRespectsMaxProbes(t *testing.T) {
	b := New(Config{Failures: 2, Cooldown:10 * time.Millisecond, HalfOpenMax:2}, nil)
	
	// Open the breaker
	b.RecordFailure()
	b.RecordFailure()
	
	// Wait for cooldown
	time.Sleep(15 * time.Millisecond)
	
	// First Allow() transitions from Open to HalfOpen (doesn't count as probe)
	b.Allow()
	
	// Probe 1 (allowed)
	if !b.Allow() {
		t.Error("expected probe 1 allowed")
	}
	
	// Probe 2 (allowed)
	if !b.Allow() {
		t.Error("expected probe 2 allowed")
	}
	
	// Probe 3 (denied, exceeds HalfOpenMax=2)
	if b.Allow() {
		t.Error("expected probe 3 denied")
	}
}

// TestBreaker_ConcurrentAccess tests race conditions with -race
func TestBreaker_ConcurrentAccess(t *testing.T) {
	b := New(Config{Failures: 3, Cooldown: 100 * time.Millisecond}, nil)
	
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.RecordFailure()
			b.RecordSuccess()
			b.Allow()
			b.State()
		}()
	}
	wg.Wait()
}

// TestRegistry_Count tests the Count method
func TestRegistry_Count(t *testing.T) {
	reg := NewRegistry(Config{Failures: 3}, nil)
	
	if reg.Count() != 0 {
		t.Errorf("expected 0 breakers, got %d", reg.Count())
	}
	
	reg.Get("route1")
	if reg.Count() != 1 {
		t.Errorf("expected 1 breaker, got %d", reg.Count())
	}
	
	reg.Get("route2")
	if reg.Count() != 2 {
		t.Errorf("expected 2 breakers, got %d", reg.Count())
	}
	
	reg.Remove("route1")
	if reg.Count() != 1 {
		t.Errorf("expected 1 breaker after remove, got %d", reg.Count())
	}
}

// TestBreaker_ZeroConfigUsesDefaults tests that defaults are applied
func TestBreaker_ZeroConfigUsesDefaults(t *testing.T) {
	b := New(Config{}, nil)
	
	// Test default failures (5)
	for i := 0; i < 5; i++ {
		b.RecordFailure()
	}
	if b.State() != StateOpen {
		t.Errorf("expected open after 5 failures (default), got %v", b.State())
	}
	
	// Test default cooldown (30s) - can't easily test, but verify config
	if b.config.Cooldown != 30*time.Second {
		t.Errorf("expected default cooldown 30s, got %v", b.config.Cooldown)
	}
}

// TestBreaker_EmitNilCallback tests that nil callback doesn't panic
func TestBreaker_EmitNilCallback(t *testing.T) {
	b := New(Config{Failures: 1, Cooldown: 10 * time.Millisecond}, nil)
	
	// Should not panic
	b.RecordFailure()
	if b.State() != StateOpen {
		t.Error("expected open state")
	}
}
