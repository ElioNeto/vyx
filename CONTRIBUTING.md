# Contributing to vyx

Thank you for your interest in contributing to **vyx**! This document explains how to get involved.

---

## Roadmap

Before picking an issue, check the **[ROADMAP.md](./ROADMAP.md)** for the suggested development order and issue dependencies. This helps avoid blocked PRs and wasted effort.

---

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](./CODE_OF_CONDUCT.md). Please treat everyone with respect.

---

## How Can I Contribute?

### 🐛 Reporting Bugs

1. Search [existing issues](../../issues) to avoid duplicates.
2. Open a new issue using the **Bug Report** template.
3. Include as much context as possible: OS, Go/Node/Python version, minimal reproduction.

### 💡 Suggesting Features

1. Open a new issue using the **Feature Request** template.
2. Describe the problem you're solving, not just the solution.
3. Be open to discussion — your idea may evolve through collaboration.

### 🔧 Submitting Pull Requests

1. **Fork** the repository and create your branch from `main`:
   ```bash
   git checkout -b feat/my-feature
   ```
2. Follow the existing code style and conventions.
3. Write or update **tests** for any changes.
4. Ensure all tests pass before opening a PR.
5. Write a clear **PR description** explaining the what and why.
6. Reference any related issues (e.g., `Closes #42`).

---

## Development Setup

### Prerequisites

- Go 1.22+
- Node.js 20+
- Python 3.11+
- Git

### Running Locally

```bash
git clone https://github.com/ElioNeto/vyx.git
cd vyx

# Build the CLI
cd core && go build -o ../vyx.exe ./cmd/vyx

# Start the hello-world example in development mode
cd ../examples/hello-world
../../vyx.exe dev
```

---

## Commit Message Convention

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): short description

feat(core): add circuit breaker support
fix(router): handle wildcard routes correctly
docs: update README with CLI examples
chore: bump go dependencies
```

Allowed types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`.

---

## Branch Naming

| Type | Pattern | Example |
|------|---------|----------|
| Feature | `feat/description` | `feat/arrow-ipc` |
| Bug fix | `fix/description` | `fix/worker-restart` |
| Docs | `docs/description` | `docs/annotation-guide` |
| Chore | `chore/description` | `chore/ci-setup` |

---

## Review Process

- All PRs require at least **1 approving review** before merging.
- The maintainer may request changes or ask for more context.
- Be patient and constructive — reviews are a learning opportunity.

---

Thank you for helping make **vyx** better! 🚀
