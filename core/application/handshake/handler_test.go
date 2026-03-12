package handshake_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ElioNeto/vyx/core/application/handshake"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
)

// ─── fakes ───────────────────────────────────────────────────────────────────

type fakeTransport struct {
	msg ipc.Message
	err error
}

func (f *fakeTransport) Receive(_ context.Context, _ string) (ipc.Message, error) {
	return f.msg, f.err
}

type fakeService struct {
	markedRunning []string
	err           error
}

func (f *fakeService) MarkRunning(_ context.Context, workerID string) error {
	f.markedRunning = append(f.markedRunning, workerID)
	return f.err
}

func handshakeMsg(t *testing.T, payload ipc.HandshakePayload) ipc.Message {
	t.Helper()
	b, _ := json.Marshal(payload)
	return ipc.Message{Type: ipc.TypeHandshake, Payload: b}
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestHandler_HappyPath(t *testing.T) {
	payload := ipc.HandshakePayload{
		WorkerID: "node:api",
		Capabilities: []ipc.HandshakeCapability{
			{Path: "/api/products", Method: "GET"},
		},
	}
	transport := &fakeTransport{msg: handshakeMsg(t, payload)}
	svc := &fakeService{}
	routes := dgw.NewRouteMap([]dgw.RouteEntry{
		{Path: "/api/products", Method: "GET", WorkerID: "node:api"},
	})
	h := handshake.NewHandler(transport, routes, svc, zap.NewNop())

	if err := h.Handle(context.Background(), "node:api"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svc.markedRunning) != 1 || svc.markedRunning[0] != "node:api" {
		t.Errorf("expected MarkRunning(node:api), got %v", svc.markedRunning)
	}
}

func TestHandler_MismatchedRoute_WarnOnly(t *testing.T) {
	payload := ipc.HandshakePayload{
		WorkerID: "node:api",
		Capabilities: []ipc.HandshakeCapability{
			{Path: "/api/unknown", Method: "POST"},
		},
	}
	transport := &fakeTransport{msg: handshakeMsg(t, payload)}
	svc := &fakeService{}
	routes := dgw.NewRouteMap(nil) // empty route map
	h := handshake.NewHandler(transport, routes, svc, zap.NewNop())

	// Should NOT return an error — mismatches are warnings.
	if err := h.Handle(context.Background(), "node:api"); err != nil {
		t.Fatalf("unexpected error on mismatch: %v", err)
	}
	if len(svc.markedRunning) != 1 {
		t.Error("expected MarkRunning to be called despite route mismatch")
	}
}

func TestHandler_Timeout(t *testing.T) {
	transport := &fakeTransport{err: errors.New("timeout")}
	svc := &fakeService{}
	routes := dgw.NewRouteMap(nil)
	h := handshake.NewHandler(transport, routes, svc, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	err := h.Handle(ctx, "node:api")
	if err == nil {
		t.Fatal("expected error on transport failure")
	}
	if len(svc.markedRunning) != 0 {
		t.Error("MarkRunning should not be called on handshake failure")
	}
}

func TestHandler_MalformedPayload(t *testing.T) {
	transport := &fakeTransport{msg: ipc.Message{
		Type:    ipc.TypeHandshake,
		Payload: []byte("not-json"),
	}}
	svc := &fakeService{}
	routes := dgw.NewRouteMap(nil)
	h := handshake.NewHandler(transport, routes, svc, zap.NewNop())

	err := h.Handle(context.Background(), "node:api")
	if err == nil {
		t.Fatal("expected error for malformed payload")
	}
}

func TestHandler_WrongMessageType(t *testing.T) {
	transport := &fakeTransport{msg: ipc.Message{Type: ipc.TypeHeartbeat, Payload: []byte{}}}
	svc := &fakeService{}
	routes := dgw.NewRouteMap(nil)
	h := handshake.NewHandler(transport, routes, svc, zap.NewNop())

	err := h.Handle(context.Background(), "node:api")
	if err == nil {
		t.Fatal("expected error for non-handshake frame")
	}
}
