package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	infracfg "github.com/ElioNeto/vyx/core/infrastructure/config"
)

func TestLoader_WithRouteMap(t *testing.T) {
	loader := infracfg.New("/tmp/test.yaml", nil)
	// Just verify it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("WithRouteMap panicked: %v", r)
		}
	}()
	loader.WithRouteMap("/tmp/route_map.json", nil)
}

func TestLoader_CurrentAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vyx.yaml")
	content := `
project:
  name: test-app
  version: 1.0.0
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loader := infracfg.New(path, nil)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Set current manually (since SetCurrent is not exported, we verify Load works)
	if cfg.Project.Name != "test-app" {
		t.Errorf("Project.Name = %q, want %q", cfg.Project.Name, "test-app")
	}
}

func TestLoader_MustLoad(t *testing.T) {
	loader := infracfg.New("/nonexistent.yaml", nil)
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustLoad should panic on error")
		}
	}()
	loader.MustLoad()
}

func TestLoader_LoadAndValidate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vyx.yaml")
	content := `
project:
  name: valid-app
workers:
  - id: node:api
    command: node index.js
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loader := infracfg.New(path, nil)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(cfg.Workers))
	}
	if cfg.Workers[0].ID != "node:api" {
		t.Errorf("Worker ID = %q, want %q", cfg.Workers[0].ID, "node:api")
	}
}

func TestLoader_ReloadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vyx.yaml")
	
	// Initial config
	content1 := `
project:
  name: version-1
`
	if err := os.WriteFile(path, []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}

	loader := infracfg.New(path, nil)
	cfg1, err := loader.Load()
	if err != nil {
		t.Fatalf("Load v1 failed: %v", err)
	}

	// Update config
	content2 := `
project:
  name: version-2
`
	if err := os.WriteFile(path, []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	cfg2, err := loader.Load()
	if err != nil {
		t.Fatalf("Load v2 failed: %v", err)
	}

	if cfg1.Project.Name == cfg2.Project.Name {
		t.Error("config should have changed between loads")
	}
}

func TestLoader_LoadWithIPCSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vyx.yaml")
	content := `
project:
  name: test-app
ipc:
  socket_dir: /tmp/custom-vyx
  arrow_threshold: 2mb
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loader := infracfg.New(path, nil)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.IPC.SocketDir != "/tmp/custom-vyx" {
		t.Errorf("IPC.SocketDir = %q, want %q", cfg.IPC.SocketDir, "/tmp/custom-vyx")
	}
}

func TestLoader_LoadWithSecuritySettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vyx.yaml")
	content := `
project:
  name: test-app
security:
  jwt_secret_env: MY_SECRET
  rate_limit:
    per_ip: 500
    per_token: 2000
  payload_max_size: 5mb
  global_timeout: 45s
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loader := infracfg.New(path, nil)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Security.JWTSecretEnv != "MY_SECRET" {
		t.Errorf("JWTSecretEnv = %q, want %q", cfg.Security.JWTSecretEnv, "MY_SECRET")
	}
	if cfg.Security.GlobalTimeout != 45*time.Second {
		t.Errorf("GlobalTimeout = %v, want 45s", cfg.Security.GlobalTimeout)
	}
}
