// Package gateway implements the HTTP gateway application-layer use cases:
// JWT validation, role authorisation, schema validation and worker dispatch.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/circuit"
	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/application/lifecycle"
)

// JWTValidator extracts and verifies a raw JWT string, returning the claims.
type JWTValidator interface {
	Validate(token string) (*dgw.Claims, error)
}

// SchemaValidator validates a JSON body against the named schema.
type SchemaValidator interface {
	Validate(schemaName string, body []byte) error
}

// Dispatcher orchestrates the full gateway pipeline for a single request.
type Dispatcher struct {
	routes    *dgw.RouteMap
	transport ipc.Transport
	jwt       JWTValidator
	schema    SchemaValidator
	timeout   time.Duration
	log       *zap.Logger
	listeners []ProxyListener
	drainer   *lifecycle.WorkerDrainer
	hooks     []RequestLifecycle
	circuit   *circuit.Registry
}

// DispatcherConfig groups the dependencies for NewDispatcher.
// Use this struct to avoid passing too many parameters.
type DispatcherConfig struct {
	Routes    *dgw.RouteMap
	Transport ipc.Transport
	JWT       JWTValidator
	Schema    SchemaValidator
	Timeout   time.Duration
	Log       *zap.Logger
	Drainer   *lifecycle.WorkerDrainer
}

// NewDispatcher creates a Dispatcher wired with all required dependencies.
// Use NewDispatcherFromParams or populate a DispatcherConfig struct for cleaner calls.
func NewDispatcher(cfg DispatcherConfig, opts ...interface{}) *Dispatcher {
	var circuitConfig circuit.Config
	var dispatcherOpts []DispatcherOption

	for _, opt := range opts {
		switch o := opt.(type) {
		case circuit.Config:
			circuitConfig = o
		case DispatcherOption:
			dispatcherOpts = append(dispatcherOpts, o)
		}
	}

	onStateChange := func(sc circuit.StateChange) {
		cfg.Log.Info("circuit breaker state change",
			zap.String("route_id", sc.RouteID),
			zap.String("from", sc.From.String()),
			zap.String("to", sc.To.String()),
			zap.String("reason", sc.Reason),
			zap.Time("time", sc.Time),
		)
	}
	d := &Dispatcher{
		routes:    cfg.Routes,
		transport: cfg.Transport,
		jwt:       cfg.JWT,
		schema:    cfg.Schema,
		timeout:   cfg.Timeout,
		log:       cfg.Log,
		drainer:   cfg.Drainer,
		hooks:     []RequestLifecycle{NewAccessLogLifecycle(cfg.Log)},
		circuit:   circuit.NewRegistry(circuitConfig, onStateChange),
	}
	for _, opt := range dispatcherOpts {
		opt(d)
	}
	return d
}

// NewDispatcherFromParams is a helper to create a Dispatcher using separate params.
// Deprecated: Use NewDispatcher with DispatcherConfig instead.
func NewDispatcherFromParams(cfg DispatcherConfig, circuitConfig circuit.Config, opts ...DispatcherOption) *Dispatcher {
	return NewDispatcher(cfg, circuitConfig, opts)
}

// DispatcherOption configures a Dispatcher.
type DispatcherOption func(*Dispatcher)

// WithProxyListeners adds lifecycle listeners to the dispatcher.
func WithProxyListeners(listeners ...ProxyListener) DispatcherOption {
	return func(d *Dispatcher) {
		d.listeners = append(d.listeners, listeners...)
	}
}

// WithLifecycleHooks adds RequestLifecycle hooks to the dispatcher.
func WithLifecycleHooks(hooks ...RequestLifecycle) DispatcherOption {
	return func(d *Dispatcher) {
		d.hooks = append(d.hooks, hooks...)
	}
}

// Routes returns the route map (used by the WebSocket proxy).
func (d *Dispatcher) Routes() *dgw.RouteMap { return d.routes }

// Transport returns the IPC transport (used by the WebSocket proxy).
func (d *Dispatcher) Transport() ipc.Transport { return d.transport }

