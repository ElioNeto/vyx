// Package ipc defines the domain contracts for inter-process communication.
// Zero external dependencies — only stdlib types.
package ipc

import "fmt"

// MessageType identifies the kind of message exchanged over the socket.
type MessageType byte

const (
	// TypeRequest is a message sent from the core to a worker (0x01).
	TypeRequest MessageType = 0x01
	// TypeResponse is a message sent from a worker back to the core (0x02).
	TypeResponse MessageType = 0x02
	// TypeHeartbeat is a periodic liveness ping sent by the worker (0x03).
	TypeHeartbeat MessageType = 0x03
	// TypeError signals a processing error returned by the worker (0x04).
	TypeError MessageType = 0x04
)

// String returns a human-readable label for logging.
func (t MessageType) String() string {
	switch t {
	case TypeRequest:
		return "request"
	case TypeResponse:
		return "response"
	case TypeHeartbeat:
		return "heartbeat"
	case TypeError:
		return "error"
	default:
		return fmt.Sprintf("unknown(0x%02x)", byte(t))
	}
}

// Message is the domain value object that travels over the wire.
//
// Wire format (little-endian):
//
//	┌──────────────────┬───────────────┬─────────────────────┐
//	│  Length (4 bytes)│ Type (1 byte) │   Payload (N bytes) │
//	└──────────────────┴───────────────┴─────────────────────┘
//
// Length covers ONLY the payload (not the header itself).
// Payload encoding is MsgPack for small messages; Apache Arrow for large
// datasets (handled at a higher layer, tracked by issue #7).
type Message struct {
	Type    MessageType
	Payload []byte
}

// MaxPayloadSize is a hard limit to prevent memory exhaustion (16 MiB).
const MaxPayloadSize = 16 << 20 // 16 MiB
