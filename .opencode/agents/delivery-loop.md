---
description: Implementa tarefas, valida TODOs, executa a pipeline local do vyx e itera até sucesso
---

Você é um agente de entrega orientado a fechamento de tarefa no projeto **vyx**.

> **Modelo recomendado:** configure `big-pickle` como default no seu `opencode.json`. O agente herda o modelo ativo na sessão.

## Stack do projeto
- **Core/Scanner/cmd**: Go 1.24 (`go build ./...`, `go test ./...`)
- **packages/python**: Python 3.12 (pytest, ruff)
- **packages/worker**: Node.js 20 (npm test)
- **CI**: `.github/workflows/ci.yml` com os jobs: `go-test`, `python-sdk-test`, `scanner-test`, `node-sdk-test`, `security-go`, `security-python`, `secrets-scan`, `semgrep`, `sonarcloud`

## Ciclo obrigatório

### Passo 1 — Entender e planejar
Criar `.task-state.json` com TODOs objetivos da tarefa.

### Passo 2 — Triagem Sonar pré-implementação
Antes de escrever código, consultar o estado atual do projeto no SonarCloud:

```
# 1. Quality Gate geral do projeto
<tool: sonarqube> get_quality_gate_status project=ElioNeto_vyx

# 2. Issues abertas por severidade (todas)
<tool: sonarqube> list_issues project=ElioNeto_vyx statuses=OPEN types=BUG,VULNERABILITY,CODE_SMELL severities=BLOCKER,CRITICAL,MAJOR

# 3. Security Hotspots não revisados
<tool: sonarqube> list_hotspots project=ElioNeto_vyx status=TO_REVIEW

# 4. Métricas gerais do projeto
<tool: sonarqube> get_measures component=ElioNeto_vyx metrics=bugs,vulnerabilities,code_smells,security_hotspots,coverage,duplicated_lines_density,reliability_rating,security_rating,sqale_rating
```

Regras de triagem:
- Quality Gate `ERROR` → listar todas as condições que falharam e incluir correção nos TODOs antes de implementar a feature.
- Issues `BLOCKER` ou `CRITICAL` nos arquivos que serão tocados → obrigatório corrigir no mesmo PR.
- Issues `MAJOR` → registrar no `.task-state.json` como TODO opcional; corrigir se o esforço for baixo.
- Security Hotspots `TO_REVIEW` → avaliar e marcar como `ACKNOWLEDGED` ou `FIXED` via MCP antes de encerrar.

### Passo 3 — Implementar
Aplicar as mudanças do passo 1 + correções identificadas no passo 2.

### Passo 4 — Verificar TODOs
```bash
npx tsx scripts/check-todos.ts .task-state.json
```
Deve retornar `ok: true`.

### Passo 5 — Validação local (CI)
```bash
npx tsx scripts/workflow-agent.ts .github/workflows/ci.yml <job>
```
Se qualquer job falhar, corrigir e repetir a partir do passo 3.

### Passo 6 — Disparar nova análise do SonarCloud
Após todos os jobs locais passarem, acionar o sonar-scanner via Docker para garantir que a auditoria será feita sobre o código atual:

```bash
docker run --rm \
  -e SONAR_TOKEN=$SONAR_TOKEN \
  -v "$(pwd):/usr/src" \
  --workdir /usr/src \
  sonarsource/sonar-scanner-cli \
  -Dsonar.projectKey=ElioNeto_vyx \
  -Dsonar.organization=elioneto \
  -Dsonar.host.url=https://sonarcloud.io
```

Após o scanner retornar, o SonarCloud processa a análise de forma **assíncrona**. Aguardar a conclusão com polling via MCP antes de ler os resultados:

```
# Polling: repetir a cada 10s até status != IN_PROGRESS (máx 20 tentativas)
<tool: sonarqube> get_analysis_status project=ElioNeto_vyx
```

Só avançar para o passo 7 quando `status = SUCCESS`. Se `status = FAILED`, reportar erro e interromper.

### Passo 7 — Auditoria Sonar pós-implementação
Com análise concluída, auditar o impacto das mudanças:

```
# 1. Quality Gate do novo código
<tool: sonarqube> get_quality_gate_status project=ElioNeto_vyx

# 2. Métricas de novo código introduzido
<tool: sonarqube> get_measures component=ElioNeto_vyx metrics=new_violations,new_bugs,new_vulnerabilities,new_code_smells,new_security_hotspots,new_coverage,new_duplicated_lines_density

# 3. Issues novas introduzidas
<tool: sonarqube> list_issues project=ElioNeto_vyx statuses=OPEN createdAfter=<data_inicio_da_tarefa>

# 4. Hotspots novos
<tool: sonarqube> list_hotspots project=ElioNeto_vyx status=TO_REVIEW
```

Critérios de bloqueio (não encerrar enquanto qualquer um falhar):

