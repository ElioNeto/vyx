# vyx

> A high-performance polyglot full-stack framework with a Go Core Orchestrator, annotation-based routing, and IPC via Unix Domain Sockets + Apache Arrow.

![License](https://img.shields.io/github/license/ElioNeto/vyx)
![Status](https://img.shields.io/badge/status-v0.1.0--MVP-brightgreen)
![Go](https://img.shields.io/badge/core-Go-00ADD8?logo=go)
![Node](https://img.shields.io/badge/worker-Node.js-339933?logo=node.js)
![Python](https://img.shields.io/badge/worker-Python-3776AB?logo=python)
![React](https://img.shields.io/badge/frontend-React-61DAFB?logo=react)

---

## What is vyx?

**vyx** is a polyglot full-stack framework where a **Core Orchestrator** written in Go acts as the single control point for routing, security, and inter-process communication. Backends can be implemented in **Go, Node.js, or Python**, while the frontend is **React** (with SSR). Routing is driven by **annotation-based discovery** — no filesystem magic, just explicit contracts.

```
[HTTP Client] → [Core Orchestrator (Go)]
                     ├── Manages workers (Node, Python, Go)
                     ├── Annotation parsing (build time)
                     ├── Routing based on route map
                     └── IPC via UDS + Apache Arrow
                          ├── Node Worker  (SSR React / APIs)
                          ├── Python Worker (APIs)
                          └── Go Worker     (Native APIs)
```

---

## Guiding Principles

- 🔒 **Security first** — auth, validation, and authorization concentrated in the core.
- 🧱 **Failure isolation** — circuit breakers and restart policies per worker.
- ⚡ **Performance** — UDS + Apache Arrow for minimal IPC overhead.
- 🧑‍💻 **Developer experience** — simple annotations and a first-class CLI.

---

## Annotation-Based Routing

**Go backend:**
```go
// @Route(POST /api/users)
// @Validate(JsonSchema: "user_create")
// @Auth(roles: ["admin"])
func CreateUser(w http.ResponseWriter, r *http.Request) { ... }
```

**Node.js / TypeScript backend:**
```typescript
// @Route(GET /api/products/:id)
// @Validate( zod )
// @Auth(roles: ["user", "guest"])
export async function getProduct(id: string) { ... }
```

**Python backend:**
```python
# @Route(POST /api/orders)
# @Validate( pydantic )
# @Auth(roles: ["user"])
def create_order(request: Dict) -> Dict: ...
```

**React frontend (SSR):**
```tsx
// @Page(/dashboard)
// @Auth(roles: ["user"])
export default function DashboardPage() { ... }
```

---

## CLI

```bash
vyx new <project-name>   # scaffold a new project with default vyx.yaml
vyx dev                  # start core in development mode (verbose logging + SIGHUP reload)
vyx build                # run annotation scanner, generate route_map.json, build binary
vyx annotate             # validate annotations and display the discovered route map
```

---

## Project Structure

```
project/
├── core/               # Go core orchestrator
├── scanner/            # Annotation scanner (Go, TypeScript, TSX)
├── backend/
│   ├── go/             # Go services
│   ├── node/           # Node.js services
│   └── python/         # Python services
├── frontend/
│   └── src/            # React + @Page annotations
├── schemas/            # Shared JSON Schemas
└── vyx.yaml            # project manifest
```

---

## Roadmap

| Phase | Scope | Status |
|-------|-------|--------|
| 1 – MVP | Go core, Node + Go workers, UDS/JSON, JWT, basic validation, CLI, WebSocket | ✅ Complete |
| 2 – Expansion | Python support, Apache Arrow, circuit breakers, worker pools, hot reload | 🔄 In Progress |
| 3 – Observability | Metrics (Prometheus), tracing (OpenTelemetry), TLS, full CLI, docs | 🗓 Planned |
| 4 – Scalability | Remote workers (gRPC), Kubernetes operator | 🗓 Planned |

---

## Phase 1 – What's included in v0.1.0

- ✅ **Go Core Orchestrator** — HTTP/1.1 + HTTP/2 + WebSocket gateway with JWT authentication, JSON Schema validation, and rate limiting
- ✅ **Worker Lifecycle** — spawn, monitor, restart (exponential backoff), graceful shutdown, handshake & registration protocol
- ✅ **IPC via Unix Domain Sockets** — binary framing protocol (request, response, heartbeat, error), bidirectional heartbeat (core ↔ worker)
- ✅ **Annotation Scanner** — static analysis for Go, TypeScript, and TSX files generating `route_map.json`
- ✅ **Router** — path parameter support (`:id`), method-based dispatch, authorization enforcement
- ✅ **CLI** — `vyx new`, `vyx dev`, `vyx build`, `vyx annotate` subcommands wired to the real scanner
- ✅ **`vyx.yaml` manifest** — schema, config loader, SIGHUP reload
- ✅ **Structured logging** — via `zap`, with access logs (method, path, worker, user, status, latency, correlation ID)

---

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines on how to contribute to vyx.

---

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE) for details.
