// Package codec provides Codec implementations for the IPC layer.
package codec

import "github.com/vmihailenco/msgpack/v5"

// MsgPackCodec implements domain/ipc.Codec using MessagePack encoding.
// It is the default codec for small payloads (< arrow threshold).
type MsgPackCodec struct{}

// Marshal encodes v to MsgPack bytes.
func (MsgPackCodec) Marshal(v any) ([]byte, error) {
	return msgpack.Marshal(v)
}

// Unmarshal decodes MsgPack bytes into v.
func (MsgPackCodec) Unmarshal(data []byte, v any) error {
	return msgpack.Unmarshal(data, v)
}
