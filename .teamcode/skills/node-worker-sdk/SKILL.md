---
name: node-worker-sdk
description: Use when writing or modifying TypeScript/JavaScript code in the vyx Node.js Worker SDK (packages/worker/). Covers @vyx/worker package, IPC client, dispatch, context, testing with vitest, ESLint config. Do NOT use for Go core or Python worker.
---

# vyx Node.js Worker SDK

This skill documents the Node.js/TypeScript worker SDK at `packages/worker/`.

## Package structure

```
packages/worker/
├── src/
│   ├── index.ts       # Entry point — exports public API
│   ├── dispatch.ts    # Request dispatcher (routes incoming requests)
│   ├── request.ts     # Request helper utilities
│   └── context.ts     # Correlation context (AsyncLocalStorage)
├── tests/
│   ├── dispatch.test.ts
│   ├── request.test.ts
│   ├── context.test.ts
│   └── index.test.ts
├── package.json       # @vyx/worker package
├── tsconfig.json      # TypeScript strict mode
├── eslint.config.js   # ESLint flat config (v10+)
└── vitest.config.ts   # Vitest configuration
```

## Key patterns

### IPC Client
- Connects to Core via Unix Domain Socket.
- Node.js `net` module for UDS connections.
- Implements the binary framing protocol (length + type + payload).
- Uses `@msgpack/msgpack` for serialization.

### Dispatch (`dispatch.ts`)
- Listens on IPC socket for incoming requests.
- Matches routes to handler functions.
- Returns responses via IPC.

### Context (`context.ts`)
- Uses Node.js `AsyncLocalStorage` for correlation ID propagation.
- Async-scoped context, no global state.
- Propagates tracing headers from Core.

### Request (`request.ts`)
- Parses incoming request from Core (method, path, headers, body).
- Provides typed helpers for response construction.

## Testing

```bash
cd packages/worker

# Run all tests
npm test

# Or with vitest directly
npx vitest run

# With coverage
npx vitest run --coverage

# Watch mode
npx vitest

# Lint
npm run lint
```

### Test patterns
- Use `vitest` with `describe`/`it` blocks.
- Mock UDS connections with in-memory streams.
- Use `vi.mock()` for module-level mocking.

```ts
import { describe, it, expect, vi } from 'vitest'

describe('dispatch', () => {
  it('should route to correct handler', () => {
    // ...
    expect(result).toEqual(expected)
  })
})
```

## ESLint
- Flat config in `eslint.config.js` (ESLint v10).
- Uses `typescript-eslint` for TS-specific rules.
- Run: `npm run lint`

## TypeScript config
- `strict: true` in tsconfig.json.
- `target: ES2022` or later.
- `moduleResolution: bundler` or `node16`.

## Adding new features

1. Export new types/functions from `src/index.ts`.
2. Add tests in `tests/`.
3. Update `package.json` exports if needed.
4. Verify: `cd packages/worker && npm test && npm run lint`

## Conventions
- camelCase for variables/functions.
- PascalCase for types/interfaces/classes.
- Prefer `const` over `let`, avoid `var`.
- Async/await over raw promises where possible.
- Error handling with typed error classes.
