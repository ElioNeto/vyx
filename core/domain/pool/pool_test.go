// Package pool implements worker pools with load balancing strategies.
package pool

import (
	"sync"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/worker"
	"github.com/stretchr/testify/require"
)

func TestPool_RoundRobin(t *testing.T) {
	pool := NewPool(RoundRobin)

	// Add 3 workers
	w1 := &worker.Worker{ID: "w1", State: worker.StateRunning}
	w2 := &worker.Worker{ID: "w2", State: worker.StateRunning}
	w3 := &worker.Worker{ID: "w3", State: worker.StateRunning}
	pool.AddWorker(w1)
	pool.AddWorker(w2)
	pool.AddWorker(w3)

	// Test round-robin distribution
	require.Equal(t, "w1", pool.SelectWorker().ID)
	require.Equal(t, "w2", pool.SelectWorker().ID)
	require.Equal(t, "w3", pool.SelectWorker().ID)
	require.Equal(t, "w1", pool.SelectWorker().ID)
}

func TestPool_LeastLoaded(t *testing.T) {
	pool := NewPool(LeastLoaded)

	// Add workers
	w1 := &worker.Worker{ID: "w1", State: worker.StateRunning}
	w2 := &worker.Worker{ID: "w2", State: worker.StateRunning}
	pool.AddWorker(w1)
	pool.AddWorker(w2)

	// Simulate active requests
	pool.IncrementActiveReqs("w1")
	pool.IncrementActiveReqs("w1")

	// w2 should be selected as it has fewer active requests
	selected := pool.SelectWorker()
	require.Equal(t, "w2", selected.ID)

	// After decrementing w1, either could be selected (both have 1 or 0)
	pool.DecrementActiveReqs("w1")
	selected = pool.SelectWorker()
	require.Contains(t, []string{"w1", "w2"}, selected.ID)
}

func TestPool_HealthyWorkers(t *testing.T) {
	pool := NewPool(RoundRobin)

	// Add mixed health workers
	w1 := &worker.Worker{ID: "w1", State: worker.StateRunning}
	w2 := &worker.Worker{ID: "w2", State: worker.StateUnhealthy}
	w3 := &worker.Worker{ID: "w3", State: worker.StateRunning}
	pool.AddWorker(w1)
	pool.AddWorker(w2)
	pool.AddWorker(w3)

	healthy := pool.HealthyWorkers()
	if len(healthy) != 2 {
		t.Errorf("expected 2 healthy workers, got %d", len(healthy))
	}
}

func TestPool_AddRemoveWorker(t *testing.T) {
	pool := NewPool(RoundRobin)

	w := &worker.Worker{ID: "test-worker", State: worker.StateRunning}
	pool.AddWorker(w)

	if pool.Size() != 1 {
		t.Errorf("expected pool size 1, got %d", pool.Size())
	}

	pool.RemoveWorker("test-worker")
	if pool.Size() != 0 {
		t.Errorf("expected pool size 0 after removal, got %d", pool.Size())
	}
}

func TestPool_EmptyPool(t *testing.T) {
	pool := NewPool(RoundRobin)
	if pool.SelectWorker() != nil {
		t.Errorf("expected nil worker from empty pool")
	}
	if pool.HealthySize() != 0 {
		t.Errorf("expected healthy size 0, got %d", pool.HealthySize())
	}
}

func TestPool_DefaultStrategy(t *testing.T) {
	pool := NewPool("") // Empty strategy should default to RoundRobin
	if pool.Strategy() != RoundRobin {
		t.Errorf("expected default strategy RoundRobin, got %s", pool.Strategy())
	}
}

func TestPool_ActiveRequestsTracking(t *testing.T) {
	pool := NewPool(LeastLoaded)

	w := &worker.Worker{ID: "w1", State: worker.StateRunning}
	pool.AddWorker(w)

	// Check initial state
	_, reqs, ok := pool.GetWorkerState("w1")
	require.True(t, ok)
	require.Equal(t, int64(0), reqs)

	// Increment and check
	pool.IncrementActiveReqs("w1")
	_, reqs, ok = pool.GetWorkerState("w1")
	require.Equal(t, int64(1), reqs)

	// Decrement and check
	pool.DecrementActiveReqs("w1")
	_, reqs, ok = pool.GetWorkerState("w1")
	require.Equal(t, int64(0), reqs)
}

func TestPool_ConcurrentAccess(t *testing.T) {
	pool := NewPool(RoundRobin)

	// Add workers
	for i := 0; i < 10; i++ {
		w := &worker.Worker{ID: string(rune('a' + i)), State: worker.StateRunning}
		pool.AddWorker(w)
	}

	// Concurrent reads and writes
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pool.SelectWorker()
			pool.Size()
			pool.HealthySize()
		}()
	}
	wg.Wait()
}

func TestPool_RoundRobinSingleWorker(t *testing.T) {
	pool := NewPool(RoundRobin)

	w1 := &worker.Worker{ID: "w1", State: worker.StateRunning}
	pool.AddWorker(w1)

	// All selections should return the same worker
	for i := 0; i < 10; i++ {
		require.Equal(t, "w1", pool.SelectWorker().ID)
	}
}

func TestPool_LeastLoadedTieBreaker(t *testing.T) {
	pool := NewPool(LeastLoaded)

	w1 := &worker.Worker{ID: "w1", State: worker.StateRunning}
	w2 := &worker.Worker{ID: "w2", State: worker.StateRunning}
	pool.AddWorker(w1)
	pool.AddWorker(w2)

	// Both have 0 active requests, should pick one consistently
	selected := pool.SelectWorker()
	require.Contains(t, []string{"w1", "w2"}, selected.ID)
}

func TestPool_AllUnhealthy(t *testing.T) {
	pool := NewPool(RoundRobin)

	w1 := &worker.Worker{ID: "w1", State: worker.StateUnhealthy}
	w2 := &worker.Worker{ID: "w2", State: worker.StateUnhealthy}
	pool.AddWorker(w1)
	pool.AddWorker(w2)

	// Should return nil when all workers are unhealthy
	require.Nil(t, pool.SelectWorker())
	require.Equal(t, 0, pool.HealthySize())
}

func TestPool_UpdateWorker(t *testing.T) {
	pool := NewPool(RoundRobin)

	w1 := &worker.Worker{ID: "w1", State: worker.StateRunning}
	pool.AddWorker(w1)

	// Update worker state
	w1.State = worker.StateUnhealthy
	pool.UpdateWorker(w1)

	// Should not be selected as healthy
	require.Nil(t, pool.SelectWorker())
}

func TestPool_HealthySizeAccuracy(t *testing.T) {
	pool := NewPool(RoundRobin)

	w1 := &worker.Worker{ID: "w1", State: worker.StateRunning}
	w2 := &worker.Worker{ID: "w2", State: worker.StateUnhealthy}
	w3 := &worker.Worker{ID: "w3", State: worker.StateRunning}
	pool.AddWorker(w1)
	pool.AddWorker(w2)
	pool.AddWorker(w3)

	require.Equal(t, 2, pool.HealthySize())
	require.Equal(t, 3, pool.Size())
}

func TestPool_StrategyChange(t *testing.T) {
	pool := NewPool(RoundRobin)

	require.Equal(t, RoundRobin, pool.Strategy())

	pool2 := NewPool(LeastLoaded)
	require.Equal(t, LeastLoaded, pool2.Strategy())
}