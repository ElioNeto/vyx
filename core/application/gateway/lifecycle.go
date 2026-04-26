package gateway

import (
	"context"

	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

// RequestLifecycle allows injecting logic at key points of the gateway pipeline.
// Implementations are called in order; returning an error from OnBeforeDispatch
// short-circuits the pipeline and returns the error to the client.
type RequestLifecycle interface {
	// OnBeforeDispatch is called after route lookup and auth, before sending to worker.
	// Returning an error aborts the request and returns the error to the client.
	OnBeforeDispatch(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry) error

	// OnAfterDispatch is called after the worker response is received.
	OnAfterDispatch(ctx context.Context, req *dgw.GatewayRequest, resp *dgw.GatewayResponse)

	// OnWorkerError is called when the worker returns an error or times out.
	OnWorkerError(ctx context.Context, workerID string, req *dgw.GatewayRequest, err error)
}

// getCorrelationID extracts the correlation ID from request headers.
func getCorrelationID(req *dgw.GatewayRequest) string {
	if req == nil {
		return "-"
	}
	if cid := req.Headers["X-Request-Id"]; cid != "" {
		return cid
	}
	return "-"
}
