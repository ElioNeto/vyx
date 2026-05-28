package uds

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testListener creates a listening UDS socket and returns its path for tests.
func testListener(t *testing.T) (string, net.Listener) {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/test.sock"
	ln, err := net.Listen("unix", path)
	require.NoError(t, err, "failed to create test listener")
	t.Cleanup(func() { ln.Close() })
	return path, ln
}

// testDialer returns a dial function that connects to the given listener.
func testDialer(ln net.Listener) func(ctx context.Context, socketPath string) (net.Conn, error) {
	return func(ctx context.Context, socketPath string) (net.Conn, error) {
		return net.DialTimeout("unix", socketPath, 2*time.Second)
	}
}

func TestNewPool_InvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := NewPool(context.Background(), PoolConfig{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SocketPath is required")
}

func TestNewPool_Prewarm(t *testing.T) {
	t.Parallel()

	path, ln := testListener(t)
	go func() {
		for {
			_, err := ln.Accept()
			if err != nil {
				return
			}
		}
	}()

	p, err := NewPool(context.Background(), PoolConfig{
		SocketPath: path,
		MinSize:    3,
		MaxSize:    10,
		DialTimeout: time.Second,
	})
	require.NoError(t, err)
	defer p.Close()

	stats := p.Stats()
	assert.Equal(t, 3, stats.Idle, "should have 3 pre-warmed connections")
	assert.Equal(t, 0, stats.InUse)
	assert.Equal(t, 3, stats.Total)
}

func TestPool_AcquireRelease(t *testing.T) {
	t.Parallel()

	path, ln := testListener(t)
	go func() {
		for {
			_, err := ln.Accept()
			if err != nil {
				return
			}
		}
	}()

	p, err := NewPool(context.Background(), PoolConfig{
		SocketPath:  path,
		MinSize:     1,
		MaxSize:     5,
		DialTimeout: time.Second,
	})
	require.NoError(t, err)
	defer p.Close()

	// Acquire a connection.
	c, err := p.Acquire(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, c)

	stats := p.Stats()
	assert.Equal(t, 0, stats.Idle)
	assert.Equal(t, 1, stats.InUse)
	assert.Equal(t, 1, stats.Total)

	// Release it back.
	p.Release(c)

	stats = p.Stats()
	assert.Equal(t, 1, stats.Idle)
	assert.Equal(t, 0, stats.InUse)
}

func TestPool_AcquireExhausted(t *testing.T) {
	t.Parallel()

	path, ln := testListener(t)
	acceptCh := make(chan struct{}, 16)
	go func() {
		for {
			_, err := ln.Accept()
			if err != nil {
				return
			}
			acceptCh <- struct{}{}
		}
	}()

	p, err := NewPool(context.Background(), PoolConfig{
		SocketPath:  path,
		MinSize:     1,
		MaxSize:     1, // Only 1 connection allowed
		DialTimeout: time.Second,
	})
	require.NoError(t, err)
	defer p.Close()

	// Wait for pre-warm to be accepted
	<-acceptCh

	// Acquire the only connection.
	_, err = p.Acquire(context.Background())
	require.NoError(t, err)

	// Pool is exhausted, acquire should fail.
	_, err = p.Acquire(context.Background())
	assert.ErrorIs(t, err, ErrPoolExhausted)
}

func TestPool_AcquireAfterClose(t *testing.T) {
	t.Parallel()

	path, ln := testListener(t)
	go func() {
		for {
			_, err := ln.Accept()
			if err != nil {
				return
			}
		}
	}()

	p, err := NewPool(context.Background(), PoolConfig{
		SocketPath:  path,
		MinSize:     1,
		MaxSize:     5,
		DialTimeout: time.Second,
	})
	require.NoError(t, err)

	p.Close()

	_, err = p.Acquire(context.Background())
	assert.ErrorIs(t, err, ErrPoolClosed)
}

func TestPool_ConcurrentAcquire(t *testing.T) {
	t.Parallel()

	path, ln := testListener(t)
	go func() {
		for {
			_, err := ln.Accept()
			if err != nil {
				return
			}
		}
	}()

	p, err := NewPool(context.Background(), PoolConfig{
		SocketPath:  path,
		MinSize:     2,
		MaxSize:     10,
		DialTimeout: time.Second,
	})
	require.NoError(t, err)
	defer p.Close()

	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := p.Acquire(context.Background())
			if err != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
			p.Release(c)
		}()
	}
	wg.Wait()

	stats := p.Stats()
	t.Logf("Pool stats after concurrent use: %+v", stats)
	assert.GreaterOrEqual(t, stats.Total, 2)
}

func TestPool_ClosedSafe(t *testing.T) {
	t.Parallel()

	path, ln := testListener(t)
	go func() {
		for {
			_, err := ln.Accept()
			if err != nil {
				return
			}
		}
	}()

	p, err := NewPool(context.Background(), PoolConfig{
		SocketPath:  path,
		MinSize:     1,
		MaxSize:     5,
		DialTimeout: time.Second,
	})
	require.NoError(t, err)

	// Close multiple times should not panic.
	p.Close()
	p.Close()
}

func TestPool_ReleaseNonPooledConn(t *testing.T) {
	t.Parallel()

	path, ln := testListener(t)
	go func() {
		for {
			_, err := ln.Accept()
			if err != nil {
				return
			}
		}
	}()

	p, err := NewPool(context.Background(), PoolConfig{
		SocketPath:  path,
		MinSize:     1,
		MaxSize:     5,
		DialTimeout: time.Second,
	})
	require.NoError(t, err)
	defer p.Close()

	// Releasing a non-pooled connection should just close it (no panic).
	dummy, _ := net.Dial("unix", path)
	p.Release(dummy)
}

func TestPool_Stats(t *testing.T) {
	t.Parallel()

	path, ln := testListener(t)
	go func() {
		for {
			_, err := ln.Accept()
			if err != nil {
				return
			}
		}
	}()

	p, err := NewPool(context.Background(), PoolConfig{
		SocketPath:  path,
		MinSize:     2,
		MaxSize:     10,
		DialTimeout: time.Second,
	})
	require.NoError(t, err)
	defer p.Close()

	stats := p.Stats()
	assert.Equal(t, 2, stats.Idle)
	assert.Equal(t, 0, stats.InUse)
	assert.Equal(t, 2, stats.Total)

	// Acquire one.
	c, _ := p.Acquire(context.Background())
	stats = p.Stats()
	assert.Equal(t, 1, stats.Idle)
	assert.Equal(t, 1, stats.InUse)
	assert.Equal(t, 2, stats.Total)

	p.Release(c)
}
