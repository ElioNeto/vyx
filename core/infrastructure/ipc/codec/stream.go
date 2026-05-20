package codec

import (
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/ipc"
)

const defaultChunkSize = 1 << 20 // 1 MB per chunk

// ChunkConfig controls how streaming splits large payloads. #7
type ChunkConfig struct {
	ChunkSize int
}

// StreamSplitter splits a payload into TypeStreamStart/Chunk/End messages. #7
type StreamSplitter struct {
	cfg ChunkConfig
}

// NewStreamSplitter creates a splitter with sensible defaults.
func NewStreamSplitter() *StreamSplitter {
	return &StreamSplitter{cfg: ChunkConfig{ChunkSize: defaultChunkSize}}
}

// Split divides a payload into a sequence of streaming messages.
// The first message is TypeStreamStart, followed by zero or more
// TypeStreamChunk messages, and finally TypeStreamEnd.
func (s *StreamSplitter) Split(streamID string, payload []byte) []ipc.Message {
	size := s.cfg.ChunkSize
	if size <= 0 {
		size = defaultChunkSize
	}

	totalLen := len(payload)
	var msgs []ipc.Message

	startPayload := fmt.Sprintf(`{"stream_id":"%s","total_size":%d}`, streamID, totalLen)
	msgs = append(msgs, ipc.Message{
		Type:    ipc.TypeStreamStart,
		Payload: []byte(startPayload),
	})

	for offset := 0; offset < totalLen; offset += size {
		end := offset + size
		if end > totalLen {
			end = totalLen
		}
		msgs = append(msgs, ipc.Message{
			Type:    ipc.TypeStreamChunk,
			Payload: payload[offset:end],
		})
	}

	endPayload := fmt.Sprintf(`{"stream_id":"%s"}`, streamID)
	msgs = append(msgs, ipc.Message{
		Type:    ipc.TypeStreamEnd,
		Payload: []byte(endPayload),
	})

	return msgs
}

// StreamAssembler reassembles streamed chunks into the full payload. #7
type StreamAssembler struct {
	streams map[string]*streamState
}

type streamState struct {
	totalSize int
	received  int
	chunks    [][]byte
}

// NewStreamAssembler creates an assembler.
func NewStreamAssembler() *StreamAssembler {
	return &StreamAssembler{
		streams: make(map[string]*streamState),
	}
}

// Feed processes one streaming message and returns the assembled payload
// when the stream is complete (nil otherwise). #7
func (a *StreamAssembler) Feed(msg ipc.Message) ([]byte, error) {
	switch msg.Type {
	case ipc.TypeStreamStart:
		var streamID string
		var totalSize int
		if _, err := fmt.Sscanf(string(msg.Payload), `{"stream_id":"%s","total_size":%d}`, &streamID, &totalSize); err != nil {
			return nil, fmt.Errorf("stream: parse start: %w", err)
		}
		a.streams[streamID] = &streamState{
			totalSize: totalSize,
		}
		return nil, nil

	case ipc.TypeStreamChunk:
		for id, state := range a.streams {
			_ = id
			state.chunks = append(state.chunks, msg.Payload)
			state.received += len(msg.Payload)
		}
		return nil, nil

	case ipc.TypeStreamEnd:
		var streamID string
		if _, err := fmt.Sscanf(string(msg.Payload), `{"stream_id":"%s"}`, &streamID); err != nil {
			return nil, fmt.Errorf("stream: parse end: %w", err)
		}
		state, ok := a.streams[streamID]
		if !ok {
			return nil, fmt.Errorf("stream: unknown stream %s", streamID)
		}
		delete(a.streams, streamID)

		total := make([]byte, 0, state.totalSize)
		for _, chunk := range state.chunks {
			total = append(total, chunk...)
		}
		return total, nil

	default:
		return nil, nil
	}
}
