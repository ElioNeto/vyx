package pool_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ElioNeto/vyx/core/domain/config"
	"github.com/ElioNeto/vyx/core/domain/pool"
	"github.com/ElioNeto/vyx/core/domain/worker"
	"github.com/stretchr/testify/require"
)

// IntegrationTestWorkerManager is a mock implementation for integration tests.
type IntegrationTestWorkerManager struct {
	mu             sync.Mutex
	spawnedWorkers map[string]*worker.Worker
}

func (m *IntegrationTestWorkerManager) Spawn(ctx context.Context, w *worker.Worker) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spawnedWorkers[w.ID] = w
	return nil
}

func (m *IntegrationTestWorkerManager) Stop(ctx context.Context, workerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.spawnedWorkers, workerID)
	return nil
}

func (m *IntegrationTestWorkerManager) StopAll(ctx context.Context) error { return nil }

func (m *IntegrationTestWorkerManager) SendHeartbeat(ctx context.Context, workerID string) error { return nil }

func TestIntegration_RoundRobinLoadBalancing(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &IntegrationTestWorkerManager{
		spawnedWorkers: make(map[string]*worker.Worker),
	}
	manager := pool.NewManager(mockRepo, mockMgr)

	cfg := pool.ManagerConfig{
		Replicas:    3,
		Strategy:    pool.RoundRobin,
		MaxPoolSize: 10,
	}
	manager.RegisterPool("test:worker", cfg)

	workerCfg := &worker.Worker{
		ID:      "test:worker",
		Command: "echo test",
		State:   worker.StateRunning,
	}

	err := manager.SpawnWorkers(context.Background(), "test:worker", workerCfg)
	require.NoError(t, err)

	// Get the pool and test round-robin distribution
	pools, ok := manager.GetPool("test:worker")
	require.True(t, ok)

	// Select workers multiple times and verify round-robin
	selectedWorkers := make([]string, 6)
	for i := 0; i < 6; i++ {
		w := pools.SelectWorker()
		require.NotNil(t, w)
		selectedWorkers[i] = w.ID
	}

	// Should cycle through all 3 workers twice
	require.Equal(t, selectedWorkers[0], selectedWorkers[3])
	require.Equal(t, selectedWorkers[1], selectedWorkers[4])
	require.Equal(t, selectedWorkers[2], selectedWorkers[5])
}

func TestIntegration_LeastLoadedLoadBalancing(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &IntegrationTestWorkerManager{
		spawnedWorkers: make(map[string]*worker.Worker),
	}
	manager := pool.NewManager(mockRepo, mockMgr)

	cfg := pool.ManagerConfig{
		Replicas:    3,
		Strategy:    pool.LeastLoaded,
		MaxPoolSize: 10,
	}
	manager.RegisterPool("test:worker", cfg)

	workerCfg := &worker.Worker{
		ID:      "test:worker",
		Command: "echo test",
		State:   worker.StateRunning,
	}

	err := manager.SpawnWorkers(context.Background(), "test:worker", workerCfg)
	require.NoError(t, err)

	pools, ok := manager.GetPool("test:worker")
	require.True(t, ok)

	// Simulate more load on first worker
	w1 := pools.SelectWorker()
	pools.IncrementActiveReqs(w1.ID)
	pools.IncrementActiveReqs(w1.ID)
	pools.IncrementActiveReqs(w1.ID)

	// Should select a different worker (w2 or w3)
	w2 := pools.SelectWorker()
	require.NotEqual(t, w1.ID, w2.ID)
}

func TestIntegration_UnhealthyWorkerReplacement(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &IntegrationTestWorkerManager{
		spawnedWorkers: make(map[string]*worker.Worker),
	}
	manager := pool.NewManager(mockRepo, mockMgr)

	cfg := pool.ManagerConfig{
		Replicas:    3,
		Strategy:    pool.RoundRobin,
		MaxPoolSize: 10,
	}
	manager.RegisterPool("test:worker", cfg)

	workerCfg := &worker.Worker{
		ID:      "test:worker",
		Command: "echo test",
		State:   worker.StateRunning,
	}
	manager.RegisterWorkerConfig("test:worker", workerCfg)

	err := manager.SpawnWorkers(context.Background(), "test:worker", workerCfg)
	require.NoError(t, err)

	// Get the pool and mark one worker as unhealthy
	pools, ok := manager.GetPool("test:worker")
	require.True(t, ok)

	workers := pools.GetWorkers()
	require.Equal(t, 3, len(workers))

	// Mark first worker as unhealthy
	workers[0].State = worker.StateUnhealthy

	// MonitorHealth should replace the unhealthy worker
	manager.MonitorHealth(context.Background())

	// Should have spawned a new worker (3 initial - 1 stopped + 1 new = 3)
	require.Equal(t, 3, len(mockMgr.spawnedWorkers))

	// Pool should have 3 healthy workers
	healthy := pools.HealthyWorkers()
	require.Equal(t, 3, len(healthy))
}

