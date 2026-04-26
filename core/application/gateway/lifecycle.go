package gateway

import (
	"context"

	"go.uber.org/zap"

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

// AccessLogLifecycle logs every request/response to the provided logger.
type AccessLogLifecycle struct {
	log *zap.Logger
}

// NewAccessLogLifecycle creates a new AccessLogLifecycle hook.
func NewAccessLogLifecycle(log *zap.Logger) *AccessLogLifecycle {
	return &AccessLogLifecycle{log: log}
}

// OnBeforeDispatch logs the incoming request before dispatching to worker.
func (a *AccessLogLifecycle) OnBeforeDispatch(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry) error {
	a.log.Info("before-dispatch",
		zap.String("method", req.Method),
		zap.String("path", req.Path),
		zap.String("worker", route.WorkerID),
		zap.String("correlation_id", getCorrelationID(req)),
	)
	return nil
}

// OnAfterDispatch logs the response after receiving from worker.
func (a *AccessLogLifecycle) OnAfterDispatch(ctx context.Context, req *dgw.GatewayRequest, resp *dgw.GatewayResponse) {
	a.log.Info("after-dispatch",
		zap.String("method", req.Method),
		zap.String("path", req.Path),
		zap.Int("status", resp.StatusCode),
		zap.String("correlation_id", getCorrelationID(req)),
	)
}

// OnWorkerError logs when a worker returns an error or times out.
func (a *AccessLogLifecycle) OnWorkerError(ctx context.Context, workerID string, req *dgw.GatewayRequest, err error) {
	a.log.Warn("worker-error",
		zap.String("worker", workerID),
		zap.String("path", req.Path),
		zap.Error(err),
		zap.String("correlation_id", getCorrelationID(req)),
	)
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
