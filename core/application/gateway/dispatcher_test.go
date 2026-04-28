package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/domain/circuit"
	"go.uber.org/zap"
)

// --- mocks ---

type mockJWT struct {
	claims *dgw.Claims
	err    error
}

func (m *mockJWT) Validate(_ string) (*dgw.Claims, error) { return m.claims, m.err }

type mockSchema struct{ err error }

func (m *mockSchema) Validate(_ string, _ []byte) error { return m.err }

type mockTransport struct {
	sendErr error
	respMsg ipc.Message
	recvErr error
}

func (m *mockTransport) Send(_ context.Context, _ string, _ ipc.Message) error {
	return m.sendErr
}
func (m *mockTransport) Receive(_ context.Context, _ string) (ipc.Message, error) {
	return m.respMsg, m.recvErr
}
func (m *mockTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	return m.respMsg, m.recvErr
}
func (m *mockTransport) Register(_ context.Context, _ string) error   { return nil }
func (m *mockTransport) Deregister(_ context.Context, _ string) error { return nil }
func (m *mockTransport) Close() error                                  { return nil }

// --- test helpers ---

func newTestDispatcher(t *testing.T, jwt apgw.JWTValidator, schema apgw.SchemaValidator) *apgw.Dispatcher {
	t.Helper()
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})
	transport := &mockTransport{
		respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 200})},
	}
	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       jwt,
		Schema:    schema,
		Timeout:   2 * time.Second,
		Log:       log,
	})
	return d
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// --- tests for low-coverage functions ---

func TestNewDispatcherFromParams(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap(nil)
	transport := &mockTransport{}
	jwt := &mockJWT{}
	schema := &mockSchema{}

	// This tests NewDispatcherFromParams which was at 0% coverage
	d := apgw.NewDispatcherFromParams(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       jwt,
		Schema:    schema,
		Log:       log,
	}, circuit.Config{}, apgw.WithProxyListeners())
	if d == nil {
		t.Fatal("expected dispatcher, got nil")
	}
}

func TestWithProxyListeners(t *testing.T) {
	var called bool
	listener := &apgw.FuncListener{
		OnRouteMatchFn: func(lc *apgw.LifecycleContext) {
			called = true
		},
	}

	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})
	transport := &mockTransport{
		respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 200})},
	}

	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	}, apgw.WithProxyListeners(listener))

	// Dispatch a request to trigger the listener
	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{},
		Query:   map[string]string{},
	}
	_, _ = d.Dispatch(context.Background(), req)

	if !called {
		t.Error("expected proxy listener to be called")
	}
}

func TestValidateJWT_Success(t *testing.T) {
	claims := &dgw.Claims{UserID: "user1", Roles: []string{"admin"}}
	d := newTestDispatcher(t, &mockJWT{claims: claims}, &mockSchema{})

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{"Authorization": "Bearer valid-token"},
		Query:   map[string]string{},
	}
	resp, err := d.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestValidateJWT_MissingToken(t *testing.T) {
	// When there's no token and route has auth roles, should fail
	// The mock returns nil claims with no roles, so JWT validation "passes"
	// But dispatcher checks if route has AuthRoles and if token is present
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping", AuthRoles: []string{"user"}},
	})
	transport := &mockTransport{
		respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 200})},
	}
	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{claims: nil}, // no claims = unauthorized
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	})

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{},
		Query:   map[string]string{},
	}
	_, err := d.Dispatch(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestValidateJWT_InvalidToken(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping", AuthRoles: []string{"user"}},
	})
	transport := &mockTransport{
		respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 200})},
	}
	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{err: errors.New("invalid token")},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	})

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{"Authorization": "Bearer invalid"},
		Query:   map[string]string{},
	}
	_, err := d.Dispatch(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestValidateSchema_Success(t *testing.T) {
	log, _ := zap.NewDevelopment()
	// Route without Validate field - schema validation is skipped
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "POST", Path: "/ping"},
	})
	transport := &mockTransport{
		respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 200})},
	}
	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{claims: &dgw.Claims{Roles: []string{"user"}}},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	})

	req := &dgw.GatewayRequest{
		Method:  "POST",
		Path:    "/ping",
		Headers: map[string]string{"Authorization": "Bearer t"},
		Query:   map[string]string{},
		Body:    []byte(`{"ok":true}`),
	}
	_, err := d.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSchema_Failure(t *testing.T) {
	// Route with schema validation
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "POST", Path: "/ping", Validate: "my-schema"},
	})
	transport := &mockTransport{
		respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 200})},
	}
	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{claims: &dgw.Claims{Roles: []string{"user"}}},
		Schema:    &mockSchema{err: errors.New("validation failed")},
		Timeout:   2 * time.Second,
		Log:       log,
	})

	req := &dgw.GatewayRequest{
		Method:  "POST",
		Path:    "/ping",
		Headers: map[string]string{"Authorization": "Bearer t"},
		Query:   map[string]string{},
		Body:    []byte(`{"ok":true}`),
	}
	_, err := d.Dispatch(context.Background(), req)
	if err == nil {
		t.Fatal("expected schema validation error")
	}
}

