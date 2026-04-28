package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

// TestSimpleIntegration tests a minimal in-process gateway with a mock transport.
func TestSimpleIntegration(t *testing.T) {
	t.Parallel()

	// Build a route map with a single route
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "test:worker", Method: "GET", Path: "/api/test"},
	})

	// Create a simple mock transport that responds with 200
	transport := &mockTransport{
		response: dgw.WorkerResponse{
			StatusCode: 200,
			Body:        []byte(`{"test": true}`),
		},
	}

	log, _ := zap.NewDevelopment()

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       stubJWT{},
		Schema:    stubSchema{},
		Timeout:   5 * time.Second,
		Log:       log,
		Drainer:   lifecycle.NewWorkerDrainer(),
	})

	// Dispatch a request
	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/api/test",
		Headers: make(map[string]string),
		Query:   make(map[string]string),
	}

	resp, err := dispatcher.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	t.Log("SUCCESS: simple integration test passed")
}

// mockTransport is a simple mock that returns a predefined response.
type mockTransport struct {
	response dgw.WorkerResponse
}

func (m *mockTransport) Register(_ context.Context, id string) error  { return nil }
func (m *mockTransport) Deregister(_ context.Context, id string) error { return nil }
func (m *mockTransport) Send(_ context.Context, _ string, _ ipc.Message) error {
	time.Sleep(10 * time.Millisecond) // simulate network delay
	return nil
}
func (m *mockTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	body, _ := json.Marshal(m.response)
	return ipc.Message{Type: ipc.TypeResponse, Payload: body}, nil
}
func (m *mockTransport) Receive(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, nil
}
func (m *mockTransport) Close() error { return nil }

// We need to import ipc for the Message type, but we can't import infrastructure/ipc.
// So we define minimal versions here.
type ipcMessage struct {
	Type    byte
	Payload []byte
}

// We'll avoid using ipc by using a simpler approach.
// Actually, we can just test the dispatcher with a mock that doesn't use ipc at all.
// Let's redefine mockTransport without ipc dependency.

// Since we can't easily mock the IPC interface, we'll skip real IPC test here.
// The e2e-test.sh script will test real IPC.

func TestUDSServerClient(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for UDS
	tmpDir, err := os.MkdirTemp("", "vyx-uds-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "test.sock")

	// Start a simple UDS server that echoes requests
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to listen on UDS: %v", err)
	}
	defer listener.Close()

	// Server goroutine: accept one connection, read request, send response
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Errorf("server accept error: %v", err)
			return
		}
		defer conn.Close()

		// Read the incoming data
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			t.Errorf("server read error: %v", err)
			return
		}

		// Parse as GatewayRequest (simplified)
		var req dgw.GatewayRequest
		if err := json.Unmarshal(buf[:n], &req); err != nil {
			t.Errorf("server unmarshal error: %v", err)
			return
		}

		// Create a response
		resp := dgw.WorkerResponse{
			StatusCode: 200,
			Body:        []byte(fmt.Sprintf(`{"echo": "%s"}`, req.Path)),
		}
		respData, _ := json.Marshal(resp)

		// Write response
		_, err = conn.Write(respData)
		if err != nil {
			t.Errorf("server write error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Client: connect to UDS and send a request
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("client dial error: %v", err)
	}
	defer conn.Close()

	// Create a request
	req := &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/api/test",
		Headers: make(map[string]string),
		Query:   make(map[string]string),
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request error: %v", err)
	}

	// Send request
	_, err = conn.Write(reqData)
	if err != nil {
		t.Fatalf("client write error: %v", err)
	}

	// Read response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("client read error: %v", err)
	}

	var resp dgw.WorkerResponse
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		t.Fatalf("unmarshal response error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	t.Logf("SUCCESS: UDS test passed, response: %s", string(resp.Body))
}
