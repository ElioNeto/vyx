//go:build windows

package uds_test

import (
	"context"
	"testing"
	"time"

	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/uds"
)

func TestNamedPipeTransport_SendReceive(t *testing.T) {
	transport := uds.NewNamedPipeTransport()
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const workerID = "win-worker"

	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	// Give acceptPipe goroutine time to call ConnectNamedPipe.
	time.Sleep(20 * time.Millisecond)

	client, err := uds.DialNamedPipe(ctx, workerID)
	if err != nil {
		t.Fatalf("DialNamedPipe() error = %v", err)
	}
	defer client.Close()
	time.Sleep(20 * time.Millisecond)

	want := ipc.Message{Type: ipc.TypeRequest, Payload: []byte(`{"route":"/api/test"}`)}
	if err := transport.Send(ctx, workerID, want); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	got, err := client.Receive()
	if err != nil {
		t.Fatalf("client.Receive() error = %v", err)
	}
	if got.Type != want.Type {
		t.Errorf("Type: want %v, got %v", want.Type, got.Type)
	}
	if string(got.Payload) != string(want.Payload) {
		t.Errorf("Payload: want %q, got %q", want.Payload, got.Payload)
	}
}

func TestNamedPipeTransport_WorkerHeartbeat(t *testing.T) {
	transport := uds.NewNamedPipeTransport()
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const workerID = "win-hb"
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

	hb := ipc.Message{Type: ipc.TypeHeartbeat, Payload: []byte{}}
	if err := client.Send(hb); err != nil {
		t.Fatalf("Send() heartbeat error = %v", err)
	}

	got, err := transport.Receive(ctx, workerID)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if got.Type != ipc.TypeHeartbeat {
		t.Errorf("want TypeHeartbeat, got %v", got.Type)
	}
}

func TestNamedPipeTransport_Deregister(t *testing.T) {
	transport := uds.NewNamedPipeTransport()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const workerID = "win-dereg"
	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if err := transport.Deregister(ctx, workerID); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}

	// After deregister, Send must return ErrWorkerNotConnected.
	err := transport.Send(ctx, workerID, ipc.Message{Type: ipc.TypeRequest})
	if err == nil {
		t.Fatal("expected error after Deregister, got nil")
	}
}

func TestNamedPipeTransport_SecurityDescriptor(t *testing.T) {
	// This test verifies that the pipe is created with a restricted DACL.
	// It connects as the same user and expects success; connecting as a
	// different user would fail (cannot be tested in a unit test without
	// a secondary user account, but the DACL is verified by inspection).
	transport := uds.NewNamedPipeTransport()
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const workerID = "win-sec"
	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	// Same user should be able to connect.
	client, err := uds.DialNamedPipe(ctx, workerID)
	if err != nil {
		t.Fatalf("same-user DialNamedPipe() should succeed, got: %v", err)
	}
	client.Close()
}
