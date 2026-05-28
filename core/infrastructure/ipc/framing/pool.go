package framing

import "sync"

// bufferPool reduces GC pressure by reusing small byte buffers for IPC frames.
// The pool creates 4KB buffers; buffers for frames larger than maxPoolSize
// are allocated directly and not pooled.
var bufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 4096) // 4KB initial capacity for small frames
		return &b
	},
}

// maxPoolSize is the maximum frame size for which the buffer pool is used.
// Frames larger than this allocate fresh buffers directly.
const maxPoolSize = 64 * 1024 // 64KB
