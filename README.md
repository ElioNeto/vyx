# vyx

> A high-performance polyglot full-stack framework with a Go Core Orchestrator, annotation-based routing, and IPC via Unix Domain Sockets + Apache Arrow.

![License](https://img.shields.io/github/license/ElioNeto/vyx)
![Status](https://img.shields.io/badge/status-WIP-orange)
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
vyx new project   # scaffold a new project
vyx dev           # start core in development mode (hot reload)
vyx build         # generate optimized artifacts
vyx annotate      # validate annotations and display route map
```

---

## Project Structure

```
project/
├── core/               # core configurations
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
| 1 – MVP | Go core, Node + Go workers, UDS/JSON, JWT, basic validation | 🔄 In Progress |
| 2 – Expansion | Python support, Apache Arrow, circuit breakers, hot reload | 🗓 Planned |
| 3 – Observability | Metrics (Prometheus), tracing (OpenTelemetry), full CLI | 🗓 Planned |
| 4 – Scalability | Remote workers (gRPC), Kubernetes operator | 🗓 Planned |

---

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines on how to contribute to vyx.

---

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE) for details.
