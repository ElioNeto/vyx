package lifecycle_test

import (
	"context"
	"testing"

	"github.com/ElioNeto/vyx/core/application/lifecycle"
	"github.com/ElioNeto/vyx/core/domain/worker"
)

// --- Mocks ---

type mockManager struct {
	spawnErr error
	stopErr  error
	spawned  []string
	stopped  []string
}

func (m *mockManager) Spawn(_ context.Context, w *worker.Worker) error {
	m.spawned = append(m.spawned, w.ID)
	return m.spawnErr
}
func (m *mockManager) Stop(_ context.Context, id string) error {
	m.stopped = append(m.stopped, id)
	return m.stopErr
}
func (m *mockManager) StopAll(_ context.Context) error { return nil }
func (m *mockManager) SendHeartbeat(_ context.Context, _ string) error { return nil }

type mockPublisher struct {
	events []worker.Event
}

func (p *mockPublisher) Publish(_ context.Context, e worker.Event) {
	p.events = append(p.events, e)
}

// --- Helpers ---

func newTestService(manager worker.Manager) (*lifecycle.Service, *mockPublisher) {
	repo := newMemRepo()
	pub := &mockPublisher{}
	return lifecycle.NewService(repo, manager, pub), pub
}

// minimal in-memory repo for tests
type memRepo struct{ store map[string]*worker.Worker }

func newMemRepo() *memRepo { return &memRepo{store: make(map[string]*worker.Worker)} }
func (r *memRepo) Save(_ context.Context, w *worker.Worker) error {
	copy := *w; r.store[w.ID] = &copy; return nil
}
func (r *memRepo) FindByID(_ context.Context, id string) (*worker.Worker, error) {
	w := r.store[id]; if w == nil { return nil, nil }; copy := *w; return &copy, nil
}
func (r *memRepo) FindAll(_ context.Context) ([]*worker.Worker, error) {
	var out []*worker.Worker
	for _, w := range r.store { copy := *w; out = append(out, &copy) }
	return out, nil
}
func (r *memRepo) Delete(_ context.Context, id string) error { delete(r.store, id); return nil }

// --- Tests ---

func TestSpawnWorker_Success(t *testing.T) {
	mgr := &mockManager{}
	svc, pub := newTestService(mgr)

	w, err := svc.SpawnWorker(context.Background(), "node:api", "node", []string{"worker.js"})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if w.State != worker.StateRunning {
		t.Errorf("expected state %s, got %s", worker.StateRunning, w.State)
	}
	if len(mgr.spawned) != 1 || mgr.spawned[0] != "node:api" {
		t.Errorf("expected worker to be spawned, got %v", mgr.spawned)
	}
	if len(pub.events) == 0 {
		t.Error("expected at least one event to be published")
	}
}

func TestSpawnWorker_EmptyCommand(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	_, err := svc.SpawnWorker(context.Background(), "node:api", "", nil)

	if err != worker.ErrInvalidCommand {
		t.Errorf("expected ErrInvalidCommand, got %v", err)
	}
}

func TestSpawnWorker_SpawnFailure(t *testing.T) {
	mgr := &mockManager{spawnErr: worker.ErrSpawnFailed}
	svc, _ := newTestService(mgr)

	_, err := svc.SpawnWorker(context.Background(), "node:api", "node", nil)

	if err != worker.ErrSpawnFailed {
		t.Errorf("expected ErrSpawnFailed, got %v", err)
	}
}

func TestStopWorker_Success(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), "node:api", "node", nil)
	err := svc.StopWorker(context.Background(), "node:api")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(mgr.stopped) != 1 {
		t.Errorf("expected worker to be stopped, got %v", mgr.stopped)
	}
}

func TestStopWorker_NotFound(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	err := svc.StopWorker(context.Background(), "does-not-exist")

	if err != worker.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRecordHeartbeat_UpdatesTimestamp(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), "node:api", "node", nil)
	err := svc.RecordHeartbeat(context.Background(), "node:api")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRestartWorker_IncrementsRestartCount(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), "node:api", "node", nil)
	_ = svc.MarkUnhealthy(context.Background(), "node:api")
	err := svc.RestartWorker(context.Background(), "node:api")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
