// Package repository provides in-memory and future persistent implementations
// of domain repository interfaces.
package repository

import (
	"context"
	"sync"

	"github.com/ElioNeto/vyx/core/domain/worker"
)

// MemoryWorkerRepository is a thread-safe in-memory implementation of worker.Repository.
type MemoryWorkerRepository struct {
	mu      sync.RWMutex
	workers map[string]*worker.Worker
}

// NewMemoryWorkerRepository creates an empty in-memory worker store.
func NewMemoryWorkerRepository() *MemoryWorkerRepository {
	return &MemoryWorkerRepository{
		workers: make(map[string]*worker.Worker),
	}
}

func (r *MemoryWorkerRepository) Save(_ context.Context, w *worker.Worker) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *w
	r.workers[w.ID] = &copy
	return nil
}

func (r *MemoryWorkerRepository) FindByID(_ context.Context, id string) (*worker.Worker, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	w, ok := r.workers[id]
	if !ok {
		return nil, nil
	}
	copy := *w
	return &copy, nil
}

func (r *MemoryWorkerRepository) FindAll(_ context.Context) ([]*worker.Worker, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*worker.Worker, 0, len(r.workers))
	for _, w := range r.workers {
		copy := *w
		result = append(result, &copy)
	}
	return result, nil
}

func (r *MemoryWorkerRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.workers, id)
	return nil
}

// LiveWorkerIDs returns the IDs of all workers currently in the repository.
// Satisfies the heartbeat.WorkerLister interface.
func (r *MemoryWorkerRepository) LiveWorkerIDs(_ context.Context) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.workers))
	for id := range r.workers {
		ids = append(ids, id)
	}
	return ids, nil
}
