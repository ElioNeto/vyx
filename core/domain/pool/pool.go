// Package pool implements worker pools with load balancing strategies.
package pool

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/ElioNeto/vyx/core/domain/worker"
)

// Strategy defines the load balancing algorithm.
type Strategy string

const (
	// RoundRobin distributes requests evenly across all workers.
	RoundRobin Strategy = "round-robin"
	// LeastLoaded sends requests to the worker with the fewest active requests.
	LeastLoaded Strategy = "least-loaded"
)

// workerState tracks runtime state of a worker in the pool.
type workerState struct {
	worker       *worker.Worker
	activeReqs   atomic.Int64 // number of in-flight requests
	lastHealthCheck time.Time
}

// Pool manages a group of worker instances for load distribution.
type Pool struct {
	mu          sync.RWMutex
	workers     map[string]*workerState // workerID -> worker state
	workerIDs   []string                // maintains insertion order for deterministic iteration
	strategy    Strategy
	counter     atomic.Int64 // for round-robin (using atomic for thread safety)
	lastUpdated time.Time
}

// NewPool creates a worker pool with the specified strategy.
func NewPool(strategy Strategy) *Pool {
	if strategy == "" {
		strategy = RoundRobin
	}
	return &Pool{
		workers:   make(map[string]*workerState),
		workerIDs: make([]string, 0),
		strategy:  strategy,
	}
}

// AddWorker adds a worker instance to the pool.
func (p *Pool) AddWorker(w *worker.Worker) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.workers[w.ID] = &workerState{
		worker:          w,
		activeReqs:      atomic.Int64{},
		lastHealthCheck: time.Now(),
	}
	p.workerIDs = append(p.workerIDs, w.ID)
	p.lastUpdated = time.Now()
}

// RemoveWorker removes a worker instance from the pool.
func (p *Pool) RemoveWorker(workerID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.workers, workerID)
	for i, id := range p.workerIDs {
		if id == workerID {
			p.workerIDs = append(p.workerIDs[:i], p.workerIDs[i+1:]...)
			break
		}
	}
	p.lastUpdated = time.Now()
}

// UpdateWorker updates a worker instance in the pool.
func (p *Pool) UpdateWorker(w *worker.Worker) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ws, ok := p.workers[w.ID]; ok {
		ws.worker = w
		ws.lastHealthCheck = time.Now()
	}
	p.lastUpdated = time.Now()
}

// GetWorkers returns all workers in the pool.
func (p *Pool) GetWorkers() []*worker.Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()
	workers := make([]*worker.Worker, 0, len(p.workers))
	for _, id := range p.workerIDs {
		if ws, ok := p.workers[id]; ok {
			workers = append(workers, ws.worker)
		}
	}
	return workers
}

// HealthyWorkers returns only the healthy (running) workers.
func (p *Pool) HealthyWorkers() []*worker.Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()
	healthy := make([]*worker.Worker, 0)
	for _, id := range p.workerIDs {
		if ws, ok := p.workers[id]; ok && ws.worker.IsAlive() {
			healthy = append(healthy, ws.worker)
		}
	}
	return healthy
}

// GetWorkerState returns the worker and its active request count.
func (p *Pool) GetWorkerState(workerID string) (*worker.Worker, int64, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ws, ok := p.workers[workerID]
	if !ok {
		return nil, 0, false
	}
	return ws.worker, ws.activeReqs.Load(), true
}

// IncrementActiveReqs increments the active request count for a worker.
func (p *Pool) IncrementActiveReqs(workerID string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if ws, ok := p.workers[workerID]; ok {
		ws.activeReqs.Add(1)
	}
}

// DecrementActiveReqs decrements the active request count for a worker.
func (p *Pool) DecrementActiveReqs(workerID string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if ws, ok := p.workers[workerID]; ok {
		ws.activeReqs.Add(-1)
	}
}

// SelectWorker chooses a worker based on the pool's strategy.
func (p *Pool) SelectWorker() *worker.Worker {
	healthy := p.HealthyWorkers()
	if len(healthy) == 0 {
		return nil
	}

	switch p.strategy {
	case LeastLoaded:
		return p.selectLeastLoaded(healthy)
	default: // RoundRobin
		return p.selectRoundRobin(healthy)
	}
}

// selectRoundRobin chooses a worker using round-robin algorithm.
func (p *Pool) selectRoundRobin(workers []*worker.Worker) *worker.Worker {
	if len(workers) == 0 {
		return nil
	}
	idx := p.counter.Load()
	worker := workers[idx%int64(len(workers))]
	p.counter.Add(1)
	return worker
}

// selectLeastLoaded chooses the worker with the fewest active requests.
func (p *Pool) selectLeastLoaded(workers []*worker.Worker) *worker.Worker {
	if len(workers) == 0 {
		return nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	var selected *worker.Worker
	var minReqs int64 = -1

	for _, w := range workers {
		ws, ok := p.workers[w.ID]
		if !ok {
			continue
		}
		reqs := ws.activeReqs.Load()
		if minReqs == -1 || reqs < minReqs {
			minReqs = reqs
			selected = w
		}
	}

	if selected == nil {
		return workers[0]
	}
	return selected
}

// Size returns the total number of workers in the pool.
func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.workers)
}

// HealthySize returns the number of healthy workers in the pool.
func (p *Pool) HealthySize() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	count := 0
	for _, id := range p.workerIDs {
		if ws, ok := p.workers[id]; ok && ws.worker.IsAlive() {
			count++
		}
	}
	return count
}

// Strategy returns the current load balancing strategy.
func (p *Pool) Strategy() Strategy {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.strategy
}

// LastUpdated returns the last time the pool was updated.
func (p *Pool) LastUpdated() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastUpdated
}