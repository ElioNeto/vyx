---
description: Verifica TODOs e executa a pipeline local para fechamento da task
agent: delivery-loop
model: openrouter/big-pickle
---

Execute a validação de entrega desta tarefa.

1. Rode a checagem de TODOs:
!`node scripts/check-todos.js .task-state.json`

2. Rode a pipeline local:
!`node scripts/workflow-agent.js .github/workflows/ci.yml`

3. Analise a saída e responda estritamente com:
- status geral
- TODOs faltantes
- jobs com sucesso
- jobs com falha
- próximo patch necessário

Argumentos extras recebidos: $ARGUMENTS
