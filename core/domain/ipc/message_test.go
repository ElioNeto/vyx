package ipc_test

import (
	"testing"

	"github.com/ElioNeto/vyx/core/domain/ipc"
)

func TestMessageType_String(t *testing.T) {
	tests := []struct {
		mt     ipc.MessageType
		expect string
	}{
		{ipc.TypeRequest, "request"},
		{ipc.TypeResponse, "response"},
		{ipc.TypeHeartbeat, "heartbeat"},
		{ipc.TypeError, "error"},
		{ipc.TypeHandshake, "handshake"},
		{ipc.TypeWSOpen, "ws_open"},
		{ipc.TypeWSMessage, "ws_message"},
		{ipc.TypeWSClose, "ws_close"},
		{ipc.MessageType(0xFF), "unknown(0xff)"},
	}

	for _, tc := range tests {
		if got := tc.mt.String(); got != tc.expect {
			t.Errorf("expected %s, got %s", tc.expect, got)
		}
	}
}

func TestMessage_DefaultMaxPayloadSize(t *testing.T) {
	if ipc.MaxPayloadSize != 16<<20 {
		t.Errorf("expected 16MiB, got %d", ipc.MaxPayloadSize)
	}
}

func TestHandshakePayload(t *testing.T) {
	payload := ipc.HandshakePayload{
		WorkerID: "worker1",
		Capabilities: []ipc.HandshakeCapability{
			{Path: "/api/users", Method: "GET"},
		},
	}

	if payload.WorkerID != "worker1" {
		t.Error("worker ID not set")
	}
	if len(payload.Capabilities) != 1 {
		t.Errorf("expected 1 capability, got %d", len(payload.Capabilities))
	}
}

func TestWSOpenPayload(t *testing.T) {
	payload := ipc.WSOpenPayload{
		SessionID: "sess-123",
		Path:      "/ws",
		Headers:   map[string]string{"Authorization": "Bearer token"},
	}

	if payload.SessionID != "sess-123" {
		t.Error("session ID not set")
	}
}

func TestWSMessagePayload(t *testing.T) {
	payload := ipc.WSMessagePayload{
		SessionID: "sess-123",
		Data:      []byte("hello"),
		IsBinary:  false,
	}

	if string(payload.Data) != "hello" {
		t.Error("data not set")
	}
}

func TestWSClosePayload(t *testing.T) {
	payload := ipc.WSClosePayload{
		SessionID: "sess-123",
		Code:      1000,
		Reason:    "normal close",
	}

	if payload.Code != 1000 {
		t.Errorf("expected code 1000, got %d", payload.Code)
	}
}