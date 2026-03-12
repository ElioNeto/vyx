// Package main is the composition root: it wires all layers together and starts the core.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	"github.com/ElioNeto/vyx/core/application/heartbeat"
	"github.com/ElioNeto/vyx/core/application/lifecycle"
	"github.com/ElioNeto/vyx/core/application/monitor"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	infracfg "github.com/ElioNeto/vyx/core/infrastructure/config"
	infragw "github.com/ElioNeto/vyx/core/infrastructure/gateway"
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

	// --- Load route_map.json (optional at startup; required for routing) ---
	var routeMap *dgw.RouteMap
	routeMapPath := cfg.Build.RouteMapOutput
	if routeMapPath == "" {
		routeMapPath = "./route_map.json"
	}
	rm, err := dgw.LoadRouteMap(routeMapPath)
	if err != nil {
		log.Warn("route_map.json not found — starting without routes",
			zap.String("path", routeMapPath),
			zap.Error(err),
		)
		rm = dgw.NewRouteMap(nil)
	}
	routeMap = rm
	log.Info("route map loaded", zap.String("path", routeMapPath))

	// Wire route map hot-reload on SIGHUP (#41).
	cfgLoader.WithRouteMap(routeMapPath, routeMap)

	// --- Dependency injection (composition root) ---
	repo := repository.NewMemoryWorkerRepository()
	manager := process.New()
	publisher := logger.New(log)
	service := lifecycle.NewService(repo, manager, publisher)
	healthMonitor := monitor.New(service, repo)

	// --- JWT validator ---
	jwtSecret := os.Getenv(cfg.Security.JWTSecretEnv)
	if jwtSecret == "" {
		log.Warn("JWT secret env var not set — auth will reject all tokens",
			zap.String("env", cfg.Security.JWTSecretEnv),
		)
	}
	jwtValidator := infragw.NewJWTValidator([]byte(jwtSecret))

	// --- Schema validator ---
	schemaValidator := infragw.NewSchemaValidator(cfg.Build.SchemasDir)

	// --- Gateway dispatcher ---
	dispatcher := apgw.NewDispatcher(
		routeMap,
		nil, // UDS transport — wired in issue #45
		jwtValidator,
		schemaValidator,
		cfg.Security.GlobalTimeout,
		log,
	)

	// --- Rate limiter ---
	rateLimiter := apgw.NewRateLimiter(
		cfg.Security.RateLimit.PerIP,
		cfg.Security.RateLimit.PerToken,
	)

	// --- HTTP server ---
	gwCfg := infragw.DefaultConfig()
	httpServer := infragw.New(gwCfg, dispatcher, rateLimiter, log)

	// --- Heartbeat sender (#38) ---
	hbSender := heartbeat.NewSender(
		nil, // UDS transport — wired in issue #45
		repo,
		heartbeat.Config{Interval: 5 * time.Second},
		log,
	)

	// --- Context wired to OS signals ---
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("vyx core starting", zap.String("addr", gwCfg.Addr))

	// Start background services.
	go healthMonitor.Run(ctx)
	go cfgLoader.WatchSIGHUP(ctx)
	go hbSender.Run(ctx) // core → worker heartbeats (#38)

	// Start HTTP server.
	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			log.Error("HTTP server stopped", zap.Error(err))
		}
	}()

	// Block until SIGTERM / SIGINT.
	<-ctx.Done()

	log.Info("vyx core shutting down — draining workers")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Graceful HTTP shutdown before stopping workers.
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown error", zap.Error(err))
	}

	if err := service.StopAll(shutdownCtx); err != nil {
		log.Error("error during graceful shutdown", zap.Error(err))
		os.Exit(1)
	}

	log.Info("vyx core stopped cleanly")
}
