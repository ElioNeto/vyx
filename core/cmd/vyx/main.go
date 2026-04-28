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
	"github.com/ElioNeto/vyx/core/domain/circuit"
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
const defaultRouteMapPath = "./route_map.json"

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
		output = defaultRouteMapPath
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
		output = defaultRouteMapPath
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
		isNotFound := strings.Contains(err.Error(), "no required module") ||
			(func() bool {
				if errors.As(err, &exitErr) {
					return exitErr.ExitCode() == 127
				}
				return false
			})()
		if isNotFound {
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
	defer func() { _ = watcher.Close() }()

	entries, absCfgPath := initWatcherEntries(watcher, cfgPath, workers, log)

	startWatcherLoop(ctx, watcher, absCfgPath, entries, service, log, debounce)
}

func initWatcherEntries(
	watcher *fsnotify.Watcher,
	cfgPath string,
	workers []doamincfg.WorkerConfig,
	log *zap.Logger,
) ([]workerWatchEntry, string) {
	entries := make([]workerWatchEntry, 0, len(workers))
	seenDirs := map[string]int{}

	for _, wcfg := range workers {
		abs, ids := processWorkerForWatch(wcfg, cfgPath, log)
		if abs == "" {
			continue
		}

		if idx, ok := seenDirs[abs]; ok {
			entries[idx].workerIDs = append(entries[idx].workerIDs, ids...)
		} else {
			seenDirs[abs] = len(entries)
			entries = append(entries, workerWatchEntry{absDir: abs, workerIDs: ids})
			addWatchDir(watcher, abs, log)
		}
	}

	absCfgPath := resolveConfigPath(cfgPath)
	addWatchDir(watcher, filepath.Dir(absCfgPath), log)

	return entries, absCfgPath
}

// processWorkerForWatch resolves the watch directory and worker IDs for a single worker config.
// Returns the absolute directory and a slice of worker IDs to watch.
func processWorkerForWatch(wcfg doamincfg.WorkerConfig, cfgPath string, log *zap.Logger) (string, []string) {
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
		return "", nil
	}

	replicas := wcfg.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	ids := buildWorkerIDs(wcfg.ID, replicas)
	return abs, ids
}

// buildWorkerIDs generates worker IDs based on the base ID and replica count.
func buildWorkerIDs(baseID string, replicas int) []string {
	ids := make([]string, 0, replicas)
	for i := 0; i < replicas; i++ {
		wid := baseID
		if replicas > 1 {
			wid = fmt.Sprintf("%s-%d", baseID, i)
		}
		ids = append(ids, wid)
	}
	return ids
}

// addWatchDir adds a directory to the watcher, logging warnings on failure.
func addWatchDir(watcher *fsnotify.Watcher, dir string, log *zap.Logger) {
	if err := watcher.Add(dir); err != nil {
		log.Warn("hot reload: could not watch dir",
			zap.String("dir", dir),
			zap.Error(err),
		)
	} else {
		log.Info("🔭 hot reload watching", zap.String("dir", dir))
	}
}

// resolveConfigPath returns the absolute path to the config file.
func resolveConfigPath(cfgPath string) string {
	absCfgPath, _ := filepath.Abs(cfgPath)
	return absCfgPath
}

type pendingTimer struct {
	timer  *time.Timer
	cancel chan struct{}
}

func startWatcherLoop(
	ctx context.Context,
	watcher *fsnotify.Watcher,
	absCfgPath string,
	entries []workerWatchEntry,
	service *lifecycle.Service,
	log *zap.Logger,
	debounce time.Duration,
) {
	var mu sync.Mutex
	timers := map[string]*pendingTimer{}

	scheduleRestart := buildScheduleRestart(ctx, service, log, debounce, &mu, timers)

	for {
		select {
		case <-ctx.Done():
			cancelAllTimers(&mu, timers)
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			handleWatcherEvent(event, absCfgPath, entries, log, scheduleRestart)

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Error("hot reload: watcher error", zap.Error(err))
		}
	}
}

