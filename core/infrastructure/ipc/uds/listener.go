// Package uds implements the domain/ipc.Transport interface using
// Unix Domain Sockets (UDS). On Windows, Named Pipes are used instead
// (see named_pipe_windows.go — tracked in a follow-up).
package uds

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/framing"
)

const (
	// DefaultSocketDir is where vyx creates its socket files.
	DefaultSocketDir = "/tmp/vyx"
	// socketPerm restricts socket access to the owner process only (0600).
	socketPerm = 0600

	// demuxChanSize is the buffer depth for typed message channels.
	// A small buffer prevents the read pump from blocking on a slow consumer.
	demuxChanSize = 16
)

// conn holds the active network connection for one worker plus a read pump
// that demultiplexes incoming frames by message type.
//
// The read pump runs in a dedicated goroutine and fans out messages to typed
// channels so that the heartbeat loop and the gateway dispatcher never race on
// the same read call.
type conn struct {
	net.Conn
	writeMu sync.Mutex // serialises concurrent writes to the same connection

	// Channels fed by the read pump — one per logical consumer.
	heartbeatCh chan ipc.Message // TypeHeartbeat + TypeHandshake
	responseCh  chan ipc.Message // TypeResponse + TypeError

	// pumpErr is set once by the read pump when the underlying connection
	// returns a non-nil error (typically io.EOF).  Subsequent Receive calls
	// return this cached error so callers learn about disconnection even
	// when the channel is drained.
	pumpMu  sync.Mutex
	pumpErr error
}

// startPump launches the background goroutine that reads frames from the
// underlying net.Conn and dispatches them to the appropriate typed channel.
// It must be called exactly once per conn.
func (c *conn) startPump() {
	go func() {
		for {
			msg, err := framing.Read(c.Conn)
			if err != nil {
				c.pumpMu.Lock()
				c.pumpErr = err
				c.pumpMu.Unlock()
				// Close channels so blocked receivers unblock with zero-value.
				close(c.heartbeatCh)
				close(c.responseCh)
				return
			}

			switch msg.Type {
			case ipc.TypeHeartbeat, ipc.TypeHandshake:
				select {
				case c.heartbeatCh <- msg:
				default:
					// Drop if consumer is too slow — heartbeat will be
					// counted as missed, which is the correct behaviour.
				}
				case ipc.TypeResponse, ipc.TypeError,
					ipc.TypeWSOpen, ipc.TypeWSMessage, ipc.TypeWSClose:
					select {
					case c.responseCh <- msg:
					default:
					}
				default:
				// Unknown type — route to response channel as a catch-all.
				select {
				case c.responseCh <- msg:
				default:
				}
			}
		}
	}()
}

// getPumpErr returns the cached read-pump error (if any).
func (c *conn) getPumpErr() error {
	c.pumpMu.Lock()
	defer c.pumpMu.Unlock()
	return c.pumpErr
}

// Transport is the UDS-backed implementation of domain/ipc.Transport.
// Each worker gets its own named socket: /tmp/vyx/<workerID>.sock
type Transport struct {
	socketDir string

	mu          sync.RWMutex
	listeners   map[string]net.Listener // workerID → server-side listener
	connections map[string]*conn        // workerID → accepted connection
}

// New creates a Transport that stores sockets under socketDir.
// Call Register(ctx, workerID) for each worker before using Send/Receive.
func New(socketDir string) *Transport {
	return &Transport{
		socketDir:   socketDir,
		listeners:   make(map[string]net.Listener),
		connections: make(map[string]*conn),
	}
}

// socketPath returns the filesystem path for a worker's socket.
func (t *Transport) socketPath(workerID string) string {
	return filepath.Join(t.socketDir, workerID+".sock")
}

