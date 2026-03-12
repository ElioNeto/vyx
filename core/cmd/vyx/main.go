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
	infracfg "github.com/ElioNeto/vyx/core/infrastructure/config"
	"github.com/ElioNeto/vyx/core/infrastructure/logger"
	"github.com/ElioNeto/vyx/core/infrastructure/process"
	"github.com/ElioNeto/vyx/core/infrastructure/repository"
)

const defaultConfigPath = "vyx.yaml"

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	// --- Load and validate vyx.yaml (fail fast) ---
	configPath := defaultConfigPath
	if p := os.Getenv("VYX_CONFIG"); p != "" {
		configPath = p
	}
	cfgLoader := infracfg.New(configPath, log)
	cfg, err := cfgLoader.Load()
	if err != nil {
		log.Fatal("failed to load vyx.yaml", zap.Error(err))
	}
	cfgLoader.SetCurrent(cfg)
	log.Info("vyx.yaml loaded",
		zap.String("project", cfg.Project.Name),
		zap.String("version", cfg.Project.Version),
		zap.Int("workers", len(cfg.Workers)),
	)

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

	// Watch for SIGHUP config reload in dev mode.
	go cfgLoader.WatchSIGHUP(ctx)

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
