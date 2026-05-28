# Plano de Implementação — Segurança VYX

**Baseado em:** `SECURITY_ARCHITECTURE.md`, `AUDIT_REPORT.md`, `docs/adr/`, `docs/ARCHITECTURE_TO_CODE_MAP.md`
**Issues:** #116 a #155
**Sprints:** 14 (divididas em 5 fases)
**Duração estimada:** 14-18 semanas

---

## Sprint 1 — Correções Críticas (P0)

**Foco:** Resolver as 5 vulnerabilidades críticas que podem comprometer o sistema em produção.

| Issue | Descrição | Arquivo | Esforço | Risco se não fizer |
|-------|-----------|---------|---------|-------------------|
| #116 | Python SDK IPC constants mismatch | `packages/python/vyx/ipc.py` | 15 min | Workers Python incomunicáveis |
| #117 | JWT secret vazio (auth bypass) | `core/cmd/vyx/main.go:724` | 30 min | Bypass total de autenticação |
| #118 | Zero panic recovery (43 goroutines) | Múltiplos arquivos | 4h | Qualquer panic derruba o core |
| #119 | Data race em cmd.Wait() | `core/infrastructure/process/manager.go:94+120` | 30 min | Race condition + comportamento indefinido |
| #120 | Windows sem graceful shutdown | `core/infrastructure/process/manager_windows.go` | 4h | Perda de dados no Windows |

### Entregáveis da Sprint 1
- ✅ Python SDK funcional (workers Python se comunicam com o core)
- ✅ JWT com secret mínimo 32 bytes (startup falha se não configurado)
- ✅ Helper `vyx.Recover()` aplicado em todas as 43 goroutines
- ✅ `cmd.Wait()` chamado uma única vez (data race eliminado)
- ✅ Windows: `GenerateConsoleCtrlEvent` + fallback `TerminateProcess`

### Checklist de Aceitação
- [ ] `go test -race ./...` passa sem races
- [ ] Workers Python conectam e processam requisições
- [ ] Sem JWT_SECRET → startup falha (exit code 1)
- [ ] `runtime.Stack()` logado quando goroutine panica (não crash)
- [ ] `go build ./...` passa em Windows (cross-compile)

---

## Sprint 2 — Schema e Headers (P1)

**Foco:** Endurecer validação de entrada/saída e headers de segurança.

| Issue | Descrição | ADR | Esforço |
|-------|-----------|-----|---------|
| #124 | Headers sem sanitização CRLF | — | 2h |
| #128 | Implementar CORS middleware | — | 4h |
| #136 | Adicionar `Permissions-Policy` | — | 30min |
| #129 | Schema WarmUp no startup | — | 1h |
| #122 | Circuit breaker key usa req.Path | — | 1h |
| #125 | Stop antes de Drain no RestartWorker | — | 2h |
| #130 | Python SDK: path params | — | 4h |
| #131 | Node.js SDK: roteamento O(n)→trie | — | 4h |

### Entregáveis da Sprint 2
- ✅ Headers HTTP sanitizados (CRLF stripping em request e response)
- ✅ CORS configurável (fechado por padrão, allowlist explícita)
- ✅ `Permissions-Policy` header em todas as respostas
- ✅ Schemas compilados no startup (WarmUp)
- ✅ Circuit breaker usa `route.Path` (sem memory leak)
- ✅ RestartWorker: Drain antes de Stop
- ✅ Python SDK: path params funcionando
- ✅ Node.js SDK: trie routing

### Checklist de Aceitação
- [ ] Teste de CRLF injection: headers com `\r\n` são rejeitados
- [ ] CORS: requisição cross-origin sem allowlist → bloqueada
- [ ] Circuit breaker: `/api/users/1` e `/api/users/2` compartilham mesmo breaker
- [ ] Python: `@Route(GET /api/users/:id)` funciona
- [ ] Node.js: 100+ rotas sem degradação de performance

---

## Sprint 3 — Performance do Hot Path (P1)

**Foco:** Otimizar o pipeline de dispatch para reduzir latência e alocação.

| Issue | Descrição | ADR | Esforço |
|-------|-----------|-----|---------|
| #121 | json.Marshal → msgpack no IPC | — | 4h |
| #127 | routeKey calculado 3-4x → 1x | — | 2h |
| #126 | Goroutine fire-and-forget em selectWorker | — | 4h |
| #132 | RouteMap.Lookup sem alocação de params | — | 2h |
| #133 | sync.Pool para buffers de frame | — | 2h |
| #134 | Lock contention no pool | — | 4h |
| #135 | Lock contention no drainer | — | 3h |