func TestIntegration_ConfigToPoolFlow(t *testing.T) {
	// Simulate the full flow from config to pool creation
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test", Version: "1.0.0"},
		Workers: []config.WorkerConfig{
			{
				ID:              "test:worker",
				Command:         "echo test",
				Replicas:        3,
				Strategy:        "round-robin",
				PoolSize:        10,
				StartupTimeout:  30 * time.Second,
				ShutdownTimeout: 5 * time.Second,
			},
		},
		Security: config.SecurityConfig{
			JWTSecretEnv: "JWT_SECRET",
		},
	}

	err := cfg.Validate()
	require.NoError(t, err)

	// Create pool manager
	mockRepo := &MockWorkerRepository{}
	mockMgr := &IntegrationTestWorkerManager{
		spawnedWorkers: make(map[string]*worker.Worker),
	}
	manager := pool.NewManager(mockRepo, mockMgr)

	// Register pool with config
	managerConfig := pool.ManagerConfig{
		Replicas:    cfg.Workers[0].Replicas,
		Strategy:    pool.Strategy(cfg.Workers[0].Strategy),
		MaxPoolSize: cfg.Workers[0].PoolSize,
	}
	manager.RegisterPool(cfg.Workers[0].ID, managerConfig)

	// Spawn workers
	workerCfg := &worker.Worker{
		ID:      cfg.Workers[0].ID,
		Command: cfg.Workers[0].Command,
		State:   worker.StateRunning,
	}

	err = manager.SpawnWorkers(context.Background(), cfg.Workers[0].ID, workerCfg)
	require.NoError(t, err)

	// Verify pool has correct number of workers
	pools, ok := manager.GetPool(cfg.Workers[0].ID)
	require.True(t, ok)
	require.Equal(t, 3, pools.Size())
}

func TestIntegration_ConcurrentWorkerSelection(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &IntegrationTestWorkerManager{
		spawnedWorkers: make(map[string]*worker.Worker),
	}
	manager := pool.NewManager(mockRepo, mockMgr)

	cfg := pool.ManagerConfig{
		Replicas:    5,
		Strategy:    pool.RoundRobin,
		MaxPoolSize: 10,
	}
	manager.RegisterPool("test:worker", cfg)

	workerCfg := &worker.Worker{
		ID:      "test:worker",
		Command: "echo test",
		State:   worker.StateRunning,
	}

	err := manager.SpawnWorkers(context.Background(), "test:worker", workerCfg)
	require.NoError(t, err)

	pools, ok := manager.GetPool("test:worker")
	require.True(t, ok)

	// Concurrent worker selection
	var wg sync.WaitGroup
	results := make([]string, 1000)
	var mu sync.Mutex

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			w := pools.SelectWorker()
			mu.Lock()
			results[idx] = w.ID
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify all selections succeeded
	for _, r := range results {
		require.NotEmpty(t, r)
	}
}

func TestIntegration_MaxPoolSizePreventsExhaustion(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &IntegrationTestWorkerManager{
		spawnedWorkers: make(map[string]*worker.Worker),
	}
	manager := pool.NewManager(mockRepo, mockMgr)

	// Set max pool size to 3
	cfg := pool.ManagerConfig{
		Replicas:    10, // Request 10 replicas
		Strategy:    pool.RoundRobin,
		MaxPoolSize: 3,  // But limit to 3
	}
	manager.RegisterPool("test:worker", cfg)

	workerCfg := &worker.Worker{
		ID:      "test:worker",
		Command: "echo test",
		State:   worker.StateRunning,
	}

	err := manager.SpawnWorkers(context.Background(), "test:worker", workerCfg)
	require.NoError(t, err)

	// MonitorHealth should respect max pool size
	manager.MonitorHealth(context.Background())

	// Should have spawned only 3 workers (max pool size)
	require.Equal(t, 3, len(mockMgr.spawnedWorkers))
}
