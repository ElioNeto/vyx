# landpage

Um exemplo completo de Landing Page para o framework VYX, demonstrando:

- **Páginas SSR** (Server-Side Rendering) com o worker Node.js usando `@Page`
- **APIs REST** com o worker Go usando `@Route`, `@Auth` e `@Validate`
- **Roteamento baseado em anotações** estáticas no código fonte
- **Protocolo IPC** via Unix Domain Sockets entre Core e Workers

## Arquitetura

```
                     ┌─────────────────────────────┐
                     │        VYX Core (Go)         │
                     │  HTTP Gateway + Orchestrator │
                     └───────┬──────────┬───────────┘
                             │          │
                    ┌────────▼──┐  ┌────▼──────────┐
                    │ go:api    │  │ node:ssr       │
                    │ (Go)      │  │ (Node.js)      │
                    │ REST APIs │  │ SSR Pages      │
                    └───────────┘  └───────────────┘
```

| Worker | ID | Tecnologia | Responsabilidade |
|--------|----|------------|------------------|
| Go API | `go:api` | Go | Endpoints REST: health check, contact form, newsletter |
| Node.js SSR | `node:ssr` | Node.js | Páginas HTML: Home, Features, Pricing, Contact |

## Rotas

### Páginas (Node.js SSR — `node:ssr`)

| Rota | Método | Descrição |
|------|--------|-----------|
| `/` | GET | Home page com hero, features, testimonials e newsletter |
| `/features` | GET | Página detalhada de funcionalidades |
| `/pricing` | GET | Planos e preços |
| `/contact` | GET | Formulário de contato |

### APIs (Go Worker — `go:api`)

| Rota | Método | Auth | Validação | Descrição |
|------|--------|------|-----------|-----------|
| `/api/health` | GET | guest | — | Health check do serviço |
| `/api/contact` | POST | guest | `contact.json` | Envio de formulário de contato |
| `/api/newsletter` | POST | guest | `newsletter.json` | Inscrição na newsletter |
| `/api/newsletter/subscribers` | GET | admin | — | Lista de assinantes (admin) |

## Como executar

### Pré-requisitos

- Go 1.22+
- Node.js 18+
- Linux ou macOS (Windows usa Named Pipes automaticamente)

### Passos

```bash
# 1. A partir da raiz do repositório, faça o build da CLI vyx
cd core && go build -o ../vyx ./cmd/vyx && cd ..

# 2. Entre no diretório do exemplo
cd examples/landpage

# 3. Configure a variável JWT_SECRET
export JWT_SECRET=supersecret

# 4. Inicie o core (ele iniciará ambos os workers automaticamente)
../../vyx dev
```

O core escutará em `http://localhost:8080`.

## Testando

### Acessar as páginas no navegador

Abra as URLs abaixo no seu navegador:

- http://localhost:8080/ — Home page
- http://localhost:8080/features — Funcionalidades
- http://localhost:8080/pricing — Preços
- http://localhost:8080/contact — Contato

### Testar as APIs com curl

#### Health check (público)
```bash
curl http://localhost:8080/api/health
# {"status":"ok","service":"landpage","timestamp":"2026-05-28T10:00:00Z"}
```

#### Enviar formulário de contato
```bash
curl -X POST \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"name":"João Silva","email":"joao@email.com","message":"Quero saber mais sobre o VYX"}' \
  http://localhost:8080/api/contact
# {"message":"Thank you for your message! We'll get back to you soon."}
```

#### Inscrever na newsletter
```bash
curl -X POST \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com"}' \
  http://localhost:8080/api/newsletter
# {"message":"Successfully subscribed to the newsletter!"}
```

#### Listar assinantes (admin apenas)
```bash
curl -H "Authorization: Bearer <TOKEN_COM_ROLE_ADMIN>" \
  http://localhost:8080/api/newsletter/subscribers
# {"subscribers":[{"email":"user@example.com","name":""}],"total":1}
```

### Gerar token JWT para teste

Use [jwt.io](https://jwt.io) com o secredo `supersecret` e payload:

```json
{
  "sub": "user-42",
  "roles": ["user"],
  "exp": 9999999999
}
```

Para testar rotas admin, use `"roles": ["admin"]`.

## Estrutura do projeto

```
landpage/
├── vyx.yaml                     # Manifesto do projeto
├── route_map.json                # Mapa de rotas gerado
├── schemas/
│   ├── contact.json              # Schema do formulário de contato
│   └── newsletter.json           # Schema da newsletter
├── workers/
│   ├── go/
│   │   ├── go.mod                # Módulo Go
│   │   ├── main.go               # Worker Go (REST APIs)
│   │   ├── dial_unix.go          # Conexão UDS (Unix)
│   │   └── dial_windows.go       # Conexão Named Pipe (Windows)
│   └── node/
│       ├── worker.js             # Worker Node.js (SSR pages)
│       └── pages/
│           ├── Home.jsx          # Home page
│           ├── Features.jsx      # Features page
│           ├── Pricing.jsx       # Pricing page
│           └── Contact.jsx       # Contact page
└── README.md                     # Este arquivo
```

## Conceitos demonstrados

1. **Anotações `@Page`**: Declaram rotas de página SSR em workers Node.js
2. **Anotações `@Route`**: Declaram rotas de API em workers Go
3. **Anotações `@Auth`**: Controle de acesso baseado em papéis (roles)
4. **Anotações `@Validate`**: Validação de body com JSON Schema
5. **IPC Binário**: Comunicação Core-Worker via UDS com framing little-endian
6. **Workers Poliglotas**: Go para APIs, Node.js para SSR —同一 protocolo
7. **Armazenamento em memória**: Workers mantêm estado local com acesso concorrente seguro
