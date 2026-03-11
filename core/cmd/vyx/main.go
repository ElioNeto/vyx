// Package main is the composition root: it wires all layers together and starts the core.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/ElioNeto/vyx/core/application/lifecycle"
	"github.com/ElioNeto/vyx/core/application/monitor"
	"github.com/ElioNeto/vyx/core/infrastructure/logger"
	"github.com/ElioNeto/vyx/core/infrastructure/process"
	"github.com/ElioNeto/vyx/core/infrastructure/repository"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	// --- Dependency injection (composition root) ---
	repo := repository.NewMemoryWorkerRepository()
	manager := process.New()
	publisher := logger.New(log)
	service := lifecycle.NewService(repo, manager, publisher)
	healthMonitor := monitor.New(service, repo)

	// --- Context wired to OS signals ---
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("vyx core starting")

	// Start the health monitor in the background.
	go healthMonitor.Run(ctx)

	// Block until SIGTERM / SIGINT.
	<-ctx.Done()

	log.Info("vyx core shutting down — draining workers")

	shutdownCtx := context.Background()
	if err := service.StopAll(shutdownCtx); err != nil {
		log.Error("error during graceful shutdown", zap.Error(err))
		os.Exit(1)
	}

	log.Info("vyx core stopped cleanly")
}
