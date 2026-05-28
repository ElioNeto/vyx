# Arquitetura de Segurança do VYX Framework

**Autor:** Arquiteto Principal de Segurança e Plataforma
**Data:** 28 de Maio de 2026
**Versão:** 1.0
**Classificação:** Interno — Arquitetura de Referência

---

## Premissas Arquiteturais

1. **Orquestrador Go é o ponto central de enforcement.** Todo tráfego de entrada (HTTP, WebSocket, gRPC) passa por ele antes de qualquer worker.
2. **Workers são não-confiáveis por padrão.** Um worker comprometido não deve comprometer outros workers nem o orquestrador.
3. **Frontend React é apenas um cliente.** Toda lógica de segurança vive no backend.
4. **A framework é multi-tenant por design.** Isolamento entre aplicações é responsabilidade do core.
5. **Runtimes são heterogêneos.** Go, Python e Node.js têm modelos de concorrência, tipos e capacidades diferentes. A segurança não pode depender de comportamento consistente entre eles — deve ser enforced no orquestrador.

---

## 1. VISÃO DE ARQUITETURA — Segurança em Camadas

### 1.1 Diagrama de Camadas

```
┌─────────────────────────────────────────────────────────────────────┐
│                        INTERNET / CLIENTE                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │               CAMADA 1: EDGE / TLS TERMINATION                │   │
│  │  • TLS 1.3 mínimo                                             │   │
│  │  • HSTS (max-age=63072000; includeSubDomains)                 │   │
│  │  • Certificate pinning (opcional)                             │   │
│  │  • HTTP/2 ou HTTP/3 (não HTTP/1.1 plaintext)                  │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                               │                                      │
│                               ▼                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │               CAMADA 2: WEB APPLICATION FIREWALL               │   │
│  │  • Request size limits (1MB default)                          │   │
│  │  • Content-Type validation (rejeitar tipo inesperado)          │   │
│  │  • Method validation (allowlist: GET,POST,PUT,PATCH,DELETE)    │   │
│  │  • Path normalization (rejeitar .., //, encoded chars)         │   │
│  │  • Header sanitization (CRLF stripping)                        │   │
│  │  • Rate limiting por IP (sliding window)                       │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                               │                                      │
│                               ▼                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │               CAMADA 3: AUTENTICAÇÃO (ORQUESTRADOR)            │   │
│  │  • JWT validation (assinatura, exp, nbf, iss, aud)            │   │
│  │  • Session token validation (revogação, rotação)               │   │
│  │  • Service-to-service mTLS                                    │   │
│  │  • MFA verification (opcional)                                │   │
│  │  • Reauthentication for critical actions                       │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                               │                                      │
│                               ▼                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │               CAMADA 4: VALIDAÇÃO (ORQUESTRADOR)               │   │
│  │  • JSON Schema validation (body, query, headers, params)       │   │
│  │  • Rejeição de campos extras (additionalProperties: false)     │   │
│  │  • Type coercion prevention                                    │   │
│  │  • Depth/cardinality limits                                    │   │
│  │  • Payload size enforcement                                    │   │
│  │  • Content negotiation validation                              │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                               │                                      │
│                               ▼                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │               CAMADA 5: AUTORIZAÇÃO (ORQUESTRADOR)            │   │
│  │  • Route-level: @Auth(roles: ["admin"])                      │   │
│  │  • Scope validation                                           │   │
│  │  • Tenant isolation check                                     │   │
│  │  • Rate limit por identidade                                  │   │
│  │  • Quota check                                                │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                               │                                      │
│                               ▼                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │               CAMADA 6: ROTEAMENTO SEGURO                      │   │
│  │  • Route map validation (sem rotas órfãs)                     │   │
│  │  • Circuit breaker por worker                                 │   │
│  │  • Worker isolation (setpgid, namespace)                      │   │
│  │  • Timeout enforcement (hard deadline)                        │   │
│  │  • Payload forwarding sanitizado                              │   │
│  │  • Response header sanitization (CRLF, Content-Type)          │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                               │                                      │
│                               ▼                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │               CAMADA 7: WORKER (GO / NODE / PYTHON)            │   │
│  │  • Recebe request já validado e autenticado                   │   │
│  │  • Contexto de segurança propagado (não adulterável)           │   │
│  │  • Objeto de request tipado (nunca raw)                        │   │
│  │  • Output validation (schema de resposta)                      │   │
│  │  • Response envelope padronizado                               │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                               │                                      │
│                               ▼                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │               CAMADA 8: RESPOSTA (ORQUESTRADOR)                │   │
│  │  • Security headers injection                                 │   │
│  │  • CORS validation                                            │   │
│  │  • Content-Type enforcement                                   │   │
│  │  • Response size limits                                       │   │
│  │  • Correlation ID injection                                   │   │
│  │  • Error sanitization (nunca stack trace)                     │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                               │                                      │
│                               ▼                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │               CAMADA 9: OBSERVABILIDADE                        │   │
│  │  • Security event log (estruturado, imutável)                 │   │
│  │  • Audit trail (authn, authz, schema violation, abuse)        │   │
│  │  • Metrics de segurança (authn failure rate, 4xx/5xx)         │   │
│  │  • Alerting thresholds                                        │   │
│  │  • Forensic data (correlation ID, timestamp, source IP)       │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.2 Matriz de Responsabilidades

| Camada | O Que Fica no Orquestrador Go | O Que Fica no Worker | O Que Fica no App |
|--------|------------------------------|---------------------|-------------------|
| **TLS/Edge** | Terminação TLS, HSTS, ALPN | Nada | Nada |
| **WAF** | Size limits, method validation, path normalization, header sanitization | Nada | Nada |
| **Autenticação** | JWT validation, session validation, mTLS, MFA, reauthentication | Recebe identidade já validada | Plugins de auth provider (OAuth, SAML, LDAP) |
| **Validação** | JSON Schema validation, type coercion prevention, depth limits | Nunca — recebe dados já validados | Schema definitions (arquivos .schema.json) |
| **Autorização** | Route-level, scope validation, tenant isolation, rate limit por identidade | Pode fazer object-level authz com contexto propagado | Policies declarativas (arquivos .policy.json) |
| **Roteamento** | Route map, circuit breaker, worker isolation, timeouts | Nada | Anotações @Route, @Auth, @Validate |
| **Resposta** | Security headers, CORS, error sanitization, correlation ID | Content-Type, body (já validado) | Nada |

### 1.3 Risco de Divergência entre Runtimes

O maior risco de uma arquitetura poliglota é que **cada runtime implemente a mesma regra de forma diferente**. Exemplos concretos:

| Runtime | Risco de Divergência | Mitigação |
|---------|---------------------|-----------|
| **Go** vs **Python** | `json.Unmarshal` vs `json.loads` tratam `null` vs `0` vs `""` de forma diferente | Orquestrador normaliza antes de enviar ao worker |
| **Node.js** vs **Go** | Number em JS é IEEE 754 (perde precisão > 2^53), Go tem int64 | Schema validation no orquestrador rejeita números fora do range do worker |
| **Python** vs **Node.js** | Timezone handling diferente (datetime vs Date) | Orquestrador converte para RFC 3339 antes de enviar |
| **Todos** | Path parameter encoding | Orquestrador normaliza path params (decodifica URL, valida UTF-8) |

**Regra de ouro:** O orquestrador Go é a fonte única da verdade para validação. Workers recebem dados já normalizados, tipados e validados. Qualquer runtime que precise fazer validação própria está violando o contrato.

---

## 2. PRINCÍPIOS DE SEGURANÇA DO CORE

### P1. Secure by Default

**Motivo:** A configuração insegura não deve ser uma opção viável. Se o desenvolvedor não configurar algo explicitamente, a framework deve escolher a opção mais segura.

**Onde se aplica:** Toda configuração da framework.

**Exemplo de enforcement:**
- CORS: por padrão, `Access-Control-Allow-Origin` não é enviado (mesma origem apenas). Se o desenvolvedor configurar `cors.origins: ["*"]`, a framework loga um WARN e exige confirmação explícita.
- Cookies de sessão: `SameSite=Lax`, `Secure=true`, `HttpOnly=true` por padrão. Não há flag para desabilitar `HttpOnly`.

### P2. Deny by Default

**Motivo:** Tudo que não é explicitamente permitido deve ser negado. Isso inclui rotas, métodos HTTP, campos em schemas, origens CORS, e headers.

**Onde se aplica:** Roteamento, validação de schema, CORS, métodos HTTP.

**Exemplo de enforcement:**
- Se uma rota não está no `route_map.json`, o orquestrador retorna 404 sem consultar nenhum worker.
- Se um schema de validação define `additionalProperties: false` (que é o default da framework), qualquer campo extra no body é rejeitado com 422.
- Se um método HTTP não está na allowlist (GET, POST, PUT, PATCH, DELETE), retorna 405.

### P3. Least Privilege

**Motivo:** Cada worker deve ter exatamente as permissões necessárias para operar, nada mais.

**Onde se aplica:** Workers, tokens de serviço, acesso a recursos.

**Exemplo de enforcement:**
- Workers rodam com `setpgid` e `syscall.Kill(-pgid, ...)` para isolamento de processo.
- Tokens de serviço para workers têm escopo restrito ao worker ID específico.
- Workers não têm acesso à rede externa por padrão (egress filtering).

### P4. Fail Securely

**Motivo:** Quando algo falha (banco, worker, timeout), a framework deve falhar de forma segura — negar acesso, não conceder.

**Onde se aplica:** Circuit breaker, timeout, worker crash, erro de validação.

**Exemplo de enforcement:**
- Circuit breaker open → retorna 503 com `Retry-After: 30` (nega, não concede).
- Timeout de worker → retorna 504 Gateway Timeout (não retorna resposta parcial).
- Worker crash → rota marcada como indisponível até health check passar.
- Erro de validação de schema → retorna 422 com detalhes específicos (nunca executa o handler).

### P5. Defense in Depth

**Motivo:** Nenhuma camada isolada é suficiente. Múltiplas camadas de controle garantem que a falha de uma não compromete o sistema.

**Onde se aplica:** Todas as camadas.

**Exemplo de enforcement:**
- Mesmo que o orquestrador valide JWT, o worker também recebe o contexto de segurança e pode verificar claims.
- Mesmo que o orquestrador valide schema, o worker deve usar tipos seguros (não `string` para tudo).
- Mesmo que o orquestrador sanitize headers, o worker não deve confiar neles cegamente.

### P6. Zero Trust entre Serviços

**Motivo:** Comunicação entre orquestrador e workers não é inerentemente confiável. Um worker comprometido não deve poder atacar outros workers ou o orquestrador.

**Onde se aplica:** IPC entre orquestrador e workers.

**Exemplo de enforcement:**
- IPC via UDS com validação de tipo de mensagem e payload.
- Workers não podem se comunicar entre si diretamente (toda comunicação passa pelo orquestrador).
- Respostas de workers são validadas (tipo, tamanho, encoding) antes de serem enviadas ao cliente.

### P7. Separação entre Autenticação, Autorização, Validação e Serialização

**Motivo:** Misturar essas responsabilidades leva a bypass e inconsistências. Cada uma deve ser uma camada independente e testável.

**Onde se aplica:** Pipeline de dispatch.

**Exemplo de enforcement:**
- Autenticação: camada 3 (antes de qualquer roteamento).
- Autorização: camada 5 (depois da autenticação, antes do worker).
- Validação: camada 4 (depois da autenticação, antes da autorização).
- Serialização: camada 8 (depois da resposta do worker).

### P8. Contratos Explícitos de Entrada e Saída

**Motivo:** Sem contratos explícitos, qualquer mudança em um runtime pode quebrar a segurança de todos.

**Onde se aplica:** Schemas de validação, definições de rota, respostas de worker.

**Exemplo de enforcement:**
- Toda rota tem um schema de entrada (`@Validate`) e um schema de saída (`@Response`).
- O schema de saída é validado pelo orquestrador antes de enviar ao cliente.
- Se um worker retorna campos não declarados no schema de saída, eles são removidos.

### P9. Minimização de Exposição de Dados

**Motivo:** Quanto menos dados trafegam, menor a superfície de ataque.

**Onde se aplica:** Respostas de API, logs, mensagens de erro.

**Exemplo de enforcement:**
- Schema de saída explicito (allowlist de campos retornáveis).
- Logs nunca contêm tokens, senhas, ou dados pessoais.
- Mensagens de erro para o cliente são genéricas (`"invalid credentials"`, não `"password too short"`).

### P10. Proteção contra Configurações Inseguras

**Motivo:** Configuração é código. Configurações inseguras são vulnerabilidades.

**Onde se aplica:** `vyx.yaml`, schemas, policies.

**Exemplo de enforcement:**
- `vyx build` falha se encontrar configurações inseguras (CORS aberto, secret curto, TLS desabilitado).
- Validação de schema de configuração no startup.
- Feature flags de segurança (ex: `security.enforce_cors`) que não podem ser desligadas em produção.

---

## 3. MODELO DE SEGURANÇA UNIFICADO

### 3.1 Contrato de Contexto de Segurança

Toda requisição, em qualquer runtime, carrega este contexto:

```json
{
  "security_context": {
    "identity": {
      "type": "user|service|anonymous",
      "id": "uuid-do-usuario",
      "username": "joao.silva",
      "email": "joao@exemplo.com",
      "authenticated_at": "2026-05-28T15:00:00Z",
      "authentication_method": "jwt|session|mtls|oauth2"
    },
    "authorization": {
      "roles": ["admin", "editor"],
      "permissions": ["post:create", "post:edit", "user:read"],
      "scopes": ["read", "write"],
      "tenant_id": "tenant-abc-123",
      "service_id": "go:api",
      "token_id": "jti-abc-456"
    },
    "request": {
      "correlation_id": "corr-abc-789",
      "client_ip": "200.201.202.203",
      "user_agent": "Mozilla/5.0...",
      "geo": "BR",
      "idempotency_key": "idem-abc-012"
    },
    "audit": {
      "request_id": "req-abc-345",
      "route": "POST /api/orders",
      "resource_type": "order",
      "resource_id": "order-abc-678",
      "action": "create"
    }
  }
}
```

### 3.2 Criação e Propagação

```
Cliente → HTTP Request
  │
  ▼
