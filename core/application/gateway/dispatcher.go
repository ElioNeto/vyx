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
	"github.com/ElioNeto/vyx/core/domain/ipc"
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
}

// NewDispatcher creates a Dispatcher wired with all required dependencies.
func NewDispatcher(
	routes *dgw.RouteMap,
	transport ipc.Transport,
	jwt JWTValidator,
	schema SchemaValidator,
	timeout time.Duration,
	log *zap.Logger,
	opts ...DispatcherOption,
) *Dispatcher {
	d := &Dispatcher{
		routes:    routes,
		transport: transport,
		jwt:       jwt,
		schema:    schema,
		timeout:   timeout,
		log:       log,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// DispatcherOption configures a Dispatcher.
type DispatcherOption func(*Dispatcher)

// WithProxyListeners adds lifecycle listeners to the dispatcher.
func WithProxyListeners(listeners ...ProxyListener) DispatcherOption {
	return func(d *Dispatcher) {
		d.listeners = append(d.listeners, listeners...)
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

	// Propagate or generate a correlation ID (#40).
	correlationID := req.Headers["X-Request-Id"]
	if correlationID == "" {
		correlationID = uuid.NewString()
	}

	lc := NewLifecycleContext(req)
	lc.CorrelationID = correlationID
	userID := "-"
	statusCode := 0

	// Deferred access log emitted on every exit path (#40).
	defer func() {
		latency := time.Since(start)
		level := d.log.Info
		if statusCode >= 400 {
			level = d.log.Warn
		}
		level("access",
			zap.String("method", req.Method),
			zap.String("path", req.Path),
			zap.String("user_id", userID),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("correlation_id", correlationID),
		)
		// Notify post-listener hooks.
		for _, l := range d.listeners {
			if statusCode >= 400 || lc.Err != nil {
				l.OnError(lc, lc.Phase)
			}
			l.OnPostDispatch(lc, latency)
		}
	}()

	// 1. Route lookup (supports path params via trie, #36).
	result, ok := d.routes.Lookup(req.Method, req.Path)
	if !ok {
		statusCode = 404
		lc.StatusCode = 404
		lc.Err = dgw.ErrRouteNotFound
		lc.Phase = PhaseRouteMatch
		return nil, dgw.ErrRouteNotFound
	}
	route := result.Entry
	lc.Route = &route
	lc.Phase = PhaseRouteMatch
	req.Params = result.Params

	// 1a. OnRouteMatch hooks — allow middleware to short-circuit before auth.
	for _, l := range d.listeners {
		l.OnRouteMatch(lc)
		if resp, aborted := lc.EarlyResponse(); aborted {
			statusCode = resp.StatusCode
			return resp, lc.Err
		}
	}

	// 2. JWT validation (skip if no auth roles defined).
	if len(route.AuthRoles) > 0 {
		token := req.Headers["Authorization"]
		if token == "" {
			statusCode = 401
			lc.StatusCode = 401
			lc.Err = dgw.ErrUnauthorized
			lc.Phase = PhasePreDispatch
			return nil, dgw.ErrUnauthorized
		}
		// Strip "Bearer " prefix if present.
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
		claims, err := d.jwt.Validate(token)
		if err != nil {
			statusCode = 401
			lc.StatusCode = 401
			lc.Err = dgw.ErrUnauthorized
			lc.Phase = PhasePreDispatch
			return nil, dgw.ErrUnauthorized
		}
		req.Claims = claims
		userID = claims.UserID

		// 3. Role-based authorisation.
		if !hasRequiredRole(claims.Roles, route.AuthRoles) {
			statusCode = 403
			lc.StatusCode = 403
			lc.Err = dgw.ErrForbidden
			lc.Phase = PhasePreDispatch
			return nil, dgw.ErrForbidden
		}
	}

	// 4. JSON Schema validation.
	if route.Validate != "" && len(req.Body) > 0 {
		if err := d.schema.Validate(route.Validate, req.Body); err != nil {
			statusCode = 400
			lc.StatusCode = 400
			lc.Err = fmt.Errorf("%w: %v", dgw.ErrSchemaValidation, err)
			lc.Phase = PhasePreDispatch
			return nil, lc.Err
		}
	}

	// 5. OnPreDispatch hooks — right before the IPC send.
	lc.Phase = PhasePreDispatch
	for _, l := range d.listeners {
		l.OnPreDispatch(lc)
		if resp, aborted := lc.EarlyResponse(); aborted {
			statusCode = resp.StatusCode
			return resp, lc.Err
		}
	}

	// 6. Build IPC request payload and forward to worker.
	payload, err := json.Marshal(map[string]any{
		"method":         req.Method,
		"path":           req.Path,
		"headers":        req.Headers,
		"query":          req.Query,
		"params":         req.Params,
		"body":           req.Body,
		"claims":         req.Claims,
		"correlation_id": correlationID,
	})
	if err != nil {
		statusCode = 500
		return nil, fmt.Errorf("gateway: marshal ipc payload: %w", err)
	}

	dispatchCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	if err := d.transport.Send(dispatchCtx, route.WorkerID, ipc.Message{
		Type:    ipc.TypeRequest,
		Payload: payload,
	}); err != nil {
		statusCode = 502
		lc.StatusCode = 502
		lc.Err = fmt.Errorf("gateway: send to worker %s: %w", route.WorkerID, err)
		lc.Phase = PhasePreDispatch
		return nil, lc.Err
	}

	// 7. Wait for the worker response (via the demuxed response channel so
	// heartbeat frames are not consumed here).
	respMsg, err := d.transport.ReceiveResponse(dispatchCtx, route.WorkerID)
	if err != nil {
		if dispatchCtx.Err() != nil {
			statusCode = 504
			lc.StatusCode = 504
			lc.Err = dgw.ErrUpstreamTimeout
			lc.Phase = PhasePostDispatch
			return nil, dgw.ErrUpstreamTimeout
		}
		statusCode = 502
		lc.StatusCode = 502
		lc.Err = fmt.Errorf("gateway: receive from worker %s: %w", route.WorkerID, err)
		lc.Phase = PhasePostDispatch
		return nil, lc.Err
	}

	if respMsg.Type == ipc.TypeError {
		statusCode = 502
		lc.StatusCode = 502
		lc.Err = fmt.Errorf("worker error: %s", string(respMsg.Payload))
		return &dgw.GatewayResponse{StatusCode: 502, Body: respMsg.Payload}, nil
	}

	if respMsg.Type == ipc.TypeError {
		statusCode = 502
		return &dgw.GatewayResponse{StatusCode: 502, Body: respMsg.Payload}, nil
	}

	// 8. Decode the structured worker response envelope (#39).
	var workerResp dgw.WorkerResponse
	if err := json.Unmarshal(respMsg.Payload, &workerResp); err != nil {
		// Fallback: treat raw payload as 200 body for backwards compatibility.
		statusCode = 200
		return &dgw.GatewayResponse{StatusCode: 200, Body: respMsg.Payload}, nil
	}

	if workerResp.StatusCode == 0 {
		workerResp.StatusCode = 200
	}
	statusCode = workerResp.StatusCode
	lc.StatusCode = workerResp.StatusCode
	lc.Phase = PhasePostDispatch

	return &dgw.GatewayResponse{
		StatusCode: workerResp.StatusCode,
		Headers:    workerResp.Headers,
		Body:       workerResp.Body,
	}, nil
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
