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

// HTTPBackend stores infrastructure state via a REST API.
// The API must support:
//   PUT    /state        — persist state (body: JSON state)
//   GET    /state        — retrieve state (returns JSON or 404)
//   DELETE /state        — remove state
//   PUT    /state/lock   — acquire lock (body: JSON LockInfo, returns true/false)
//   DELETE /state/lock   — release lock
type HTTPBackend struct {
	addr   string
	client *http.Client
}

// HTTPBackendConfig holds configuration for the HTTP state backend.
type HTTPBackendConfig struct {
	Addr string `json:"address"`
}

// NewHTTPBackend creates a new HTTP-backed state backend.
func NewHTTPBackend(cfg HTTPBackendConfig) *HTTPBackend {
	return &HTTPBackend{
		addr: cfg.Addr,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (b *HTTPBackend) do(method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, b.addr+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return b.client.Do(req)
}

// Init does nothing for HTTP backend — the server must already be running.
func (b *HTTPBackend) Init(ctx context.Context) error {
	resp, err := b.do(http.MethodGet, "/health", nil)
	if err != nil {
		return fmt.Errorf("http backend: health check failed: %w", err)
	}
	resp.Body.Close()
	return nil
}

// Lock acquires a lock via the HTTP API.
func (b *HTTPBackend) Lock(ctx context.Context, info infra.LockInfo) error {
	resp, err := b.do(http.MethodPut, "/state/lock", info)
	if err != nil {
		return &infra.ErrLockAcquisition{Info: info, Err: fmt.Errorf("http lock: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &infra.ErrLockAcquisition{Info: info, Err: fmt.Errorf("http lock: status %d", resp.StatusCode)}
	}

	var locked bool
	if err := json.NewDecoder(resp.Body).Decode(&locked); err != nil {
		return &infra.ErrLockAcquisition{Info: info, Err: fmt.Errorf("decode response: %w", err)}
	}
	if !locked {
		return &infra.ErrLockAcquisition{Info: info, Err: fmt.Errorf("lock already held")}
	}
	return nil
}

// Unlock releases a lock via the HTTP API.
func (b *HTTPBackend) Unlock(ctx context.Context, info infra.LockInfo) error {
	resp, err := b.do(http.MethodDelete, "/state/lock", info)
	if err != nil {
		return &infra.ErrLockAcquisition{Info: info, Err: fmt.Errorf("http unlock: %w", err)}
	}
	resp.Body.Close()
	return nil
}

// Get retrieves the state from the HTTP API.
func (b *HTTPBackend) Get(ctx context.Context) (*infra.State, error) {
	resp, err := b.do(http.MethodGet, "/state", nil)
	if err != nil {
		return nil, fmt.Errorf("http backend: get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http backend: get: status %d", resp.StatusCode)
	}

	var state infra.State
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, fmt.Errorf("http backend: parse state: %w", err)
	}
	return &state, nil
}

// Put persists the state via the HTTP API.
func (b *HTTPBackend) Put(ctx context.Context, state *infra.State) error {
	state.UpdatedAt = time.Now()
	resp, err := b.do(http.MethodPut, "/state", state)
	if err != nil {
		return fmt.Errorf("http backend: put: %w", err)
	}
	resp.Body.Close()
	return nil
}

// Delete removes the state via the HTTP API.
func (b *HTTPBackend) Delete(ctx context.Context) error {
	resp, err := b.do(http.MethodDelete, "/state", nil)
	if err != nil {
		return fmt.Errorf("http backend: delete: %w", err)
	}
	resp.Body.Close()
	return nil
}
