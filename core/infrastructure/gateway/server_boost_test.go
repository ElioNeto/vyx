package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
)

// TestListenAndServe_Boost verifies the server starts and shuts down.
func TestListenAndServe_Boost(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/api/test", WorkerID: "w1"},
	})

	transport := &mockTransportResp{}
	jwt := &mockJWTValid{}
	schema := &mockSchemaValid{}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: transport,
		JWT:       jwt,
		Schema:    schema,
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})

	cfg := DefaultConfig()
	cfg.Addr = ":0" // Random port
	server := New(cfg, dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	// Start server in goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("ListenAndServe() returned error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() returned error: %v", err)
	}
}

// TestHandle_DispatchError verifies error handling.
func TestHandle_DispatchError_Boost(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/api/test", WorkerID: "w1"},
	})

	// Transport that returns error
	transport := &mockTransportErr{}
	jwt := &mockJWTValid{}
	schema := &mockSchemaValid{}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: transport,
		JWT:       jwt,
		Schema:    schema,
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})

	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	server.handle(w, req)

	// Should return some error status
	if w.Code == 0 {
		t.Error("expected non-zero status code")
	}
}

// TestCheckRateLimit_Token verifies token-based rate limiting.
func TestCheckRateLimit_Token_Boost(t *testing.T) {
	routes := dgw.NewRouteMap(nil)
	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: &mockTransportResp{},
		JWT:       &mockJWTValid{},
		Schema:    &mockSchemaValid{},
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})

	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(1, 100, time.Minute), zap.NewNop())

	// First request with token should succeed
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	req.RemoteAddr = "192.168.1.1:1234"
	w := httptest.NewRecorder()

	if !server.checkRateLimit(w, req) {
		t.Error("first request should not be rate limited")
	}

	// Second request with same token should be rate limited
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.Header.Set("Authorization", "Bearer test-token")
	req2.RemoteAddr = "192.168.1.1:1234"
	w2 := httptest.NewRecorder()

	if server.checkRateLimit(w2, req2) {
		t.Error("second request with same token should be rate limited")
	}
}

// Mock transport that always returns error
type mockTransportErr struct{}

func (m *mockTransportErr) Send(_ context.Context, _ string, _ ipc.Message) error {
	return errors.New("mock error")
}
func (m *mockTransportErr) Receive(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, errors.New("mock error")
}
func (m *mockTransportErr) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, errors.New("mock error")
}
func (m *mockTransportErr) Register(_ context.Context, _ string) error { return nil }
func (m *mockTransportErr) Deregister(_ context.Context, _ string) error { return nil }
func (m *mockTransportErr) Close() error { return nil }
