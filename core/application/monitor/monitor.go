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
)

const (
	defaultHeartbeatInterval = 5 * time.Second
	backoffBase              = 1 * time.Second
	backoffMax               = 30 * time.Second
	backoffFactor            = 2.0
)

// Monitor periodically checks all workers and triggers restarts when needed.
type Monitor struct {
	service  *lifecycle.Service
	repo     worker.Repository
	interval time.Duration
	mu       sync.Mutex
	backoffs map[string]int // workerID → consecutive restart count
}

// New creates a Monitor with the default heartbeat interval.
func New(service *lifecycle.Service, repo worker.Repository) *Monitor {
	return &Monitor{
		service:  service,
		repo:     repo,
		interval: defaultHeartbeatInterval,
		backoffs: make(map[string]int),
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
		if w.State == worker.StateStopped || w.State == worker.StateRestarting || w.State == worker.StateStarting {
			continue
		}

		deadline := time.Now().Add(-m.interval * 2)
		if w.LastHeartbeat.Before(deadline) && w.State == worker.StateRunning {
			_ = m.service.MarkUnhealthy(ctx, w.ID)
			go m.scheduleRestart(ctx, w.ID)
		}
	}
}

// scheduleRestart waits for the computed backoff duration then restarts the worker.
func (m *Monitor) scheduleRestart(ctx context.Context, id string) {
	delay := m.nextBackoff(id)

	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
	}

	err := m.service.RestartWorker(ctx, id)
	if err != nil {
		m.mu.Lock()
		m.backoffs[id]++
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	delete(m.backoffs, id)
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
