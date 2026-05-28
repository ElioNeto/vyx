# ecommerce

Um exemplo completo de comércio eletrônico para o framework **vyx**, demonstrando todos os recursos principais do framework com três workers em três linguagens diferentes — **Go**, **Node.js** e **Python**.

## O que este exemplo demonstra

| Funcionalidade | Descrição |
|---------------|-----------|
| **@Route** | Anotações estáticas para declarar rotas em Go, Node.js e Python |
| **@Auth** | Controle de acesso baseado em funções (roles) |
| **@Validate** | Validação de payload com JSON Schema |
| **Workers multi-linguagem** | Go (produtos/carrinho), Node.js (pedidos/checkout), Python (pagamentos) |
| **IPC via UDS** | Comunicação core-worker via Unix Domain Sockets |
| **JWT** | Autenticação via tokens JWT com claims |

## Arquitetura

| Worker | ID | Linguagem | Rotas |
|--------|----|-----------|-------|
| **Products & Cart API** | `go:api` | Go | `GET/POST /api/products`, `GET/PUT/DELETE /api/products/:id`, `GET /api/cart`, `POST /api/cart/items` |
| **Checkout & Orders** | `node:checkout` | Node.js | `GET /api/orders`, `POST /api/checkout`, `GET /api/orders/:id` |
| **Payments** | `python:payments` | Python | `POST /api/payments/process`, `GET /api/payments/:id` |

### Fluxo de requisição

```
Cliente → Core (Go) → Roteamento (RouteMap) → Worker apropriado → Resposta
```

1. Cliente envia requisição HTTP com token JWT
2. Core valida JWT, extrai claims (sub, roles)
3. Core consulta RouteMap (trie) para encontrar a rota
4. Core seleciona worker do pool (round-robin)
5. Core serializa requisição e envia via UDS
6. Worker processa e retorna resposta via UDS
7. Core devolve resposta HTTP ao cliente

## Pré-requisitos

- Go 1.22+
- Node.js 18+
- Python 3.10+
- Sistema Unix-like (Linux ou macOS) — Windows usa Named Pipes automaticamente

## Configuração

```bash
# 1. Build do CLI vyx (a partir da raiz do repositório)
cd core && go build -o ../vyx ./cmd/vyx && cd ..

# 2. Entre no diretório do exemplo
cd examples/ecommerce

# 3. Defina a chave secreta JWT
export JWT_SECRET=supersecret

# 4. Inicie o core (ele vai iniciar os 3 workers automaticamente)
../../vyx dev
```

O core irá escutar em `http://localhost:8080`.

## Autenticação

Todas as rotas (exceto algumas públicas) exigem um token JWT no header `Authorization: Bearer <token>`.

### Gerando um token JWT de teste

Use o secret `supersecret` e o payload abaixo no [jwt.io](https://jwt.io):

**Token de admin:**
```json
{
  "sub": "admin-1",
  "roles": ["admin"],
  "exp": 9999999999
}
```

**Token de usuário:**
```json
{
  "sub": "user-42",
  "roles": ["user"],
  "exp": 9999999999
}
```

**Token de guest (acesso limitado a leitura de produtos):**
```json
{
  "sub": "guest-1",
  "roles": ["guest"],
  "exp": 9999999999
}
```

## Testando as rotas

### Produtos (Go worker)

```bash
# Listar todos os produtos (guest, user, admin)
curl -H "Authorization: Bearer <TOKEN>" http://localhost:8080/api/products

# Criar produto (admin apenas)
curl -X POST \
  -H "Authorization: Bearer <TOKEN_ADMIN>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Headphone","price":199.99,"category":"electronics","stock":30}' \
  http://localhost:8080/api/products

# Obter produto por ID (guest, user, admin)
curl -H "Authorization: Bearer <TOKEN>" http://localhost:8080/api/products/1

# Atualizar produto (admin apenas)
curl -X PUT \
  -H "Authorization: Bearer <TOKEN_ADMIN>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Smartphone Pro","price":1299.99}' \
  http://localhost:8080/api/products/1

# Deletar produto (admin apenas)
curl -X DELETE -H "Authorization: Bearer <TOKEN_ADMIN>" http://localhost:8080/api/products/3
```

### Carrinho (Go worker)

```bash
# Ver carrinho do usuário (user, admin)
curl -H "Authorization: Bearer <TOKEN_USER>" http://localhost:8080/api/cart

# Adicionar item ao carrinho (user, admin)
curl -X POST \
  -H "Authorization: Bearer <TOKEN_USER>" \
  -H "Content-Type: application/json" \
  -d '{"product_id":"1","quantity":2}' \
  http://localhost:8080/api/cart/items
```

### Checkout e Pedidos (Node.js worker)

```bash
# Listar pedidos do usuário (user, admin)
curl -H "Authorization: Bearer <TOKEN_USER>" http://localhost:8080/api/orders

# Realizar checkout (user, admin)
curl -X POST \
  -H "Authorization: Bearer <TOKEN_USER>" \
  -H "Content-Type: application/json" \
  -d '{"shipping_address":"Rua Exemplo, 123, São Paulo, SP","payment_method":"credit_card","notes":"Entregar somente em horário comercial"}' \
  http://localhost:8080/api/checkout

# Obter pedido por ID (user, admin)
curl -H "Authorization: Bearer <TOKEN_USER>" http://localhost:8080/api/orders/1000
```

### Pagamentos (Python worker)

```bash
# Processar pagamento (user, admin)
curl -X POST \
  -H "Authorization: Bearer <TOKEN_USER>" \
  -H "Content-Type: application/json" \
  -d '{"order_id":"1000","amount":199.98,"payment_method":"credit_card"}' \
  http://localhost:8080/api/payments/process

# Obter detalhes do pagamento (user, admin)
curl -H "Authorization: Bearer <TOKEN_USER>" http://localhost:8080/api/payments/5000
```

## Estrutura do projeto

```
ecommerce/
├── vyx.yaml                  # Manifesto do projeto
├── route_map.json            # Mapa de rotas pré-gerado
├── schemas/
│   ├── product.json          # JSON Schema para produto
│   ├── cart-item.json        # JSON Schema para item do carrinho
│   └── checkout.json         # JSON Schema para checkout
├── workers/
│   ├── go/
│   │   ├── main.go           # Worker Go (produtos + carrinho)
│   │   ├── go.mod            # Módulo Go
│   │   ├── dial_unix.go      # Conexão UDS (Unix)
│   │   └── dial_windows.go   # Conexão Named Pipe (Windows)
│   ├── node/
│   │   └── worker.js         # Worker Node.js (pedidos + checkout)
│   └── python/
│       └── worker.py         # Worker Python (pagamentos)
└── README.md                 # Esta documentação
```

## Protocolo IPC

Todos os workers seguem o mesmo protocolo binário:

```
[4 bytes LE length][1 byte type][payload JSON]
```

Tipos de mensagem:
- `0x01` — Request (Core → Worker)
- `0x02` — Response (Worker → Core)
- `0x03` — Heartbeat (bidirecional)
- `0x05` — Handshake (Worker → Core, na conexão inicial)

## Geração do route_map

Para regenerar o `route_map.json` a partir das anotações no código fonte:

```bash
../../vyx build
```

Isso escaneia todos os workers, identifica as anotações `@Route`, `@Auth` e `@Validate`, e gera o mapa de rotas atualizado.
