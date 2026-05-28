---
name: annotation-system
description: Use when working with vyx's annotation-based routing system. Covers @Route, @Auth, @Validate, @Page annotations, the static scanner (Go/TS/TSX/Python), route_map.json generation, and the RouteMap trie. Do NOT use for other topics.
---

# vyx Annotation System

This skill documents the annotation-based routing system that is central to vyx.

## Overview

vyx uses **static annotations** in source code comments to declare routes, auth requirements, and validation schemas. The scanner (`scanner/`) parses these at build time and generates a `route_map.json` consumed by the Core.

## Supported Annotations

### `@Route(path, method?)` — Go, TypeScript, Python
Declares an API endpoint.

```
// @Route(/api/users)
// @Route(/api/users, POST)
```

| Lang     | Parser          | Example |
|----------|-----------------|---------|
| Go       | `go_parser.go`  | `// @Route(/api/users)` |
| TypeScript | `ts_parser.go` | `// @Route(/api/users)` |
| Python   | `py_parser.go`  | `# @Route(/api/users)` |

### `@Auth(roles: [...])` — Go, TypeScript, Python
Restricts route to specific roles.

```
// @Route(/admin)
// @Auth(roles: ["admin", "superuser"])
```

### `@Validate(schema)` — Go, TypeScript
Attaches a JSON Schema for request validation.

```
// @Route(/users)
// @Validate(createUserSchema)
```

### `@Page(path)` — TSX (React)
Declares a page route (always GET).

```
// @Page(/dashboard)
```

## Route Entry Model

```go
type RouteEntry struct {
    Path      string   // e.g., "/api/users/:id"
    Method    string   // GET, POST, PUT, DELETE, PATCH
    WorkerID  string   // "node:ssr", "python:api", "go:api"
    AuthRoles []string // ["admin", "user"] or nil for public
    Validate  string   // schema name or ""
    Type      string   // "api" or "page"
    File      string   // source file path
    Line      int      // line number in source
}
```

## Scanner Architecture

Each language has its own parser file in `scanner/`:

```
scanner/
├── go_parser.go      # Parses // @Route, // @Auth, // @Validate in .go files
├── ts_parser.go      # Parses // @Route, // @Auth, // @Validate in .ts files
├── tsx_parser.go     # Parses // @Page, // @Auth in .tsx files
├── py_parser.go      # Parses # @Route, # @Auth in .py files
├── validator.go      # Validates parsed routes for conflicts/duplicates
├── generator.go      # Generates route_map.json from parsed routes
└── ..._test.go       # Test files
```

### Adding a new annotation type
1. Add regex pattern in the relevant parser.
2. Update `RouteEntry` struct if new fields are needed.
3. Update `validator.go` to validate the new field.
4. Update `generator.go` to include the field in JSON output.

## RouteMap Trie

The `RouteMap` (in `core/domain/gateway/`) is a **trie** (prefix tree) that enables O(k) route matching where k is path segment count.

### Hot-swap mechanism
- `RouteMap` uses `unsafe.Pointer` with `atomic.Load/Store` for lock-free reads.
- On `SIGHUP` or file change, a new `RouteMap` is built from `route_map.json` and atomically swapped.
- Old `RouteMap` is garbage-collected after all in-flight requests finish.

### Route matching
```
GET /api/users/123
→ RouteMap.Match("GET", "/api/users/123")
→ Matches /api/users/:id with params {id: "123"}
```

### Wildcard segments
- `:param` — named parameter (matches one segment)
- `*` — catch-all (matches remainder of path)

## workerID convention

| Worker | Example workerID |
|--------|-----------------|
| Node.js SSR | `"node:ssr"` |
| Node.js API | `"node:api"` |
| Python API | `"python:api"` |
| Go API | `"go:api"` |

## Testing the scanner

```bash
cd scanner && go test -v -run "TestParse.*File" -count=1
cd scanner && go test -v -run "TestValidator" -count=1
```

Key test patterns:
- **Unreadable file**: use non-existent paths (NOT permission-based tests)
- **Error messages**: test `AnnotationError.Message` contains meaningful text
- **Route output**: test `Route.Path`, `Route.Method`, `Route.AuthRoles`
- **Edge cases**: empty path, missing annotation, multiple annotations per file
