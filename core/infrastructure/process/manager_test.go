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

func TestSpawn_ValidCommand(t *testing.T) {
	mgr := process.New()
	w := workerWith("test-sleep", sleepCmd, sleepArgs)

	if err := mgr.Spawn(context.Background(), w); err != nil {
		t.Fatalf("expected spawn to succeed, got: %v", err)
	}
	_ = mgr.Stop(context.Background(), "test-sleep")
}

func TestSpawn_EmptyCommand(t *testing.T) {
	mgr := process.New()
	w := workerWith("bad", "", nil)

	err := mgr.Spawn(context.Background(), w)
	if err != worker.ErrInvalidCommand {
		t.Errorf("expected ErrInvalidCommand, got: %v", err)
	}
}

func TestSpawn_InvalidBinary(t *testing.T) {
	mgr := process.New()
	w := workerWith("bad-bin", "__definitely_not_a_real_binary__", nil)

	err := mgr.Spawn(context.Background(), w)
	if err == nil {
		t.Fatal("expected error for non-existent binary, got nil")
	}
}

func TestStop_GracefulShutdown(t *testing.T) {
	mgr := process.New()
	w := workerWith("test-stop", sleepCmd, sleepArgs)

	if err := mgr.Spawn(context.Background(), w); err != nil {
		t.Fatalf("spawn failed: %v", err)
	}
	if err := mgr.Stop(context.Background(), "test-stop"); err != nil {
		t.Errorf("expected clean stop, got: %v", err)
	}
}

func TestStop_NotFound(t *testing.T) {
	mgr := process.New()

	err := mgr.Stop(context.Background(), "ghost")
	if err != worker.ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestStopAll_StopsMultipleWorkers(t *testing.T) {
	mgr := process.New()

	w1 := workerWith("w1", sleepCmd, sleepArgs)
	w2 := workerWith("w2", sleepCmd, sleepArgs)

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

func TestSendHeartbeat_NoOp(t *testing.T) {
	mgr := process.New()

	if err := mgr.SendHeartbeat(context.Background(), "any-id"); err != nil {
		t.Errorf("expected nil from SendHeartbeat, got: %v", err)
	}
}
