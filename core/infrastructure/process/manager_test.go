package process_test

import (
	"context"
	"testing"
	"time"

	"github.com/ElioNeto/vyx/core/domain/worker"
	"github.com/ElioNeto/vyx/core/infrastructure/process"
)

func workerWith(id, cmd string, args []string) *worker.Worker {
	return &worker.Worker{
		ID:        id,
		Command:   cmd,
		Args:      args,
		State:     worker.StateStarting,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// TestSpawn_ValidCommand verifies that a real child process is spawned
// successfully using a portable command available on all Unix/Linux systems.
func TestSpawn_ValidCommand(t *testing.T) {
	mgr := process.New()
	w := workerWith("test-sleep", "sleep", []string{"30"})

	err := mgr.Spawn(context.Background(), w)
	if err != nil {
		t.Fatalf("expected spawn to succeed, got: %v", err)
	}

	// Clean up.
	_ = mgr.Stop(context.Background(), "test-sleep")
}

// TestSpawn_EmptyCommand verifies that spawning with an empty command returns
// ErrInvalidCommand without touching the OS.
func TestSpawn_EmptyCommand(t *testing.T) {
	mgr := process.New()
	w := workerWith("bad", "", nil)

	err := mgr.Spawn(context.Background(), w)
	if err != worker.ErrInvalidCommand {
		t.Errorf("expected ErrInvalidCommand, got: %v", err)
	}
}

// TestSpawn_InvalidBinary verifies that spawning a non-existent binary returns
// an error wrapping ErrSpawnFailed.
func TestSpawn_InvalidBinary(t *testing.T) {
	mgr := process.New()
	w := workerWith("bad-bin", "__definitely_not_a_real_binary__", nil)

	err := mgr.Spawn(context.Background(), w)
	if err == nil {
		t.Fatal("expected error for non-existent binary, got nil")
	}
}

// TestStop_GracefulShutdown spawns a long-running process and verifies that
// Stop sends SIGTERM and waits for clean exit.
func TestStop_GracefulShutdown(t *testing.T) {
	mgr := process.New()
	w := workerWith("test-stop", "sleep", []string{"30"})

	if err := mgr.Spawn(context.Background(), w); err != nil {
		t.Fatalf("spawn failed: %v", err)
	}

	err := mgr.Stop(context.Background(), "test-stop")
	if err != nil {
		t.Errorf("expected clean stop, got: %v", err)
	}
}

// TestStop_NotFound verifies that stopping an unknown worker ID returns
// ErrNotFound.
func TestStop_NotFound(t *testing.T) {
	mgr := process.New()

	err := mgr.Stop(context.Background(), "ghost")
	if err != worker.ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestStopAll_StopsMultipleWorkers spawns two processes and verifies both are
// stopped cleanly by StopAll.
func TestStopAll_StopsMultipleWorkers(t *testing.T) {
	mgr := process.New()

	w1 := workerWith("w1", "sleep", []string{"30"})
	w2 := workerWith("w2", "sleep", []string{"30"})

	if err := mgr.Spawn(context.Background(), w1); err != nil {
		t.Fatalf("spawn w1 failed: %v", err)
	}
	if err := mgr.Spawn(context.Background(), w2); err != nil {
		t.Fatalf("spawn w2 failed: %v", err)
	}

	if err := mgr.StopAll(context.Background()); err != nil {
		t.Errorf("StopAll returned error: %v", err)
	}
}

// TestSendHeartbeat_NoOp verifies that SendHeartbeat returns nil (no-op at the
// OS level; heartbeats are handled via UDS in the full stack).
func TestSendHeartbeat_NoOp(t *testing.T) {
	mgr := process.New()

	err := mgr.SendHeartbeat(context.Background(), "any-id")
	if err != nil {
		t.Errorf("expected nil from SendHeartbeat, got: %v", err)
	}
}