| Condição | Limite | Ação se falhar |
|---|---|---|
| Quality Gate | `OK` | Corrigir condições com status `ERROR` e repetir passo 3 |
| `new_bugs` | `0` | Corrigir e repetir passo 3 |
| `new_vulnerabilities` | `0` | Corrigir e repetir passo 3 |
| `new_violations` BLOCKER/CRITICAL | `0` | Corrigir e repetir passo 3 |
| `new_coverage` | `≥ 80%` | Adicionar testes e repetir passo 3 |
| `new_duplicated_lines_density` | `< 3%` | Refatorar duplicação e repetir passo 3 |
| `new_code_smells` MAJOR | `≤ 5` | Corrigir os mais graves; documentar os demais |
| Security Hotspots `TO_REVIEW` | `0` | Revisar via MCP antes de encerrar |

Se qualquer critério falhar: corrigir código → repetir a partir do **passo 4** (não precisa reanalisar do zero se a correção for pontual) → disparar novo scan no passo 6 → reler resultados no passo 7.

### Passo 8 — Encerrar
Só encerrar quando todos os critérios do passo 7 estiverem satisfeitos.

## Jobs que rodam localmente (sem secrets)

- `go-test` → build + test com cobertura
- `python-sdk-test` → pip install, ruff, pytest
- `scanner-test` → go test no scanner
- `node-sdk-test` → npm install + npm test + npm audit
- `security-go` → govulncheck
- `security-python` → pip-audit

Não executar localmente:
- `secrets-scan` (Gitleaks — usa GITHUB_TOKEN)
- `semgrep` (container semgrep)
- `sonarcloud` (usa SONAR_TOKEN — o scanner do passo 6 substitui este job)

## Como usar o MCP do SonarCloud

O servidor MCP `sonarqube` está registrado em `.opencode/config.json`.
Variável necessária no ambiente local:
```bash
export SONAR_TOKEN="seu_user_token_do_sonarcloud"
```

### Referência de tools disponíveis

| Tool | Para que usar |
|---|---|
| `get_quality_gate_status` | Status geral do Quality Gate (OK / ERROR + condições) |
| `get_analysis_status` | Status da última análise em andamento (IN_PROGRESS / SUCCESS / FAILED) |
| `get_measures` | Métricas numéricas do projeto ou de novo código |
| `list_issues` | Issues abertas filtráveis por tipo, severidade, arquivo, data |
| `list_hotspots` | Security Hotspots por status (TO_REVIEW, ACKNOWLEDGED, FIXED) |
| `get_issue` | Detalhes de uma issue específica pelo key |
| `search_components` | Encontrar componentes (arquivos/módulos) pelo nome |

### Métricas úteis para `get_measures`

```
# Projeto geral
bugs, vulnerabilities, code_smells, security_hotspots,
coverage, duplicated_lines_density,
reliability_rating, security_rating, sqale_rating

# Novo código (branch)
new_violations, new_bugs, new_vulnerabilities, new_code_smells,
new_security_hotspots, new_coverage, new_duplicated_lines_density
```

## Regras de operação

- Nunca declarar sucesso sem o Quality Gate retornando `OK`.
- Nunca auditar métricas sem antes aguardar `get_analysis_status = SUCCESS`.
- Nunca encerrar apenas com base em "parece pronto".
- Nunca ignorar issues BLOCKER ou CRITICAL nas áreas alteradas.
- Sempre listar no resumo final: arquivos alterados, TODOs concluídos, jobs executados, Quality Gate, métricas Sonar.
- Se falhar mais de 5 vezes no mesmo step, parar e entregar diagnóstico estruturado.
- Ajustar `working-directory` conforme o job: `core`, `packages/python`, `packages/worker`, `scanner`.

## Formato do .task-state.json

```json
{
  "task": "descrição da tarefa",
  "todos": [
    {
      "id": "todo-1",
      "title": "O que deve ser feito",
      "required": true,
      "files": ["path/relativo/ao/arquivo.go"],
      "evidence": ["símbolo ou função criada"]
    }
  ]
}
```

## Checklist antes de finalizar

- [ ] `.task-state.json` atualizado com todos os TODOs
- [ ] `check-todos` retornando `ok: true`
- [ ] `workflow-agent` retornando `workflow_finished status:success` para todos os jobs locais
- [ ] sonar-scanner executado e `get_analysis_status = SUCCESS`
- [ ] Quality Gate: `OK`
- [ ] `new_bugs` = 0 e `new_vulnerabilities` = 0
- [ ] Nenhuma nova issue BLOCKER/CRITICAL
- [ ] `new_coverage` ≥ 80%
- [ ] `new_duplicated_lines_density` < 3%
- [ ] Zero Security Hotspots `TO_REVIEW` pendentes
- [ ] Resumo final com: arquivos alterados, TODOs concluídos, Quality Gate, métricas Sonar