// JWT returns the JWT validator (used by the WebSocket proxy).
func (d *Dispatcher) JWT() JWTValidator { return d.jwt }

// Timeout returns the dispatch timeout (used by the WebSocket proxy).
func (d *Dispatcher) Timeout() time.Duration { return d.timeout }

// Dispatch runs the full pipeline: JWT → route → auth → schema → UDS → response.
func (d *Dispatcher) Dispatch(ctx context.Context, req *dgw.GatewayRequest) (*dgw.GatewayResponse, error) {
	start := time.Now()
	correlationID := d.initCorrelationID(req)
	lc := NewLifecycleContext(req)
	lc.CorrelationID = correlationID
	statusCode := 0

	defer d.notifyListeners(lc, &statusCode, start)

	route, ok := d.lookupRoute(req, lc)
	if !ok {
		return nil, dgw.ErrRouteNotFound
	}
	req.Params = lc.RouteParams

	if resp, ok := d.checkCircuitBreaker(ctx, req, route, lc, &statusCode); !ok {
		return resp, lc.Err
	}

	if resp, ok := d.runRouteMatchHooks(ctx, req, lc, &statusCode); !ok {
		return resp, lc.Err
	}

	if resp, ok := d.checkDrainStatus(ctx, req, route, lc, &statusCode); !ok {
		return resp, lc.Err
	}

	d.trackInFlight(route.WorkerID, lc)

	if resp, ok := d.validateJWT(ctx, req, route, lc, &statusCode); !ok {
		d.releaseInFlight(lc)
		return resp, lc.Err
	}

	if resp, ok := d.validateSchema(ctx, req, route, lc, &statusCode); !ok {
		d.releaseInFlight(lc)
		return resp, lc.Err
	}

	if resp, ok := d.runPreDispatchHooks(ctx, req, *route, lc, &statusCode); !ok {
		d.releaseInFlight(lc)
		return resp, lc.Err
	}

	workerResp, resp, ok := d.sendAndReceive(ctx, req, route, lc, &statusCode)
	if !ok {
		d.releaseInFlight(lc)
		return resp, lc.Err
	}

	resp = d.processWorkerResponse(ctx, req, workerResp, lc, &statusCode, correlationID)
	d.releaseInFlight(lc)
	return resp, nil
}

// initCorrelationID initializes or generates a correlation ID for the request.
func (d *Dispatcher) initCorrelationID(req *dgw.GatewayRequest) string {
	correlationID := req.Headers["X-Request-Id"]
	if correlationID == "" {
		correlationID = uuid.NewString()
	}
	req.Headers["X-Request-Id"] = correlationID
	return correlationID
}

// notifyListeners defers the listener notifications.
func (d *Dispatcher) notifyListeners(lc *LifecycleContext, statusCode *int, start time.Time) {
	latency := time.Since(start)
	for _, l := range d.listeners {
		if *statusCode >= 400 || lc.Err != nil {
			l.OnError(lc, lc.Phase)
		}
		l.OnPostDispatch(lc, latency)
	}
}

// lookupRoute performs route lookup and returns the route entry.
func (d *Dispatcher) lookupRoute(req *dgw.GatewayRequest, lc *LifecycleContext) (*dgw.RouteEntry, bool) {
	result, ok := d.routes.Lookup(req.Method, req.Path)
	if !ok {
		lc.StatusCode = 404
		lc.Err = dgw.ErrRouteNotFound
		lc.Phase = PhaseRouteMatch
		return nil, false
	}
	route := result.Entry
	lc.Route = &route
	lc.Phase = PhaseRouteMatch
	lc.RouteParams = result.Params
	return &route, true
}