### Entregáveis da Sprint 3
- ✅ IPC usa msgpack em vez de JSON (30-50% menos alocação)
- ✅ routeKey calculado uma vez, armazenado no LifecycleContext
- ✅ selectWorker sem goroutine leak (defer no chamador)
- ✅ RouteMap.Lookup sem alocação de params para rotas estáticas
- ✅ sync.Pool para buffers de frame (reduz GC pressure)
- ✅ Pool.IncrementActiveReqs sem lock (sync.Map)
- ✅ Drainer sem mutex global (sync.Map[string]*WaitGroup)

### Checklist de Aceitação
- [ ] `go test -bench=. -benchmem` mostra melhoria mensurável
- [ ] P99 latency do dispatch reduzido
- [ ] Race detector não encontra novos races
- [ ] `go test ./... -count=1` passa

---

## Sprint 4 — AuthN e Config (P1 + P2)

**Foco:** Completar validação de JWT, configuração segura e observabilidade.

| Issue | Descrição | ADR | Esforço |
|-------|-----------|-----|---------|
| #152 | Go toolchain 1.25.0 → 1.25.10 | — | 1h |
| #153 | Credenciais hardcoded em exemplos | — | 2h |
| #154 | DevDependencies duplicadas | — | 30min |
| #137 | Erros de resposta vazam detalhes | — | 3h |
| #145 | Rate limiter fixed window → sliding | — | 3h |
| #146 | Rate limit sem Retry-After | — | 2h |
| #144 | Cipher suites TLS (só AEAD) | — | 1h |
| #138 | Monitor marca Starting como unhealthy | — | 1h |
| #139 | Monitor.backoffs sem limite | — | 1h |
| #140 | Pool replenish goroutines leak | — | 2h |

### Entregáveis da Sprint 4
- ✅ Go toolchain 1.25.10 (22 vulnerabilidades stdlib corrigidas)
- ✅ Exemplos sem credenciais hardcoded (env vars + avisos)
- ✅ package.json limpo (sem duplicatas)
- ✅ Respostas de erro com código VYX-* e mensagens genéricas
- ✅ Rate limiter sliding window (token bucket)
- ✅ Header `Retry-After` nas respostas 429
- ✅ TLS cipher suites restritas (só AEAD)
- ✅ Monitor: pula workers Starting, backoffs com limite
- ✅ Pool: replenish com contexto de lifecycle

### Checklist de Aceitação
- [ ] `govulncheck ./...` mostra 0 vulnerabilidades
- [ ] Erros 500 retornam `{"error":{"code":"VYX-INTERNAL-001"}}` (sem stack)
- [ ] Rate limiter com sliding window não tem efeito stampede
- [ ] TLS handshake usa apenas AEAD ciphers

---

## Sprint 5 — Validação de Resposta (Fase 1)

**Foco:** Implementar schema de resposta obrigatório e validação de saída.

| Item | Descrição | ADR | Esforço |
|------|-----------|-----|---------|
| ADR-003 | Schema de resposta obrigatório (@Response) | ADR-003 | 5 dias |
| — | @Response annotation no scanner | — | 2 dias |
| — | Validação de resposta no orquestrador | — | 3 dias |
| — | Remoção de campos não declarados | — | 2 dias |
| — | Build step: falha se rota sem @Response | — | 1 dia |

### Entregáveis da Sprint 5
- ✅ Annotation `@Response(nome-do-schema)` no scanner Go/TS/Python
- ✅ Orquestrador valida resposta do worker contra schema
- ✅ Campos extras removidos (não chegam ao cliente)
- ✅ Build step falha se rota GET não tem `@Response`
- ✅ Documentação: schemas de resposta geram OpenAPI

### Checklist de Aceitação
- [ ] Rota GET sem `@Response` → erro no `vyx build`
- [ ] Worker retorna campo extra → removido silenciosamente
- [ ] Worker retorna tipo errado → 502 Bad Gateway
- [ ] Worker retorna campo `password` → rejeitado (schema não lista)

---

## Sprint 6 — SecurityContext (Fase 1)

**Foco:** Implementar propagação de contexto de segurança entre orquestrador e workers.

