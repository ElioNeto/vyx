package process_test

import (
	"context"
	"testing"
	"time"

	"github.com/ElioNeto/vyx/core/domain/worker"
	"github.com/ElioNeto/vyx/core/infrastructure/process"
)

func TestManager_New(t *testing.T) {
	mgr := process.New()
	if mgr == nil {
		t.Fatal("New() returned nil")
	}
}

func TestManager_NewWithOptions(t *testing.T) {
	var loggedMessages []string
	mockLogWriter := func(workerID string, line string) {
		loggedMessages = append(loggedMessages, workerID+": "+line)
	}

	mgr := process.New(process.WithLogWriter(mockLogWriter))
	if mgr == nil {
		t.Fatal("New() with options returned nil")
	}
}

func TestManager_SpawnAndStop(t *testing.T) {
	mgr := process.New()
	w := &worker.Worker{
		ID:        "test-sleep",
		Command:   sleepCmd,
		Args:      sleepArgs,
		State:     worker.StateStarting,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := mgr.Spawn(context.Background(), w); err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	if err := mgr.Stop(context.Background(), "test-sleep"); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

func TestManager_StopAlreadyStopped(t *testing.T) {
	mgr := process.New()
	
	// Try to stop a worker that was never started
	err := mgr.Stop(context.Background(), "non-existent")
	if err != worker.ErrNotFound {
		t.Errorf("Expected ErrNotFound, got: %v", err)
	}
}

func TestManager_StopAllEmpty(t *testing.T) {
	mgr := process.New()
	
	err := mgr.StopAll(context.Background())
	if err != nil {
		t.Errorf("StopAll on empty manager should not error, got: %v", err)
	}
}

func TestManager_StopAllMultiple(t *testing.T) {
	mgr := process.New()
	
	w1 := workerWith("w1", sleepCmd, sleepArgs)
	w2 := workerWith("w2", sleepCmd, sleepArgs)
	
	if err := mgr.Spawn(context.Background(), w1); err != nil {
		t.Fatalf("Spawn w1 failed: %v", err)
	}
	if err := mgr.Spawn(context.Background(), w2); err != nil {
		t.Fatalf("Spawn w2 failed: %v", err)
	}
	
	time.Sleep(100 * time.Millisecond)
	
	if err := mgr.StopAll(context.Background()); err != nil {
		t.Errorf("StopAll failed: %v", err)
	}
}

func TestManager_SpawnWithWorkDir(t *testing.T) {
	mgr := process.New()
	w := &worker.Worker{
		ID:        "test-workdir",
		Command:   sleepCmd,
		Args:      sleepArgs,
		WorkDir:   "/tmp",
		State:     worker.StateStarting,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	if err := mgr.Spawn(context.Background(), w); err != nil {
		t.Fatalf("Spawn with WorkDir failed: %v", err)
	}
	
	_ = mgr.Stop(context.Background(), "test-workdir")
}

func TestManager_SendHeartbeat(t *testing.T) {
	mgr := process.New()
	
	err := mgr.SendHeartbeat(context.Background(), "any-id")
	if err != nil {
		t.Errorf("SendHeartbeat should return nil, got: %v", err)
	}
}

func TestManager_StopWithTimeout(t *testing.T) {
	mgr := process.New()
	
	// Use a command that runs for a while
	w := workerWith("long-running", sleepCmd, []string{"10"})
	
	if err := mgr.Spawn(context.Background(), w); err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	
	// Stop should work within timeout
	if err := mgr.Stop(context.Background(), "long-running"); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

// Test processBufferChunk function indirectly through pipeLog
func TestManager_PipeLog(t *testing.T) {
	var loggedLines []string
	mockLogWriter := func(workerID string, line string) {
		loggedLines = append(loggedLines, line)
	}
	
	mgr := process.New(process.WithLogWriter(mockLogWriter))
	
	// We can't easily test pipeLog directly as it's unexported
	// But we can verify the LogWriter is set
	// This is more of a compile-time check
	_ = mgr
}

func TestSleepCMD(t *testing.T) {
	// Verify sleep command is defined
	if sleepCmd == "" {
		t.Error("sleepCmd should not be empty")
	}
	if len(sleepArgs) == 0 {
		t.Error("sleepArgs should not be empty")
	}
}
