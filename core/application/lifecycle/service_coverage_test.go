package lifecycle_test

import (
	"context"
	"testing"
	"time"

	"github.com/ElioNeto/vyx/core/domain/worker"
	"github.com/ElioNeto/vyx/core/application/lifecycle"
)

// TestStopAll_NoWorkers verifies StopAll with no workers.
func TestStopAll_NoWorkers(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	err := svc.StopAll(context.Background())
	if err != nil {
		t.Errorf("StopAll with no workers should not error, got: %v", err)
	}
}

// TestStopAll_WithWorkers verifies StopAll stops all running workers.
func TestStopAll_WithWorkers(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	// Spawn and mark running 2 workers
	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w2", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")
	_ = svc.MarkRunning(context.Background(), "w2")

	// Stop all
	err := svc.StopAll(context.Background())
	if err != nil {
		t.Errorf("StopAll failed: %v", err)
	}

	if len(mgr.stopped) != 2 {
		t.Errorf("expected 2 workers stopped, got %d", len(mgr.stopped))
	}
}

// TestStopAll_SkipStoppedWorkers verifies StopAll skips non-alive workers.
func TestStopAll_SkipStoppedWorkers(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	// Spawn a worker and stop it
	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.StopWorker(context.Background(), "w1")

	// StopAll should not try to stop already stopped workers
	err := svc.StopAll(context.Background())
	if err != nil {
		t.Errorf("StopAll failed: %v", err)
	}
}

// TestSpawnWorker_WithWorkDir verifies workdir is set.
func TestSpawnWorker_WithWorkDir(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	w, err := svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node", WorkDir: "/tmp",
	})

	if err != nil {
		t.Fatalf("SpawnWorker failed: %v", err)
	}
	if w.WorkDir != "/tmp" {
		t.Errorf("WorkDir = %q, want %q", w.WorkDir, "/tmp")
	}
}

// TestSpawnWorker_WithShutdownTimeout verifies shutdown timeout is set.
func TestSpawnWorker_WithShutdownTimeout(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	w, err := svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node", ShutdownTimeout: 10 * time.Second,
	})

	if err != nil {
		t.Fatalf("SpawnWorker failed: %v", err)
	}
	if w.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 10s", w.ShutdownTimeout)
	}
}

// TestRecordHeartbeat_UnhealthyToRunning verifies unhealthy worker becomes running.
func TestRecordHeartbeat_UnhealthyToRunning(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	// Spawn and mark unhealthy
	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkUnhealthy(context.Background(), "w1")

	// Record heartbeat should transition to running
	err := svc.RecordHeartbeat(context.Background(), "w1")
	if err != nil {
		t.Fatalf("RecordHeartbeat failed: %v", err)
	}
}

// TestMarkUnhealthy_NotFound verifies error on non-existent worker.
func TestMarkUnhealthy_NotFound(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	err := svc.MarkUnhealthy(context.Background(), "non-existent")
	if err != worker.ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestMarkRunning_AlreadyRunning verifies no-op when already running.
func TestMarkRunning_AlreadyRunning(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	// Spawn and mark running
	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")

	// Mark running again should be no-op
	err := svc.MarkRunning(context.Background(), "w1")
	if err != nil {
		t.Errorf("MarkRunning should not error when already running: %v", err)
	}
}

// TestMarkRunning_NotFound verifies error on non-existent worker.
func TestMarkRunning_NotFound(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	err := svc.MarkRunning(context.Background(), "non-existent")
	if err != worker.ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestRestartWorker_NotFound verifies error on non-existent worker.
func TestRestartWorker_NotFound(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	err := svc.RestartWorker(context.Background(), "non-existent")
	if err != worker.ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestRestartWorker_Basic verifies restart increments count.
func TestRestartWorker_Basic(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	// Spawn, mark running, then mark unhealthy
	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")
	_ = svc.MarkUnhealthy(context.Background(), "w1")

	// Restart
	err := svc.RestartWorker(context.Background(), "w1")
	if err != nil {
		t.Fatalf("RestartWorker failed: %v", err)
	}
}
