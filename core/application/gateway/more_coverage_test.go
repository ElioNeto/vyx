package gateway_test

import (
	"context"
	"testing"
	"time"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/application/lifecycle"
	"go.uber.org/zap"
)

// TestCheckDrainStatus_NotDraining tests when worker is not draining
func TestCheckDrainStatus_NotDraining(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})

	transport := &mockTransport{
		respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 200})},
	}

	drainer := lifecycle.NewWorkerDrainer()
	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
		Drainer:   drainer,
	})

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{},
		Query:   map[string]string{},
	}

	resp, err := d.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestCheckDrainStatus_Draining tests when worker is draining
func TestCheckDrainStatus_Draining(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})

	transport := &mockTransport{}
	drainer := lifecycle.NewWorkerDrainer()
	drainer.MarkDraining("w1")

	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
		Drainer:   drainer,
	})

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{},
		Query:   map[string]string{},
	}

	resp, err := d.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should get 503 Service Unavailable
	if resp.StatusCode != 503 {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

// TestLookupRoute_NotFound tests when route is not found
func TestLookupRoute_NotFound(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})

	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: &mockTransport{},
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	})

	req := &dgw.GatewayRequest{
		Method:  "POST", // Different method
		Path:    "/ping",
		Headers: map[string]string{},
		Query:   map[string]string{},
	}

	_, err := d.Dispatch(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for route not found")
	}
}

// TestValidateJWT_WithRoles tests JWT with roles
func TestValidateJWT_WithRoles(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping", AuthRoles: []string{"admin"}},
	})

	transport := &mockTransport{
		respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 200})},
	}

	// Caller has the required role
	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{claims: &dgw.Claims{UserID: "user1", Roles: []string{"admin"}}},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	})

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{"Authorization": "Bearer valid-token"},
		Query:   map[string]string{},
	}

	resp, err := d.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestValidateJWT_InsufficientRoles tests when caller lacks required role
func TestValidateJWT_InsufficientRoles(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping", AuthRoles: []string{"admin"}},
	})

	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: &mockTransport{},
		JWT:       &mockJWT{claims: &dgw.Claims{UserID: "user1", Roles: []string{"user"}}},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	})

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{"Authorization": "Bearer valid-token"},
		Query:   map[string]string{},
	}

	_, err := d.Dispatch(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for insufficient roles")
	}
}

// TestDispatch_WithCorrelationID tests correlation ID handling
func TestDispatch_WithCorrelationID(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})

	transport := &mockTransport{
		respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 200})},
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
		Headers: map[string]string{apgw.HeaderCorrelationID: "test-correlation-id"},
		Query:   map[string]string{},
	}

	resp, err := d.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CorrelationID != "test-correlation-id" {
		t.Errorf("expected correlation ID, got %q", resp.CorrelationID)
	}
}

// TestDispatch_NoRouteMatch tests when no route matches
func TestDispatch_NoRouteMatch(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})

	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: &mockTransport{},
		JWT:       &mockJWT{},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	})

	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/non-existent",
		Headers: map[string]string{},
		Query:   map[string]string{},
	}

	_, err := d.Dispatch(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for no route match")
	}
}

// TestStripBearerPrefix_WithBearer tests Bearer prefix removal
func TestStripBearerPrefix_WithBearer(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})

	transport := &mockTransport{
		respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 200})},
	}

	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{claims: &dgw.Claims{UserID: "user1"}},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	})

	// The stripBearerPrefix is called internally when processing Authorization header
	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{"Authorization": "Bearer mytoken"},
		Query:   map[string]string{},
	}

	resp, err := d.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestStripBearerPrefix_WithoutBearer tests token without Bearer prefix
func TestStripBearerPrefix_WithoutBearer(t *testing.T) {
	log, _ := zap.NewDevelopment()
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})

	transport := &mockTransport{
		respMsg: ipc.Message{Type: ipc.TypeResponse, Payload: mustMarshal(t, dgw.WorkerResponse{StatusCode: 200})},
	}

	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       &mockJWT{claims: &dgw.Claims{UserID: "user1"}},
		Schema:    &mockSchema{},
		Timeout:   2 * time.Second,
		Log:       log,
	})

	// Token without Bearer prefix
	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{"Authorization": "mytoken"},
		Query:   map[string]string{},
	}

	resp, err := d.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
