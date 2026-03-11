// Package uds also provides a Client for the worker side of the UDS connection.
// Workers use this to connect back to the core's socket.
package uds

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/framing"
)

// Client connects to a core-managed UDS socket and exposes Send/Receive.
// Intended for use by in-process worker stubs and integration tests.
type Client struct {
	mu   sync.Mutex
	conn net.Conn
}

// Dial connects to the socket at socketPath and returns a ready Client.
func Dial(ctx context.Context, socketPath string) (*Client, error) {
	var d net.Dialer
	c, err := d.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("uds: dial %s: %w", socketPath, err)
	}
	return &Client{conn: c}, nil
}

// Send writes a framed message to the core.
func (c *Client) Send(msg ipc.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return framing.Write(c.conn, msg)
}

// Receive reads one framed message from the core.
func (c *Client) Receive() (ipc.Message, error) {
	return framing.Read(c.conn)
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
