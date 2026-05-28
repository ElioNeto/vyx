---
name: ipc-protocol
description: Use when working with vyx's IPC layer — Unix Domain Sockets, binary framing protocol, MsgPack serialization, Apache Arrow, handshake, heartbeat, and request/response message flow between Core and Workers.
---

# vyx IPC Protocol

This skill documents the inter-process communication layer between the Go Core Orchestrator and workers.

## Architecture

```
[Core (Go)]  ←→  UDS Socket Pair  ←→  [Worker (Go/Node/Python)]
                ↑              ↑
           cmd/req        resp/stream
```

- Each worker gets a **pair of Unix Domain Sockets** (one for commands/requests, one for responses/streams).
- Core listens on each socket. Workers connect as clients.
- On Windows: Named Pipes instead of UDS.

## Binary Frame Protocol

Messages follow a simple binary format:

```
┌────────────┬──────────┬──────────────────┐
│  Length    │   Type   │    Payload       │
│  (4 bytes) │ (1 byte) │  (Length bytes)  │
│  big-endian│          │                  │
└────────────┴──────────┴──────────────────┘
```

Total header: 5 bytes. Payload is `Length` bytes.

### Message Types

| Code | Type | Direction | Description |
|------|------|-----------|-------------|
| 0x01 | Request | Core → Worker | Incoming HTTP request to process |
| 0x02 | Response | Worker → Core | HTTP response from worker |
| 0x03 | Heartbeat | Worker → Core | Liveness ping (every 5s) |
| 0x04 | Error | Bidirectional | Error notification |
| 0x05 | Handshake | Worker → Core | Worker registration |
| 0x06 | WS_Open | Core → Worker | WebSocket connection opened |
| 0x07 | WS_Message | Core → Worker | WebSocket message |
| 0x08 | WS_Close | Core → Worker | WebSocket connection closed |

## Serialization

### Small payloads — MessagePack
For requests, responses, handshakes, heartbeats, and errors.
- Go: `github.com/vmihailenco/msgpack/v5`
- Node: `@msgpack/msgpack`
- Python: `msgpack`

### Large datasets — Apache Arrow
For tabular data or payloads > ~64KB.
- Go: `github.com/apache/arrow/go/v18/arrow`
- Supported by codec in `core/infrastructure/ipc/codec/`

### Selector
`core/infrastructure/ipc/codec/selector.go` — routes between MsgPack and Arrow based on payload heuristics.

## Handshake Protocol

When a worker connects to the Core:

1. Worker connects to both UDS sockets.
2. Worker sends `HandshakePayload` (0x05) containing:
   - `worker_id` — unique identifier (e.g., `"node:ssr"`)
   - `technology` — `"go"`, `"node"`, or `"python"`
   - `version` — SDK version
3. Core validates, registers worker, assigns to pool.
4. Core replies with acknowledgment.

## Heartbeat

- Workers send a heartbeat (0x03) every **5 seconds**.
- Core tracks last heartbeat time per worker.
- If no heartbeat for **15 seconds**, worker is marked unhealthy.
- Unhealthy workers are restarted after configured timeout.

## Request/Response Flow

```
Client                    Core                        Worker
  │                        │                            │
  │── HTTP Request ───────►│                            │
  │                        │── 0x01 Request (UDS) ─────►│
  │                        │                            │── Process
  │                        │◄── 0x02 Response (UDS) ────│
  │◄── HTTP Response ──────│                            │
```

1. Core receives HTTP request
2. Core matches route in RouteMap trie
3. Core picks worker from pool (round-robin)
4. Core serializes request via MsgPack, sends over UDS
5. Worker deserializes, processes, sends response
6. Core deserializes response, sends HTTP response to client

## Streams

For long-lived streams (Server-Sent Events, large responses):
- Worker writes frames of type 0x02 sequentially over the response socket.
- Each frame carries a sequence number and an optional continuation flag.

## Cross-platform support

- **Unix**: UDS in `core/infrastructure/ipc/uds/` — `client.go`, `listener.go`
- **Windows**: Named pipes in `core/infrastructure/ipc/uds/` — `named_pipe_windows.go` (build tag `windows`)
- **Build tags**: Files use `//go:build !windows` and `//go:build windows` to select implementation.

## Key packages

| Package | Description |
|---------|-------------|
| `domain/ipc/` | Message types, HandshakePayload (no deps) |
| `infrastructure/ipc/codec/` | MsgPack + Arrow + Selector |
| `infrastructure/ipc/framing/` | Binary frame read/write |
| `infrastructure/ipc/uds/` | UDS client + listener (cross-platform) |
| `infrastructure/ipc/shm/` | Shared memory (mmap) |

## Testing IPC

```bash
# Run all IPC tests
cd core && go test ./infrastructure/ipc/... -v -count=1

# Run with race detection
cd core && go test ./infrastructure/ipc/... -race -count=1
```
