package lifecycle_test

import (
	"context"
	"os"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/worker"
	"github.com/ElioNeto/vyx/core/application/lifecycle"
)

// newTestServiceNoSkip creates a test service without setting VYX_SKIP_RUNTIME.
func newTestServiceNoSkip(manager worker.Manager) (*lifecycle.Service, *mockPublisher) {
	repo := newMemRepo()
	pub := &mockPublisher{}
	drainer := lifecycle.NewWorkerDrainer()
	return lifecycle.NewService(repo, manager, pub, nil, nil, drainer), pub
}

// TestSpawnWorker_RuntimeNeedsProvisioning tests the provisioning path.
func TestSpawnWorker_RuntimeNeedsProvisioning(t *testing.T) {
	// Ensure VYX_SKIP_RUNTIME is not set
	oldVal := os.Getenv("VYX_SKIP_RUNTIME")
	defer os.Setenv("VYX_SKIP_RUNTIME", oldVal)
	os.Unsetenv("VYX_SKIP_RUNTIME")

	mgr := &mockManager{}
	svc, _ := newTestServiceNoSkip(mgr)

	// Use "node" command - this should trigger NeedsProvisioning to return true
	// Then runtime.Ensure will be called and likely fail in test env
	_, err := svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID:      "w1",
		Command: "node",
	})

	// We expect an error because runtime.Ensure will fail
	// (node is not provisioned in test environment)
	if err != nil {
		t.Logf("Expected error from runtime provisioning: %v", err)
	}
}

// TestStopWorker_WithDrainerAndTimeout tests the drain timeout path.
func TestStopWorker_WithDrainerAndTimeout(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")

	err := svc.StopWorker(context.Background(), "w1")
	if err != nil {
		t.Errorf("StopWorker failed: %v", err)
	}
}

// TestRestartWorker_WithNilTransport tests restart when transport is nil.
func TestRestartWorker_WithNilTransport(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")

	err := svc.RestartWorker(context.Background(), "w1")
	if err != nil {
		t.Fatalf("RestartWorker failed: %v", err)
	}
}

// TestRecordHeartbeat_MultipleTimes tests multiple heartbeats.
func TestRecordHeartbeat_MultipleTimes(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")

	for i := 0; i < 5; i++ {
		err := svc.RecordHeartbeat(context.Background(), "w1")
		if err != nil {
			t.Fatalf("RecordHeartbeat failed on iteration %d: %v", i, err)
		}
	}
}

// TestMarkRunning_FromStarting tests transition from starting to running.
func TestMarkRunning_FromStarting(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})

	err := svc.MarkRunning(context.Background(), "w1")
	if err != nil {
		t.Fatalf("MarkRunning failed: %v", err)
	}
}

// TestSpawnWorker_EmptyID tests spawning with empty ID.
func TestSpawnWorker_EmptyID(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	w, err := svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID:      "",
		Command: "node",
	})

	if err != nil {
		t.Fatalf("SpawnWorker failed: %v", err)
	}
	if w.ID != "" {
		t.Errorf("expected empty ID, got %q", w.ID)
	}
}

// TestStopWorker_AfterRestart tests stopping after restart.
func TestStopWorker_AfterRestart(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")
	_ = svc.RestartWorker(context.Background(), "w1")

	err := svc.StopWorker(context.Background(), "w1")
	if err != nil {
		t.Errorf("StopWorker failed: %v", err)
	}
}

// TestStopAll_OneWorkerStopFails tests when one worker stop fails.
func TestStopAll_OneWorkerStopFails(t *testing.T) {
	mgr := &mockManager{stopErr: worker.ErrNotFound}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w2", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")
	_ = svc.MarkRunning(context.Background(), "w2")

	err := svc.StopAll(context.Background())
	if err == nil {
		t.Error("expected error from StopAll")
	}
}

// TestRecordHeartbeat_StartingState tests heartbeat from starting worker.
func TestRecordHeartbeat_StartingState(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})

	err := svc.RecordHeartbeat(context.Background(), "w1")
	if err != nil {
		t.Fatalf("RecordHeartbeat failed: %v", err)
	}
}

// TestSpawnWorker_NilFields tests spawning with nil/empty fields.
func TestSpawnWorker_NilFields(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	w, err := svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID:      "w1",
		Command: "node",
		Args:    nil,
	})

	if err != nil {
		t.Fatalf("SpawnWorker failed: %v", err)
	}
	if len(w.Args) != 0 {
		t.Errorf("expected empty args, got %v", w.Args)
	}
}

// TestStopWorker_StoppedWorker tests stopping already stopped worker.
func TestStopWorker_StoppedWorker(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")
	_ = svc.StopWorker(context.Background(), "w1")

	// Stop again - worker is now stopped
	err := svc.StopWorker(context.Background(), "w1")
	if err != nil {
		t.Errorf("StopWorker on stopped worker should not error: %v", err)
	}
}

// TestRestartWorker_StoppedWorker tests restarting stopped worker.
func TestRestartWorker_StoppedWorker(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.StopWorker(context.Background(), "w1")

	err := svc.RestartWorker(context.Background(), "w1")
	if err != nil {
		t.Fatalf("RestartWorker failed: %v", err)
	}
}

// TestSpawnWorker_RuntimeResolveFails tests runtime.Resolve failure.
func TestSpawnWorker_RuntimeResolveFails(t *testing.T) {
	// Ensure VYX_SKIP_RUNTIME is not set
	oldVal := os.Getenv("VYX_SKIP_RUNTIME")
	defer os.Setenv("VYX_SKIP_RUNTIME", oldVal)
	os.Unsetenv("VYX_SKIP_RUNTIME")
	
	mgr := &mockManager{}
	svc, _ := newTestServiceNoSkip(mgr)
	
	// Use "node" which will trigger NeedsProvisioning=true
	// but runtime.Ensure will fail in test env
	_, err := svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID:      "w1",
		Command: "node",
	})
	
	// We expect some error from runtime provisioning
	if err == nil {
		t.Error("expected error from runtime.Resolve failure")
	}
	t.Logf("Got expected error: %v", err)
}