// checkCircuitBreaker checks if the circuit breaker allows the request.
func (d *Dispatcher) checkCircuitBreaker(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry, lc *LifecycleContext, statusCode *int) (*dgw.GatewayResponse, bool) {
	routeKey := fmt.Sprintf("%s:%s", req.Method, req.Path)
	cb := d.circuit.Get(routeKey)
	if !cb.Allow() {
		state, _, _ := cb.Stats()
		*statusCode = 503
		lc.StatusCode = 503
		lc.Err = fmt.Errorf("circuit breaker open for route %s (state: %s)", routeKey, state)
		lc.Phase = PhasePreDispatch
		return &dgw.GatewayResponse{
			StatusCode:    503,
			Headers:       map[string]string{"Retry-After": "30"},
			Body:          []byte(fmt.Sprintf(`{"error":"circuit breaker open","state":"%s"}`, state)),
			CorrelationID: req.Headers["X-Request-Id"],
		}, false
	}
	return nil, true
}

// runRouteMatchHooks runs the OnRouteMatch hooks.
func (d *Dispatcher) runRouteMatchHooks(ctx context.Context, req *dgw.GatewayRequest, lc *LifecycleContext, statusCode *int) (*dgw.GatewayResponse, bool) {
	for _, l := range d.listeners {
		l.OnRouteMatch(lc)
		if resp, aborted := lc.EarlyResponse(); aborted {
			*statusCode = resp.StatusCode
			resp.CorrelationID = req.Headers["X-Request-Id"]
			return resp, false
		}
	}
	return nil, true
}

// checkDrainStatus checks if the worker is draining.
func (d *Dispatcher) checkDrainStatus(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry, lc *LifecycleContext, statusCode *int) (*dgw.GatewayResponse, bool) {
	if d.drainer != nil && d.drainer.IsDraining(route.WorkerID) {
		*statusCode = 503
		lc.StatusCode = 503
		// Intentionally returning 503, don't set lc.Err to avoid confusing callers
		lc.Phase = PhasePreDispatch
		return &dgw.GatewayResponse{
			StatusCode:    503,
			Headers:       map[string]string{"Retry-After": "1"},
			Body:          []byte(`{"error":"worker draining"}`),
			CorrelationID: req.Headers["X-Request-Id"],
		}, false
	}
	return nil, true
}

// trackInFlight tracks the in-flight request for graceful draining.
func (d *Dispatcher) trackInFlight(workerID string, lc *LifecycleContext) {
	if d.drainer != nil {
		d.drainer.Acquire(workerID)
		lc.WorkerID = workerID
	}
}

// releaseInFlight releases the in-flight request tracking.
func (d *Dispatcher) releaseInFlight(lc *LifecycleContext) {
	if d.drainer != nil && lc.WorkerID != "" {
		d.drainer.Release(lc.WorkerID)
	}
}

// validateJWT handles JWT validation and authorization.
func (d *Dispatcher) validateJWT(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry, lc *LifecycleContext, statusCode *int) (*dgw.GatewayResponse, bool) {
	if len(route.AuthRoles) == 0 {
		return nil, true
	}

	token := req.Headers["Authorization"]
	if token == "" {
		*statusCode = 401
		lc.StatusCode = 401
		lc.Err = dgw.ErrUnauthorized
		lc.Phase = PhasePreDispatch
		return nil, false
	}

	token = d.stripBearerPrefix(token)
	claims, err := d.jwt.Validate(token)
	if err != nil {
		*statusCode = 401
		lc.StatusCode = 401
		lc.Err = dgw.ErrUnauthorized
		lc.Phase = PhasePreDispatch
		return nil, false
	}
	req.Claims = claims

	if !hasRequiredRole(claims.Roles, route.AuthRoles) {
		*statusCode = 403
		lc.StatusCode = 403
		lc.Err = dgw.ErrForbidden
		lc.Phase = PhasePreDispatch
		return nil, false
	}

	return nil, true
}

// stripBearerPrefix removes the "Bearer " prefix from a token if present.
func (d *Dispatcher) stripBearerPrefix(token string) string {
	if len(token) > 7 && token[:7] == "Bearer " {
		return token[7:]
	}
	return token
}

