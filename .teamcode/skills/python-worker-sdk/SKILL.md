---
name: python-worker-sdk
description: Use when writing or modifying Python code in the vyx Python Worker SDK (packages/python/). Covers the vyx package, IPC client, annotation scanner, dispatch logic, testing with pytest. Do NOT use for Go core or Node.js worker.
---

# vyx Python Worker SDK

This skill documents the Python worker SDK at `packages/python/`.

## Package structure

```
packages/python/
├── vyx/
│   ├── __init__.py
│   ├── cli.py          # CLI entry point (vyx scan)
│   ├── ipc.py          # UDS client + IPC protocol
│   ├── dispatch.py     # Request dispatcher
│   ├── context.py      # Correlation ID via ContextVars
│   ├── validate.py     # Pydantic validation helpers
│   └── scanner.py      # Python annotation scanner (# @Route, # @Auth)
├── tests/
│   ├── test_ipc.py
│   ├── test_dispatch.py
│   ├── test_context.py
│   ├── test_scanner.py
│   └── test_validate.py
├── pyproject.toml      # or setup.py — package configuration
└── requirements.txt    # or pyproject.toml deps
```

## Key patterns

### IPC Client (`ipc.py`)
- Connects to Core via Unix Domain Socket.
- Implements the binary framing protocol (length + type + payload).
- Serializes/deserializes with `msgpack`.
- Handles reconnection with exponential backoff.

### Dispatch (`dispatch.py`)
- Receives requests from Core via IPC.
- Routes to the appropriate handler function.
- Returns response via IPC.

### Context (`context.py`)
- Uses `contextvars.ContextVar` for correlation ID propagation.
- Request-scoped context, no global state.

### Scanner (`scanner.py`)
- Static analysis of Python files for `# @Route` and `# @Auth` annotations.
- Mirrors the Go scanner for consistency.
- Used by `vyx scan` CLI command.

### Validation (`validate.py`)
- Pydantic-based schema validation.
- Request body validation, response serialization.

## Adding new features

1. Add tests first (`tests/test_*.py`).
2. Implement in `vyx/`.
3. Update `__init__.py` exports if needed.
4. Verify: `cd packages/python && pip install -e . && python -m pytest tests/ -v`

## Testing

```bash
cd packages/python

# Install in dev mode
pip install -e .

# Run all tests
python -m pytest tests/ -v

# With coverage
python -m pytest tests/ --cov=vyx -v

# Specific test file
python -m pytest tests/test_scanner.py -v
```

## Code style

- Follow PEP 8 (enforced by `ruff`).
- Type hints for all function signatures.
- Docstrings for public APIs.
- Async support for non-blocking operations.
