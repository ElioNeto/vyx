package lifecycle_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ElioNeto/vyx/core/application/lifecycle"
)

func TestDrain_NormalCompletion(t *testing.T) {
	drainer := lifecycle.NewWorkerDrainer()
	ctx := context.Background()
	workerID := "test-worker"

	// Simulate two in-flight requests
	drainer.Acquire(workerID)
	drainer.Acquire(workerID)

	// Drain should wait for both to be released
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := drainer.Drain(ctx, workerID, 100*time.Millisecond); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}()

	// Release both requests
	drainer.Release(workerID)
	drainer.Release(workerID)

	select {
	case <-done:
		// success
	case <-time.After(200 * time.Millisecond):
		t.Errorf("drain did not complete in time")
	}
}

func TestDrain_TimeoutExpiry(t *testing.T) {
	drainer := lifecycle.NewWorkerDrainer()
	ctx := context.Background()
	workerID := "test-worker"

	// Simulate one in-flight request that we never release
	drainer.Acquire(workerID)

	// Drain should timeout
	if err := drainer.Drain(ctx, workerID, 10*time.Millisecond); err == nil {
		t.Errorf("expected timeout error, got nil")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestDrain_ZeroInflight(t *testing.T) {
	drainer := lifecycle.NewWorkerDrainer()
	ctx := context.Background()
	workerID := "test-worker"

	// No in-flight requests
	if err := drainer.Drain(ctx, workerID, 10*time.Millisecond); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestIsDraining_BeforeMark(t *testing.T) {
	drainer := lifecycle.NewWorkerDrainer()
	workerID := "test-worker"

	if drainer.IsDraining(workerID) {
		t.Errorf("expected IsDraining to be false before MarkDraining")
	}
}

func TestIsDraining_AfterMark(t *testing.T) {
	drainer := lifecycle.NewWorkerDrainer()
	workerID := "test-worker"

	drainer.MarkDraining(workerID)
	if !drainer.IsDraining(workerID) {
		t.Errorf("expected IsDraining to be true after MarkDraining")
	}
}

func TestCleanup_RemovesDrainingState(t *testing.T) {
	drainer := lifecycle.NewWorkerDrainer()
	workerID := "test-worker"

	drainer.MarkDraining(workerID)
	drainer.Acquire(workerID)
	drainer.Release(workerID) // inflight goes to zero

	drainer.Cleanup(workerID)

	if drainer.IsDraining(workerID) {
		t.Errorf("expected IsDraining to be false after Cleanup")
	}
}

// TestRelease_NoAcquire verifies Release on non-acquired worker does not panic.
func TestRelease_NoAcquire(t *testing.T) {
	drainer := lifecycle.NewWorkerDrainer()
	
	// Release without Acquire - should not panic
	done := make(chan struct{})
	go func() {
		defer close(done)
		drainer.Release("nonexistent-worker")
	}()
	
	select {
	case <-done:
		// OK
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Release blocked or panicked")
	}
}
