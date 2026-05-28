# ADR-002: Policy DSL e Fluxo PDP/PEP

## Status
Proposto

## Contexto
O VYX framework precisa de um modelo de autorização que suporte:
- Route-level authorization (verificado antes do dispatch)
- Function-level authorization (dentro do handler do worker)
- Object-level authorization (ownership checks)
- Property-level authorization (campos condicionais na resposta)
- Tenant isolation

Cada runtime (Go, Python, Node.js) precisa implementar o mesmo modelo de forma consistente.

## Decisão

### Arquitetura PDP/PEP

```
┌─────────────────────────────────────────────────────────────────────┐
│                    ORQUESTRADOR GO (PEP + PDP parcial)               │
│                                                                      │
│  NÍVEL 1: Route-level authorization (PEP no orquestrador)            │
│  ├── Verifica @Auth(roles: [...]) antes do dispatch                  │
│  ├── Verifica scopes do token                                        │
│  └── Verifica tenant isolation básica                                │
│                                                                      │
│  NÍVEL 4: Property-level authorization (PEP no orquestrador)         │
│  ├── Aplica schema de resposta condicional por role                 │
│  └── Remove campos não autorizados                                   │
└─────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│              POLICY SERVICE (Go — PDP central)                       │
│                                                                      │
│  ● Carrega policies de arquivos .policy.json                        │
│  ● Avalia decisões de autorização baseado em:                       │
│    - Usuário (roles, permissions, tenant_id)                        │
│    - Recurso (tipo, ID, tenant_id, owner_id)                        │
│    - Ação (create, read, update, delete, execute)                   │
│    - Contexto (IP, método, horário)                                 │
│  ● Cache de decisões (TTL 5s para reduzir latência)                │
│  ● Retorna: allow | deny | not_applicable                           │
└─────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│              WORKER SDK (Go/Python/Node — PEP local)                 │
│                                                                      │
│  ● NÍVEL 2: Function-level authorization                            │
│    ctx.HasPermission("order:cancel")                                 │
│    ctx.HasRole("superadmin")                                         │
│                                                                      │
│  ● NÍVEL 3: Object-level authorization                              │
│    ctx.CanAccess("order", orderId)                                   │
│    → Chama Policy Service via IPC ou cache local                    │
│                                                                      │
│  ● Ambos delegam a decisão ao PDP (não decidem localmente)          │
└─────────────────────────────────────────────────────────────────────┘
```

### Formato da Policy (.policy.json)

```json
{
  "policy": "order-access",
  "version": "1",
  "description": "Access control for orders",
  "resources": ["order:*"],
  "rules": [
    {
      "id": "rule-001",
      "effect": "allow",
      "actions": ["order:read", "order:list"],
      "conditions": {
        "all": [
          {"match": ["user.tenant_id", "resource.tenant_id"]},
          {
            "any": [
              {"eq": ["user.role", "admin"]},
              {"eq": ["user.id", "resource.owner_id"]}
            ]
          }
        ]
      }
    },
    {
      "id": "rule-002",
      "effect": "deny",
      "actions": ["order:delete"],
      "priority": 100,
      "conditions": {
        "neq": ["user.role", "superadmin"]
      }
    }
  ]
}
```

### DSL de Condições

| Operador | Descrição | Exemplo |
|----------|-----------|---------|
| `eq` | Igualdade | `{"eq": ["user.tenant_id", "resource.tenant_id"]}` |
| `neq` | Diferença | `{"neq": ["user.role", "guest"]}` |
| `in` | Em lista | `{"in": ["user.role", ["admin", "superadmin"]]}` |
| `match` | Regex | `{"match": ["resource.id", "^order-.*$"]}` |
| `all` | AND lógico | `{"all": [cond1, cond2, cond3]}` |
| `any` | OR lógico | `{"any": [cond1, cond2]}` |
| `none` | NOT lógico | `{"none": [cond1]}` |
| `before` | Temporal | `{"before": ["now", "resource.expires_at"]}` |
| `ip_in_cidr` | Rede | `{"ip_in_cidr": ["context.client_ip", "10.0.0.0/8"]}` |

### Fluxo de Avaliação

```
1. Handler do worker chama ctx.CanAccess("order", "order-123")
2. Worker SDK monta requisição de decisão:
   {
     "user": { "id": "user-abc", "roles": ["user"], "tenant_id": "t-1" },
     "resource": { "type": "order", "id": "order-123", "tenant_id": "t-1", "owner_id": "user-abc" },
     "action": "order:read",
     "context": { "time": "2026-05-28T15:00:00Z", "method": "GET" }
   }
3. Se PDP está em cache → resposta imediata
4. Se não → PDP carrega policy, avalia regras, retorna allow/deny
5. Worker SDK recebe decisão e age conforme
```

### Regras de Precedência
1. Regras `deny` com prioridade > 0 têm precedência sobre `allow`
2. Na ausência de regra explícita → `deny` (deny by default)
3. Conflitos resolvem por `deny` (safe default)

### Implementação por Runtime
- Go: PDP + PEP nativos (package `domain/authz`)
- Python: SDK chama PDP via IPC (não implementa PDP próprio)
- Node.js: SDK chama PDP via IPC (não implementa PDP próprio)

Isso garante que a decisão de autorização é SEMPRE tomada pelo PDP central, nunca pelo worker.

## Consequências

### Positivas
1. Modelo de autorização consistente entre todos os runtimes
2. Decisões centralizadas e auditáveis
3. Policies declarativas (mudança sem deploy de código)
4. Cache de decisões reduz latência

### Negativas
1. Latência adicional para chamadas IPC ao PDP (mitigado por cache)
2. Complexidade de implementação do PDP em Go
3. Policies precisam ser versionadas e testadas

### Riscos
- PDP como SPOF → precisa de cache local + fallback (deny por padrão)
- Cache pode servir decisão obsoleta → TTL curto (5s) + invalidação por evento

## Referências
- SECURITY_ARCHITECTURE.md — Seção 6
- OWASP ASVS Nível 1: V1.4, V2.1
