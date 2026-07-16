# ADR-006: Módulo de Infraestrutura (IaC) — Provider Model, State Management e Annotation DSL

## Status
Proposto

## Contexto
O VYX framework gerencia atualmente workers poliglotas (Go, Python, Node.js) e utiliza anotações estáticas para descoberta de rotas. Com a maturação do framework, surge a necessidade de também gerenciar programaticamente a infraestrutura de cloud onde esses workers e aplicações são deployados — incluindo bancos de dados, filas, buckets, funções serverless, redes e configurações de DNS.

As principais exigências são:
- Suporte multi-cloud (AWS, GCP, Azure) com modelo de driver pluggável
- Workflow clássico de IaC: init → plan → apply → destroy
- Definição de recursos via anotações estáticas (mesma filosofia do roteamento)
- Estado versionado com locking para operações concorrentes
- Capacidade de exportar para formatos existentes (Terraform HCL, CloudFormation)
- Integração com o ecossistema vyx: CLI, scanner, config, pipelines

## Decisão

### Arquitetura em Camadas (Clean Architecture)

O módulo de infraestrutura segue exatamente a mesma separação em camadas do core:

```
core/
├── domain/infra/       # Entidades + interfaces — ZERO dependências externas
├── application/infra/  # Casos de uso (planner, applier, destroyer)
└── infrastructure/infra/ # Providers concretos, state backends, templaters
```

**Regras:**
- `domain/infra/` não importa nada além da stdlib
- `application/infra/` importa apenas `domain/` (Dependency Inversion)
- `infrastructure/infra/` implementa as interfaces definidas no domain
- A composição (wiring) é feita no `core/cmd/vyx/main.go`, sama como os demais módulos

### Provider Model (Driver Pattern)

Cada cloud provider implementa a interface `Provider`:

```go
type Provider interface {
    ID() ProviderID
    Validate(ctx context.Context, r *Resource) error
    Plan(ctx context.Context, desired, current *Resource) (*ResourceChange, error)
    Create(ctx context.Context, r *Resource) (*Resource, error)
    Read(ctx context.Context, r *Resource) (*Resource, error)
    Update(ctx context.Context, desired, current *Resource) (*Resource, error)
    Delete(ctx context.Context, r *Resource) error
    Capabilities() []ResourceCapability
}
```

**Justificativa:** O pattern de driver (mesmo usado em `database/sql`, `net/http` Handler) permite que cada provider seja desenvolvido, testado e versionado independentemente. O core conhece apenas a interface.

### Isolamento de Providers via Build Tags

Providers são compilados condicionalmente com build tags para não inflar o binário base:

```go
//go:build with_aws
package aws
```

```bash
# Core puro (~15MB)
go build ./cmd/vyx

# Core + AWS provider (~65MB com SDK)
go build -tags with_aws ./cmd/vyx
```

**Justificativa:** O AWS SDK Go v2 adiciona ~50MB ao binário. A maioria dos projetos usa apenas um provider. Build tags mantêm o core leve por padrão.

### Annotation DSL para Infraestrutura

Recursos de infra são declarados via anotações estáticas em arquivos na pasta `infra/`:

```
// @Resource(type: "aws_s3_bucket", id: "assets")
// @Provider(aws)
// @DependsOn(database, cache)
// @Tags(env: "production", team: "platform")
// @Output(bucket_arn)
```

Alternativamente, recursos podem ser definidos diretamente na `vyx.yaml`:

```yaml
infrastructure:
  resources:
    - type: aws_s3_bucket
      id: assets
      provider: aws
      properties:
        bucket: myapp-assets
```

**Justificativa:** A mesma filosofia de anotações usada em `@Route`/`@Auth` é aplicada aqui. Isso mantém consistência e permite que o scanner existente seja estendido com um novo parser (`infra_parser.go`).

### Formato de Estado (State)

O estado é um JSON versionado com serial incremental:

```json
{
  "serial": 42,
  "version": "0.2.0",
  "resources": [
    {
      "id": "assets",
      "type": "aws_s3_bucket",
      "provider": "aws",
      "properties": { ... },
      "outputs": {
        "bucket_arn": "arn:aws:s3:::myapp-assets"
      },
      "state": "created"
    }
  ],
  "metadata": {
    "created_at": "2026-07-16T10:00:00Z",
    "updated_at": "2026-07-16T14:30:00Z"
  }
}
```

**Justificativa:** JSON foi escolhido por ser:
- Legível por humanos (debugging)
- Fácil de parsear em qualquer linguagem
- Compatível com ferramentas de diff (`git diff`)
- Serial incremental permite detectar concorrência (stale serial)

