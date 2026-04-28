# AGENTS.md — vyx (OmniStack Engine)

## Projeto

**vyx** implementa o **OmniStack Engine**: um framework full-stack poliglota onde um **Core Orchestrator** em Go atua como ponto central de controle. O core gerencia workers (Node.js, Python, Go), faz roteamento baseado em anotações descobertas em tempo de build, e comunica com os workers via **Unix Domain Sockets (UDS) + Apache Arrow**.

O core é o **único processo exposto à rede**. Todos os workers são processos filhos gerenciados pelo core.

## Stack

- **Linguagem principal**: Go (core orchestrator)
- **Workers suportados**: Node.js (TypeScript), Python, Go
- **Frontend**: React com anotações `@Page`
- **IPC**: Unix Domain Sockets + Apache Arrow (datasets grandes) / MsgPack (payloads pequenos)
- **Autenticação**: JWT validado no core; workers recebem apenas claims
- **CLI**: `omni` (new, dev, build, annotate)
- **Testes**: `go test ./... -race`
- **Lint**: `golangci-lint run`
- **Segurança**: `govulncheck ./...`

## Estrutura do repositório

```
cmd/          ← entry points do CLI (omni)
core/         ← Core Orchestrator: dispatcher, circuit breaker, runtime manager
scanner/      ← annotation parser (build time) → gera route_map.json
packages/     ← bibliotecas compartilhadas
examples/     ← projetos de exemplo
docs/         ← documentação
scripts/      ← ferramentas de CI local (workflow-agent, check-todos)
```

## Comandos úteis

```bash
# Build
go build ./...

# Testes com race detector
go test ./... -race -coverprofile=coverage.txt

# Lint
golangci-lint run

# Análise de vulnerabilidades
govulncheck ./...

# Executar CI localmente (antes de abrir PR)
npx tsx scripts/workflow-agent.ts .github/workflows/ci.yml

# Verificar TODOs da task
npx tsx scripts/check-todos.ts .task-state.json

# Instalar dependências dos scripts
bash scripts/install-deps.sh
```

## Convenções

### Go
- Seguir [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- `context.Context` sempre como primeiro parâmetro em funções que fazem I/O
- Erros sempre tratados — nunca `_` para erros
- Interfaces com sufixo `-er` quando possível (`Worker`, `Dispatcher`, `Handler`)
- Pacotes em lowercase sem underscores
- Sem `fmt.Println` de debug no código de produção

### Anotações (sistema de rotas)
- Anotações em comentários Go: `// @Route(METHOD /path)`, `// @Auth(roles: [...])`, `// @Validate(JsonSchema: "name")`
- Scanner (`scanner/`) processa anotações em build time e gera `route_map.json`
- Nunca modificar `route_map.json` manualmente

### Protocolo IPC
- Protocolo binário: `[Length][Type 1 byte][Payload]`
- Types: `0x01` request, `0x02` response, `0x03` heartbeat, `0x04` error
- Payload: JSON para desenvolvimento, MsgPack para produção, Arrow para datasets grandes

### Commits
- Conventional Commits: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`
- Mensagens em inglês, imperativo presente
- PR sem testes não é mergeada

## Ciclo de entrega (obrigatório)

1. Criar `.task-state.json` com TODOs da tarefa
2. Implementar
3. `npx tsx scripts/check-todos.ts .task-state.json` → deve retornar `ok: true`
4. `npx tsx scripts/workflow-agent.ts .github/workflows/ci.yml` → todos os jobs devem passar
5. Só encerrar com ambos passando

<!-- AUTO-GENERATED:START -->
<!-- AUTO-GENERATED:END -->