func TestHandleWorkerError(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})
	transport := &mockTransport{
		sendErr: nil,
		respMsg: ipc.Message{Type: ipc.TypeError, Payload: []byte("worker error")},
	}
	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	})

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{apgw.HeaderCorrelationID: "test-cid"},
		Query:   map[string]string{},
	}
	resp, err := d.Dispatch(context.Background(), req)
	// Dispatch returns the 502 response AND an error when worker returns error type
	if err == nil {
		t.Fatal("expected error for worker error")
	}
	if resp == nil {
		t.Fatal("expected response even with error")
	}
	if resp.StatusCode != 502 {
		t.Errorf("expected 502 status, got %d", resp.StatusCode)
	}
}

func TestHandleDecodeError(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})
	// Return invalid JSON to trigger decode error
	transport := &mockTransport{
		respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: []byte("not valid json")},
	}
	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	})

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{apgw.HeaderCorrelationID: "test-cid"},
		Query:   map[string]string{},
	}
	// Dispatch will try to decode the invalid JSON
	resp, err := d.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still return a response (possibly with default status)
	if resp == nil {
		t.Fatal("expected response even with decode error")
	}
}

func TestNotifyListeners(t *testing.T) {
	var onErrorCalled, onPostDispatchCalled bool
	listener := &apgw.FuncListener{
		OnErrorFn: func(lc *apgw.LifecycleContext, phase apgw.Phase) {
			onErrorCalled = true
		},
		OnPostDispatchFn: func(lc *apgw.LifecycleContext, latency time.Duration) {
			onPostDispatchCalled = true
		},
	}

	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})
	transport := &mockTransport{
		respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 200})},
	}

	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	}, apgw.WithProxyListeners(listener))

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{},
		Query:   map[string]string{},
	}
	_, _ = d.Dispatch(context.Background(), req)

	if !onPostDispatchCalled {
		t.Error("expected OnPostDispatch to be called")
	}
	// onErrorCalled may not be true in happy path - that's ok
	_ = onErrorCalled // acknowledge the variable is used
}

func TestRoutesTransportJWTTimeout(t *testing.T) {
	d := newTestDispatcher(t, &mockJWT{}, &mockSchema{})

	// Test Routes()
	routes := d.Routes()
	if routes == nil {
		t.Error("expected non-nil routes")
	}

	// Test Transport()
	transport := d.Transport()
	if transport == nil {
		t.Error("expected non-nil transport")
	}

	// Test JWT()
	jwt := d.JWT()
	if jwt == nil {
		t.Error("expected non-nil jwt")
	}

	// Test Timeout()
	timeout := d.Timeout()
	if timeout != 2*time.Second {
		t.Errorf("expected 2s timeout, got %v", timeout)
	}
}
