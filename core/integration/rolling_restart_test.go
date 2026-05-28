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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// controllableTransport wraps slowTransport with a notification channel that
// signals each time a request enters the Send phase (after drainer.Acquire).
// This lets the test synchronize with the exact moment all pre-drain requests
// are in-flight, eliminating flaky time.Sleep-based coordination.
type controllableTransport struct {
	*slowTransport
	inflight chan struct{} // buffered; signalled once per Send call
}

func newControllableTransport(hold time.Duration, buf int) *controllableTransport {
	return &controllableTransport{
		slowTransport: &slowTransport{
			registered:   make(map[string]bool),
			holdDuration: hold,
		},
		inflight: make(chan struct{}, buf),
	}
}

// Send sleeps for holdDuration (simulating a slow worker) and notifies
// the test that this request has moved past drainer.Acquire.
func (t *controllableTransport) Send(ctx context.Context, id string, msg ipc.Message) error {
	// Non-blocking send: if the test already collected enough signals the
	// channel stays full — that's fine.
	select {
	case t.inflight <- struct{}{}:
	default:
	}
	return t.slowTransport.Send(ctx, id, msg)
}

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

// requestResult categorizes the result of a single dispatch.
type requestResult struct {
	statusCode int
	err        error
}

// dispatchAndClassify sends a request and classifies the result.
func dispatchAndClassify(dispatcher *apgw.Dispatcher, reqNum int) requestResult {
	resp, err := dispatcher.Dispatch(context.Background(), makeRequest())
	sc := 0
	if resp != nil {
		sc = resp.StatusCode
	}
	return requestResult{statusCode: sc, err: err}
}

// ─── test ─────────────────────────────────────────────────────────────────────

// TestNoDropsDuringRollingRestart verifies that:
//   - In-flight requests that started before MarkDraining all complete with 200.
//   - Requests dispatched after MarkDraining receive 503 (not 502).
//   - No request ever receives 502 (premature kill).
//
// It uses a controllableTransport to synchronise on the exact moment all
// pre-drain requests have called drainer.Acquire, eliminating flaky sleeps.
func TestNoDropsDuringRollingRestart(t *testing.T) {
	t.Parallel()

	const (
		workerID        = "node:api"
		numPreDrain     = 20 // requests started BEFORE MarkDraining
		numPostDrain    = 20 // requests started AFTER  MarkDraining
		holdDuration    = 200 * time.Millisecond
		shutdownTimeout = 10 * time.Second
	)

	log, _ := zap.NewDevelopment()
	drainer := lifecycle.NewWorkerDrainer()
	transport := newControllableTransport(holdDuration, numPreDrain+numPostDrain)
	require.NoError(t, transport.Register(context.Background(), workerID),
		"initial transport registration must succeed")

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
		ok503   atomic.Int64 // requests correctly rejected after drain start
		ok200   atomic.Int64 // requests that completed normally
		bad502  atomic.Int64 // should remain 0
		totalOK atomic.Int64 // total requests that returned 200 or 503
	)
	var wg sync.WaitGroup

	// ── phase 1: start pre-drain requests ──────────────────────────────────
	//
	// These requests start before MarkDraining and simulate slow in-flight
	// requests that are still being processed during a rolling restart.
	for i := range numPreDrain {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			result := dispatchAndClassify(dispatcher, idx)
			switch {
			case result.err == nil && result.statusCode == 200:
				ok200.Add(1)
				totalOK.Add(1)
			case result.err == nil && result.statusCode == 502:
				bad502.Add(1)
				t.Errorf("pre-drain request %d: got 502 — worker killed while in flight", idx)
			default:
				t.Errorf("pre-drain request %d: unexpected status=%d err=%v",
					idx, result.statusCode, result.err)
			}
		}(i)
	}

	// Wait for every pre-drain request to have called drainer.Acquire
	// (signalled by controllableTransport.Send).  This guarantees they are
	// all in-flight before we trigger the rolling restart, so none can
	// accidentally race to checkDrainStatus and get 503.
	t.Log("waiting for all pre-drain requests to acquire in-flight slot ...")
	for range numPreDrain {
		<-transport.inflight
	}
	t.Log("all pre-drain requests are in-flight")

	// ── phase 2: mark draining and start post-drain requests ───────────────
	//
	// drainStarted is closed after MarkDraining so post-drain requests
	// cannot dispatch before the draining flag is set.
	drainStarted := make(chan struct{})

	drainer.MarkDraining(workerID)
	close(drainStarted)

	for i := range numPostDrain {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-drainStarted // still useful if goroutine creation is delayed
			result := dispatchAndClassify(dispatcher, numPreDrain+idx)
			switch {
			case result.err == nil && result.statusCode == 503:
				ok503.Add(1)
				totalOK.Add(1)
			case result.err == nil && result.statusCode == 502:
				bad502.Add(1)
				t.Errorf("post-drain request %d: got 502", idx)
			default:
				t.Errorf("post-drain request %d: unexpected status=%d err=%v",
					idx, result.statusCode, result.err)
			}
		}(i)
	}

	// ── phase 3: drain in-flight requests ──────────────────────────────────
	//
	// Drain waits for the pre-drain WaitGroup counter to reach 0.  Because
	// we fixed the Dispatcher's Acquire/Release balance, this completes as
	// soon as the pre-drain requests finish their holdDuration sleep.
	drainCtx, drainCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer drainCancel()

	require.NoError(t, drainer.Drain(drainCtx, workerID, shutdownTimeout),
		"drain must complete before timeout — all in-flight requests finished")

	// ── phase 4: cleanup and wait for all goroutines ───────────────────────
	drainer.Cleanup(workerID)
	require.NoError(t, transport.Register(context.Background(), workerID),
		"re-registration after restart must succeed")

	wg.Wait()

	t.Logf("results: 200=%d  503=%d  502=%d  totalOK=%d",
		ok200.Load(), ok503.Load(), bad502.Load(), totalOK.Load())
	fmt.Printf("rolling restart: 200=%d 503=%d 502=%d\n",
		ok200.Load(), ok503.Load(), bad502.Load())

	// ── assertions ────────────────────────────────────────────────────────
	assert.Equal(t, int64(numPreDrain), ok200.Load(),
		"all pre-drain requests must complete with 200")
	assert.Equal(t, int64(numPostDrain), ok503.Load(),
		"all post-drain requests must be rejected with 503")
	assert.Zero(t, bad502.Load(),
		"no request must return 502 (premature worker kill)")
}
