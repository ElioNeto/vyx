# ADR-004: mTLS Interno e Rotação de Chaves

## Status
Proposto (Pós-MVP — Fase 4 do roadmap)

## Contexto
O VYX framework utiliza UDS (Unix Domain Sockets) para comunicação entre orquestrador e workers. UDS é inerentemente seguro no nível de transporte (apenas processos no mesmo host podem conectar), mas não fornece:
- Autenticação de worker (quem está conectando?)
- Autorização de worker (o que este worker pode fazer?)
- Proteção contra worker malicioso no mesmo host

Para ambientes multi-host (workers remotos, Issue #59) e para ambientes multi-tenant, é necessário um mecanismo de autenticação e autorização mútua entre orquestrador e workers.

## Decisão

### Dois Modos de Operação

#### Modo 1: UDS Local (Padrão — Fase 1-3)
- Sem mTLS
- Autenticação via handshake com JWT de serviço
- Worker enoga `worker_id` + `capabilities` + `service_token` no handshake
- Orquestrador valida `service_token` (JWT assinado pela CA interna)
- Suficiente para ambientes single-host

#### Modo 2: mTLS Remoto (Fase 4+)
- mTLS obrigatório para conexões TCP (workers remotos)
- Certificados emitidos por CA interna da framework
- Worker apresenta cert + service_token
- Orquestrador valida cert chain + JWT

### Hierarquia de Certificados

```
CA ROOT (offline, guardada em cofre)
  │
  ├── CA Intermediária - Orquestrador
  │     ├── cert: orquestrador-01.vyx.internal
  │     ├── cert: orquestrador-02.vyx.internal
  │     └── ...
  │
  └── CA Intermediária - Workers
        ├── cert: go:api-01.vyx.internal
        ├── cert: go:api-02.vyx.internal
        ├── cert: node:ssr-01.vyx.internal
        ├── cert: python:ml-01.vyx.internal
        └── ...
```

### Geração e Distribuição

```bash
# Geração da CA (uma vez, offline)
vyx cert ca init --output ./certs/ca

# Geração de certificado do orquestrador
vyx cert generate --type orchestrator --name orquestrador-01

# Geração de certificado de worker
vyx cert generate --type worker --id go:api --output ./workers/go/certs
```

### Rotação de Chaves HMAC (JWT)

O orquestrador mantém um **key ring** com múltiplas chaves HMAC:

```
Key Ring (em memória, carregado de variável de ambiente ou Vault)
  │
  ├── Chave Atual (signing key) — usada para ASSINAR novos tokens
  │     ├── kid: "k1"
  │     ├── criada: 2026-05-28
  │     └── expira: 2026-05-29 (24h)
  │
  ├── Chave Anterior (verifying key) — usada para VALIDAR tokens existentes
  │     ├── kid: "k0"
  │     ├── criada: 2026-05-27
  │     └── expira: 2026-05-29 (48h após criação)
  │
  └── (opcional) Chaves históricas — para validação de tokens com TTL longo
```

### Formato do service_token

```json
{
  "sub": "go:api",
  "type": "service",
  "kid": "k1",
  "scopes": ["read:products", "write:orders"],
  "iss": "vyx-ca",
  "aud": ["vyx-orchestrator"],
  "exp": 1745856000,
  "iat": 1745855100,
  "jti": "unique-token-id"
}
```

### Rotação Automática

1. A cada 24h, orquestrador gera nova chave e adiciona ao key ring
2. Chave mais antiga é removida (após garantir que todos os tokens com ela expiraram)
3. Workers recebem novos service_tokens via heartbeat response (token refresh)
4. CLI `vyx cert rotate` força rotação manual

### Integração com Vault

Para produção, o key ring pode ser armazenado no HashiCorp Vault:

```yaml
# vyx.yaml
security:
  key_management:
    provider: vault
    vault:
      address: https://vault.vyx.internal:8200
      path: vyx/secrets/jwt-keys
      role: vyx-orchestrator
    rotation_interval: 24h
```

## Consequências

### Positivas
1. Workers autenticados na conexão (não apenas no handshake)
2. Rotação de chaves sem downtime
3. Service tokens com escopo limitado (princípio do menor privilégio)
4. Suporte a workers remotos (multi-host) quando necessário

### Negativas
1. Complexidade operacional de gerenciamento de certificados
2. Dependência de CA interna (precisa ser mantida segura)
3. mTLS adiciona ~5ms de latência por conexão

### Quando Usar Cada Modo

| Cenário | Modo | Motivo |
|---------|------|--------|
| Desenvolvimento local | UDS + JWT | Simples, sem certificados |
| Produção single-host | UDS + JWT | UDS já é seguro no transporte |
| Produção multi-host | mTLS + JWT | TCP não é seguro sem TLS |
| Multi-tenant | mTLS + JWT | Isolamento adicional entre tenants |
| PCI/SOC2 | mTLS + JWT | Compliance exige mTLS |

## Referências
- SECURITY_ARCHITECTURE.md — Seção 5.2.2
- Issue #59: Remote Workers (TCP)
