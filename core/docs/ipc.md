# IPC Architecture — Unix Domain Sockets

This document describes the Inter-Process Communication (IPC) layer used by the vyx Core to communicate with all worker processes.

---

## Overview

The vyx Core is the **only process exposed to the network**. All workers are child processes that communicate exclusively with the Core via IPC. No worker ever receives a raw HTTP request.

```
[HTTP Client]
      │
      ▼
[Core Orchestrator (Go)]  ← only network-facing process
      │
      ├── /tmp/vyx/node-api.sock     ──▶ [Node.js Worker]
      ├── /tmp/vyx/python-orders.sock ──▶ [Python Worker]
      └── /tmp/vyx/go-users.sock     ──▶ [Go Worker]
```

---

## Transport

### Unix (Linux / macOS)

Each worker gets a dedicated **Unix Domain Socket** at:

```
/tmp/vyx/<worker-id>.sock
```

Socket files are created with permission `0600` (owner read/write only), enforced via an explicit `chmod` after `net.Listen` — the process umask is not relied upon.

### Windows

On Windows, **Named Pipes** are used as the transport (`\\.\pipe\vyx-<worker-id>`). The interface is identical from the application layer's perspective — the platform switch happens exclusively in `infrastructure/ipc/uds/platform_unix.go` and `named_pipe_windows.go` via build tags.

> **Current status**: the Windows implementation uses TCP loopback (`127.0.0.1:0`) as a stand-in until `winio` is reviewed. Full Named Pipe ACL support is a tracked follow-up.

---

## Wire Protocol

All messages use a **5-byte fixed header** followed by a variable-length payload:

```
┌──────────────────────┬───────────────┬──────────────────────┐
│   Length  (4 bytes)  │ Type (1 byte) │  Payload  (N bytes)  │
│   uint32, LE         │  0x01–0x04    │  MsgPack or Arrow    │
└──────────────────────┴───────────────┴──────────────────────┘
```

- **Length**: number of payload bytes only (does not include the header).
- **Encoding**: `uint32` little-endian.
- **`io.ReadFull`** is used for all reads to handle partial socket writes correctly.

### Message Types

| Hex    | Constant        | Direction        | Description |
|--------|-----------------|------------------|-------------|
| `0x01` | `TypeRequest`   | Core → Worker    | Dispatch an HTTP request to the worker |
| `0x02` | `TypeResponse`  | Worker → Core    | Worker's HTTP response |
| `0x03` | `TypeHeartbeat` | Worker → Core    | Periodic liveness signal (every 5s) |
| `0x04` | `TypeError`     | Worker → Core    | Worker-level processing error |

---

## Payload Serialisation

### MsgPack (default — small payloads)

All request and response payloads use **MessagePack** (`vmihailenco/msgpack/v5`) by default. MsgPack is ~30% smaller than JSON and significantly faster to encode/decode.

```go
type RequestPayload struct {
    Route   string            `msgpack:"route"`
    Method  string            `msgpack:"method"`
    Headers map[string]string `msgpack:"headers"`
    Body    []byte            `msgpack:"body"`
    Claims  map[string]any    `msgpack:"claims"` // JWT claims, not raw token
}
```

### Apache Arrow (large datasets — issue #7)

When a worker returns a large dataset (e.g., a report or list), the payload is an **Arrow `RecordBatch`**, shared via `mmap`/shared memory for zero-copy. The `domain/ipc.Codec` interface abstracts this — the framing layer is unaware of the encoding.

---

## Heartbeat Protocol

Workers send a `TypeHeartbeat` frame (empty payload) every **5 seconds**. The Core reads these via `application/heartbeat.Loop`, which runs per worker in a dedicated goroutine.

```
Worker                          Core
  │                              │
  │──── TypeHeartbeat ──────────▶│  RecordHeartbeat(workerID)
  │  (every 5s)                  │
  │                              │  if 2 consecutive misses:
  │  (silence > 10s)             │    MarkUnhealthy(workerID)
  │                              │    → monitor triggers restart
```

### Missed heartbeat logic

The heartbeat loop uses a **per-read deadline** (not a global ticker):

1. Each `Receive` call has a deadline of `interval` (5s).
2. A timeout counts as a missed heartbeat.
3. After `missedThreshold` (default: 2) consecutive misses → `MarkUnhealthy`.
4. A successful `TypeHeartbeat` **resets the counter to 0**.

This means a worker has up to `interval × missedThreshold = 10s` of silence before being restarted.

---

## Concurrency Model

| Concern | Solution |
|---------|----------|
| Multiple goroutines writing to the same worker socket | Per-connection `sync.Mutex` on writes |
| Multiple goroutines reading from the same worker socket | Single reader goroutine per worker (heartbeat loop) |
| Worker connecting before Core is ready | `Register` spawns an `Accept` goroutine; connection stored when worker connects |
| Stale socket from previous crash | `os.Remove(path)` before `net.Listen` |
| Memory exhaustion from oversized payload | Hard 16 MiB limit enforced in `framing.Read` |

---

## Package Structure

```
core/
├── domain/ipc/
│   ├── message.go       # Message value object, MessageType constants
│   ├── errors.go        # Typed sentinel errors
│   ├── transport.go     # Transport port (interface)
│   └── codec.go         # Codec port (interface)
│
├── application/heartbeat/
│   └── loop.go          # 5s heartbeat read loop (application use case)
│
└── infrastructure/ipc/
    ├── framing/
    │   └── framing.go   # Binary frame encoder/decoder
    ├── uds/
    │   ├── listener.go          # UDS Transport (Unix)
    │   ├── client.go            # UDS Client (worker-side / tests)
    │   ├── platform_unix.go     # PlatformTransport() for Unix
    │   └── named_pipe_windows.go # Named Pipe Transport (Windows)
    └── codec/
        └── msgpack.go   # MsgPackCodec
```

---

## Security Considerations

- **Socket isolation**: each worker has its own socket; a compromised worker cannot read another worker's traffic.
- **0600 permissions (Unix)**: only the Core process (same UID) can connect.
- **Named Pipe ACL (Windows)**: restricted to the creating user's SID (full implementation pending `winio`).
- **Payload size limit**: 16 MiB hard cap prevents memory exhaustion attacks via malformed length headers.
- **No raw token forwarding**: the `RequestPayload` carries only JWT claims, never the raw token.

---

## Testing

```bash
# Framing (pure unit tests, no OS deps)
go test ./infrastructure/ipc/framing/... -v

# UDS transport (real sockets via t.TempDir())
go test ./infrastructure/ipc/uds/... -v

# MsgPack codec
go test ./infrastructure/ipc/codec/... -v

# Heartbeat loop (mock transport + mock service)
go test ./application/heartbeat/... -v

# All IPC-related tests at once
go test ./... -run 'TestTransport|TestFraming|TestMsgPack|TestLoop|TestNamedPipe' -v
```

---

## Related Issues

- **#1** Worker lifecycle (spawn/stop/restart) — IPC is registered after spawn
- **#4** HTTP Gateway — uses `Transport.Send` to dispatch requests to workers
- **#7** Apache Arrow — plugs in as a second `Codec` implementation
- **#18** Worker handshake — first message sent after connection via this transport
