# Technical Specification: OmniStack Engine

## 1. Overview and Objective

Develop a high‑performance polyglot full‑stack framework where a **Core Orchestrator** written in Go acts as the central control point. The framework allows backends to be implemented in Go, Node.js, or Python, while the frontend is mandatory React. The routing model breaks away from traditional file‑system routing, adopting an **Annotation‑based Discovery** (metadata‑driven discovery) system, aiming for greater security, explicit contract control, and failure isolation.

**Guiding principles:**
- **Security first**: validations, authentication, and authorization concentrated in the core.
- **Failure isolation**: the core manages workers independently, applying circuit breakers and restart policies.
- **Performance**: communication via optimized IPC (Unix Domain Sockets + Apache Arrow) with minimal overhead.
- **Developer experience**: simple annotations and CLI tools for scaffolding and build.

---

## 2. High‑Level Architecture

```
[HTTP Client] → [Core Orchestrator (Go)]
                     ├── Manages workers (Node, Python, Go)
                     ├── Annotation parsing (build time)
                     ├── Routing based on route map
                     └── Communication via UDS + Arrow
                          ├── Node Worker (SSR React / APIs)
                          ├── Python Worker (APIs)
                          └── Go Worker (Native APIs)
```

The core is the only process exposed to the network. All workers are child processes managed by the core, communicating exclusively via IPC.

---

## 3. Core Orchestrator (Go)

### 3.1 Responsibilities
- **Runtime Manager**: spawn, monitoring (health checks), restart, and graceful shutdown of workers.
- **Annotation Parser**: static scanner at build time that generates a global route map (routing, validation, authorization).
- **Request Dispatcher**: receives HTTP requests, applies authentication, validates payloads against schemas, and forwards to the correct worker via UDS.
- **Circuit Breaker**: monitors consecutive failures on routes and temporarily diverts/refuses traffic.
- **HTTP Gateway**: support for HTTP/1.1, HTTP/2, and WebSocket (proxying to workers).

### 3.2 Worker Management
- Each worker is a separate process, running in its own runtime (Node.js, Python, Go).
- The core defines **worker pools** per technology/route, allowing vertical scaling (multiple instances of the same worker).
- **Graceful shutdown**: upon receiving SIGTERM, the core notifies workers and waits for in‑flight requests.
- **Restart policy**: failed workers are automatically restarted (with exponential backoff) after logging the event.

### 3.3 IPC – Unix Domain Sockets (UDS) and Apache Arrow
- For each worker, the core creates a pair of UDS sockets (one for commands/requests, another for responses/streams).
- **Apache Arrow**: used for efficient serialization of tabular data or large volumes. For small payloads, binary JSON (MsgPack) is used for simplicity.
- The core and workers share a simple binary protocol (with header for size, message type, and metadata).

---

## 4. Annotation System (Routing & Contracts)

Annotations are interpreted at **build time** by a static analyzer that scans the project directories and generates a `route_map.json` (or binary) file consumed by the core.

### 4.1 Backend Annotations

**Go:**
```go
// @Route(POST /api/users)
// @Validate(JsonSchema: "user_create")
// @Auth(roles: ["admin"])
func CreateUser(w http.ResponseWriter, r *http.Request) { ... }
```

**Node.js (TypeScript):**
```typescript
// @Route(GET /api/products/:id)
// @Validate( zod )
// @Auth(roles: ["user", "guest"])
export async function getProduct(id: string) { ... }
```

**Python:**
```python
# @Route(POST /api/orders)
# @Validate( pydantic )
# @Auth(roles: ["user"])
def create_order(request: Dict) -> Dict: ...
```

### 4.2 Frontend Annotations (React)

**React (TSX):**
```tsx
// @Page(/dashboard)
// @Auth(roles: ["user"])
export default function DashboardPage() { ... }
```

### 4.3 Validation Contracts
- Schemas can be defined inline (via libraries like Zod, Pydantic) or by referencing JSON Schema files.
- The core converts these schemas to an internal format (e.g., JSON Schema) and uses them to validate requests **before** forwarding to the worker.

