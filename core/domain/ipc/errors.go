package ipc

import "errors"

var (
	// ErrPayloadTooLarge is returned when a received payload exceeds MaxPayloadSize.
	ErrPayloadTooLarge = errors.New("ipc: payload exceeds maximum allowed size")
	// ErrUnknownMessageType is returned when the type byte is not recognised.
	ErrUnknownMessageType = errors.New("ipc: unknown message type")
	// ErrConnectionClosed is returned when a read/write is attempted on a closed conn.
	ErrConnectionClosed = errors.New("ipc: connection is closed")
	// ErrWorkerNotConnected is returned when no socket exists for the requested worker.
	ErrWorkerNotConnected = errors.New("ipc: worker is not connected")
)
