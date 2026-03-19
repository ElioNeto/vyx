//go:build windows

// Package uds provides the Windows Named Pipe implementation of domain/ipc.Transport.
// Named Pipes are the Windows equivalent of Unix Domain Sockets and provide
// equivalent security guarantees via a DACL restricted to the creating user's SID.
package uds

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
	"unsafe"

	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/framing"
	"golang.org/x/sys/windows"
)

const (
	namedPipePrefix = `\\.\pipe\vyx-`
	pipeBufferSize  = 65536
)

// windowsConn wraps a net.Conn with a per-connection write mutex.
type windowsConn struct {
	net.Conn
	mu sync.Mutex
}

// NamedPipeTransport implements domain/ipc.Transport using Windows Named Pipes.
type NamedPipeTransport struct {
	mu          sync.RWMutex
	listeners   map[string]*namedPipeListener
	connections map[string]*windowsConn
}

type namedPipeListener struct {
	path   string
	handle windows.Handle
}

// NewNamedPipeTransport creates an empty NamedPipeTransport.
func NewNamedPipeTransport() *NamedPipeTransport {
	return &NamedPipeTransport{
		listeners:   make(map[string]*namedPipeListener),
		connections: make(map[string]*windowsConn),
	}
}

func pipePath(workerID string) string {
	return namedPipePrefix + workerID
}

// createSecureNamedPipe creates a Named Pipe with a DACL restricted to the
// current user's SID only (equivalent to Unix mode 0600).
func createSecureNamedPipe(path string) (windows.Handle, error) {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return windows.InvalidHandle, fmt.Errorf("named pipe: OpenCurrentProcessToken: %w", err)
	}
	defer token.Close()

	user, err := token.GetTokenUser()
	if err != nil {
		return windows.InvalidHandle, fmt.Errorf("named pipe: GetTokenUser: %w", err)
	}

	userSIDStr := user.User.Sid.String()
	sddl := fmt.Sprintf("O:%sD:(A;;GA;;;%s)", userSIDStr, userSIDStr)
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return windows.InvalidHandle, fmt.Errorf("named pipe: SecurityDescriptorFromString(%q): %w", sddl, err)
	}

	sa := &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
		InheritHandle:      0,
	}

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, fmt.Errorf("named pipe: UTF16PtrFromString: %w", err)
	}

	const (
		pipeAccessDuplex       = 0x00000003
		pipeTypeByte           = 0x00000000
		pipeReadModeByte       = 0x00000000
		pipeWait               = 0x00000000
		pipeUnlimitedInstances = uint32(255)
	)

	handle, err := windows.CreateNamedPipe(
		pathPtr,
		pipeAccessDuplex,
		pipeTypeByte|pipeReadModeByte|pipeWait,
		pipeUnlimitedInstances,
		pipeBufferSize,
		pipeBufferSize,
		0,
		sa,
	)
	if err != nil {
		return windows.InvalidHandle, fmt.Errorf("named pipe: CreateNamedPipe(%q): %w", path, err)
	}
	return handle, nil
}

// Register creates a Named Pipe for workerID and spawns a goroutine to accept
// exactly one connection. Safe to call again after Deregister (restart path).
func (t *NamedPipeTransport) Register(ctx context.Context, workerID string) error {
	path := pipePath(workerID)

	handle, err := createSecureNamedPipe(path)
	if err != nil {
		return err
	}

	l := &namedPipeListener{path: path, handle: handle}

	t.mu.Lock()
	// Clear any stale connection entry left by a previous run so that
	// getConn never returns a closed handle while acceptPipe is in flight.
	if old, ok := t.connections[workerID]; ok {
		_ = old.Close()
		delete(t.connections, workerID)
	}
	t.listeners[workerID] = l
	t.mu.Unlock()

	go t.acceptPipe(ctx, workerID, l)
	return nil
}

func (t *NamedPipeTransport) acceptPipe(ctx context.Context, workerID string, l *namedPipeListener) {
	type result struct{ err error }
	ch := make(chan result, 1)

	go func() {
		err := windows.ConnectNamedPipe(l.handle, nil)
		if err == windows.ERROR_PIPE_CONNECTED {
			err = nil
		}
		ch <- result{err}
	}()

	select {
	case <-ctx.Done():
		windows.CloseHandle(l.handle)
		return
	case r := <-ch:
		if r.err != nil {
			// ConnectNamedPipe failed — close this handle and bail.
			// The next Register call (triggered by a worker restart) will
			// create a fresh handle and try again.
			windows.CloseHandle(l.handle)
			return
		}
	}

	c := newHandleConn(l.handle, l.path)
	t.mu.Lock()
	t.connections[workerID] = &windowsConn{Conn: c}
	// The OS handle is now owned by handleConn — mark listener as consumed
	// so Deregister does not double-close it.
	l.handle = windows.InvalidHandle
	t.mu.Unlock()
}

// handleConn wraps a windows.Handle as a net.Conn.
type handleConn struct {
	handle windows.Handle
	path   string
	once   sync.Once
}

func newHandleConn(h windows.Handle, path string) *handleConn {
	return &handleConn{handle: h, path: path}
}

func (c *handleConn) Read(b []byte) (int, error) {
	var n uint32
	if err := windows.ReadFile(c.handle, b, &n, nil); err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, io.EOF
	}
	return int(n), nil
}

func (c *handleConn) Write(b []byte) (int, error) {
	var n uint32
	if err := windows.WriteFile(c.handle, b, &n, nil); err != nil {
		return 0, err
	}
	return int(n), nil
}

func (c *handleConn) Close() error {
	var closeErr error
	c.once.Do(func() {
		// Flush and disconnect before closing so the client side gets a
		// clean EOF rather than a broken-pipe error.
		_ = windows.FlushFileBuffers(c.handle)
		_ = windows.DisconnectNamedPipe(c.handle)
		closeErr = windows.CloseHandle(c.handle)
	})
	return closeErr
}

func (c *handleConn) LocalAddr() net.Addr  { return pipeAddr(c.path) }
func (c *handleConn) RemoteAddr() net.Addr { return pipeAddr(c.path) }

// SetDeadline, SetReadDeadline, SetWriteDeadline satisfy net.Conn.
// Named Pipes use blocking I/O; deadlines are not supported at this layer.
func (c *handleConn) SetDeadline(_ time.Time) error      { return nil }
func (c *handleConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *handleConn) SetWriteDeadline(_ time.Time) error { return nil }

// pipeAddr implements net.Addr for Named Pipe connections.
type pipeAddr string

func (p pipeAddr) Network() string { return "namedpipe" }
func (p pipeAddr) String() string  { return string(p) }

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
	if l, ok := t.listeners[workerID]; ok {
		if l.handle != windows.InvalidHandle {
			_ = windows.CloseHandle(l.handle)
		}
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
	for id, l := range t.listeners {
		if l.handle != windows.InvalidHandle {
			_ = windows.CloseHandle(l.handle)
		}
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

// DialNamedPipe connects to the Named Pipe for the given workerID.
func DialNamedPipe(ctx context.Context, workerID string) (*Client, error) {
	path := pipePath(workerID)
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("named pipe: UTF16PtrFromString: %w", err)
	}

	handle, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("named pipe: CreateFile(%q): %w", path, err)
	}

	return &Client{conn: newHandleConn(handle, path)}, nil
}

// PlatformTransport returns the Named Pipe transport for Windows.
func PlatformTransport() ipc.Transport {
	return NewNamedPipeTransport()
}

// Compile-time interface check.
var _ ipc.Transport = (*NamedPipeTransport)(nil)
