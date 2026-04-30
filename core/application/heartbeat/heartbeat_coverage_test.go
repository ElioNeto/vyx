package heartbeat

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/domain/worker"
)

// Mock types for coverage tests (package heartbeat internal tests)
type mockLifecycleService struct {
	mu            sync.Mutex
	heartbeats    int
	unhealthy     bool
	running       bool
	lastWorkerID  string
}

func (s *mockLifecycleService) RecordHeartbeat(_ context.Context, workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.heartbeats++
	s.lastWorkerID = workerID
	return nil
}

func (s *mockLifecycleService) MarkUnhealthy(_ context.Context, workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unhealthy = true
	s.lastWorkerID = workerID
	return nil
}

func (s *mockLifecycleService) MarkRunning(_ context.Context, workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = true
	s.lastWorkerID = workerID
	return nil
}

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
		time.Sleep(10 * time.Millisecond)
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
func (m *mockTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, nil
}
func (m *mockTransport) Register(_ context.Context, _ string) error  { return nil }
func (m *mockTransport) Deregister(_ context.Context, _ string) error { return nil }
func (m *mockTransport) Close() error                             { return nil }

// Test isNotConnectedError directly
func TestIsNotConnectedError_Direct(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"exact match", errors.New("ipc: worker is not connected"), true},
		{"wrapped with context", errors.New("ipc: worker is not connected: context canceled"), true},
		{"different error", errors.New("some other error"), false},
		{"empty error", errors.New(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExportIsNotConnectedError(tt.err)
			if result != tt.expected {
				t.Errorf("isNotConnectedError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

// Test handleReceiveError during grace period with not-connected error
func TestHandleReceiveError_GracePeriod_NotConnected(t *testing.T) {
	svc := &mockLifecycleService{}
	cfg := Config{
		Interval:        20 * time.Millisecond,
		MissedThreshold: 2,
		ConnectGrace:   200 * time.Millisecond,
		RetryInterval:   10 * time.Millisecond,
	}
	loop := New("w1", &mockTransport{}, svc, cfg, zap.NewNop())

	startTime := time.Now()
	missed := loop.ExportHandleReceiveError(context.Background(), 0, startTime, errors.New("ipc: worker is not connected"))

	if missed != 0 {
		t.Errorf("expected missed to remain 0 during grace period, got %d", missed)
	}

	svc.mu.Lock()
	unhealthy := svc.unhealthy
	svc.mu.Unlock()

	if unhealthy {
		t.Error("should not mark unhealthy during grace period")
	}
}

// Test handleReceiveError after grace period
func TestHandleReceiveError_AfterGracePeriod(t *testing.T) {
	svc := &mockLifecycleService{}
	cfg := Config{
		Interval:        20 * time.Millisecond,
		MissedThreshold: 2,
		ConnectGrace:   0, // No grace period
	}
	loop := New("w1", &mockTransport{}, svc, cfg, zap.NewNop())

	startTime := time.Now().Add(-time.Hour) // Well past grace period
	// With missed=1 and threshold=2, after increment it will be 2 which equals threshold
	// So it should return -1 (unhealthy)
	missed := loop.ExportHandleReceiveError(context.Background(), 1, startTime, errors.New("ipc: worker is not connected"))

	if missed != -1 {
		t.Errorf("expected -1 when threshold exceeded, got %d", missed)
	}

	svc.mu.Lock()
	unhealthy := svc.unhealthy
	svc.mu.Unlock()

	if !unhealthy {
		t.Error("should mark unhealthy when threshold exceeded")
	}
}

// Test handleReceiveError exceeds threshold
func TestHandleReceiveError_ThresholdExceeded(t *testing.T) {
	svc := &mockLifecycleService{}
	cfg := Config{
		Interval:        20 * time.Millisecond,
		MissedThreshold: 2,
		ConnectGrace:   0,
	}
	loop := New("w1", &mockTransport{}, svc, cfg, zap.NewNop())

	startTime := time.Now().Add(-time.Hour)
	missed := loop.ExportHandleReceiveError(context.Background(), 2, startTime, errors.New("connection lost"))

	if missed != -1 {
		t.Errorf("expected -1 when threshold exceeded, got %d", missed)
	}

	svc.mu.Lock()
	unhealthy := svc.unhealthy
	svc.mu.Unlock()

	if !unhealthy {
		t.Error("should mark unhealthy when threshold exceeded")
	}
}

// Test handleReceiveError context cancelled
func TestHandleReceiveError_ContextCancelled(t *testing.T) {
	svc := &mockLifecycleService{}
	cfg := DefaultConfig()
	loop := New("w1", &mockTransport{}, svc, cfg, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	startTime := time.Now()
	missed := loop.ExportHandleReceiveError(ctx, 0, startTime, errors.New("some error"))

	if missed != -1 {
		t.Errorf("expected -1 when context cancelled, got %d", missed)
	}
}

// Test handleMessage with all message types
func TestHandleMessage_AllTypes(t *testing.T) {
	tests := []struct {
		name       string
		msg        ipc.Message
		expectRun  bool
		expectHB   bool
	}{
		{
			name:     "TypeHandshake marks running",
			msg:      ipc.Message{Type: ipc.TypeHandshake, Payload: []byte(`{"worker_id":"w1"}`)},
			expectRun: true,
			expectHB:  false,
		},
		{
			name:     "TypeHeartbeat records heartbeat",
			msg:      ipc.Message{Type: ipc.TypeHeartbeat, Payload: []byte{}},
			expectRun: false,
			expectHB:  true,
		},
		{
			name:     "TypeResponse ignored",
			msg:      ipc.Message{Type: ipc.TypeResponse, Payload: []byte(`{"body":"ok"}`)},
			expectRun: false,
			expectHB:  false,
		},
		{
			name:     "TypeError ignored",
			msg:      ipc.Message{Type: ipc.TypeError, Payload: []byte("error")},
			expectRun: false,
			expectHB:  false,
		},
		{
			name:     "Unknown type logs warning",
			msg:      ipc.Message{Type: ipc.MessageType(0x99), Payload: []byte("unknown")},
			expectRun: false,
			expectHB:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockLifecycleService{}
			cfg := DefaultConfig()
			loop := New("w1", &mockTransport{}, svc, cfg, zap.NewNop())

			loop.ExportHandleMessage(context.Background(), tt.msg)

			svc.mu.Lock()
			running := svc.running
			heartbeats := svc.heartbeats
			svc.mu.Unlock()

			if tt.expectRun && !running {
				t.Error("expected MarkRunning to be called")
			}
			if tt.expectHB && heartbeats == 0 {
				t.Error("expected RecordHeartbeat to be called")
			}
		})
	}
}

// Test handleMessage with RecordHeartbeat error
func TestHandleMessage_RecordHeartbeatError(t *testing.T) {
	svc := &errorLifecycleService{}
	cfg := DefaultConfig()
	loop := New("w1", &mockTransport{}, svc, cfg, zap.NewNop())

	msg := ipc.Message{Type: ipc.TypeHeartbeat, Payload: []byte{}}
	loop.ExportHandleMessage(context.Background(), msg)

	// Should not panic - error is logged
}

// Test handleMessage with ErrNotFound
func TestHandleMessage_RecordHeartbeatNotFound(t *testing.T) {
	svc := &notFoundLifecycleService{}
	cfg := DefaultConfig()
	loop := New("w1", &mockTransport{}, svc, cfg, zap.NewNop())

	msg := ipc.Message{Type: ipc.TypeHeartbeat, Payload: []byte{}}
	loop.ExportHandleMessage(context.Background(), msg)

	// Should return early without error
}

// Test Run loop exit conditions
func TestRun_ContextCancelled(t *testing.T) {
	transport := &mockTransport{
		messages: []ipc.Message{},
		errors:   []error{},
	}
	svc := &mockLifecycleService{}
	cfg := Config{Interval: 50 * time.Millisecond, MissedThreshold: 10}
	loop := New("w-ctx", transport, svc, cfg, zap.NewNop())

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
		t.Error("Loop.Run did not exit after context cancellation")
	}
}

// Test receiveWithTimeout
func TestReceiveWithTimeout(t *testing.T) {
	transport := &mockTransport{
		messages: []ipc.Message{{Type: ipc.TypeHeartbeat, Payload: []byte{}}},
	}
	svc := &mockLifecycleService{}
	cfg := DefaultConfig()
	loop := New("w1", transport, svc, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	msg, err := loop.receiveWithTimeout(ctx, time.Now())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if msg.Type != ipc.TypeHeartbeat {
		t.Errorf("expected TypeHeartbeat, got %v", msg.Type)
	}
}

// Additional mock services for testing
type errorLifecycleService struct{}

func (s *errorLifecycleService) RecordHeartbeat(_ context.Context, _ string) error {
	return errors.New("db error")
}
func (s *errorLifecycleService) MarkUnhealthy(_ context.Context, _ string) error { return nil }
func (s *errorLifecycleService) MarkRunning(_ context.Context, _ string) error  { return nil }

type notFoundLifecycleService struct{}

func (s *notFoundLifecycleService) RecordHeartbeat(_ context.Context, _ string) error {
	return worker.ErrNotFound
}
func (s *notFoundLifecycleService) MarkUnhealthy(_ context.Context, _ string) error { return nil }
func (s *notFoundLifecycleService) MarkRunning(_ context.Context, _ string) error  { return nil }
