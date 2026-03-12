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
	// TypeHandshake is the registration frame sent by a worker on connect (0x05). #18
	TypeHandshake MessageType = 0x05
	// TypeWSOpen signals the start of a WebSocket session from the core to the worker (0x06). #19
	TypeWSOpen MessageType = 0x06
	// TypeWSMessage carries a WebSocket message frame (bidirectional) (0x07). #19
	TypeWSMessage MessageType = 0x07
	// TypeWSClose signals that a WebSocket session has ended (0x08). #19
	TypeWSClose MessageType = 0x08
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
	case TypeHandshake:
		return "handshake"
	case TypeWSOpen:
		return "ws_open"
	case TypeWSMessage:
		return "ws_message"
	case TypeWSClose:
		return "ws_close"
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

// HandshakePayload is the JSON structure sent by a worker in the TypeHandshake frame. #18
type HandshakePayload struct {
	WorkerID     string               `json:"worker_id"`
	Capabilities []HandshakeCapability `json:"capabilities"`
}

// HandshakeCapability declares a single route the worker implements. #18
type HandshakeCapability struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

// WSOpenPayload carries metadata for a new WebSocket session. #19
type WSOpenPayload struct {
	SessionID string            `json:"session_id"`
	Path      string            `json:"path"`
	Headers   map[string]string `json:"headers"`
	Claims    any               `json:"claims,omitempty"`
}

// WSMessagePayload wraps a single WebSocket data frame. #19
type WSMessagePayload struct {
	SessionID string `json:"session_id"`
	Data      []byte `json:"data"`
	IsBinary  bool   `json:"is_binary"`
}

// WSClosePayload signals session termination. #19
type WSClosePayload struct {
	SessionID string `json:"session_id"`
	Code      int    `json:"code"`
	Reason    string `json:"reason"`
}
