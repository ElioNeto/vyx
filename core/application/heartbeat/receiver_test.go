package heartbeat_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ElioNeto/vyx/core/application/heartbeat"
	"github.com/ElioNeto/vyx/core/domain/ipc"
)

// ─── fakes ───────────────────────────────────────────────────────────────────

type fakeTransport struct {
	mu   sync.Mutex
	msgs map[string][]ipc.Message // workerID → queued messages
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{msgs: make(map[string][]ipc.Message)}
}

func (f *fakeTransport) Send(_ context.Context, workerID string, msg ipc.Message) error {
	f.mu.Lock()
	f.msgs[workerID] = append(f.msgs[workerID], msg)
	f.mu.Unlock()
	return nil
}

func (f *fakeTransport) Receive(ctx context.Context, workerID string) (ipc.Message, error) {
	for {
		f.mu.Lock()
		if len(f.msgs[workerID]) > 0 {
			msg := f.msgs[workerID][0]
			f.msgs[workerID] = f.msgs[workerID][1:]
			f.mu.Unlock()
			return msg, nil
		}
		f.mu.Unlock()

		select {
		case <-ctx.Done():
			return ipc.Message{}, ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (f *fakeTransport) ReceiveResponse(ctx context.Context, workerID string) (ipc.Message, error) {
	return f.Receive(ctx, workerID)
}
func (f *fakeTransport) Register(_ context.Context, _ string) error  { return nil }
func (f *fakeTransport) Deregister(_ context.Context, _ string) error { return nil }
func (f *fakeTransport) Close() error                                  { return nil }

// ─── fakes for WorkerLister and LifecycleService ─────────────────────────────

type fakeLister struct {
	mu  sync.Mutex
	ids []string
}

func (f *fakeLister) LiveWorkerIDs(_ context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]string, len(f.ids))
	copy(result, f.ids)
	return result, nil
}

type fakeService struct {
	mu         sync.Mutex
	heartbeats []string
	unhealthy  []string
}

func (f *fakeService) RecordHeartbeat(_ context.Context, workerID string) error {
	f.mu.Lock()
	f.heartbeats = append(f.heartbeats, workerID)
	f.mu.Unlock()
	return nil
}

func (f *fakeService) MarkUnhealthy(_ context.Context, workerID string) error {
	f.mu.Lock()
	f.unhealthy = append(f.unhealthy, workerID)
	f.mu.Unlock()
	return nil
}

func (f *fakeService) MarkRunning(_ context.Context, _ string) error {
	return nil
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestReceiver_StartLoop_RecordsHeartbeat(t *testing.T) {
	transport := newFakeTransport()
	lister := &fakeLister{ids: []string{"w1"}}
	svc := &fakeService{}
	log := zap.NewNop()

	cfg := heartbeat.Config{Interval: 50 * time.Millisecond, MissedThreshold: 3}
	recv := heartbeat.NewReceiver(transport, lister, svc, cfg, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Pre-queue a TypeHeartbeat frame for w1.
	transport.Send(ctx, "w1", ipc.Message{Type: ipc.TypeHeartbeat})

	// Start a loop for w1 directly.
	recv.StartLoop(ctx, "w1")

	// Give the loop time to process the frame.
	time.Sleep(80 * time.Millisecond)

	svc.mu.Lock()
	got := len(svc.heartbeats)
	svc.mu.Unlock()

	if got == 0 {
		t.Fatal("expected at least one RecordHeartbeat call, got none")
	}
}

func TestReceiver_MissedBeats_MarksUnhealthy(t *testing.T) {
	transport := newFakeTransport()
	lister := &fakeLister{ids: []string{"w2"}}
	svc := &fakeService{}
	log := zap.NewNop()

	// Very short interval so the test finishes fast; threshold = 2 misses.
	cfg := heartbeat.Config{Interval: 20 * time.Millisecond, MissedThreshold: 2}
	recv := heartbeat.NewReceiver(transport, lister, svc, cfg, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Do NOT queue any frames — every read will time out (missed heartbeat).
	recv.StartLoop(ctx, "w2")

	// Wait long enough for 2 missed beats + margin.
	time.Sleep(120 * time.Millisecond)

	svc.mu.Lock()
	unhealthyCalls := len(svc.unhealthy)
	svc.mu.Unlock()

	if unhealthyCalls == 0 {
		t.Fatal("expected MarkUnhealthy to be called after missed threshold, got none")
	}
}

func TestReceiver_ContextCancellation_StopsGracefully(t *testing.T) {
	transport := newFakeTransport()
	lister := &fakeLister{ids: []string{"w3"}}
	svc := &fakeService{}
	log := zap.NewNop()

	cfg := heartbeat.Config{Interval: 100 * time.Millisecond, MissedThreshold: 5}
	recv := heartbeat.NewReceiver(transport, lister, svc, cfg, log)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		recv.Run(ctx)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// clean exit
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Receiver.Run did not return after context cancellation")
	}
}

func TestReceiver_ListerError_DoesNotPanic(t *testing.T) {
	transport := newFakeTransport()
	lister := &errorLister{}
	svc := &fakeService{}
	log := zap.NewNop()

	cfg := heartbeat.Config{Interval: 20 * time.Millisecond, MissedThreshold: 2}
	recv := heartbeat.NewReceiver(transport, lister, svc, cfg, log)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	// Should not panic even if lister always errors.
	recv.Run(ctx)
}

type errorLister struct{}

func (e *errorLister) LiveWorkerIDs(_ context.Context) ([]string, error) {
	return nil, errors.New("repository unavailable")
}
