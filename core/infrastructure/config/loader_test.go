package config_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"context"

	"go.uber.org/zap"

	infracfg "github.com/ElioNeto/vyx/core/infrastructure/config"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoader_Load_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "vyx.yaml", `
project:
  name: test-app
  version: 1.2.3
workers:
  - id: node:api
    command: node index.js
    replicas: 2
    strategy: round-robin
    startup_timeout: 10s
    shutdown_timeout: 5s
security:
  jwt_secret_env: MY_SECRET
  rate_limit:
    per_ip: 200
    per_token: 1000
  payload_max_size: 2mb
  global_timeout: 60s
ipc:
  socket_dir: /tmp/myapp
  arrow_threshold: 1mb
build:
  schemas_dir: ./my-schemas
  route_map_output: ./out/route_map.json
`)

	loader := infracfg.New(path, zap.NewNop())
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Project.Name != "test-app" {
		t.Errorf("expected project.name=test-app, got %q", cfg.Project.Name)
	}
	if cfg.Project.Version != "1.2.3" {
		t.Errorf("expected project.version=1.2.3, got %q", cfg.Project.Version)
	}
	if len(cfg.Workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(cfg.Workers))
	}
	w := cfg.Workers[0]
	if w.ID != "node:api" || w.Command != "node index.js" {
		t.Errorf("unexpected worker: %+v", w)
	}
	if w.Replicas != 2 || w.Strategy != "round-robin" {
		t.Errorf("unexpected worker replicas/strategy: %+v", w)
	}
	if w.StartupTimeout != 10*time.Second {
		t.Errorf("expected startup_timeout=10s, got %v", w.StartupTimeout)
	}
	if cfg.Security.JWTSecretEnv != "MY_SECRET" {
		t.Errorf("expected jwt_secret_env=MY_SECRET, got %q", cfg.Security.JWTSecretEnv)
	}
	if cfg.IPC.SocketDir != "/tmp/myapp" {
		t.Errorf("expected ipc.socket_dir=/tmp/myapp, got %q", cfg.IPC.SocketDir)
	}
	if cfg.Build.RouteMapOutput != "./out/route_map.json" {
		t.Errorf("unexpected route_map_output: %q", cfg.Build.RouteMapOutput)
	}
}

func TestLoader_Load_Defaults_Applied(t *testing.T) {
	dir := t.TempDir()
	// Minimal config — most keys omitted; defaults must fill in.
	path := writeFile(t, dir, "vyx.yaml", `
project:
  name: minimal-app
`)

	loader := infracfg.New(path, zap.NewNop())
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.IPC.SocketDir != "/tmp/vyx" {
		t.Errorf("expected default ipc.socket_dir=/tmp/vyx, got %q", cfg.IPC.SocketDir)
	}
	if cfg.Security.GlobalTimeout != 30*time.Second {
		t.Errorf("expected default global_timeout=30s, got %v", cfg.Security.GlobalTimeout)
	}
	if cfg.Build.SchemasDir != "./schemas" {
		t.Errorf("expected default schemas_dir=./schemas, got %q", cfg.Build.SchemasDir)
	}
}

func TestLoader_Load_MissingFile(t *testing.T) {
	loader := infracfg.New("/nonexistent/vyx.yaml", zap.NewNop())
	_, err := loader.Load()
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoader_Load_ValidationError_MissingProjectName(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "vyx.yaml", `
project:
  version: 1.0.0
`)

	loader := infracfg.New(path, zap.NewNop())
	_, err := loader.Load()
	if err == nil {
		t.Error("expected validation error for missing project.name")
	}
}

func TestLoader_Load_ValidationError_WorkerMissingCommand(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "vyx.yaml", `
project:
  name: my-app
workers:
  - id: node:api
`)

	loader := infracfg.New(path, zap.NewNop())
	_, err := loader.Load()
	if err == nil {
		t.Error("expected validation error for worker missing command")
	}
}

func TestLoader_Load_ValidationError_InvalidStrategy(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "vyx.yaml", `
project:
  name: my-app
workers:
  - id: node:api
    command: node index.js
    strategy: random
`)

	loader := infracfg.New(path, zap.NewNop())
	_, err := loader.Load()
	if err == nil {
		t.Error("expected validation error for invalid strategy")
	}
}

func TestLoader_WatchSIGHUP_ReloadsConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "vyx.yaml", `
project:
  name: original-app
`)

	loader := infracfg.New(path, zap.NewNop())
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("initial load failed: %v", err)
	}
	loader.SetCurrent(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go loader.WatchSIGHUP(ctx)

	// Update the file and send SIGHUP.
	writeFile(t, dir, "vyx.yaml", `
project:
  name: reloaded-app
`)

	time.Sleep(20 * time.Millisecond)
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGHUP)

	// Wait for reload to propagate.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if c := loader.Current(); c != nil && c.Project.Name == "reloaded-app" {
			return // success
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("config was not reloaded after SIGHUP")
}
