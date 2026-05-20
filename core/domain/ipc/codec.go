package ipc

// Codec encodes and decodes arbitrary values to/from a byte slice.
// The domain layer depends on this abstraction; concrete implementations
// (MsgPack, JSON, Arrow) live in infrastructure.
type Codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

// TransferConfig controls how the IPC layer selects between different
// serialisation and transport strategies based on payload size. #7
type TransferConfig struct {
	// ArrowThreshold is the minimum payload size (in bytes) to use Arrow
	// encoding instead of MsgPack. Defaults to 512 KB.
	ArrowThreshold int `json:"arrow_threshold,omitempty" yaml:"arrow_threshold,omitempty"`

	// ArrowMMapThreshold is the minimum payload size (in bytes) to use
	// shared-memory (mmap) transport instead of inline Arrow IPC bytes.
	// Defaults to 4 MB.
	ArrowMMapThreshold int `json:"arrow_mmap_threshold,omitempty" yaml:"arrow_mmap_threshold,omitempty"`

	// ArrowStreamingThreshold is the minimum payload size (in bytes) to
	// split the data into streaming chunks rather than a single message.
	// Defaults to 256 MB.
	ArrowStreamingThreshold int `json:"arrow_streaming_threshold,omitempty" yaml:"arrow_streaming_threshold,omitempty"`
}
