package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/application/lifecycle"
)

type hookCallOrder struct {
	calls []string
}

func (h *hookCallOrder) record(name string) {
	h.calls = append(h.calls, name)
}

func (h *hookCallOrder) get() []string {
	return h.calls
}

type lifecycleHook struct {
	onBeforeDispatch  func(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry) error
	onAfterDispatch   func(ctx context.Context, req *dgw.GatewayRequest, resp *dgw.GatewayResponse)
	onWorkerError     func(ctx context.Context, workerID string, req *dgw.GatewayRequest, err error)
	onBeforeDispatchErr error
}

func (h *lifecycleHook) OnBeforeDispatch(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry) error {
	if h.onBeforeDispatch != nil {
		h.onBeforeDispatch(ctx, req, route)
	}
	return h.onBeforeDispatchErr
}

func (h *lifecycleHook) OnAfterDispatch(ctx context.Context, req *dgw.GatewayRequest, resp *dgw.GatewayResponse) {
	if h.onAfterDispatch != nil {
		h.onAfterDispatch(ctx, req, resp)
	}
}

func (h *lifecycleHook) OnWorkerError(ctx context.Context, workerID string, req *dgw.GatewayRequest, err error) {
	if h.onWorkerError != nil {
		h.onWorkerError(ctx, workerID, req, err)
	}
}

func TestLifecycleHooks_Ordering(t *testing.T) {
	order := &hookCallOrder{}

	hook1 := &lifecycleHook{
		onBeforeDispatch: func(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry) error {
			order.record("hook1-before")
			return nil
		},
		onAfterDispatch: func(ctx context.Context, req *dgw.GatewayRequest, resp *dgw.GatewayResponse) {
			order.record("hook1-after")
		},
	}

	hook2 := &lifecycleHook{
		onBeforeDispatch: func(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry) error {
			order.record("hook2-before")
			return nil
		},
		onAfterDispatch: func(ctx context.Context, req *dgw.GatewayRequest, resp *dgw.GatewayResponse) {
			order.record("hook2-after")
		},
	}

	hook3 := &lifecycleHook{
		onBeforeDispatch: func(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry) error {
			order.record("hook3-before")
			return nil
		},
		onAfterDispatch: func(ctx context.Context, req *dgw.GatewayRequest, resp *dgw.GatewayResponse) {
			order.record("hook3-after")
		},
	}

	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/test", WorkerID: "worker1"},
	})

	payload, _ := json.Marshal(dgw.WorkerResponse{
		StatusCode: 200,
		Body:       []byte(`{"ok":true}`),
	})
	transport := &mockTransport{
		respMsg: ipc.Message{Payload: payload},
	}

	d := apgw.NewDispatcher(
		routes,
		transport,
		&mockJWT{},
		&mockSchema{},
		5*time.Second,
		zap.NewNop(),
		lifecycle.NewWorkerDrainer(),
		apgw.WithLifecycleHooks(hook1, hook2, hook3),
	)

	_, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "GET", Path: "/test",
		Headers: map[string]string{"X-Request-Id": "test-cid"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := order.get()
	expected := []string{
		"hook1-before",
		"hook2-before",
		"hook3-before",
		"hook1-after",
		"hook2-after",
		"hook3-after",
	}

	if len(calls) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(calls), calls)
	}

	for i, exp := range expected {
		if calls[i] != exp {
			t.Errorf("call %d: expected %q, got %q", i, exp, calls[i])
		}
	}
}

func TestLifecycleHooks_ErrorShortCircuit(t *testing.T) {
	hook := &lifecycleHook{
		onBeforeDispatchErr: errors.New("hook blocked request"),
	}

	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "POST", Path: "/users", WorkerID: "worker1"},
	})

	transport := &mockTransport{}

	d := apgw.NewDispatcher(
		routes,
		transport,
		&mockJWT{},
		&mockSchema{},
		5*time.Second,
		zap.NewNop(),
		lifecycle.NewWorkerDrainer(),
		apgw.WithLifecycleHooks(hook),
	)

	_, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "POST", Path: "/users",
		Body:    []byte(`{"name":"test"}`),
		Headers: map[string]string{"X-Request-Id": "test-cid"},
	})

	if err == nil {
		t.Fatal("expected error from OnBeforeDispatch, got nil")
	}

	if err.Error() != "hook blocked request" {
		t.Errorf("expected 'hook blocked request', got %q", err.Error())
	}

	if transport.sendErr != nil {
		t.Error("request should not be sent to worker when OnBeforeDispatch returns error")
	}
}

