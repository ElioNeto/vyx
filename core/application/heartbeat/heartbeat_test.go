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

type testTransport struct {
	mu       sync.Mutex
	messages []ipc.Message
	errors   []error
	cursor   int
}

func (m *testTransport) Receive(_ context.Context, _ string) (ipc.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cursor >= len(m.messages) {
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
func (m *testTransport) Send(_ context.Context, _ string, _ ipc.Message) error { return nil }
func (m *testTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, nil
}
func (m *testTransport) Register(_ context.Context, _ string) error             { return nil }
func (m *testTransport) Deregister(_ context.Context, _ string) error           { return nil }
func (m *testTransport) Close() error                                           { return nil }

type testService struct {
	mu            sync.Mutex
	heartbeats    int
	unhealthy     bool
	running       bool
	unhealthyIDs []string
	heartbeatIDs []string
}

func (s *testService) RecordHeartbeat(_ context.Context, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.heartbeats++
	s.heartbeatIDs = append(s.heartbeatIDs, "w1")
	return nil
}
func (s *testService) MarkUnhealthy(_ context.Context, workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unhealthy = true
	s.unhealthyIDs = append(s.unhealthyIDs, workerID)
	return nil
}
func (s *testService) MarkRunning(_ context.Context, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = true
	return nil
}

type testLister struct {
	mu  sync.Mutex
	ids []string
}

func (l *testLister) LiveWorkerIDs(_ context.Context) ([]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]string, len(l.ids))
	copy(result, l.ids)
	return result, nil
}

