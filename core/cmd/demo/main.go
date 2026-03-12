// Demo is a self-contained visual demonstration of the vyx IPC and lifecycle stack.
// It does NOT spawn a real OS process — it simulates a worker in a goroutine so the
// demo works on any machine without extra binaries.
//
// What you will see:
//   - Worker registered and Named Pipe (Windows) / UDS (Unix) socket created
//   - Worker goroutine connects and sends heartbeats every 2s
//   - Core receives heartbeats and logs them
//   - After ~8s the worker "crashes" (stops sending heartbeats)
//   - Core detects missed heartbeats and marks worker unhealthy
//   - Monitor triggers an automatic restart
//   - Worker reconnects and resumes heartbeats
//   - Ctrl+C triggers graceful shutdown
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/ElioNeto/vyx/core/application/heartbeat"
	"github.com/ElioNeto/vyx/core/application/lifecycle"
	"github.com/ElioNeto/vyx/core/application/monitor"
	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/domain/worker"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/uds"
	"github.com/ElioNeto/vyx/core/infrastructure/repository"
)

// ──────────────────────────────────────────────────────────────────────────────
// noopManager: satisfies worker.Manager without spawning a real process.
// ──────────────────────────────────────────────────────────────────────────────

type noopManager struct{}

func (n *noopManager) Spawn(_ context.Context, _ *worker.Worker) error  { return nil }
func (n *noopManager) Stop(_ context.Context, _ string) error           { return nil }
func (n *noopManager) StopAll(_ context.Context) error                  { return nil }
func (n *noopManager) SendHeartbeat(_ context.Context, _ string) error  { return nil }

// ──────────────────────────────────────────────────────────────────────────────
// printPublisher: prints lifecycle events to the terminal with colors.
// ──────────────────────────────────────────────────────────────────────────────

type printPublisher struct{ log *zap.Logger }

func (p *printPublisher) Publish(_ context.Context, e worker.Event) {
	p.log.Info(fmt.Sprintf("[EVENT] %-20s worker=%-20s state=%-12s %s",
		e.Type, e.WorkerID, e.State, e.Details))
}

// ──────────────────────────────────────────────────────────────────────────────
// main
// ──────────────────────────────────────────────────────────────────────────────

func main() {
	// ── Logger with human-readable output ────────────────────────────────────
	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("15:04:05.000"))
	}
	log, _ := cfg.Build()
	defer log.Sync()

	banner()

	// ── Main context ────────────────────────────────────────────────────
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	const workerID = "demo-worker-1"

	// ── Transport (Named Pipe on Windows, UDS on Unix) ──────────────────
	transport := uds.PlatformTransport()
	log.Info("[CORE] transport started", zap.String("type", transportName()))

	if err := transport.Register(ctx, workerID); err != nil {
		log.Fatal("[CORE] failed to register socket", zap.Error(err))
	}
	log.Info("[CORE] socket created", zap.String("worker", workerID))

	// ── Lifecycle infrastructure ──────────────────────────────────────
	repo := repository.NewMemoryWorkerRepository()
	publisher := &printPublisher{log: log}
	manager := &noopManager{}
	service := lifecycle.NewService(repo, manager, publisher)

	// Manually registers the worker in the repository (usually done by SpawnWorker).
	_, err := service.SpawnWorker(ctx, workerID, "<demo-goroutine>", nil)
	if err != nil {
		log.Fatal("SpawnWorker failed", zap.Error(err))
	}

	// ── Health monitor ────────────────────────────────────────────────
	healthMonitor := monitor.New(service, repo)
	go healthMonitor.Run(ctx)

	// ── Core heartbeat loop (reads messages from the worker) ──────────
	hbCfg := heartbeat.DefaultConfig()
	hbCfg.Interval = 3 * time.Second // demo: 3s timeout (prod is 5s)
	hbCfg.MissedThreshold = 2
	hbLoop := heartbeat.New(workerID, transport, service, hbCfg, log)
	go hbLoop.Run(ctx)

	// ── Simulated worker in goroutine ───────────────────────────────
	// Wait for the socket to be ready before connecting.
	time.Sleep(50 * time.Millisecond)

	workerCtx, workerCancel := context.WithCancel(ctx)
	crashCh := make(chan struct{})

	go runWorker(workerCtx, workerID, transport, log, crashCh)

	// ── Demo scenario ───────────────────────────────────────────────
	go func() {
		// Simulates a crash after 8s: stops sending heartbeats
		select {
		case <-time.After(8 * time.Second):
			log.Warn("[DEMO] simulating worker crash — heartbeats will stop")
			close(crashCh)
			workerCancel()
		case <-ctx.Done():
			workerCancel()
		}
	}()

	// ── Wait for shutdown signal ────────────────────────────────────
	<-ctx.Done()
	log.Info("[CORE] shutting down gracefully…")
	_ = service.StopAll(context.Background())
	log.Info("[CORE] shut down")
}

// runWorker simulates the worker side: connects to the pipe/socket and sends heartbeats.
func runWorker(ctx context.Context, workerID string, transport ipc.Transport, log *zap.Logger, crashCh <-chan struct{}) {
	time.Sleep(30 * time.Millisecond)

	client, err := dialPlatform(ctx, workerID)
	if err != nil {
		log.Error("[WORKER] failed to connect", zap.Error(err))
		return
	}
	defer client.Close()
	log.Info("[WORKER] connected to core via " + transportName())

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-crashCh:
			log.Warn("[WORKER] crash — terminating worker goroutine")
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := client.Send(ipc.Message{Type: ipc.TypeHeartbeat}); err != nil {
				log.Error("[WORKER] error sending heartbeat", zap.Error(err))
				return
			}
			log.Debug("[WORKER] ♥ heartbeat sent")
		}
	}
}

func banner() {
	fmt.Println()
	fmt.Println("  ┌───────────────────────────────────────────────────────┐")
	fmt.Println("  │       vyx — IPC Demo (Phase 1)                        │")
	fmt.Println("  │  Named Pipes • Heartbeat • Lifecycle • Monitor        │")
	fmt.Println("  └───────────────────────────────────────────────────────┘")
	fmt.Println("  Scenario:")
	fmt.Println("[0s]   worker connects and sends heartbeats every 2s")
	fmt.Println("  [8s]   worker simulates crash (stops sending heartbeats)")
	fmt.Println("  [~14s] core detects 2 missed heartbeats -> unhealthy")
	fmt.Println("  [~15s] monitor triggers automatic restart")
	fmt.Println("  Press Ctrl+C to stop at any time")
	fmt.Println()
}