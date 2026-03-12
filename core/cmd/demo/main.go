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
// noopManager: satisfaz worker.Manager sem spawnar processo real.
// ──────────────────────────────────────────────────────────────────────────────

type noopManager struct{}

func (n *noopManager) Spawn(_ context.Context, _ *worker.Worker) error  { return nil }
func (n *noopManager) Stop(_ context.Context, _ string) error           { return nil }
func (n *noopManager) StopAll(_ context.Context) error                  { return nil }
func (n *noopManager) SendHeartbeat(_ context.Context, _ string) error  { return nil }

// ──────────────────────────────────────────────────────────────────────────────
// printPublisher: imprime eventos de lifecycle no terminal com cores.
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
	// ── Logger com output human-readable ─────────────────────────────────────
	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("15:04:05.000"))
	}
	log, _ := cfg.Build()
	defer log.Sync()

	banner()

	// ── Context principal ───────────────────────────────────────────────
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	const workerID = "demo-worker-1"

	// ── Transport (Named Pipe no Windows, UDS no Unix) ────────────────────
	transport := uds.PlatformTransport()
	log.Info("[CORE] transport iniciado", zap.String("tipo", transportName()))

	if err := transport.Register(ctx, workerID); err != nil {
		log.Fatal("[CORE] falha ao registrar socket", zap.Error(err))
	}
	log.Info("[CORE] socket criado", zap.String("worker", workerID))

	// ── Infraestrutura de lifecycle ──────────────────────────────────
	repo := repository.NewMemoryWorkerRepository()
	publisher := &printPublisher{log: log}
	manager := &noopManager{}
	service := lifecycle.NewService(repo, manager, publisher)

	// Registra o worker manualmente no repositório (normalmente feito pelo SpawnWorker).
	_, err := service.SpawnWorker(ctx, workerID, "<demo-goroutine>", nil)
	if err != nil {
		log.Fatal("SpawnWorker failed", zap.Error(err))
	}

	// ── Monitor de saúde ───────────────────────────────────────────────
	healthMonitor := monitor.New(service, repo)
	go healthMonitor.Run(ctx)

	// ── Heartbeat loop do core (lê mensagens do worker) ────────────────
	hbCfg := heartbeat.DefaultConfig()
	hbCfg.Interval = 3 * time.Second // demo: timeout de 3s (prod é 5s)
	hbCfg.MissedThreshold = 2
	hbLoop := heartbeat.New(workerID, transport, service, hbCfg, log)
	go hbLoop.Run(ctx)

	// ── Worker simulado em goroutine ─────────────────────────────────
	// Aguarda o socket ficar pronto antes de conectar.
	time.Sleep(50 * time.Millisecond)

	workerCtx, workerCancel := context.WithCancel(ctx)
	crashCh := make(chan struct{})

	go runWorker(workerCtx, workerID, transport, log, crashCh)

	// ── Cenário do demo ─────────────────────────────────────────────
	go func() {
		// Após 8s simula crash: para de enviar heartbeats
		select {
		case <-time.After(8 * time.Second):
			log.Warn("[DEMO] simulando crash do worker — heartbeats vão parar")
			close(crashCh)
			workerCancel()
		case <-ctx.Done():
			workerCancel()
		}
	}()

	// ── Aguarda sinal de encerramento ───────────────────────────────
	<-ctx.Done()
	log.Info("[CORE] encerrando graciosamente…")
	_ = service.StopAll(context.Background())
	log.Info("[CORE] encerrado")
}

// runWorker simula o lado do worker: conecta ao pipe/socket e envia heartbeats.
func runWorker(ctx context.Context, workerID string, transport ipc.Transport, log *zap.Logger, crashCh <-chan struct{}) {
	time.Sleep(30 * time.Millisecond)

	client, err := dialPlatform(ctx, workerID)
	if err != nil {
		log.Error("[WORKER] falha ao conectar", zap.Error(err))
		return
	}
	defer client.Close()
	log.Info("[WORKER] conectado ao core via " + transportName())

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-crashCh:
			log.Warn("[WORKER] crash — encerrando goroutine do worker")
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := client.Send(ipc.Message{Type: ipc.TypeHeartbeat}); err != nil {
				log.Error("[WORKER] erro ao enviar heartbeat", zap.Error(err))
				return
			}
			log.Debug("[WORKER] ♥ heartbeat enviado")
		}
	}
}

func banner() {
	fmt.Println()
	fmt.Println("  ┌───────────────────────────────────────────────────────┐")
	fmt.Println("  │       vyx — IPC Demo (Phase 1)                        │")
	fmt.Println("  │  Named Pipes • Heartbeat • Lifecycle • Monitor        │")
	fmt.Println("  └───────────────────────────────────────────────────────┘")
	fmt.Println("  Cenario:")
	fmt.Println("  [0s]  worker conecta e envia heartbeats a cada 2s")
	fmt.Println("  [8s]  worker simula crash (para de enviar heartbeats)")
	fmt.Println("  [~14s] core detecta 2 heartbeats perdidos -> unhealthy")
	fmt.Println("  [~15s] monitor dispara restart automatico")
	fmt.Println("  Ctrl+C para encerrar a qualquer momento")
	fmt.Println()
}
