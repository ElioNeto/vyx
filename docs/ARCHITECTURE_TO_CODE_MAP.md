# Mapeamento Arquitetura → Código → Gap → Patch

**Objetivo:** Mapear cada requisito da arquitetura de segurança (`SECURITY_ARCHITECTURE.md`) para o código atual, identificando gaps e patches necessários.

**Legenda:**
- ✅ **Implementado** — requisito já funciona conforme a arquitetura
- ⚠️ **Parcial** — requisito existe mas tem gaps
- ❌ **Não implementado** — requisito não existe no código atual
- 📄 **ADR** — decisão documentada, aguardando implementação

---

## 1. AUTENTICAÇÃO JWT

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 1.1 | Validar assinatura HMAC | ✅ | `core/infrastructure/gateway/jwt.go:42` | Nenhum | — |
| 1.2 | Validar `exp` (expiração) | ✅ | `core/infrastructure/gateway/jwt.go:42` (golang-jwt faz isso) | Nenhum | — |
| 1.3 | Validar `nbf` (not before) | ⚠️ | `core/infrastructure/gateway/jwt.go:42` | golang-jwt valida `nbf` por padrão, mas não é verificado explicitamente | Adicionar verificação explícita de `nbf` |
| 1.4 | Validar `iss` (issuer) | ❌ | `core/infrastructure/gateway/jwt.go` | Não existe validação de `iss` | Adicionar campo `ExpectedIssuer` no JWTConfig |
| 1.5 | Validar `aud` (audience) | ❌ | `core/infrastructure/gateway/jwt.go` | Não existe validação de `aud` | Adicionar campo `ExpectedAudience` no JWTConfig |
| 1.6 | Validar `jti` (token ID) contra blacklist | ❌ | `core/infrastructure/gateway/jwt.go` | `jti` parseado mas não verificado contra blacklist | Implementar blacklist de `jti` (Redis para produção, memória para dev) |
| 1.7 | JWT secret mínimo 32 bytes | ❌ | `core/cmd/vyx/main.go:724` | Aceita secret vazio (auth bypass) | Adicionar `if len(secret) < 32 { log.Fatal("...") }` |
| 1.8 | Key rotation com key ring | ❌ | — | Chave única, sem rotação | Implementar `KeyRing` com múltiplas chaves HMAC (ADR-004) |
| 1.9 | MFA plugável | ❌ | — | Não existe | Interface `MFAProvider` (Fase 4) |
| 1.10 | Refresh token | ❌ | — | Só access token | Implementar refresh token opaco + rotação (ADR-001) |

---

## 2. VALIDAÇÃO DE SCHEMA (ENTRADA)

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 2.1 | JSON Schema validation (body) | ✅ | `core/infrastructure/gateway/schema.go:85` | Nenhum | — |
| 2.2 | `additionalProperties: false` | ⚠️ | `core/infrastructure/gateway/schema.go` | O schema valida, mas a framework não EXIGE que o schema tenha `additionalProperties: false` | Validar no build step: falhar se schema não tiver `additionalProperties: false` |
| 2.3 | Validação de path params | ❌ | — | Path params não passam por schema validation | Adicionar schema por path param na anotação `@Route(POST /api/orders/:id{uuid})` |
| 2.4 | Validação de query params | ❌ | — | Query params repassados sem validação | Adicionar schema de query params opcional |
| 2.5 | Limite de profundidade (maxDepth: 10) | ⚠️ | `core/infrastructure/gateway/schema.go` | Schema validator não aplica depth limits | Configurar `maxDepth` no compilador de schemas |
| 2.6 | Limite de cardinalidade (maxItems/maxProperties) | ⚠️ | `core/infrastructure/gateway/schema.go` | Schema validator suporta se configurado no schema, mas não tem defaults seguros | Adicionar defaults: `maxItems: 1000`, `maxProperties: 100` |
| 2.7 | Content-Type validation (só JSON) | ❌ | `core/infrastructure/gateway/server.go` | Não valida Content-Type | Adicionar validação: rejeitar se não for `application/json` |
| 2.8 | Payload size limit (1MB) | ✅ | `core/infrastructure/gateway/server.go` (MaxBodyBytes) | Nenhum | — |
| 2.9 | Schema WarmUp no startup | ❌ | `core/cmd/vyx/main.go` | `WarmUp()` existe em `schema.go:37` mas não é chamado no startup | Adicionar `schemaValidator.WarmUp()` em `runServer()` |
| 2.10 | Rejeitar campos extras (mass assignment) | ⚠️ | `core/infrastructure/gateway/schema.go` | Depende do schema ter `additionalProperties: false` | Tornar obrigatório no build step |