### 4.4 Route Map Storage
The core maintains an in‑memory structure:
```go
type Route struct {
    Path        string
    Method      string
    WorkerID    string   // worker identifier (e.g., "node:products")
    AuthRoles   []string
    Validate    string   // reference to the schema
    Type        string   // "api" or "page"
}
```

---

## 5. Communication and Data Transfer

### 5.1 Unix Domain Sockets (UDS)
- Low latency, security (only local processes can access).
- The core creates named sockets in the filesystem with restricted permissions (0600).
- For Windows: use Named Pipes with equivalent security.

### 5.2 Message Protocol
Simple binary format:
```
┌─────────┬──────────────┬─────────────────┐
│  Length │ Type (1 byte)│ Payload (bytes) │
└─────────┴──────────────┴─────────────────┘
```
- **Type**: 0x01 = request, 0x02 = response, 0x03 = heartbeat, 0x04 = error.
- Payload can be JSON, MsgPack, or Arrow RecordBatch.

### 5.3 Apache Arrow
- Used when the worker needs to return large datasets (e.g., reports, lists).
- The core and workers share a memory buffer (via `mmap` or shared memory) for zero‑copy.
- Fallback to streaming chunks if the volume exceeds limits.

### 5.4 Heartbeats and Health Checks
- Workers send periodic heartbeats (every 5s) to the core.
- The core responds with updated configurations (e.g., new schemas).

---

## 6. Detailed Request Flow

1. **Client** sends an HTTP request to the core (port 80/443).
2. **Core** (Go) extracts path, method, headers, body.
3. **Authentication**: validates JWT (or other token) and extracts claims (roles, userID).
4. **Routing**: consults the route map – finds the matching route.
5. **Authorization**: checks if the user’s roles satisfy `@Auth`. If not, returns 403.
6. **Contract validation**: applies the schema defined in `@Validate` to body/query/params. If invalid, returns 400 with details.
7. **Forwarding**:
   - If API route: core serializes the request (including claims) and sends via UDS to the worker.
   - If page route (SSR): core sends to the Node worker responsible for the frontend.
8. **Worker processes** the logic and returns a response via UDS.
9. **Core** receives the response, applies transformations (e.g., adds security headers) and sends to the client.
10. **Observability**: structured logs and metrics are emitted at each stage.

**Circuit Breaker:** If a route fails N consecutive times (timeout or worker dead), the core opens the circuit and returns 503 for a period, avoiding overload.

---

## 7. Security

- **TLS termination**: the core manages certificates and terminates HTTPS.
- **Input validation**: all requests undergo header sanitization and schema validation at the core, preventing malicious payloads from reaching vulnerable workers.
- **Worker isolation**: each worker runs under a different user (optional containerization) and restricted filesystem access.
- **JWT tokens**: signed and validated at the core; workers receive only claims (no need to handle cryptography).
- **Protection against common attacks**: rate limiting (by IP, by token), global timeouts, payload size limits.
- **Auditing**: the core logs all access attempts with details (timestamp, route, user, status).

---

## 8. Worker Management

### 8.1 Lifecycle
- **Start**: core executes the command defined in the project manifest (e.g., `node worker.js`, `python worker.py`).
- **Registration**: upon startup, the worker connects to the core via socket and sends a handshake informing its ID and capabilities (routes it implements – optional, for cross‑validation).
- **Monitoring**: core sends periodic heartbeats; if a worker does not respond, it is considered dead and restarted.
- **Stop**: when shutting down, core sends SIGTERM and waits up to `shutdown_timeout`.

### 8.2 Pools and Scalability
- For high‑demand routes, multiple workers can be defined (e.g., 4 Node processes). The core balances requests among them (round‑robin or least‑loaded).
- Workers can be scaled horizontally (across multiple machines) using a core in cluster mode (future).

