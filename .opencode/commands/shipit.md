---
description: Verifica TODOs e executa os jobs locais do ci.yml do vyx
agent: delivery-loop
---

Execute a validação de entrega desta tarefa no projeto vyx.

**Passo 1 — verificar TODOs:**
!`npx tsx scripts/check-todos.ts .task-state.json 2>&1 || echo '{"ok":false,"error":"task-state.json nao encontrado"}'`

**Passo 2 — executar jobs Go locais:**
!`npx tsx scripts/workflow-agent.ts .github/workflows/ci.yml go-test 2>&1`

**Passo 3 — executar jobs Python locais:**
!`npx tsx scripts/workflow-agent.ts .github/workflows/ci.yml python-sdk-test 2>&1`

**Passo 4 — executar scanner test:**
!`npx tsx scripts/workflow-agent.ts .github/workflows/ci.yml scanner-test 2>&1`

**Passo 5 — executar Node.js SDK test:**
!`npx tsx scripts/workflow-agent.ts .github/workflows/ci.yml node-sdk-test 2>&1`

Com base em toda a saída acima, responda obrigatoriamente com:

```
STATUS GERAL: [SUCCESS | FAILED]

TODOs:
- [x] id: título (evidência encontrada)
- [ ] id: título (pendente: motivo)

JOBS:
- go-test: [SUCCESS | FAILED] — motivo se falhou
- python-sdk-test: [SUCCESS | FAILED] — motivo se falhou
- scanner-test: [SUCCESS | FAILED] — motivo se falhou
- node-sdk-test: [SUCCESS | FAILED] — motivo se falhou

PRÓXIMO PASSO: [FINALIZAR | patch necessário descrito aqui]
```

Argumentos extras recebidos: $ARGUMENTS
