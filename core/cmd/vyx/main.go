// Package main is the vyx CLI entry point. It dispatches to subcommands:
//
//	vyx new <name>   — scaffold a new project
//	vyx dev          — start core in development mode (h2c + verbose logs)
//	vyx build        — scan annotations and generate route_map.json
//	vyx annotate     — print discovered routes to stdout (dry run)
//	vyx start        — start core in production mode (same as running `vyx` with no subcommand)
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	"github.com/ElioNeto/vyx/core/application/handshake"
	"github.com/ElioNeto/vyx/core/application/heartbeat"
	"github.com/ElioNeto/vyx/core/application/lifecycle"
	"github.com/ElioNeto/vyx/core/application/monitor"
	doamincfg "github.com/ElioNeto/vyx/core/domain/config"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
	infracfg "github.com/ElioNeto/vyx/core/infrastructure/config"
	infragw "github.com/ElioNeto/vyx/core/infrastructure/gateway"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/uds"
	"github.com/ElioNeto/vyx/core/infrastructure/logger"
	"github.com/ElioNeto/vyx/core/infrastructure/process"
	"github.com/ElioNeto/vyx/core/infrastructure/repository"
)

const defaultConfigPath = "vyx.yaml"

func main() {
	if len(os.Args) < 2 {
		runServer(false)
		return
	}

	switch os.Args[1] {
	case "new":
		if len(os.Args) < 3 {
			fatalf("usage: vyx new <project-name>")
		}
		cmdNew(os.Args[2])
	case "dev":
		runServer(true)
	case "start":
		runServer(false)
	case "build":
		cmdBuild()
	case "annotate":
		cmdAnnotate()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "vyx: unknown subcommand %q\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`vyx — polyglot process core

Usage:
  vyx new <name>    Scaffold a new vyx project
  vyx dev           Start core in development mode (h2c, verbose logs, hot reload)
  vyx start         Start core in production mode
  vyx build         Scan annotations and generate route_map.json
  vyx annotate      Dry-run: print discovered routes to stdout
  vyx help          Show this help`)
}

// ─── vyx new ──────────────────────────────────────────────────────────────────────

func cmdNew(name string) {
	if err := os.MkdirAll(name, 0755); err != nil {
		fatalf("vyx new: create directory: %v", err)
	}

	cfg := doamincfg.Defaults()
	cfg.Project.Name = name
	cfg.Project.Version = "0.1.0"
	cfg.Workers = []doamincfg.WorkerConfig{
		{
			ID:              "node:api",
			Command:         "node",
			Replicas:        1,
			Strategy:        "round-robin",
			StartupTimeout:  10 * time.Second,
			ShutdownTimeout: 5 * time.Second,
		},
	}

	cfgData, err := yaml.Marshal(cfg)
	if err != nil {
		fatalf("vyx new: marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(name, "vyx.yaml"), cfgData, 0644); err != nil {
		fatalf("vyx new: write vyx.yaml: %v", err)
	}

	for _, dir := range []string{"workers", "schemas"} {
		if err := os.MkdirAll(filepath.Join(name, dir), 0755); err != nil {
			fatalf("vyx new: create %s: %v", dir, err)
		}
	}

	readme := fmt.Sprintf("# %s\n\nCreated with `vyx new %s`.\n\nRun `cd %s && vyx dev` to start in development mode.\n", name, name, name)
	if err := os.WriteFile(filepath.Join(name, "README.md"), []byte(readme), 0644); err != nil {
		fatalf("vyx new: write README: %v", err)
	}

	fmt.Printf("✓ Project %q created.\n  Next: cd %s && vyx dev\n", name, name)
}

// ─── vyx build ────────────────────────────────────────────────────────────────────

func cmdBuild() {
	cfg := mustLoadConfig()

	goDir := ""
	tsDir := ""
	if _, err := os.Stat("workers"); err == nil {
		tsDir = "workers"
	}
	if _, err := os.Stat("backend/go"); err == nil {
		goDir = "backend/go"
	}

	output := cfg.Build.RouteMapOutput
	if output == "" {
		output = "./route_map.json"
	}

	if err := runAnnotateCmd(goDir, tsDir, output); err != nil {
		fatalf("vyx build: %v", err)
	}
	fmt.Printf("✓ route_map.json written to %s\n", output)
}

// ─── vyx annotate ───────────────────────────────────────────────────────────────────

func cmdAnnotate() {
	cfg := mustLoadConfig()

	tsDir := ""
	if _, err := os.Stat("workers"); err == nil {
		tsDir = "workers"
	}

	output := cfg.Build.RouteMapOutput
	if output == "" {
		output = "./route_map.json"
	}

	if err := runAnnotateCmd("", tsDir, output); err != nil {
		fatalf("vyx annotate: %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		fatalf("vyx annotate: read output: %v", err)
	}
	var pretty map[string]any
	if err := json.Unmarshal(data, &pretty); err != nil {
		fatalf("vyx annotate: parse output: %v", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(pretty)
}

// runAnnotateCmd invokes the vyx annotation scanner via os/exec.
func runAnnotateCmd(goDir, tsDir, output string) error {
	args := []string{
		"run", "github.com/ElioNeto/vyx/cmd/annotate",
		"--output", output,
	}
	if goDir != "" {
		args = append(args, "--go-dir", goDir)
	}
	if tsDir != "" {
		args = append(args, "--ts-dir", tsDir)
	}

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if strings.Contains(err.Error(), "no required module") ||
			(func() bool {
				if ok := errors.As(err, &exitErr); ok {
					return exitErr.ExitCode() == 127
				}
				return false
			}()) {
			return fmt.Errorf("scanner binary not found — run `go install github.com/ElioNeto/vyx/cmd/annotate@latest`: %w", err)
		}
		return err
	}
	return nil
}

// ─── vyx dev / vyx start ──────────────────────────────────────────────────────────────────

func runServer(devMode bool) {
	var log *zap.Logger
	if devMode {
		var err error
		log, err = zap.NewDevelopment()
		if err != nil {
			fatalf("logger: %v", err)
		}
	} else {
		var err error
		log, err = zap.NewProduction()
		if err != nil {
			fatalf("logger: %v", err)
		}
	}
	defer log.Sync()

	// --- Load config ---
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
		zap.Bool("dev_mode", devMode),
	)

	// --- Load route_map.json ---
	routeMapPath := cfg.Build.RouteMapOutput
	if routeMapPath == "" {
		routeMapPath = "./route_map.json"
	}
	// Resolve relative route_map path against the config file's directory
	// so that `VYX_CONFIG=/abs/path/vyx.yaml` finds the route_map next to it.
	if !filepath.IsAbs(routeMapPath) {
		routeMapPath = filepath.Join(filepath.Dir(configPath), routeMapPath)
	}
	rm, err := dgw.LoadRouteMap(routeMapPath)
	if err != nil {
		log.Warn("route_map.json not found — starting without routes",
			zap.String("path", routeMapPath), zap.Error(err))
		rm = dgw.NewRouteMap(nil)
	}
	cfgLoader.WithRouteMap(routeMapPath, rm)
	log.Info("route map loaded", zap.String("path", routeMapPath))

	// --- Infrastructure: IPC transport (UDS on Unix, Named Pipes on Windows) ---
	var transport ipc.Transport
	socketDir := cfg.IPC.SocketDir
	if socketDir == "" {
		socketDir = uds.DefaultSocketDir
	}
	transport = platformTransport(socketDir)
	log.Info("IPC transport initialised",
		zap.String("socket_dir", socketDir),
	)

	// --- Core services ---
	repo := repository.NewMemoryWorkerRepository()
	manager := process.New()
	publisher := logger.New(log)

	// hbReceiver is created before lifecycle.Service so it can be passed in.
	hbCfg := heartbeat.DefaultConfig()
	hbReceiver := heartbeat.NewReceiver(transport, repo, nil, hbCfg, log)

	// lifecycle.Service needs the transport (to re-register on restart) and
	// the receiver (to re-arm the heartbeat loop after restart).
	service := lifecycle.NewService(repo, manager, publisher, transport, hbReceiver)

	// Now wire the service back into the receiver (it needs service for MarkRunning etc).
	hbReceiver.SetService(service)

	healthMonitor := monitor.New(service, repo)

	// --- JWT + Schema validators ---
	jwtSecret := os.Getenv(cfg.Security.JWTSecretEnv)
	if jwtSecret == "" {
		log.Warn("JWT secret env var not set — auth will reject all tokens",
			zap.String("env", cfg.Security.JWTSecretEnv))
	}
	jwtValidator := infragw.NewJWTValidator([]byte(jwtSecret))
	schemaValidator := infragw.NewSchemaValidator(cfg.Build.SchemasDir)

	// --- Dispatcher ---
	dispatcher := apgw.NewDispatcher(
		rm,
		transport,
		jwtValidator,
		schemaValidator,
		cfg.Security.GlobalTimeout,
		log,
	)

	// --- Rate limiter ---
	rateLimiter := apgw.NewRateLimiter(
		cfg.Security.RateLimit.PerIP,
		cfg.Security.RateLimit.PerToken,
		time.Minute,
	)

	// --- HTTP server (dev=h2c, prod=H2+TLS) ---
	var gwCfg infragw.Config
	if devMode {
		gwCfg = infragw.DevConfig()
	} else {
		gwCfg = infragw.DefaultConfig()
	}
	httpServer := infragw.New(gwCfg, dispatcher, rateLimiter, log)

	// --- Heartbeat sender (core → worker) ---
	hbSender := heartbeat.NewSender(transport, repo, hbCfg, log)

	// --- Context + signal handling ---
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if devMode {
		log.Info("vyx core starting in DEV mode",
			zap.String("addr", gwCfg.Addr),
		)
	} else {
		log.Info("vyx core starting",
			zap.String("addr", gwCfg.Addr),
		)
	}

	// --- Auto-spawn workers from vyx.yaml ---
	for _, wcfg := range cfg.Workers {
		replicas := wcfg.Replicas
		if replicas <= 0 {
			replicas = 1
		}
		for i := 0; i < replicas; i++ {
			workerID := wcfg.ID
			if replicas > 1 {
				workerID = fmt.Sprintf("%s-%d", wcfg.ID, i)
			}

			if err := transport.Register(ctx, workerID); err != nil {
				log.Error("failed to register IPC socket for worker",
					zap.String("worker_id", workerID), zap.Error(err))
				continue
			}

			args := parseArgs(wcfg.Command)
			cmd := resolveCommand(args[0])
			cmdArgs := args[1:]

			if isWindows() {
				cmdArgs = append(cmdArgs, "--vyx-socket",
					`\\.\pipe\vyx-`+workerID)
			} else {
				cmdArgs = append(cmdArgs, "--vyx-socket",
					filepath.Join(socketDir, workerID+".sock"))
			}

			spawnCtx := ctx
			if wcfg.StartupTimeout > 0 {
				var cancel context.CancelFunc
				spawnCtx, cancel = context.WithTimeout(ctx, wcfg.StartupTimeout)
				defer cancel()
			}

			// Resolve relative working_dir against the config file's directory
			// so that workers with their own go.mod (or any sub-module) are
			// spawned with the correct absolute CWD regardless of where the
			// vyx binary itself was invoked from.
			workDir := wcfg.WorkingDir
			if workDir != "" && !filepath.IsAbs(workDir) {
				workDir = filepath.Join(filepath.Dir(configPath), workDir)
				if abs, err := filepath.Abs(workDir); err == nil {
					workDir = abs
				}
			}

			w, err := service.SpawnWorker(spawnCtx, workerID, cmd, cmdArgs, workDir)
			if err != nil {
				log.Error("failed to spawn worker",
					zap.String("worker_id", workerID),
					zap.String("command", wcfg.Command),
					zap.Error(err),
				)
				continue
			}
			log.Info("worker spawned",
				zap.String("worker_id", w.ID),
				zap.String("command", wcfg.Command),
				zap.Int("replica", i),
			)

			// Wait for the worker to connect and complete the IPC handshake.
			// The handshake handler retries until the worker dials the socket
			// or the startup-timeout context expires.
			hsHandler := handshake.NewHandler(transport, rm, service, log)
			if err := waitForHandshake(spawnCtx, hsHandler, workerID, log); err != nil {
				log.Error("worker handshake failed",
					zap.String("worker_id", workerID),
					zap.Error(err),
				)
				_ = service.StopWorker(ctx, workerID)
				continue
			}

			hbReceiver.StartLoop(ctx, w.ID)
		}
	}

	// --- Start background services ---
	go healthMonitor.Run(ctx)
	go cfgLoader.WatchSIGHUP(ctx)
	go hbSender.Run(ctx)
	go hbReceiver.Run(ctx)

	go func() {
		var srvErr error
		if gwCfg.TLSCertFile != "" {
			srvErr = httpServer.ListenAndServeTLS(gwCfg.TLSCertFile, gwCfg.TLSKeyFile)
		} else {
			srvErr = httpServer.ListenAndServe()
		}
		if srvErr != nil && srvErr.Error() != "http: Server closed" {
			log.Error("HTTP server stopped", zap.Error(srvErr))
		}
	}()

	<-ctx.Done()
	log.Info("vyx core shutting down — draining workers")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown error", zap.Error(err))
	}
	if err := transport.Close(); err != nil {
		log.Error("IPC transport close error", zap.Error(err))
	}
	if err := service.StopAll(shutdownCtx); err != nil {
		log.Error("error during graceful shutdown", zap.Error(err))
		os.Exit(1)
	}
	log.Info("vyx core stopped cleanly")
}

// ─── helpers ──────────────────────────────────────────────────────────────────────

func mustLoadConfig() *doamincfg.Config {
	log, _ := zap.NewProduction()
	cfgPath := defaultConfigPath
	if p := os.Getenv("VYX_CONFIG"); p != "" {
		cfgPath = p
	}
	loader := infracfg.New(cfgPath, log)
	cfg, err := loader.Load()
	if err != nil {
		fatalf("failed to load %s: %v", cfgPath, err)
	}
	return cfg
}

func parseArgs(command string) []string {
	return strings.Fields(command)
}

// resolveCommand converts a command token into an executable path that Go's
// exec package will accept on all platforms.
//
// On Windows, exec.LookPath refuses to run a binary found only in the current
// directory (Go 1.19+ security hardening). Converting a bare name or a
// "./"-prefixed token to an absolute path bypasses that restriction while
// keeping the behaviour correct on Unix as well.
func resolveCommand(cmd string) string {
	// Already absolute — nothing to do.
	if filepath.IsAbs(cmd) {
		return cmd
	}
	// Relative path (starts with ./ or .\) or bare name that lives in CWD:
	// resolve to absolute so exec.Command never hits the Windows dot-directory block.
	if abs, err := filepath.Abs(cmd); err == nil {
		if _, statErr := os.Stat(abs); statErr == nil {
			return abs
		}
	}
	// Fall back to the original token (e.g. "node", "python") so exec.LookPath
	// can find it on PATH as usual.
	return cmd
}

// waitForHandshake retries the handshake until the worker connects and sends
// its TypeHandshake frame, or until ctx expires. This bridges the gap between
// process spawn (which returns immediately) and the worker dialling the IPC
// socket (which may take a few hundred milliseconds for a compiled binary or
// several seconds for `go run`).
func waitForHandshake(ctx context.Context, h *handshake.Handler, workerID string, log *zap.Logger) error {
	const retryInterval = 500 * time.Millisecond
	for {
		err := h.Handle(ctx, workerID)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return fmt.Errorf("handshake timeout for worker %s: %w", workerID, ctx.Err())
		}
		// Worker hasn't connected yet — retry after a short sleep.
		log.Debug("waiting for worker to connect for handshake",
			zap.String("worker_id", workerID),
			zap.Error(err),
		)
		select {
		case <-ctx.Done():
			return fmt.Errorf("handshake timeout for worker %s: %w", workerID, ctx.Err())
		case <-time.After(retryInterval):
		}
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "vyx: "+format+"\n", args...)
	os.Exit(1)
}
