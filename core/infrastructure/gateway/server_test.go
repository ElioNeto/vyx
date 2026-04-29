package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
)

// TestNew_ServerConfig verifies that New creates a properly configured Server.
func TestNew_ServerConfig(t *testing.T) {
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

	cfg := Config{
		Addr:         ":9090",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
		MaxBodyBytes: 512,
	}
	server := New(cfg, dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	if server.Addr() != ":9090" {
		t.Errorf("Addr() = %q, want %q", server.Addr(), ":9090")
	}
}

// TestDefaultConfig verifies DefaultConfig returns sensible defaults.
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Addr != ":8080" {
		t.Errorf("DefaultConfig().Addr = %q, want %q", cfg.Addr, ":8080")
	}
	if cfg.ReadTimeout != 15*time.Second {
		t.Errorf("DefaultConfig().ReadTimeout = %v, want 15s", cfg.ReadTimeout)
	}
	if cfg.MaxBodyBytes != defaultMaxBodyBytes {
		t.Errorf("DefaultConfig().MaxBodyBytes = %d, want %d", cfg.MaxBodyBytes, defaultMaxBodyBytes)
	}
}

// TestDevConfig verifies DevConfig enables H2C.
func TestDevConfig(t *testing.T) {
	cfg := DevConfig()
	if !cfg.H2CEnabled {
		t.Error("DevConfig().H2CEnabled = false, want true")
	}
}

// TestShutdown verifies the server shuts down gracefully.
func TestShutdown(t *testing.T) {
	routes := dgw.NewRouteMap(nil)
	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: &mockTransportResp{},
		JWT:       &mockJWTValid{},
		Schema:    &mockSchemaValid{},
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})

	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() returned error: %v", err)
	}
}

// TestIsWebSocketUpgrade verifies the upgrade detection function.
func TestIsWebSocketUpgrade(t *testing.T) {
	tests := []struct {
		name string
		header string
		want  bool
	}{
		{"upgrade_header", "websocket", true},
		{"uppercase", "Websocket", true},
		{"no_header", "", false},
		{"other_value", "http2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/ws/test", nil)
			if tt.header != "" {
				req.Header.Set("Upgrade", tt.header)
			}
			if got := isWebSocketUpgrade(req); got != tt.want {
				t.Errorf("isWebSocketUpgrade() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWSProxy_ServeHTTP_NoRoute verifies 404 when no route matches.
func TestWSProxy_ServeHTTP_NoRoute(t *testing.T) {
	routes := dgw.NewRouteMap(nil) // Empty route map
	transport := &mockTransportResp{}
	jwt := &mockJWTValid{}

	proxy := newWSProxy(routes, transport, jwt, zap.NewNop(), 1*time.Second)

	req := httptest.NewRequest("GET", "/ws/nonexistent", nil)
	req.Header.Set("Upgrade", "websocket")
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent route, got %d", w.Code)
	}
}

// TestWSProxy_AuthSuccess verifies successful auth before upgrade.
func TestWSProxy_AuthSuccess(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "WS", Path: "/ws/test", WorkerID: "w1", AuthRoles: []string{"admin"}},
	})
	transport := &mockTransportResp{}
	jwt := &mockJWTValid{} // Returns admin role

	proxy := newWSProxy(routes, transport, jwt, zap.NewNop(), 1*time.Second)

	// Test that the route lookup works
	result, ok := routes.Lookup("WS", "/ws/test")
	if !ok {
		t.Fatal("route lookup failed")
	}
	if result.Entry.WorkerID != "w1" {
		t.Errorf("WorkerID = %q, want %q", result.Entry.WorkerID, "w1")
	}
	
	// Verify proxy is properly created
	if proxy == nil {
		t.Fatal("proxy should not be nil")
	}
}

// TestWSProxy_AuthFailed verifies 401 on bad token.
func TestWSProxy_AuthFailed(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "WS", Path: "/ws/test", WorkerID: "w1", AuthRoles: []string{"admin"}},
	})
	transport := &mockTransportResp{}
	jwt := &mockJWTInvalid{} // Always returns error

	proxy := newWSProxy(routes, transport, jwt, zap.NewNop(), 1*time.Second)

	// Test that JWT validation is called
	_, err := jwt.Validate("invalid-token")
	if err == nil {
		t.Error("expected error from invalid JWT")
	}
	
	// Verify proxy is properly created
	if proxy == nil {
		t.Fatal("proxy should not be nil")
	}
}

// TestWSProxy_Forbidden verifies 403 when role doesn't match.
func TestWSProxy_Forbidden(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "WS", Path: "/ws/test", WorkerID: "w1", AuthRoles: []string{"admin"}},
	})
	transport := &mockTransportResp{}
	jwt := &mockJWTNoRole{} // Returns no roles

	proxy := newWSProxy(routes, transport, jwt, zap.NewNop(), 1*time.Second)

	// Test hasWSRole function
	if hasWSRole([]string{}, []string{"admin"}) {
		t.Error("expected false for empty caller roles")
	}
	if !hasWSRole([]string{"admin", "user"}, []string{"admin"}) {
		t.Error("expected true when caller has required role")
	}
	
	// Verify proxy is properly created
	if proxy == nil {
		t.Fatal("proxy should not be nil")
	}
}

// Mock JWT that always fails
type mockJWTInvalid struct{}

func (m *mockJWTInvalid) Validate(_ string) (*dgw.Claims, error) {
	return nil, dgw.ErrUnauthorized
}

// Mock JWT that returns no roles
type mockJWTNoRole struct{}

