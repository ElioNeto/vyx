package lifecycle_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/worker"
	"github.com/ElioNeto/vyx/core/application/lifecycle"
)

// --- Mocks adicionais para cobrir paths de erro ---

// errorRepo é um repositório que sempre retorna erro
type errorRepo struct {
	err error
}

func (r *errorRepo) Save(_ context.Context, _ *worker.Worker) error {
	return r.err
}
func (r *errorRepo) FindByID(_ context.Context, _ string) (*worker.Worker, error) {
	return nil, r.err
}
func (r *errorRepo) FindAll(_ context.Context) ([]*worker.Worker, error) {
	return nil, r.err
}
func (r *errorRepo) Delete(_ context.Context, _ string) error {
	return r.err
}

// partialErrorRepo retorna erro apenas em FindByID
type partialErrorRepo struct {
	*memRepo
	findByIDErr error
}

func (r *partialErrorRepo) FindByID(_ context.Context, id string) (*worker.Worker, error) {
	return nil, r.findByIDErr
}

// newTestServiceWithRepo cria serviço com repo customizado
func newTestServiceWithRepo(repo worker.Repository, manager worker.Manager) (*lifecycle.Service, *mockPublisher) {
	os.Setenv("VYX_SKIP_RUNTIME", "1")
	pub := &mockPublisher{}
	drainer := lifecycle.NewWorkerDrainer()
	return lifecycle.NewService(repo, manager, pub, nil, nil, drainer), pub
}

// TestSpawnWorker_RepoSaveError testa erro em repo.Save
func TestSpawnWorker_RepoSaveError(t *testing.T) {
	mgr := &mockManager{}
	repo := &errorRepo{err: errors.New("save failed")}
	svc, _ := newTestServiceWithRepo(repo, mgr)

	_, err := svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})

	if err == nil {
		t.Fatal("expected error from repo.Save")
	}
}

// TestStopWorker_FindByIDError testa erro em FindByID
func TestStopWorker_FindByIDError(t *testing.T) {
	mgr := &mockManager{}
	repo := &errorRepo{err: errors.New("find failed")}
	svc, _ := newTestServiceWithRepo(repo, mgr)

	err := svc.StopWorker(context.Background(), "w1")
	if err == nil {
		t.Fatal("expected error from repo.FindByID")
	}
}

// TestRecordHeartbeat_FindByIDError testa erro em FindByID
func TestRecordHeartbeat_FindByIDError(t *testing.T) {
	mgr := &mockManager{}
	repo := &errorRepo{err: errors.New("find failed")}
	svc, _ := newTestServiceWithRepo(repo, mgr)

	err := svc.RecordHeartbeat(context.Background(), "w1")
	if err == nil {
		t.Fatal("expected error from repo.FindByID")
	}
}

// TestMarkUnhealthy_FindByIDError testa erro em FindByID
func TestMarkUnhealthy_FindByIDError(t *testing.T) {
	mgr := &mockManager{}
	repo := &errorRepo{err: errors.New("find failed")}
	svc, _ := newTestServiceWithRepo(repo, mgr)

	err := svc.MarkUnhealthy(context.Background(), "w1")
	if err == nil {
		t.Fatal("expected error from repo.FindByID")
	}
}

// TestMarkRunning_FindByIDError testa erro em FindByID
func TestMarkRunning_FindByIDError(t *testing.T) {
	mgr := &mockManager{}
	repo := &errorRepo{err: errors.New("find failed")}
	svc, _ := newTestServiceWithRepo(repo, mgr)

	err := svc.MarkRunning(context.Background(), "w1")
	if err == nil {
		t.Fatal("expected error from repo.FindByID")
	}
}

// TestRestartWorker_FindByIDError testa erro em FindByID
func TestRestartWorker_FindByIDError(t *testing.T) {
	mgr := &mockManager{}
	repo := &errorRepo{err: errors.New("find failed")}
	svc, _ := newTestServiceWithRepo(repo, mgr)

	err := svc.RestartWorker(context.Background(), "w1")
	if err == nil {
		t.Fatal("expected error from repo.FindByID")
	}
}

// TestStopAll_FindAllError testa erro em FindAll
func TestStopAll_FindAllError(t *testing.T) {
	mgr := &mockManager{}
	repo := &errorRepo{err: errors.New("find all failed")}
	svc, _ := newTestServiceWithRepo(repo, mgr)

	err := svc.StopAll(context.Background())
	if err == nil {
		t.Fatal("expected error from repo.FindAll")
	}
}

// TestMarkRunning_AlreadyRunning2 verifica no-op quando já está rodando
func TestMarkRunning_AlreadyRunning2(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	// Spawn e marca como running
	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")

	// Marcar como running novamente deve ser no-op
	err := svc.MarkRunning(context.Background(), "w1")
	if err != nil {
		t.Errorf("MarkRunning should be no-op when already running: %v", err)
	}
}

// TestStopWorker_ManagerStopError testa erro em manager.Stop
func TestStopWorker_ManagerStopError(t *testing.T) {
	mgr := &mockManager{stopErr: errors.New("stop failed")}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")

	err := svc.StopWorker(context.Background(), "w1")
	if err == nil {
		t.Fatal("expected error from manager.Stop")
	}
}
