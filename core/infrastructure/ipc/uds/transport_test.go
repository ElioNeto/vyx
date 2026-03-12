package uds_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/uds"
)

func TestTransport_SendReceive(t *testing.T) {
	dir := t.TempDir()
	transport := uds.New(dir)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const workerID = "test-worker"

	// Server: register socket and start listening.
	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Give the goroutine a moment to start Accept().
	time.Sleep(10 * time.Millisecond)

	// Client: dial the socket (simulates a worker connecting).
	sockPath := filepath.Join(dir, workerID+".sock")
	client, err := uds.Dial(ctx, sockPath)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer client.Close()

	// Give accept goroutine time to register the connection.
	time.Sleep(20 * time.Millisecond)

	// Core sends a request to the worker.
	want := ipc.Message{Type: ipc.TypeRequest, Payload: []byte(`{"route":"/api/users"}`)}
	if err := transport.Send(ctx, workerID, want); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Worker receives the request.
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

func TestTransport_WorkerSendsHeartbeat(t *testing.T) {
	dir := t.TempDir()
	transport := uds.New(dir)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const workerID = "heartbeat-worker"

	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	sockPath := filepath.Join(dir, workerID+".sock")
	client, err := uds.Dial(ctx, sockPath)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer client.Close()
	time.Sleep(20 * time.Millisecond)

	// Worker sends heartbeat to the core.
	hb := ipc.Message{Type: ipc.TypeHeartbeat, Payload: []byte{}}
	if err := client.Send(hb); err != nil {
		t.Fatalf("client.Send() heartbeat error = %v", err)
	}

	// Core reads the heartbeat.
	got, err := transport.Receive(ctx, workerID)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if got.Type != ipc.TypeHeartbeat {
		t.Errorf("want TypeHeartbeat, got %v", got.Type)
	}
}

func TestTransport_Deregister_RemovesSocketFile(t *testing.T) {
	dir := t.TempDir()
	transport := uds.New(dir)

	ctx := context.Background()
	const workerID = "temp-worker"

	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	sockPath := filepath.Join(dir, workerID+".sock")
	if _, err := os.Stat(sockPath); err != nil {
		t.Fatalf("socket file should exist after Register: %v", err)
	}

	if err := transport.Deregister(ctx, workerID); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}

	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("socket file should be removed after Deregister")
	}
}

func TestTransport_Send_WorkerNotConnected(t *testing.T) {
	dir := t.TempDir()
	transport := uds.New(dir)
	defer transport.Close()

	ctx := context.Background()
	msg := ipc.Message{Type: ipc.TypeRequest, Payload: []byte("test")}

	err := transport.Send(ctx, "ghost-worker", msg)
	if err == nil {
		t.Error("expected error for unregistered worker, got nil")
	}
}

func TestTransport_SocketPermissions(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission tests are not meaningful as root")
	}

	dir := t.TempDir()
	transport := uds.New(dir)
	defer transport.Close()

	ctx := context.Background()
	const workerID = "perm-worker"

	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	sockPath := filepath.Join(dir, workerID+".sock")
	info, err := os.Stat(sockPath)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("socket permission: want 0600, got %04o", perm)
	}
}
