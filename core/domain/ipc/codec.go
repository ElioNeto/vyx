package ipc

// Codec encodes and decodes arbitrary values to/from a byte slice.
// The domain layer depends on this abstraction; concrete implementations
// (MsgPack, JSON, Arrow) live in infrastructure.
type Codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}
