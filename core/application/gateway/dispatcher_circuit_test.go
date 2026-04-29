package gateway_test

import (
	"context"
	"errors"
	"testing"
	"time"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/domain/circuit"
	"go.uber.org/zap"
)

// mockTransportForCircuit is a transport that can simulate various scenarios
type mockTransportForCircuit struct {
	sendErr     error
	recvErr     error
	recvMsg     ipc.Message
	sendCount   int
	recvCount   int
}

func (m *mockTransportForCircuit) Send(_ context.Context, _ string, _ ipc.Message) error {
	m.sendCount++
	return m.sendErr
}
func (m *mockTransportForCircuit) Receive(_ context.Context, _ string) (ipc.Message, error) {
	m.recvCount++
	return m.recvMsg, m.recvErr
}
func (m *mockTransportForCircuit) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	m.recvCount++
	return m.recvMsg, m.recvErr
}
func (m *mockTransportForCircuit) Register(_ context.Context, _ string) error   { return nil }
func (m *mockTransportForCircuit) Deregister(_ context.Context, _ string) error { return nil }
func (m *mockTransportForCircuit) Close() error                                  { return nil }

// TestCircuitBreakerOpen tests when circuit breaker is open
func TestCircuitBreakerOpen(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})
	
	transport := &mockTransportForCircuit{
		recvMsg: ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 200})},
	}
	
	// Create dispatcher with circuit config that opens quickly
	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	}, circuit.Config{Failures: 1, Cooldown: 1 * time.Hour}) // Long cooldown to keep it open

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{},
		Query:   map[string]string{},
	}

	// First request fails (send error) - records failure
	transport.sendErr = errors.New("connection refused")
	resp, err := d.Dispatch(context.Background(), req)
	if err == nil {
		t.Error("expected error on first failed request")
	}
	// Reset error for next request
	transport.sendErr = nil

	// Circuit should be open now - next request should get 503
	resp, err = d.Dispatch(context.Background(), req)
	if err == nil {
		t.Error("expected error when circuit is open")
	}
	if resp == nil || resp.StatusCode != 503 {
		t.Errorf("expected 503 when circuit is open, got %v", resp)
	}
}

// TestCircuitBreakerRecordResult tests success/failure recording
func TestCircuitBreakerRecordResult(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
		{WorkerID: "w2", Method: "POST", Path: "/ping"},
	})
	
	transport := &mockTransportForCircuit{
		recvMsg: ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 500})},
	}
	
	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	}, circuit.Config{Failures: 3})

	// Dispatch with 500 response - should record failure
	req500 := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{},
		Query:   map[string]string{},
	}
	resp, err := d.Dispatch(context.Background(), req500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	// Dispatch with 200 response - should record success
	transport.recvMsg = ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 200})}
	req200 := &dgw.GatewayRequest{
		Method:  "POST",
		Path:    "/ping",
		Headers: map[string]string{},
		Query:   map[string]string{},
	}
	resp, err = d.Dispatch(context.Background(), req200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestResolveCorrelationIDWorkerProvides tests when worker provides correlation ID
func TestResolveCorrelationIDWorkerProvides(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})
	
	// Worker response includes correlation ID
	transport := &mockTransportForCircuit{
		recvMsg: ipc.Message{
			Type: ipc.TypeResponse, 
			Payload: mustMarshal(t, dgw.WorkerResponse{
				StatusCode:     200,
				CorrelationID: "worker-provided-id",
			}),
		},
	}
	
	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	})

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{apgw.HeaderCorrelationID: "request-id"},
		Query:   map[string]string{},
	}
	resp, err := d.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Worker's correlation ID should be used
	if resp.CorrelationID != "worker-provided-id" {
		t.Errorf("expected worker-provided-id, got %q", resp.CorrelationID)
	}
}

// TestResolveCorrelationIDWorkerEmpty tests when worker doesn't provide correlation ID
func TestResolveCorrelationIDWorkerEmpty(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})
	
	// Worker response without correlation ID
	transport := &mockTransportForCircuit{
		recvMsg: ipc.Message{
			Type: ipc.TypeResponse, 
			Payload: mustMarshal(t, dgw.WorkerResponse{
				StatusCode: 200,
				// CorrelationID is empty
			}),
		},
	}
	
	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	})

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{apgw.HeaderCorrelationID: "request-id"},
		Query:   map[string]string{},
	}
	resp, err := d.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall back to request correlation ID
	if resp.CorrelationID != "request-id" {
		t.Errorf("expected request-id, got %q", resp.CorrelationID)
	}
}
