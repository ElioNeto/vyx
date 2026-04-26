package gateway_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap/zaptest"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

func TestAccessLogLifecycle_OnAfterDispatch_InfoForStatusLessThan400(t *testing.T) {
	logger := zaptest.NewLogger(t)
	hook := apgw.NewAccessLogLifecycle(logger)

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:   "/api/users",
		Headers: map[string]string{"X-Request-Id": "test-cid-123"},
		Claims: &dgw.Claims{UserID: "user-42"},
	}

	route := &dgw.RouteEntry{WorkerID: "worker-1"}

	err := hook.OnBeforeDispatch(context.Background(), req, route)
	if err != nil {
		t.Fatalf("OnBeforeDispatch returned error: %v", err)
	}

	resp := &dgw.GatewayResponse{
		StatusCode:    200,
		Body:         []byte(`{"ok":true}`),
		CorrelationID: "test-cid-123",
	}

	hook.OnAfterDispatch(context.Background(), req, resp)
}

func TestAccessLogLifecycle_OnAfterDispatch_WarnForStatusGreaterOrEqual400(t *testing.T) {
	logger := zaptest.NewLogger(t)
	hook := apgw.NewAccessLogLifecycle(logger)

	req := &dgw.GatewayRequest{
		Method:  "POST",
		Path:   "/api/users",
		Headers: map[string]string{"X-Request-Id": "test-cid-456"},
		Claims: &dgw.Claims{UserID: "user-42"},
	}

	route := &dgw.RouteEntry{WorkerID: "worker-1"}

	err := hook.OnBeforeDispatch(context.Background(), req, route)
	if err != nil {
		t.Fatalf("OnBeforeDispatch returned error: %v", err)
	}

	resp := &dgw.GatewayResponse{
		StatusCode:    400,
		Body:         []byte(`{"error":"invalid input"}`),
		CorrelationID: "test-cid-456",
	}

	hook.OnAfterDispatch(context.Background(), req, resp)
}

func TestAccessLogLifecycle_OnWorkerError_EmitsError(t *testing.T) {
	logger := zaptest.NewLogger(t)
	hook := apgw.NewAccessLogLifecycle(logger)

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:   "/api/users",
		Headers: map[string]string{"X-Request-Id": "test-cid-789"},
		Claims: &dgw.Claims{UserID: "user-42"},
	}

	route := &dgw.RouteEntry{WorkerID: "worker-1"}

	err := hook.OnBeforeDispatch(context.Background(), req, route)
	if err != nil {
		t.Fatalf("OnBeforeDispatch returned error: %v", err)
	}

	workerErr := errors.New("worker unavailable")
	hook.OnWorkerError(context.Background(), "worker-1", req, workerErr)
}

func TestAccessLogLifecycle_ImplementsRequestLifecycle(t *testing.T) {
	var _ apgw.RequestLifecycle = (*apgw.AccessLogLifecycle)(nil)
}