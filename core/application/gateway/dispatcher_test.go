package gateway_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
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

func (m *mockTransport) Send(_ context.Context, _ string, _ ipc.Message) error { return m.sendErr }
func (m *mockTransport) Receive(_ context.Context, _ string) (ipc.Message, error) {
	return m.respMsg, m.recvErr
}
func (m *mockTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	return m.respMsg, m.recvErr
}
func (m *mockTransport) Register(_ context.Context, _ string) error   { return nil }
func (m *mockTransport) Deregister(_ context.Context, _ string) error { return nil }
func (m *mockTransport) Close() error                                  { return nil }

func makeDispatcher(routes *dgw.RouteMap, transport *mockTransport, jwt *mockJWT, schema *mockSchema) *apgw.Dispatcher {
	return apgw.NewDispatcher(routes, transport, jwt, schema, 5*time.Second, zap.NewNop())
}

// --- tests ---

func TestDispatcher_RouteNotFound(t *testing.T) {
	routes := dgw.NewRouteMap(nil)
	d := makeDispatcher(routes, &mockTransport{}, &mockJWT{}, &mockSchema{})

	_, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{Method: "GET", Path: "/unknown"})
	if !errors.Is(err, dgw.ErrRouteNotFound) {
		t.Errorf("expected ErrRouteNotFound, got %v", err)
	}
}

func TestDispatcher_MissingJWT_ReturnsUnauthorized(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/secure", WorkerID: "node:api", AuthRoles: []string{"admin"}},
	})
	d := makeDispatcher(routes, &mockTransport{}, &mockJWT{}, &mockSchema{})

	_, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "GET", Path: "/secure",
		Headers: map[string]string{},
	})
	if !errors.Is(err, dgw.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestDispatcher_InvalidJWT_ReturnsUnauthorized(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/secure", WorkerID: "node:api", AuthRoles: []string{"admin"}},
	})
	d := makeDispatcher(routes, &mockTransport{}, &mockJWT{err: errors.New("bad token")}, &mockSchema{})

	_, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "GET", Path: "/secure",
		Headers: map[string]string{"Authorization": "Bearer bad"},
	})
	if !errors.Is(err, dgw.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestDispatcher_InsufficientRole_ReturnsForbidden(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/admin", WorkerID: "node:api", AuthRoles: []string{"admin"}},
	})
	d := makeDispatcher(routes, &mockTransport{}, &mockJWT{claims: &dgw.Claims{UserID: "u1", Roles: []string{"user"}}}, &mockSchema{})

	_, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "GET", Path: "/admin",
		Headers: map[string]string{"Authorization": "Bearer tok"},
	})
	if !errors.Is(err, dgw.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestDispatcher_SchemaValidationError_ReturnsBadRequest(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "POST", Path: "/users", WorkerID: "go:api", Validate: "user_create"},
	})
	d := makeDispatcher(routes, &mockTransport{}, &mockJWT{}, &mockSchema{err: errors.New("missing field")})

	_, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "POST", Path: "/users",
		Body:    []byte(`{}`),
		Headers: map[string]string{},
	})
	if !errors.Is(err, dgw.ErrSchemaValidation) {
		t.Errorf("expected ErrSchemaValidation, got %v", err)
	}
}

func TestDispatcher_SuccessfulDispatch(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/ping", WorkerID: "go:api"},
	})
	transport := &mockTransport{respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: []byte(`{"ok":true}`)}}
	d := makeDispatcher(routes, transport, &mockJWT{}, &mockSchema{})

	resp, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "GET", Path: "/ping",
		Headers: map[string]string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestDispatcher_WorkerError_Returns502(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/crash", WorkerID: "go:api"},
	})
	transport := &mockTransport{respMsg: ipc.Message{Type: ipc.TypeError, Payload: []byte(`worker panic`)}}
	d := makeDispatcher(routes, transport, &mockJWT{}, &mockSchema{})

	resp, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "GET", Path: "/crash",
		Headers: map[string]string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 502 {
		t.Errorf("expected 502 for worker error, got %d", resp.StatusCode)
	}
}

// --- ProxyListener hook tests ---

type hookRecorder struct {
	routeMatched bool
	preDispatch  bool
	postDispatch bool
	onError      bool
	errorPhase   apgw.Phase
	correlationID string
	route        *dgw.RouteEntry
}

func (r *hookRecorder) OnRouteMatch(lc *apgw.LifecycleContext) {
	r.routeMatched = true
	r.correlationID = lc.CorrelationID
	r.route = lc.Route
}
func (r *hookRecorder) OnPreDispatch(lc *apgw.LifecycleContext) {
	r.preDispatch = true
}
func (r *hookRecorder) OnPostDispatch(lc *apgw.LifecycleContext, _ time.Duration) {
	r.postDispatch = true
}
func (r *hookRecorder) OnError(lc *apgw.LifecycleContext, phase apgw.Phase) {
	r.onError = true
	r.errorPhase = phase
}

func TestDispatcher_Hooks_FullPipeline(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/ping", WorkerID: "go:api"},
	})
	transport := &mockTransport{respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: []byte(`{"ok":true}`)}}
	rec := &hookRecorder{}
	d := apgw.NewDispatcher(routes, transport, &mockJWT{}, &mockSchema{}, 5*time.Second, zap.NewNop(),
		apgw.WithProxyListeners(rec))

	_, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "GET", Path: "/ping",
		Headers: map[string]string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rec.routeMatched {
		t.Error("OnRouteMatch not called")
	}
	if !rec.preDispatch {
		t.Error("OnPreDispatch not called")
	}
	if !rec.postDispatch {
		t.Error("OnPostDispatch not called")
	}
	if rec.onError {
		t.Error("OnError should not be called on success")
	}
	if rec.correlationID == "" {
		t.Error("correlation ID should be non-empty")
	}
	if rec.route == nil || rec.route.Path != "/ping" {
		t.Errorf("route = %v, want /ping", rec.route)
	}
}

func TestDispatcher_Hooks_OnError(t *testing.T) {
	routes := dgw.NewRouteMap(nil)
	rec := &hookRecorder{}
	d := apgw.NewDispatcher(routes, &mockTransport{}, &mockJWT{}, &mockSchema{}, 5*time.Second, zap.NewNop(),
		apgw.WithProxyListeners(rec))

	_, _ = d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "GET", Path: "/unknown",
		Headers: map[string]string{},
	})
	if !rec.onError {
		t.Error("OnError should be called on 404")
	}
}

func TestDispatcher_Hooks_EarlyReturn(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/ping", WorkerID: "go:api"},
	})

	blocker := apgw.FuncListener{
		OnRouteMatchFn: func(lc *apgw.LifecycleContext) {
			lc.RespondBeforeDispatch(&dgw.GatewayResponse{
				StatusCode: 503, Body: []byte("maintenance"),
			})
		},
	}

	d := apgw.NewDispatcher(routes, &mockTransport{}, &mockJWT{}, &mockSchema{}, 5*time.Second, zap.NewNop(),
		apgw.WithProxyListeners(blocker))

	resp, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "GET", Path: "/ping",
		Headers: map[string]string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 503 {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
	if string(resp.Body) != "maintenance" {
		t.Errorf("body = %q, want %q", string(resp.Body), "maintenance")
	}
}
