package pool_test

import (
	"context"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/pool"
	"github.com/ElioNeto/vyx/core/domain/worker"
	"github.com/stretchr/testify/require"
)

// MockWorkerManager is a mock implementation of worker.Manager for testing.
type MockWorkerManager struct {
	spawnedWorkers []*worker.Worker
	stoppedWorkers []string
}

func (m *MockWorkerManager) Spawn(ctx context.Context, w *worker.Worker) error {
	m.spawnedWorkers = append(m.spawnedWorkers, w)
	return nil
}

func (m *MockWorkerManager) Stop(ctx context.Context, workerID string) error {
	m.stoppedWorkers = append(m.stoppedWorkers, workerID)
	return nil
}

func (m *MockWorkerManager) StopAll(ctx context.Context) error { return nil }

func (m *MockWorkerManager) SendHeartbeat(ctx context.Context, workerID string) error { return nil }

// MockWorkerRepository is a mock implementation of worker.Repository for testing.
type MockWorkerRepository struct{}

func (m *MockWorkerRepository) Save(ctx context.Context, w *worker.Worker) error {
	return nil
}

func (m *MockWorkerRepository) FindByID(ctx context.Context, workerID string) (*worker.Worker, error) {
	return nil, worker.ErrNotFound
}

func (m *MockWorkerRepository) FindAll(ctx context.Context) ([]*worker.Worker, error) {
	return nil, nil
}

func (m *MockWorkerRepository) Delete(ctx context.Context, workerID string) error {
	return nil
}

func TestManager_SpawnWorkers(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &MockWorkerManager{}
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
	require.Equal(t, 3, len(mockMgr.spawnedWorkers))
}

func TestManager_ReplaceWorker(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &MockWorkerManager{}
	manager := pool.NewManager(mockRepo, mockMgr)

	cfg := pool.ManagerConfig{
		Replicas:    2,
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

	// Get the first worker
	pools, _ := manager.GetPool("test:worker")
	workers := pools.GetWorkers()
	require.Equal(t, 2, len(workers))

	// Replace the first worker
	err = manager.ReplaceWorker(context.Background(), workers[0].ID)
	require.NoError(t, err)

	// Should have spawned a new worker
	require.Equal(t, 3, len(mockMgr.spawnedWorkers))
}

func TestManager_EnsureReplicas(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &MockWorkerManager{}
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

	// Spawn only 2 workers initially
	err := manager.SpawnWorkers(context.Background(), "test:worker", workerCfg)
	require.NoError(t, err)

	// MonitorHealth should spawn 3 more to reach 5 replicas
	ctx := context.Background()
	manager.MonitorHealth(ctx)

	// Should have spawned 5 workers total
	require.Equal(t, 5, len(mockMgr.spawnedWorkers))
}

func TestManager_MaxPoolSizeEnforcement(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &MockWorkerManager{}
	manager := pool.NewManager(mockRepo, mockMgr)

	cfg := pool.ManagerConfig{
		Replicas:    10,
		Strategy:    pool.RoundRobin,
		MaxPoolSize: 3,
	}
	manager.RegisterPool("test:worker", cfg)

	workerCfg := &worker.Worker{
		ID:      "test:worker",
		Command: "echo test",
		State:   worker.StateRunning,
	}

	// Spawn 3 workers (max pool size)
	err := manager.SpawnWorkers(context.Background(), "test:worker", workerCfg)
	require.NoError(t, err)

	// MonitorHealth should not spawn more due to max pool size
	manager.MonitorHealth(context.Background())

	// Should have spawned only 3 workers
	require.Equal(t, 3, len(mockMgr.spawnedWorkers))
}

func TestManager_MonitorHealth_UnhealthyReplacement(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &MockWorkerManager{}
	manager := pool.NewManager(mockRepo, mockMgr)

	cfg := pool.ManagerConfig{
		Replicas:    2,
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

	// Get the first worker and mark it as unhealthy
	pools, _ := manager.GetPool("test:worker")
	workers := pools.GetWorkers()
	require.Equal(t, 2, len(workers))

	// Mark worker as unhealthy
	workers[0].State = worker.StateUnhealthy

	// MonitorHealth should replace the unhealthy worker
	manager.MonitorHealth(context.Background())

	// Should have spawned 1 new worker to replace the unhealthy one
	// (2 initial + 1 from ensureReplicas)
	require.Equal(t, 3, len(mockMgr.spawnedWorkers))
}

func TestManager_GetPool_NotFound(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &MockWorkerManager{}
	manager := pool.NewManager(mockRepo, mockMgr)

	_, ok := manager.GetPool("nonexistent")
	require.False(t, ok)
}

func TestManager_DefaultStrategy(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &MockWorkerManager{}
	manager := pool.NewManager(mockRepo, mockMgr)

	// Register without specifying strategy
	manager.RegisterPool("test:worker", pool.ManagerConfig{
		Replicas: 1,
	})

	poolObj, ok := manager.GetPool("test:worker")
	require.True(t, ok)
	require.Equal(t, pool.RoundRobin, poolObj.Strategy())
}

func TestManager_DefaultReplicas(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &MockWorkerManager{}
	manager := pool.NewManager(mockRepo, mockMgr)

	// Register without specifying replicas
	manager.RegisterPool("test:worker", pool.ManagerConfig{
		Strategy: pool.RoundRobin,
	})

	poolObj, ok := manager.GetPool("test:worker")
	require.True(t, ok)
	// RegisterPool registers the pool configuration, it does not spawn workers.
	require.Equal(t, 0, poolObj.Size())

	// After spawning with default replicas=1 config, pool should have 1 worker.
	workerCfg := &worker.Worker{
		ID:      "test:worker",
		Command: "echo test",
		State:   worker.StateRunning,
	}
	err := manager.SpawnWorkers(context.Background(), "test:worker", workerCfg)
	require.NoError(t, err)
	require.Equal(t, 1, poolObj.Size())
}

func TestManager_ReplaceWorker_NotFound(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &MockWorkerManager{}
	manager := pool.NewManager(mockRepo, mockMgr)

	err := manager.ReplaceWorker(context.Background(), "nonexistent-worker")
	require.Error(t, err)
}

func TestManager_StopAll(t *testing.T) {
	mockRepo := &MockWorkerRepository{}
	mockMgr := &MockWorkerManager{}
	manager := pool.NewManager(mockRepo, mockMgr)

	cfg := pool.ManagerConfig{
		Replicas:    2,
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

	err = manager.StopAll(context.Background())
	require.NoError(t, err)

	// Should have stopped 2 workers
	require.Equal(t, 2, len(mockMgr.stoppedWorkers))
}
