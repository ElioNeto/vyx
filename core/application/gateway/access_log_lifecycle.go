package gateway

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

type AccessLogLifecycle struct {
	log      *zap.Logger
	startTimes sync.Map
}

func NewAccessLogLifecycle(log *zap.Logger) *AccessLogLifecycle {
	return &AccessLogLifecycle{log: log}
}

func (a *AccessLogLifecycle) OnBeforeDispatch(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry) error {
	correlationID := getCorrelationID(req)
	a.startTimes.Store(correlationID, time.Now())
	return nil
}

func (a *AccessLogLifecycle) OnAfterDispatch(ctx context.Context, req *dgw.GatewayRequest, resp *dgw.GatewayResponse) {
	correlationID := getCorrelationID(req)
	userID := getUserID(req)
	
	startI, ok := a.startTimes.LoadAndDelete(correlationID)
	if !ok {
		startI, _ = a.startTimes.Load(correlationID)
	}
	start := startI.(time.Time)
	latency := time.Since(start)

	level := a.log.Info
	if resp.StatusCode >= 400 {
		level = a.log.Warn
	}
	level("access",
		zap.String("method", req.Method),
		zap.String("path", req.Path),
		zap.String("user_id", userID),
		zap.Int("status", resp.StatusCode),
		zap.Duration("latency", latency),
		zap.String("correlation_id", correlationID),
	)
}

func (a *AccessLogLifecycle) OnWorkerError(ctx context.Context, workerID string, req *dgw.GatewayRequest, err error) {
	correlationID := getCorrelationID(req)
	a.startTimes.LoadAndDelete(correlationID)
	a.log.Error("worker-error",
		zap.String("worker_id", workerID),
		zap.String("path", req.Path),
		zap.Error(err),
		zap.String("correlation_id", correlationID),
	)
}

func getUserID(req *dgw.GatewayRequest) string {
	if req == nil || req.Claims == nil {
		return "-"
	}
	return req.Claims.UserID
}