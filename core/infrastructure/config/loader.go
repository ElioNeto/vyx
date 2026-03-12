// Package config implements reading and hot-reloading of the vyx.yaml manifest.
package config

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	domaincfg "github.com/ElioNeto/vyx/core/domain/config"
)

// Loader reads vyx.yaml from disk and watches for SIGHUP to reload it.
type Loader struct {
	path string
	log  *zap.Logger

	mu     sync.RWMutex
	current *domaincfg.Config
}

// New creates a Loader for the given file path.
func New(path string, log *zap.Logger) *Loader {
	return &Loader{path: path, log: log}
}

// Load reads and validates vyx.yaml. It merges the file over top of the
// defaults, so omitted keys keep their sensible values.
// Returns an error if the file cannot be read, parsed, or fails validation.
func (l *Loader) Load() (*domaincfg.Config, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", l.path, err)
	}

	cfg := domaincfg.Defaults()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", l.path, err)
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

// WatchSIGHUP starts a goroutine that reloads the config whenever SIGHUP is
// received. Intended for development mode. Blocks until ctx is cancelled.
func (l *Loader) WatchSIGHUP(ctx context.Context) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	defer signal.Stop(ch)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			l.log.Info("SIGHUP received — reloading vyx.yaml", zap.String("path", l.path))
			cfg, err := l.Load()
			if err != nil {
				l.log.Error("config reload failed — keeping previous config", zap.Error(err))
				continue
			}
			l.mu.Lock()
			l.current = cfg
			l.mu.Unlock()
			l.log.Info("config reloaded successfully", zap.String("project", cfg.Project.Name))
		}
	}
}
