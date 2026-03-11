# Getting Started

This guide walks you through installing **vyx** and running your first polyglot project on Windows, Linux, and macOS.

---

## Prerequisites

| Tool | Minimum version | Required for |
|------|-----------------|--------------|
| [Go](https://go.dev/dl/) | 1.22+ | Core (mandatory) |
| [Node.js](https://nodejs.org/) | 20+ | Node workers + React SSR |
| [Python](https://python.org/) | 3.11+ | Python workers |
| [Git](https://git-scm.com/) | any | Cloning the repo |

---

## Installation

### Linux and macOS

Run the one-liner installer:

```bash
curl -fsSL https://raw.githubusercontent.com/ElioNeto/vyx/main/scripts/install.sh | bash
```

Or clone and run manually:

```bash
git clone https://github.com/ElioNeto/vyx.git
cd vyx
bash scripts/install.sh
```

The script will:
1. Verify all prerequisites (Go, Node.js, Python, Git)
2. Build the `vyx` binary from source
3. Print the command to add `vyx` to your `$PATH`

Add to PATH permanently (bash/zsh):

```bash
echo 'export PATH="$HOME/vyx/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

---

### Windows (PowerShell)

Run the one-liner installer (PowerShell 5.1+ or PowerShell 7+):

```powershell
irm https://raw.githubusercontent.com/ElioNeto/vyx/main/scripts/install.ps1 | iex
```

Or clone and run manually:

```powershell
git clone https://github.com/ElioNeto/vyx.git
cd vyx
.\scripts\install.ps1
```

The script will:
1. Verify all prerequisites
2. Build `vyx.exe` from source
3. Print the command to add `vyx.exe` to your `PATH`

Add to PATH for the current session:

```powershell
$env:PATH += ';C:\path\to\vyx\bin'
```

Add to PATH permanently (run as Administrator):

```powershell
[Environment]::SetEnvironmentVariable('PATH', $env:PATH + ';C:\path\to\vyx\bin', 'Machine')
```

---

## Your First Project

Once `vyx` is in your PATH, scaffold a new project:

```bash
vyx new my-app
cd my-app
```

This generates the following structure:

```
my-app/
├── core/
├── backend/
│   ├── go/
│   ├── node/
│   └── python/
├── frontend/
│   └── src/
├── schemas/
└── vyx.yaml
```

---

## Development Mode

Start the core with hot reload:

```bash
vyx dev
```

The core will:
- Parse all `@Route`, `@Auth`, `@Validate`, and `@Page` annotations
- Generate `route_map.json`
- Spawn all workers defined in `vyx.yaml`
- Watch for file changes and restart affected workers automatically

---

## Annotating Your First Route

### Node.js (TypeScript)

```typescript
// backend/node/users.ts
import { z } from 'zod';

// @Route(POST /api/users)
// @Validate( zod )
// @Auth(roles: ["admin"])
export async function createUser(body: unknown) {
  const schema = z.object({
    name: z.string(),
    email: z.string().email(),
  });
  const data = schema.parse(body);
  return { id: 1, ...data };
}
```

### Python

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
    return {"order_id": 456}
```

### Go backend

```go
// backend/go/users.go

// @Route(POST /api/users)
// @Validate(JsonSchema: "user_create")
// @Auth(roles: ["admin"])
func CreateUser(w http.ResponseWriter, r *http.Request) {
    // ...
}
```

### React page (SSR)

```tsx
// frontend/src/pages/Dashboard.tsx
// @Page(/dashboard)
// @Auth(roles: ["user"])
export default function Dashboard() {
  return <main>Welcome to your dashboard</main>;
}
```

---

## Validate Your Annotations

```bash
vyx annotate
```

Scans all source files and prints the discovered route map. Errors include file path and line number.

---

## Build for Production

```bash
vyx build
```

Generates:
- Optimized `vyx` core binary
- Worker bundles (Node, Python)
- Final `route_map.json`

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `vyx: command not found` | Ensure `bin/` is in your `$PATH` / `%PATH%` |
| `Go version too old` | Install Go 1.22+ from [go.dev/dl](https://go.dev/dl/) |
| Worker fails to start | Check `vyx.yaml` `command` field and worker logs |
| Port already in use | Change the `port` in `vyx.yaml` or kill the conflicting process |
| Node worker not found | Ensure Node.js 20+ is installed and `node` is in `$PATH` |
| PowerShell execution policy | Run `Set-ExecutionPolicy RemoteSigned -Scope CurrentUser` |

---

## Next Steps

- Read the [Annotation Reference](./annotations.md)
- Explore the [CLI Reference](./cli.md)
- Review the [Architecture Overview](./architecture.md)
- Check the [`vyx.yaml` Configuration Reference](./config.md)