### 8.3 Hot Reload
- In development, the core watches for file changes and restarts workers automatically.
- In production, support for zero‑downtime updates via `SIGUSR1` that makes workers reload code.

---

## 9. Development and Build

### 9.1 CLI (`omni`)
- `omni new project` – creates directory structure with core, backend, and frontend.
- `omni dev` – starts core in development mode, with hot reload and detailed logs.
- `omni build` – generates optimized artifacts (core binary, worker bundles).
- `omni annotate` – validates annotations and displays route map.

### 9.2 Suggested Directory Structure
```
project/
├── core/               # (optional) core configurations
├── backend/
│   ├── go/             # Go services
│   ├── node/           # Node services
│   └── python/         # Python services
├── frontend/
│   └── src/            # React + @Page annotations
├── schemas/            # Shared JSON Schemas
└── omni.yaml           # project manifest
```

### 9.3 Continuous Integration
- During build, the annotation scanner generates the route map.
- Tests can be run by mocking the core or using a test core.

---

## 10. Observability and Monitoring

- **Structured logs**: core and workers emit logs in JSON format (level, timestamp, service, correlation ID).
- **Metrics**: exposed via `/metrics` endpoint (Prometheus) – latency per route, request count, errors, worker status.
- **Distributed tracing**: support for OpenTelemetry – trace headers are propagated to workers via message metadata.
- **Dashboards**: sample Grafana dashboard for system health visualization.

---

## 11. Performance and Scalability Considerations

- **Low latency**: UDS + MsgPack for small payloads; Arrow for large volumes.
- **Concurrency**: core uses goroutines to handle multiple simultaneous requests; workers can be single‑threaded (Node) or multi‑threaded (Python/Go).
- **Schema caching**: validation schemas are compiled once and reused.
- **Worker limits**: maximum number of workers per technology configurable to avoid resource exhaustion.
- **Horizontal scalability**: future support for core connecting to remote workers via gRPC for multi‑host deployments.

---

## 12. Use Cases and Examples

### 12.1 Node.js API with Zod validation
```typescript
// backend/node/users.ts
import { z } from 'zod';

// @Route(POST /api/users)
// @Validate( zod )
// @Auth(roles: ["admin"])
export async function createUser(body: unknown) {
  const schema = z.object({ name: z.string(), email: z.string().email() });
  const data = schema.parse(body);
  // logic...
  return { id: 123 };
}
```

### 12.2 React page with SSR
```tsx
// frontend/src/pages/Profile.tsx
// @Page(/profile)
// @Auth(roles: ["user"])
export default function Profile({ user }) {
  return <div>Hello {user.name}</div>;
}
```

### 12.3 Python API with Pydantic
```python
# backend/python/orders.py
from pydantic import BaseModel

# @Route(POST /api/orders)
# @Validate( pydantic )
# @Auth(roles: ["user"])
class OrderInput(BaseModel):
    product_id: str
    quantity: int

def create_order(data: OrderInput):
    # logic...
    return {"order_id": 456}
```

---

## 13. Roadmap and Next Steps

**Phase 1 – MVP (3 months)**
- Go core with basic worker management (Node and Go).
- Annotation scanner for Node (TypeScript) and Go.
- UDS communication with JSON.
- Routing, JWT authentication, basic validation (JSON Schema).

**Phase 2 – Expansion (3 months)**
- Python support.
- Apache Arrow for large datasets.
- Circuit breakers and worker pools.
- Hot reload in development.

**Phase 3 – Observability and Production (2 months)**
- Metrics, tracing, structured logging.
- Complete CLI.
- Documentation and examples.

**Phase 4 – Scalability (ongoing)**
- Support for remote workers (gRPC).
- Kubernetes operator integration.

---

## Conclusion

The OmniStack Engine proposes an innovative architecture that centralizes routing, security, and communication control in a Go core, while allowing developers to use their preferred languages and tools on the backend. The use of annotations brings clarity and traceability, and UDS + Arrow communication ensures superior performance. With this refinement, the specification is ready to guide the implementation of a robust and modern framework.
