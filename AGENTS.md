# AGENTS.md

<!-- Este arquivo é gerado automaticamente pelo boilerplate-opencode. -->
<!-- Não edite manualmente a seção entre as tags AUTO-GENERATED. -->
<!-- Preencha as seções marcadas com > após a instalação. -->

## Projeto

**vyx** — um framework full-stack poliglota de alta performance onde um Core Orchestrator em Go gerencia workers em Go, Node.js e Python. Roteamento baseado em anotações estáticas (@Route, @Auth, @Validate, @Page) que geram um route_map.json consumido pelo Core.

## Stack

- **Core**: Go 1.25, Clean Architecture (domain → application → infrastructure)
- **Workers**: Node.js (TypeScript, @vyx/worker), Python 3.12 (vyx package), Go
- **IPC**: Unix Domain Sockets + MsgPack + Apache Arrow (protocolo binário)
- **Scanner**: Go puro — parse estático de anotações em Go/TS/TSX/Python
- **HTTP Gateway**: JWT, JSON Schema, Rate Limiter, Circuit Breaker
- **CLI**: Cobra (Go) — dev, build, new, annotate
- **CI/CD**: GitHub Actions (12+ jobs), Docker multi-stage, SonarCloud, Codecov
- **TUI**: Bubble Tea (Go) para log tailing

<!-- AUTO-GENERATED:START -->
## Regras gerais

### Commits
- Seguir Conventional Commits: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`
- Mensagens em inglês, imperativo presente: "add feature" não "added feature"
- Commits atômicos: uma responsabilidade por commit

### Pull Requests
- Título segue Conventional Commits
- Descrição inclui: o que foi feito, por que, como testar
- PR sem testes não é mergeada

### Código
- Sem código morto ou comentado
- Sem debugging esquecido (`console.log`, `fmt.Println`, `print()`)
- Sem secrets no código
- Tratamento de erros obrigatório

### CI/CD
- Pipeline deve passar antes do merge
- Jobs locais devem ser validados com `workflow-agent` antes de abrir PR
- Arquivo `.task-state.json` deve estar limpo após conclusão da tarefa

## Regras de CI/CD

### workflow-agent
O script `scripts/workflow-agent.ts` executa localmente os jobs do `ci.yml` que não dependem de secrets externos.

Saída JSON linha a linha:
- `job_started` — início de um job
- `step_started` / `step_finished` — início/fim de cada step com `exitCode`
- `job_finished` — status do job (`success` | `failed` | `skipped`)
- `workflow_finished` — resultado final

Jobs pulados automaticamente quando requerem secrets externos:
- `secrets-scan`, `semgrep`, `sonarcloud` e similares

### check-todos
O script `scripts/check-todos.ts` verifica se os arquivos listados nos TODOs do `.task-state.json` existem.

Saída JSON: `{ ok: boolean, totals: {...}, results: [...] }`

### Pré-requisitos locais
- Docker disponível no PATH
- Node.js ≥ 20
- `cd scripts && npm install`

## Regras Go

### Convenções
- Seguir o [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Nomes de pacotes: lower case, sem underscores
- Interfaces: sufixo `-er` quando possível (`Reader`, `Writer`, `Handler`)
- Erros: sempre tratados, nunca `_`
- `context.Context` sempre como primeiro parâmetro

### Estrutura de pacotes
- `internal/` para código não exportado
- `cmd/` para entry points
- `pkg/` para bibliotecas exportadas (quando aplicável)

### Testes
- Arquivos de teste: `*_test.go` no mesmo pacote
- Table-driven tests para múltiplos casos
- `testify` permitido; prefer `t.Fatal` sobre `t.Error` quando o estado é inválido

### Build e ferramentas
```bash
go build ./...
go test ./... -race -coverprofile=coverage.txt
go vet ./...
golangci-lint run
govulncheck ./...
```

### CI jobs locais
- `go-test`: `go build ./...` + `go test ./...`
- `security-go`: `govulncheck ./...`
<!-- AUTO-GENERATED:END -->

## Comandos úteis

```bash
# Core (Go) — build, test, lint
cd core && go build ./...
cd core && go test ./... -race -count=1
cd core && go vet ./...
cd core && golangci-lint run
cd core && govulncheck ./...

# Scanner (Go)
cd scanner && go test ./... -race -count=1

# Node.js Worker SDK
cd packages/worker && npm install
cd packages/worker && npm test
cd packages/worker && npm run lint

# Python Worker SDK
cd packages/python && pip install -e .
cd packages/python && python -m pytest tests/ -v
cd packages/python && ruff check .

# Workflow agent (local CI)
cd scripts && npm install && npx tsx workflow-agent.ts

# Check task state
cd scripts && npx tsx check-todos.ts

# CLI
cd core && go build -o ../bin/vyx ./cmd/vyx
./bin/vyx dev   # start dev mode
./bin/vyx build # build project
./bin/vyx annotate # scan annotations
./bin/vyx new <name> # scaffold project
```

## Convenções

- **Commits**: Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`, `perf:`)
- **Scopes**: `core`, `scanner`, `worker`, `python`, `cli`, `infra`, `docs`
- **Branches**: `feat/<slug>`, `fix/<slug>`, `chore/<slug>`
- **Go naming**: camelCase vars/funcs, PascalCase exports, interfaces suffix `-er`
- **Testes Go**: `*_test.go` mesmo pacote, table-driven tests, testify
- **Testes Node**: `vitest`, arquivos `*.test.ts`
- **Testes Python**: `pytest`, arquivos `test_*.py`
- **Estrutura**: Clean Architecture em `core/` (domain → application → infrastructure)

## Contexto de domínio

| Termo | Definição |
|-------|-----------|
| **Core** | Processo Go que orquestra tudo (HTTP gateway + worker manager) |
| **Worker** | Processo filho (Go/Node/Python) que executa lógica de negócio |
| **RouteMap** | Trie de rotas construída a partir de route_map.json (hot-swappable) |
| **RouteEntry** | Path + Method + WorkerID + AuthRoles + Validate + Type |
| **Annotation** | @Route, @Auth, @Validate, @Page em comentários de código fonte |
| **Circuit Breaker** | Máquina de estados (Closed → Open → Half-Open) por rota |
| **Worker Pool** | Múltiplos réplicas do mesmo worker com round-robin |
| **UDS** | Unix Domain Sockets para IPC core-worker |
| **Handshake** | Protocolo de registro do worker ao conectar |
| **Heartbeat** | Ping periódico (5s) do worker para o core |