func TestLifecycleHooks_OnWorkerError(t *testing.T) {
	errorCalled := false
	errorWorkerID := ""
	errorRequest := (*dgw.GatewayRequest)(nil)
	errorErr := error(nil)

	hook := &lifecycleHook{
		onWorkerError: func(ctx context.Context, workerID string, req *dgw.GatewayRequest, err error) {
			errorCalled = true
			errorWorkerID = workerID
			errorRequest = req
			errorErr = err
		},
	}

	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/test", WorkerID: "worker1"},
	})

	transport := &mockTransport{
		sendErr: errors.New("connection refused"),
	}

	d := apgw.NewDispatcher(
		routes,
		transport,
		&mockJWT{},
		&mockSchema{},
		5*time.Second,
		zap.NewNop(),
		lifecycle.NewWorkerDrainer(),
		apgw.WithLifecycleHooks(hook),
	)

	_, _ = d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "GET", Path: "/test",
		Headers: map[string]string{
			"X-Request-Id": "test-cid",
			"Authorization": "Bearer token",
		},
	})

	if !errorCalled {
		t.Fatal("OnWorkerError was not called")
	}

	if errorWorkerID != "worker1" {
		t.Errorf("expected workerID 'worker1', got %q", errorWorkerID)
	}

	if errorRequest == nil {
		t.Fatal("errorRequest is nil")
	}

	if errorRequest.Method != "GET" || errorRequest.Path != "/test" {
		t.Errorf("errorRequest has wrong values: method=%s, path=%s", errorRequest.Method, errorRequest.Path)
	}

	if errorErr == nil {
		t.Fatal("errorErr is nil")
	}
}

func TestLifecycleHooks_OnWorkerError_Timeout(t *testing.T) {
	errorCalled := false
	errorWorkerID := ""
	errorErr := error(nil)

	hook := &lifecycleHook{
		onWorkerError: func(ctx context.Context, workerID string, req *dgw.GatewayRequest, err error) {
			errorCalled = true
			errorWorkerID = workerID
			errorErr = err
		},
	}

	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/slow", WorkerID: "worker1"},
	})

	transport := &mockTransport{
		recvErr: errors.New("context deadline exceeded"),
	}

	d := apgw.NewDispatcher(
		routes,
		transport,
		&mockJWT{},
		&mockSchema{},
		100*time.Millisecond,
		zap.NewNop(),
		lifecycle.NewWorkerDrainer(),
		apgw.WithLifecycleHooks(hook),
	)

	_, _ = d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "GET", Path: "/slow",
		Headers: map[string]string{"X-Request-Id": "timeout-cid"},
	})

	if !errorCalled {
		t.Fatal("OnWorkerError was not called on timeout")
	}

	if errorWorkerID != "worker1" {
		t.Errorf("expected workerID 'worker1', got %q", errorWorkerID)
	}

	if errors.Is(errorErr, dgw.ErrUpstreamTimeout) {
		t.Log("correctly received ErrUpstreamTimeout")
	} else if errorErr != nil {
		t.Logf("error: %v", errorErr)
	}
}

func TestLifecycleHooks_MultipleHooks_Success(t *testing.T) {
	modifiedHeaders := false
	bodyTransformed := false

	hook1 := &lifecycleHook{
		onBeforeDispatch: func(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry) error {
			if req.Headers == nil {
				req.Headers = make(map[string]string)
			}
			req.Headers["X-Custom-Header"] = "from-hook1"
			modifiedHeaders = true
			return nil
		},
	}

	hook2 := &lifecycleHook{
		onBeforeDispatch: func(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry) error {
			var bodyMap map[string]interface{}
			if err := json.Unmarshal(req.Body, &bodyMap); err == nil {
				bodyMap["transformed"] = true
				updatedBody, _ := json.Marshal(bodyMap)
				req.Body = updatedBody
				bodyTransformed = true
			}
			return nil
		},
	}

	hook3 := &lifecycleHook{
		onAfterDispatch: func(ctx context.Context, req *dgw.GatewayRequest, resp *dgw.GatewayResponse) {
			var bodyMap map[string]interface{}
			if err := json.Unmarshal(resp.Body, &bodyMap); err == nil {
				if bodyMap["transformed"] == true {
					bodyMap["seenByHook3"] = true
					resp.Body, _ = json.Marshal(bodyMap)
				}
			}
		},
	}

	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "POST", Path: "/transform", WorkerID: "worker1"},
	})

	payload, _ := json.Marshal(dgw.WorkerResponse{
		StatusCode: 200,
		Body:       []byte(`{"data":"test","transformed":true}`),
	})
	transport := &mockTransport{
		respMsg: ipc.Message{Payload: payload},
	}

	d := apgw.NewDispatcher(
		routes,
		transport,
		&mockJWT{},
		&mockSchema{},
		5*time.Second,
		zap.NewNop(),
		lifecycle.NewWorkerDrainer(),
		apgw.WithLifecycleHooks(hook1, hook2, hook3),
	)

	resp, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "POST", Path: "/transform",
		Body:    []byte(`{"data":"test"}`),
		Headers: map[string]string{"X-Request-Id": "transform-cid"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if !modifiedHeaders {
		t.Error("hook1 did not modify headers")
	}

	if !bodyTransformed {
		t.Error("hook2 did not modify body")
	}

	var respBody map[string]interface{}
	if err := json.Unmarshal(resp.Body, &respBody); err == nil {
		if respBody["seenByHook3"] != true {
			t.Error("hook3 did not see transformed data")
		}
	}
}

