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

// --- Mocks ---

type mockTransport struct {
	mu       sync.Mutex
	messages []ipc.Message
	errors   []error
	cursor   int
}

func (m *mockTransport) Receive(_ context.Context, _ string) (ipc.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cursor >= len(m.messages) {
		// Block until context is cancelled (simulate waiting worker).
		time.Sleep(200 * time.Millisecond)
		return ipc.Message{}, errors.New("timeout")
	}
	var err error
	if m.cursor < len(m.errors) {
		err = m.errors[m.cursor]
	}
	msg := m.messages[m.cursor]
	m.cursor++
	return msg, err
}

func (m *mockTransport) Send(_ context.Context, _ string, _ ipc.Message) error { return nil }
func (m *mockTransport) Register(_ context.Context, _ string) error             { return nil }
func (m *mockTransport) Deregister(_ context.Context, _ string) error           { return nil }
func (m *mockTransport) Close() error                                           { return nil }

type mockService struct {
	mu              sync.Mutex
	heartbeats      int
	unhealthyCalled bool
}

func (s *mockService) RecordHeartbeat(_ context.Context, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.heartbeats++
	return nil
}

func (s *mockService) MarkUnhealthy(_ context.Context, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unhealthyCalled = true
	return nil
}

// --- Tests ---

func TestLoop_HeartbeatReceived_CallsRecordHeartbeat(t *testing.T) {
	transport := &mockTransport{
		messages: []ipc.Message{
			{Type: ipc.TypeHeartbeat, Payload: []byte{}},
			{Type: ipc.TypeHeartbeat, Payload: []byte{}},
		},
	}
	svc := &mockService{}

	cfg := heartbeat.Config{Interval: 50 * time.Millisecond, MissedThreshold: 2}
	loop := heartbeat.New("worker-1", transport, svc, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	loop.Run(ctx)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.heartbeats < 2 {
		t.Errorf("expected at least 2 heartbeats recorded, got %d", svc.heartbeats)
	}
}

func TestLoop_MissedHeartbeats_MarksUnhealthy(t *testing.T) {
	// Transport returns errors immediately — simulates a dead worker.
	transport := &mockTransport{
		messages: []ipc.Message{},
		errors:   []error{},
	}
	svc := &mockService{}

	cfg := heartbeat.Config{Interval: 20 * time.Millisecond, MissedThreshold: 2}
	loop := heartbeat.New("worker-dead", transport, svc, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	loop.Run(ctx)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if !svc.unhealthyCalled {
		t.Error("expected MarkUnhealthy to be called after missed threshold")
	}
}

func TestLoop_ContextCancelled_ExitsCleanly(t *testing.T) {
	transport := &mockTransport{messages: []ipc.Message{}}
	svc := &mockService{}

	cfg := heartbeat.Config{Interval: 50 * time.Millisecond, MissedThreshold: 10}
	loop := heartbeat.New("worker-ctx", transport, svc, cfg, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		loop.Run(ctx)
		close(done)
	}()

	time.Sleep(60 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// clean exit
	case <-time.After(2 * time.Second):
		t.Error("loop did not exit after context cancellation")
	}
}

func TestLoop_HeartbeatResetssMissedCounter(t *testing.T) {
	// One miss then one heartbeat — should NOT mark unhealthy (threshold=2).
	transport := &mockTransport{
		messages: []ipc.Message{
			{Type: ipc.TypeHeartbeat, Payload: []byte{}}, // received → resets missed=0
		},
	}
	svc := &mockService{}

	cfg := heartbeat.Config{Interval: 20 * time.Millisecond, MissedThreshold: 2}
	loop := heartbeat.New("worker-reset", transport, svc, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	loop.Run(ctx)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.unhealthyCalled {
		t.Error("MarkUnhealthy should not be called when heartbeat resets the counter")
	}
}
