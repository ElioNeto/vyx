// Package process implements the worker.Manager interface using os/exec to manage
// real child processes. This is the infrastructure layer — it contains OS-level details.
package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/ElioNeto/vyx/core/domain/worker"
)

const defaultShutdownTimeout = 5 * time.Second

// Manager spawns and manages OS child processes for each worker.
type Manager struct {
	mu        sync.RWMutex
	processes map[string]*exec.Cmd
}

// New creates an empty process Manager.
func New() *Manager {
	return &Manager{
		processes: make(map[string]*exec.Cmd),
	}
}

// Spawn starts a child process for the given worker.
func (m *Manager) Spawn(ctx context.Context, w *worker.Worker) error {
	if w.Command == "" {
		return worker.ErrInvalidCommand
	}

	cmd := exec.CommandContext(ctx, w.Command, w.Args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// WorkDir allows workers with their own go.mod (or any sub-module) to be
	// spawned with the correct working directory so tools like `go run` can
	// locate the module root without walking up to the repo root.
	if w.WorkDir != "" {
		cmd.Dir = w.WorkDir
	}
	setProcAttr(cmd) // platform-specific: Setpgid on Unix, CREATE_NEW_PROCESS_GROUP on Windows

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %s", worker.ErrSpawnFailed, err.Error())
	}

	m.mu.Lock()
	m.processes[w.ID] = cmd
	m.mu.Unlock()

	// Watch for unexpected process exit in the background.
	go func() {
		_ = cmd.Wait()
	}()

	return nil
}

// Stop sends a termination signal to the worker process and waits for it to exit.
// Falls back to kill after the shutdown timeout.
func (m *Manager) Stop(ctx context.Context, id string) error {
	m.mu.RLock()
	cmd, ok := m.processes[id]
	m.mu.RUnlock()

	if !ok || cmd.Process == nil {
		return worker.ErrNotFound
	}

	if err := stopProcess(cmd); err != nil {
		return err
	}

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(defaultShutdownTimeout):
		_ = killProcess(cmd)
		return worker.ErrStopTimeout
	}

	m.mu.Lock()
	delete(m.processes, id)
	m.mu.Unlock()

	return nil
}

// StopAll gracefully stops all managed processes.
func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.RLock()
	ids := make([]string, 0, len(m.processes))
	for id := range m.processes {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	var lastErr error
	for _, id := range ids {
		if err := m.Stop(ctx, id); err != nil && err != worker.ErrNotFound {
			lastErr = err
		}
	}
	return lastErr
}

// SendHeartbeat is not implemented at the OS process layer.
// Core→worker heartbeat frames are sent by application/heartbeat.Sender
// over the IPC transport — keeping this manager focused on spawn/stop only.
func (m *Manager) SendHeartbeat(_ context.Context, _ string) error {
	return nil
}
