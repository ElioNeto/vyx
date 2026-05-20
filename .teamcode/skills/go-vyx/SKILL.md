---
name: go-vyx
description: Use when writing or modifying Go code in the vyx Core Orchestrator or Scanner. Covers Clean Architecture layering, error handling, testing conventions, and project-specific patterns. Do NOT use for Node.js, Python, or infra code.
---

# Go Patterns in vyx

This skill documents the Go conventions and patterns used across the vyx project.

## Architecture — Clean Architecture in `core/`

```
core/
├── domain/          # Entities, value objects, interfaces — ZERO external dependencies
├── application/     # Use cases, orchestration, business logic
├── infrastructure/  # I/O, frameworks, concrete implementations
```

### Layering rules

- **domain/** imports nothing outside stdlib. Defines interfaces (`Repository`, `Manager`, `EventPublisher`) that `infrastructure/` implements.
- **application/** imports domain. Never imports infrastructure directly (Dependency Inversion).
- **infrastructure/** imports both domain and application to wire implementations.
- **Test files** in `infrastructure/` may test against concrete implementations using test doubles from `testutil/` when available.

### Key domain packages

| Package | Purpose | Key Types |
|---------|---------|-----------|
| `domain/circuit/` | Circuit Breaker state machine | `State` (Closed/Open/HalfOpen), `Breaker` |
| `domain/gateway/` | Route map + request/response | `RouteMap` (trie), `RouteEntry`, `GatewayRequest` |
| `domain/ipc/` | IPC message types | `MessageType` (0x01-0x08), `HandshakePayload` |
| `domain/worker/` | Worker entity | `State`, `Manager`, `Repository`, `EventPublisher` |
| `domain/pool/` | Worker pool | Pool configuration, round-robin |

### Error handling

- Use `fmt.Errorf("context: %w", err)` for error wrapping (Go 1.13+ style).
- Sentinel errors when the caller needs to distinguish: `var ErrNotFound = errors.New("not found")`.
- Always check errors. Never use `_` for error returns.
- Context cancellation: check `ctx.Err()` when appropriate.
- Custom error types only when callers need structured fields.

## Testing conventions

### General
- File: `*_test.go` in the **same package** (internal tests).
- Table-driven tests with `[]struct{ name string; ... }` and `t.Run(tc.name, ...)`.
- `testify/assert` for most assertions; `testify/require` for fatal conditions.
- Coverage: `gateway` package target ≥80%.

### Patterns
```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name string
        input string
        want string
    }{
        {name: "valid input", input: "hello", want: "HELLO"},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            got := MyFunc(tc.input)
            assert.Equal(t, tc.want, got)
        })
    }
}
```

- **Parallel tests**: `t.Parallel()` for independent test cases.
- **Subtests**: `t.Run("case name", ...)` for logical grouping.
- **Fixtures**: use `t.TempDir()` for temporary directories (auto-cleaned).
- **Race detector**: always run with `-race`.
- **Integration tests**: build tag `//go:build integration` in `core/integration/`.

### Mocks
- Prefer **interfaces** over concrete types for testability.
- Hand-written test doubles (no mock frameworks).
- Example pattern in `domain/worker/` where `Repository` interface is implemented in-memory.

## Scanner conventions (`scanner/`)

The scanner is a **pure Go** package that statically analyzes source files for annotations.

### Adding a new parser
1. Create `scanner/<lang>_parser.go` with:
   - Regex patterns for annotation matching
   - A `parse<Lang>File(path, workerID)` function
   - A `Parse<Lang>Files(dir, workerID)` walk function
2. Register in `scanner/validator.go` for validation.
3. Register in `scanner/generator.go` for route_map.json generation.
4. Add tests in `scanner/<lang>_parser_test.go`.

### Annotation error reporting
```go
type AnnotationError struct {
    File    string
    Line    int
    Message string
}
```

### File-open error pattern
When `os.Open` fails, return the error — never silently return `nil`:
```go
f, err := os.Open(path)
if err != nil {
    return nil, []AnnotationError{{
        File: path, Line: 0,
        Message: fmt.Sprintf("cannot open file: %v", err),
    }}
}
```

For tests that exercise the file-open error path, use non-existent paths (NOT permission-based tests):
```go
// BAD: os.WriteFile(path, data, 0000) — root bypasses permissions
// GOOD: routes, errs := ParseTSFiles("/nonexistent/path", ...)
```

## CLI conventions (`cmd/vyx/`)

- Each subcommand (dev, build, new, annotate) gets its own file or group.
- CLI uses Cobra — register new commands in `cmd/vyx/` following existing patterns.
- User-facing errors should be descriptive and actionable.

## Build & run

```bash
# Build everything
cd core && go build ./...

# Run all tests with race detection
go test ./... -race -count=1 -coverprofile=coverage.txt

# Lint
golangci-lint run

# Security scan
govulncheck ./...

# Run specific package tests
go test ./domain/... ./application/... ./infrastructure/...
