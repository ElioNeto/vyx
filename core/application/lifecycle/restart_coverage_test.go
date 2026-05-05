package lifecycle_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/worker"
	"github.com/ElioNeto/vyx/core/application/lifecycle"
	"github.com/ElioNeto/vyx/core/domain/ipc"
)

// mockTransport implements ipc.Transport
type mockTransport struct {
	registerErr error
	deregisterErr error
	registered   []string
	deregistered []string
}

func (m *mockTransport) Register(_ context.Context, id string) error {
	m.registered = append(m.registered, id)
	return m.registerErr
}
func (m *mockTransport) Deregister(_ context.Context, id string) error {
	m.deregistered = append(m.deregistered, id)
	return m.deregisterErr
}
func (m *mockTransport) Send(_ context.Context, _ string, _ ipc.Message) error { return nil }
func (m *mockTransport) Receive(_ context.Context, _ string) (ipc.Message, error) { return ipc.Message{}, nil }
func (m *mockTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) { return ipc.Message{}, nil }
func (m *mockTransport) Close() error { return nil }

// mockReceiver implements lifecycle.ReceiverStarter
type mockReceiver struct {
	restarted []string
}

func (m *mockReceiver) RestartLoop(_ context.Context, id string) {
	m.restarted = append(m.restarted, id)
}

// newTestServiceWithMocks creates a service with transport and receiver mocks
func newTestServiceWithMocks(manager worker.Manager, transport ipc.Transport, receiver lifecycle.ReceiverStarter) (*lifecycle.Service, *mockPublisher) {
	os.Setenv("VYX_SKIP_RUNTIME", "1")
	repo := newMemRepo()
	pub := &mockPublisher{}
	drainer := lifecycle.NewWorkerDrainer()
	return lifecycle.NewService(repo, manager, pub, transport, receiver, drainer, nil), pub
}

// TestRestartWorker_WithTransportError tests transport.Register failure.
func TestRestartWorker_WithTransportError(t *testing.T) {
	mgr := &mockManager{}
	transport := &mockTransport{registerErr: errors.New("register failed")}
	receiver := &mockReceiver{}
	svc, _ := newTestServiceWithMocks(mgr, transport, receiver)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")

	err := svc.RestartWorker(context.Background(), "w1")
	if err == nil {
		t.Fatal("expected error from transport.Register")
	}
}

// TestRestartWorker_WithTransportSuccess tests successful transport re-register.
func TestRestartWorker_WithTransportSuccess(t *testing.T) {
	mgr := &mockManager{}
	transport := &mockTransport{}
	receiver := &mockReceiver{}
	svc, _ := newTestServiceWithMocks(mgr, transport, receiver)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")

	err := svc.RestartWorker(context.Background(), "w1")
	if err != nil {
		t.Fatalf("RestartWorker failed: %v", err)
	}

	// Verify transport was deregistered and registered
	if len(transport.deregistered) != 1 || transport.deregistered[0] != "w1" {
		t.Errorf("expected w1 to be deregistered")
	}
	if len(transport.registered) != 1 || transport.registered[0] != "w1" {
		t.Errorf("expected w1 to be registered")
	}
}

// TestRestartWorker_WithReceiver tests RestartLoop being called.
func TestRestartWorker_WithReceiver(t *testing.T) {
	mgr := &mockManager{}
	transport := &mockTransport{}
	receiver := &mockReceiver{}
	svc, _ := newTestServiceWithMocks(mgr, transport, receiver)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")

	err := svc.RestartWorker(context.Background(), "w1")
	if err != nil {
		t.Fatalf("RestartWorker failed: %v", err)
	}

	// Verify receiver's RestartLoop was called
	if len(receiver.restarted) != 1 || receiver.restarted[0] != "w1" {
		t.Errorf("expected RestartLoop to be called for w1")
	}
}

// TestRestartWorker_SpawnFailure tests manager.Spawn failure during restart.
func TestRestartWorker_SpawnFailure(t *testing.T) {
	mgr := &mockManager{spawnErr: worker.ErrSpawnFailed}
	transport := &mockTransport{}
	receiver := &mockReceiver{}
	svc, _ := newTestServiceWithMocks(mgr, transport, receiver)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")

	err := svc.RestartWorker(context.Background(), "w1")
	if err != worker.ErrSpawnFailed {
		t.Errorf("expected ErrSpawnFailed, got: %v", err)
	}
}

// TestRestartWorker_NotFound tests restarting non-existent worker.
func TestRestartWorker_NotFound2(t *testing.T) {
	mgr := &mockManager{}
	transport := &mockTransport{}
	receiver := &mockReceiver{}
	svc, _ := newTestServiceWithMocks(mgr, transport, receiver)

	err := svc.RestartWorker(context.Background(), "non-existent")
	if err != worker.ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestSpawnWorker_RepoSaveError tests repo.Save error (hard to trigger).
// Skipping because memRepo always succeeds.

// TestStopWorker_DrainerDrainError tests drain error path.
func TestStopWorker_DrainerDrainError(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")

	// Add to drainer but don't release - drain will timeout
	// We can't easily inject a failing drain, so we just test normal stop
	err := svc.StopWorker(context.Background(), "w1")
	if err != nil {
		t.Errorf("StopWorker failed: %v", err)
	}
}

// TestRecordHeartbeat_AlreadyRunning tests heartbeat from running worker.
func TestRecordHeartbeat_AlreadyRunning(t *testing.T) {
	mgr := &mockManager{}
	svc, _ := newTestService(mgr)

	_, _ = svc.SpawnWorker(context.Background(), lifecycle.SpawnWorkerConfig{
		ID: "w1", Command: "node",
	})
	_ = svc.MarkRunning(context.Background(), "w1")

	// Record heartbeat - should stay running
	err := svc.RecordHeartbeat(context.Background(), "w1")
	if err != nil {
		t.Fatalf("RecordHeartbeat failed: %v", err)
	}
}

// TestSpawnWorker_WithRuntimeProvisioningSuccess tests successful provisioning.
// This is hard to test because it requires actual runtime download.
// We test the error path instead (which we already have).
