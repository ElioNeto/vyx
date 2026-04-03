package ipc

import "context"

// Transport is the port (interface) that the application layer uses to send
// and receive messages. Concrete implementations live in infrastructure/ipc.
//
// Implementations must be safe for concurrent use.
type Transport interface {
	// Send serialises msg and writes it to the worker identified by workerID.
	Send(ctx context.Context, workerID string, msg Message) error

	// Receive blocks until a heartbeat or handshake message arrives from the
	// worker identified by workerID, or until ctx is cancelled.
	// Used by the heartbeat loop and handshake handler.
	Receive(ctx context.Context, workerID string) (Message, error)

	// ReceiveResponse blocks until a response or error message arrives from
	// the worker identified by workerID, or until ctx is cancelled.
	// Used by the gateway dispatcher to read worker responses without
	// competing with the heartbeat loop for reads on the same connection.
	ReceiveResponse(ctx context.Context, workerID string) (Message, error)

	// Register opens (or accepts) the UDS socket for the given worker.
	Register(ctx context.Context, workerID string) error

	// Deregister closes and removes the socket for the given worker.
	Deregister(ctx context.Context, workerID string) error

	// Close shuts down the entire transport, closing all open sockets.
	Close() error
}
