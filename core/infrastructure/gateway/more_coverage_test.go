package gateway

import (
	"context"
	"testing"
	"time"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
	"go.uber.org/zap"
)

// Minimal mock types for testing
type testTransport struct{}

func (t *testTransport) Send(_ context.Context, _ string, _ ipc.Message) error { return nil }
func (t *testTransport) Receive(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, nil
}
func (t *testTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, nil
}
func (t *testTransport) Register(_ context.Context, _ string) error  { return nil }
func (t *testTransport) Deregister(_ context.Context, _ string) error { return nil }
func (t *testTransport) Close() error                                   { return nil }

type testJWT struct{}

func (j *testJWT) Validate(_ string) (*dgw.Claims, error) { return nil, nil }

type testSchema struct{}

func (s *testSchema) Validate(_ string, _ []byte) error { return nil }

// TestHandleFunction tests the handle function indirectly
func TestHandleFunction(t *testing.T) {
	// We can't call handle directly as it's unexported and needs http.ResponseWriter
	// But we can test it through the server
	cfg := DefaultConfig()
	log := zap.NewNop()

	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/test"},
	})

	transport := &testTransport{}
	jwt := &testJWT{}
	schema := &testSchema{}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       jwt,
		Schema:    schema,
		Timeout:    time.Second,
		Log:       log,
	})

	_ = New(cfg, dispatcher, nil, log)

	// The handle function would be called by the server
	// This test just ensures New doesn't panic
}

// TestNew_WithRateLimiter tests New with rate limiter
func TestNew_WithRateLimiter(t *testing.T) {
	cfg := DefaultConfig()
	log := zap.NewNop()

	rm := dgw.NewRouteMap(nil)
	transport := &testTransport{}
	jwt := &testJWT{}
	schema := &testSchema{}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       jwt,
		Schema:    schema,
		Timeout:    time.Second,
		Log:       log,
	})

	rateLimiter := apgw.NewRateLimiter(100, 50, time.Minute)
	s := New(cfg, dispatcher, rateLimiter, log)
	if s == nil {
		t.Fatal("expected server, got nil")
	}
}

// TestServerMethods tests exported methods
func TestServerMethods(t *testing.T) {
	cfg := DefaultConfig()
	log := zap.NewNop()

	rm := dgw.NewRouteMap(nil)
	transport := &testTransport{}
	jwt := &testJWT{}
	schema := &testSchema{}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       jwt,
		Schema:    schema,
		Timeout:    time.Second,
		Log:       log,
	})

	s := New(cfg, dispatcher, nil, log)

	// Test Addr
	addr := s.Addr()
	if addr != ":8080" {
		t.Errorf("Addr() = %q, want %q", addr, ":8080")
	}
}
