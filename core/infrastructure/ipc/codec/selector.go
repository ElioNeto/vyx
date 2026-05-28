package codec

import (
	"github.com/ElioNeto/vyx/core/domain/ipc"
)

const (
	defaultArrowThreshold     = 512 * 1024       // 512 KB
	defaultMMapThreshold      = 4 * 1024 * 1024  // 4 MB
	defaultStreamThreshold    = 256 * 1024 * 1024 // 256 MB
)

func SelectCodec(payloadSize int64, cfg ipc.TransferConfig) ipc.Codec {
	threshold := defaultArrowThreshold
	if cfg.ArrowThreshold > 0 {
		threshold = int(cfg.ArrowThreshold)
	}
	if payloadSize >= int64(threshold) {
		return ArrowCodec{}
	}
	return MsgPackCodec{}
}

func SelectMessageType(payloadSize int64, cfg ipc.TransferConfig) ipc.MessageType {
	threshold := defaultArrowThreshold
	if cfg.ArrowThreshold > 0 {
		threshold = int(cfg.ArrowThreshold)
	}
	mmapThreshold := defaultMMapThreshold
	if cfg.ArrowMMapThreshold > 0 {
		mmapThreshold = int(cfg.ArrowMMapThreshold)
	}
	streamThreshold := defaultStreamThreshold
	if cfg.ArrowStreamingThreshold > 0 {
		streamThreshold = int(cfg.ArrowStreamingThreshold)
	}

	if payloadSize < int64(threshold) {
		return ipc.TypeRequest
	}
	if payloadSize >= int64(streamThreshold) {
		return ipc.TypeStreamStart
	}
	if payloadSize >= int64(mmapThreshold) {
		return ipc.TypeArrowSHM
	}
	return ipc.TypeArrowData
}
