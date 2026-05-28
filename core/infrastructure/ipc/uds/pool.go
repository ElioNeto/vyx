package uds

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// DefaultPool settings.
const (
	DefaultMinSize    = 2
	DefaultMaxSize    = 16
	DefaultIdleTimeout = 30 * time.Second
	DefaultDialTimeout = 5 * time.Second
)

var (
	// ErrPoolClosed is returned when Acquire is called after Close.
	ErrPoolClosed = errors.New("uds pool: closed")
	// ErrPoolExhausted is returned when the pool is at max capacity
	// and no idle connections are available.
	ErrPoolExhausted = errors.New("uds pool: exhausted")
)

// PoolConfig holds the configuration for a UDS connection pool.
type PoolConfig struct {
	// SocketPath is the path to the Unix Domain Socket.
	SocketPath string

	// MinSize is the minimum number of idle connections to maintain.
	// Defaults to DefaultMinSize (2) when zero.
	MinSize int

	// MaxSize is the maximum total number of connections (idle + in-use).
	// Defaults to DefaultMaxSize (16) when zero.
	MaxSize int

	// IdleTimeout is the maximum time an idle connection stays open.
	// Connections idle longer than this are closed. Zero uses DefaultIdleTimeout.
	IdleTimeout time.Duration

	// DialTimeout is the timeout for establishing a new UDS connection.
	// Zero uses DefaultDialTimeout.
	DialTimeout time.Duration
}

// conn wraps a net.Conn with its last-used timestamp for idle reaping.
type pooledConn struct {
	net.Conn
	lastUsed time.Time
}

// Pool is a connection pool for UDS connections.
// It maintains a configurable number of pre-warmed connections
// and supports Acquire/Release semantics for callers.
//
// A zero-value Pool is NOT valid — use NewPool to create one.
type Pool struct {
	cfg PoolConfig

	mu        sync.Mutex
	idle      []*pooledConn
	total     int           // total connections (idle + in-use)
	inUse     int           // currently acquired count
	closed    bool
	closeOnce sync.Once

	dialFn func(ctx context.Context, socketPath string) (net.Conn, error)

	// reapTicker triggers idle connection reaping.
	reapTicker *time.Ticker
	reapDone   chan struct{}
}

// NewPool creates a UDS connection pool with the given configuration.
// It pre-warms the pool to cfg.MinSize connections.
// Returns an error if the initial connections cannot be established.
func NewPool(ctx context.Context, cfg PoolConfig) (*Pool, error) {
	if cfg.SocketPath == "" {
		return nil, errors.New("uds pool: SocketPath is required")
	}
	if cfg.MinSize <= 0 {
		cfg.MinSize = DefaultMinSize
	}
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = DefaultMaxSize
	}
	if cfg.MaxSize < cfg.MinSize {
		cfg.MaxSize = cfg.MinSize
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = DefaultIdleTimeout
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = DefaultDialTimeout
	}

	p := &Pool{
		cfg:     cfg,
		dialFn:  defaultDial,
		reapDone: make(chan struct{}),
	}

	// Pre-warm connections.
	for range cfg.MinSize {
		if err := p.dialAndAdd(ctx); err != nil {
			// If we can't even dial the first connection, fail fast.
			p.Close()
			return nil, fmt.Errorf("uds pool: pre-warm: %w", err)
		}
	}

	// Start idle reaper.
	p.reapTicker = time.NewTicker(cfg.IdleTimeout / 2)
	go p.reapLoop()

	return p, nil
}

// defaultDial is the default connection dialer.
func defaultDial(ctx context.Context, socketPath string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", socketPath)
}

