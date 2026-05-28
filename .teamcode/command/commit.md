---
description: git commit and push

subtask: true
---

commit and push

Use Conventional Commits with these vyx project scopes:
- `core:` — Core Orchestrator Go code
- `scanner:` — Annotation scanner
- `worker:` — Node.js worker SDK
- `python:` — Python worker SDK
- `cli:` — CLI commands
- `infra:` — CI/CD, Docker, build config
- `docs:` — Documentation
- `chore:` — Chores, deps, maintenance

prefer to explain WHY something was done from an end user perspective instead of
WHAT was done.

do not do generic messages like "improved agent experience" be very specific
about what user facing changes were made

if there are conflicts DO NOT FIX THEM. notify me and I will fix them

## GIT DIFF

!`git diff`

## GIT DIFF --cached

!`git diff --cached`

## GIT STATUS --short

!`git status --short`