---

## 3. VALIDAÇÃO DE RESPOSTA (SAÍDA)

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 3.1 | Schema de resposta obrigatório | ❌ | — | Não existe `@Response` annotation | Adicionar anotação `@Response(nome)` + validação no orquestrador |
| 3.2 | Validação de resposta contra schema | ❌ | — | Resposta do worker é repassada sem validação | Implementar response validator no orquestrador (ADR-003) |
| 3.3 | Remoção de campos não declarados | ❌ | — | Worker pode retornar qualquer campo | Implementar `additionalProperties: false` na resposta |
| 3.4 | Property-level authorization (campos condicionais) | ❌ | — | Não existe | Implementar `vyx:authorization` em schemas de resposta (ADR-003) |

---

## 4. AUTORIZAÇÃO

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 4.1 | Route-level authorization (@Auth) | ✅ | `core/application/gateway/dispatcher.go:409` | Nenhum | — |
| 4.2 | Scope validation | ❌ | `core/application/gateway/dispatcher.go` | Roles são verificadas, scopes não | Adicionar verificação de scopes após role check |
| 4.3 | Policy DSL (arquivos .policy.json) | ❌ | — | Não existe | Implementar PDP + Policy DSL (ADR-002) |
| 4.4 | Object-level authorization helpers | ❌ | — | Não existe helper `CanAccess()` | Implementar no SDK de cada runtime (ADR-002) |
| 4.5 | Property-level authorization | ❌ | — | Não existe | Implementar schemas condicionais (ADR-003) |
| 4.6 | Tenant isolation automático | ❌ | — | `tenant_id` no contexto mas sem enforcement | Implementar injeção automática de `tenant_id` em queries |
| 4.7 | Service-to-service authorization | ❌ | — | Não existe | Implementar `service_token` + scopes (ADR-004) |

---

## 5. SECURITY HEADERS

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 5.1 | `X-Content-Type-Options: nosniff` | ✅ | `core/application/gateway/security_headers.go:5` | Nenhum | — |
| 5.2 | `X-Frame-Options: DENY` | ✅ | `core/application/gateway/security_headers.go:5` | Nenhum | — |
| 5.3 | `Strict-Transport-Security` (HSTS) | ✅ | `core/application/gateway/security_headers.go:5` | Nenhum | — |
| 5.4 | `Content-Security-Policy` | ✅ | `core/application/gateway/security_headers.go:5` | CSP definido mas pode ser muito restritivo para apps React | Tornar configurável por app |
| 5.5 | `Referrer-Policy: strict-origin-when-cross-origin` | ✅ | `core/application/gateway/security_headers.go:5` | Nenhum | — |
| 5.6 | `Permissions-Policy` | ❌ | `core/application/gateway/security_headers.go:5` | Não existe | Adicionar `Permissions-Policy: geolocation=(), camera=(), microphone=(), payment=(), usb=()` |
| 5.7 | CORS configurável | ❌ | `core/infrastructure/gateway/server.go` | Não existe nenhum header CORS | Implementar CORS middleware configurável |

---

## 6. RATE LIMITING

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 6.1 | Rate limit por IP (sliding window) | ⚠️ | `core/application/gateway/ratelimiter.go` | Algoritmo: fixed window (stampede effect) | Migrar para token bucket (`golang.org/x/time/rate`) |
| 6.2 | Rate limit por identidade | ✅ | `core/application/gateway/ratelimiter.go:58` | Nenhum | — |
| 6.3 | Header `Retry-After` | ❌ | `core/infrastructure/gateway/server.go:193` | Response texto plano sem header | Adicionar `Retry-After` + JSON estruturado |
| 6.4 | Account lockout após N falhas | ❌ | — | Não existe | Implementar lockout tracking |

---

