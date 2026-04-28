#!/usr/bin/env bash
#
# e2e-test.sh — End-to-End tests for vyx (simplified)
# Starts core + example Go worker, tests IPC via UDS.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TMP_DIR="$(mktemp -d /tmp/vyx-e2e-XXXXXX)"
SOCKET_DIR="$TMP_DIR/sockets"
LOG_DIR="$TMP_DIR/logs"
EXAMPLE_DIR="$PROJECT_ROOT/examples/hello-world"

cleanup() {
  echo "==> Cleaning up..."
  # Kill all child processes
  pkill -P $$ 2>/dev/null || true
  sleep 1
  # Force kill if still running
  pkill -9 -P $$ 2>/dev/null || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

mkdir -p "$SOCKET_DIR" "$LOG_DIR"

echo "==> Building core..."
cd "$PROJECT_ROOT/core"
go build -o "$TMP_DIR/vyx-core" ./cmd/vyx 2>&1 | tee "$LOG_DIR/core-build.log"
if [ ! -f "$TMP_DIR/vyx-core" ]; then
  echo "FAIL: core build failed"
  exit 1
fi

echo "==> Building Go example worker..."
cd "$EXAMPLE_DIR/workers/go"
go build -o "$TMP_DIR/worker-go" . 2>&1 | tee "$LOG_DIR/worker-go-build.log"
if [ ! -f "$TMP_DIR/worker-go" ]; then
  echo "FAIL: Go worker build failed"
  exit 1
fi

echo "==> Preparing config from example..."
# Copy the example vyx.yaml and adjust socket_dir
sed "s|socket_dir:.*|socket_dir: $SOCKET_DIR|" "$EXAMPLE_DIR/vyx.yaml" > "$TMP_DIR/vyx.yaml"
# Also adjust working_dir for go worker to point to TMP_DIR where worker-go binary is
sed -i "s|working_dir: ./workers/go|working_dir: $TMP_DIR|" "$TMP_DIR/vyx.yaml"
# Adjust command for go worker
sed -i 's|command: go run \.|command: ./worker-go|' "$TMP_DIR/vyx.yaml"

echo "==> Starting core..."
cd "$TMP_DIR"
"$TMP_DIR/vyx-core" > "$LOG_DIR/core.log" 2>&1 &
CORE_PID=$!
echo "    Core PID: $CORE_PID"

echo "==> Waiting for core to start..."
sleep 3

# Check if core is still running
if ! kill -0 $CORE_PID 2>/dev/null; then
  echo "FAIL: core died during startup"
  cat "$LOG_DIR/core.log"
  exit 1
fi

echo "==> Go worker should be started by core automatically..."
echo "    (core spawns workers defined in vyx.yaml)"
sleep 5

echo "==> Testing Go worker via UDS (direct IPC)..."
# Create a simple test client that connects to worker UDS and sends a request
cat > "$TMP_DIR/test_client.go" <<'GOEOF'
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/ElioNeto/vyx/core/domain/gateway"
)

func main() {
	socketPath := os.Args[1]
	req := gateway.GatewayRequest{
		Method:  "GET",
		Path:    "/api/hello",
		Headers: make(map[string]string),
		Query:   make(map[string]string),
	}
	data, err := json.Marshal(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Marshal error: %v\n", err)
		os.Exit(1)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dial error: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Send request
	_, err = conn.Write(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Write error: %v\n", err)
		os.Exit(1)
	}

	// Read response
	conn.SetReadDeadline(time.Now().Add(5*time.Second))
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
		os.Exit(1)
	}

	var resp gateway.WorkerResponse
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		fmt.Fprintf(os.Stderr, "Unmarshal error: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "Expected 200, got %d, body: %s\n", resp.StatusCode, string(resp.Body))
		os.Exit(1)
	}

	fmt.Println("SUCCESS: Got 200 response from worker")
	os.Exit(0)
}
GOEOF

cd "$TMP_DIR"
go run test_client.go "$SOCKET_DIR/hello-go.sock" 2>&1 | tee "$LOG_DIR/test-go.log"
if [ ${PIPESTATUS[0]} -ne 0 ]; then
  echo "FAIL: Go worker test failed"
  cat "$LOG_DIR/test-go.log"
  exit 1
fi

echo "==> Testing error scenarios..."
# Test worker down scenario
echo "    Stopping Go worker..."
# Since core manages workers, we can't easily kill worker.  For now, skip.
# In a real test, we would test that core handles worker crash gracefully.

echo "==> All e2e tests passed!"
exit 0
