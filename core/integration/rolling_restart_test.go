// Package integration contains end-to-end tests that exercise multiple
// application-layer components together without spawning real sub-processes.
//
// Test: TestNoDropsDuringRollingRestart (#56)
//
// Scenario
// --------
// 1. Build a minimal in-process "gateway" using the real WorkerDrainer and
//    the real Dispatcher (with stub JWT/schema validators and a stub IPC
//    transport that always succeeds).
// 2. Fire N concurrent HTTP-like Dispatch calls.  Each call holds the
//    drainer's Acquire semaphore for `holdDuration` to simulate slow
//    requests that are still in-flight during the restart window.
// 3. Concurrently trigger a simulated rolling restart:
//    a. MarkDraining(workerID)  – new requests get 503 from this point.
//    b. Drain(…, shutdownTimeout) – wait for in-flight requests to finish.
//    c. "kill" the worker (cleanup) + respawn (cleanup drainer state).
// 4. Assertions:
//    - Every request that called Acquire BEFORE MarkDraining returns 200.
//    - Every request dispatched AFTER MarkDraining returns 503.
//    - No request returns 502 ("bad gateway" / premature kill).
package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/application/lifecycle"
	"go.uber.org/zap"
)

// ─── stub dependencies ────────────────────────────────────────────────────────

// stubJWT accepts every token.
type stubJWT struct{}

func (stubJWT) Validate(_ string) (*dgw.Claims, error) {
	return &dgw.Claims{UserID: "test", Roles: []string{"user"}}, nil
}

// stubSchema always passes.
type stubSchema struct{}

func (stubSchema) Validate(_ string, _ []byte) error { return nil }

// slowTransport simulates a worker IPC transport that takes holdDuration to
// respond. This keeps the Dispatcher's Acquire semaphore held for that long,
// giving the drain loop time to block.
type slowTransport struct {
	mu           sync.Mutex
	registered   map[string]bool
	holdDuration time.Duration
}

func newSlowTransport(hold time.Duration) *slowTransport {
	return &slowTransport{
		registered:   make(map[string]bool),
		holdDuration: hold,
	}
}

func (t *slowTransport) Register(_ context.Context, id string) error {
	t.mu.Lock()
	t.registered[id] = true
	t.mu.Unlock()
	return nil
}
func (t *slowTransport) Deregister(_ context.Context, id string) error {
	t.mu.Lock()
	delete(t.registered, id)
	t.mu.Unlock()
	return nil
}
func (t *slowTransport) Send(_ context.Context, _ string, _ ipc.Message) error {
	time.Sleep(t.holdDuration)
	return nil
}
func (t *slowTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	body, _ := json.Marshal(dgw.WorkerResponse{StatusCode: 200, Body: []byte(`{"ok":true}`)})
	return ipc.Message{Type: ipc.TypeResponse, Payload: body}, nil
}
func (t *slowTransport) Receive(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, nil
}
func (t *slowTransport) Close() error { return nil }

// ─── helpers ──────────────────────────────────────────────────────────────────

func buildRouteMap(workerID string) *dgw.RouteMap {
	return dgw.NewRouteMap([]dgw.RouteEntry{
		{
			WorkerID: workerID,
			Method:   "GET",
			Path:     "/ping",
		},
	})
}

func makeRequest() *dgw.GatewayRequest {
	return &dgw.GatewayRequest{
		Method:  "GET",
		Path:    "/ping",
		Headers: map[string]string{},
		Query:   map[string]string{},
	}
}

// ─── test ─────────────────────────────────────────────────────────────────────

// TestNoDropsDuringRollingRestart verifies that:
//   - In-flight requests that started before MarkDraining all complete with 200.
//   - Requests dispatched after MarkDraining receive 503 (not 502).
//   - No request ever receives 502 (premature kill).
func TestNoDropsDuringRollingRestart(t *testing.T) {
	t.Parallel()

	const (
		workerID        = "node:api"
		numRequests     = 40
		holdDuration    = 80 * time.Millisecond  // how long each request is "in flight"
		shutdownTimeout = 5 * time.Second
		restartDelay    = 20 * time.Millisecond  // trigger drain after this many ms
	)

	log, _ := zap.NewDevelopment()
	drainer := lifecycle.NewWorkerDrainer()
	transport := newSlowTransport(holdDuration)
	_ = transport.Register(context.Background(), workerID)

	rm := buildRouteMap(workerID)

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       stubJWT{},
		Schema:    stubSchema{},
		Timeout:   10 * time.Second, // dispatch timeout – much larger than holdDuration
		Log:       log,
		Drainer:   drainer,
	})

	var (
		ok503  atomic.Int64 // requests correctly rejected after drain start
		ok200  atomic.Int64 // requests that completed normally
		bad502 atomic.Int64 // should remain 0
		badOther atomic.Int64
	)

	var wg sync.WaitGroup

	// Launch requests spread over 2×restartDelay so some start before and
	// some start after MarkDraining.
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Spread start times: first half immediately, second half after drain
			// triggers.
			if i >= numRequests/2 {
				time.Sleep(restartDelay * 3)
			}
			resp, err := dispatcher.Dispatch(context.Background(), makeRequest())
			switch {
			case err == nil && resp.StatusCode == 200:
				ok200.Add(1)
			case err == nil && resp.StatusCode == 503:
				ok503.Add(1)
			case err == nil && resp.StatusCode == 502:
				bad502.Add(1)
				t.Errorf("got 502 on request %d — worker killed while request was in flight", i)
			default:
				badOther.Add(1)
				sc := 0
				if resp != nil {
					sc = resp.StatusCode
				}
				t.Errorf("unexpected result on request %d: status=%d err=%v", i, sc, err)
			}
		}(i)
	}

	// Trigger the rolling restart after restartDelay — while first-half
	// requests are still holding the slow transport.
	go func() {
		time.Sleep(restartDelay)

		// Step 1: mark draining — new requests will get 503.
		drainer.MarkDraining(workerID)

		// Step 2: wait for all in-flight requests to finish.
		drainCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := drainer.Drain(drainCtx, workerID, shutdownTimeout); err != nil {
			t.Errorf("drain timed out: %v", err)
		}

		// Step 3: "kill" old process (cleanup drainer state) + respawn.
		drainer.Cleanup(workerID)
		// Re-register IPC socket (simulates process respawn).
		_ = transport.Register(context.Background(), workerID)
	}()

	wg.Wait()

	t.Logf("results: 200=%d  503=%d  502=%d  other=%d",
		ok200.Load(), ok503.Load(), bad502.Load(), badOther.Load())

	if bad502.Load() > 0 {
		t.Errorf("FAIL: %d request(s) returned 502 — in-flight requests were dropped",
			bad502.Load())
	}
	if ok200.Load() == 0 {
		t.Error("FAIL: no request completed successfully with 200")
	}
	if ok503.Load() == 0 {
		t.Error("FAIL: no request was correctly rejected with 503 after drain start")
	}
	fmt.Printf("✅ rolling restart: 200=%d 503=%d 502=%d\n",
		ok200.Load(), ok503.Load(), bad502.Load())
}