| Item | Descrição | ADR | Esforço |
|------|-----------|-----|---------|
| ADR-001 | SecurityContextToken formato + assinatura | ADR-001 | 5 dias |
| — | Propagação no payload IPC | — | 2 dias |
| — | Validação de retorno (assinatura + nonce + TTL) | — | 2 dias |
| — | SDK Go: ctx.UserID(), ctx.HasRole() | — | 2 dias |
| — | SDK Python: ctx.user_id, ctx.has_role() | — | 2 dias |
| — | SDK Node.js: ctx.userId, ctx.hasRole() | — | 2 dias |

### Entregáveis da Sprint 6
- ✅ SecurityContextToken com HMAC-SHA256 + nonce + TTL
- ✅ Orquestrador valida token na resposta do worker
- ✅ Go SDK: `ctx.UserID()`, `ctx.HasRole()`, `ctx.TenantID()`, `ctx.CorrelationID()`
- ✅ Python SDK: `ctx.user_id`, `ctx.has_role()`, `ctx.tenant_id`, `ctx.correlation_id`
- ✅ Node.js SDK: `ctx.userId`, `ctx.hasRole()`, `ctx.tenantId`, `ctx.correlationId`
- ✅ Testes de conformidade (mesmo comportamento nos 3 runtimes)

### Checklist de Aceitação
- [ ] SecurityContext chega ao worker com dados corretos
- [ ] Worker não consegue adulterar (assinatura quebra)
- [ ] Nonce repetido dentro do TTL → 502
- [ ] Teste de conformidade: mesma request → mesmos claims nos 3 runtimes

---

## Sprint 7 — Route Inventory + Content Type (Fase 1-2)

**Foco:** Completar validações de configuração e inventário de rotas.

| Item | Descrição | Esforço |
|------|-----------|---------|
| Route inventory: build step valida schemas | 3 dias |
| Content-Type validation (só JSON) | 1 dia |
| Method validation (allowlist) | 1 dia |
| CORS com preflight (OPTIONS) | 2 dias |
| Error handling padronizado (códigos VYX-*) | 3 dias |

### Entregáveis da Sprint 7
- ✅ `vyx build` falha se rota sem schema de entrada OU saída
- ✅ Content-Type diferente de `application/json` → 415
- ✅ Método fora da allowlist → 405
- ✅ OPTIONS tratado automaticamente (CORS preflight)
- ✅ Todos os erros seguem formato VYX-*

---

## Sprint 8 — SSRF Protection (Fase 2)

**Foco:** Implementar client HTTP seguro com proteção SSRF nos 3 runtimes.

| Item | Descrição | ADR | Esforço |
|------|-----------|-----|---------|
| ADR-005 | Client HTTP com SSRF protection (Go) | ADR-005 | 3 dias |
| ADR-005 | Client HTTP com SSRF protection (Python) | ADR-005 | 3 dias |
| ADR-005 | Client HTTP com SSRF protection (Node.js) | ADR-005 | 3 dias |
| — | Bloqueio em 2 camadas (pré-DNS + pós-DNS) | — | 5 dias |
| — | Allowlist por worker (redes internas) | — | 2 dias |
| — | Testes de conformidade SSRF | — | 3 dias |

### Entregáveis da Sprint 8
- ✅ `vyx.HTTPClient` nos 3 runtimes com proteção SSRF
- ✅ Bloqueio RFC 1918, localhost, metadata cloud
- ✅ Bloqueio em 2 camadas (DNS rebinding prevention)
- ✅ Allowlist configurável por worker
- ✅ 10+ testes de conformidade SSRF

### Checklist de Aceitação
- [ ] `http://169.254.169.254/` → SSRF_BLOCKED (AWS metadata)
- [ ] `http://127.0.0.1:9200/` → SSRF_BLOCKED
- [ ] `http://10.0.0.1/` → SSRF_BLOCKED
- [ ] URL com redirect para IP privado → SSRF_BLOCKED
- [ ] Worker com allowlist: `10.100.0.0/16` → access liberado

---

## Sprint 9-10 — Policy DSL (Fase 3)

**Foco:** Implementar o modelo de autorização declarativa (Policy DSL + PDP + PEP).

