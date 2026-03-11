package ipc

import "context"

// Transport is the port (interface) that the application layer uses to send
// and receive messages. Concrete implementations live in infrastructure/ipc.
//
// Implementations must be safe for concurrent use.
type Transport interface {
	// Send serialises msg and writes it to the worker identified by workerID.
	Send(ctx context.Context, workerID string, msg Message) error

	// Receive blocks until a message arrives from the worker identified by
	// workerID, or until ctx is cancelled.
	Receive(ctx context.Context, workerID string) (Message, error)

	// Register opens (or accepts) the UDS socket for the given worker.
	Register(ctx context.Context, workerID string) error

	// Deregister closes and removes the socket for the given worker.
	Deregister(ctx context.Context, workerID string) error

	// Close shuts down the entire transport, closing all open sockets.
	Close() error
}