// Acquire returns a connection from the pool.
// It blocks until a connection is available or ctx is cancelled.
// If the pool is empty and below MaxSize, a new connection is created.
// Callers MUST call Release when done.
func (p *Pool) Acquire(ctx context.Context) (net.Conn, error) {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}

	// Try to reuse an idle connection.
	for len(p.idle) > 0 {
		c := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]

		// Test if the connection is still alive.
		if err := p.isAlive(c); err != nil {
			p.total--
			p.mu.Unlock()
			c.Close()
			p.mu.Lock()
			continue
		}

		p.inUse++
		p.mu.Unlock()
		return c, nil
	}

	// No idle connections — create a new one if below max.
	if p.total >= p.cfg.MaxSize {
		p.mu.Unlock()
		return nil, ErrPoolExhausted
	}

	p.total++
	p.inUse++
	p.mu.Unlock()

	c, err := p.dialFn(ctx, p.cfg.SocketPath)
	if err != nil {
		p.mu.Lock()
		p.total--
		p.inUse--
		p.mu.Unlock()
		return nil, fmt.Errorf("uds pool: dial: %w", err)
	}

	return &pooledConn{Conn: c, lastUsed: time.Now()}, nil
}

// Release returns a connection to the pool for reuse.
// If the pool is closed or at capacity, the connection is closed instead.
func (p *Pool) Release(c net.Conn) {
	pc, ok := c.(*pooledConn)
	if !ok {
		// If it's not a pooled connection, just close it.
		c.Close()
		return
	}

	pc.lastUsed = time.Now()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed || len(p.idle) >= p.cfg.MaxSize {
		p.total--
		p.inUse--
		pc.Close()
		return
	}

	p.idle = append(p.idle, pc)
	p.inUse--
}

// Close shuts down the pool, closing all idle connections.
// In-use connections are NOT closed (callers retain ownership until Release).
func (p *Pool) Close() error {
	p.closeOnce.Do(func() {
		if p.reapTicker != nil {
			p.reapTicker.Stop()
		}
		close(p.reapDone)

		p.mu.Lock()
		defer p.mu.Unlock()

		p.closed = true
		for _, c := range p.idle {
			c.Close()
		}
		p.idle = nil
		p.total -= p.inUse // remaining are in-use
	})
	return nil
}

// Stats returns current pool statistics.
func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return PoolStats{
		Idle:  len(p.idle),
		InUse: p.inUse,
		Total: p.total,
	}
}

// PoolStats exposes pool metrics.
type PoolStats struct {
	Idle  int
	InUse int
	Total int
}

// dialAndAdd creates a new connection and adds it to the idle pool.
func (p *Pool) dialAndAdd(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, p.cfg.DialTimeout)
	defer cancel()

	c, err := p.dialFn(ctx, p.cfg.SocketPath)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		c.Close()
		return ErrPoolClosed
	}

	p.idle = append(p.idle, &pooledConn{Conn: c, lastUsed: time.Now()})
	p.total++
	return nil
}

// isAlive checks if the connection is still usable.
func (p *Pool) isAlive(c *pooledConn) error {
	// Setting a write deadline and sending a zero-byte probe.
	// This is a lightweight check that doesn't require actual data.
	return c.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
}

// reapLoop periodically closes connections that have been idle too long.
func (p *Pool) reapLoop() {
	for {
		select {
		case <-p.reapTicker.C:
			p.reap()
		case <-p.reapDone:
			return
		}
	}
}

func (p *Pool) reap() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	now := time.Now()
	keep := make([]*pooledConn, 0, len(p.idle))
	for _, c := range p.idle {
		if now.Sub(c.lastUsed) > p.cfg.IdleTimeout {
			p.total--
			c.Close()
		} else {
			keep = append(keep, c)
		}
	}

	// Replenish if below minimum.
	need := p.cfg.MinSize - len(keep)
	if need > 0 && p.total+need <= p.cfg.MaxSize {
		for range need {
			// Fire-and-forget replenish — best effort.
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), p.cfg.DialTimeout)
				defer cancel()
				_ = p.dialAndAdd(ctx)
			}()
		}
	}

	p.idle = keep
}
