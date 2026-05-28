# ADR-003: Validação de Resposta e Schemas Condicionais

## Status
Proposto

## Contexto
O VYX framework precisa garantir que respostas de workers nunca exponham dados não autorizados. O schema de body com `additionalProperties: false` já protege entradas (mass assignment), mas a saída precisa de proteção equivalente — especialmente em cenários onde campos diferentes são retornáveis dependendo da role do usuário.

## Decisão

### Schema de Resposta Obrigatório

TODA rota de leitura (GET) DEVE declarar um schema de resposta via `@Response(nome-do-schema)`.

```go
// @Route(GET /api/users/:id)
// @Auth(roles: ["admin", "user"])
// @Response(user-response)
func handleGetUser(ctx *vyx.Context, req *Request) (*Response, error) {
    user := db.GetUser(req.Params["id"])
    return vyx.JSON(200, user)
}
```

O schema de resposta é validado pelo orquestrador ANTES de enviar ao cliente:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "user-response",
  "type": "object",
  "required": ["id", "name", "email"],
  "additionalProperties": false,
  "properties": {
    "id": { "type": "string", "format": "uuid" },
    "name": { "type": "string", "maxLength": 100 },
    "email": { "type": "string", "format": "email" },
    "created_at": { "type": "string", "format": "date-time" }
  }
}
```

### Schemas Condicionais (Property-level Authorization)

Para campos que só devem ser retornados para roles específicas:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "user-response",
  "type": "object",
  "required": ["id", "name", "email"],
  "additionalProperties": false,
  "properties": {
    "id": { "type": "string", "format": "uuid" },
    "name": { "type": "string", "maxLength": 100 },
    "email": { "type": "string", "format": "email" },
    "created_at": { "type": "string", "format": "date-time" },
    "phone": {
      "type": "string",
      "vyx:authorization": {
        "required_roles": ["admin"],
        "required_permissions": ["user:read-sensitive"]
      }
    },
    "internal_note": {
      "type": "string",
      "vyx:authorization": {
        "required_roles": ["superadmin"]
      }
    }
  }
}
```

### Fluxo de Validação de Saída

```
Worker retorna resposta bruta (map ou struct)
    │
    ▼
Orquestrador recebe resposta do worker
    │
    ▼
Obtém schema de resposta da rota (@Response)
    │
    ▼
Verifica se há campos com vyx:authorization
    │
    ▼
Remove campos que o usuário não tem permissão
    │
    ▼
Valida resposta contra schema base (campos obrigatórios, tipos)
    │
    ▼
Remove campos extras (additionalProperties: false)
    │
    ▼
Aplica security headers
    │
    ▼
Retorna resposta ao cliente
```

### Implementação

O campo `vyx:authorization` é uma extensão do JSON Schema via `$vocabulary`:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "user-response",
  "$vocabulary": {
    "https://vyx.dev/schema/security/v1": true
  },
  ...
}
```

O orquestrador, ao compilar o schema, parseia os campos `vyx:authorization` e cria uma árvore de decisão para cada campo condicional.

### Schema de Resposta como Contrato de API

O schema de resposta também serve como documentação da API. O VYX pode gerar automaticamente:
- Documentação OpenAPI/Swagger
- Tipos TypeScript para o frontend React
- Tipos Python para workers Python

### Regras de Negócio

1. Se `@Response` não for declarado → **erro no build step** (rota não compila)
2. Se worker retornar campo não declarado → **removido** (não causa erro, mas loga WARN)
3. Se worker retornar campo condicional sem permissão → **removido** (silenciosamente)
4. Se worker não retornar campo `required` → **erro 502** (schema violation)
5. Se worker retornar tipo incompatível com schema → **erro 502** (type mismatch)

## Consequências

### Positivas
1. Zero data over-exposure por omissão de schema
2. Property-level authorization sem complexidade no worker
3. Schemas servem como documentação viva
4. Geração automática de tipos para frontend
5. Mudança de schema sem deploy de worker (só atualizar .json)

### Negativas
1. Mais arquivos de schema para manter
2. Overhead de validação de saída (5-50µs por resposta)
3. Schema condicional adiciona complexidade ao compilador de schemas

### Trade-offs
- Schema de resposta obrigatório pode ser visto como "burocracia" por devs → justificar com "segurança por padrão"
- Campos condicionais adicionam complexidade → começar sem eles (Fase 1), adicionar depois (Fase 3)

## Referências
- SECURITY_ARCHITECTURE.md — Seção 4.6
- OWASP API Security Top 10 — API8:2023 (Security Misconfiguration), API9:2023 (Improper Inventory Management)