func buildScheduleRestart(
	ctx context.Context,
	service *lifecycle.Service,
	log *zap.Logger,
	debounce time.Duration,
	mu *sync.Mutex,
	timers map[string]*pendingTimer,
) func(string) {
	return func(workerID string) {
		mu.Lock()
		defer mu.Unlock()

		if p, ok := timers[workerID]; ok {
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
		timers[workerID] = &pendingTimer{timer: t, cancel: cancelCh}
	}
}

func handleWatcherEvent(
	event fsnotify.Event,
	absCfgPath string,
	entries []workerWatchEntry,
	log *zap.Logger,
	scheduleRestart func(string),
) {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
		return
	}

	absEvt, _ := filepath.Abs(event.Name)

	if absEvt == absCfgPath {
		log.Info("🔄 hot reload: vyx.yaml changed — reloading config (SIGHUP)")
		if p, err := os.FindProcess(os.Getpid()); err == nil {
			_ = p.Signal(syscall.SIGHUP)
		}
		return
	}

	if !isSourceFile(event.Name) {
		return
	}

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
}

func cancelAllTimers(mu *sync.Mutex, timers map[string]*pendingTimer) {
	mu.Lock()
	defer mu.Unlock()
	for _, p := range timers {
		p.timer.Stop()
		close(p.cancel)
	}
}

func runServer(devMode, withTUI bool) {
	mux := setupLogger(withTUI)
	log := mux.log

	cfgLoader, cfg := loadConfig(log)
	rm := loadRouteMap(cfg, log)
	transport := setupTransport(cfg, log)

	repo, drainer, manager, publisher := setupCoreServices(mux.mux, log)
	hbReceiver, service, healthMonitor := setupLifecycleServices(transport, repo, manager, publisher, drainer, log)

	jwtValidator, schemaValidator := setupValidators(cfg, log)
	dispatcher := setupDispatcher(rm, transport, jwtValidator, schemaValidator, cfg, drainer, log)

	rateLimiter := setupRateLimiter(cfg)
	gwCfg := setupHTTPServerConfig(devMode)
	httpServer := setupHTTPServer(gwCfg, dispatcher, rateLimiter, log)

	hbSender := heartbeat.NewSender(transport, repo, heartbeat.DefaultConfig(), log)

	ctx, stop := setupSignalHandling()
	defer stop()
	startServices(startServicesConfig{
		Ctx:           ctx,
		DevMode:       devMode,
		Mux:           mux.mux,
		Cfg:           cfg,
		Service:       service,
		Transport:     transport,
		CfgLoader:     cfgLoader,
		HbSender:      hbSender,
		HbReceiver:    hbReceiver,
		HealthMonitor: healthMonitor,
		HttpServer:    httpServer,
		GwCfg:         gwCfg,
		Log:           log,
	})

	waitForShutdown(ctx, log, httpServer, transport, service)
}

// loggerSetup holds the multiplexer and logger instances.
type loggerSetup struct {
	mux *ilog.Multiplexer
	log *zap.Logger
}

// setupLogger creates the logger and multiplexer if TUI is enabled.
func setupLogger(withTUI bool) loggerSetup {
	var mux *ilog.Multiplexer
	if withTUI {
		mux = ilog.New()
	}
	log, err := newLogger(false, mux) // devMode will be handled later
	if err != nil {
		fatalf("logger: %v", err)
	}
	return loggerSetup{mux: mux, log: log}
}

// loadConfig loads the vyx.yaml configuration.
func loadConfig(log *zap.Logger) (*infracfg.Loader, *doamincfg.Config) {
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
		zap.Bool("dev_mode", false), // will be updated later
	)
	return cfgLoader, cfg
}

// loadRouteMap loads the route_map.json file.
func loadRouteMap(cfg *doamincfg.Config, log *zap.Logger) *dgw.RouteMap {
	routeMapPath := cfg.Build.RouteMapOutput
	if routeMapPath == "" {
		routeMapPath = defaultRouteMapPath
	}
	if !filepath.IsAbs(routeMapPath) {
		routeMapPath = filepath.Join(filepath.Dir(os.Getenv("VYX_CONFIG")), routeMapPath)
	}
	rm, err := dgw.LoadRouteMap(routeMapPath)
	if err != nil {
		log.Warn("route_map.json not found — starting without routes",
			zap.String("path", routeMapPath), zap.Error(err))
		rm = dgw.NewRouteMap(nil)
	}
	return rm
}

// setupTransport creates the IPC transport.
func setupTransport(cfg *doamincfg.Config, log *zap.Logger) ipc.Transport {
	socketDir := cfg.IPC.SocketDir
	if socketDir == "" {
		socketDir = uds.DefaultSocketDir
	}
	transport := platformTransport(socketDir)
	log.Info("IPC transport initialised", zap.String("socket_dir", socketDir))
	return transport
}

