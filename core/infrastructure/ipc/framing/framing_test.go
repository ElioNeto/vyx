package framing_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/framing"
)

func TestWriteRead_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		msgType ipc.MessageType
		payload []byte
	}{
		{"request with payload", ipc.TypeRequest, []byte(`{"route":"/api/users"}`)},
		{"response with payload", ipc.TypeResponse, []byte(`{"id":1}`)},
		{"heartbeat empty payload", ipc.TypeHeartbeat, []byte{}},
		{"error with message", ipc.TypeError, []byte(`{"error":"not found"}`)},
		{"large payload", ipc.TypeResponse, bytes.Repeat([]byte{'x'}, 64*1024)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			msg := ipc.Message{Type: tt.msgType, Payload: tt.payload}

			if err := framing.Write(&buf, msg); err != nil {
				t.Fatalf("Write() error = %v", err)
			}

			got, err := framing.Read(&buf)
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}

			if got.Type != msg.Type {
				t.Errorf("Type: want %v, got %v", msg.Type, got.Type)
			}
			if !bytes.Equal(got.Payload, msg.Payload) {
				t.Errorf("Payload mismatch: want len=%d, got len=%d", len(msg.Payload), len(got.Payload))
			}
		})
	}
}

func TestRead_PayloadTooLarge(t *testing.T) {
	var buf bytes.Buffer
	// Manually craft a frame with length > MaxPayloadSize.
	header := make([]byte, 5)
	header[0] = 0xFF
	header[1] = 0xFF
	header[2] = 0xFF
	header[3] = 0x0F // 0x0FFFFFFF = 268,435,455 bytes >> 16MiB
	header[4] = byte(ipc.TypeRequest)
	buf.Write(header)

	_, err := framing.Read(&buf)
	if err != ipc.ErrPayloadTooLarge {
		t.Errorf("want ErrPayloadTooLarge, got %v", err)
	}
}

func TestRead_UnknownMessageType(t *testing.T) {
	var buf bytes.Buffer
	header := make([]byte, 5)
	header[0] = 0x02 // length = 2
	header[4] = 0xFF // unknown type
	buf.Write(header)
	buf.Write([]byte{0x01, 0x02})

	_, err := framing.Read(&buf)
	if err != ipc.ErrUnknownMessageType {
		t.Errorf("want ErrUnknownMessageType, got %v", err)
	}
}

func TestRead_TruncatedHeader(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0x00, 0x00}) // only 2 bytes instead of 5

	_, err := framing.Read(&buf)
	if err == nil {
		t.Error("expected error for truncated header, got nil")
	}
}

func TestRead_TruncatedPayload(t *testing.T) {
	var buf bytes.Buffer
	header := make([]byte, 5)
	header[0] = 0x10 // claims 16 bytes
	header[4] = byte(ipc.TypeRequest)
	buf.Write(header)
	buf.Write([]byte{0x01, 0x02}) // only 2 bytes provided

	_, err := framing.Read(&buf)
	if err == nil || err == io.EOF {
		// Should get an unexpected EOF, not nil
		if err == nil {
			t.Error("expected error for truncated payload, got nil")
		}
	}
}

func TestWriteRead_MultipleFrames(t *testing.T) {
	var buf bytes.Buffer

	msgs := []ipc.Message{
		{Type: ipc.TypeRequest, Payload: []byte("first")},
		{Type: ipc.TypeHeartbeat, Payload: []byte{}},
		{Type: ipc.TypeResponse, Payload: []byte("third")},
	}

	for _, m := range msgs {
		if err := framing.Write(&buf, m); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	for i, want := range msgs {
		got, err := framing.Read(&buf)
		if err != nil {
			t.Fatalf("Read() frame %d error = %v", i, err)
		}
		if got.Type != want.Type {
			t.Errorf("frame %d: Type want %v got %v", i, want.Type, got.Type)
		}
		if !bytes.Equal(got.Payload, want.Payload) {
			t.Errorf("frame %d: Payload mismatch", i)
		}
	}
}
