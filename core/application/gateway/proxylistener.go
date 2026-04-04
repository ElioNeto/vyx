package gateway

import (
	"time"

	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

// Phase identifies the exact point in the dispatch lifecycle where a hook was
// invoked. It is exposed to ProxyListener callbacks so a single implementation
// can multiplex all four phases through one method.
type Phase string

const (
	PhaseRouteMatch  Phase = "route_match"  // after lookup, before auth
	PhasePreDispatch Phase = "pre_dispatch"  // after all validation, before IPC send
	PhasePostDispatch Phase = "post_dispatch" // after IPC response received
	PhaseError       Phase = "error"         // any error path
)

// LifecycleContext carries the mutable state of a single request through
// the dispatch pipeline.  ProxyListener hooks receive a pointer so they
// may inspect, mutate fields, or call Abort to short-circuit the pipeline.
type LifecycleContext struct {
	Request       *dgw.GatewayRequest
	Route         *dgw.RouteEntry
	CorrelationID string
	Phase         Phase

	// Err is nil on the happy path; set when the Phase is PhaseError or
	// when a listener calls Abort with an error.
	Err error

	// StatusCode is populated as the pipeline runs (404, 401, 200, …).
	// A listener may overwrite it via RespondBeforeDispatch.
	StatusCode int

	// Metadata is a free-form bag that listeners may use to pass data
	// between hooks (e.g. a middleware timer, enriched user info).
	Metadata map[string]any

	respondEarly bool
	earlyResp    *dgw.GatewayResponse
}

// NewLifecycleContext creates a context wired to the incoming request.
func NewLifecycleContext(req *dgw.GatewayRequest) *LifecycleContext {
	return &LifecycleContext{
		Request:  req,
		Metadata: make(map[string]any),
	}
}

// Abort immediately flag the pipeline for early termination with the given error.
func (lc *LifecycleContext) Abort(err error) {
	lc.Err = err
	lc.respondEarly = true
	lc.earlyResp = &dgw.GatewayResponse{StatusCode: lc.StatusCode, Body: []byte(err.Error())}
}

// RespondBeforeDispatch returns the given response instead of continuing with
// the normal pipeline. Use this for short-circuit middleware behaviour such as
// rate-limiting, caching, or maintenance-mode responses.
func (lc *LifecycleContext) RespondBeforeDispatch(resp *dgw.GatewayResponse) {
	lc.respondEarly = true
	lc.earlyResp = resp
	lc.StatusCode = resp.StatusCode
}

// EarlyResponse tells the Dispatcher that a listener short-circuited the
// pipeline and the returned GatewayResponse should be used as-is.
func (lc *LifecycleContext) EarlyResponse() (*dgw.GatewayResponse, bool) {
	if lc.respondEarly {
		return lc.earlyResp, true
	}
	return nil, false
}

// ProxyListener defines a hook that is called at multiple points during the
// request dispatch pipeline.  Every method is optional — the Dispatcher only
// calls a phase if it exists (zero-value struct is a no-op).
type ProxyListener interface {
	// OnRouteMatch is called immediately after a successful route lookup.
	// Useful for logging, metrics, rate-limiting keyed by route.
	// lc.Err and lc.Abort() can short-circuit the pipeline before auth.
	OnRouteMatch(lc *LifecycleContext)

	// OnPreDispatch is called after JWT validation, role check, and schema
	// validation — right before the IPC payload is sent to the worker.
	// Useful for injecting additional headers, request mutation, or final
	// guard checks (maintenance mode, feature flags, quota).
	OnPreDispatch(lc *LifecycleContext)

	// OnPostDispatch is called after the worker response has been decoded.
	// The response is available via lc.StatusCode / lc.Request.
	// Useful for access logging, metrics, response mutation, audit trails.
	OnPostDispatch(lc *LifecycleContext, duration time.Duration)

	// OnError is called whenever the pipeline encounters an error.
	// The phase parameter indicates which stage the error occurred in.
	OnError(lc *LifecycleContext, phase Phase)
}

// FuncListener is a convenience implementation that turns simple functions
// into a ProxyListener — only set the fields you care about, the rest are
// no-ops.
type FuncListener struct {
	OnRouteMatchFn  func(lc *LifecycleContext)
	OnPreDispatchFn func(lc *LifecycleContext)
	OnPostDispatchFn func(lc *LifecycleContext, duration time.Duration)
	OnErrorFn       func(lc *LifecycleContext, phase Phase)
}

// OnRouteMatch implements ProxyListener.
func (fl FuncListener) OnRouteMatch(lc *LifecycleContext) {
	if fl.OnRouteMatchFn != nil {
		fl.OnRouteMatchFn(lc)
	}
}

// OnPreDispatch implements ProxyListener.
func (fl FuncListener) OnPreDispatch(lc *LifecycleContext) {
	if fl.OnPreDispatchFn != nil {
		fl.OnPreDispatchFn(lc)
	}
}

// OnPostDispatch implements ProxyListener.
func (fl FuncListener) OnPostDispatch(lc *LifecycleContext, duration time.Duration) {
	if fl.OnPostDispatchFn != nil {
		fl.OnPostDispatchFn(lc, duration)
	}
}

// OnError implements ProxyListener.
func (fl FuncListener) OnError(lc *LifecycleContext, phase Phase) {
	if fl.OnErrorFn != nil {
		fl.OnErrorFn(lc, phase)
	}
}