| Item | Descrição | ADR | Esforço |
|------|-----------|-----|---------|
| ADR-002 | PDP em Go (Policy Decision Point) | ADR-002 | 5 dias |
| ADR-002 | Policy loader (arquivos .policy.json) | ADR-002 | 3 dias |
| ADR-002 | SDK: ctx.HasPermission(), ctx.CanAccess() | ADR-002 | 5 dias |
| — | Cache de decisões (TTL 5s) | — | 2 dias |
| — | Object-level authorization helpers | — | 5 dias |
| — | Property-level authorization (schemas condicionais) | ADR-003 | 5 dias |
| — | Tenant isolation (injeção automática em queries) | — | 5 dias |

### Entregáveis da Sprint 9-10
- ✅ PDP em Go com avaliação de policies
- ✅ Policy DSL com operadores: eq, neq, in, match, all, any, none, before
- ✅ Regras de precedência (deny > allow, deny by default)
- ✅ Cache de decisões (TTL 5s, invalidação por evento)
- ✅ `ctx.HasPermission("order:cancel")` nos 3 runtimes
- ✅ `ctx.CanAccessResource("order", orderId)` nos 3 runtimes
- ✅ Property-level authorization (schemas com `vyx:authorization`)
- ✅ Tenant isolation automático (tenant_id injetado em queries)

### Checklist de Aceitação
- [ ] Policy DSL avaliada corretamente (testes de unidade)
- [ ] Cache hit: < 1ms, Cache miss: < 5ms
- [ ] Deny by default: requisição sem policy → negada
- [ ] Tenant isolation: user do tenant A não acessa dados do tenant B
- [ ] Cross-tenant query → 403

---

## Sprint 11 — Observabilidade (Fase 3)

**Foco:** Implementar security event taxonomy, audit trail e alertas.

| Item | Descrição | Esforço |
|------|-----------|---------|
| Security event taxonomy (struct Event) | 3 dias |
| Audit logger (imutável, estruturado) | 3 dias |
| Log redaction (password, token, secret) | 2 dias |
| Correlation ID em todos os eventos | 1 dia |
| Métricas de segurança (authN failure, 4xx, 5xx) | 3 dias |
| Alertas obrigatórios (thresholds) | 2 dias |

### Entregáveis da Sprint 11
- ✅ Security event taxonomy implementada
- ✅ Audit trail: authN, authZ, schema violation, secret access
- ✅ Log redaction automático (14 campos sensíveis)
- ✅ Correlation ID em todo evento
- ✅ Métricas Prometheus para eventos de segurança
- ✅ Alertas: taxa de falha authN > 10%, circuit breaker open, worker crash

---

## Sprint 12 — Hardening (Fase 4)

**Foco:** mTLS, key rotation, fuzzing.

| Item | Descrição | ADR | Esforço |
|------|-----------|-----|---------|
| Key rotation (key ring + rotação 24h) | ADR-004 | 5 dias |
| mTLS service-to-service (workers) | ADR-004 | 5 dias |
| Fuzzing pipeline no CI | — | 5 dias |
| Account lockout (N falhas) | — | 3 dias |
| MFA plugável (interface) | — | 5 dias |

### Entregáveis da Sprint 12
- ✅ Key ring com rotação automática a cada 24h
- ✅ mTLS para workers remotos (TCP)
- ✅ Fuzzing: go-fuzz (Go), Atheris (Python), jsfuzz (Node)
- ✅ Account lockout após N falhas (configurável)
- ✅ Interface MFAProvider (TOTP implementation reference)

---

## Sprint 13 — Conformidade (Fase 5)

**Foco:** Test suite de conformidade cross-runtime, SBOM, ASVS.

| Item | Descrição | Esforço |
|------|-----------|---------|
| Test suite de conformidade (Go) | 5 dias |
| Test suite de conformidade (Python) | 5 dias |
| Test suite de conformidade (Node.js) | 5 dias |
| SBOM generation no CI | 2 dias |
| OWASP ASVS Nível 1 assessment | 5 dias |
| Security documentation | 3 dias |

### Entregáveis da Sprint 13
- ✅ Test suite de conformidade para Go (100+ testes)
- ✅ Test suite de conformidade para Python (mesmos testes)
- ✅ Test suite de conformidade para Node.js (mesmos testes)
- ✅ SBOM gerado automaticamente (go.spdx, npm sbom, pip sbom)
- ✅ Relatório OWASP ASVS Nível 1
- ✅ Security playbook (incident response)

---

## Sprint 14 — Refinamento e Documentação

**Foco:** Fechar gaps, documentar, treinar equipe.

