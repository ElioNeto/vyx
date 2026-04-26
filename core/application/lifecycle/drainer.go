// Package lifecycle contains the application use cases for worker lifecycle management.
package lifecycle

import (
	"context"
	"sync"
	"time"
)

// WorkerDrainer manages in-flight request tracking and graceful draining
// for worker processes during shutdown or restart operations.
type WorkerDrainer struct {
    mu         sync.RWMutex
    inflight   map[string]*sync.WaitGroup
    draining   map[string]bool  // NOVO: Estado de drain ativo
}

// NewWorkerDrainer creates a new WorkerDrainer instance.
func NewWorkerDrainer() *WorkerDrainer {
	return &WorkerDrainer{
		inflight: make(map[string]*sync.WaitGroup),
		draining: make(map[string]bool),
	}
}

// Acquire registers a new in-flight request for the given worker.
// Must be called before sending a request to the worker (e.g., in Dispatcher).
func (d *WorkerDrainer) Acquire(workerID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	wg, exists := d.inflight[workerID]
	if !exists {
		wg = &sync.WaitGroup{}
		d.inflight[workerID] = wg
	}
	wg.Add(1)
}

// Release marks a request as completed for the given worker.
// Must be called after receiving a response (or error) from the worker.
func (d *WorkerDrainer) Release(workerID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	wg, exists := d.inflight[workerID]
	if !exists {
		return
	}
	wg.Done()
}

// Drain waits for all in-flight requests for a worker to complete,
// or until the provided timeout expires. Returns nil on successful drain,
// context.DeadlineExceeded on timeout, or other error on context cancellation.
func (d *WorkerDrainer) Drain(ctx context.Context, workerID string, timeout time.Duration) error {
	d.mu.RLock()
	wg, exists := d.inflight[workerID]
	if !exists {
		d.mu.RUnlock()
		return nil // nothing to drain
	}
	d.mu.RUnlock()

	// Check current count without blocking
	// We need to wait for the WaitGroup to reach zero
	drainCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-drainCtx.Done():
		return drainCtx.Err()
	}
}

// MarkDraining marca o worker como em estado de drain antes da chamada Drain.
// Deve ser chamado pelo lifecycle.Service antes de iniciar o Drain.
func (d *WorkerDrainer) MarkDraining(workerID string) {
	d.mu.Lock()
	d.draining[workerID] = true
	d.mu.Unlock()
}

// IsDraining returns true if the worker is currently in a draining state.
// Note: This is advisory; actual draining is controlled by Drain().
func (d *WorkerDrainer) IsDraining(workerID string) bool {
    d.mu.RLock()
    defer d.mu.RUnlock()
    return d.draining[workerID]
}

// Cleanup removes internal state for a worker after it has been stopped.
// Should be called after successful drain and process termination.
func (d *WorkerDrainer) Cleanup(workerID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.inflight, workerID)
	delete(d.draining, workerID)
}
