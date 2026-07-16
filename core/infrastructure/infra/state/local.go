// Package state implements infrastructure state backends.
package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

// lockFileExt is the extension used for lock files.
const lockFileExt = ".lock"

// LocalBackend stores state in a local JSON file and uses a lock file for
// concurrency control. It implements infra.Backend.
type LocalBackend struct {
	statePath string
	lockPath  string
	mu        sync.Mutex
}

// NewLocalBackend creates a new LocalBackend for the given state file path.
func NewLocalBackend(statePath string) (*LocalBackend, error) {
	if statePath == "" {
		return nil, fmt.Errorf("state path is required")
	}

	absPath, err := filepath.Abs(statePath)
	if err != nil {
		return nil, fmt.Errorf("resolve state path: %w", err)
	}

	return &LocalBackend{
		statePath: absPath,
		lockPath:  absPath + lockFileExt,
	}, nil
}

// Init creates the parent directory for the state file if it doesn't exist.
func (b *LocalBackend) Init(ctx context.Context) error {
	dir := filepath.Dir(b.statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("local backend: create state dir %s: %w", dir, err)
	}

	// Create initial empty state if not exists.
	if _, err := os.Stat(b.statePath); os.IsNotExist(err) {
		emptyState := infra.NewState("default")
		return b.writeState(emptyState)
	}

	return nil
}

// Lock acquires a file-based lock. Returns an error if lock already held.
func (b *LocalBackend) Lock(ctx context.Context, info infra.LockInfo) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if lock file exists and is still valid.
	if data, err := os.ReadFile(b.lockPath); err == nil {
		var existing infra.LockInfo
		if err := json.Unmarshal(data, &existing); err == nil {
			// Check if lock has expired (default TTL: 2 minutes).
			ttl := 2 * time.Minute
			if time.Since(existing.CreatedAt) < ttl {
				return &infra.ErrLockAcquisition{
					Info: info,
					Err:  fmt.Errorf("lock held by %q since %s (operation: %s)", existing.Who, existing.CreatedAt.Format(time.RFC3339), existing.Operation),
				}
			}
			// Expired lock — overwrite.
		}
	}

	now := time.Now()
	info.CreatedAt = now
	if info.TTL == "" {
		info.TTL = "2m"
	}

	data, err := json.Marshal(info)
	if err != nil {
		return &infra.ErrLockAcquisition{
			Info: info,
			Err:  fmt.Errorf("marshal lock info: %w", err),
		}
	}

	if err := os.WriteFile(b.lockPath, data, 0644); err != nil {
		return &infra.ErrLockAcquisition{
			Info: info,
			Err:  fmt.Errorf("write lock file: %w", err),
		}
	}

	return nil
}

// Unlock releases the lock by removing the lock file.
func (b *LocalBackend) Unlock(ctx context.Context, info infra.LockInfo) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := os.Remove(b.lockPath); err != nil && !os.IsNotExist(err) {
		return &infra.ErrLockAcquisition{
			Info: info,
			Err:  fmt.Errorf("remove lock file: %w", err),
		}
	}
	return nil
}

// Get retrieves the current state from the file. Returns (nil, nil) if the
// state file does not exist.
func (b *LocalBackend) Get(ctx context.Context) (*infra.State, error) {
	data, err := os.ReadFile(b.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("local backend: read state file %s: %w", b.statePath, err)
	}

	var state infra.State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("local backend: parse state file %s: %w", b.statePath, err)
	}

	return &state, nil
}

// Put persists the state to the file.
func (b *LocalBackend) Put(ctx context.Context, state *infra.State) error {
	state.UpdatedAt = time.Now()
	return b.writeState(state)
}

// Delete removes the state file.
func (b *LocalBackend) Delete(ctx context.Context) error {
	if err := os.Remove(b.statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("local backend: remove state file %s: %w", b.statePath, err)
	}
	// Also remove lock file if present.
	_ = os.Remove(b.lockPath)
	return nil
}

// writeState is an internal helper that marshals and writes state atomically.
func (b *LocalBackend) writeState(state *infra.State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("local backend: marshal state: %w", err)
	}

	// Write atomically: write to temp file, then rename.
	tmpPath := b.statePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("local backend: write state: %w", err)
	}
	if err := os.Rename(tmpPath, b.statePath); err != nil {
		return fmt.Errorf("local backend: rename state file: %w", err)
	}

	return nil
}
