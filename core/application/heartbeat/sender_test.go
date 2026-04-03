package heartbeat_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ElioNeto/vyx/core/application/heartbeat"
	"github.com/ElioNeto/vyx/core/domain/ipc"
)

// mockLister returns a fixed list of worker IDs.
type mockLister struct {
	ids []string
}

func (m *mockLister) LiveWorkerIDs(_ context.Context) ([]string, error) {
	return m.ids, nil
}

// recordingTransport records every Send call.
type recordingTransport struct {
	mu      sync.Mutex
	sent    []ipc.Message
	workers []string
}

func (r *recordingTransport) Send(_ context.Context, workerID string, msg ipc.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, msg)
	r.workers = append(r.workers, workerID)
	return nil
}
func (r *recordingTransport) Receive(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, nil
}
func (r *recordingTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, nil
}
func (r *recordingTransport) Register(_ context.Context, _ string) error   { return nil }
func (r *recordingTransport) Deregister(_ context.Context, _ string) error { return nil }
func (r *recordingTransport) Close() error                                  { return nil }

func TestSender_SendsHeartbeatFrame(t *testing.T) {
	transport := &recordingTransport{}
	lister := &mockLister{ids: []string{"worker-1", "worker-2"}}

	cfg := heartbeat.Config{Interval: 30 * time.Millisecond, MissedThreshold: 2}
	sender := heartbeat.NewSender(transport, lister, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	sender.Run(ctx)

	transport.mu.Lock()
	defer transport.mu.Unlock()

	if len(transport.sent) < 2 {
		t.Fatalf("expected at least 2 sends (one per worker per tick), got %d", len(transport.sent))
	}
	for i, msg := range transport.sent {
		if msg.Type != ipc.TypeHeartbeat {
			t.Errorf("send[%d]: expected TypeHeartbeat (0x03), got 0x%02x", i, msg.Type)
		}
	}
}

func TestSender_SendsToAllWorkers(t *testing.T) {
	transport := &recordingTransport{}
	lister := &mockLister{ids: []string{"node:api", "go:billing", "python:ml"}}

	cfg := heartbeat.Config{Interval: 20 * time.Millisecond, MissedThreshold: 2}
	sender := heartbeat.NewSender(transport, lister, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	sender.Run(ctx)

	transport.mu.Lock()
	defer transport.mu.Unlock()

	seen := map[string]bool{}
	for _, id := range transport.workers {
		seen[id] = true
	}
	for _, expected := range []string{"node:api", "go:billing", "python:ml"} {
		if !seen[expected] {
			t.Errorf("expected heartbeat sent to %q, but it was not", expected)
		}
	}
}

func TestSender_ContextCancelled_ExitsCleanly(t *testing.T) {
	transport := &recordingTransport{}
	lister := &mockLister{ids: []string{"worker-1"}}

	cfg := heartbeat.Config{Interval: 50 * time.Millisecond, MissedThreshold: 2}
	sender := heartbeat.NewSender(transport, lister, cfg, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		sender.Run(ctx)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// clean exit
	case <-time.After(2 * time.Second):
		t.Error("Sender.Run did not exit after context cancellation")
	}
}

func TestSender_SendError_DoesNotPanic(t *testing.T) {
	// Transport that always fails on Send
	failTransport := &failingSendTransport{}
	lister := &mockLister{ids: []string{"worker-broken"}}

	cfg := heartbeat.Config{Interval: 20 * time.Millisecond, MissedThreshold: 2}
	sender := heartbeat.NewSender(failTransport, lister, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	// Must not panic
	sender.Run(ctx)
}

// failingSendTransport always returns an error on Send.
type failingSendTransport struct{}

func (f *failingSendTransport) Send(_ context.Context, _ string, _ ipc.Message) error {
	return &mockSendError{}
}
func (f *failingSendTransport) Receive(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, nil
}
func (f *failingSendTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, nil
}
func (f *failingSendTransport) Register(_ context.Context, _ string) error   { return nil }
func (f *failingSendTransport) Deregister(_ context.Context, _ string) error { return nil }
func (f *failingSendTransport) Close() error                                  { return nil }

type mockSendError struct{}

func (e *mockSendError) Error() string { return "send failed" }
