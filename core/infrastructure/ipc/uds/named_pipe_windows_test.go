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
	time.Sleep(10 * time.Millisecond)

	addr, err := transport.ListenAddr(workerID)
	if err != nil {
		t.Fatalf("ListenAddr() error = %v", err)
	}

	client, err := uds.DialNamedPipe(ctx, addr)
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
	time.Sleep(10 * time.Millisecond)

	addr, _ := transport.ListenAddr(workerID)
	client, err := uds.DialNamedPipe(ctx, addr)
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