// setupCoreServices initializes the core services (repo, drainer, manager, publisher).
func setupCoreServices(mux *ilog.Multiplexer, log *zap.Logger) (
	*repository.MemoryWorkerRepository, *lifecycle.WorkerDrainer, *process.Manager, *logger.EventPublisher) {
	repo := repository.NewMemoryWorkerRepository()
	drainer := lifecycle.NewWorkerDrainer()

	var managerOpts []process.Option
	if mux != nil {
		managerOpts = append(managerOpts, process.WithLogWriter(muxLogWriter(mux)))
	}
	manager := process.New(managerOpts...)
	publisher := logger.New(log)
	return repo, drainer, manager, publisher
}

// setupLifecycleServices initializes heartbeat receiver, lifecycle service, and health monitor.
func setupLifecycleServices(
	transport ipc.Transport,
	repo *repository.MemoryWorkerRepository,
	manager *process.Manager,
	publisher *logger.EventPublisher,
	drainer *lifecycle.WorkerDrainer,
	log *zap.Logger,
) (*heartbeat.Receiver, *lifecycle.Service, *monitor.Monitor) {
	hbCfg := heartbeat.DefaultConfig()
	hbReceiver := heartbeat.NewReceiver(transport, repo, nil, hbCfg, log)
	service := lifecycle.NewService(repo, manager, publisher, transport, hbReceiver, drainer)
	hbReceiver.SetService(service)
	healthMonitor := monitor.New(service, repo)
	return hbReceiver, service, healthMonitor
}

// setupValidators creates JWT and schema validators.
func setupValidators(cfg *doamincfg.Config, log *zap.Logger) (*infragw.JWTValidator, *infragw.SchemaValidator) {
	jwtSecret := os.Getenv(cfg.Security.JWTSecretEnv)
	if jwtSecret == "" {
		log.Warn("JWT secret env var not set — auth will reject all tokens",
			zap.String("env", cfg.Security.JWTSecretEnv))
	}
	return infragw.NewJWTValidator([]byte(jwtSecret)), infragw.NewSchemaValidator(cfg.Build.SchemasDir)
}

// setupDispatcher creates the gateway dispatcher.
func setupDispatcher(
	rm *dgw.RouteMap,
	transport ipc.Transport,
	jwtValidator *infragw.JWTValidator,
	schemaValidator *infragw.SchemaValidator,
	cfg *doamincfg.Config,
	drainer *lifecycle.WorkerDrainer,
	log *zap.Logger,
) *apgw.Dispatcher {
	return apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       jwtValidator,
		Schema:    schemaValidator,
		Timeout:   cfg.Security.GlobalTimeout,
		Log:       log,
		Drainer:   drainer,
	}, circuit.Config{
		Failures:    cfg.Security.CircuitBreaker.Failures,
		Cooldown:    cfg.Security.CircuitBreaker.Cooldown,
		HalfOpenMax: cfg.Security.CircuitBreaker.HalfOpenMax,
	})
}

// setupRateLimiter creates the rate limiter.
func setupRateLimiter(cfg *doamincfg.Config) *apgw.RateLimiter {
	return apgw.NewRateLimiter(
		cfg.Security.RateLimit.PerIP,
		cfg.Security.RateLimit.PerToken,
		time.Minute,
	)
}

// setupHTTPServerConfig creates the HTTP server configuration.
func setupHTTPServerConfig(devMode bool) infragw.Config {
	if devMode {
		return infragw.DevConfig()
	}
	return infragw.DefaultConfig()
}

// setupHTTPServer creates the HTTP server.
func setupHTTPServer(cfg infragw.Config, dispatcher *apgw.Dispatcher, rateLimiter *apgw.RateLimiter, log *zap.Logger) *infragw.Server {
	return infragw.New(cfg, dispatcher, rateLimiter, log)
}

// setupSignalHandling creates the context with signal handling.
func setupSignalHandling() (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	return ctx, stop
}

// startServicesConfig holds parameters for startServices to reduce parameter count.
type startServicesConfig struct {
	Ctx           context.Context
	DevMode       bool
	Mux           *ilog.Multiplexer
	Cfg           *doamincfg.Config
	Service       *lifecycle.Service
	Transport     ipc.Transport
	CfgLoader     *infracfg.Loader
	HbSender      *heartbeat.Sender
	HbReceiver    *heartbeat.Receiver
	HealthMonitor *monitor.Monitor
	HttpServer    *infragw.Server
	GwCfg         infragw.Config
	Log           *zap.Logger
}

