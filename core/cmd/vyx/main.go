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
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	"github.com/ElioNeto/vyx/core/application/handshake"
	"github.com/ElioNeto/vyx/core/application/heartbeat"
	"github.com/ElioNeto/vyx/core/application/lifecycle"
	"github.com/ElioNeto/vyx/core/application/monitor"
	doamincfg "github.com/ElioNeto/vyx/core/domain/config"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
	dlog "github.com/ElioNeto/vyx/core/domain/log"
	infracfg "github.com/ElioNeto/vyx/core/infrastructure/config"
	infragw "github.com/ElioNeto/vyx/core/infrastructure/gateway"
	ilog "github.com/ElioNeto/vyx/core/infrastructure/log"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/uds"
	"github.com/ElioNeto/vyx/core/infrastructure/logger"
	"github.com/ElioNeto/vyx/core/infrastructure/process"
	"github.com/ElioNeto/vyx/core/infrastructure/repository"
	"github.com/ElioNeto/vyx/core/cmd/tui"
	"github.com/fsnotify/fsnotify"
)

const defaultConfigPath = "vyx.yaml"

func main() {
	if len(os.Args) < 2 {
		runServer(false, false)
		return
	}

	subcmd := os.Args[1]

	// Check for --tui flag on dev
	withTUI := false
	switch subcmd {
	case "dev":
		for _, a := range os.Args[2:] {
			if a == "--tui" {
				withTUI = true
			}
		}
	}

	switch subcmd {
	case "new":
		if len(os.Args) < 3 {
			fatalf("usage: vyx new <project-name>")
		}
		cmdNew(os.Args[2])
	case "dev":
		runServer(true, withTUI)
	case "logs":
		cmdLogs()
	case "start":
		runServer(false, false)
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
  vyx dev           Start core in development mode (h2c, verbose logs)
  vyx dev --tui     Start core with interactive log viewer
  vyx logs          Launch the log viewer (TUI) for a running core
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

// ─── vyx logs ───────────────────────────────────────────────────────────────────

func cmdLogs() {
	fmt.Println("vyx logs: launching log viewer (standalone mode requires a running core)")
	fmt.Println("Tip: use 'vyx dev --tui' to start core with the log viewer")
}

// ─── vyx dev / vyx start ──────────────────────────────────────────────────────────────────

// newLogger creates a zap.Logger. If mux is non-nil, a copy of entries is also
// pushed to the multiplexer for TUI display.
func newLogger(devMode bool, mux *ilog.Multiplexer) (*zap.Logger, error) {
	pe := zap.NewProductionEncoderConfig()
	pe.TimeKey = "timestamp"
	pe.EncodeTime = zapcore.ISO8601TimeEncoder
	pe.MessageKey = "message"
	pe.LevelKey = "level"

	var base zapcore.Encoder
	var level zapcore.Level
	if devMode {
		ce := zap.NewDevelopmentEncoderConfig()
		ce.EncodeTime = zapcore.ISO8601TimeEncoder
		base = zapcore.NewConsoleEncoder(ce)
		level = zap.DebugLevel
	} else {
		base = zapcore.NewJSONEncoder(pe)
		level = zap.InfoLevel
	}

	// Core writes to stderr by default.
	ws := zapcore.Lock(os.Stderr)
	core := zapcore.NewCore(base, ws, level)

	// If a multiplexer is provided, also push every log entry into it.
	if mux != nil {
		tuiEncoder := zapcore.NewJSONEncoder(pe)
		tuiCore := zapcore.NewCore(tuiEncoder, muxWriteSyncer{mux: mux}, zap.DebugLevel)
		core = zapcore.NewTee(core, tuiCore)
	}

	return zap.New(core), nil
}

// muxWriteSyncer is a zapcore.WriteSyncer that decodes JSON and pushes to the
// multiplexer as a structured Entry.
type muxWriteSyncer struct {
	mux *ilog.Multiplexer
}

func (w muxWriteSyncer) Write(p []byte) (int, error) {
	var entry dlog.Entry
	if err := json.Unmarshal(p, &entry); err != nil {
		// Fallback: push raw text
		entry = dlog.Entry{
			Timestamp: time.Now(),
			Source:    "CORE",
			Level:     "INFO",
			Raw:       string(p),
		}
	} else {
		if entry.Source == "" {
			entry.Source = "CORE"
		}
	}
	w.mux.Push(entry)
	return len(p), nil
}

func (w muxWriteSyncer) Sync() error { return nil }

// workerWatchEntry maps a resolved working directory to one or more worker IDs
// that should be restarted when a file inside it changes.
type workerWatchEntry struct {
	workerIDs []string
	absDir    string
}

// hotReloadWatcher starts an fsnotify watcher in dev mode. It watches each
// worker's working_dir (falling back to ".") for source-file changes, debounces
// rapid bursts within a 300 ms window, then calls service.RestartWorker for
// every affected worker ID. RestartWorker already calls drainer.MarkDraining +
// drainer.Drain before killing the old process, so zero in-flight requests are
// dropped during the rolling restart.
//
// It also watches vyx.yaml itself: a change there sends SIGHUP to the current
// process so cfgLoader.WatchSIGHUP picks it up without a full restart.
func hotReloadWatcher(
	ctx context.Context,
	cfgPath string,
	workers []doamincfg.WorkerConfig,
	service *lifecycle.Service,
	log *zap.Logger,
) {
	const debounce = 300 * time.Millisecond

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error("hot reload: failed to create watcher", zap.Error(err))
		return
	}
	defer watcher.Close()

	// Build the dir → workerIDs index and register watches.
	entries := make([]workerWatchEntry, 0, len(workers))
	seenDirs := map[string]int{} // absDir → index in entries slice

	for _, wcfg := range workers {
		watchDir := wcfg.WorkingDir
		if watchDir == "" {
			watchDir = "."
		}
		if !filepath.IsAbs(watchDir) {
			watchDir = filepath.Join(filepath.Dir(cfgPath), watchDir)
		}
		abs, err := filepath.Abs(watchDir)
		if err != nil {
			log.Warn("hot reload: could not resolve worker dir",
				zap.String("worker_id", wcfg.ID),
				zap.String("dir", watchDir),
				zap.Error(err),
			)
			continue
		}

		replicas := wcfg.Replicas
		if replicas <= 0 {
			replicas = 1
		}

		// Collect all replica IDs for this config entry.
		var ids []string
		for i := 0; i < replicas; i++ {
			wid := wcfg.ID
			if replicas > 1 {
				wid = fmt.Sprintf("%s-%d", wcfg.ID, i)
			}
			ids = append(ids, wid)
		}

		if idx, ok := seenDirs[abs]; ok {
			// Multiple workers sharing the same dir — merge IDs.
			entries[idx].workerIDs = append(entries[idx].workerIDs, ids...)
		} else {
			seenDirs[abs] = len(entries)
			entries = append(entries, workerWatchEntry{absDir: abs, workerIDs: ids})
			if err := watcher.Add(abs); err != nil {
				log.Warn("hot reload: could not watch dir",
					zap.String("dir", abs),
					zap.Error(err),
				)
			} else {
				log.Info("🔭 hot reload watching", zap.String("dir", abs))
			}
		}
	}

	// Also watch vyx.yaml so config changes trigger SIGHUP.
	absCfgPath, _ := filepath.Abs(cfgPath)
	if err := watcher.Add(filepath.Dir(absCfgPath)); err != nil {
		log.Warn("hot reload: could not watch config dir",
			zap.String("path", absCfgPath),
			zap.Error(err),
		)
	}

	// Per-worker debounce timers: workerID → pending timer.
	type pending struct {
		timer  *time.Timer
		cancel chan struct{}
	}
	var mu sync.Mutex
	timers := map[string]*pending{}

	scheduleRestart := func(workerID string) {
		mu.Lock()
		defer mu.Unlock()

		if p, ok := timers[workerID]; ok {
			// Reset the debounce window.
			p.timer.Reset(debounce)
			return
		}

		cancelCh := make(chan struct{})
		t := time.AfterFunc(debounce, func() {
			select {
			case <-cancelCh:
				return
			case <-ctx.Done():
				return
			default:
			}

			mu.Lock()
			delete(timers, workerID)
			mu.Unlock()

			log.Info("🔄 hot reload: restarting worker",
				zap.String("worker_id", workerID),
			)
			restartCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			if err := service.RestartWorker(restartCtx, workerID); err != nil {
				log.Error("hot reload: restart failed",
					zap.String("worker_id", workerID),
					zap.Error(err),
				)
			}
		})
		timers[workerID] = &pending{timer: t, cancel: cancelCh}
	}

	for {
		select {
		case <-ctx.Done():
			// Cancel all pending timers on shutdown.
			mu.Lock()
			for _, p := range timers {
				p.timer.Stop()
				close(p.cancel)
			}
			mu.Unlock()
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}

			absEvt, _ := filepath.Abs(event.Name)

			// vyx.yaml changed → send SIGHUP to self so cfgLoader picks it up.
			if absEvt == absCfgPath {
				log.Info("🔄 hot reload: vyx.yaml changed — reloading config (SIGHUP)")
				if p, err := os.FindProcess(os.Getpid()); err == nil {
					_ = p.Signal(syscall.SIGHUP)
				}
				continue
			}

			if !isSourceFile(event.Name) {
				continue
			}

			// Find which entry owns this file.
			for _, entry := range entries {
				if strings.HasPrefix(absEvt, entry.absDir+string(filepath.Separator)) ||
					absEvt == entry.absDir {
					for _, wid := range entry.workerIDs {
						log.Debug("hot reload: source change detected",
							zap.String("file", event.Name),
							zap.String("worker_id", wid),
						)
						scheduleRestart(wid)
					}
					break
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Error("hot reload: watcher error", zap.Error(err))
		}
	}
}

func runServer(devMode bool, withTUI bool) {
	var mux *ilog.Multiplexer
	if withTUI {
		mux = ilog.New()
	}

	log, err := newLogger(devMode, mux)
	if err != nil {
		fatalf("logger: %v", err)
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
	drainer := lifecycle.NewWorkerDrainer()

	// Wire manager to capture worker output when TUI is enabled.
	var managerOpts []process.Option
	if mux != nil {
		managerOpts = append(managerOpts, process.WithLogWriter(muxLogWriter(mux)))
	}
	manager := process.New(managerOpts...)
	publisher := logger.New(log)

	// hbReceiver is created before lifecycle.Service so it can be passed in.
	hbCfg := heartbeat.DefaultConfig()
	hbReceiver := heartbeat.NewReceiver(transport, repo, nil, hbCfg, log)

	// lifecycle.Service needs the transport (to re-register on restart) and
	// the receiver (to re-arm the heartbeat loop after restart).
	service := lifecycle.NewService(repo, manager, publisher, transport, hbReceiver, drainer)

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
		drainer,
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

	// --- Start TUI in a separate goroutine when requested ---
	if mux != nil {
		go func() {
			if err := tui.Run(mux); err != nil {
				log.Error("tui exited with error", zap.Error(err))
			}
		}()
	}

	// --- Hot reload watcher (dev mode only) ---
	// Launched after service is fully wired so RestartWorker is safe to call.
	if devMode {
		go hotReloadWatcher(ctx, configPath, cfg.Workers, service, log)
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
				var spawnCancel context.CancelFunc
				if wcfg.StartupTimeout > 0 {
					spawnCtx, spawnCancel = context.WithTimeout(ctx, wcfg.StartupTimeout)
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

			// Use the root ctx for spawning so the process lifetime is NOT
			// tied to the startup_timeout context.  spawnCtx is only for the
			// handshake wait below.
			w, err := service.SpawnWorker(ctx, workerID, cmd, cmdArgs, workDir, wcfg.ShutdownTimeout)
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
				hsErr := waitForHandshake(spawnCtx, hsHandler, workerID, log)
				// Release the startup-timeout context immediately so it does not
				// leak for the lifetime of runServer.  Using defer inside a for
				// loop would accumulate all cancel funcs until the function exits.
				if spawnCancel != nil {
					spawnCancel()
				}
				if hsErr != nil {
					log.Error("worker handshake failed",
						zap.String("worker_id", workerID),
						zap.Error(hsErr),
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

// isSourceFile returns true if the file is a source file we want to watch for changes.
func isSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".ts", ".tsx", ".js":
		return true
	default:
		return false
	}
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

// muxLogWriter returns a process.LogWriter that pushes lines into the
// multiplexer for TUI display.
func muxLogWriter(mux *ilog.Multiplexer) process.LogWriter {
	return func(workerID string, line string) {
		entry := dlog.ParseEntry(workerID, line)
		if entry.Source == "" {
			entry.Source = workerID
		}
		mux.Push(entry)
	}
}