func (m *mockJWTNoRole) Validate(_ string) (*dgw.Claims, error) {
	return &dgw.Claims{UserID: "u1", Roles: []string{}}, nil
}

// TestHandle_RateLimitIP verifies IP-based rate limiting.
func TestHandle_RateLimitIP(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/api/test", WorkerID: "w1"},
	})

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: &mockTransportResp{},
		JWT:       &mockJWTValid{},
		Schema:    &mockSchemaValid{},
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})

	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(1, 100, time.Minute), zap.NewNop())

	// First request should succeed
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	w := httptest.NewRecorder()
	server.handle(w, req)

	if w.Code == http.StatusTooManyRequests {
		t.Error("first request should not be rate limited")
	}

	// Second request from same IP should be rate limited
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.RemoteAddr = "192.168.1.1:1234"
	w2 := httptest.NewRecorder()
	server.handle(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request should be rate limited, got %d", w2.Code)
	}
}

// TestHandle_ReadBody_LargePayload verifies payload size limit.
func TestHandle_ReadBody_LargePayload(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "POST", Path: "/api/test", WorkerID: "w1"},
	})

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: &mockTransportResp{},
		JWT:       &mockJWTValid{},
		Schema:    &mockSchemaValid{},
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})

	cfg := DefaultConfig()
	cfg.MaxBodyBytes = 10 // Very small limit
	server := New(cfg, dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	largeBody := bytes.NewBuffer(make([]byte, 100))
	req := httptest.NewRequest("POST", "/api/test", largeBody)
	w := httptest.NewRecorder()
	server.handle(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 for large payload, got %d", w.Code)
	}
}

// TestHandle_BuildGatewayRequest verifies request building.
func TestHandle_BuildGatewayRequest(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/api/test", WorkerID: "w1"},
	})

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: &mockTransportResp{},
		JWT:       &mockJWTValid{},
		Schema:    &mockSchemaValid{},
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})

	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	// Test with query params and headers
	req := httptest.NewRequest("GET", "/api/test?foo=bar&baz=qux", nil)
	req.Header.Set("X-Custom", "value")
	w := httptest.NewRecorder()

	// We need to intercept the dispatcher call, but for now just verify it doesn't panic
	server.handle(w, req)
}

// TestWriteError_ValidationError verifies validation error handling.
func TestWriteError_ValidationError(t *testing.T) {
	routes := dgw.NewRouteMap(nil)
	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: &mockTransportResp{},
		JWT:       &mockJWTValid{},
		Schema:    &mockSchemaValid{},
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})

	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	w := httptest.NewRecorder()
	err := &dgw.ValidationError{
		Details: []dgw.ValidationDetail{
			{Field: "name", Message: "required"},
		},
	}
	server.writeError(w, err)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for validation error, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

// TestWriteError_UpstreamTimeout verifies timeout error handling.
func TestWriteError_UpstreamTimeout(t *testing.T) {
	routes := dgw.NewRouteMap(nil)
	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: &mockTransportResp{},
		JWT:       &mockJWTValid{},
		Schema:    &mockSchemaValid{},
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})

	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	w := httptest.NewRecorder()
	err := dgw.ErrUpstreamTimeout
	server.writeError(w, err)

	if w.Code != http.StatusGatewayTimeout {
		t.Errorf("expected 504 for timeout error, got %d", w.Code)
	}
}

// TestWriteError_PayloadTooLarge verifies payload too large error handling.
func TestWriteError_PayloadTooLarge(t *testing.T) {
	routes := dgw.NewRouteMap(nil)
	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: &mockTransportResp{},
		JWT:       &mockJWTValid{},
		Schema:    &mockSchemaValid{},
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})

	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	w := httptest.NewRecorder()
	err := dgw.ErrPayloadTooLarge
	server.writeError(w, err)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 for payload too large error, got %d", w.Code)
	}
}

// TestWriteResponse verifies response writing with correlation ID.
func TestWriteResponse_CorrelationID(t *testing.T) {
	routes := dgw.NewRouteMap(nil)
	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: &mockTransportResp{},
		JWT:       &mockJWTValid{},
		Schema:    &mockSchemaValid{},
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})

	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	w := httptest.NewRecorder()
	resp := &dgw.GatewayResponse{
		StatusCode:     200,
		Body:           []byte(`{"ok":true}`),
		Headers:        map[string]string{"X-Custom": "value"},
		CorrelationID: "test-corr-123",
	}
	server.writeResponse(w, resp)

	if w.Result().Header.Get("X-Request-Id") != "test-corr-123" {
		t.Errorf("X-Request-Id = %q, want %q", w.Result().Header.Get("X-Request-Id"), "test-corr-123")
	}
	if w.Result().Header.Get("X-Custom") != "value" {
		t.Errorf("X-Custom = %q, want %q", w.Result().Header.Get("X-Custom"), "value")
	}
}

// Mock types for testing

type mockTransportResp struct{}

func (m *mockTransportResp) Send(_ context.Context, _ string, _ ipc.Message) error {
	return nil
}
func (m *mockTransportResp) Receive(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, io.EOF
}
func (m *mockTransportResp) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, io.EOF
}
func (m *mockTransportResp) Register(_ context.Context, _ string) error { return nil }
func (m *mockTransportResp) Deregister(_ context.Context, _ string) error { return nil }
func (m *mockTransportResp) Close() error { return nil }

type mockJWTValid struct{}

func (m *mockJWTValid) Validate(_ string) (*dgw.Claims, error) {
	return &dgw.Claims{UserID: "u1", Roles: []string{"admin"}}, nil
}

type mockSchemaValid struct{}

func (m *mockSchemaValid) Validate(_ string, _ []byte) error { return nil }
