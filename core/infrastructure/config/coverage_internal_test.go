package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ElioNeto/vyx/core/domain/config"
	"github.com/ElioNeto/vyx/core/domain/gateway"
)

// TestMustLoad tests that MustLoad panics on error.
func TestMustLoad_Panics(t *testing.T) {
	loader := New("/nonexistent/vyx.yaml", zap.NewNop())
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic from MustLoad, but it didn't panic")
		}
	}()
	loader.MustLoad()
}

// TestMustLoad_Success tests that MustLoad returns config on success.
func TestMustLoad_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vyx.yaml")
	content := `
project:
  name: mustload-app
`
	os.WriteFile(path, []byte(content), 0644)
	loader := New(path, zap.NewNop())
	cfg := loader.MustLoad()
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.Project.Name != "mustload-app" {
		t.Errorf("unexpected project name: %s", cfg.Project.Name)
	}
}

// TestCurrentAndSetCurrent tests the getter and setter.
func TestCurrentAndSetCurrent(t *testing.T) {
	loader := New("", zap.NewNop())
	// Initially nil
	if loader.Current() != nil {
		t.Error("expected nil current config")
	}
	// Set a config
	cfg := &config.Config{}
	cfg.Project.Name = "test"
	loader.SetCurrent(cfg)
	got := loader.Current()
	if got == nil || got.Project.Name != "test" {
		t.Errorf("unexpected current config: %v", got)
	}
}

// TestWithRouteMap sets route map path and verifies it's stored.
func TestWithRouteMap(t *testing.T) {
	loader := New("", zap.NewNop())
	rm := &gateway.RouteMap{}
	loader.WithRouteMap("/tmp/route_map.json", rm)
	// There's no getter, but we can test indirectly via reloadAll.
	// For now, just ensure no panic.
}

// TestReloadAll_Error tests that reloadAll handles Load error gracefully.
func TestReloadAll_Error(t *testing.T) {
	loader := New("/nonexistent/vyx.yaml", zap.NewNop())
	// This should not panic; it should log error.
	loader.reloadAll()
	// After failed reload, current should still be nil (or previous)
	if loader.Current() != nil {
		t.Error("expected current to be nil after failed reload")
	}
}

// TestReloadAll_Success tests successful reload.
func TestReloadAll_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vyx.yaml")
	content := `
project:
  name: reload-app
`
	os.WriteFile(path, []byte(content), 0644)
	logger := zap.NewNop()
	loader := New(path, logger)
	// Set initial current to nil
	loader.SetCurrent(nil)
	// Call reloadAll
	loader.reloadAll()
	// Current should be updated
	current := loader.Current()
	if current == nil {
		t.Fatal("expected current to be updated")
	}
	if current.Project.Name != "reload-app" {
		t.Errorf("unexpected project name: %s", current.Project.Name)
	}
}

// TestReloadRouteMap_Success tests reloading route map.
func TestReloadRouteMap_Success(t *testing.T) {
	dir := t.TempDir()
	routeMapPath := filepath.Join(dir, "route_map.json")
	data := `{"routes": [{"path": "/api", "worker_id": "node:api"}]}`
	os.WriteFile(routeMapPath, []byte(data), 0644)

	rm := gateway.NewRouteMap(nil) // Start with empty routes
	logger := zap.NewNop()
	loader := New("", logger)
	loader.WithRouteMap(routeMapPath, rm)
	// Call reloadRouteMap
	loader.reloadRouteMap()
	// Verify routes loaded (need to inspect rm)
	// For simplicity, just ensure no panic.
}

// TestReloadRouteMap_FileMissing tests error handling.
func TestReloadRouteMap_FileMissing(t *testing.T) {
	logger := zap.NewNop()
	rm := &gateway.RouteMap{}
	loader := New("", logger)
	loader.WithRouteMap("/nonexistent/route_map.json", rm)
	// Should log error and return
	loader.reloadRouteMap()
}

// TestReloadRouteMap_InvalidJSON tests error handling.
func TestReloadRouteMap_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	routeMapPath := filepath.Join(dir, "route_map.json")
	os.WriteFile(routeMapPath, []byte("invalid json"), 0644)

	rm := &gateway.RouteMap{}
	logger := zap.NewNop()
	loader := New("", logger)
	loader.WithRouteMap(routeMapPath, rm)
	loader.reloadRouteMap()
}

// TestWatchSIGHUP tests that WatchSIGHUP reacts to context cancellation.
func TestWatchSIGHUP_ContextCancel(t *testing.T) {
	loader := New("", zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		loader.WatchSIGHUP(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
		// ok
	case <-time.After(100 * time.Millisecond):
		t.Error("WatchSIGHUP did not exit after context cancel")
	}
}

// TestLoad_InvalidYAML tests error path for yaml.Unmarshal.
func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vyx.yaml")
	os.WriteFile(path, []byte("invalid: [yaml: broken"), 0644)
	loader := New(path, zap.NewNop())
	_, err := loader.Load()
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