| Item | Descrição | Esforço |
|------|-----------|---------|
| Golden tests de autorização | 5 dias |
| Penetration test | 5 dias |
| Documentação de segurança para usuários | 5 dias |
| Treinamento da equipe | 3 dias |
| Revisão final dos ADRs | 2 dias |

### Entregáveis da Sprint 14
- ✅ Golden tests: mesmas entradas → mesmas decisões de authz nos 3 runtimes
- ✅ Relatório de penetration test (sem críticos)
- ✅ Documentação pública de segurança (secure by default)
- ✅ Equipe treinada nos princípios de segurança da framework

---

## Resumo do Roadmap

| Sprint | Fase | Foco | Issues | Esforço |
|--------|------|------|--------|---------|
| 1 | P0 | Correções críticas | #116-#120 | 1 semana |
| 2 | P1 | Schema + Headers | #122, #124, #125, #128, #129, #130, #131, #136 | 1 semana |
| 3 | P1 | Performance hot path | #121, #126, #127, #132, #133, #134, #135 | 1 semana |
| 4 | P1+P2 | AuthN + Config | #137, #138, #139, #140, #144, #145, #146, #152, #153, #154 | 1 semana |
| 5 | Fase 1 | Response validation | ADR-003 | 1 semana |
| 6 | Fase 1 | SecurityContext | ADR-001 | 1 semana |
| 7 | Fase 1-2 | Route inventory | — | 1 semana |
| 8 | Fase 2 | SSRF protection | ADR-005 | 1 semana |
| 9-10 | Fase 3 | Policy DSL + Authz | ADR-002, ADR-003 | 2 semanas |
| 11 | Fase 3 | Observabilidade | — | 1 semana |
| 12 | Fase 4 | Hardening | ADR-004 | 1 semana |
| 13 | Fase 5 | Conformidade | — | 1 semana |
| 14 | Fase 5 | Refinamento | — | 1 semana |
| **Total** | **5 fases** | **14 sprints** | **40 issues + 5 ADRs** | **14 semanas** |

---

## Dependências entre Sprints

```
Sprint 1 (P0) ──────────────────── Sem dependências
       │
Sprint 2 (Schema/Headers) ──────── Sem dependências (pode rodar em paralelo com S3)
       │
Sprint 3 (Performance) ─────────── Sem dependências (pode rodar em paralelo com S2)
       │
Sprint 4 (AuthN/Config) ────────── Depende de S1 (#117 JWT)
       │
       ├── Sprint 5 (Response) ──── Depende de S4 (códigos VYX-*)
       │
       ├── Sprint 6 (SecurityCtx) ── Depende de S4 (JWT validation)
       │                              Depende de S1 (#116 Python IPC)
       │
       ├── Sprint 7 (Inventory) ──── Depende de S5 (@Response)
       │
       ├── Sprint 8 (SSRF) ──────── Independente (pode começar cedo)
       │
       ├── Sprint 9-10 (Policy) ──── Depende de S6 (SecurityContext)
       │                              Depende de S1 (#118 panic recovery)
       │
       ├── Sprint 11 (Observab.) ─── Depende de S4 (erros VYX-*)
       │
       └── Sprint 12 (Hardening) ─── Depende de S9-10 (mTLS + authz)
                                      Depende de S6 (key rotation)

Sprint 13 (Conformidade) ────────── Depende de S9-10 (Policy DSL)
                                    Depende de S6 (SecurityContext SDKs)
Sprint 14 (Refinamento) ─────────── Depende de S13 (golden tests)
```

---

## Riscos do Plano

| Risco | Probabilidade | Impacto | Mitigação |
|-------|-------------|---------|-----------|
| Sprint 1 (P0) não termina em 1 semana | Baixa | Crítico | São mudanças de 1-4h cada; priorizar #116 e #117 |
| Sprints 2-3 dependem de mudanças no dispatcher | Média | Alto | Dispatcher é o arquivo mais crítico; revisão de código obrigatória |
| Policy DSL (Sprint 9-10) é complexa | Alta | Médio | Fazer protótipo em Go antes de expandir para Python/Node |
| Testes de conformidade cross-runtime (Sprint 13) | Média | Médio | Começar com Go, depois expandir; aceitar 80% cobertura inicial |
| mTLS (Sprint 12) adiciona complexidade operacional | Alta | Baixo | Adiar para Fase 4; UDS + JWT é suficiente para MVP |
| Equipe não conhece todos os runtimes | Média | Médio | Parear Go+Python, Go+Node para cada implementação cross-runtime |
