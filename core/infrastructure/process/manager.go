// Package process implements the worker.Manager interface using os/exec to manage
// real child processes. This is the infrastructure layer — it contains OS-level details.
package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/ElioNeto/vyx/core/domain/worker"
)

const defaultShutdownTimeout = 5 * time.Second

// LogWriter is the callback invoked for each line of stdout/stderr from a worker.
type LogWriter func(workerID string, line string)

// Manager spawns and manages OS child processes for each worker.
type Manager struct {
	mu        sync.RWMutex
	processes map[string]*exec.Cmd
	logWriter LogWriter
}

// New creates an empty process Manager. An optional LogWriter can be provided
// to capture worker stdout/stderr (used by the TUI log viewer).
func New(opts ...Option) *Manager {
	m := &Manager{
		processes: make(map[string]*exec.Cmd),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Option configures a Manager.
type Option func(*Manager)

// WithLogWriter sets a callback for capturing worker output.
func WithLogWriter(w LogWriter) Option {
	return func(m *Manager) {
		m.logWriter = w
	}
}

// Spawn starts a child process for the given worker.
func (m *Manager) Spawn(ctx context.Context, w *worker.Worker) error {
	if w.Command == "" {
		return worker.ErrInvalidCommand
	}

	cmd := exec.Command(w.Command, w.Args...)

	if m.logWriter != nil {
		// Capture stdout/stderr through pipes so lines can be multiplexed
		// into the TUI while still preserving the data in the ring buffer.
		outReader, outWriter := io.Pipe()
		errReader, errWriter := io.Pipe()
		cmd.Stdout = outWriter
		cmd.Stderr = errWriter

		// Fan-out: tee the output to the log writer AND stderr (so it's still
		// visible when not running the TUI).
		workerID := w.ID
		go m.pipeLog(m.logWriter, workerID, outReader)
		go m.pipeLog(m.logWriter, workerID, errReader)

		outWriter.Close()  // writer side — cmd will take over
		errWriter.Close()
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if w.WorkDir != "" {
		cmd.Dir = w.WorkDir
	}
	setProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %s", worker.ErrSpawnFailed, err.Error())
	}

	m.mu.Lock()
	m.processes[w.ID] = cmd
	m.mu.Unlock()

	go func() {
		_ = cmd.Wait()
	}()

	return nil
}

// pipeLog reads from a pipe and calls the logWriter for each non-empty line.
func (m *Manager) pipeLog(writer LogWriter, workerID string, r io.Reader) {
	buf := make([]byte, 0, 64*1024)
	lineStart := 0
	for {
		n, err := r.Read(buf[lineStart:cap(buf)])
		if n > 0 {
			buf = buf[:lineStart+n]
			for i, b := range buf {
				if b == '\n' {
					line := string(buf[:i])
					buf = buf[i+1:]
					if line != "" {
						writer(workerID, line)
					}
				}
			}
			lineStart = 0
		}
		if err != nil {
			if len(buf) > 0 {
				line := string(buf)
				if line != "" {
					writer(workerID, line)
				}
			}
			return
		}
	}
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
