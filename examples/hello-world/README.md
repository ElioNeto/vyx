# hello-world

A minimal vyx example with two workers — one in **Go** and one in **Node.js** — demonstrating annotation-based routing, JWT authentication, JSON Schema validation, and IPC via Unix Domain Sockets.

## What's included

| Worker | Language | Routes |
|--------|----------|--------|
| `go:api` | Go | `GET /api/hello`, `POST /api/greet` |
| `node:api` | Node.js | `GET /api/products`, `GET /api/products/:id` |

## Prerequisites

- Go 1.22+
- Node.js 18+
- A Unix-like OS (Linux or macOS) — Windows uses Named Pipes automatically

## Running

```bash
# 1. From the repo root, build the vyx CLI
cd core && go build -o ../vyx ./cmd/vyx && cd ..

# 2. Move into the example directory
cd examples/hello-world

# 3. Set the JWT secret
export JWT_SECRET=supersecret

# 4. Start the core (it will auto-spawn both workers)
../../vyx dev
```

You should see both workers connect and the core listening on `http://localhost:8080`.

## Testing the routes

### Generate a JWT token (HS256, roles: ["user"])

You can generate a test token at [jwt.io](https://jwt.io) using secret `supersecret` and payload:
```json
{ "sub": "user-42", "roles": ["user"], "exp": 9999999999 }
```

### GET /api/hello (Go worker)
```bash
curl -H "Authorization: Bearer <TOKEN>" http://localhost:8080/api/hello
# {"message":"Hello from the Go worker!","user":"user-42"}
```

### POST /api/greet (Go worker, validated)
```bash
curl -X POST \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice"}' \
  http://localhost:8080/api/greet
# {"message":"Hello, Alice! Greetings from the Go worker."}
```

### GET /api/products (Node.js worker)
```bash
curl -H "Authorization: Bearer <TOKEN>" http://localhost:8080/api/products
# {"products":[{"id":"1","name":"Widget Alpha","price":9.99}, ...]}
```

### GET /api/products/:id (Node.js worker)
```bash
curl -H "Authorization: Bearer <TOKEN>" http://localhost:8080/api/products/2
# {"id":"2","name":"Widget Beta","price":19.99}
```

## Project structure

```
hello-world/
├── vyx.yaml                  # project manifest
├── route_map.json            # pre-generated route map (run `vyx build` to regenerate)
├── schemas/
│   └── greet.json            # JSON Schema for POST /api/greet
└── workers/
    ├── go/
    │   └── main.go           # Go worker (standalone binary)
    └── node/
        └── worker.js         # Node.js worker (no npm dependencies)
```