// startServices starts all background services and spawns workers.
func startServices(cfg startServicesConfig) {
	if cfg.DevMode {
		cfg.Log.Info("vyx core starting in DEV mode", zap.String("addr", cfg.GwCfg.Addr))
	} else {
		cfg.Log.Info("vyx core starting", zap.String("addr", cfg.GwCfg.Addr))
	}

	if cfg.Mux != nil {
		go func() {
			if err := tui.Run(cfg.Mux); err != nil {
				cfg.Log.Error("tui exited with error", zap.Error(err))
			}
		}()
	}

	if cfg.DevMode {
		go hotReloadWatcher(cfg.Ctx, os.Getenv("VYX_CONFIG"), cfg.Cfg.Workers, cfg.Service, cfg.Log)
	}

	spawnWorkers(cfg.Ctx, cfg.Cfg, cfg.Service, cfg.Transport, cfg.Log, cfg.HbReceiver)

	go cfg.HealthMonitor.Run(cfg.Ctx)
	go cfg.CfgLoader.WatchSIGHUP(cfg.Ctx)
	go cfg.HbSender.Run(cfg.Ctx)
	go cfg.HbReceiver.Run(cfg.Ctx)

	go func() {
		var srvErr error
		if cfg.GwCfg.TLSCertFile != "" {
			srvErr = cfg.HttpServer.ListenAndServeTLS(cfg.GwCfg.TLSCertFile, cfg.GwCfg.TLSKeyFile)
		} else {
			srvErr = cfg.HttpServer.ListenAndServe()
		}
		if srvErr != nil && srvErr.Error() != "http: Server closed" {
			cfg.Log.Error("HTTP server stopped", zap.Error(srvErr))
		}
	}()
}

// spawnWorkers spawns all workers defined in the config.
func spawnWorkers(ctx context.Context, cfg *doamincfg.Config, service *lifecycle.Service, transport ipc.Transport, log *zap.Logger, hbReceiver *heartbeat.Receiver) {
	socketDir := cfg.IPC.SocketDir
	if socketDir == "" {
		socketDir = uds.DefaultSocketDir
	}
	for _, wcfg := range cfg.Workers {
		spawnWorker(ctx, wcfg, service, transport, socketDir, log, hbReceiver)
	}
}

// spawnWorker spawns a single worker with the given config.
func spawnWorker(ctx context.Context, wcfg doamincfg.WorkerConfig, service *lifecycle.Service, transport ipc.Transport, socketDir string, log *zap.Logger, hbReceiver *heartbeat.Receiver) {
	replicas := wcfg.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	for i := 0; i < replicas; i++ {
		workerID := buildWorkerID(wcfg.ID, i, replicas)
		spawnWorkerInstance(spawnWorkerInstanceConfig{
			Ctx:         ctx,
			WorkerID:    workerID,
			Wcfg:        wcfg,
			Service:     service,
			Transport:   transport,
			SocketDir:   socketDir,
			Log:         log,
			HbReceiver:  hbReceiver,
		})
	}
}

// buildWorkerID generates the worker ID based on index and replica count.
func buildWorkerID(baseID string, index, replicas int) string {
	if replicas > 1 {
		return fmt.Sprintf("%s-%d", baseID, index)
	}
	return baseID
}

// spawnWorkerInstanceConfig holds parameters for spawnWorkerInstance.
type spawnWorkerInstanceConfig struct {
	Ctx         context.Context
	WorkerID    string
	Wcfg        doamincfg.WorkerConfig
	Service     *lifecycle.Service
	Transport   ipc.Transport
	SocketDir   string
	Log         *zap.Logger
	HbReceiver  *heartbeat.Receiver
}

