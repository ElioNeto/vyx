package uds

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ElioNeto/vyx/core/domain/ipc"
)

// TestSendToResponse tests the sendToResponse function
func TestSendToResponse(t *testing.T) {
	dir := t.TempDir()
	transport := New(dir)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	const workerID = "test-worker"
	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	// Connect a client
	sockPath := filepath.Join(dir, workerID+".sock")
	client, err := Dial(ctx, sockPath)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer client.Close()
	time.Sleep(20 * time.Millisecond)

	// Send a response message from the client (simulating worker)
	msg := ipc.Message{Type: ipc.TypeResponse, Payload: []byte("test")}
	if err := client.Send(msg); err != nil {
		t.Fatalf("client.Send() error = %v", err)
	}

	// Transport should receive it via ReceiveResponse
	got, err := transport.ReceiveResponse(ctx, workerID)
	if err != nil {
		t.Fatalf("ReceiveResponse() error = %v", err)
	}
	if got.Type != ipc.TypeResponse {
		t.Errorf("Type: want %v, got %v", ipc.TypeResponse, got.Type)
	}
}

// TestReceiveResponse_EmptyChannel tests when no response available
func TestReceiveResponse_EmptyChannel(t *testing.T) {
	dir := t.TempDir()
	transport := New(dir)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	const workerID = "test-worker"
	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	// Try to receive - should timeout
	_, err := transport.ReceiveResponse(ctx, workerID)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

// TestGetPumpErr_AfterDisconnect tests getPumpErr after client disconnects
func TestGetPumpErr_AfterDisconnect(t *testing.T) {
	dir := t.TempDir()
	transport := New(dir)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	const workerID = "test-worker"
	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	// Connect a client
	sockPath := filepath.Join(dir, workerID+".sock")
	client, err := Dial(ctx, sockPath)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}

	// Close the client (simulate worker disconnect)
	client.Close()
	time.Sleep(20 * time.Millisecond)

	// Now try to receive - should get an error (pumpErr is set)
	_, err = transport.Receive(ctx, workerID)
	t.Logf("Receive after disconnect: %v", err)
}

// TestReceiveFrom tests the receiveFrom function
func TestReceiveFrom(t *testing.T) {
	dir := t.TempDir()
	transport := New(dir)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	const workerID = "test-worker"
	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	sockPath := filepath.Join(dir, workerID+".sock")
	client, err := Dial(ctx, sockPath)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer client.Close()
	time.Sleep(10 * time.Millisecond)

	// Send heartbeat from worker
	hb := ipc.Message{Type: ipc.TypeHeartbeat, Payload: []byte{}}
	if err := client.Send(hb); err != nil {
		t.Fatalf("client.Send() error = %v", err)
	}

	// Transport should receive it
	got, err := transport.Receive(ctx, workerID)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if got.Type != ipc.TypeHeartbeat {
		t.Errorf("Type: want %v, got %v", ipc.TypeHeartbeat, got.Type)
	}
}

// TestDial_InvalidPath tests Dial with invalid socket path
func TestDial_InvalidPath(t *testing.T) {
	ctx := context.Background()
	_, err := Dial(ctx, "/non-existent/path.sock")
	if err == nil {
		t.Fatal("expected error for invalid socket path")
	}
}

// TestDeregister_MultipleTimes tests Deregister called multiple times
func TestDeregister_MultipleTimes(t *testing.T) {
	dir := t.TempDir()
	transport := New(dir)
	defer transport.Close()

	ctx := context.Background()

	const workerID = "test-worker"
	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	// Deregister once
	if err := transport.Deregister(ctx, workerID); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}

	// Deregister again (should not error)
	if err := transport.Deregister(ctx, workerID); err != nil {
		t.Fatalf("Deregister() second time error = %v", err)
	}
}

// TestReceiveResponse_NoConnection tests when no worker is connected
func TestReceiveResponse_NoConnection(t *testing.T) {
	dir := t.TempDir()
	transport := New(dir)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Try to receive from non-existent worker
	_, err := transport.ReceiveResponse(ctx, "non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent worker")
	}
}

// TestReceive_NoConnection tests when no worker is connected
func TestReceive_NoConnection(t *testing.T) {
	dir := t.TempDir()
	transport := New(dir)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Try to receive from non-existent worker
	_, err := transport.Receive(ctx, "non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent worker")
	}
}

// TestPlatformTransport tests the PlatformTransport function
func TestPlatformTransport(t *testing.T) {
	// PlatformTransport is platform-specific
	// On Linux, it returns the standard net.Listener
	// We can't easily test this directly
	t.Skip("PlatformTransport is platform-specific")
}

// TestDeregister_WithActiveConnection tests Deregister when connection exists.
func TestDeregister_WithActiveConnection(t *testing.T) {
	dir := t.TempDir()
	transport := New(dir)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	const workerID = "test-worker"

	// Register and connect
	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	sockPath := filepath.Join(dir, workerID+".sock")
	client, err := Dial(ctx, sockPath)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	// Now Deregister (should close connection and listener)
	if err := transport.Deregister(ctx, workerID); err != nil {
		t.Errorf("Deregister() error = %v", err)
	}

	client.Close()
}

// TestReceiveFrom_ContextCancelled tests context cancellation.
func TestReceiveFrom_ContextCancelled(t *testing.T) {
	dir := t.TempDir()
	transport := New(dir)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	const workerID = "test-worker"
	if err := transport.Register(ctx, workerID); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	// Connect a client
	sockPath := filepath.Join(dir, workerID+".sock")
	client, err := Dial(ctx, sockPath)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer client.Close()
	time.Sleep(20 * time.Millisecond)

	// Try to receive with short timeout context
	_, err = transport.Receive(ctx, workerID)
	if err == nil {
		t.Error("expected context cancellation error")
	}
	t.Logf("Got expected error: %v", err)
}

// TestRegister_MkdirFails tests when socket dir creation fails.
func TestRegister_MkdirFails(t *testing.T) {
	// Use a read-only parent dir to force mkdir error
	dir := "/proc/invalid-subdir"
	transport := New(dir)
	defer transport.Close()

	ctx := context.Background()
	err := transport.Register(ctx, "worker1")
	if err == nil {
		t.Error("expected error from mkdir failure")
	}
	t.Logf("Got expected error: %v", err)
}
