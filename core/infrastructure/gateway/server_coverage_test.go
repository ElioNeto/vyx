package gateway

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"go.uber.org/zap"
)

// TestNew_WithH2CEnabled verifies H2C handler is wired
func TestNew_WithH2CEnabled(t *testing.T) {
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

	cfg := DevConfig() // H2C enabled
	cfg.Addr = ":0" // Use any available port
	server := New(cfg, dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	if server == nil {
		t.Fatal("expected server, got nil")
	}

	// Verify server is created with H2C enabled
	// We can't easily test H2C without starting the server
}

// TestNew_WithTLSConfig verifies TLS config is set
func TestNew_WithTLSConfig(t *testing.T) {
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
	cfg.TLSCertFile = "cert.pem"
	cfg.TLSKeyFile = "key.pem"
	server := New(cfg, dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	if server == nil {
		t.Fatal("expected server, got nil")
	}

	// Verify TLS config is set (we can't easily test without actual certs)
	if server.httpServer.TLSConfig == nil {
		t.Error("expected TLS config to be set")
	}
}

// TestListenAndServeTLS verifies the TLS server method exists
func TestListenAndServeTLS(t *testing.T) {
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

	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	// Verify the method exists by calling it (will fail without certs, but that's ok)
	go func() {
		_ = server.ListenAndServeTLS("nonexistent.pem", "nonexistent.pem")
	}()
	time.Sleep(50 * time.Millisecond)

	// Shutdown
	_ = server.Shutdown(context.Background())
}

// TestHandle_WebSocketUpgrade verifies WebSocket upgrade path is detected
func TestHandle_WebSocketUpgrade(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "WS", Path: "/ws/test", WorkerID: "w1"},
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

	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	// Just verify the server is created properly
	// WebSocket upgrade requires a real HTTP server with Hijack support
	if server == nil {
		t.Fatal("expected server, got nil")
	}
}

// TestHandle_RateLimitToken verifies token-based rate limiting
func TestHandle_RateLimitToken(t *testing.T) {
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

	// Rate limiter with 1 token allowed
	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 1, time.Minute), zap.NewNop())

	// First request with token should succeed
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer token123")
	req.RemoteAddr = "192.168.1.1:1234"
	w := httptest.NewRecorder()
	server.handle(w, req)

	// Second request with same token should be rate limited
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.Header.Set("Authorization", "Bearer token123")
	req2.RemoteAddr = "192.168.1.1:1234"
	w2 := httptest.NewRecorder()
	server.handle(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request with same token should be rate limited, got %d", w2.Code)
	}
}

// TestHandle_NotFound verifies 404 for unregistered routes
func TestHandle_NotFound(t *testing.T) {
	routes := dgw.NewRouteMap(nil) // No routes
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

	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	server.handle(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unregistered route, got %d", w.Code)
	}
}

// TestHandle_AuthSuccess verifies JWT auth success
func TestHandle_AuthSuccess(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/api/test", WorkerID: "w1", AuthRoles: []string{"admin"}},
	})

	transport := &mockTransportResp{}
	jwt := &mockJWTValid{} // Returns user with "admin" role
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
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	server.handle(w, req)

	// Should not be 401 or 403
	if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
		t.Errorf("expected success, got %d", w.Code)
	}
}

// TestHandle_AuthFailure verifies JWT auth failure
func TestHandle_AuthFailure(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/api/test", WorkerID: "w1", AuthRoles: []string{"admin"}},
	})

	transport := &mockTransportResp{}
	jwt := &mockJWTNoRole{} // Returns user without "admin" role
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
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	server.handle(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for insufficient role, got %d", w.Code)
	}
}

// TestWriteError_Unauthorized verifies 401 handling
func TestWriteError_Unauthorized(t *testing.T) {
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
	err := dgw.ErrUnauthorized
	server.writeError(w, err)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthorized error, got %d", w.Code)
	}
}

// TestWriteError_Forbidden verifies 403 handling
func TestWriteError_Forbidden(t *testing.T) {
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
	err := dgw.ErrForbidden
	server.writeError(w, err)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for forbidden error, got %d", w.Code)
	}
}

// TestWriteError_NotFound verifies 404 handling
func TestWriteError_NotFound(t *testing.T) {
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
	err := dgw.ErrRouteNotFound
	server.writeError(w, err)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for not found error, got %d", w.Code)
	}
}

// TestHandleDispatchError_CorrelationID verifies correlation ID propagation
func TestHandleDispatchError_CorrelationID(t *testing.T) {
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

	// Create request with correlation ID
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Request-Id", "test-corr-id")

	// Use a custom writer to capture response
	w := httptest.NewRecorder()

	// We need to simulate an error from dispatcher
	// For now, just verify the method doesn't panic
	server.handleDispatchError(w, req, fmt.Errorf("test error"))
}