type fakeTransport struct {
	mu   sync.Mutex
	msgs map[string][]ipc.Message
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

type failingTransport struct{}
func (f *failingTransport) Send(_ context.Context, _ string, _ ipc.Message) error {
	return &mockErr{}
}
func (f *failingTransport) Receive(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, nil
}
func (f *failingTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, nil
}
func (f *failingTransport) Register(_ context.Context, _ string) error   { return nil }
func (f *failingTransport) Deregister(_ context.Context, _ string) error { return nil }
func (f *failingTransport) Close() error                                  { return nil }

type mockErr struct{}
func (e *mockErr) Error() string { return "send failed" }

type errorLister struct{}
func (e *errorLister) LiveWorkerIDs(_ context.Context) ([]string, error) {
	return nil, errors.New("repository unavailable")
}

// --- Tests for DefaultConfig ---

func TestDefaultConfig(t *testing.T) {
	cfg := heartbeat.DefaultConfig()
	if cfg.Interval != 5*time.Second {
		t.Errorf("expected Interval 5s, got %v", cfg.Interval)
	}
	if cfg.ReadTimeout != 10*time.Second {
		t.Errorf("expected ReadTimeout 10s, got %v", cfg.ReadTimeout)
	}
	if cfg.MissedThreshold != 2 {
		t.Errorf("expected MissedThreshold 2, got %d", cfg.MissedThreshold)
	}
	if cfg.ConnectGrace != 30*time.Second {
		t.Errorf("expected ConnectGrace 30s, got %v", cfg.ConnectGrace)
	}
	if cfg.RetryInterval != 500*time.Millisecond {
		t.Errorf("expected RetryInterval 500ms, got %v", cfg.RetryInterval)
	}
}

// --- Tests for Loop ---

func TestLoop_HeartbeatReceived_CallsRecordHeartbeat(t *testing.T) {
	transport := &testTransport{
		messages: []ipc.Message{
			{Type: ipc.TypeHeartbeat, Payload: []byte{}},
			{Type: ipc.TypeHeartbeat, Payload: []byte{}},
		},
	}
	svc := &testService{}

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
	transport := &testTransport{
		messages: []ipc.Message{},
		errors:   []error{},
	}
	svc := &testService{}

	cfg := heartbeat.Config{Interval: 20 * time.Millisecond, MissedThreshold: 2}
	loop := heartbeat.New("worker-dead", transport, svc, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	loop.Run(ctx)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if !svc.unhealthy {
		t.Error("expected MarkUnhealthy to be called after missed threshold")
	}
}

func TestLoop_ContextCancelled_ExitsCleanly(t *testing.T) {
	transport := &testTransport{messages: []ipc.Message{}}
	svc := &testService{}

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

func TestLoop_HeartbeatResetsMissedCounter(t *testing.T) {
	transport := &testTransport{
		messages: []ipc.Message{
			{Type: ipc.TypeHeartbeat, Payload: []byte{}},
		},
	}
	svc := &testService{}

	cfg := heartbeat.Config{Interval: 20 * time.Millisecond, MissedThreshold: 2}
	loop := heartbeat.New("worker-reset", transport, svc, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	loop.Run(ctx)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.unhealthy {
		t.Error("MarkUnhealthy should not be called when heartbeat resets the counter")
	}
}

func TestLoop_HandleMessage_Handshake(t *testing.T) {
	transport := &testTransport{
		messages: []ipc.Message{
			{Type: ipc.TypeHandshake, Payload: []byte(`{"worker_id":"w1"}`)},
			{Type: ipc.TypeHeartbeat, Payload: []byte{}},
		},
	}
	svc := &testService{}

	cfg := heartbeat.Config{Interval: 50 * time.Millisecond, MissedThreshold: 2}
	loop := heartbeat.New("w1", transport, svc, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	loop.Run(ctx)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if !svc.running {
		t.Error("expected MarkRunning to be called on handshake")
	}
}

func TestLoop_HandleMessage_ResponseIgnored(t *testing.T) {
	transport := &testTransport{
		messages: []ipc.Message{
			{Type: ipc.TypeResponse, Payload: []byte(`{"body":"ok"}`)},
			{Type: ipc.TypeHeartbeat, Payload: []byte{}},
		},
	}
	svc := &testService{}

	cfg := heartbeat.Config{Interval: 50 * time.Millisecond, MissedThreshold: 2}
	loop := heartbeat.New("w1", transport, svc, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	loop.Run(ctx)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.unhealthy {
		t.Error("TypeResponse should not trigger MarkUnhealthy")
	}
}

func TestLoop_HandleMessage_ErrorIgnored(t *testing.T) {
	transport := &testTransport{
		messages: []ipc.Message{
			{Type: ipc.TypeError, Payload: []byte("something broke")},
			{Type: ipc.TypeHeartbeat, Payload: []byte{}},
		},
	}
	svc := &testService{}

	cfg := heartbeat.Config{Interval: 50 * time.Millisecond, MissedThreshold: 2}
	loop := heartbeat.New("w1", transport, svc, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	loop.Run(ctx)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.unhealthy {
		t.Error("TypeError should not trigger MarkUnhealthy")
	}
}

func TestLoop_HandleMessage_DefaultCase(t *testing.T) {
	transport := &testTransport{
		messages: []ipc.Message{
			{Type: 0x99, Payload: []byte("unknown")},
			{Type: ipc.TypeHeartbeat, Payload: []byte{}},
		},
	}
	svc := &testService{}

	cfg := heartbeat.Config{Interval: 50 * time.Millisecond, MissedThreshold: 2}
	loop := heartbeat.New("w1", transport, svc, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	loop.Run(ctx)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.unhealthy {
		t.Error("unknown message type should not trigger MarkUnhealthy")
	}
}

func TestLoop_ReceiveWithTimeout_ContextCancelled(t *testing.T) {
	transport := &testTransport{
		messages: []ipc.Message{}, // no messages, will block
	}
	svc := &testService{}

	cfg := heartbeat.Config{Interval: 50 * time.Millisecond, MissedThreshold: 10}
	loop := heartbeat.New("w-ctx-timeout", transport, svc, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should return early due to context cancellation
	loop.Run(ctx)

	// If we get here without panic, the test passes
}

// --- Tests for Receiver ---

func TestReceiver_StartLoop_RecordsHeartbeat(t *testing.T) {
	transport := newFakeTransport()
	lister := &testLister{ids: []string{"w1"}}
	svc := &fakeService{}
	log := zap.NewNop()

	cfg := heartbeat.Config{Interval: 50 * time.Millisecond, MissedThreshold: 3}
	recv := heartbeat.NewReceiver(transport, lister, svc, cfg, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = transport.Send(ctx, "w1", ipc.Message{Type: ipc.TypeHeartbeat})

	recv.StartLoop(ctx, "w1")

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
	lister := &testLister{ids: []string{"w2"}}
	svc := &fakeService{}
	log := zap.NewNop()

	cfg := heartbeat.Config{Interval: 20 * time.Millisecond, MissedThreshold: 2}
	recv := heartbeat.NewReceiver(transport, lister, svc, cfg, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	recv.StartLoop(ctx, "w2")

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
	lister := &testLister{ids: []string{"w3"}}
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

	recv.Run(ctx)
}

func TestReceiver_SetService(t *testing.T) {
	transport := newFakeTransport()
	lister := &testLister{ids: []string{"w-set-service"}}
	svc := &fakeService{}
	log := zap.NewNop()

	cfg := heartbeat.Config{Interval: 50 * time.Millisecond, MissedThreshold: 3}
	recv := heartbeat.NewReceiver(transport, lister, nil, cfg, log)

	// SetService should wire the service (previously nil).
	recv.SetService(svc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Pre-queue a TypeHeartbeat frame for w-set-service.
	_ = transport.Send(ctx, "w-set-service", ipc.Message{Type: ipc.TypeHeartbeat})

	// Start a loop for the worker.
	recv.StartLoop(ctx, "w-set-service")

	// Give it time to process the frame.
	time.Sleep(80 * time.Millisecond)

	svc.mu.Lock()
	got := len(svc.heartbeats)
	svc.mu.Unlock()

	if got == 0 {
		t.Fatal("expected at least one RecordHeartbeat call after SetService")
	}
}

func TestReceiver_RestartLoop(t *testing.T) {
	transport := newFakeTransport()
	lister := &testLister{ids: []string{"w-restart"}}
	svc := &fakeService{}
	log := zap.NewNop()

	cfg := heartbeat.Config{Interval: 50 * time.Millisecond, MissedThreshold: 3}
	recv := heartbeat.NewReceiver(transport, lister, svc, cfg, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start initial loop.
	recv.StartLoop(ctx, "w-restart")

	// Give it time to establish.
	time.Sleep(80 * time.Millisecond)

	// Restart the loop (simulate worker restart).
	recv.RestartLoop(ctx, "w-restart")

	// Pre-queue a heartbeat for the restarted loop.
	_ = transport.Send(ctx, "w-restart", ipc.Message{Type: ipc.TypeHeartbeat})

	// Should not panic; old loop canceled, new one started.
	time.Sleep(80 * time.Millisecond)

	svc.mu.Lock()
	got := len(svc.heartbeats)
	svc.mu.Unlock()

	if got == 0 {
		t.Fatal("expected heartbeats after RestartLoop")
	}
}

func TestReceiver_StopAll(t *testing.T) {
	transport := newFakeTransport()
	lister := &testLister{ids: []string{"w1", "w2"}}
	svc := &fakeService{}
	log := zap.NewNop()

	cfg := heartbeat.Config{Interval: 50 * time.Millisecond, MissedThreshold: 3}
	recv := heartbeat.NewReceiver(transport, lister, svc, cfg, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	recv.StartLoop(ctx, "w1")
	recv.StartLoop(ctx, "w2")

	time.Sleep(80 * time.Millisecond)
	cancel()

	time.Sleep(50 * time.Millisecond)
	// If we reach here without panic, stopAll worked.
}

func TestReceiver_Reconcile_AddsAndRemoves(t *testing.T) {
	transport := newFakeTransport()
	lister := &testLister{ids: []string{"w1", "w2"}}
	svc := &fakeService{}
	log := zap.NewNop()

	cfg := heartbeat.Config{Interval: 50 * time.Millisecond, MissedThreshold: 3}
	recv := heartbeat.NewReceiver(transport, lister, svc, cfg, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		recv.Run(ctx)
		close(done)
	}()

	time.Sleep(80 * time.Millisecond)

	lister.mu.Lock()
	lister.ids = []string{"w1", "w3"}
	lister.mu.Unlock()

	time.Sleep(80 * time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not exit after context cancellation")
	}
}

// --- Tests for Sender ---

func TestSender_SendsHeartbeatFrame(t *testing.T) {
	transport := &recordingTransport{}
	lister := &testLister{ids: []string{"worker-1", "worker-2"}}

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
	lister := &testLister{ids: []string{"node:api", "go:billing", "python:ml"}}

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
	lister := &testLister{ids: []string{"worker-1"}}

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
	transport := &failingTransport{}
	lister := &testLister{ids: []string{"worker-broken"}}

	cfg := heartbeat.Config{Interval: 20 * time.Millisecond, MissedThreshold: 2}
	sender := heartbeat.NewSender(transport, lister, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	sender.Run(ctx)
}

func TestSender_SendAll_LogsWarningOnError(t *testing.T) {
	errorTransport := &partialErrorTransport{errorFor: "worker-fail"}
	lister := &testLister{ids: []string{"worker-ok", "worker-fail"}}

	cfg := heartbeat.Config{Interval: 30 * time.Millisecond, MissedThreshold: 2}
	sender := heartbeat.NewSender(errorTransport, lister, cfg, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	sender.Run(ctx)

	errorTransport.mu.Lock()
	sentCount := len(errorTransport.sent)
	errorTransport.mu.Unlock()

	if sentCount < 2 {
		t.Errorf("expected at least 2 sends (one per worker per tick), got %d", sentCount)
	}
}

type partialErrorTransport struct {
	recordingTransport
	errorFor string
}

func (p *partialErrorTransport) Send(ctx context.Context, workerID string, msg ipc.Message) error {
	if workerID == p.errorFor {
		return errors.New("simulated send error")
	}
	return p.recordingTransport.Send(ctx, workerID, msg)
}
