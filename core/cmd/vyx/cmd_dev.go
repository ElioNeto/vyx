package main

import (
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// DevHotReloader manages file watching for workers
type DevHotReloader struct {
	watcher *fsnotify.Watcher
	log     *zap.Logger
}

func NewDevHotReloader(log *zap.Logger) (*DevHotReloader, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &DevHotReloader{watcher: watcher, log: log}, nil
}

func (h *DevHotReloader) Watch(dir string) error {
	return h.watcher.Add(dir)
}

func (h *DevHotReloader) Close() error {
	return h.watcher.Close()
}
