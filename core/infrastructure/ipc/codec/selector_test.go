package codec_test

import (
	"testing"

	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/codec"
)

func TestSelectCodec_SmallPayload_UsesMsgPack(t *testing.T) {
	cfg := ipc.TransferConfig{ArrowThreshold: 1024}
	c := codec.SelectCodec(512, cfg)
	if _, ok := c.(codec.MsgPackCodec); !ok {
		t.Error("expected MsgPackCodec for small payload")
	}
}

func TestSelectCodec_LargePayload_UsesArrow(t *testing.T) {
	cfg := ipc.TransferConfig{ArrowThreshold: 1024}
	c := codec.SelectCodec(2048, cfg)
	if _, ok := c.(codec.ArrowCodec); !ok {
		t.Error("expected ArrowCodec for large payload")
	}
}

func TestSelectCodec_DefaultThreshold(t *testing.T) {
	// Zero config should use default threshold (512KB)
	c := codec.SelectCodec(1024, ipc.TransferConfig{})
	if _, ok := c.(codec.MsgPackCodec); !ok {
		t.Error("expected MsgPackCodec for 1KB with default 512KB threshold")
	}

	c2 := codec.SelectCodec(1024*1024, ipc.TransferConfig{})
	if _, ok := c2.(codec.ArrowCodec); !ok {
		t.Error("expected ArrowCodec for 1MB with default 512KB threshold")
	}
}

func TestSelectMessageType_SmallPayload(t *testing.T) {
	cfg := ipc.TransferConfig{
		ArrowThreshold:          1024,
		ArrowMMapThreshold:      4096,
		ArrowStreamingThreshold: 1048576,
	}
	mt := codec.SelectMessageType(512, cfg)
	if mt != ipc.TypeRequest {
		t.Errorf("expected TypeRequest, got %v", mt)
	}
}

func TestSelectMessageType_ArrowData(t *testing.T) {
	cfg := ipc.TransferConfig{
		ArrowThreshold:          1024,
		ArrowMMapThreshold:      4096,
		ArrowStreamingThreshold: 1048576,
	}
	mt := codec.SelectMessageType(2048, cfg)
	if mt != ipc.TypeArrowData {
		t.Errorf("expected TypeArrowData, got %v", mt)
	}
}

func TestSelectMessageType_ArrowSHM(t *testing.T) {
	cfg := ipc.TransferConfig{
		ArrowThreshold:          1024,
		ArrowMMapThreshold:      4096,
		ArrowStreamingThreshold: 1048576,
	}
	mt := codec.SelectMessageType(8192, cfg)
	if mt != ipc.TypeArrowSHM {
		t.Errorf("expected TypeArrowSHM, got %v", mt)
	}
}

func TestSelectMessageType_Streaming(t *testing.T) {
	cfg := ipc.TransferConfig{
		ArrowThreshold:          1024,
		ArrowMMapThreshold:      4096,
		ArrowStreamingThreshold: 1048576,
	}
	mt := codec.SelectMessageType(2*1048576, cfg)
	if mt != ipc.TypeStreamStart {
		t.Errorf("expected TypeStreamStart, got %v", mt)
	}
}
