//go:build windows

package main

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/sys/windows"
)

// dialNamedPipe connects to a Windows Named Pipe and returns a net.Conn.
// path must be in the form \\.\pipe\<name>.
// It retries up to 10 times (500 ms apart) waiting for the core to create the pipe.
func dialNamedPipe(path string) (net.Conn, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("named pipe dial: UTF16PtrFromString: %w", err)
	}

	const (
		maxAttempts = 10
		retryDelay  = 500 * time.Millisecond
	)

	var handle windows.Handle
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// WaitNamedPipe blocks until an instance is available (up to 2 s per call).
		waitPtr, _ := windows.UTF16PtrFromString(path)
		_ = windows.WaitNamedPipe(waitPtr, 2000)

		handle, err = windows.CreateFile(
			pathPtr,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			0,
			nil,
			windows.OPEN_EXISTING,
			0,
			0,
		)
		if err == nil {
			break
		}
		if attempt < maxAttempts {
			time.Sleep(retryDelay)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("named pipe dial: CreateFile(%q) after %d attempts: %w",
			path, maxAttempts, err)
	}

	return &pipeConn{handle: handle, path: path}, nil
}

// pipeConn wraps a windows.Handle as a net.Conn.
type pipeConn struct {
	handle windows.Handle
	path   string
}

func (c *pipeConn) Read(b []byte) (int, error) {
	var n uint32
	if err := windows.ReadFile(c.handle, b, &n, nil); err != nil {
		return 0, err
	}
	return int(n), nil
}

func (c *pipeConn) Write(b []byte) (int, error) {
	var n uint32
	if err := windows.WriteFile(c.handle, b, &n, nil); err != nil {
		return 0, err
	}
	return int(n), nil
}

func (c *pipeConn) Close() error {
	_ = windows.FlushFileBuffers(c.handle)
	return windows.CloseHandle(c.handle)
}

func (c *pipeConn) LocalAddr() net.Addr               { return pipeNetAddr(c.path) }
func (c *pipeConn) RemoteAddr() net.Addr              { return pipeNetAddr(c.path) }
func (c *pipeConn) SetDeadline(_ time.Time) error     { return nil }
func (c *pipeConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *pipeConn) SetWriteDeadline(_ time.Time) error { return nil }

type pipeNetAddr string

func (p pipeNetAddr) Network() string { return "namedpipe" }
func (p pipeNetAddr) String() string  { return string(p) }

var _ net.Conn = (*pipeConn)(nil)
