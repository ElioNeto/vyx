# Vyx Python Worker SDK

> Python SDK for building worker services that communicate with the vyx Core via UDS.

![Python](https://img.shields.io/badge/Python-3.10+-3776AB?logo=python)
![Status](https://img.shields.io/badge/status-v0.2.0--beta-blue)

## Installation

```bash
pip install vyx
```

With Pydantic validation support:

```bash
pip install vyx[pydantic]
```

## Quick Start

### 1. Create a worker file

```python
# workers/api.py
from pydantic import BaseModel
from vyx import ipc

# @Route(POST /api/orders)
# @Validate(pydantic)
# @Auth(roles: ["user"])
class OrderInput(BaseModel):
    product_id: str
    quantity: int

async def create_order(request: dict) -> dict:
    body = request.get("body", {})
    data = OrderInput(**body)
    return {"order_id": 123, "product": data.product_id}

# @Route(GET /api/orders)
async def list_orders(request: dict) -> dict:
    return {"orders": []}

handlers = {
    ("POST", "/api/orders"): create_order,
    ("GET", "/api/orders"): list_orders,
}

if __name__ == "__main__":
    import asyncio
    asyncio.run(ipc.start_worker(handlers))
```

### 2. Run the worker

```bash
VYX_SOCKET=/tmp/vyx.sock python workers/api.py
```

## Annotations

Place annotations as comments directly above function or class definitions:

```python
# @Route(GET /api/users/:id)
# @Validate(pydantic)
# @Auth(roles: ["admin", "user"])
async def get_user(request: dict) -> dict:
    return {"user": request["params"]["id"]}
```

### Supported Annotations

| Annotation | Description |
|------------|-------------|
| `@Route(METHOD /path)` | HTTP method and path pattern |
| `@Validate(type)` | Validation type (`pydantic`, `jsonschema`) |
| `@Auth(roles: ["role1", "role2"])` | Required roles |

## Scanner

Generate `route_map.json` from your worker files:

```bash
# Scan a directory
python -m vyx.cli scan --dir workers/ --worker-id python:api -o route_map.json

# Scan specific files
python -m vyx.cli scan --files api.py admin.py --worker-id python:api
```

### Using the SDK programmatically

```python
from vyx.scanner import scan_directory, generate_route_map

routes = scan_directory("workers/", "python:api")
route_map = generate_route_map(routes)

import json
print(json.dumps(route_map, indent=2))
```

## Modules

| Module | Description |
|--------|-------------|
| `vyx.scanner` | Annotation parser for `@Route`, `@Auth`, `@Validate` |
| `vyx.ipc` | UDS client and IPC protocol |
| `vyx.dispatch` | Request dispatcher |
| `vyx.context` | ContextVars for correlation ID |
| `vyx.validate` | Pydantic validation helpers |
| `vyx.cli` | CLI entry point |

## IPC Protocol

The worker communicates with Core via Unix Domain Sockets using binary framing:

- `TYPE_HANDSHAKE` (0x01) — Initial handshake
- `TYPE_REQUEST` (0x02) — Incoming request
- `TYPE_RESPONSE` (0x03) — Response
- `TYPE_HEARTBEAT` (0x04) — Keep-alive
- `TYPE_ERROR` (0x05) — Error response

## Testing

```bash
pip install -e ".[dev]"
pytest tests/ -v
```

## Example Route Map Output

```json
{
  "routes": [
    {
      "method": "POST",
      "path": "/api/orders",
      "worker_id": "python:api",
      "auth": {
        "required": true,
        "roles": ["user"]
      },
      "validate": {
        "type": "pydantic",
        "model": "create_order"
      },
      "source": {
        "file": "workers/api.py",
        "line": 10,
        "symbol": "create_order"
      }
    }
  ]
}
```

## License

MIT License — see [LICENSE](../LICENSE).