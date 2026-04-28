package gateway

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
)

type mockTransport struct {
	resp ipc.Message
}

func (m *mockTransport) Send(_ context.Context, _ string, _ ipc.Message) error { return nil }
func (m *mockTransport) Receive(_ context.Context, _ string) (ipc.Message, error) {
	return m.resp, nil
}
func (m *mockTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	return m.resp, nil
}
func (m *mockTransport) Register(_ context.Context, _ string) error   { return nil }
func (m *mockTransport) Deregister(_ context.Context, _ string) error { return nil }
func (m *mockTransport) Close() error                                  { return nil }

type mockJWT struct{}

func (m *mockJWT) Validate(_ string) (*dgw.Claims, error) {
	return &dgw.Claims{UserID: "u1"}, nil
}

type mockSchema struct{}

func (m *mockSchema) Validate(_ string, _ []byte) error { return nil }

// TestCorrelationID_WorkerEchoedID verifies that when the worker echoes a
// correlation_id in its response envelope, that value is forwarded to the
// client as the X-Request-Id response header (#52).
func TestCorrelationID_WorkerEchoedID(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/api/test", WorkerID: "w1"},
	})

	// Worker echoes its own correlation ID.
	workerCid := uuid.NewString()
	payload, _ := json.Marshal(dgw.WorkerResponse{
		StatusCode:    200,
		Body:          []byte(`{"ok":true}`),
		CorrelationID: workerCid,
	})
	transport := &mockTransport{
		resp: ipc.Message{Type: ipc.TypeResponse, Payload: payload},
	}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: transport,
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})
	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	clientCid := uuid.NewString()
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Request-Id", clientCid)
	w := httptest.NewRecorder()
	server.handle(w, req)

	got := w.Result().Header.Get("X-Request-Id")
	if got != workerCid {
		t.Errorf("X-Request-Id = %q, want worker-echoed %q", got, workerCid)
	}
}

// TestCorrelationID_FallbackToRequestID verifies that when the worker does NOT
// echo a correlation_id, the original X-Request-Id sent by the client is
// reflected back (#52).
func TestCorrelationID_FallbackToRequestID(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/api/test", WorkerID: "w1"},
	})

	// Worker omits correlation_id — old/unaware worker.
	payload, _ := json.Marshal(dgw.WorkerResponse{
		StatusCode: 200,
		Body:       []byte(`{"ok":true}`),
	})
	transport := &mockTransport{
		resp: ipc.Message{Type: ipc.TypeResponse, Payload: payload},
	}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: transport,
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})
	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	clientCid := uuid.NewString()
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Request-Id", clientCid)
	w := httptest.NewRecorder()
	server.handle(w, req)

	got := w.Result().Header.Get("X-Request-Id")
	if got != clientCid {
		t.Errorf("X-Request-Id = %q, want client-echoed %q", got, clientCid)
	}
}

// TestCorrelationID_GeneratedWhenAbsent verifies that when neither the client
// nor the worker provide a correlation ID, the server generates one and
// includes it in the response (#52).
func TestCorrelationID_GeneratedWhenAbsent(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/api/test", WorkerID: "w1"},
	})

	payload, _ := json.Marshal(dgw.WorkerResponse{
		StatusCode: 200,
		Body:       []byte(`{"ok":true}`),
	})
	transport := &mockTransport{
		resp: ipc.Message{Type: ipc.TypeResponse, Payload: payload},
	}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: transport,
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})
	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	// No X-Request-Id header — dispatcher should auto-generate one.
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	server.handle(w, req)

	got := w.Result().Header.Get("X-Request-Id")
	if got == "" {
		t.Error("expected a generated X-Request-Id header, got empty string")
	}
}

// TestCorrelationID_EchoedOnErrorPath verifies that the X-Request-Id is
// present even when the dispatcher returns an error (e.g., 404) (#52).
func TestCorrelationID_EchoedOnErrorPath(t *testing.T) {
	// Empty route map — every request returns 404.
	routes := dgw.NewRouteMap(nil)
	transport := &mockTransport{}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    routes,
		Transport: transport,
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   1 * time.Second,
		Log:       zap.NewNop(),
	})
	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	clientCid := uuid.NewString()
	req := httptest.NewRequest("GET", "/not-found", nil)
	req.Header.Set("X-Request-Id", clientCid)
	w := httptest.NewRecorder()
	server.handle(w, req)

	if w.Code != 404 {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	got := w.Result().Header.Get("X-Request-Id")
	if got != clientCid {
		t.Errorf("X-Request-Id on error = %q, want %q", got, clientCid)
	}
}
