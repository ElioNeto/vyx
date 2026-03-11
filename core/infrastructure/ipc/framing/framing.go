// Package framing implements the binary wire protocol used by vyx IPC.
//
// Frame layout (little-endian):
//
//	┌──────────────────┬───────────────┬─────────────────────┐
//	│  Length (4 bytes)│ Type (1 byte) │   Payload (N bytes) │
//	└──────────────────┴───────────────┴─────────────────────┘
//
// Length = number of payload bytes only (uint32, little-endian).
// This package has NO external dependencies — only stdlib io utilities.
package framing

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ElioNeto/vyx/core/domain/ipc"
)

const headerSize = 5 // 4 bytes length + 1 byte type

// Write encodes msg into the binary frame format and writes it to w.
// It performs a single vectorised write to minimise syscalls.
func Write(w io.Writer, msg ipc.Message) error {
	payloadLen := uint32(len(msg.Payload))

	header := make([]byte, headerSize)
	binary.LittleEndian.PutUint32(header[0:4], payloadLen)
	header[4] = byte(msg.Type)

	// Combine header + payload in one allocation to avoid two write syscalls.
	frame := make([]byte, headerSize+int(payloadLen))
	copy(frame, header)
	copy(frame[headerSize:], msg.Payload)

	_, err := w.Write(frame)
	return err
}

// Read reads one frame from r and returns the decoded Message.
// It validates the message type and enforces the MaxPayloadSize limit.
func Read(r io.Reader) (ipc.Message, error) {
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return ipc.Message{}, fmt.Errorf("framing: read header: %w", err)
	}

	payloadLen := binary.LittleEndian.Uint32(header[0:4])
	msgType := ipc.MessageType(header[4])

	if err := validateType(msgType); err != nil {
		return ipc.Message{}, err
	}

	if uint64(payloadLen) > uint64(ipc.MaxPayloadSize) {
		return ipc.Message{}, ipc.ErrPayloadTooLarge
	}

	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return ipc.Message{}, fmt.Errorf("framing: read payload: %w", err)
		}
	}

	return ipc.Message{Type: msgType, Payload: payload}, nil
}

func validateType(t ipc.MessageType) error {
	switch t {
	case ipc.TypeRequest, ipc.TypeResponse, ipc.TypeHeartbeat, ipc.TypeError:
		return nil
	}
	return ipc.ErrUnknownMessageType
}
