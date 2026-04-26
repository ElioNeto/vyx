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
func (m *mockTransport) Receive(_ context.Context, _ string) (ipc.Message, error) { return m.resp, nil }
func (m *mockTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) { return m.resp, nil }
func (m *mockTransport) Register(_ context.Context, _ string) error   { return nil }
func (m *mockTransport) Deregister(_ context.Context, _ string) error { return nil }
func (m *mockTransport) Close() error                                  { return nil }

type mockJWT struct{}
func (m *mockJWT) Validate(_ string) (*dgw.Claims, error) { return &dgw.Claims{UserID: "u1"}, nil }

type mockSchema struct{}
func (m *mockSchema) Validate(_ string, _ []byte) error { return nil }

func TestIntegration_CorrelationIDFlow(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/api/test", WorkerID: "w1"},
	})
	
	// Create a worker response that includes a correlation ID
	workerCid := uuid.NewString()
	payload, _ := json.Marshal(dgw.WorkerResponse{
		StatusCode:    200,
		Body:          []byte(`{"ok":true}`),
		CorrelationID: workerCid,
	})

	transport := &mockTransport{
		resp: ipc.Message{Type: ipc.TypeResponse, Payload: payload},
	}

	dispatcher := apgw.NewDispatcher(routes, transport, &mockJWT{}, &mockSchema{}, 1*time.Second, zap.NewNop(), nil)
	// We need a rate limiter to create a Server
	server := New(DefaultConfig(), dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), zap.NewNop())

	// Test case: Client sends request with X-Request-Id, worker returns its own
	clientCid := uuid.NewString()
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Request-Id", clientCid)
	
	w := httptest.NewRecorder()
	
	// Invoke server.handle
	server.handle(w, req)
	
	resp := w.Result()
	
	if resp.Header.Get("X-Request-Id") != workerCid {
		t.Errorf("expected correlation ID '%s', got '%s'", workerCid, resp.Header.Get("X-Request-Id"))
	}
}
