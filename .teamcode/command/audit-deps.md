---
description: "Run a full dependency audit across the vyx monorepo — checks Go modules, npm packages, and Python dependencies for outdated, conflicting, or vulnerable packages."
agent: deps
subtask: true
---

Run a dependency audit across the entire vyx monorepo (Go + npm + Python).

Use the @deps agent to:

1. Scan ALL dependency manifests:
   - `core/go.mod` (Go)
   - `packages/worker/package.json` (npm)
   - `packages/python/pyproject.toml` (Python)
   - Root `go.mod` and `package.json`
2. Check for:
   - Outdated dependencies (newer versions available)
   - Version drift (same dep at different versions)
   - Deprecated or unmaintained packages
   - Security vulnerabilities (`govulncheck`, `npm audit`, `pip-audit`)
   - Missing or unused dependencies
3. Generate a comprehensive `dependency-audit-report.md`

$ARGUMENTS

Focus areas if specified in arguments:
- Specific packages to audit
- Specific dependency categories to check
- Output format preferences