Orquestrador Go
  │
  ├── 1. Autenticação → extrai identity + authorization do token/session
  ├── 2. Validação → request validado contra schema
  ├── 3. Autorização → roles/permissions verificadas
  ├── 4. Gera context_id → UUID único para o contexto de segurança
  │
  ├── Contexto serializado como JSON assinado (HMAC) + nonce
  │   ↳ Assinatura impede adulteração pelo worker
  │   ↳ Nonce impede replay
  │
  ├── IPC → Worker (Go/Python/Node)
  │   └── Worker recebe:
  │       ├── request payload (já validado)
  │       ├── security_context (assinado + nonce)
  │       └── headers sanitizados (sem Authorization original)
  │
  └── Worker processa → retorna resposta
      └── Orquestrador:
          ├── valida resposta contra schema de saída
          ├── injeta security headers
          └── retorna ao cliente
```

### 3.3 Modelo de Confiança do SecurityContext

O SecurityContext opera em **dois níveis de confiança** que precisam ser claramente separados:

#### Nível 1: Contexto Opaco (Transporte)
O `SecurityContextToken` é uma string assinada que o worker **não pode validar**, mas **pode usar** para ler claims. A assinatura HMAC protege a integridade entre orquestrador e worker.

```
SecurityContextToken = base64url({
  "context": { ... dados confiáveis ... },
  "nonce": uuid-v4,
  "created_at": timestamp,
  "ttl_seconds": 120,
  "signature": HMAC-SHA256(context + nonce + created_at, server_secret)
})
```

- **Worker NÃO tem a chave** → não pode validar a assinatura
- **Worker NÃO pode adulterar** → se modificar, a assinatura quebra na validação do retorno
- **Orquestrador valida no retorno** → quando o worker devolve o token na resposta, o orquestrador verifica assinatura + nonce + TTL

#### Nível 2: Claims Utilizáveis (Consumo)
O worker decodifica o payload JSON e obtém claims que ele PODE usar para:

| ✅ Pode Usar Para | ❌ Não Pode Usar Para |
|------------------|----------------------|
| Identificar o usuário (`user_id`) em queries | Decidir autorização de segurança (object-level authz) |
| Contexto de logging (`correlation_id`) | Bypassar verificações de role/permission |
| Cache key prefix (`tenant_id`) | Modificar claims para obter privilégios |
| Adaptar comportamento de UI/dados | Confiar cegamente para operações sensíveis |

#### Modelo de Confiança Explícito

```
ORQUESTRADOR:
  ├── CRIA o SecurityContext com dados CONFIÁVEIS (validou JWT/sessão)
  ├── ASSINA com HMAC (prova de integridade)
  └── CONFIA no worker ATÉ PROVA EM CONTRÁRIO

WORKER:
  ├── RECEBE claims LEGÍVEIS (JSON decodificado)
  ├── USA claims para OPERAÇÕES DE NEGÓCIO (queries, cache, log)
  ├── CONFIA no orquestrador para AUTENTICAÇÃO (não verifica assinatura)
  └── NÃO USA claims para DECISÕES DE SEGURANÇA (object-level authz vai ao PDP)

ORQUESTRADOR (no retorno):
  ├── VALIDA assinatura + nonce + TTL
  └── Se inválido → 502 + log de segurança (worker comprometido detectado)
