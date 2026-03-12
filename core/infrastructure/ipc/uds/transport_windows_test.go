//go:build windows

package uds_test

import (
	"context"
	"testing"
	"time"

	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/uds"
)

// On Windows, Unix Domain Sockets are not available for general use.
// The UDS transport is replaced by NamedPipeTransport (named_pipe_windows.go).
// These tests exercise the same contracts using the Named Pipe backend.

func TestTransport_SendReceive(t *testing.T) {
	transport := uds.NewNamedPipeTransport()
	defer transport.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const workerID = "test-worker"
	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	client, err := uds.DialNamedPipe(ctx, workerID)
	if err != nil {
		t.Fatalf("DialNamedPipe() error = %v", err)
	}
	defer client.Close()
	time.Sleep(20 * time.Millisecond)

	want := ipc.Message{Type: ipc.TypeRequest, Payload: []byte(`{"route":"/api/users"}`)}
	if err := transport.Send(ctx, workerID, want); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	got, err := client.Receive()
	if err != nil {
		t.Fatalf("client.Receive() error = %v", err)
	}
	if got.Type != want.Type || string(got.Payload) != string(want.Payload) {
		t.Errorf("got %v %q, want %v %q", got.Type, got.Payload, want.Type, want.Payload)
	}
}

func TestTransport_WorkerSendsHeartbeat(t *testing.T) {
	transport := uds.NewNamedPipeTransport()
	defer transport.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const workerID = "heartbeat-worker"
	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	client, err := uds.DialNamedPipe(ctx, workerID)
	if err != nil {
		t.Fatalf("DialNamedPipe() error = %v", err)
	}
	defer client.Close()
	time.Sleep(20 * time.Millisecond)

	if err := client.Send(ipc.Message{Type: ipc.TypeHeartbeat}); err != nil {
		t.Fatalf("client.Send() error = %v", err)
	}

	got, err := transport.Receive(ctx, workerID)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if got.Type != ipc.TypeHeartbeat {
		t.Errorf("want TypeHeartbeat, got %v", got.Type)
	}
}

func TestTransport_Deregister_RemovesSocketFile(t *testing.T) {
	// No socket file on Windows — Named Pipe handle is released by Deregister.
	// Verify the worker becomes unreachable after deregistration.
	transport := uds.NewNamedPipeTransport()

	ctx := context.Background()
	const workerID = "temp-worker"

	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := transport.Deregister(ctx, workerID); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}

	err := transport.Send(ctx, workerID, ipc.Message{Type: ipc.TypeRequest})
	if err == nil {
		t.Error("expected error after Deregister, got nil")
	}
}

func TestTransport_Send_WorkerNotConnected(t *testing.T) {
	transport := uds.NewNamedPipeTransport()
	defer transport.Close()

	ctx := context.Background()
	err := transport.Send(ctx, "ghost-worker", ipc.Message{Type: ipc.TypeRequest})
	if err == nil {
		t.Error("expected error for unregistered worker, got nil")
	}
}

func TestTransport_SocketPermissions(t *testing.T) {
	// On Windows, pipe security is enforced via DACL, not filesystem permissions.
	// Covered by TestNamedPipeTransport_SecurityDescriptor.
	t.Skip("socket permissions enforced via DACL on Windows")
}
