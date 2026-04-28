## Regras de CI/CD

### workflow-agent
O script `scripts/workflow-agent.ts` executa localmente os jobs do CI que não dependem de secrets externos.

Saída JSON linha a linha:
- `job_started` — início de um job
- `step_started` / `step_finished` — início/fim de cada step com `exitCode`
- `job_finished` — status: `success` | `failed` | `skipped`
- `workflow_finished` — resultado final

Jobs pulados automaticamente (requerem secrets/serviços externos):
- `sonarcloud`, `secrets-scan`, `semgrep`, `codeql`, `snyk`

### check-todos
`scripts/check-todos.ts` verifica se os arquivos dos TODOs existem.
Saída JSON: `{ ok: boolean, totals, results }`

### Pré-requisitos locais
- Docker disponível no PATH
- Node.js ≥ 20
- `bash scripts/install-deps.sh`