```

#### Fluxo Completo

```
1. Orquestrador autentica usuário → obtém claims
2. Orquestrador monta SecurityContext + assina HMAC
3. Envia para worker: { request, security_context_token (opaco) }
4. Worker decodifica security_context_token → obtém claims LEGÍVEIS
5. Worker usa claims para lógica de negócio (não para authz)
6. Worker retorna: { response, security_context_token (inalterado) }
7. Orquestrador valida security_context_token + assinatura
8. Se OK → processa resposta. Se não → 502 + alerta.
```

#### Proteção contra Replay

O `nonce` (UUID) + `created_at` + `ttl_seconds` previnem que um token interceptado seja reutilizado:

- Nonce único por requisição (bloom filter de nonces recentes)
- TTL de 120 segundos (mesmo que o dispatch timeout máximo)
- Após TTL expirar, nonce é removido do bloom filter
- Se nonce repetido dentro do TTL → 502 Bad Gateway + log crítico

### 3.4 Consumo Consistente entre Runtimes

Cada runtime SDK expõe um objeto `SecurityContext` idêntico:

| Método | Go | Python | Node.js |
|--------|----|--------|---------|
| Acessar user ID | `ctx.UserID()` | `ctx.user_id` | `ctx.userId` |
| Verificar role | `ctx.HasRole("admin")` | `ctx.has_role("admin")` | `ctx.hasRole("admin")` |
| Verificar permission | `ctx.HasPermission("post:create")` | `ctx.has_permission("post:create")` | `ctx.hasPermission("post:create")` |
| Correlation ID | `ctx.CorrelationID()` | `ctx.correlation_id` | `ctx.correlationId` |
| Tenant ID | `ctx.TenantID()` | `ctx.tenant_id` | `ctx.tenantId` |

**Importante:** Nomes de métodos e propriedades são diferentes entre linguagens (seguem convenções locais), mas o **contrato semântico é idêntico**. Testes de conformidade validam isso.

---

## 4. VALIDAÇÃO E NORMALIZAÇÃO

### 4.1 Schema Canônico Compartilhado

A framework usa **JSON Schema (draft-07+) como formato canônico** para definição de schemas de validação. Motivos:

1. **Linguagem-independente** — não favorece nenhum runtime.
2. **Ecossistema maduro** — validadores em Go, Python, Node.js.
3. **Suporte a `additionalProperties: false`** — essencial para prevenir mass assignment.
4. **Suporte a formatos** (`email`, `date-time`, `uri`, `uuid`).
5. **Extensível** — `$vocabulary` para extensões customizadas.

Os schemas ficam em `schemas/*.json` e são compilados pelo orquestrador no startup.

### 4.2 Validação por Camada

| Camada | O Que Valida | Schema | Local |
|--------|-------------|--------|-------|
| **Path params** | Tipo, formato, tamanho | Definição na anotação `@Route(POST /api/orders/:id{uuid})` | Anotação da rota |
| **Query params** | Tipo, required/optional, enum, tamanho | Schema query definido em schema.json | `schemas/*.json` |
| **Headers** | Content-Type, Accept, Authorization (formato) | Validação built-in do core | Código do core |
| **Body** | Schema completo com `additionalProperties: false` | Schema referenciado por `@Validate(nome)` | `schemas/*.json` |
| **Cookies** | Formato, HttpOnly, Secure (validado pelo core) | Validação built-in do core | Código do core |
| **Resposta** | Schema de saída (tipo, campos, tamanho) | Schema referenciado por `@Response(nome)` | `schemas/*.json` |

### 4.3 Regras de Validação Obrigatórias

```
TODO ROTA COM @Validate DEVE TER:
  ✅ additionalProperties: false
  ✅ Limite de profundidade: maxDepth: 10 (default)
  ✅ Limite de cardinalidade: maxItems/maxProperties configurado
  ✅ Tipos explícitos (não usar type: ["string", "number"] — ambíguo)
  ✅ Formato para strings (email, date-time, uri, uuid conforme aplicável)
  ✅ Limite de tamanho para strings (minLength/maxLength)
  ✅ Limite de valor para números (minimum/maximum)

TODO SCHEMA DE RESPOSTA DEVE TER:
  ✅ additionalProperties: false
  ✅ Campos explícitos (allowlist de campos retornáveis)
  ✅ maximum para arrays (evitar vazamento de dados)
```

### 4.4 Proteção contra Mass Assignment

O schema de body SEMPRE tem `additionalProperties: false`. O orquestrador rejeita **qualquer campo extra** antes de enviar ao worker.

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["name", "email"],
  "additionalProperties": false,
  "properties": {
    "name": { "type": "string", "minLength": 2, "maxLength": 100 },
    "email": { "type": "string", "format": "email", "maxLength": 254 }
  }
}
```

Se o cliente enviar `{"name": "João", "email": "joao@ex.com", "role": "admin"}`, o orquestrador rejeita com 422:
```json
{
  "error": "validation_error",
  "code": "VYX-VAL-004",
  "detail": "Campo 'role' não permitido neste schema"
}
```

### 4.5 Normalização de Tipos entre Linguagens

O orquestrador normaliza tipos ANTES de enviar ao worker:

| Tipo JSON | Normalização | Go | Python | Node.js |
|-----------|-------------|----|--------|---------|
| `string` | Valida UTF-8, trimmed | `string` | `str` | `string` |
| `integer` | Range check (int64) | `int64` | `int` (Python 3 = ilimitado) | `number` (perde > 2^53) |
| `number` | Decimal como string (evitar IEEE 754) | `string` + lib decimal | `Decimal` | `string` + lib decimal |
| `boolean` | `true`/`false` apenas (não `"true"`) | `bool` | `bool` | `boolean` |
| `null` | Rejeitado por padrão (exceto nullable explícito) | `nil` não aceito | `None` não aceito | `null` não aceito |
| `date-time` | RFC 3339, UTC obrigatório | `time.Time` | `datetime` (timezone-aware) | `Date` (ISO string) |
| `array` | Limite de cardinalidade | `[]T` | `list[T]` | `T[]` |
| `object` | `additionalProperties: false` | `struct` | `dict` tipado | `interface` tipado |

### 4.6 Validação de Saída

A resposta do worker é validada contra um schema de saída antes de ser enviada ao cliente:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["id", "name", "created_at"],
  "additionalProperties": false,
  "properties": {
    "id": { "type": "string", "format": "uuid" },
    "name": { "type": "string", "maxLength": 100 },
    "created_at": { "type": "string", "format": "date-time" },
    "updated_at": { "type": "string", "format": "date-time" }
  }
}
```

Se o worker retornar um campo extra (`"internal_note": "segredo"`), ele é removido pelo orquestrador (não chega ao cliente).

---

## 5. AUTENTICAÇÃO

### 5.1 Arquitetura do Subsistema

```
┌─────────────────────────────────────────────┐
│           ORQUESTRADOR GO                    │
│                                              │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐  │
│  │ JWT      │   │ Session  │   │ mTLS     │  │
│  │ Validator│   │ Validator│   │ Verifier │  │
│  └────┬─────┘   └────┬─────┘   └────┬─────┘  │
│       │              │              │         │
│       └──────┬───────┘              │         │
│              │                     │         │
│              ▼                     ▼         │
│  ┌─────────────────────────────────────┐     │
│  │      Identity Resolver              │     │
│  │  • Extrai identity do token         │     │
│  │  • Valida assinatura, exp, nbf     │     │
│  │  • Valida iss, aud                 │     │
│  │  • Popula SecurityContext           │     │
│  └──────────────┬──────────────────────┘     │
│                 │                             │
│                 ▼                             │
│  ┌─────────────────────────────────────┐     │
│  │      Identity Provider Interface     │     │
│  │  • Plugável: OAuth, SAML, LDAP,     │     │
│  │    OpenID Connect, custom           │     │
│  └─────────────────────────────────────┘     │
└─────────────────────────────────────────────┘
```

### 5.2 Fluxos de Autenticação

#### 5.2.1 Usuário Final (JWT Access + Refresh Token)

```
1. POST /api/auth/login → { username, password }
2. Orquestrador valida credentials (delega ao Identity Provider plugin)
3. Orquestrador gera:
   - Access Token (JWT, 15 min)
   - Refresh Token (opaco, 7 dias, armazenado no core)
4. Access Token contém:
   {
     "sub": "user-uuid",
     "roles": ["admin"],
     "permissions": ["post:create"],
     "tenant_id": "tenant-abc",
     "iss": "vyx://tenant-abc",
     "aud": ["vyx-api"],
     "exp": 1745856000,
     "iat": 1745855100,
     "jti": "unique-token-id"
   }
5. Cliente envia Access Token no header Authorization: Bearer <token>
6. Orquestrador valida em TODA requisição:
   - Assinatura HMAC-SHA256
   - exp (expiração)
   - nbf (not before)
   - iss (issuer = tenant ID)
   - aud (audience = serviço)
   - jti (não está na blacklist de revogação)
7. Refresh: POST /api/auth/refresh com Refresh Token opaco
   - Core valida, revoga old refresh, emite novo par
```

#### 5.2.2 Service-to-Service (mTLS + JWT)

```
1. Worker Go/Python/Node inicia conexão UDS com orquestrador
2. mTLS handshake com certificado de worker (assinado pela CA interna da framework)
3. Worker envia JWT de serviço:
   {
     "sub": "go:api",
     "type": "service",
     "scopes": ["read:products", "write:orders"],
     "iss": "vyx-service-ca",
     "aud": ["vyx-orchestrator"]
   }
4. Orquestrador valida:
   - Certificado mTLS (worker authentication)
   - JWT (autorização do serviço)
   - Escopo da operação solicitada
```

#### 5.2.3 Rotação de Chaves

```
1. O orquestrador mantém um key ring com múltiplas chaves HMAC
2. A chave atual é usada para assinar NOVOS tokens
3. Chaves anteriores são mantidas para VALIDAÇÃO de tokens existentes
4. Rotação automática a cada 24h ou manual via API /_vyx/security/rotate-keys
5. Revogação via blacklist de jti (em memória + Redis para resiliência)
```

### 5.3 O Que o Orquestrador Verifica Obrigatoriamente

| Verificação | Obrigatório? | O Que Acontece se Falhar |
|------------|-------------|-------------------------|
| Assinatura do token | ✅ Sim | 401 Unauthorized |
| `exp` (expiração) | ✅ Sim | 401 Unauthorized |
| `nbf` (not before) | ✅ Sim | 401 Unauthorized |
| `iss` (issuer) | ✅ Sim (match tenant) | 401 Unauthorized |
| `aud` (audience) | ✅ Sim (match serviço) | 401 Unauthorized |
| `jti` blacklist | ✅ Sim | 401 Unauthorized |
| MFA (se exigido) | ✅ Sim (por rota) | 403 Forbidden + desafio MFA |
| Reautenticação (ação crítica) | ✅ Sim (se configurado) | 403 Forbidden |

### 5.4 O Que é Plugável vs Nativo

| Componente | Nativo | Plugável | Exemplos de Plugin |
|-----------|--------|----------|-------------------|
| JWT validation | ✅ Sim | — | — |
| Session management | ✅ Sim | — | — |
| mTLS | ✅ Sim | — | — |
| Identity Provider | — | ✅ Sim | OAuth, SAML, LDAP, OpenID Connect, PostgreSQL, custom |
| MFA | — | ✅ Sim | TOTP, SMS, WebAuthn |
| Key rotation | ✅ Sim | ✅ Sim (KMS) | AWS KMS, HashiCorp Vault, Azure Key Vault |
| Token revocation | ✅ Sim | ✅ Sim (storage) | Redis, PostgreSQL, DynamoDB |

### 5.5 Prevenindo Implementações Frágeis

O orquestrador **não permite** que aplicações criem suas próprias verificações de autenticação. O fluxo é:

1. ✅ Orquestrador valida o token (obrigatório).
2. ✅ Orquestrador popula `SecurityContext` (obrigatório).
3. ✅ Orquestrador verifica `@Auth(roles: [...])` (obrigatório).
4. ❌ Worker não pode "pular" a autenticação.
5. ❌ Worker não pode sobrescrever `SecurityContext.user_id`.
6. ❌ App não pode criar rotas sem `@Auth` que aceitem usuários não autenticados (exige `roles: ["guest"]` explícito).

---

## 6. AUTORIZAÇÃO

### 6.1 Modelo de Autorização em Camadas

```
NÍVEL 1: Route-level Authorization (Orquestrador)
  └── @Auth(roles: ["admin"])
  └── @Scope("write:orders")
  └── Verificado ANTES de rotear para o worker

NÍVEL 2: Function-level Authorization (Worker)
  └── ctx.HasPermission("order:cancel")
  └── ctx.HasRole("superadmin")
  └── Verificado DENTRO do handler

NÍVEL 3: Object-level Authorization (Worker)
  └── ctx.CanAccess("order", orderId)
  └── Verifica se o usuário é dono do recurso
  └── Baseado em policy declarativa

NÍVEL 4: Property-level Authorization (Orquestrador)
  └── Schema de resposta com campos condicionais
  └── @Response(user, roles: ["admin"] → include "internal_note")
  └── Verificado na serialização da resposta

NÍVEL 5: Tenant Isolation (Orquestrador)
  └── ctx.TenantID() check automático em queries
  └── Cross-tenant data access bloqueado
```

### 6.2 Policy DSL

As policies são declaradas em arquivos `.policy.json`:

```json
{
  "policy": "order-access",
  "description": "Access control for orders",
  "rules": [
    {
      "effect": "allow",
      "actions": ["order:read", "order:list"],
      "resources": ["order:*"],
      "conditions": {
        "match": ["user.tenant_id", "resource.tenant_id"],
        "or": [
          {"user.role": "admin"},
          {"user.id": "resource.owner_id"}
        ]
      }
    },
    {
      "effect": "deny",
      "actions": ["order:delete"],
      "resources": ["order:*"],
      "conditions": {
        "user.role": {"neq": "superadmin"}
      }
    }
  ]
}
```

### 6.3 Fluxo de Avaliação

```
1. Request chega → Orquestrador identifica rota
2. Orquestrador carrega policy da rota (se houver)
3. Orquestrador avalia NÍVEL 1 (route-level):
   - roles do token contêm roles exigidas?
   - scopes do token contêm scopes exigidos?
   - Se não → 403 Forbidden
4. Request enviado ao worker com SecurityContext
5. Worker avalia NÍVEL 2 (function-level):
   - ctx.HasPermission("order:cancel")
   - Se não → retorna 403
6. Worker avalia NÍVEL 3 (object-level):
   - ctx.CanAccess("order", orderId)
   - Verifica ownership + tenant
   - Se não → retorna 403 (não 404 — evitar enumeração)
7. Worker retorna resposta
8. Orquestrador avalia NÍVEL 4 (property-level):
   - Aplica schema de resposta condicional
   - Remove campos que o usuário não pode ver
9. Orquestrador retorna resposta ao cliente
```

### 6.4 Prevenção de BOLA (Broken Object Level Authorization)

O core da framework **fornece um mecanismo nativo** para evitar BOLA:

1. **Contexto de tenant é automático:** `ctx.TenantID()` retorna o tenant do usuário autenticado. O worker DEVE usar isso em queries.
2. **Ownership check helper:** `ctx.CanAccessResource(resourceType, resourceID)` executa a policy declarada.
3. **Policy enforcement point:** O framework oferece um middleware de objeto que verifica ownership antes de qualquer operação.

```go
// Go worker — padrão obrigatório
func handleGetOrder(ctx context.Context, req *Request) (*Response, error) {
    orderID := req.Params["id"]
    
    // Verificação de ownership OBRIGATÓRIA
    if !ctx.CanAccessResource("order", orderID) {
        return nil, ErrForbidden  // 403 — sem revelar se o recurso existe
    }
    
    order := db.GetOrder(orderID)
    return &Response{Body: order}, nil
}
```

### 6.5 Prevenção de Mass Assignment (BOPLA)

O schema de body com `additionalProperties: false` no orquestrador já previne mass assignment em criação/atualização.

Para **propriedades condicionais** (ex: apenas admin pode definir `role`), usa-se schema dinâmico:

```json
{
  "schemas": {
    "create-user": {
      "base": "user-base",
      "admin_overrides": {
        "properties": {
          "role": { "type": "string", "enum": ["user", "admin"] }
        }
      }
    }
  }
}
```

O orquestrador seleciona o schema com base na role do usuário ANTES de validar.

### 6.6 Exigência de Allowlist Explícita

Para **cada rota de escrita** (POST, PUT, PATCH), o desenvolvedor DEVE declarar um schema de body com `additionalProperties: false` e a lista exaustiva de campos permitidos.

Para **cada rota de leitura** (GET), o desenvolvedor DEVE declarar um schema de resposta com a lista exaustiva de campos retornáveis.

Se um schema de resposta não for declarado, o orquestrador **rejeita a rota no startup** com erro:

```
ERRO: Rota GET /api/users não tem schema de resposta declarado.
Adicione @Response(user-response) à rota.
```

---

## 7. PROTEÇÃO CONTRA VULNERABILIDADES COMUNS

### Matriz Ameaça → Controle → Camada Responsável

| # | Ameaça | Risco | Controle Nativo da Framework | Camada | Prioridade |
|---|--------|-------|------------------------------|--------|-----------|
| 1 | **XSS** | Alta | Content-Type validation (só JSON), CSP headers, output encoding automático em SSR, `X-XSS-Protection` header | Orquestrador + React SSR | 🔴 Crítica |
| 2 | **CSRF** | Alta | `SameSite=Lax` em cookies de sessão, `Origin`/`Referer` validation em mutações, CSRF token nativo para forms | Orquestrador (built-in) | 🔴 Crítica |
| 3 | **SSRF** | Crítica | Client HTTP do worker bloqueia RFC 1918 por padrão, egress policy obrigatória, DNS validation, URL validation com allowlist de schemas (só https) | Orquestrador (enforcement) + Worker SDK | 🔴 Crítica |
| 4 | **SQL Injection** | Crítica | ORM obrigatório (sem raw SQL), query parameterization enforced, input validation no orquestrador | App (framework exige) | 🔴 Crítica |
| 5 | **Command Injection** | Crítica | `exec.Command` bloqueado por padrão, allowlist de comandos, validação de argumentos | Orquestrador + Worker SDK | 🔴 Crítica |
| 6 | **Path Traversal** | Alta | File operations bloqueadas para fora do diretório do projeto, allowlist de paths | Orquestrador + Worker SDK | 🔴 Crítica |
| 7 | **Insecure Deserialization** | Crítica | Só JSON + MsgPack permitidos, schema validation antes de deserializar, rejeição de tipos polimórficos | Orquestrador (built-in) | 🔴 Crítica |
| 8 | **XXE** | Média | XML parser desabilitado por padrão, JSON obrigatório | Orquestrador (built-in) | 🟠 Alta |
| 9 | **File Upload Inseguro** | Alta | File type validation (magic bytes), size limit, scan obrigatório, storage isolado por tenant | Orquestrador + Worker SDK | 🔴 Crítica |
| 10 | **Exposição Excessiva de Propriedades** | Alta | Schema de resposta obrigatório (`additionalProperties: false`), property-level authorization | Orquestrador (built-in) | 🟠 Alta |
| 11 | **Mass Assignment** | Alta | `additionalProperties: false` obrigatório em schemas de body | Orquestrador (built-in) | 🟠 Alta |
| 12 | **Rate Abuse** | Alta | Rate limiting por IP + identidade (sliding window), quotas por tenant | Orquestrador (built-in) | 🟠 Alta |
| 13 | **Brute Force** | Alta | Account lockout após N falhas, exponential backoff, rate limit por identidade | Orquestrador (built-in) | 🟠 Alta |
| 14 | **Credential Stuffing** | Alta | Rate limit por IP, CAPTCHA opcional, detectar padrões de falha em massa | Orquestrador + Plugin | 🟠 Alta |
| 15 | **Enumeração de Usuários** | Média | Mensagens de erro genéricas (`"invalid credentials"`), timing-safe comparison, mesmo response time para user exists/not-exists | Orquestrador (built-in) | 🟡 Média |
| 16 | **Vazamento de Segredos** | Crítica | Secrets scanner no CI, env vars obrigatórias (não arquivos .env), log redaction automático, `Authorization` header nunca logado | CI + Orquestrador | 🔴 Crítica |
| 17 | **Open Redirect** | Média | Validação de redirect URLs contra allowlist, `validate_redirect` helper | Worker SDK | 🟡 Média |
| 18 | **CORS Mal Configurado** | Alta | CORS fechado por padrão, validação de origem, `Access-Control-Allow-Origin` nunca `*` com credentials | Orquestrador (built-in) | 🟠 Alta |
| 19 | **Cache Poisoning** | Média | `Cache-Control` headers seguros por padrão, `Vary: Origin`, variar por tenant | Orquestrador | 🟡 Média |
| 20 | **Request Smuggling** | Média | HTTP/2 preferencial, HTTP/1.1 connection close em dev, validação de Content-Length vs Transfer-Encoding | Orquestrador (HTTP server) | 🟡 Média |
| 21 | **Consumo Excessivo de Recursos** | Alta | Payload size limit, depth/cardinality limits, timeout por rota, circuit breaker, rate limit | Orquestrador (built-in) | 🟠 Alta |
| 22 | **Dependências Inseguras** | Alta | `govulncheck` + `npm audit` + `pip-audit` no CI, fail build em CVE conhecida, SBOM generation | CI/CD | 🟠 Alta |
| 23 | **Rotas Órfãs** | Alta | Rota sem handler = erro no startup, inventory automático de rotas, rota esquecida = não acessível | Orquestrador (build step) | 🟠 Alta |

### 7.1 Detalhamento dos Controles Críticos

#### SSRF — Controle Nativo

O Client HTTP da framework (para workers) implementa proteção SSRF por padrão:

```go
// Go SDK — comportamento padrão do client HTTP
client := vyx.NewHTTPClient(vyx.HTTPClientConfig{
    BlockPrivateIPs: true,        // Bloqueia 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 127.0.0.0/8, ::1/128
    AllowedSchemes: []string{"https"},  // Só HTTPS (não http, file, ftp)
    AllowLocalhost: false,        // Bloqueia localhost
    MaxRedirects: 5,              // Limita redirects
    Timeout: 10 * time.Second,    // Timeout obrigatório
    ValidateURL: true,            // Valida URL contra allowlist de domínios
})
```

#### Insecure Deserialization — Controle Nativo

A framework só aceita **dois formatos de serialização**: JSON e MsgPack (binário). Ambos passam por schema validation ANTES de serem deserializados:

1. Payload chega → valida encoding (UTF-8 para JSON)
2. Parseia para `map[string]any` ou `interface{}`
3. Valida contra schema (rejeita tipos não esperados, `additionalProperties: false`)
4. Converte para struct tipado do worker

XML, YAML, Pickle, e `eval()` são **bloqueados** pelo worker SDK.

#### SQL Injection — Controle por Design

A framework **não fornece** acesso a raw SQL. Toda query deve ser feita através de um ORM/query builder que use parameterized queries:

```go
// Permitido — ORM com parameterized queries
db.Query("SELECT * FROM users WHERE id = ?", userID)

// Bloqueado — raw SQL
db.Exec("SELECT * FROM users WHERE id = " + userID)  // erro de compilação
```

---

## 8. ORQUESTRADOR EM GO — Ponto de Enforcement

### 8.1 Mecanismos Centralizados

| Mecanismo | Obrigatório? | Local no Pipeline | O Que Acontece se Falhar |
|-----------|-------------|-------------------|-------------------------|
| **Autenticação upstream** | ✅ Sempre | Antes do roteamento | 401 Unauthorized |
| **Autorização preliminar** | ✅ Sempre | Depois da autenticação, antes do worker | 403 Forbidden |
| **Validação de schema** | ✅ Sempre (se schema existe) | Depois da autenticação | 422 Unprocessable Entity |
| **Rate limiting** | ✅ Sempre | Antes da autenticação (IP) + depois (identidade) | 429 Too Many Requests |
| **Quotas** | ✅ Se configurado | Depois da autorização | 429 + `X-RateLimit-*` headers |
| **Timeouts** | ✅ Sempre | Todo o pipeline | 504 Gateway Timeout |
| **Circuit breaking** | ✅ Sempre | Por worker | 503 Service Unavailable |
| **Roteamento seguro** | ✅ Sempre | Valida rota existe | 404 Not Found |
| **Service discovery seguro** | ✅ Sempre | Worker registration | Worker não registrado |
| **Client HTTP seguro** | ✅ Sempre (SDK) | Nas chamadas do worker | Bloqueia RFC 1918 |
| **Políticas de egress** | ✅ Se configurado | Nas chamadas do worker | Bloqueia destino |
| **Proteção SSRF** | ✅ Sempre | Nas chamadas do worker | Bloqueia IP interno |
| **Logs de segurança** | ✅ Sempre | Todo evento | Loga independente |
| **Auditoria** | ✅ Sempre | AuthN, AuthZ, schema violation | Loga evento |
| **Inventory de endpoints** | ✅ Sempre | Build step | Falha se rota sem schema |
| **Gestão de segredos** | ✅ Sempre | Startup | Falha se secret vazio |
| **Configuração segura** | ✅ Sempre | Startup | Falha se config insegura |

### 8.2 O Que NÃO Deve Ficar no Orquestrador

| Funcionalidade | Motivo da Exclusão | Onde Deve Ficar |
|---------------|-------------------|-----------------|
| **Business logic** | Viola separação de responsabilidades, torna o orquestrador um monólito | Worker (Go/Python/Node) |
| **Object-level authorization** | Orquestrador não conhece o domínio do negócio (não sabe o que é "owner" de um recurso) | Worker + Policy engine |
| **Database queries** | Orquestrador não deve ter acesso a databases de aplicação | Worker |
| **File storage** | Orquestrador gerencia sockets e processos, não arquivos | Worker + Storage service |
| **Cache de aplicação** | Cache de sessão OK, cache de negócio não | Redis/Memcached (via worker) |
| **Processamento de arquivos** | Upload validation OK, processing não | Worker + Job queue |
| **Integrações externas** | API calls para terceiros é responsabilidade do worker | Worker |

### 8.3 Riscos de Fazer Demais no Orquestrador

| Risco | Consequência | Mitigação |
|-------|-------------|-----------|
| **SPOF lógico** | Se o orquestrador cai, tudo cai | Design stateless, restart rápido, múltiplas réplicas |
| **Latência** | Cada funcionalidade adiciona latência ao pipeline | Pipeline assíncrono onde possível, timeouts estritos |
| **Complexidade** | Orquestrador vira God object | Módulos plugáveis, interfaces claras, testes de isolamento |
| **Memory pressure** | Orquestrador acumula estado | Cache com TTL, limites de memória, profiling contínuo |
| **Acoplamento** | Mudança em um worker exige mudança no orquestrador | Contratos estáveis (schemas, interfaces), versionamento |

### 8.4 Manutenção de Observabilidade e Latência

O pipeline de segurança do orquestrador não deve adicionar mais que **5ms** ao tempo total de resposta (em condições normais). Para garantir isso:

1. **Métricas de latência por fase:** cada fase do pipeline (authN, authZ, validation, routing) tem um histograma.
2. **Budget de latência:** alerta se P99 de qualquer fase excede 10ms.
3. **Cache de schemas compilados:** schemas JSON são compilados uma vez no startup.
4. **Cache de policies:** policies são carregadas e avaliadas em cache.
5. **JWT validation sem network:** chave HMAC em memória (não vai a banco).

---

## 9. CLIENTES E BIBLIOTECAS INTERNAS

### 9.1 APIs Consistentes entre Runtimes

Cada runtime SDK expõe os mesmos conceitos através de APIs que seguem as convenções da linguagem:

#### Validação

```go
// Go — validação declarativa com tags
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=2,max=100"`
    Email string `json:"email" validate:"required,email"`
}
```

```python
# Python — validação com dataclasses + pydantic
@vyx.schema
class CreateUserRequest:
    name: str = vyx.Field(min_length=2, max_length=100)
    email: str = vyx.Field(format="email")
```

```typescript
// Node.js — validação com decorators
@vyx.schema
class CreateUserRequest {
    @vyx.required() @vyx.min(2) @vyx.max(100)
    name: string;
    
    @vyx.required() @vyx.email()
    email: string;
}
```

**Comportamento idêntico:** todos os três rejeitam campos extras, normalizam tipos, e produzem a mesma estrutura de erro.

#### Autenticação

```go
// Go
userID := ctx.UserID()
isAdmin := ctx.HasRole("admin")
tenantID := ctx.TenantID()
```

```python
# Python
user_id = ctx.user_id
is_admin = ctx.has_role("admin")
tenant_id = ctx.tenant_id
```

```typescript
// Node.js
const userId = ctx.userId;
const isAdmin = ctx.hasRole("admin");
const tenantId = ctx.tenantId;
```

#### Resposta Segura

```go
// Go — helpers que garantem resposta segura
return vyx.JSON(200, user, vyx.WithSchema("user-response"))
```

```python
# Python
return vyx.json(200, user, schema="user-response")
```

```typescript
// Node.js
return vyx.json(200, user, { schema: 'user-response' });
```

**Comportamento idêntico:** todos validam a resposta contra o schema antes de serializar.

#### Client HTTP Seguro (para chamadas externas do worker)

```go
// Go — SSRF protection nativa
client := vyx.NewHTTPClient()
resp, err := client.Get("https://api.externa.com/data")
// Bloqueia automaticamente: 10.0.0.0/8, 192.168.0.0/16, localhost, etc.
```

```python
# Python
client = vyx.HTTPClient()
resp = await client.get("https://api.externa.com/data")
# Mesma proteção SSRF
```

```typescript
// Node.js
const client = new vyx.HTTPClient();
const resp = await client.get('https://api.externa.com/data');
// Mesma proteção SSRF
```

### 9.2 Prevenindo Comportamento Divergente

Cada SDK é gerado a partir de uma **especificação de comportamento comum** (arquivo YAML):

```yaml
# vyx-security-contract.yaml
behaviors:
  - name: schema_validation
    rules:
      - "additionalProperties: false"  # OBRIGATÓRIO
      - "null values rejeitados exceto nullable explícito"
      - "campos extras retornam 422 com código VYX-VAL-004"
    test_cases:
      - input: {"name": "João", "extra_field": "x"}
        expected_status: 422
        expected_code: "VYX-VAL-004"

  - name: http_client
    rules:
      - "bloqueia RFC 1918 (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)"
      - "bloqueia localhost (127.0.0.1, ::1, localhost)"
      - "apenas https permitido (http bloqueado)"
      - "timeout padrão 10s"
    test_cases:
      - url: "http://192.168.1.1/admin"
        expected_error: "SSRF_BLOCKED"
```

Testes de conformidade (seção 14) validam que cada SDK implementa exatamente o comportamento especificado.

---

## 10. CONFIGURAÇÕES SEGURAS POR PADRÃO

### 10.1 Tabela de Defaults Obrigatórios

| Configuração | Default | Sobrescrevível? | Justificativa | Telemetria |
|-------------|---------|-----------------|---------------|------------|
| `cors.origins` | `[]` (mesma origem) | ✅ Sim, mas loga WARN se `["*"]` | CORS aberto é risco de exfiltração | `cors_blocked_requests_total` |
| `cors.allow_credentials` | `true` | ❌ Não (requer origens específicas) | `*` com credentials quebra segurança | — |
| `security.csp` | `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'` | ✅ Sim | CSP baseline evita XSS | `csp_violations_total` |
| `security.hsts` | `max-age=63072000; includeSubDomains` | ❌ Não em produção | HSTS previne downgrade | — |
| `security.cookies.same_site` | `Lax` | ❌ Não | CSRF prevention | — |
| `security.cookies.secure` | `true` | ❌ Não (só false em dev local) | Previne MITM | — |
| `security.cookies.http_only` | `true` | ❌ Não | Previne XSS access | — |
| `server.read_timeout` | `15s` | ✅ Sim | Slowloris prevention | `http_read_timeouts_total` |
| `server.write_timeout` | `30s` | ✅ Sim (WebSocket exige 0) | Slow response prevention | `http_write_timeouts_total` |
| `server.idle_timeout` | `60s` | ✅ Sim | Connection exhaustion | — |
| `server.max_header_bytes` | `1MB` | ✅ Sim | Header flood prevention | `http_header_too_large_total` |
| `validation.max_body_bytes` | `1MB` | ✅ Sim | Payload flood prevention | `http_payload_too_large_total` |
| `validation.max_depth` | `10` | ✅ Sim | Nested object attack | — |
| `validation.max_properties` | `100` | ✅ Sim | Object flood prevention | — |
| `validation.max_items` | `1000` | ✅ Sim | Array flood prevention | — |
| `upload.max_file_size` | `10MB` | ✅ Sim | Disk exhaustion | `upload_too_large_total` |
| `upload.allowed_mime_types` | `[]` (bloqueado) | ✅ Sim | Precisa allowlist explícita | `upload_blocked_total` |
| `error.expose_details` | `false` | ❌ Não (só true em dev) | Stack trace exposure | — |
| `log.redact_secrets` | `true` | ❌ Não | Secret leak prevention | — |
| `log.redact_fields` | `["password", "token", "secret", "credit_card", "ssn"]` | ✅ Sim (append only) | PII protection | — |
| `http_client.block_private_ips` | `true` | ❌ Não | SSRF prevention | `ssrf_blocked_total` |
| `http_client.allowed_schemes` | `["https"]` | ❌ Não (só https) | SSRF + MITM prevention | — |
| `http_client.timeout` | `10s` | ❌ Não (obrigatório) | Resource exhaustion | `http_client_timeouts_total` |
| `rate_limit.ip.enabled` | `true` | ✅ Sim (mas loga WARN) | Brute force prevention | `rate_limit_ip_blocked_total` |
| `rate_limit.ip.max_requests` | `100` | ✅ Sim | Configurável por app | — |
| `rate_limit.ip.window` | `1m` | ✅ Sim | Configurável por app | — |
| `rate_limit.identity.enabled` | `true` | ✅ Sim (mas loga WARN) | Per-user rate limit | `rate_limit_identity_blocked_total` |
| `circuit_breaker.failures` | `5` | ✅ Sim | Protege workers | `circuit_breaker_trips_total` |
| `circuit_breaker.cooldown` | `30s` | ✅ Sim | Recuperação gradual | — |
| `redirect.validate` | `true` | ❌ Não | Open redirect prevention | `redirect_blocked_total` |
| `redirect.allowed_hosts` | `[]` (rejeita todos) | ✅ Sim | Precisa allowlist | — |

---

## 11. OBSERVABILIDADE, AUDITORIA E FORENSIA

### 11.1 Security Event Taxonomy

Todo evento de segurança segue esta estrutura:

```json
{
  "event": {
    "id": "evt-uuid-001",
    "timestamp": "2026-05-28T15:00:00.000Z",
    "type": "authentication.failure|authentication.success|authorization.denied|validation.failure|rate_limit.exceeded|abuse.detected|secret.access|admin.action|schema.violation|certificate.expiry|worker.crash|config.change",
    "severity": "critical|high|medium|low|info",
    "correlation_id": "corr-uuid-002",
    "request_id": "req-uuid-003",
    "source_ip": "200.201.202.203",
    "identity": {
      "type": "user|service|anonymous",
      "id": "user-uuid-004"
    },
    "resource": {
      "type": "route|worker|secret|config",
      "id": "POST /api/orders"
    },
    "action": "create|read|update|delete|access|modify|execute",
    "result": "success|failure|blocked",
    "detail": {
      "reason": "token_expired|role_mismatch|schema_mismatch|rate_exceeded",
      "code": "VYX-AUTH-003"
    },
    "context": {
      "tenant_id": "tenant-uuid-005",
      "environment": "production",
      "worker_id": "go:api"
    }
  }
}
```

### 11.2 O Que Logar vs O Que Nunca Logar

| ✅ Logar Sempre | ❌ Nunca Logar |
|----------------|---------------|
| Tentativas de autenticação (successo + falha) | Tokens JWT completos |
| Tentativas de autorização negadas | Senhas (qualquer forma) |
| Violações de schema de validação | Credit card numbers |
| Rate limit excedido | Secrets/API keys |
| Erros de validação (apenas tipo + campo, não valor) | Dados biométricos |
| Mudanças de configuração | Session IDs completos |
| Acesso a secrets | Dados de health check sensíveis |
| Ações administrativas | Dados de usuário desnecessários |
| Worker crashes | Raw request/response bodies |
| Circuit breaker state changes | Headers de autorização |

### 11.3 Correlação entre Orquestrador e Workers

Cada evento de segurança no orquestrador inclui o `correlation_id` que é propagado para o worker. Eventos no worker também incluem o mesmo `correlation_id`. Isso permite:

```
Orquestrador: {
  "correlation_id": "corr-abc",
  "type": "authentication.success"
}
Worker: {
  "correlation_id": "corr-abc",
  "type": "authorization.denied",
  "detail": "user lacks permission order:cancel"
}
```

Ferramentas de observabilidade (Grafana, Datadog, ELK) podem correlacionar esses eventos pelo `correlation_id`.

### 11.4 Retenção e Redaction

| Tipo de Evento | Retenção Mínima | Redaction |
|----------------|----------------|-----------|
| Autenticação (sucesso) | 90 dias | Remover IP após 30 dias |
| Autenticação (falha) | 1 ano | Nenhum (forense) |
| Autorização (negada) | 1 ano | Nenhum |
| Violação de schema | 90 dias | Remover body values |
| Rate limit | 30 dias | Remover IP após 7 dias |
| Ação administrativa | 2 anos | Nenhum |
| Acesso a secrets | 2 anos | Nenhum (quem, quando, qual secret) |
| Worker crash | 90 dias | Remover env vars |

### 11.5 Alertas Obrigatórios

| Evento | Threshold | Canal | Prioridade |
|--------|-----------|-------|-----------|
| Taxa de falha de autenticação | > 10% em 5 min | PagerDuty/OpsGenie | 🔴 Crítica |
| Múltiplas autorizações negadas (mesmo user) | > 5 em 1 min | Slack/Email | 🟠 Alta |
| Violação de schema (mesmo IP) | > 20 em 1 min | Slack/Email | 🟠 Alta |
| Rate limit excedido (mesmo IP) | > 50 em 5 min | Slack | 🟡 Média |
| Circuit breaker open | Qualquer | PagerDuty | 🔴 Crítica |
| Worker crash | Qualquer | PagerDuty | 🔴 Crítica |
| Config change | Qualquer | Slack (audit log) | 🟡 Média |

---

## 12. MODO MULTITENANT

### 12.1 Isolamento por Camada

| Camada | Mecanismo de Isolamento | Como é Enforced |
|--------|------------------------|-----------------|
| **Autenticação** | `iss` claim (issuer = tenant ID) no JWT | Orquestrador valida `iss` contra tenant do request |
| **Autorização** | `tenant_id` no SecurityContext, policies com tenant check | Toda policy evaluation inclui `resource.tenant_id == user.tenant_id` |
| **Rate limiting** | Buckets separados por tenant | `rate_limiter:tenant_abc` e `rate_limiter:tenant_def` independentes |
| **Cache** | Prefixo de chave por tenant: `cache:tenant_abc:user:123` | Cache SDK força prefixo |
| **Storage** | Schema/Database separado por tenant (opcional) ou row-level security | Query builder injeta `WHERE tenant_id = ?` automaticamente |
| **Jobs** | Fila separada por tenant | Job dispatcher roteia para fila do tenant |
| **Logs** | `tenant_id` em todo evento | Campo indexado, queries filtram por tenant |
| **Secrets** | Key ring por tenant | `VYX_SECRET_<TENANT>_<KEY>` |

### 12.2 Cross-Tenant Data Leak Prevention

O orquestrador **injeta `tenant_id` em toda query** que o worker faz via framework SDK:

```go
// Go — o que o worker escreve
users := db.Query("SELECT * FROM users WHERE id = ?", userID)

// O que o framework executa (injeção automática)
// SELECT * FROM users WHERE id = ? AND tenant_id = ?
// Parâmetros: [userID, ctx.TenantID()]
```

Se o worker tentar fazer uma query sem tenant (usando raw SQL), o framework bloqueia:

```go
// Isso é BLOQUEADO pelo framework
db.Exec("SELECT * FROM users") // ERRO: tenant isolation required
```

### 12.3 Autorização Contextual por Tenant

```go
// Go — policy check automático
func handleGetOrder(ctx *vyx.Context, req *Request) (*Response, error) {
    orderID := req.Params["id"]
    
    // Automaticamente verifica:
    // 1. O recurso order:{orderID} existe?
    // 2. O tenant do recurso == ctx.TenantID()?
    // 3. O user tem permissão no recurso?
    order, err := db.GetOrder(orderID)  ← tenant_id injetado automaticamente
    if err != nil {
        return nil, vyx.ErrNotFound  // 404 genérico (não revela existência)
    }
    
    return vyx.JSON(200, order, vyx.WithSchema("order-response"))
}
```

---

## 13. DESIGN DE ERROS E RESPOSTAS

### 13.1 Formato Padrão de Erro

```json
{
  "error": {
    "code": "VYX-AUTH-001",
    "title": "Token expirado",
    "detail": "O token de acesso expirou. Faça refresh.",
    "source": {
      "pointer": "/headers/authorization",
      "parameter": "authorization"
    },
    "meta": {
      "correlation_id": "corr-abc-123",
      "timestamp": "2026-05-28T15:00:00Z"
    }
  }
}
```

### 13.2 Códigos Internos vs HTTP Status

| HTTP Status | Prefixo de Código | Exemplo | Significado |
|-------------|-------------------|---------|-------------|
| 400 | `VYX-VAL-` | `VYX-VAL-001` | Erro de validação (campo obrigatório) |
| 401 | `VYX-AUTH-` | `VYX-AUTH-001` | Token expirado |
| 403 | `VYX-AUTHZ-` | `VYX-AUTHZ-001` | Role insuficiente |
| 404 | `VYX-ROUTE-` | `VYX-ROUTE-001` | Rota não encontrada |
| 409 | `VYX-CONFLICT-` | `VYX-CONFLICT-001` | Conflito (resource já existe) |
| 422 | `VYX-VAL-` | `VYX-VAL-004` | Campo extra não permitido |
| 429 | `VYX-RATE-` | `VYX-RATE-001` | Rate limit excedido |
| 500 | `VYX-INTERNAL-` | `VYX-INTERNAL-001` | Erro interno (genérico) |
| 502 | `VYX-UPSTREAM-` | `VYX-UPSTREAM-001` | Worker error |
| 503 | `VYX-CB-` | `VYX-CB-001` | Circuit breaker open |
| 504 | `VYX-TIMEOUT-` | `VYX-TIMEOUT-001` | Upstream timeout |

### 13.3 Mensagens Seguras para Cliente vs Logs Internos

```
Cliente recebe:
{
  "error": {
    "code": "VYX-INTERNAL-001",
    "title": "Erro interno do servidor",
    "detail": "Ocorreu um erro inesperado. Tente novamente.",
    "meta": {
      "correlation_id": "corr-abc-123"
    }
  }
}

Log interno (NUNCA enviado ao cliente):
{
  "event": {
    "type": "internal.error",
    "correlation_id": "corr-abc-123",
    "detail": {
      "error": "connection refused: worker go:api at /tmp/vyx/go:api.sock",
      "stack": "goroutine 42 ...\n...",
      "worker_id": "go:api",
      "retry_count": 3
    }
  }
}
```

---

## 14. TESTABILIDADE E CONFORMIDADE

### 14.1 Test Suite de Conformidade

Cada runtime SDK (Go, Python, Node.js) DEVE passar o mesmo test suite de conformidade:

```
vyx-security-conformance/
├── authn/
│   ├── test_jwt_validation.go/py/ts     — token válido, expirado, mau-assinado, iss errado
│   ├── test_session_validation.go/py/ts  — sessão válida, expirada, revogada
│   └── test_mtls.go/py/ts               — cert válido, expirado, self-signed
├── authz/
│   ├── test_role_check.go/py/ts         — role match, mismatch, múltiplas roles
│   ├── test_scope_check.go/py/ts        — scope match, mismatch
│   └── test_tenant_isolation.go/py/ts   — cross-tenant blocked
├── validation/
│   ├── test_schema.go/py/ts             — body válido, campo extra, tipo errado
│   ├── test_additional_properties.go/py/ts — campo extra = 422
│   ├── test_depth_limit.go/py/ts        — nested > 10 = 422
│   ├── test_cardinality.go/py/ts        — array > 1000 = 422
│   └── test_null_handling.go/py/ts      — null rejeitado
├── http/
│   ├── test_security_headers.go/py/ts   — todos os headers presentes
│   ├── test_cors.go/py/ts               — CORS bloqueado por padrão
│   ├── test_rate_limit.go/py/ts         — rate limit funciona
│   └── test_ssrf_block.go/py/ts         — RFC 1918 bloqueado
└── serialization/
    ├── test_json_safe.go/py/ts          — rejeita __proto__, constructor
    ├── test_msgpack.go/py/ts            — msgpack válido
    └── test_response_schema.go/py/ts    — campos extras removidos
```

### 14.2 Casos Negativos Obrigatórios

```
TODO HANDLER DEVE TER TESTES NEGATIVOS PARA:
  ✅ Token ausente → 401
  ✅ Token expirado → 401
  ✅ Token mau-assinado → 401
  ✅ Role insuficiente → 403
  ✅ Scope insuficiente → 403
  ✅ Tenant mismatch → 403
  ✅ Campo extra no body → 422
  ✅ Tipo errado no body → 422
  ✅ Body muito grande → 413
  ✅ Rate limit excedido → 429
  ✅ Worker timeout → 504
  ✅ Circuit breaker open → 503
  ✅ Rota inexistente → 404
  ✅ Método não permitido → 405
  ✅ Content-Type inválido → 415
  ✅ SSRF attempt → 403
  ✅ Path traversal → 400
  ✅ XSS in headers → 400
  ✅ Unicode/encoding attacks → 400
```

### 14.3 Fuzzing de Parsers

```
ALVOS DE FUZZING:
  - JSON parser (body, query, headers)
  - MsgPack parser
  - URL parser (path params, query params)
  - JWT parser
  - HTTP header parser
  - Cookie parser
  - Multipart parser (uploads)

FERRAMENTAS:
  - Go: go-fuzz
  - Python: Atheris
  - Node.js: jsfuzz
```

### 14.4 Critérios para Liberar Novo Adapter/Runtime

Um novo runtime (ex: Ruby, Rust, Elixir) só pode ser integrado se:

1. ✅ Passa 100% do test suite de conformidade.
2. ✅ Implementa o contrato de segurança (SecurityContext, validação, client HTTP).
3. ✅ Testes de fuzzing não encontram crashes.
4. ✅ Golden tests de autorização produzem mesmos resultados que Go.
5. ✅ Schema validation rejeita mesmos casos que Go.
6. ✅ Rate limiting, circuit breaker, e timeouts funcionam como em Go.
7. ✅ Logs de segurança seguem a taxonomy definida.
8. ✅ Documentação de divergências (se houver) é revisada e aprovada.

---

## 15. ROADMAP DE IMPLEMENTAÇÃO

### Fase 1: Fundações Mínimas (Sprint 1-2)

**Objetivo:** Framework segura por padrão para o caso mais comum (API REST + JWT).

| Entregável | Risco Mitigado | Dependência | Esforço |
|-----------|---------------|-------------|---------|
| JWT validation no orquestrador (com `iss`, `aud`, `jti`) | AUTH-001, AUTH-002 | Nenhuma | 2 dias |
| Schema validation com `additionalProperties: false` | Mass assignment | Nenhuma | 2 dias |
| Security headers (HSTS, CSP, X-Frame-Options, etc.) | XSS, clickjacking | Nenhuma | 1 dia |
| Rate limiting por IP (sliding window) | Brute force, DDoS | Nenhuma | 1 dia |
| Read/Write/Idle timeouts configurados | Slowloris, resource exhaustion | Nenhuma | 0.5 dia |
| CORS fechado por padrão | Cross-origin data leak | Nenhuma | 0.5 dia |
| Error handling padronizado (códigos VYX-*) | Information disclosure | Nenhuma | 1 dia |
| Log redaction (campos sensíveis) | Secret leak | Nenhuma | 0.5 dia |

**Quick win:** JWT validation + schema validation já resolvem 60% dos riscos de segurança mais comuns.

**Trade-off:** Sem object-level authorization ainda. Apps precisam implementar manualmente.

### Fase 2: Enforcement Central (Sprint 3-4)

**Objetivo:** Orquestrador como ponto de enforcement obrigatório.

| Entregável | Risco Mitigado | Dependência | Esforço |
|-----------|---------------|-------------|---------|
| Circuit breaker por worker | Cascading failure | Fase 1 | 1 dia |
| Payload size limits + depth/cardinality | Resource exhaustion | Fase 1 | 1 dia |
| Content-Type validation | Deserialization attacks | Fase 1 | 0.5 dia |
| Header sanitization (CRLF stripping) | Response splitting | Fase 1 | 0.5 dia |
| Response schema validation (saída) | Data over-exposure | Fase 1 | 2 dias |
| Worker isolation (setpgid, cgroups) | Worker-to-worker attack | Nenhuma | 2 dias |
| Route inventory validation (build step) | Orphan routes | Nenhuma | 1 dia |
| Client HTTP com SSRF protection | SSRF | Fase 1 | 1 dia |

**Quick win:** Response schema validation + header sanitization previnem duas das vulnerabilidades mais comuns em APIs.

**Trade-off:** SSRF protection no client HTTP pode quebrar integrações legítimas. Exige allowlist.

### Fase 3: Authz Forte e Observabilidade (Sprint 5-7)

**Objetivo:** Autorização granular e capacidade de investigação.

| Entregável | Risco Mitigado | Dependência | Esforço |
|-----------|---------------|-------------|---------|
| Policy DSL + PDP (Policy Decision Point) | BOLA, BOPLA | Fase 2 | 5 dias |
| Object-level authorization helpers | BOLA | Fase 2 + Policy DSL | 3 dias |
| Tenant isolation automático | Cross-tenant leak | Fase 2 + Policy DSL | 3 dias |
| Security event taxonomy + logging | Audit, forensics | Fase 1 | 2 dias |
| Audit trail (authN, authZ, schema violation) | Compliance | Fase 2 | 2 dias |
| Rate limiting por identidade (além de IP) | Credential stuffing | Fase 1 | 1 dia |
| Account lockout após N falhas | Brute force | Fase 2 | 1 dia |
| Security metrics + alerting | OpsSec | Fase 2 | 2 dias |

**Quick win:** Policy DSL + tenant isolation resolvem as vulnerabilidades mais críticas de APIs multi-tenant.

**Trade-off:** Policy DSL adiciona complexidade. Exige documentação e exemplos claros.

### Fase 4: Hardening Avançado (Sprint 8-10)

**Objetivo:** Proteção contra ataques avançados e cenários extremos.

| Entregável | Risco Mitigado | Dependência | Esforço |
|-----------|---------------|-------------|---------|
| MFA verification plugável | Account takeover | Fase 2 | 5 dias |
| Reautenticação para ações críticas | Privilege escalation | Fase 3 | 2 dias |
| Key rotation automática | Key compromise | Fase 1 | 2 dias |
| mTLS service-to-service | Worker impersonation | Fase 2 | 3 dias |
| File upload validation (magic bytes + scan) | Malware upload | Fase 2 | 3 dias |
| CSP violation reporting | XSS detection | Fase 1 | 1 dia |
| Fuzzing pipeline no CI | 0-day in parsers | Fase 1 | 3 dias |
| Rate limiting adaptativo (baseado em padrões) | Advanced brute force | Fase 3 | 5 dias |

**Quick win:** MFA + reautenticação previnem os ataques mais comuns de account takeover.

**Trade-off:** mTLS adiciona complexidade operacional significativa. Vale a pena apenas em ambientes multi-tenant.

### Fase 5: Conformidade e Ecossistema (Sprint 11-14)

**Objetivo:** Framework certificável e extensível.

| Entregável | Risco Mitigado | Dependência | Esforço |
|-----------|---------------|-------------|---------|
| Test suite de conformidade (todos runtimes) | Comportamento divergente | Fase 1-4 | 5 dias |
| Gold master tests de autorização | Regression em authz | Fase 3 | 3 dias |
| SBOM generation + dependency scanning | Supply chain | Fase 1 | 2 dias |
| OWASP ASVS compliance report | Certification | Fase 1-4 | 5 dias |
| SOC 2 evidence collection | Audit | Fase 3 | 3 dias |
| New adapter checklist (Ruby, Rust, Elixir) | Ecosystem growth | Fase 1-4 | 2 dias |
| Security documentation (threat model, playbook) | Team knowledge | Fase 1-4 | 5 dias |

**Quick win:** Test suite de conformidade garante que todos os runtimes se comportam igualmente — o risco número 1 de frameworks poliglotas.

**Trade-off:** OWASP ASVS compliance é caro. Focar nos controles de nível 1 primeiro.

---

## 16. RESULTADO FINAL

### 16.1 Diagrama Textual da Arquitetura

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        CLIENTE (Browser / Mobile / SPA)                      │
└────────────────────────────┬────────────────────────────────────────────────┘
                             │ HTTPS (TLS 1.3)
                             ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                     ORQUESTRADOR GO (Ponto de Enforcement)                   │
│                                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │  TLS        │  │  WAF        │  │  AuthN      │  │  Schema Validation  │ │
│  │  Termination│→│  (size,     │→│  (JWT,      │→│  (JSON Schema,      │ │
│  │  + HSTS     │  │   method,   │  │   session,  │  │   additionalProps,  │ │
│  │             │  │   header)   │  │   mTLS)     │  │   depth limits)     │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────────┘ │
│                                                    │                         │
│  ┌─────────────────────┐  ┌─────────────┐         │                         │
│  │  Authorization      │  │  Rate Limit │         │                         │
│  │  (Route-level,      │←│  (IP +      │←────────┘                         │
│  │   Policy DSL)       │  │   Identity) │                                   │
│  └─────────────────────┘  └─────────────┘                                   │
│           │                                                                  │
│           ▼                                                                  │
│  ┌────────────────────────────────────────────────────────────────────┐     │
│  │                    SECURE ROUTER (Circuit Breaker + Timeout)        │     │
│  └────────────────────────────────────────────────────────────────────┘     │
│           │                                                                  │
│           ├──────────────────────────┬──────────────────────────┐            │
│           ▼                         ▼                          ▼            │
│  ┌──────────────┐        ┌──────────────┐          ┌──────────────┐         │
│  │  Worker GO   │        │  Worker NODE │          │  Worker PY   │         │
│  │  (go:api)    │        │  (node:ssr)  │          │  (python:ml) │         │
│  └──────┬───────┘        └──────┬───────┘          └──────┬───────┘         │
│         │                       │                         │                  │
│         └───────────┬───────────┘                         │                  │
│                     │                                     │                  │
│                     ▼                                     │                  │
│  ┌────────────────────────────────────────┐               │                  │
│  │     RESPONSE VALIDATOR                  │               │                  │
│  │  • Schema de saída                      │               │                  │
│  │  • Security headers                     │◄──────────────┘                  │
│  │  • CORS enforcement                     │                                  │
│  │  • Error sanitization                   │                                  │
│  └────────────────────────────────────────┘                                  │
└─────────────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                       RESPOSTA AO CLIENTE                                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 16.2 Matriz Ameaça → Controle → Camada Responsável

| Ameaça | Controle Primário | Camada | Controle Secundário | Camada |
|--------|------------------|--------|-------------------|--------|
| XSS | CSP headers | Orquestrador | Output encoding (SSR) | Worker/React |
| CSRF | SameSite cookie | Orquestrador | Origin validation | Orquestrador |
| SSRF | HTTP client bloqueia RFC 1918 | Worker SDK | URL validation | Worker |
| SQL Injection | ORM obrigatório | Worker | Schema validation | Orquestrador |
| Command Injection | exec.Command bloqueado | Worker SDK | Input validation | Orquestrador |
| Path Traversal | File ops bloqueadas fora do projeto | Worker SDK | Path allowlist | Worker |
| Insecure Deserialization | Só JSON/MsgPack | Orquestrador | Schema validation | Orquestrador |
| XXE | XML desabilitado | Orquestrador | — | — |
| File Upload | Magic bytes + size limit | Orquestrador | AV scan | Worker |
| Mass Assignment | additionalProperties: false | Orquestrador | Schema de body explícito | App |
| BOLA | Policy DSL + tenant check | Worker | Ownership helpers | Worker SDK |
| BOPLA | Schema de resposta | Orquestrador | Property-level authz | Orquestrador |
| Rate Abuse | Rate limit IP + identity | Orquestrador | Quotas | Orquestrador |
| Brute Force | Account lockout | Orquestrador | Rate limit | Orquestrador |
| Credential Stuffing | Rate limit + CAPTCHA | Orquestrador | Pattern detection | Orquestrador |
| User Enumeration | Mensagens genéricas | Orquestrador | Timing-safe comparison | Orquestrador |
| Secret Leak | Log redaction | Orquestrador | CI scanner | CI/CD |
| Open Redirect | redirect.validate=true | Orquestrador | Allowlist | App |
| CORS | Fechado por padrão | Orquestrador | Origin validation | Orquestrador |
| Cache Poisoning | Cache-Control headers | Orquestrador | Vary: Origin | Orquestrador |
| Request Smuggling | HTTP/2 preference | Orquestrador | Content-Length validation | Orquestrador |
| Resource Exhaustion | Timeouts + circuit breaker | Orquestrador | Payload limits | Orquestrador |
| Supply Chain | govulncheck + npm audit | CI/CD | SBOM | CI/CD |
| Orphan Routes | Build-time validation | Orquestrador | Route inventory | Build step |

### 16.3 Não Negociáveis do Core

1. **JWT validation com `iss`, `aud`, `exp`, `nbf`, `jti`** — sem exceção. Não há flag para desabilitar.
2. **Schema validation com `additionalProperties: false`** — sem exceção. Todo body é validado.
3. **Schema de resposta obrigatório** — toda rota de leitura precisa declarar campos retornáveis.
4. **Security headers** — HSTS, CSP, X-Content-Type-Options, X-Frame-Options em toda resposta.
5. **CORS fechado por padrão** — `Access-Control-Allow-Origin` nunca é `*` com credentials.
6. **Rate limiting por IP** — sempre ativo (threshold configurável, mas não desligável).
7. **Timeouts** — Read/Write/Idle timeouts obrigatórios.
8. **Circuit breaker** — proteção contra falha em cascata de workers.
9. **Log redaction** — campos sensíveis nunca aparecem em logs.
10. **Client HTTP seguro** — bloqueia RFC 1918, exige https, timeout obrigatório.
11. **Error handling padronizado** — códigos VYX-*, mensagens genéricas para cliente.
12. **Worker isolation** — `setpgid`, namespace, sem comunicação direta entre workers.
13. **Startup validation** — core falha se configurações inseguras (secret curto, CORS aberto).
14. **Route inventory** — toda rota no route_map.json tem handler correspondente.
15. **Correlation ID** — toda requisição tem ID único rastreável.

### 16.4 Extensível pelo Usuário

1. **Identity Provider** — OAuth, SAML, LDAP, OpenID Connect.
2. **MFA** — TOTP, SMS, WebAuthn.
3. **Policy DSL** — regras customizadas de autorização.
4. **Schemas de validação** — schemas específicos da aplicação.
5. **Rate limit thresholds** — configuração por rota/tenant.
6. **CORS origins** — allowlist de origens confiáveis.
7. **Upload MIME types** — allowlist de tipos permitidos.
8. **Event handlers** — hooks para eventos de segurança (login, logout, ação crítica).
9. **Storage backend** — sessões, tokens de refresh, audit log.
10. **Secrets manager** — Vault, AWS KMS, Azure Key Vault.

### 16.5 Checklist Operacional para a Primeira Versão

```
□ 1. JWT validation com iss + aud + exp + nbf + jti + blacklist
□ 2. Schema validation com additionalProperties: false + depth/cardinality limits
□ 3. Schema de resposta obrigatório (allowlist de campos retornáveis)
□ 4. Security headers (HSTS, CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy)
□ 5. CORS fechado (mesma origem apenas, allowlist explícita quando necessário)
□ 6. Rate limiting: IP (100/min) + Identity (500/min)
□ 7. Timeouts: Read 15s, Write 30s, Idle 60s, Dispatch 30s
□ 8. Circuit breaker: 5 failures, 30s cooldown, 1 half-open probe
□ 9. Payload limits: 1MB body, 10 depth, 100 properties, 1000 items
□ 10. Log redaction: password, token, secret, credit_card, ssn
□ 11. Error handling: códigos VYX-*, detail genérico, correlation ID
□ 12. Client HTTP: bloqueia RFC 1918, exige https, timeout 10s
□ 13. Worker isolation: setpgid, SIGTERM→SIGKILL, WaitDelay 3s
□ 14. Startup validation: JWT secret ≥ 32 bytes, schemas compilados, rotas inventariadas
□ 15. Correlation ID: gerado no orquestrador, propagado para workers, retornado ao cliente
□ 16. Route inventory: build step valida que toda rota tem schema + handler
□ 17. Schema WarmUp: chamado no startup (primeiro request não paga custo)
□ 18. Dependencies: govulncheck + npm audit + pip-audit no CI
□ 19. Content-Type validation: application/json obrigatório (rejeitar outros)
□ 20. Method validation: allowlist GET, POST, PUT, PATCH, DELETE (rejeitar outros)
```

---

## Apêndice A: Glossário de Segurança

| Termo | Definição |
|-------|-----------|
| **BOLA** | Broken Object Level Authorization — acessar recurso de outro usuário manipulando ID |
| **BOPLA** | Broken Object Property Level Authorization — acessar propriedades não autorizadas de um recurso |
| **PDP** | Policy Decision Point — ponto que avalia políticas e toma decisões de autorização |
| **PEP** | Policy Enforcement Point — ponto que executa a decisão do PDP |
| **Security Context** | Objeto imutável que carrega identidade, autorização, request metadata e audit info |
| **Enforcement Central** | Padrão onde o orquestrador Go aplica regras de segurança que workers não podem contornar |
| **Contrato de Comportamento** | Especificação que define como cada runtime SDK deve se comportar |
| **Deny by Default** | Princípio: tudo que não é explicitamente permitido é negado |
| **Fail Securely** | Princípio: em caso de falha, negar acesso (não conceder) |

---

## Apêndice B: Architecture Decision Records (ADRs)

As seguintes ADRs foram criadas para detalhar decisões arquiteturais específicas deste documento:

| ADR | Título | Status | Local |
|-----|--------|--------|-------|
| ADR-001 | Formato e Modelo de Confiança do SecurityContext Token | ✅ Aceito | `docs/adr/001-security-context-token.md` |
| ADR-002 | Policy DSL e Fluxo PDP/PEP | 📄 Proposto | `docs/adr/002-policy-dsl-pdp-pep.md` |
| ADR-003 | Validação de Resposta e Schemas Condicionais | 📄 Proposto | `docs/adr/003-response-validation-conditional-schemas.md` |
| ADR-004 | mTLS Interno e Rotação de Chaves | 📄 Proposto (Fase 4) | `docs/adr/004-internal-mtls-key-rotation.md` |
| ADR-005 | Proteção SSRF no Client HTTP Padrão | ✅ Aceito | `docs/adr/005-ssrf-protection-http-client.md` |

## Apêndice C: Mapeamento Arquitetura → Código

O documento `docs/ARCHITECTURE_TO_CODE_MAP.md` mapeia cada requisito desta arquitetura para:
- Arquivo atual no código
- Gap (o que está faltando ou errado)
- Patch necessário

**Resumo do mapeamento (116 requisitos totais):**

| Status | Quantidade | % |
|--------|-----------|---|
| ✅ Implementado | 15 | 13% |
| ⚠️ Parcial | 10 | 9% |
| ❌ Não implementado | 91 | 78% |

Os 91 requisitos não implementados estão priorizados nas issues do GitHub:
- 🔴 P0 (5 issues): Críticos — resolvem agora
- 🟠 P1 (14 issues): Alta — resolvem nesta sprint
- 🟡 P2 (15 issues): Média — próxima sprint
- 📋 Roadmap: 5 épicos (Fases 1-5)

---

*Este documento é um artefato de arquitetura. Cada seção foi refinada em ADRs específicos (Apêndice B). O mapeamento para código está em `docs/ARCHITECTURE_TO_CODE_MAP.md` (Apêndice C).*

*Próximos passos recomendados:*
1. *Implementar P0 do roadmap (5 issues críticas)*
2. *Implementar P1 do roadmap (14 issues de alta prioridade)*
3. *Iniciar ADR-002 (Policy DSL) — detalhar formato e implementar protótipo*
4. *Iniciar ADR-003 (Response validation) — implementar @Response annotation*
5. *Criar test suite de conformidade para Go SDK*
6. *Iniciar Fase 1 do roadmap (2 sprints)*