// validateSchema handles JSON Schema validation.
func (d *Dispatcher) validateSchema(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry, lc *LifecycleContext, statusCode *int) (*dgw.GatewayResponse, bool) {
	if route.Validate != "" && len(req.Body) > 0 {
		if err := d.schema.Validate(route.Validate, req.Body); err != nil {
			*statusCode = 400
			lc.StatusCode = 400
			lc.Err = fmt.Errorf("%w: %v", dgw.ErrSchemaValidation, err)
			lc.Phase = PhasePreDispatch
			return nil, false
		}
	}
	return nil, true
}

// runPreDispatchHooks runs the OnBeforeDispatch hooks.
func (d *Dispatcher) runPreDispatchHooks(ctx context.Context, req *dgw.GatewayRequest, route dgw.RouteEntry, lc *LifecycleContext, statusCode *int) (*dgw.GatewayResponse, bool) {
	lc.Phase = PhasePreDispatch
	for _, hook := range d.hooks {
		if err := hook.OnBeforeDispatch(ctx, req, &route); err != nil {
			*statusCode = 400
			lc.StatusCode = 400
			lc.Err = err
			return nil, false
		}
	}
	return nil, true
}

// sendAndReceive sends the request to the worker and receives the response.
func (d *Dispatcher) sendAndReceive(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry, lc *LifecycleContext, statusCode *int) (*dgw.WorkerResponse, *dgw.GatewayResponse, bool) {
	correlationID := req.Headers["X-Request-Id"]
	cb := d.circuit.Get(fmt.Sprintf("%s:%s", req.Method, req.Path))

	payload, err := d.buildIPCPayload(req, correlationID)
	if err != nil {
		*statusCode = 500
		return nil, nil, false
	}

	dispatchCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	if err := d.transport.Send(dispatchCtx, route.WorkerID, ipc.Message{
		Type:    ipc.TypeRequest,
		Payload: payload,
	}); err != nil {
		return d.handleSendError(ctx, req, route, lc, statusCode, err, cb)
	}

	respMsg, err := d.transport.ReceiveResponse(dispatchCtx, route.WorkerID)
	if err != nil {
		return d.handleReceiveError(ctx, req, route, lc, statusCode, err, dispatchCtx, cb)
	}

	if respMsg.Type == ipc.TypeError {
		return d.handleWorkerError(ctx, req, route, lc, statusCode, respMsg, cb)
	}

	return d.decodeWorkerResponse(respMsg, correlationID)
}

// buildIPCPayload builds the IPC request payload.
func (d *Dispatcher) buildIPCPayload(req *dgw.GatewayRequest, correlationID string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"method":         req.Method,
		"path":           req.Path,
		"headers":        req.Headers,
		"query":          req.Query,
		"params":         req.Params,
		"body":           req.Body,
		"claims":         req.Claims,
		"correlation_id": correlationID,
	})
}

// handleSendError handles errors when sending to the worker fails.
func (d *Dispatcher) handleSendError(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry, lc *LifecycleContext, statusCode *int, err error, cb *circuit.Breaker) (*dgw.WorkerResponse, *dgw.GatewayResponse, bool) {
	*statusCode = 502
	lc.StatusCode = 502
	lc.Err = fmt.Errorf("gateway: send to worker %s: %w", route.WorkerID, err)
	lc.Phase = PhasePreDispatch
	for _, hook := range d.hooks {
		hook.OnWorkerError(ctx, route.WorkerID, req, lc.Err)
	}
	cb.RecordFailure()
	return nil, nil, false
}

// handleReceiveError handles errors when receiving from the worker fails.
func (d *Dispatcher) handleReceiveError(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry, lc *LifecycleContext, statusCode *int, err error, dispatchCtx context.Context, cb *circuit.Breaker) (*dgw.WorkerResponse, *dgw.GatewayResponse, bool) {
	if dispatchCtx.Err() != nil {
		*statusCode = 504
		lc.StatusCode = 504
		lc.Err = dgw.ErrUpstreamTimeout
		lc.Phase = PhasePostDispatch
	} else {
		*statusCode = 502
		lc.StatusCode = 502
		lc.Err = fmt.Errorf("gateway: receive from worker %s: %w", route.WorkerID, err)
		lc.Phase = PhasePostDispatch
	}
	for _, hook := range d.hooks {
		hook.OnWorkerError(ctx, route.WorkerID, req, lc.Err)
	}
	cb.RecordFailure()
	return nil, nil, false
}

