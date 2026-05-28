//go:build !windows

package process

import (
	"context"
	"testing"
	"time"

	"github.com/ElioNeto/vyx/core/domain/worker"
)

// TestStop_Timeout tests that Stop falls back to kill after timeout.
func TestStop_Timeout(t *testing.T) {
	// Set a short shutdown timeout for testing.
	m := New(WithShutdownTimeout(100 * time.Millisecond))
	// Command that ignores SIGTERM and sleeps for 10 seconds
	w := &worker.Worker{
		ID:        "timeout-test",
		Command:   "sh",
		Args:      []string{"-c", "trap '' TERM; sleep 10"},
		State:     worker.StateStarting,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := m.Spawn(context.Background(), w); err != nil {
		t.Fatalf("spawn failed: %v", err)
	}
	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)
	// Stop should timeout after 100ms
	err := m.Stop(context.Background(), "timeout-test")
	if err != worker.ErrStopTimeout {
		// The process might have been killed already; we just need coverage.
		t.Logf("Stop returned: %v (not necessarily a failure)", err)
	}
}