// Register creates a UDS socket file for workerID, starts listening,
// and spawns a goroutine that accepts exactly one connection from the worker.
//
// The socket file is created with mode 0600 so only the owning process can
// connect — no authentication beyond OS-level DAC is needed for local IPC.
func (t *Transport) Register(ctx context.Context, workerID string) error {
	if err := os.MkdirAll(t.socketDir, 0700); err != nil {
		return fmt.Errorf("uds: create socket dir: %w", err)
	}

	path := t.socketPath(workerID)
	// Remove stale socket from a previous crashed run.
	_ = os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		return fmt.Errorf("uds: listen %s: %w", path, err)
	}

	// Enforce 0600 — net.Listen creates the file with umask-derived perms.
	if err := os.Chmod(path, socketPerm); err != nil {
		ln.Close()
		return fmt.Errorf("uds: chmod %s: %w", path, err)
	}

	t.mu.Lock()
	t.listeners[workerID] = ln
	t.mu.Unlock()

	// Accept the worker connection asynchronously.
	go t.accept(ctx, workerID, ln)

	return nil
}

// accept blocks until the worker connects, then stores the connection and
// starts the read pump.
func (t *Transport) accept(ctx context.Context, workerID string, ln net.Listener) {
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
		return
	case r := <-ch:
		if r.err != nil {
			return
		}
		c := &conn{
			Conn:        r.conn,
			heartbeatCh: make(chan ipc.Message, demuxChanSize),
			responseCh:  make(chan ipc.Message, demuxChanSize),
		}
		c.startPump()
		t.mu.Lock()
		t.connections[workerID] = c
		t.mu.Unlock()
	}
}

// Deregister closes the socket and removes the socket file for workerID.
func (t *Transport) Deregister(_ context.Context, workerID string) error {
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

	_ = os.Remove(t.socketPath(workerID))
	return nil
}

// Close shuts down all listeners and connections.
func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for id, c := range t.connections {
		_ = c.Close()
		delete(t.connections, id)
	}
	for id, ln := range t.listeners {
		_ = ln.Close()
		_ = os.Remove(t.socketPath(id))
		delete(t.listeners, id)
	}
	return nil
}

// Send serialises msg using the binary frame protocol and writes it to the
// connection associated with workerID. It holds the per-connection mutex to
// prevent interleaved frames from concurrent goroutines.
func (t *Transport) Send(_ context.Context, workerID string, msg ipc.Message) error {
	c, err := t.getConn(workerID)
	if err != nil {
		return err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return framing.Write(c.Conn, msg)
}

// Receive reads the next heartbeat/handshake message from the worker.
//
// The background read pump demultiplexes incoming frames by type:
//   - TypeHeartbeat and TypeHandshake are routed here.
//   - TypeResponse and TypeError are routed to ReceiveResponse.
//
// This method is used by the heartbeat loop and the handshake handler.
func (t *Transport) Receive(ctx context.Context, workerID string) (ipc.Message, error) {
	c, err := t.getConn(workerID)
	if err != nil {
		return ipc.Message{}, err
	}

	return t.receiveFrom(ctx, workerID, c.heartbeatCh, c)
}

// ReceiveResponse reads the next response/error message from the worker.
//
// The background read pump demultiplexes incoming frames by type:
//   - TypeResponse and TypeError are routed here.
//   - TypeHeartbeat and TypeHandshake are routed to Receive.
//
// This method is used by the gateway dispatcher.
func (t *Transport) ReceiveResponse(ctx context.Context, workerID string) (ipc.Message, error) {
	c, err := t.getConn(workerID)
	if err != nil {
		return ipc.Message{}, err
	}

	return t.receiveFrom(ctx, workerID, c.responseCh, c)
}

// receiveFrom is the shared implementation for Receive and ReceiveResponse.
func (t *Transport) receiveFrom(ctx context.Context, workerID string, ch <-chan ipc.Message, c *conn) (ipc.Message, error) {
	select {
	case <-ctx.Done():
		return ipc.Message{}, ctx.Err()
	case msg, ok := <-ch:
		if !ok {
			// Channel closed by read pump — return the pump error.
			if pErr := c.getPumpErr(); pErr != nil {
				return ipc.Message{}, fmt.Errorf("framing: read header: %w", pErr)
			}
			return ipc.Message{}, fmt.Errorf("%w: %s", ipc.ErrWorkerNotConnected, workerID)
		}
		return msg, nil
	}
}

func (t *Transport) getConn(workerID string) (*conn, error) {
	t.mu.RLock()
	c, ok := t.connections[workerID]
	t.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ipc.ErrWorkerNotConnected, workerID)
	}
	return c, nil
}