## 7. TIMEOUTS

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 7.1 | `ReadTimeout: 15s` | ✅ | `core/infrastructure/gateway/server.go:120` | Nenhum | — |
| 7.2 | `WriteTimeout: 30s` | ⚠️ | `core/infrastructure/gateway/server.go:126` | Hardcoded `0` (desabilitado) para WebSocket | Usar `http.ResponseController` para deadlines por handler |
| 7.3 | `IdleTimeout: 60s` | ✅ | `core/infrastructure/gateway/server.go:125` | Nenhum | — |
| 7.4 | `Transport.Send` respeita context | ❌ | `core/infrastructure/ipc/uds/listener.go:229` | `Send` ignora o contexto (timeout não funcional) | Adicionar `SetWriteDeadline` baseado no contexto |
| 7.5 | Dispatch timeout cobre pipeline inteiro | ⚠️ | `core/application/gateway/dispatcher.go:493` | Timeout só cobre IPC, não JWT/validation | Envolver todo o `Dispatch` em `context.WithTimeout` |

---

## 8. CIRCUIT BREAKER

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 8.1 | Circuit breaker por worker | ✅ | `core/domain/circuit/breaker.go` | Nenhum | — |
| 8.2 | Half-open probing | ✅ | `core/domain/circuit/breaker.go:133` | Nenhum | — |
| 8.3 | Configurable failure threshold | ✅ | `core/domain/circuit/breaker.go:17` | Nenhum | — |
| 8.4 | Chave usa route pattern (não req.Path) | ❌ | `core/application/gateway/dispatcher.go:256` | Usa `req.Path` (memory leak com path params) | Trocar para `route.Path` (ver Issue #122) |

---

## 9. SHUTDOWN E WORKER LIFECYCLE

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 9.1 | Ordem correta: HTTP → Drain → Stop → Transport | ❌ | `core/cmd/vyx/main.go:1021` | Transport fechado antes dos workers | Reordenar shutdown (Issue #123) |
| 9.2 | Grace period para workers terminarem | ⚠️ | `core/application/lifecycle/service.go` | `Stop` chamado antes de `Drain` | Reordenar RestartWorker (Issue #125) |
| 9.3 | SIGTERM → wait → SIGKILL (Unix) | ✅ | `core/infrastructure/process/manager_unix.go` | Nenhum | — |
| 9.4 | CTRL_BREAK_EVENT (Windows) | ❌ | `core/infrastructure/process/manager_windows.go:30` | Envia SIGKILL direto | Implementar `GenerateConsoleCtrlEvent` (Issue #120) |
| 9.5 | `cmd.Wait()` chamado uma vez | ❌ | `core/infrastructure/process/manager.go:94+120` | Chamado duas vezes (data race) | Unificar goroutines (Issue #119) |

---

## 10. PANIC RECOVERY

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 10.1 | `recover()` em TODAS goroutines de produção | ❌ | 43 goroutines sem recover | Zero `recover()` em toda a base | Adicionar helper `vyx.Recover()` + aplicar em todos os pontos (Issue #118) |

---

## 11. IPC E WORKER COMUNICAÇÃO

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 11.1 | Constantes IPC consistentes entre runtimes | ❌ | `packages/python/vyx/ipc.py:12` | Python SDK tem constantes diferentes do Go core | Corrigir constantes Python (Issue #116) |
| 11.2 | SecurityContext propagado para workers | ❌ | — | Não existe contexto de segurança propagado | Implementar SecurityContextToken (ADR-001) |
| 11.3 | Python SDK: path params no roteamento | ❌ | `packages/python/vyx/dispatch.py:53` | Matching exato sem suporte a `:id` | Implementar path param matching (Issue #130) |
| 11.4 | Python SDK: `read_exact` | ❌ | `packages/python/vyx/ipc.py:68` | Leitura única sem loop | Implementar `read_exact` (Issue #142) |
| 11.5 | Python SDK: `MaxPayloadSize` check | ❌ | `packages/python/vyx/ipc.py:77` | Sem limite de payload | Adicionar check (Issue #143) |
| 11.6 | Node.js SDK: roteamento O(n) → trie | ❌ | `packages/worker/src/dispatch.ts:74` | Linear scan | Implementar trie routing (Issue #131) |
| 11.7 | Node.js SDK: remove `setInterval` vazio | ❌ | `packages/worker/src/dispatch.ts:254` | Timer leak | Usar `socket.setKeepAlive(true)` (Issue #141) |
| 11.8 | Header sanitization (CRLF) | ❌ | `core/infrastructure/gateway/server.go:215,247` | Headers copiados sem sanitização | Adicionar CRLF stripping (Issue #124) |

---

## 12. LOGS E OBSERVABILIDADE

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 12.1 | Log redaction (password, token, secret, credit_card, ssn) | ❌ | — | Logs podem conter dados sensíveis | Implementar `log.Redact()` hook |
| 12.2 | Security event taxonomy | ❌ | — | Não existe taxonomy de eventos de segurança | Implementar `security.Event` struct |
| 12.3 | Audit trail (authN, authZ, schema violation) | ❌ | — | Apenas logs genéricos (zap) | Implementar audit logger separado |
| 12.4 | Correlation ID propagation | ✅ | `core/application/gateway/dispatcher.go` (correlationID) | Nenhum | — |

---

## 13. CONFIGURAÇÃO

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 13.1 | Startup fail em configuração insegura | ❌ | `core/cmd/vyx/main.go:724` | JWT secret vazio não falha startup | Adicionar validações de configuração no startup (Issue #117) |
| 13.2 | CORS fechado por padrão | ❌ | `core/infrastructure/gateway/server.go` | Não existe CORS | Implementar CORS middleware (Issue #128) |
| 13.3 | Content-Type validation (só JSON) | ❌ | `core/infrastructure/gateway/server.go` | Aceita qualquer Content-Type | Adicionar validação |
| 13.4 | Method validation (allowlist) | ❌ | `core/infrastructure/gateway/server.go` | Mux aceita qualquer método | Validar método contra allowlist |
| 13.5 | Route inventory validation (build step) | ❌ | — | Rota sem schema compila sem erro | Adicionar validação no `vyx build` |

---

## 14. PERFORMANCE (HOT PATHS)

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 14.1 | Usar msgpack em vez de json no IPC | ❌ | `core/application/gateway/dispatcher.go:591` | Usa `json.Marshal(map[string]any{})` | Trocar para `msgpack.Marshal(struct{})` (Issue #121) |
| 14.2 | routeKey calculado uma vez | ❌ | `core/application/gateway/dispatcher.go` (múltiplas linhas) | Calculado 3-4x por request | Consolidar em `LifecycleContext` (Issue #127) |
| 14.3 | Goroutine fire-and-forget em selectWorker | ❌ | `core/application/gateway/dispatcher.go:560` | Goroutine leak sob carga | Remover, usar `defer` no chamador (Issue #126) |
| 14.4 | RouteMap.Lookup sem alocação de params | ❌ | `core/domain/gateway/route.go:99` | Map alocado mesmo sem params | Lazy-init (Issue #132) |
| 14.5 | `sync.Pool` para buffers de frame | ❌ | `core/infrastructure/ipc/framing/framing.go:28` | 2 allocs por mensagem | Adicionar `sync.Pool` (Issue #133) |
| 14.6 | Lock contention no pool (IncrementActiveReqs) | ❌ | `core/domain/pool/pool.go:128` | RWMutex desnecessário | Usar `sync.Map` (Issue #134) |
| 14.7 | Lock contention no drainer | ❌ | `core/application/lifecycle/drainer.go:28` | Mutex global | Usar `sync.Map[string]*WaitGroup` (Issue #135) |

---

## 15. SSRF PROTECTION

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 15.1 | Bloquear RFC 1918 | ❌ | — | Client HTTP padrão do Go sem proteção | Implementar `vyx.HTTPClient` com SSRF protection (ADR-005) |
| 15.2 | Bloquear localhost | ❌ | — | Idem | Idem |
| 15.3 | Bloquear metadata cloud | ❌ | — | Idem | Idem |
| 15.4 | Apenas HTTPS | ❌ | — | Idem | Idem |
| 15.5 | Timeout obrigatório | ❌ | — | Idem | Idem |

---

## 16. WORKER ISOLAMENTO

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 16.1 | `Setpgid` (isolamento de processo) | ✅ | `core/infrastructure/process/manager_unix.go` | Nenhum | — |
| 16.2 | SIGTERM + WaitDelay + SIGKILL | ✅ | `core/infrastructure/process/manager.go` | Nenhum | — |
| 16.3 | Workers sem comunicação direta | ✅ | Arquitetura (toda comunicação passa pelo orquestrador) | Nenhum | — |

---

## 17. ERROS E RESPOSTAS

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 17.1 | Erro padronizado com código VYX-* | ❌ | `core/infrastructure/gateway/server.go:292` | Erros genéricos sem código | Implementar formato padronizado de erro |
| 17.2 | Mensagem genérica para cliente | ⚠️ | `core/infrastructure/gateway/server.go:292` | `err.Error()` vaza detalhes internos | Mapear erros para mensagens seguras |
| 17.3 | Detalhes internos só em logs | ⚠️ | `core/infrastructure/gateway/server.go:293` | Loga erro mas também retorna detalhe | Separar mensagem de cliente vs log |

---

## 18. DEPENDÊNCIAS

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 18.1 | Go toolchain 1.25+ (patches de segurança) | ❌ | `core/go.mod` | `go 1.25.0` (22 vulns conhecidas) | Atualizar para `go 1.25.10` (Issue #152) |
| 18.2 | DevDependencies sem duplicatas | ❌ | `packages/worker/package.json` | Duplicatas de eslint, vitest | Limpar package.json (Issue #154) |
| 18.3 | SBOM generation | ❌ | — | Não existe | Adicionar geração de SBOM no CI (Fase 5) |

---

## 19. EXEMPLOS (SEGURANÇA)

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 19.1 | Exemplos sem credenciais hardcoded | ❌ | `examples/dashboard/workers/go/main.go:144` | admin/admin123, user/password | Substituir por env vars + aviso (Issue #153) |
| 19.2 | Mock token não expõe dados internos | ❌ | `examples/dashboard/workers/go/main.go:266` | Token plaintext com role/ID | Usar JWT real ou aviso claro |

---

## 20. TESTES

| # | Requisito (Arquitetura) | Status | Arquivo Atual | Gap | Patch |
|---|------------------------|--------|---------------|-----|-------|
| 20.1 | Benchmark tests para hot paths | ❌ | — | Zero benchmarks | Adicionar benchmarks (Issue #132) |
| 20.2 | Testes de conformidade cross-runtime | ❌ | — | Não existe | Implementar test suite (Fase 5) |
| 20.3 | Fuzzing de parsers | ❌ | — | Não existe | Adicionar fuzzing pipeline (Fase 4) |
| 20.4 | Golden tests de autorização | ❌ | — | Não existe | Implementar golden tests (Fase 5) |

---

## Resumo

| Status | Quantidade | % |
|--------|-----------|---|
| ✅ **Implementado** | 15 | 13% |
| ⚠️ **Parcial** | 10 | 9% |
| ❌ **Não implementado** | 91 | 78% |
| **Total** | **116** | **100%** |

### Por Área

| Área | Total | ✅ | ⚠️ | ❌ |
|------|-------|----|----|----|
| Autenticação JWT | 10 | 2 | 2 | 6 |
| Validação (entrada) | 10 | 2 | 5 | 3 |
| Validação (saída) | 4 | 0 | 0 | 4 |
| Autorização | 7 | 1 | 0 | 6 |
| Security headers | 7 | 5 | 0 | 2 |
| Rate limiting | 4 | 1 | 1 | 2 |
| Timeouts | 5 | 2 | 2 | 1 |
| Circuit breaker | 4 | 3 | 0 | 1 |
| Shutdown | 5 | 1 | 1 | 3 |
| Panic recovery | 1 | 0 | 0 | 1 |
| IPC | 8 | 0 | 0 | 8 |
| Logs/Observabilidade | 4 | 1 | 0 | 3 |
| Configuração | 5 | 0 | 0 | 5 |
| Performance | 7 | 0 | 0 | 7 |
| SSRF | 5 | 0 | 0 | 5 |
| Worker isolamento | 3 | 3 | 0 | 0 |
| Erros | 3 | 0 | 2 | 1 |
| Dependências | 3 | 0 | 0 | 3 |
| Exemplos | 2 | 0 | 0 | 2 |
| Testes | 4 | 0 | 0 | 4 |
| **Total** | **116** | **15** | **10** | **91** |
