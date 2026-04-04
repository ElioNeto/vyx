package gateway

import (
	"testing"
	"time"

	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

func TestLifecycleContext_Abort(t *testing.T) {
	req := &dgw.GatewayRequest{Method: "GET", Path: "/test"}
	lc := NewLifecycleContext(req)
	lc.StatusCode = 500

	lc.Abort(dgw.ErrUpstreamTimeout)

	resp, ok := lc.EarlyResponse()
	if !ok {
		t.Fatal("expected early response flag to be true")
	}
	if lc.StatusCode != 500 {
		t.Errorf("status = %d, want 500", lc.StatusCode)
	}
	if resp == nil {
		t.Fatal("early response is nil")
	}
}

func TestLifecycleContext_RespondBeforeDispatch(t *testing.T) {
	req := &dgw.GatewayRequest{}
	lc := NewLifecycleContext(req)

	custom := &dgw.GatewayResponse{StatusCode: 418, Body: []byte("I'm a teapot")}
	lc.RespondBeforeDispatch(custom)

	if lc.StatusCode != 418 {
		t.Errorf("status = %d, want 418", lc.StatusCode)
	}
	resp, ok := lc.EarlyResponse()
	if !ok {
		t.Fatal("expected early response")
	}
	if resp.StatusCode != 418 {
		t.Errorf("resp status = %d, want 418", resp.StatusCode)
	}
}

func TestFuncListener_AllHooks(t *testing.T) {
	var phases []Phase

	listener := FuncListener{
		OnRouteMatchFn:  func(lc *LifecycleContext) { phases = append(phases, PhaseRouteMatch) },
		OnPreDispatchFn: func(lc *LifecycleContext) { phases = append(phases, PhasePreDispatch) },
		OnPostDispatchFn: func(lc *LifecycleContext, d time.Duration) {
			phases = append(phases, PhasePostDispatch)
		},
		OnErrorFn: func(lc *LifecycleContext, phase Phase) { phases = append(phases, PhaseError) },
	}

	lc := &LifecycleContext{
		Request:  &dgw.GatewayRequest{},
		Metadata: make(map[string]any),
	}

	listener.OnRouteMatch(lc)
	listener.OnPreDispatch(lc)
	listener.OnPostDispatch(lc, time.Millisecond)
	listener.OnError(lc, PhaseRouteMatch)

	want := []Phase{PhaseRouteMatch, PhasePreDispatch, PhasePostDispatch, PhaseError}
	if len(phases) != len(want) {
		t.Fatalf("phases = %v, want %v", phases, want)
	}
	for i := range want {
		if phases[i] != want[i] {
			t.Errorf("phase[%d] = %q, want %q", i, phases[i], want[i])
		}
	}
}

func TestFuncListener_ZeroValue(t *testing.T) {
	// A zero-value FuncListener must not panic on any hook call.
	var l FuncListener
	lc := &LifecycleContext{Request: &dgw.GatewayRequest{}}
	l.OnRouteMatch(lc)
	l.OnPreDispatch(lc)
	l.OnPostDispatch(lc, 0)
	l.OnError(lc, PhaseError)
}
