// Package monitor implements the health-check loop that watches workers via heartbeats
// and triggers restarts using exponential backoff when a worker becomes unhealthy.
package monitor

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/ElioNeto/vyx/core/application/lifecycle"
	"github.com/ElioNeto/vyx/core/domain/worker"
	"github.com/ElioNeto/vyx/core/infrastructure/recovery"
)

const (
	defaultHeartbeatInterval = 5 * time.Second
	backoffBase              = 1 * time.Second
	backoffMax               = 30 * time.Second
	backoffFactor            = 2.0
)

var backoffMapMaxSize = 1000

// Monitor periodically checks all workers and triggers restarts when needed.
type Monitor struct {
	service      *lifecycle.Service
	repo         worker.Repository
	interval     time.Duration
	mu           sync.Mutex
	backoffs     map[string]int // workerID → consecutive restart count
	backoffOrder []string       // insertion order for FIFO eviction when map is full
}

// New creates a Monitor with the default heartbeat interval.
func New(service *lifecycle.Service, repo worker.Repository) *Monitor {
	return &Monitor{
		service:      service,
		repo:         repo,
		interval:     defaultHeartbeatInterval,
		backoffs:     make(map[string]int),
		backoffOrder: make([]string, 0, backoffMapMaxSize),
	}
}

// Run starts the health-check loop. Blocks until ctx is cancelled.
func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAll(ctx)
		}
	}
}

func (m *Monitor) checkAll(ctx context.Context) {
	workers, err := m.repo.FindAll(ctx)
	if err != nil {
		return
	}

	for _, w := range workers {
		// Only check health for workers in StateRunning that have received at least one heartbeat.
		// Workers in StateStarting (or any other non-running state) that haven't sent their first
		// heartbeat yet have LastHeartbeat.IsZero(), which would be before any deadline and would
		// cause a false-positive unhealthy mark.
		if w.State != worker.StateRunning || w.LastHeartbeat.IsZero() {
			continue
		}

		deadline := time.Now().Add(-m.interval * 2)
		if w.LastHeartbeat.Before(deadline) {
			_ = m.service.MarkUnhealthy(ctx, w.ID)
			go m.scheduleRestart(ctx, w.ID)
		}
	}
}

// scheduleRestart waits for the computed backoff duration then restarts the worker.
// It runs in a goroutine launched by checkAll.
func (m *Monitor) scheduleRestart(ctx context.Context, id string) {
	defer recovery.LogPanic(nil, "monitor.schedule_restart", nil)
	delay := m.nextBackoff(id)

	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
	}

	err := m.service.RestartWorker(ctx, id)
	if err != nil {
		m.mu.Lock()
		m.addBackoffEntry(id)
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	m.removeBackoffEntry(id)
	m.mu.Unlock()
}

// nextBackoff returns the backoff duration for a given worker (exponential, capped at backoffMax).
func (m *Monitor) nextBackoff(id string) time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()

	attempt := m.backoffs[id]
	duration := time.Duration(float64(backoffBase) * math.Pow(backoffFactor, float64(attempt)))
	if duration > backoffMax {
		duration = backoffMax
	}
	return duration
}

// addBackoffEntry increments the backoff counter for a worker.
// If the map is at capacity, the oldest entry is evicted first (FIFO).
// Must be called with m.mu held.
func (m *Monitor) addBackoffEntry(id string) {
	if _, exists := m.backoffs[id]; !exists {
		// Evict the oldest entry if at capacity.
		if len(m.backoffs) >= backoffMapMaxSize && len(m.backoffOrder) > 0 {
			oldest := m.backoffOrder[0]
			m.backoffOrder = m.backoffOrder[1:]
			delete(m.backoffs, oldest)
		}
		m.backoffOrder = append(m.backoffOrder, id)
	}
	m.backoffs[id]++
}

// removeBackoffEntry deletes the backoff tracking for a worker.
// Must be called with m.mu held.
func (m *Monitor) removeBackoffEntry(id string) {
	delete(m.backoffs, id)
	for i, v := range m.backoffOrder {
		if v == id {
			m.backoffOrder = append(m.backoffOrder[:i], m.backoffOrder[i+1:]...)
			return
		}
	}
}
