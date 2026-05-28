# Dashboard — VYX Example Application

Exemplo completo de um dashboard com três workers demonstrando as capacidades do VYX: **Go** (gerenciamento de usuários), **Python** (análise de dados) e **Node.js** (renderização SSR de páginas).

## O que este exemplo demonstra

- **@Page** — Rotas de páginas renderizadas no servidor (Node.js SSR)
- **@Route** — Rotas de API com workers em Go e Python
- **@Auth** — Controle de acesso baseado em roles (role-based access control)
- **@Validate** — Validação de requisições com JSON Schema
- **JWT** — Autenticação via tokens JWT (simulada para demonstração)
- **3 Worker SDKs** — Go, Python e Node.js se comunicando via IPC

## Arquitetura

```
                    ┌─────────────────────────────────────────────┐
                    │              VYX Core (Go)                  │
                    │  HTTP Gateway + Router + Auth + Validation  │
                    └────┬──────────┬─────────────┬───────────────┘
                         │          │             │
                    UDS  │    UDS   │        UDS  │
                         ▼          ▼             ▼
              ┌────────────┐ ┌──────────┐ ┌──────────────┐
              │  Go:api    │ │Python:   │ │  Node:ssr    │
              │ User Mgmt  │ │Analytics │ │  SSR Pages   │
              └────────────┘ └──────────┘ └──────────────┘
```

| Worker | ID | Tecnologia | Rotas |
|--------|----|------------|-------|
| **Go** | `go:api` | Go 1.25 | `/api/auth/login`, `/api/users/*`, `/api/users/:id/settings` |
| **Python** | `python:analytics` | Python 3.12 | `/api/analytics/overview`, `/api/analytics/revenue`, `/api/analytics/users/growth` |
| **Node.js** | `node:ssr` | Node.js | `/login`, `/dashboard`, `/settings` |

## Rotas

### Páginas (Node.js SSR)
| Rota | Auth | Descrição |
|------|------|-----------|
| `GET /login` | guest | Formulário de login |
| `GET /dashboard` | user, admin | Dashboard com métricas e gráficos |
| `GET /settings` | user, admin | Página de configurações do usuário |

### API de Usuários (Go)
| Rota | Auth | Validação | Descrição |
|------|------|-----------|-----------|
| `POST /api/auth/login` | guest | `login.json` | Autenticação |
| `GET /api/users` | admin | — | Listar todos os usuários |
| `POST /api/users` | admin | `user.json` | Criar novo usuário |
| `GET /api/users/:id` | admin, user | — | Obter dados do usuário |
| `PUT /api/users/:id` | admin, user | `user.json` | Atualizar usuário |
| `GET /api/users/:id/settings` | admin, user | — | Obter configurações |
| `PUT /api/users/:id/settings` | admin, user | `settings.json` | Atualizar configurações |

### API de Analytics (Python)
| Rota | Auth | Descrição |
|------|------|-----------|
| `GET /api/analytics/overview` | admin, user | Métricas agregadas |
| `GET /api/analytics/revenue` | admin | Dados de receita |
| `GET /api/analytics/users/growth` | admin, user | Crescimento de usuários |

## Usuários pré-cadastrados

| Usuário | Senha | Role |
|---------|-------|------|
| `admin` | `admin123` | admin |
| _(qualquer nome)_ | `password` | user |

## Pré-requisitos

- Go 1.22+
- Python 3.12+
- Node.js 18+
- Sistema Unix-like (Linux ou macOS) — Windows usa Named Pipes

## Como executar

```bash
# 1. A partir da raiz do repositório, compile o CLI do VYX
cd core && go build -o ../vyx ./cmd/vyx && cd ..

# 2. Entre no diretório do exemplo
cd examples/dashboard

# 3. Configure a variável de ambiente JWT_SECRET
export JWT_SECRET=supersecret

# 4. Inicie o core (que irá iniciar os 3 workers automaticamente)
../../vyx dev
```

Você verá os três workers se conectando e o core ouvindo em `http://localhost:8080`.

## Testando com o navegador

1. Abra `http://localhost:8080/login`
2. Faça login com **admin** / **admin123**
3. O dashboard será exibido com métricas e gráficos
4. Navegue até a página de **Settings**

## Testando com curl

### Login como admin
```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

Resposta:
```json
{
  "token": "vyx_1_admin",
  "user": {
    "id": "1",
    "name": "Admin User",
    "email": "admin@dashboard.local",
    "role": "admin"
  }
}
```

### Listar usuários (admin apenas)
```bash
TOKEN="vyx_1_admin"
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/users
```

### Analytics (qualquer usuário autenticado)
```bash
TOKEN="vyx_1_admin"
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/analytics/overview
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/analytics/users/growth
```

### Revenue (admin apenas)
```bash
TOKEN="vyx_1_admin"
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/analytics/revenue
```

## Estrutura do projeto

```
dashboard/
├── vyx.yaml                    # Manifesto do projeto VYX
├── route_map.json              # Mapa de rotas pré-gerado
├── schemas/
│   ├── login.json              # Schema de validação para login
│   ├── user.json               # Schema de validação para usuário
│   └── settings.json           # Schema de validação para configurações
├── workers/
│   ├── go/
│   │   ├── go.mod              # Módulo Go
│   │   ├── main.go             # Worker Go (API de usuários)
│   │   ├── dial_unix.go        # Conexão UDS para Unix
│   │   └── dial_windows.go     # Conexão Named Pipe para Windows
│   ├── python/
│   │   └── worker.py           # Worker Python (API de analytics)
│   └── node/
│       ├── worker.js           # Worker Node.js (SSR pages)
│       └── pages/
│           ├── Login.jsx       # Documentação da página de login
│           ├── Dashboard.jsx   # Documentação do dashboard
│           └── Settings.jsx    # Documentação das configurações
└── README.md                   # Este arquivo
```