// handleWorkerError handles error responses from the worker.
func (d *Dispatcher) handleWorkerError(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry, lc *LifecycleContext, statusCode *int, respMsg ipc.Message, cb *circuit.Breaker) (*dgw.WorkerResponse, *dgw.GatewayResponse, bool) {
	*statusCode = 502
	lc.StatusCode = 502
	lc.Err = fmt.Errorf("worker error: %s", string(respMsg.Payload))
	for _, hook := range d.hooks {
		hook.OnWorkerError(ctx, route.WorkerID, req, lc.Err)
	}
	cb.RecordFailure()
	return nil, &dgw.GatewayResponse{
		StatusCode:    502,
		Body:          respMsg.Payload,
		CorrelationID: req.Headers["X-Request-Id"],
	}, false
}

// decodeWorkerResponse decodes the worker response.
func (d *Dispatcher) decodeWorkerResponse(respMsg ipc.Message, correlationID string) (*dgw.WorkerResponse, *dgw.GatewayResponse, bool) {
	var workerResp dgw.WorkerResponse
	if err := json.Unmarshal(respMsg.Payload, &workerResp); err != nil {
		return d.handleDecodeError(respMsg, correlationID)
	}
	if workerResp.StatusCode == 0 {
		workerResp.StatusCode = 200
	}
	return &workerResp, nil, true
}

// handleDecodeError handles errors when decoding the worker response.
func (d *Dispatcher) handleDecodeError(respMsg ipc.Message, correlationID string) (*dgw.WorkerResponse, *dgw.GatewayResponse, bool) {
	resp := &dgw.GatewayResponse{
		StatusCode:    200,
		Body:          respMsg.Payload,
		CorrelationID: correlationID,
	}
	return nil, resp, false
}

// processWorkerResponse processes the worker response and returns the final gateway response.
func (d *Dispatcher) processWorkerResponse(ctx context.Context, req *dgw.GatewayRequest, workerResp *dgw.WorkerResponse, lc *LifecycleContext, statusCode *int, correlationID string) *dgw.GatewayResponse {
	*statusCode = workerResp.StatusCode
	lc.StatusCode = workerResp.StatusCode
	lc.Phase = PhasePostDispatch

	respCorrelationID := d.resolveCorrelationID(workerResp.CorrelationID, correlationID)

	resp := &dgw.GatewayResponse{
		StatusCode:    workerResp.StatusCode,
		Headers:       workerResp.Headers,
		Body:          workerResp.Body,
		CorrelationID: respCorrelationID,
	}
	for _, hook := range d.hooks {
		hook.OnAfterDispatch(ctx, req, resp)
	}

	d.recordCircuitBreakerResult(req, workerResp.StatusCode)
	return resp
}

// resolveCorrelationID returns the worker's correlation ID or falls back to the request's.
func (d *Dispatcher) resolveCorrelationID(workerCorrelationID, requestCorrelationID string) string {
	if workerCorrelationID != "" {
		return workerCorrelationID
	}
	return requestCorrelationID
}

// recordCircuitBreakerResult records success or failure in the circuit breaker.
func (d *Dispatcher) recordCircuitBreakerResult(req *dgw.GatewayRequest, statusCode int) {
	cb := d.circuit.Get(fmt.Sprintf("%s:%s", req.Method, req.Path))
	if statusCode >= 500 {
		cb.RecordFailure()
	} else {
		cb.RecordSuccess()
	}
}

// hasRequiredRole returns true if the caller holds at least one required role.
func hasRequiredRole(callerRoles, requiredRoles []string) bool {
	set := make(map[string]struct{}, len(callerRoles))
	for _, r := range callerRoles {
		set[r] = struct{}{}
	}
	for _, r := range requiredRoles {
		if _, ok := set[r]; ok {
			return true
		}
	}
	return false
}
