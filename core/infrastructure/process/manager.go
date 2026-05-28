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
	"github.com/ElioNeto/vyx/core/infrastructure/recovery"
)

const defaultShutdownTimeout = 5 * time.Second

// LogWriter is the callback invoked for each line of stdout/stderr from a worker.
type LogWriter func(workerID string, line string)

// Manager spawns and manages OS child processes for each worker.
type Manager struct {
	mu               sync.RWMutex
	processes        map[string]*exec.Cmd
	waitDone         map[string]chan struct{}
	logWriter        LogWriter
	shutdownTimeout  time.Duration
}

// New creates an empty process Manager. An optional LogWriter can be provided
// to capture worker stdout/stderr (used by the TUI log viewer).
func New(opts ...Option) *Manager {
	m := &Manager{
		processes: make(map[string]*exec.Cmd),
		waitDone:  make(map[string]chan struct{}),
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

// WithShutdownTimeout sets the timeout for graceful shutdown.
// If not set, defaultShutdownTimeout is used.
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(m *Manager) {
		m.shutdownTimeout = timeout
	}
}

// Spawn starts a child process for the given worker.
func (m *Manager) Spawn(ctx context.Context, w *worker.Worker) error {
	if w.Command == "" {
		return worker.ErrInvalidCommand
	}

	cmd := exec.Command(w.Command, w.Args...)

	// Use a reasonable WaitDelay to cap pipe I/O wait time, preventing
	// orphaned goroutines from blocking test teardown if child processes
	// leak (defence in depth — the process-group kill should handle it).
	cmd.WaitDelay = 3 * time.Second

	// Set up pipeLog goroutines for stdout/stderr when a log writer exists.
	// The context for these goroutines is created AFTER cmd.Start() succeeds
	// and cancelled when the process exits (see wait goroutine below).
	var (
		outWriter  io.WriteCloser
		errWriter  io.WriteCloser
		outReader  io.Reader
		errReader  io.Reader
		cancel     context.CancelFunc
	)
	if m.logWriter != nil {
		outReader, outWriter = io.Pipe()
		errReader, errWriter = io.Pipe()
		cmd.Stdout = outWriter
		cmd.Stderr = errWriter
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

	// Create cancel context for pipeLog goroutines AFTER process starts.
	// This ensures cancel is always called on all code paths.
	var pipeCtx context.Context
	if m.logWriter != nil {
		pipeCtx, cancel = context.WithCancel(ctx)
		workerID := w.ID
		go func() {
			defer recovery.LogPanic(nil, "process.pipe_log_stdout", nil)
			m.pipeLog(pipeCtx, m.logWriter, workerID, outReader)
		}()
		go func() {
			defer recovery.LogPanic(nil, "process.pipe_log_stderr", nil)
			m.pipeLog(pipeCtx, m.logWriter, workerID, errReader)
		}()
	}

	m.mu.Lock()
	m.processes[w.ID] = cmd
	waitCh := make(chan struct{})
	m.waitDone[w.ID] = waitCh
	m.mu.Unlock()

	// Wait for the process to exit, then clean up pipeLog goroutines
	// and signal the waitCh so Stop() can complete.
	go func() {
		defer recovery.LogPanic(nil, "process.wait_cleanup", nil)
		_ = cmd.Wait()
		if cancel != nil {
			cancel()
		}
		if outWriter != nil {
			outWriter.Close()
		}
		if errWriter != nil {
			errWriter.Close()
		}
		close(waitCh)
	}()

	return nil
}

// pipeLog reads from a pipe and calls the logWriter for each non-empty line.
// It exits when the context is cancelled or the reader returns an error.
func (m *Manager) pipeLog(ctx context.Context, writer LogWriter, workerID string, r io.Reader) {
	const chunkSize = 64 * 1024
	buf := make([]byte, chunkSize)
	var partial []byte // carries over incomplete lines across reads

	for {
		select {
		case <-ctx.Done():
			// Flush any remaining partial data.
			if len(partial) > 0 {
				writer(workerID, string(partial))
			}
			return
		default:
		}

		n, err := r.Read(buf)
		if n > 0 {
			data := append(partial, buf[:n]...)
			partial = m.processChunk(writer, workerID, data)
		}
		if err != nil {
			// Flush remaining partial data on EOF.
			if len(partial) > 0 {
				writer(workerID, string(partial))
			}
			return
		}
	}
}

// processChunk scans data for newlines and emits complete lines
// to writer. It returns any trailing data that didn't end with a
// newline so the caller can carry it over to the next read.
func (m *Manager) processChunk(writer LogWriter, workerID string, data []byte) []byte {
	start := 0
	for i, b := range data {
		if b == '\n' {
			line := string(data[start:i])
			if line != "" {
				writer(workerID, line)
			}
			start = i + 1
		}
	}
	if start < len(data) {
		// Incomplete line — preserve for the next chunk.
		out := make([]byte, len(data)-start)
		copy(out, data[start:])
		return out
	}
	return nil
}

func (m *Manager) processBufferChunk(writer LogWriter, workerID string, buf []byte) int {
	lineStart := 0
	for i, b := range buf {
		if b == '\n' {
			line := string(buf[lineStart:i])
			if line != "" {
				writer(workerID, line)
			}
			lineStart = i + 1
		}
	}
	return lineStart
}

// Stop sends a termination signal to the worker process and waits for it to exit.
// Falls back to kill after the shutdown timeout.
func (m *Manager) Stop(ctx context.Context, id string) error {
	m.mu.RLock()
	cmd, cmdOk := m.processes[id]
	waitCh, waitOk := m.waitDone[id]
	m.mu.RUnlock()

	if !cmdOk || cmd.Process == nil {
		return worker.ErrNotFound
	}

	if err := stopProcess(cmd); err != nil {
		return err
	}

	timeout := defaultShutdownTimeout
	if m.shutdownTimeout > 0 {
		timeout = m.shutdownTimeout
	}

	if waitOk {
		select {
		case <-waitCh:
		case <-time.After(timeout):
			_ = killProcess(cmd)
			m.mu.Lock()
			delete(m.processes, id)
			delete(m.waitDone, id)
			m.mu.Unlock()
			return worker.ErrStopTimeout
		}
	}

	m.mu.Lock()
	delete(m.processes, id)
	delete(m.waitDone, id)
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
