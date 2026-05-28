package codec_test

import (
	"testing"

	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/codec"
)

func TestStreamSplitter_New(t *testing.T) {
	ss := codec.NewStreamSplitter()
	if ss == nil {
		t.Fatal("NewStreamSplitter() returned nil")
	}
}

func TestStreamSplitter_SplitSmallPayload(t *testing.T) {
	ss := codec.NewStreamSplitter()
	payload := []byte("hello")
	msgs := ss.Split("stream-1", payload)

	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages (start+chunk+end), got %d", len(msgs))
	}

	if msgs[0].Type != ipc.TypeStreamStart {
		t.Errorf("msg[0].Type = %v, want TypeStreamStart", msgs[0].Type)
	}
	if msgs[1].Type != ipc.TypeStreamChunk {
		t.Errorf("msg[1].Type = %v, want TypeStreamChunk", msgs[1].Type)
	}
	if msgs[2].Type != ipc.TypeStreamEnd {
		t.Errorf("msg[2].Type = %v, want TypeStreamEnd", msgs[2].Type)
	}

	// Verify payload data
	if string(msgs[1].Payload) != "hello" {
		t.Errorf("chunk payload = %q, want %q", string(msgs[1].Payload), "hello")
	}
}

func TestStreamSplitter_SplitLargePayload(t *testing.T) {
	// Use smaller chunks for testing
	ss := codec.NewStreamSplitter()
	payload := make([]byte, 3*1024*1024) // 3 MB
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	msgs := ss.Split("large-stream", payload)

	// Expect start + 3 chunks (1MB each) + end = 5 messages
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages for 3MB payload, got %d", len(msgs))
	}

	if msgs[0].Type != ipc.TypeStreamStart {
		t.Errorf("msg[0].Type = %v, want TypeStreamStart", msgs[0].Type)
	}
	if msgs[4].Type != ipc.TypeStreamEnd {
		t.Errorf("msg[4].Type = %v, want TypeStreamEnd", msgs[4].Type)
	}

	// Check chunk sizes
	totalChunkSize := 0
	for i := 1; i <= 3; i++ {
		totalChunkSize += len(msgs[i].Payload)
	}
	if totalChunkSize != len(payload) {
		t.Errorf("total chunk size = %d, want %d", totalChunkSize, len(payload))
	}
}

func TestStreamAssembler_New(t *testing.T) {
	sa := codec.NewStreamAssembler()
	if sa == nil {
		t.Fatal("NewStreamAssembler() returned nil")
	}
}

func TestStreamAssembler_FullRoundTrip(t *testing.T) {
	ss := codec.NewStreamSplitter()
	sa := codec.NewStreamAssembler()

	original := []byte("the quick brown fox jumps over the lazy dog")
	msgs := ss.Split("t1", original)

	var result []byte
	for _, msg := range msgs {
		data, err := sa.Feed(msg)
		if err != nil {
			t.Fatalf("Feed() error = %v", err)
		}
		if data != nil {
			result = data
		}
	}

	if string(result) != string(original) {
		t.Errorf("assembled = %q, want %q", string(result), string(original))
	}
}

func TestStreamAssembler_MultipleStreams(t *testing.T) {
	sa := codec.NewStreamAssembler()

	// Feed stream start
	_, err := sa.Feed(ipc.Message{
		Type:    ipc.TypeStreamStart,
		Payload: []byte(`{"stream_id":"s1","total_size":5}`),
	})
	if err != nil {
		t.Fatalf("Feed start error = %v", err)
	}

	// Feed chunk
	_, err = sa.Feed(ipc.Message{
		Type:    ipc.TypeStreamChunk,
		Payload: []byte("hello"),
	})
	if err != nil {
		t.Fatalf("Feed chunk error = %v", err)
	}

	// Feed stream end
	data, err := sa.Feed(ipc.Message{
		Type:    ipc.TypeStreamEnd,
		Payload: []byte(`{"stream_id":"s1"}`),
	})
	if err != nil {
		t.Fatalf("Feed end error = %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("assembled = %q, want %q", string(data), "hello")
	}
}

func TestStreamAssembler_InvalidStart(t *testing.T) {
	sa := codec.NewStreamAssembler()
	_, err := sa.Feed(ipc.Message{
		Type:    ipc.TypeStreamStart,
		Payload: []byte("invalid json"),
	})
	if err == nil {
		t.Error("expected error for invalid start payload")
	}
}

func TestStreamAssembler_InvalidEnd(t *testing.T) {
	sa := codec.NewStreamAssembler()
	_, err := sa.Feed(ipc.Message{
		Type:    ipc.TypeStreamEnd,
		Payload: []byte(`{"stream_id":"s1"}`),
	})
	if err == nil {
		t.Error("expected error for unknown stream")
	}
}

func TestStreamAssembler_UnknownMessageType(t *testing.T) {
	sa := codec.NewStreamAssembler()
	data, err := sa.Feed(ipc.Message{
		Type:    ipc.TypeRequest,
		Payload: []byte("hello"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Error("expected nil data for unknown message type")
	}
}
