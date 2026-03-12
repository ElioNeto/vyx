// Package gateway implements the HTTP gateway application-layer use cases:
// JWT validation, role authorisation, schema validation and worker dispatch.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
}

// NewDispatcher creates a Dispatcher wired with all required dependencies.
func NewDispatcher(
	routes *dgw.RouteMap,
	transport ipc.Transport,
	jwt JWTValidator,
	schema SchemaValidator,
	timeout time.Duration,
	log *zap.Logger,
) *Dispatcher {
	return &Dispatcher{
		routes:    routes,
		transport: transport,
		jwt:       jwt,
		schema:    schema,
		timeout:   timeout,
		log:       log,
	}
}

// Dispatch runs the full pipeline: JWT → route → auth → schema → UDS → response.
func (d *Dispatcher) Dispatch(ctx context.Context, req *dgw.GatewayRequest) (*dgw.GatewayResponse, error) {
	// 1. Route lookup.
	route, ok := d.routes.Lookup(req.Method, req.Path)
	if !ok {
		return nil, dgw.ErrRouteNotFound
	}

	// 2. JWT validation (skip if no auth roles defined).
	if len(route.AuthRoles) > 0 {
		token := req.Headers["Authorization"]
		if token == "" {
			return nil, dgw.ErrUnauthorized
		}
		// Strip "Bearer " prefix if present.
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
		claims, err := d.jwt.Validate(token)
		if err != nil {
			return nil, dgw.ErrUnauthorized
		}
		req.Claims = claims

		// 3. Role-based authorisation.
		if !hasRequiredRole(claims.Roles, route.AuthRoles) {
			return nil, dgw.ErrForbidden
		}
	}

	// 4. JSON Schema validation.
	if route.Validate != "" && len(req.Body) > 0 {
		if err := d.schema.Validate(route.Validate, req.Body); err != nil {
			return nil, fmt.Errorf("%w: %v", dgw.ErrSchemaValidation, err)
		}
	}

	// 5. Build IPC request payload and forward to worker.
	payload, err := json.Marshal(map[string]any{
		"method":  req.Method,
		"path":    req.Path,
		"headers": req.Headers,
		"body":    req.Body,
		"claims":  req.Claims,
	})
	if err != nil {
		return nil, fmt.Errorf("gateway: marshal ipc payload: %w", err)
	}

	dispatchCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	if err := d.transport.Send(dispatchCtx, route.WorkerID, ipc.Message{
		Type:    ipc.TypeRequest,
		Payload: payload,
	}); err != nil {
		return nil, fmt.Errorf("gateway: send to worker %s: %w", route.WorkerID, err)
	}

	// 6. Wait for the worker response.
	respMsg, err := d.transport.Receive(dispatchCtx, route.WorkerID)
	if err != nil {
		if dispatchCtx.Err() != nil {
			return nil, dgw.ErrUpstreamTimeout
		}
		return nil, fmt.Errorf("gateway: receive from worker %s: %w", route.WorkerID, err)
	}

	if respMsg.Type == ipc.TypeError {
		return &dgw.GatewayResponse{StatusCode: 502, Body: respMsg.Payload}, nil
	}

	d.log.Debug("request dispatched",
		zap.String("method", req.Method),
		zap.String("path", req.Path),
		zap.String("worker", route.WorkerID),
	)

	return &dgw.GatewayResponse{StatusCode: 200, Body: respMsg.Payload}, nil
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
