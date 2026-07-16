package state

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalBackendInit(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "infra.tfstate")

	backend, err := NewLocalBackend(statePath)
	require.NoError(t, err)

	err = backend.Init(context.Background())
	require.NoError(t, err)

	// State file should exist
	_, err = os.Stat(statePath)
	assert.NoError(t, err)
}

func TestLocalBackendPutGet(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "infra.tfstate")

	backend, err := NewLocalBackend(statePath)
	require.NoError(t, err)
	require.NoError(t, backend.Init(context.Background()))

	ctx := context.Background()

	// Put a state
	state := infra.NewState("test")
	state.Resources = append(state.Resources, infra.NewResource("bucket-1", "aws_s3_bucket", "aws"))
	state.Serial = 2

	err = backend.Put(ctx, state)
	require.NoError(t, err)

	// Get it back
	got, err := backend.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, state.Serial, got.Serial)
	assert.Len(t, got.Resources, 1)
	assert.Equal(t, infra.ResourceID("bucket-1"), got.Resources[0].ID)
}

func TestLocalBackendGetNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "nonexistent.tfstate")

	backend, err := NewLocalBackend(statePath)
	require.NoError(t, err)

	state, err := backend.Get(context.Background())
	require.NoError(t, err)
	assert.Nil(t, state)
}

func TestLocalBackendDelete(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "infra.tfstate")

	backend, err := NewLocalBackend(statePath)
	require.NoError(t, err)
	require.NoError(t, backend.Init(context.Background()))

	ctx := context.Background()

	// Put and delete
	state := infra.NewState("test")
	require.NoError(t, backend.Put(ctx, state))
	require.NoError(t, backend.Delete(ctx))

	// Should be gone
	got, err := backend.Get(ctx)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestLocalBackendLockUnlock(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "infra.tfstate")

	backend, err := NewLocalBackend(statePath)
	require.NoError(t, err)

	ctx := context.Background()
	info := infra.LockInfo{
		ID:        "test-lock",
		Operation: "plan",
		Who:       "tester",
	}

	// Lock
	err = backend.Lock(ctx, info)
	require.NoError(t, err)

	// Lock again should fail
	err = backend.Lock(ctx, info)
	assert.Error(t, err)
	assert.IsType(t, &infra.ErrLockAcquisition{}, err)

	// Unlock
	err = backend.Unlock(ctx, info)
	require.NoError(t, err)

	// Lock again should succeed
	err = backend.Lock(ctx, info)
	require.NoError(t, err)

	// Cleanup
	_ = backend.Unlock(ctx, info)
}

func TestLocalBackendConcurrentLock(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "concurrent.tfstate")

	backend, err := NewLocalBackend(statePath)
	require.NoError(t, err)

	ctx := context.Background()
	info := infra.LockInfo{ID: "concurrent", Operation: "apply", Who: "test"}

	// Acquire lock
	err = backend.Lock(ctx, info)
	require.NoError(t, err)

	// Try to acquire from another goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	var secondErr error
	go func() {
		defer wg.Done()
		secondErr = backend.Lock(ctx, info)
	}()
	wg.Wait()

	assert.Error(t, secondErr)
	_ = backend.Unlock(ctx, info)
}

func TestLocalBackendAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "atomic.tfstate")

	backend, err := NewLocalBackend(statePath)
	require.NoError(t, err)
	require.NoError(t, backend.Init(context.Background()))

	ctx := context.Background()

	// Write larger state
	state := infra.NewState("test")
	for i := 0; i < 100; i++ {
		r := infra.NewResource(
			infra.ResourceID(filepath.Join("res", itoa(i))),
			"mock",
			"mock",
		)
		r.Properties = map[string]any{"index": i}
		state.Resources = append(state.Resources, r)
	}

	err = backend.Put(ctx, state)
	require.NoError(t, err)

	// Read back
	got, err := backend.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Len(t, got.Resources, 100)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
