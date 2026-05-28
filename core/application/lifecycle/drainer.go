// Package lifecycle contains the application use cases for worker lifecycle management.
package lifecycle

import (
	"context"
	"sync"
	"time"

	"github.com/ElioNeto/vyx/core/infrastructure/recovery"
)

// WorkerDrainer manages in-flight request tracking and graceful draining
// for worker processes during shutdown or restart operations.
//
// Each worker gets its own *sync.WaitGroup so that Acquire/Release
// operations are scoped per-worker — eliminating the global mutex
// contention that would otherwise occur under high throughput.
type WorkerDrainer struct {
	inflight sync.Map // map[string]*sync.WaitGroup
	draining sync.Map // map[string]bool
}

// NewWorkerDrainer creates a new WorkerDrainer instance.
func NewWorkerDrainer() *WorkerDrainer {
	return &WorkerDrainer{}
}

// Acquire registers a new in-flight request for the given worker.
// Must be called before sending a request to the worker (e.g., in Dispatcher).
func (d *WorkerDrainer) Acquire(workerID string) {
	wg, _ := d.inflight.LoadOrStore(workerID, &sync.WaitGroup{})
	wg.(*sync.WaitGroup).Add(1)
}

// Release marks a request as completed for the given worker.
// Must be called after receiving a response (or error) from the worker.
func (d *WorkerDrainer) Release(workerID string) {
	wg, ok := d.inflight.Load(workerID)
	if !ok {
		return
	}
	wg.(*sync.WaitGroup).Done()
}

// Drain waits for all in-flight requests for a worker to complete,
// or until the provided timeout expires. Returns nil on successful drain,
// context.DeadlineExceeded on timeout, or other error on context cancellation.
func (d *WorkerDrainer) Drain(ctx context.Context, workerID string, timeout time.Duration) error {
	wgI, ok := d.inflight.Load(workerID)
	if !ok {
		return nil // nothing to drain
	}
	wg := wgI.(*sync.WaitGroup)

	drainCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer recovery.LogPanic(nil, "drainer.wait", nil)
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
	d.draining.Store(workerID, true)
}

// IsDraining returns true if the worker is currently in a draining state.
// Note: This is advisory; actual draining is controlled by Drain().
func (d *WorkerDrainer) IsDraining(workerID string) bool {
	v, ok := d.draining.Load(workerID)
	return ok && v.(bool)
}

// Cleanup removes internal state for a worker after it has been stopped.
// Should be called after successful drain and process termination.
func (d *WorkerDrainer) Cleanup(workerID string) {
	d.inflight.Delete(workerID)
	d.draining.Delete(workerID)
}
