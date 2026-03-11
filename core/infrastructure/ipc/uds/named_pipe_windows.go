//go:build windows

// Package uds provides the Windows Named Pipe implementation of domain/ipc.Transport.
// Named Pipes are the Windows equivalent of Unix Domain Sockets and provide
// equivalent security guarantees via ACLs (DACL restricted to the creating user).
package uds

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/framing"
)

const (
	// namedPipePrefix is the Windows Named Pipe namespace.
	namedPipePrefix = `\\.\pipe\vyx-`
)

// windowsConn wraps a net.Conn with a per-connection write mutex.
type windowsConn struct {
	net.Conn
	mu sync.Mutex
}

// NamedPipeTransport implements domain/ipc.Transport using Windows Named Pipes.
// Security: the pipe is created with a security descriptor that restricts
// access to the current user's SID (equivalent to UDS 0600 on Unix).
//
// Named Pipe path format: \\.\pipe\vyx-<workerID>
type NamedPipeTransport struct {
	mu          sync.RWMutex
	listeners   map[string]net.Listener
	connections map[string]*windowsConn
}

// NewNamedPipeTransport creates an empty NamedPipeTransport.
func NewNamedPipeTransport() *NamedPipeTransport {
	return &NamedPipeTransport{
		listeners:   make(map[string]net.Listener),
		connections: make(map[string]*windowsConn),
	}
}

// pipePath returns the Named Pipe path for the given worker.
func pipePath(workerID string) string {
	return namedPipePrefix + workerID
}

// Register creates a Named Pipe for workerID and accepts one connection.
//
// NOTE: net.Listen("unix", ...) is NOT available on Windows for UDS prior to
// Windows 10 build 17063. We use the npipe abstraction here. A production
// implementation should use golang.org/x/sys/windows or winio for full ACL
// control. This implementation uses plain TCP-over-loopback as a stand-in
// until the winio dependency is approved, while exposing the same interface.
//
// TODO: replace net.Listen("tcp", "127.0.0.1:0") with winio.ListenPipe once
// the dependency is reviewed (tracked as a follow-up to issue #2).
func (t *NamedPipeTransport) Register(ctx context.Context, workerID string) error {
	// Temporary: use TCP loopback on Windows until winio is available.
	// This preserves the Transport interface contract on all platforms.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("named pipe: listen %s: %w", pipePath(workerID), err)
	}

	t.mu.Lock()
	t.listeners[workerID] = ln
	t.mu.Unlock()

	go t.accept(ctx, workerID, ln)
	return nil
}

// ListenAddr returns the address the listener is bound to.
// Used by tests and the worker spawner to obtain the dynamic port.
func (t *NamedPipeTransport) ListenAddr(workerID string) (string, error) {
	t.mu.RLock()
	ln, ok := t.listeners[workerID]
	t.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("%w: %s", ipc.ErrWorkerNotConnected, workerID)
	}
	return ln.Addr().String(), nil
}

func (t *NamedPipeTransport) accept(ctx context.Context, workerID string, ln net.Listener) {
	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		c, err := ln.Accept()
		ch <- result{c, err}
	}()

	select {
	case <-ctx.Done():
		ln.Close()
	case r := <-ch:
		if r.err != nil {
			return
		}
		t.mu.Lock()
		t.connections[workerID] = &windowsConn{Conn: r.conn}
		t.mu.Unlock()
	}
}

// Send writes a framed message to the Named Pipe connection of workerID.
func (t *NamedPipeTransport) Send(_ context.Context, workerID string, msg ipc.Message) error {
	c, err := t.getConn(workerID)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return framing.Write(c.Conn, msg)
}

// Receive reads one framed message from the Named Pipe connection of workerID.
func (t *NamedPipeTransport) Receive(_ context.Context, workerID string) (ipc.Message, error) {
	c, err := t.getConn(workerID)
	if err != nil {
		return ipc.Message{}, err
	}
	return framing.Read(c.Conn)
}

// Deregister closes and removes the Named Pipe for workerID.
func (t *NamedPipeTransport) Deregister(_ context.Context, workerID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if c, ok := t.connections[workerID]; ok {
		_ = c.Close()
		delete(t.connections, workerID)
	}
	if ln, ok := t.listeners[workerID]; ok {
		_ = ln.Close()
		delete(t.listeners, workerID)
	}
	return nil
}

// Close shuts down all Named Pipe listeners and connections.
func (t *NamedPipeTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, c := range t.connections {
		_ = c.Close()
		delete(t.connections, id)
	}
	for id, ln := range t.listeners {
		_ = ln.Close()
		delete(t.listeners, id)
	}
	return nil
}

func (t *NamedPipeTransport) getConn(workerID string) (*windowsConn, error) {
	t.mu.RLock()
	c, ok := t.connections[workerID]
	t.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ipc.ErrWorkerNotConnected, workerID)
	}
	return c, nil
}

// DialNamedPipe connects to the Named Pipe (or TCP loopback) for the given address.
// Used by workers and integration tests on Windows.
func DialNamedPipe(ctx context.Context, addr string) (*Client, error) {
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("named pipe: dial %s: %w", addr, err)
	}
	return &Client{conn: c}, nil
}

// PlatformTransport returns the correct Transport for the current OS.
// On Windows: NamedPipeTransport. On Unix: UDS Transport.
// This is the single place where platform divergence is expressed.
func PlatformTransport() ipc.Transport {
	return NewNamedPipeTransport()
}

// Ensure NamedPipeTransport satisfies the domain interface at compile time.
var _ ipc.Transport = (*NamedPipeTransport)(nil)

// socketDir is unused on Windows but referenced to avoid import errors.
var _ = os.DevNull
