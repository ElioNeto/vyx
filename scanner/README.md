# scanner

The `scanner` package implements the **build-time annotation scanner** for the OmniStack/vyx framework.
It parses Go and TypeScript source files for `@Route`, `@Validate`, `@Auth`, and `@Page` annotations
and generates a `route_map.json` consumed by the core at startup.

## Supported annotations

| Annotation | Languages | Description |
|---|---|---|
| `@Route(METHOD /path)` | Go, TS | Registers an API endpoint |
| `@Validate(schema)` | Go, TS | Schema used for request validation |
| `@Auth(roles: ["r1"])` | Go, TS | Required roles for authorization |
| `@Page(/path)` | TS/TSX | Registers an SSR React page |

## Usage via CLI

```bash
# From project root
go run ./cmd/annotate \
  -go backend/go \
  -ts backend/node \
  -frontend frontend/src \
  -output route_map.json
```

Or after building:

```bash
vyx annotate -go backend/go -ts backend/node -frontend frontend/src
```

## route_map.json schema

```json
{
  "routes": [
    {
      "path": "/api/users",
      "method": "POST",
      "worker_id": "go:api",
      "auth_roles": ["admin"],
      "validate": "JsonSchema: \"user_create\"",
      "type": "api"
    }
  ]
}
```

## Running tests

```bash
go test ./scanner/...
```

## Validation rules

- HTTP method must be one of: `GET POST PUT PATCH DELETE HEAD OPTIONS`
- Route path must start with `/`
- Duplicate `METHOD /path` combinations are reported as errors
- Malformed annotations report file path and line number