func TestLifecycleHooks_AfterDispatchCalled(t *testing.T) {
	afterDispatchCalled := false

	hook := &lifecycleHook{
		onAfterDispatch: func(ctx context.Context, req *dgw.GatewayRequest, resp *dgw.GatewayResponse) {
			afterDispatchCalled = true
			if resp.StatusCode != 200 {
				t.Errorf("expected status 200 in OnAfterDispatch, got %d", resp.StatusCode)
			}
		},
	}

	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/test", WorkerID: "worker1"},
	})

	payload, _ := json.Marshal(dgw.WorkerResponse{
		StatusCode: 200,
		Body:       []byte(`{"ok":true}`),
	})
	transport := &mockTransport{
		respMsg: ipc.Message{Payload: payload},
	}

	d := apgw.NewDispatcher(
		routes,
		transport,
		&mockJWT{},
		&mockSchema{},
		5*time.Second,
		zap.NewNop(),
		lifecycle.NewWorkerDrainer(),
		apgw.WithLifecycleHooks(hook),
	)

	_, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "GET", Path: "/test",
		Headers: map[string]string{"X-Request-Id": "test-cid"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !afterDispatchCalled {
		t.Error("OnAfterDispatch was not called")
	}
}

func TestLifecycleHooks_HookModification_Persists(t *testing.T) {
	hook := &lifecycleHook{
		onBeforeDispatch: func(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry) error {
			if req.Headers == nil {
				req.Headers = make(map[string]string)
			}
			req.Headers["X-Modified"] = "true"
			req.Body = []byte(`{"modified":true}`)
			return nil
		},
	}

	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "POST", Path: "/echo", WorkerID: "worker1"},
	})

	payload, _ := json.Marshal(dgw.WorkerResponse{
		StatusCode: 200,
		Body:       []byte(`{"modified":true}`),
	})
	transport := &mockTransport{
		respMsg: ipc.Message{Payload: payload},
	}

	d := apgw.NewDispatcher(
		routes,
		transport,
		&mockJWT{},
		&mockSchema{},
		5*time.Second,
		zap.NewNop(),
		lifecycle.NewWorkerDrainer(),
		apgw.WithLifecycleHooks(hook),
	)

	_, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "POST", Path: "/echo",
		Body:    []byte(`{"original":true}`),
		Headers: map[string]string{"X-Request-Id": "test-cid"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLifecycleHooks_NoHooks(t *testing.T) {
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/test", WorkerID: "worker1"},
	})

	payload, _ := json.Marshal(dgw.WorkerResponse{
		StatusCode: 200,
		Body:       []byte(`{"ok":true}`),
	})
	transport := &mockTransport{
		respMsg: ipc.Message{Payload: payload},
	}

	d := apgw.NewDispatcher(
		routes,
		transport,
		&mockJWT{},
		&mockSchema{},
		5*time.Second,
		zap.NewNop(),
		lifecycle.NewWorkerDrainer(),
	)

	_, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "GET", Path: "/test",
		Headers: map[string]string{"X-Request-Id": "test-cid"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLifecycleHooks_ErrorInMiddleOfChain(t *testing.T) {
	order := &hookCallOrder{}

	hook1 := &lifecycleHook{
		onBeforeDispatch: func(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry) error {
			order.record("hook1-before")
			return nil
		},
	}

	hook2 := &lifecycleHook{
		onBeforeDispatchErr: errors.New("hook2 failed"),
	}

	hook3 := &lifecycleHook{
		onBeforeDispatch: func(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry) error {
			order.record("hook3-before")
			return nil
		},
	}

	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Method: "GET", Path: "/test", WorkerID: "worker1"},
	})

	transport := &mockTransport{}

	d := apgw.NewDispatcher(
		routes,
		transport,
		&mockJWT{},
		&mockSchema{},
		5*time.Second,
		zap.NewNop(),
		lifecycle.NewWorkerDrainer(),
		apgw.WithLifecycleHooks(hook1, hook2, hook3),
	)

	_, err := d.Dispatch(context.Background(), &dgw.GatewayRequest{
		Method: "GET", Path: "/test",
		Headers: map[string]string{"X-Request-Id": "test-cid"},
	})

	if err == nil {
		t.Fatal("expected error from hook2, got nil")
	}

	calls := order.get()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d: %v", len(calls), calls)
	}
	if calls[0] != "hook1-before" {
		t.Errorf("expected 'hook1-before', got %q", calls[0])
	}
}