// spawnWorkerInstance spawns a single worker instance.
func spawnWorkerInstance(cfg spawnWorkerInstanceConfig) {
	if err := cfg.Transport.Register(cfg.Ctx, cfg.WorkerID); err != nil {
		cfg.Log.Error("failed to register IPC socket for worker",
			zap.String("worker_id", cfg.WorkerID), zap.Error(err))
		return
	}

	cmd, cmdArgs := prepareWorkerCommand(cfg.Wcfg, cfg.WorkerID, cfg.SocketDir)
	workDir := resolveWorkerDir(cfg.Wcfg, os.Getenv("VYX_CONFIG"))

	spawnCtx, spawnCancel := createSpawnContext(cfg.Ctx, cfg.Wcfg.StartupTimeout)
	defer spawnCancel()

	vyxDir := getVyxDir()
	w, err := cfg.Service.SpawnWorker(cfg.Ctx, lifecycle.SpawnWorkerConfig{
		ID:              cfg.WorkerID,
		Command:         cmd,
		Args:            cmdArgs,
		WorkDir:         workDir,
		ShutdownTimeout: cfg.Wcfg.ShutdownTimeout,
		RuntimeVersion:  cfg.Wcfg.RuntimeVersion,
		VyxDir:          vyxDir,
	})
	if err != nil {
		cfg.Log.Error("failed to spawn worker",
			zap.String("worker_id", cfg.WorkerID),
			zap.String("command", cfg.Wcfg.Command),
			zap.Error(err),
		)
		return
	}
	cfg.Log.Info("worker spawned",
		zap.String("worker_id", w.ID),
		zap.String("command", cfg.Wcfg.Command),
	)

	waitForWorkerHandshake(spawnCtx, cfg.WorkerID, cfg.Transport, cfg.Service, cfg.Log)
	startWorkerHeartbeat(cfg.Ctx, w.ID, cfg.HbReceiver, cfg.Log)
}

// prepareWorkerCommand prepares the command and arguments for a worker.
func prepareWorkerCommand(wcfg doamincfg.WorkerConfig, workerID, socketDir string) (string, []string) {
	args := parseArgs(wcfg.Command)
	cmd := resolveCommand(args[0])
	cmdArgs := args[1:]

	if isWindows() {
		cmdArgs = append(cmdArgs, "--vyx-socket", `\\.\pipe\vyx-`+workerID)
	} else {
		cmdArgs = append(cmdArgs, "--vyx-socket", filepath.Join(socketDir, workerID+".sock"))
	}
	return cmd, cmdArgs
}

// resolveWorkerDir resolves the working directory for a worker.
func resolveWorkerDir(wcfg doamincfg.WorkerConfig, configPath string) string {
	workDir := wcfg.WorkingDir
	if workDir != "" && !filepath.IsAbs(workDir) {
		workDir = filepath.Join(filepath.Dir(configPath), workDir)
		if abs, err := filepath.Abs(workDir); err == nil {
			workDir = abs
		}
	}
	return workDir
}

// createSpawnContext creates a context with timeout for worker spawning.
func createSpawnContext(ctx context.Context, startupTimeout time.Duration) (context.Context, context.CancelFunc) {
	if startupTimeout > 0 {
		return context.WithTimeout(ctx, startupTimeout)
	}
	return ctx, func() {
		// No-op: no timeout was set, so there is nothing to cancel.
	}
}

// getVyxDir returns the VYX_DIR environment variable or default.
func getVyxDir() string {
	vyxDir := os.Getenv("VYX_DIR")
	if vyxDir == "" {
		vyxDir = ".vyx"
	}
	return vyxDir
}

// waitForWorkerHandshake waits for the worker to complete IPC handshake.
func waitForWorkerHandshake(spawnCtx context.Context, workerID string, transport ipc.Transport, service *lifecycle.Service, log *zap.Logger) {
	hsHandler := handshake.NewHandler(transport, nil, service, log) // RouteMap not needed here
	hsErr := waitForHandshake(spawnCtx, hsHandler, workerID, log)
	if hsErr != nil {
		log.Error("worker handshake failed",
			zap.String("worker_id", workerID),
			zap.Error(hsErr),
		)
		_ = service.StopWorker(context.Background(), workerID)
	}
}

// startWorkerHeartbeat starts the heartbeat loop for a worker.
func startWorkerHeartbeat(ctx context.Context, workerID string, hbReceiver *heartbeat.Receiver, log *zap.Logger) {
	if hbReceiver != nil {
		hbReceiver.StartLoop(ctx, workerID)
	}
}

// waitForShutdown waits for the shutdown signal and performs graceful shutdown.
func waitForShutdown(ctx context.Context, log *zap.Logger, httpServer *infragw.Server, transport ipc.Transport, service *lifecycle.Service) {
	<-ctx.Done()
	log.Info("vyx core shutting down — draining workers")

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
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
	return func(workerID, line string) {
		entry := dlog.ParseEntry(workerID, line)
		if entry.Source == "" {
			entry.Source = workerID
		}
		mux.Push(entry)
	}
}
