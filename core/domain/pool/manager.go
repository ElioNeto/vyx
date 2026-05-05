// Package pool implements worker pools with load balancing strategies and lifecycle management.
package pool

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ElioNeto/vyx/core/domain/worker"
)

// ErrPoolNotFound is returned when a pool is not found.
var ErrPoolNotFound = errors.New("pool not found")

// ManagerConfig holds configuration for the pool manager.
type ManagerConfig struct {
	Replicas    int
	Strategy    Strategy
	MaxPoolSize int // 0 means unlimited
}

// Manager handles the lifecycle of worker pools including spawning, health checking, and replacement.
type Manager struct {
	mu         sync.RWMutex
	pools      map[string]*Pool            // workerID prefix -> Pool
	configs    map[string]ManagerConfig
	workerCfgs map[string]*worker.Worker   // workerID prefix -> base worker config
	workerRepo worker.Repository
	workerMgr  worker.Manager
}

// NewManager creates a new pool manager.
func NewManager(workerRepo worker.Repository, workerMgr worker.Manager) *Manager {
	return &Manager{
		pools:      make(map[string]*Pool),
		configs:    make(map[string]ManagerConfig),
		workerCfgs: make(map[string]*worker.Worker),
		workerRepo: workerRepo,
		workerMgr:  workerMgr,
	}
}

// RegisterPool registers a new pool with the given configuration.
func (m *Manager) RegisterPool(workerIDPrefix string, cfg ManagerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg.Strategy == "" {
		cfg.Strategy = RoundRobin
	}
	if cfg.Replicas <= 0 {
		cfg.Replicas = 1
	}

	m.configs[workerIDPrefix] = cfg
	m.pools[workerIDPrefix] = NewPool(cfg.Strategy)
}

// RegisterWorkerConfig stores the base worker configuration for a pool.
func (m *Manager) RegisterWorkerConfig(workerIDPrefix string, cfg *worker.Worker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workerCfgs[workerIDPrefix] = cfg
}

// GetPool returns the pool for the given worker ID prefix.
func (m *Manager) GetPool(workerIDPrefix string) (*Pool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pool, ok := m.pools[workerIDPrefix]
	return pool, ok
}

// SpawnWorkers creates the initial set of worker instances for a pool.
func (m *Manager) SpawnWorkers(ctx context.Context, workerIDPrefix string, workerCfg *worker.Worker) error {
	m.mu.RLock()
	cfg, cfgOk := m.configs[workerIDPrefix]
	pool, poolOk := m.pools[workerIDPrefix]
	m.mu.RUnlock()

	if !cfgOk || !poolOk {
		return ErrPoolNotFound
	}

	count := cfg.Replicas
	// Respect MaxPoolSize: limit spawning to available capacity.
	currentSize := pool.Size()
	if cfg.MaxPoolSize > 0 {
		if currentSize >= cfg.MaxPoolSize {
			return nil // pool already at max capacity
		}
		maxAllowed := cfg.MaxPoolSize - currentSize
		if count > maxAllowed {
			count = maxAllowed
		}
	}

	for i := 0; i < count; i++ {
		instanceID := workerIDPrefix
		if i > 0 {
			instanceID = workerIDPrefix + "-" + itoa(i)
		}

		w := *workerCfg // copy
		w.ID = instanceID

		if err := m.workerMgr.Spawn(ctx, &w); err != nil {
			return err
		}

		pool.AddWorker(&w)
	}

	return nil
}

// ReplaceWorker removes an unhealthy worker and spawns a new one.
func (m *Manager) ReplaceWorker(ctx context.Context, workerID string) error {
	// Extract prefix
	prefix := extractPrefix(workerID)

	m.mu.RLock()
	pool, ok := m.pools[prefix]
	m.mu.RUnlock()

	if !ok {
		return ErrPoolNotFound
	}

	// Get the old worker
	oldWorker, _, found := pool.GetWorkerState(workerID)
	if !found {
		return errors.New("worker not found in pool")
	}

	// Remove the old worker
	pool.RemoveWorker(workerID)
	_ = m.workerMgr.Stop(ctx, workerID)

	// Spawn a new worker with the same config
	newWorker := *oldWorker
	newWorker.ID = generateWorkerID(prefix)

	if err := m.workerMgr.Spawn(ctx, &newWorker); err != nil {
		return err
	}

	pool.AddWorker(&newWorker)
	return nil
}

// MonitorHealth checks the health of all workers and replaces unhealthy ones.
func (m *Manager) MonitorHealth(ctx context.Context) {
	m.mu.RLock()
	pools := make(map[string]*Pool)
	for k, v := range m.pools {
		pools[k] = v
	}
	m.mu.RUnlock()

	for prefix, pool := range pools {
		workers := pool.GetWorkers()
		for _, w := range workers {
			if !w.IsAlive() {
				// Remove unhealthy worker; ensureReplicas will spawn a replacement.
				pool.RemoveWorker(w.ID)
				_ = m.workerMgr.Stop(ctx, w.ID)
			}
		}
		// Ensure we have the correct number of replicas
		m.ensureReplicas(ctx, prefix, pool)
	}
}

// ensureReplicas checks if the pool has the correct number of healthy workers.
func (m *Manager) ensureReplicas(ctx context.Context, prefix string, pool *Pool) {
	m.mu.RLock()
	cfg, ok := m.configs[prefix]
	workerCfg, cfgOk := m.workerCfgs[prefix]
	m.mu.RUnlock()

	if !ok || !cfgOk {
		return
	}

	healthy := pool.HealthySize()
	spawnCount := cfg.Replicas - healthy

	// Check max pool size enforcement
	currentSize := pool.Size()
	if cfg.MaxPoolSize > 0 {
		maxAllowed := cfg.MaxPoolSize - currentSize
		if spawnCount > maxAllowed {
			spawnCount = maxAllowed
		}
		if spawnCount < 0 {
			spawnCount = 0
		}
	}

	if spawnCount > 0 && workerCfg != nil {
		for i := 0; i < spawnCount; i++ {
			newWorker := *workerCfg
			newWorker.ID = generateWorkerID(prefix)

			if err := m.workerMgr.Spawn(ctx, &newWorker); err != nil {
				continue // Continue spawning other workers even if one fails
			}

			pool.AddWorker(&newWorker)
		}
	}
}

// StopAll stops all workers in all pools.
func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.RLock()
	pools := make(map[string]*Pool)
	for k, v := range m.pools {
		pools[k] = v
	}
	m.mu.RUnlock()

	for _, pool := range pools {
		workers := pool.GetWorkers()
		for _, w := range workers {
			_ = m.workerMgr.Stop(ctx, w.ID)
		}
	}
	return nil
}

// extractPrefix extracts the prefix from a worker ID.
// Example: "node:products-2" -> "node:products"
func extractPrefix(workerID string) string {
	// Find the last dash followed by a number
	for i := len(workerID) - 1; i >= 0; i-- {
		if workerID[i] == '-' {
			// Check if the rest is a number
			isNum := true
			for j := i + 1; j < len(workerID); j++ {
				if workerID[j] < '0' || workerID[j] > '9' {
					isNum = false
					break
				}
			}
			if isNum && i > 0 {
				return workerID[:i]
			}
		}
	}
	return workerID
}

// generateWorkerID generates a new unique worker ID for the given prefix.
func generateWorkerID(prefix string) string {
	return prefix + "-" + time.Now().Format("20060102-150405.000")
}

// itoa converts an integer to a string (simplified version).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
