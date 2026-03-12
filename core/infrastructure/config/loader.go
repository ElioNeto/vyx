// Package config implements reading and hot-reloading of the vyx.yaml manifest
// and the route_map.json route table.
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	domaincfg "github.com/ElioNeto/vyx/core/domain/config"
)

// Loader reads vyx.yaml from disk and watches for SIGHUP to reload it.
// On every reload it also refreshes the RouteMap atomically (#41).
type Loader struct {
	configPath   string
	routeMapPath string
	log          *zap.Logger

	mu      sync.RWMutex
	current *domaincfg.Config

	// routeMap is the live RouteMap reference; may be nil when no route map path
	// is configured. Swapped atomically on SIGHUP (#41).
	routeMap *dgw.RouteMap
}

// New creates a Loader for the given config file path.
// routeMapPath may be empty; in that case route map reloading is skipped.
func New(configPath string, log *zap.Logger) *Loader {
	return &Loader{configPath: configPath, log: log}
}

// WithRouteMap configures the path to route_map.json that will be reloaded
// alongside vyx.yaml on SIGHUP. Call before WatchSIGHUP.
func (l *Loader) WithRouteMap(routeMapPath string, rm *dgw.RouteMap) {
	l.routeMapPath = routeMapPath
	l.routeMap = rm
}

// Load reads and validates vyx.yaml. It merges the file over top of the
// defaults, so omitted keys keep their sensible values.
// Returns an error if the file cannot be read, parsed, or fails validation.
func (l *Loader) Load() (*domaincfg.Config, error) {
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", l.configPath, err)
	}

	cfg := domaincfg.Defaults()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", l.configPath, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: validation failed: %w", err)
	}

	return &cfg, nil
}

// MustLoad calls Load and panics on any error. Useful for the startup path
// where a bad config should abort immediately.
func (l *Loader) MustLoad() *domaincfg.Config {
	cfg, err := l.Load()
	if err != nil {
		panic(err)
	}
	return cfg
}

// Current returns the most recently loaded config. Safe for concurrent use.
func (l *Loader) Current() *domaincfg.Config {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.current
}

// WatchSIGHUP starts a goroutine that reloads vyx.yaml (and route_map.json
// when configured) whenever SIGHUP is received. Blocks until ctx is cancelled.
func (l *Loader) WatchSIGHUP(ctx context.Context) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	defer signal.Stop(ch)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			l.reloadAll()
		}
	}
}

// reloadAll reloads vyx.yaml and, when configured, route_map.json.
func (l *Loader) reloadAll() {
	l.log.Info("SIGHUP received — reloading config", zap.String("path", l.configPath))

	cfg, err := l.Load()
	if err != nil {
		l.log.Error("config reload failed — keeping previous config", zap.Error(err))
	} else {
		l.mu.Lock()
		l.current = cfg
		l.mu.Unlock()
		l.log.Info("config reloaded", zap.String("project", cfg.Project.Name))
	}

	// Reload route_map.json atomically (#41).
	if l.routeMap != nil && l.routeMapPath != "" {
		l.reloadRouteMap()
	}
}

// reloadRouteMap reads route_map.json and swaps the in-memory RouteMap.
func (l *Loader) reloadRouteMap() {
	l.log.Info("reloading route map", zap.String("path", l.routeMapPath))

	data, err := os.ReadFile(l.routeMapPath)
	if err != nil {
		l.log.Error("route map reload failed — keeping previous routes",
			zap.String("path", l.routeMapPath),
			zap.Error(err),
		)
		return
	}

	var payload struct {
		Routes []dgw.RouteEntry `json:"routes"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		l.log.Error("route map parse failed — keeping previous routes",
			zap.String("path", l.routeMapPath),
			zap.Error(err),
		)
		return
	}

	// Atomic swap — no request sees a partially updated map (#41).
	l.routeMap.Swap(payload.Routes)
	l.log.Info("route map reloaded",
		zap.String("path", l.routeMapPath),
		zap.Int("routes", len(payload.Routes)),
	)
}