### Backends de Estado

| Backend | Uso | Locking |
|---------|-----|---------|
| **Local** | Desenvolvimento, `.vyx/infra.tfstate` | Lock file (`flock`) |
| **S3 + DynamoDB** | Produção AWS | DynamoDB TTL + conditional put |
| **Consul** | Produção multi-cloud | Consul KV session |
| **HTTP** | Custom/enterprise | A critério do backend |

**Justificativa:** O backend local serve para desenvolvimento. O S3+DynamoDB é o padrão da indústria (Terraform-style). Consul é ideal para ambientes multi-cloud.

### Workflow Init → Plan → Apply → Destroy

O ciclo de vida segue o workflow estabelecido pelo Terraform:

1. **init**: Configura backend de estado, baixa schemas de provider (se aplicável)
2. **plan**: Lê estado atual, lê definições desejadas, calcula diff, exibe mudanças
3. **apply**: Executa mudanças na ordem correta de dependências, persiste novo estado
4. **destroy**: Executa remoções na ordem reversa de dependências, limpa estado

**Justificativa:** Este workflow é o padrão da indústria de IaC. Usuários familiarizados com Terraform/Pulumi reconhecerão imediatamente o fluxo.

### Ordenação por Grafo de Dependências

Recursos podem depender uns dos outros explicitamente (`@DependsOn`) ou implicitamente (referências a outputs). O motor resolve a ordem topológica antes de aplicar:

```
database ──→ cache ──→ api ──→ dns
                 ↘
            function
```

**Justificativa:** Dependências são onipresentes em infraestrutura (e.g., RDS precisa de VPC, Lambda precisa de IAM Role). A resolução automática previne erros de ordenação.

### Templater (Exportação para outros formatos)

O módulo pode exportar definições vyx para outros formatos de IaC:

```
vyx infra export --format terraform > main.tf
vyx infra export --format cloudformation > template.yaml
```

**Justificativa:** Nem todos os ambientes suportam vyx nativamente. A exportação permite integração com pipelines existentes e adoção gradual.

### Outputs Como Interface entre Stacks

Recursos podem declarar outputs (`@Output(nome)`) que são expostos após o apply. Outputs podem ser consumidos por:
- Outros stacks vyx
- CI/CD pipelines
- Arquivos `.env` para workers

```bash
vyx infra output database_host  # → "mydb.cluster-xxx.us-east-1.rds.amazonaws.com"
vyx infra output --format=json  # → todos os outputs em JSON
```

## Consequências

### Positivas

1. **Consistência com o ecossistema vyx**: Mesmas anotações, mesma CLI, mesma arquitetura em camadas
2. **Provider-agnóstico**: Trocar de cloud provider não requer mudança na lógica de aplicação
3. **Binário leve por padrão**: Build tags isolam providers pesados
4. **Workflow familiar**: Usuários de Terraform/Pulumi adotam rapidamente
5. **Estado versionado**: Serial permite detectar e prevenir conflitos concorrentes
6. **Exportável**: Não há vendor lock-in com o formato vyx
7. **Testável**: Providers mock permitem testar plan/apply sem cloud real

### Negativas

1. **Complexidade adicional**: Manter múltiplos providers (AWS, GCP, Azure) requer esforço contínuo
2. **SDK dependency hell**: Cada provider tem seu próprio SDK com versões e breaking changes
3. **Tamanho do binário**: Provider AWS adiciona ~50MB (mitigado por build tags)
4. **Curva de aprendizado**: Usuários precisam aprender o DSL de anotações de infra
5. **Manutenção de compatibilidade**: APIs de cloud mudam frequentemente

### Riscos e Mitigações

| Risco | Mitigação |
|-------|-----------|
| SDKs de cloud desatualizados | Dependabot + testes periódicos com cloud real |
| State locking falha silenciosamente | Timeout configurável + health check do backend |
| Drift não detectado | `vyx infra refresh` dedicado + `vyx infra plan` obrigatório antes do apply |
| Credenciais vazadas no state | Secrets nunca são armazenados no state; apenas outputs não-sensíveis |
| Concorrência em pipelines CI/CD | Lock obrigatório; falha de lock aborta o apply |

## Referências
- ADR-001: SecurityContext Token (mesmo modelo de anotações)
- ADR-002: Policy DSL (mesmo princípio de separação domain/application/infrastructure)
- TECH_SPEC.md — Seção 3 (Clean Architecture)
- ROADMAP.md — Issue #15 (Kubernetes Operator, futuro)
- `docs/plans/infra-module-plan.md` — Plano detalhado de implementação
