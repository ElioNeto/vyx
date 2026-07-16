// Package state implements infrastructure state backends, including
// a Consul KV-backed state backend using the Consul HTTP REST API directly
// (no external SDK dependency).

package state

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

// ConsulBackend stores state in HashiCorp Consul's KV store with session-based locking.
type ConsulBackend struct {
	addr       string
	path       string
	datacenter string
	client     *http.Client
}

// ConsulBackendConfig holds configuration for the Consul state backend.
type ConsulBackendConfig struct {
	Addr       string `json:"address"`
	Path       string `json:"path"`
	Datacenter string `json:"datacenter,omitempty"`
}

// NewConsulBackend creates a new Consul-backed state backend.
func NewConsulBackend(cfg ConsulBackendConfig) (*ConsulBackend, error) {
	if cfg.Addr == "" {
		cfg.Addr = "http://localhost:8500"
	}
	if cfg.Path == "" {
		cfg.Path = "vyx/infra/tfstate"
	}

	return &ConsulBackend{
		addr:       cfg.Addr,
		path:       cfg.Path,
		datacenter: cfg.Datacenter,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// Init checks that Consul is reachable.
func (b *ConsulBackend) Init(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1/status/leader", b.addr)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("consul backend: create request: %w", err)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("consul backend: cannot reach %s: %w", b.addr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("consul backend: unexpected status %d from %s", resp.StatusCode, b.addr)
	}

	return nil
}

// Lock acquires a lock via Consul KV lock (using the consul lock API).
func (b *ConsulBackend) Lock(ctx context.Context, info infra.LockInfo) error {
	lockKey := b.path + "/.lock"
	url := fmt.Sprintf("%s/v1/kv/%s?acquire=%s", b.addr, lockKey, info.ID)

	body, err := json.Marshal(info)
	if err != nil {
		return &infra.ErrLockAcquisition{Info: info, Err: fmt.Errorf("marshal: %w", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return &infra.ErrLockAcquisition{Info: info, Err: fmt.Errorf("request: %w", err)}
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return &infra.ErrLockAcquisition{Info: info, Err: fmt.Errorf("consul lock: %w", err)}
	}
	defer resp.Body.Close()

	var result bool
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return &infra.ErrLockAcquisition{Info: info, Err: fmt.Errorf("decode response: %w", err)}
	}
	if !result {
		return &infra.ErrLockAcquisition{
			Info: info,
			Err:  fmt.Errorf("lock already held on %s", lockKey),
		}
	}

	return nil
}

// Unlock releases a lock by deleting the lock key.
func (b *ConsulBackend) Unlock(ctx context.Context, info infra.LockInfo) error {
	lockKey := b.path + "/.lock"
	url := fmt.Sprintf("%s/v1/kv/%s?release=%s", b.addr, lockKey, info.ID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return &infra.ErrLockAcquisition{Info: info, Err: fmt.Errorf("request: %w", err)}
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return &infra.ErrLockAcquisition{Info: info, Err: fmt.Errorf("consul unlock: %w", err)}
	}
	resp.Body.Close()

	return nil
}

// Get retrieves state from Consul KV.
func (b *ConsulBackend) Get(ctx context.Context) (*infra.State, error) {
	url := fmt.Sprintf("%s/v1/kv/%s?raw", b.addr, b.path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("consul backend: create request: %w", err)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("consul backend: get %s: %w", b.path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("consul backend: get %s: status %d", b.path, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("consul backend: read response: %w", err)
	}

	var state infra.State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("consul backend: parse state: %w", err)
	}

	return &state, nil
}

// Put persists state to Consul KV.
func (b *ConsulBackend) Put(ctx context.Context, state *infra.State) error {
	state.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("consul backend: marshal: %w", err)
	}

	url := fmt.Sprintf("%s/v1/kv/%s", b.addr, b.path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("consul backend: create put request: %w", err)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("consul backend: put %s: %w", b.path, err)
	}
	resp.Body.Close()

	return nil
}

// Delete removes state from Consul KV.
func (b *ConsulBackend) Delete(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1/kv/%s?recurse", b.addr, b.path)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("consul backend: create delete request: %w", err)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("consul backend: delete %s: %w", b.path, err)
	}
	resp.Body.Close()

	return nil
}
