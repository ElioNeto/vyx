# ADR-001: Formato e Modelo de Confiança do SecurityContext Token

## Status
Aceito

## Contexto
O VYX framework precisa propagar contexto de segurança (identidade, autorização, metadados) entre o orquestrador Go e workers em Go, Python e Node.js. O contexto não pode ser adulterado pelo worker, mas precisa ser consumível por ele para decisões de negócio (ex: "qual é o tenant ID do usuário?").

## Decisão

### Modelo de Confiança em Duas Camadas

```
┌─────────────────────────────────────────────────────────────────────┐
│                     ORQUESTRADOR GO                                  │
│                                                                      │
│  Cria SecurityContextToken assinado com HMAC-SHA256                  │
│  Contém: {context + nonce + created_at + signature}                 │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  SecurityContextToken = base64url({                          │    │
│  │    "context": { ... dados confiáveis ... },                  │    │
│  │    "nonce": uuid-v4,                                        │    │
│  │    "created_at": timestamp,                                  │    │
│  │    "signature": HMAC-SHA256(context_json, server_secret)    │    │
│  │  })                                                          │    │
│  └─────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     WORKER (Go/Python/Node)                          │
│                                                                      │
│  ● RECEBE → SecurityContext como string opaca (não valida assinatura)│
│  ● USA → claims para lógica de negócio (tenant_id, user_id, roles)  │
│  ● CONFIA → parcialmente. Worker trata claims como "não confiáveis   │
│             para decisões de segurança críticas"                     │
│  ● RETORNA → o mesmo token na resposta (para o orquestrador validar) │
│                                                                      │
│  ⚠️ Worker NÃO DEVE:                                                 │
│    - Usar claims para autorização de segurança (object-level authz   │
│      deve ser feita via policy check no orquestrador)                │
│    - Modificar claims e reenviar (orquestrador detecta adulteração)  │
│                                                                      │
│  ✅ Worker PODE:                                                     │
│    - Usar user_id para queries de banco (com tenant_id injetado)     │
│    - Usar roles para adaptar comportamento de UI/dados               │
│    - Usar correlation_id para logging                                │
│    - Usar tenant_id para chaves de cache                             │
└─────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     ORQUESTRADOR GO (na resposta)                     │
│                                                                      │
│  ● RECEBE → SecurityContextToken de volta na resposta do worker      │
│  ● VALIDA → assinatura, nonce (não reutilizado), created_at (TTL)   │
│  ● REJEITA → se adulterado ou expirado (502 Bad Gateway)            │
│                                                                      │
│  Se o worker tentou modificar o contexto, a assinatura quebra        │
│  e o orquestrador sabe que o worker está comprometido ou bugado.    │
└─────────────────────────────────────────────────────────────────────┘
```

### Formato do SecurityContextToken

```json
{
  "context": {
    "identity": {
      "type": "user|service|anonymous",
      "id": "uuid",
      "username": "joao.silva"
    },
    "authorization": {
      "roles": ["admin"],
      "permissions": ["post:create"],
      "tenant_id": "tenant-abc"
    },
    "request": {
      "correlation_id": "corr-abc",
      "client_ip": "200.201.202.203"
    },
    "audit": {
      "request_id": "req-abc",
      "route": "POST /api/orders"
    }
  },
  "nonce": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "created_at": "2026-05-28T15:00:00Z",
  "ttl_seconds": 120,
  "signature": "HMAC-SHA256(encoded_context + nonce + created_at)"
}
```

### Transporte
- Do orquestrador para o worker: campo `security_context_token` no payload da request IPC
- Do worker para o orquestrador: campo `security_context_token` no payload da response IPC
- Nunca exposto ao cliente final

### Validação no Retorno
Quando o worker retorna o token, o orquestrador:
1. Verifica se `nonce` está na blacklist de nonces já usados (proteção replay)
2. Verifica se `created_at + ttl_seconds` não expirou
3. Recalcula HMAC e compara com `signature`
4. Se qualquer verificação falhar → 502 Bad Gateway + log de segurança

### Nonce Replay Protection
- Orquestrador mantém bloom filter de nonces recentes (5 minutos)
- Nonce é removido após TTL expirar
- Em caso de colisão (nonce repetido) → log crítico + 502

## Consequências

### Positivas
1. Worker pode usar claims sem precisar validar assinatura (mais rápido, menos código)
2. Orquestrador detecta adulteração na resposta
3. Nonce previne replay de tokens
4. TTL curto (120s) limita janela de ataque
5. Sem necessidade de distribuir chave HMAC para workers

### Negativas
1. Worker não valida ativamente — confia no orquestrador pela rede segura (UDS)
2. Se worker for comprometido, pode usar claims indevidamente dentro do seu processo
3. Overhead de ~200 bytes por payload IPC

### Mitigações para Risco de Worker Comprometido
1. Workers rodam isolados (setpgid, namespace)
2. Workers não têm acesso à rede externa por padrão
3. Object-level authorization não depende de claims do worker (é feita via policy check)
4. Auditoria permite detectar abuso

## Referências
- SECURITY_ARCHITECTURE.md — Seção 3
- AUDIT_REPORT.md — Seção 1.4, IPC-001
