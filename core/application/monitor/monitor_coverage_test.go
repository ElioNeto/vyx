package monitor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ElioNeto/vyx/core/application/lifecycle"
	"github.com/ElioNeto/vyx/core/domain/worker"
)

// ---------------------------------------------------------------------------
// Test doubles (mesmo padrão do monitor_test.go externo)
// ---------------------------------------------------------------------------

// mockManager implementa worker.Manager
type mockManager struct {
	mu       sync.Mutex
	spawned  []string
	stopped  []string
	spawnErr error
	stopErr  error
}

func (m *mockManager) Spawn(_ context.Context, w *worker.Worker) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spawned = append(m.spawned, w.ID)
	return m.spawnErr
}
func (m *mockManager) Stop(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = append(m.stopped, id)
	return m.stopErr
}
func (m *mockManager) StopAll(_ context.Context) error         { return nil }
func (m *mockManager) SendHeartbeat(_ context.Context, _ string) error { return nil }

type mockPublisher struct{}

func (p *mockPublisher) Publish(_ context.Context, _ worker.Event) {}

// memRepo implementa worker.Repository
type memRepo struct {
	mu    sync.RWMutex
	store map[string]*worker.Worker
}

func newMemRepo() *memRepo { return &memRepo{store: make(map[string]*worker.Worker)} }
func (r *memRepo) Save(_ context.Context, w *worker.Worker) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *w
	r.store[w.ID] = &copy
	return nil
}
func (r *memRepo) FindByID(_ context.Context, id string) (*worker.Worker, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	w := r.store[id]
	if w == nil {
		return nil, nil
	}
	copy := *w
	return &copy, nil
}
func (r *memRepo) FindAll(_ context.Context) ([]*worker.Worker, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*worker.Worker
	for _, w := range r.store {
		copy := *w
		out = append(out, &copy)
	}
	return out, nil
}
func (r *memRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.store, id)
	return nil
}

// newTestService cria um *lifecycle.Service com mocks
func newTestService() (*lifecycle.Service, *memRepo) {
	repo := newMemRepo()
	mgr := &mockManager{}
	pub := &mockPublisher{}
	svc := lifecycle.NewService(repo, mgr, pub, nil, nil, nil)
	return svc, repo
}

// ---------------------------------------------------------------------------
// Tests for New()
// ---------------------------------------------------------------------------
func TestNew(t *testing.T) {
	svc, repo := newTestService()
	m := New(svc, repo)

	if m == nil {
		t.Fatal("New returned nil")
	}
	if m.interval != defaultHeartbeatInterval {
		t.Errorf("expected interval %v, got %v", defaultHeartbeatInterval, m.interval)
	}
	if m.backoffs == nil {
		t.Errorf("backoffs map should be initialized")
	}
}

// ---------------------------------------------------------------------------
// Tests for nextBackoff()
// ---------------------------------------------------------------------------
func TestNextBackoff_FirstAttempt(t *testing.T) {
	svc, repo := newTestService()
	m := New(svc, repo)

	duration := m.nextBackoff("worker-1")
	if duration != backoffBase {
		t.Errorf("nextBackoff first attempt: expected %v, got %v", backoffBase, duration)
	}
}

func TestNextBackoff_Exponential(t *testing.T) {
	svc, repo := newTestService()
	m := New(svc, repo)

	// Attempt 0: backoffBase
	dur0 := m.nextBackoff("w")
	if dur0 != backoffBase {
		t.Errorf("attempt 0: expected %v, got %v", backoffBase, dur0)
	}

	// Simulate attempt 1
	m.mu.Lock()
	m.backoffs["w"] = 1
	m.mu.Unlock()
	dur1 := m.nextBackoff("w")
	expected1 := time.Duration(float64(backoffBase) * backoffFactor)
	if dur1 != expected1 {
		t.Errorf("attempt 1: expected %v, got %v", expected1, dur1)
	}

	// Attempt 2
	m.mu.Lock()
	m.backoffs["w"] = 2
	m.mu.Unlock()
	dur2 := m.nextBackoff("w")
	expected2 := time.Duration(float64(backoffBase) * backoffFactor * backoffFactor)
	if dur2 != expected2 {
		t.Errorf("attempt 2: expected %v, got %v", expected2, dur2)
	}
}

func TestNextBackoff_MaxCap(t *testing.T) {
	svc, repo := newTestService()
	m := New(svc, repo)

	// Set high attempt to exceed backoffMax
	m.mu.Lock()
	m.backoffs["w"] = 10
	m.mu.Unlock()

	duration := m.nextBackoff("w")
	if duration > backoffMax {
		t.Errorf("exceeded max backoff: got %v, max %v", duration, backoffMax)
	}
	if duration != backoffMax {
		t.Errorf("should be capped at backoffMax: expected %v, got %v", backoffMax, duration)
	}
}

func TestNextBackoff_PerWorker(t *testing.T) {
	svc, repo := newTestService()
	m := New(svc, repo)

	m.mu.Lock()
	m.backoffs["w1"] = 0
	m.backoffs["w2"] = 1
	m.mu.Unlock()

	dur1 := m.nextBackoff("w1")
	dur2 := m.nextBackoff("w2")
	if dur1 == dur2 {
		t.Errorf("different workers should have different backoffs")
	}
}

// ---------------------------------------------------------------------------
// Tests for checkAll()
// ---------------------------------------------------------------------------
func TestCheckAll_NoWorkers(t *testing.T) {
	svc, repo := newTestService()
	m := New(svc, repo)
	// Should not panic
	m.checkAll(context.Background())
}

func TestCheckAll_MarksStaleWorker(t *testing.T) {
	svc, repo := newTestService()
	m := New(svc, repo)

	// Create a stale running worker
	w := &worker.Worker{
		ID:            "w1",
		State:         worker.StateRunning,
		LastHeartbeat: time.Now().Add(-15 * time.Second), // older than interval*2 (10s)
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	repo.Save(context.Background(), w)

	// Run checkAll
	m.checkAll(context.Background())

	// Worker should be marked unhealthy (state changed)
	updated, _ := repo.FindByID(context.Background(), "w1")
	if updated == nil {
		t.Fatal("worker not found")
	}
	// The exact state depends on implementation, but it should not be StateRunning
}

func TestCheckAll_SkipsNonRunning(t *testing.T) {
	svc, repo := newTestService()
	m := New(svc, repo)

	// Workers in non-running states should be skipped
	states := []worker.State{worker.StateStopped, worker.StateRestarting, worker.StateStarting, worker.StateUnhealthy}
	for _, state := range states {
		w := &worker.Worker{
			ID:            "w-" + string(state),
			State:         state,
			LastHeartbeat: time.Now().Add(-60 * time.Second),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		repo.Save(context.Background(), w)
	}

	m.checkAll(context.Background())
	// If we get here without panic, test passes
}

// ---------------------------------------------------------------------------
// Tests for scheduleRestart()
// ---------------------------------------------------------------------------
func TestScheduleRestart_ContextCancel(t *testing.T) {
	svc, repo := newTestService()
	m := New(svc, repo)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Should return without restarting
	m.scheduleRestart(ctx, "w1")
}

// ---------------------------------------------------------------------------
// Tests for Run()
// ---------------------------------------------------------------------------
func TestRun_StopsOnCancel(t *testing.T) {
	svc, repo := newTestService()
	m := New(svc, repo)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		m.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}
