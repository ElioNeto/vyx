# Plano de Implementação: Módulo de Infraestrutura (IaC) para vyx

> **Status:** Proposto  
> **Data:** 2026-07-16  
> **Autor:** Equipe Core  
> **Issues relacionadas:** (a criar)

---

## 1. Visão Geral

### 1.1 Objetivo

Estender o framework vyx com um módulo de **Infrastructure as Code (IaC)** que permite definir, provisionar e gerenciar cloud resources programaticamente — de forma similar ao Terraform, CloudFormation e Pulumi — mas seguindo a filosofia de **anotações estáticas** e **Clean Architecture** do vyx.

### 1.2 O Que Será Possível

```yaml
# vyx.yaml (extensão)
infrastructure:
  backend:
    type: s3
    config:
      bucket: vyx-state-acme
      key: infra/production
      region: us-east-1
  providers:
    - name: aws
      version: "~> 5.0"
```

```python
# infra/database.py
# @Resource(type: "aws_rds_instance")
# @Provider(aws)
# @Tags(env: "production", project: "myapp")
def create_database():
    """PostgreSQL database for the main application."""
    return {
        "engine": "postgres",
        "engine_version": "16.3",
        "instance_class": "db.r6g.large",
        "allocated_storage": 100,
        "db_name": "myapp",
        "username": "db_admin",
        # @Output(host), @Output(port), @Output(arn)
    }
```

```go
// infra/storage.go
// @Resource(type: "aws_s3_bucket", id: "assets")
// @Provider(aws)
// @Tags(env: "production")
type Storage struct {
    // @Input(description: "Bucket name", required: true)
    BucketName string `vyx:"bucket_name"`
    // @Input(description: "Enable versioning", default: true)
    Versioning bool `vyx:"versioning"`
}
```

```bash
vyx infra init      # Inicializa backend de estado
vyx infra plan      # Mostra diff entre estado atual e desejado
vyx infra apply     # Aplica mudanças na infraestrutura
vyx infra destroy   # Destroi todos os recursos gerenciados
vyx infra graph     # Gera grafo de dependências (Mermaid/Graphviz)
vyx infra output    # Mostra outputs dos recursos gerenciados
```

### 1.3 Filosofia de Design

| Princípio | Descrição |
|-----------|-----------|
| **Annotation-first** | A definição de infra usa as mesmas anotações estáticas que o roteamento |
| **Provider pluggable** | Cada cloud provider é um driver que implementa interfaces do domínio |
| **State versionado** | Estado versionado com suporte a locking, backend S3/local/consul |
| **Plan-Apply-Destroy** | Workflow clássico de IaC com dry-run |
| **Dependency graph** | Resolução automática de dependências entre recursos |
| **Codegen opcional** | Capacidade de gerar CloudFormation/Terraform HCL do modelo vyx |

---

## 2. Arquitetura

### 2.1 Clean Architecture Layers

Seguindo o mesmo padrão do core:

```
core/domain/infra/         # Entidades, interfaces — ZERO dependências externas
core/application/infra/    # Casos de uso (plan, apply, destroy)
core/infrastructure/infra/ # Providers concretos, state backends
scanner/infra_parser.go    # Parse de anotações @Resource, @Provider, etc.
cmd/vyx/infra.go           # Subcomandos CLI: vyx infra plan/apply/destroy/init/graph
```

### 2.2 Diagrama de Camadas

```
┌─────────────────────────────────────────────────────────────┐
│                      CLI (cmd/vyx/infra.go)                  │
│  vyx infra init | plan | apply | destroy | graph | output   │
└──────────────────────┬──────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────┐
│              Application Layer (use cases)                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────┐ │
│  │ Planner  │  │ Applier  │  │Destroyer │  │ StateManager│ │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └──────┬──────┘ │
└───────┼──────────────┼─────────────┼────────────────┼───────┘
        │              │             │                │
┌───────▼──────────────▼─────────────▼────────────────▼───────┐
│              Domain Layer (interfaces + entities)            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────┐ │
│  │ Resource │  │ Provider │  │   State  │  │   Stack     │ │
│  │  entity  │  │interface │  │ interface│  │  (collection)│ │
│  └──────────┘  └────┬─────┘  └────┬─────┘  └─────────────┘ │
└──────────────────────┼─────────────┼────────────────────────┘
                       │             │
┌──────────────────────▼─────────────▼────────────────────────┐
│            Infrastructure Layer (implementations)            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────┐ │
│  │AWS Prov. │  │ GCP Prov.│  │Azure Pr. │  │State Backend│ │
│  │          │  │          │  │          │  │ (S3/Local)  │ │
│  └──────────┘  └──────────┘  └──────────┘  └─────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Domain Layer (`core/domain/infra/`)

### 3.1 `resource.go` — Entidade Resource

```go
// ResourceType identifica um tipo de recurso de infraestrutura.
type ResourceType string

// ResourceID é um identificador único dentro de um stack.
type ResourceID string

// ResourceState representa o estado atual de provisionamento.
type ResourceState string

const (
    ResourceStatePending   ResourceState = "pending"
    ResourceStateCreating  ResourceState = "creating"
    ResourceStateCreated   ResourceState = "created"
    ResourceStateUpdating  ResourceState = "updating"
    ResourceStateDeleted   ResourceState = "deleted"
    ResourceStateFailed    ResourceState = "failed"
)

// Resource é a entidade central do módulo de infra.
type Resource struct {
    ID           ResourceID            `json:"id"`
    Type         ResourceType          `json:"type"`
    ProviderName string                `json:"provider"`
    Properties   map[string]any        `json:"properties"`
    Inputs       map[string]any        `json:"inputs"`       // valores fornecidos pelo usuário
    Outputs      map[string]string     `json:"outputs"`      // valores retornados pelo provider
    DependsOn    []ResourceID          `json:"depends_on"`    // dependências explícitas
    Tags         map[string]string     `json:"tags"`
    Metadata     ResourceMetadata      `json:"metadata"`
    State        ResourceState         `json:"state"`
}

type ResourceMetadata struct {
    SourceFile  string `json:"source_file"`
    SourceLine  int    `json:"source_line"`
    Description string `json:"description"`
}
```

### 3.2 `provider.go` — Interface Provider

```go
// ProviderID identifica um provedor de cloud.
type ProviderID string

// Provider é o contrato que cada cloud driver deve implementar.
type Provider interface {
    // ID retorna o identificador do provider (e.g., "aws", "gcp").
    ID() ProviderID

    // Validate valida as propriedades de um recurso antes do plan.
    Validate(ctx context.Context, r *Resource) error

    // Plan determina o que precisa mudar: create, update, delete, noop.
    Plan(ctx context.Context, desired, current *Resource) (*ResourceChange, error)

    // Create provisiona um novo recurso.
    Create(ctx context.Context, r *Resource) (*Resource, error)

    // Read consulta o estado atual de um recurso existente.
    Read(ctx context.Context, r *Resource) (*Resource, error)

    // Update modifica um recurso existente.
    Update(ctx context.Context, desired, current *Resource) (*Resource, error)

    // Delete destrói um recurso.
    Delete(ctx context.Context, r *Resource) error

    // Capabilities retorna os tipos de recurso suportados.
    Capabilities() []ResourceCapability
}

type ResourceCapability struct {
    Type        ResourceType       `json:"type"`
    Description string             `json:"description"`
    InputSchema map[string]SchemaField `json:"input_schema"`
    OutputFields []string          `json:"output_fields"`
}

type SchemaField struct {
    Type        string      `json:"type"`
    Required    bool        `json:"required"`
    Description string      `json:"description"`
    Default     any         `json:"default,omitempty"`
}

// ChangeType descreve a operação necessária.
type ChangeType string

const (
    ChangeCreate ChangeType = "create"
    ChangeUpdate ChangeType = "update"
    ChangeDelete ChangeType = "delete"
    ChangeNoop   ChangeType = "noop"
)

// ResourceChange representa uma única mudança planejada.
type ResourceChange struct {
    ResourceID   ResourceID          `json:"resource_id"`
    ChangeType   ChangeType          `json:"change_type"`
    ResourceType ResourceType        `json:"resource_type"`
    Diff         map[string]DiffValue `json:"diff,omitempty"` // para updates
}

type DiffValue struct {
    Old any `json:"old"`
    New any `json:"new"`
}

// ProviderRegistry gerencia o registro de providers.
type ProviderRegistry interface {
    Register(p Provider) error
    Get(id ProviderID) (Provider, error)
    List() []ProviderID
}
```

### 3.3 `state.go` — Interface de Estado

```go
// StateID é um identificador único para um estado de infraestrutura.
type StateID string

// State representa o snapshot completo da infraestrutura gerenciada.
type State struct {
    ID          StateID              `json:"id"`
    Serial      uint64               `json:"serial"`
    Version     string               `json:"version"` // vyx version
    Resources   []*Resource          `json:"resources"`
    ProviderIDs []ProviderID         `json:"providers"`
    Metadata    StateMetadata        `json:"metadata"`
    CreatedAt   time.Time            `json:"created_at"`
    UpdatedAt   time.Time            `json:"updated_at"`
}

// Backend é a interface para persistência de estado.
type Backend interface {
    // Init inicializa o backend (cria bucket, tabela, etc.).
    Init(ctx context.Context) error

    // Lock adquire um lock para evitar operações concorrentes.
    Lock(ctx context.Context, info LockInfo) error

    // Unlock libera o lock.
    Unlock(ctx context.Context, info LockInfo) error

    // Get recupera o estado atual.
    Get(ctx context.Context) (*State, error)

    // Put persiste um novo estado.
    Put(ctx context.Context, state *State) error

    // Delete remove o estado (após destroy).
    Delete(ctx context.Context) error
}

type LockInfo struct {
    ID        string    `json:"id"`
    Operation string    `json:"operation"`
    Who       string    `json:"who"`
    CreatedAt time.Time `json:"created_at"`
}
```

### 3.4 `stack.go` — Coleção de Recursos

```go
// Stack é uma coleção nomeada de recursos de infraestrutura.
type Stack struct {
    Name        string              `json:"name"`
    Description string              `json:"description"`
    Resources   []*Resource         `json:"resources"`
    ProviderRefs []ProviderID       `json:"provider_refs"`
    BackendCfg  BackendConfig       `json:"backend_config"`
}

// BackendConfig descreve como e onde o estado é armazenado.
type BackendConfig struct {
    Type   string         `yaml:"type"`    // "local", "s3", "consul"
    Config map[string]any `yaml:"config"`
}
```

### 3.5 `plan.go` — Resultado do Plan

```go
// PlanResult é o resultado completo de uma operação de planejamento.
type PlanResult struct {
    StackName    string             `json:"stack_name"`
    Serial       uint64             `json:"serial"`
    Changes      []*ResourceChange  `json:"changes"`
    Summary      PlanSummary        `json:"summary"`
    CreatedAt    time.Time          `json:"created_at"`
}

type PlanSummary struct {
    ToCreate int `json:"to_create"`
    ToUpdate int `json:"to_update"`
    ToDelete int `json:"to_delete"`
    Noop     int `json:"noop"`
    Errors   int `json:"errors"`
}
```

### 3.6 `config.go` — Configuração de Infra

```go
// Config é a seção de infraestrutura da vyx.yaml.
type Config struct {
    Backend   BackendConfig    `yaml:"backend"`
    Providers []ProviderConfig `yaml:"providers"`
    Stacks    []StackConfig    `yaml:"stacks"`
    DefaultTags map[string]string `yaml:"default_tags"`
}

type ProviderConfig struct {
    Name    string `yaml:"name"`
    Version string `yaml:"version"`
    Config  map[string]any `yaml:"config"` // credenciais, região, etc.
}

type StackConfig struct {
    Name    string   `yaml:"name"`
    Sources []string `yaml:"sources"` // diretórios com definições de recurso
}
```

---

## 4. Application Layer (`core/application/infra/`)

### 4.1 `planner.go` — Motor de Planejamento

```go
// Planner calcula o diff entre estado desejado e atual.
type Planner struct {
    registry ProviderRegistry
    backend  state.Backend
}

// Plan gera um PlanResult completo.
func (p *Planner) Plan(ctx context.Context, stack *Stack) (*PlanResult, error) {
    // 1. Carrega estado atual do backend
    // 2. Para cada recurso no stack:
    //    a. Provider.Plan(desired, current)
    //    b. Acumula ResourceChange
    // 3. Detecta recursos no estado atual mas não no desejado (delete)
    // 4. Ordena mudanças por dependência topológica
    // 5. Retorna PlanResult com summary
}
```

### 4.2 `applier.go` — Motor de Aplicação

```go
// Applier executa as mudanças planejadas.
type Applier struct {
    registry ProviderRegistry
    backend  state.Backend
    logger   *zap.Logger
}

// Apply executa um plano, recurso por recurso.
func (a *Applier) Apply(ctx context.Context, plan *PlanResult, stack *Stack) (*State, error) {
    // 1. Lock do backend
    // 2. Para cada ResourceChange (ordenado por dependência):
    //    a. Create → Provider.Create
    //    b. Update → Provider.Update
    //    c. Delete → Provider.Delete
    //    d. Atualiza outputs no estado
    // 3. Persiste novo estado
    // 4. Unlock
}
```

### 4.3 `destroyer.go` — Motor de Destruição

```go
// Destroyer remove todos os recursos gerenciados.
type Destroyer struct {
    registry ProviderRegistry
    backend  state.Backend
}

// Destroy executa a destruição em ordem reversa de dependência.
func (d *Destroyer) Destroy(ctx context.Context, stack *Stack) error {
    // 1. Carrega estado atual
    // 2. Para cada recurso (ordem reversa de dependência):
    //    a. Provider.Delete
    // 3. Remove estado
}
```

### 4.4 `graph.go` — Gerador de Grafo

```go
// GraphGenerator produz representações visuais das dependências.
type GraphGenerator struct{}

// GenerateMermaid produz um diagrama Mermaid.
func (g *GraphGenerator) GenerateMermaid(resources []*Resource) (string, error) {}

// GenerateGraphviz produz um DOT file.
func (g *GraphGenerator) GenerateGraphviz(resources []*Resource) (string, error) {}
```

### 4.5 `output.go` — Coletor de Outputs

```go
// OutputCollector agrega valores de output dos recursos.
type OutputCollector struct {
    backend state.Backend
}

// Outputs retorna todos os outputs do estado atual.
func (c *OutputCollector) Outputs(ctx context.Context, stackName string) (map[string]map[string]string, error) {
    // resource_id -> { output_name -> value }
}
```

---

## 5. Infrastructure Layer (`core/infrastructure/infra/`)

### 5.1 `core/infrastructure/infra/providers/aws/` — Provider AWS

```
infrastructure/infra/providers/aws/
├── provider.go           # AWS provider (implementa domain.Provider)
├── ec2.go                # aws_instance
├── rds.go                # aws_db_instance
├── s3.go                 # aws_s3_bucket
├── lambda.go             # aws_lambda_function
├── iam.go                # aws_iam_role, aws_iam_policy
├── vpc.go                # aws_vpc, aws_subnet, etc.
├── apigateway.go         # aws_api_gateway_*
├── cloudfront.go         # aws_cloudfront_distribution
├── route53.go            # aws_route53_zone, aws_route53_record
├── elasticache.go        # aws_elasticache_cluster
├── sqs.go                # aws_sqs_queue
├── sns.go                # aws_sns_topic
├── client.go             # AWS SDK client factory
└── credentials.go        # Resolução de credenciais (env, file, IAM role)
```

**Implementação de exemplo para `ec2.go`:**

```go
func (p *AWSProvider) Capabilities() []domain.ResourceCapability {
    return []domain.ResourceCapability{
        {
            Type: "aws_instance",
            Description: "Provisions an EC2 instance",
            InputSchema: map[string]SchemaField{
                "ami":              {Type: "string", Required: true, Description: "AMI ID"},
                "instance_type":    {Type: "string", Required: true, Description: "Instance type"},
                "subnet_id":        {Type: "string", Required: false},
                "key_name":         {Type: "string", Required: false},
                "security_groups":  {Type: "list(string)", Required: false},
                "user_data":        {Type: "string", Required: false},
                "ebs_optimized":    {Type: "bool", Required: false, Default: false},
                "tags":             {Type: "map(string)", Required: false},
            },
            OutputFields: []string{"id", "arn", "public_ip", "private_ip", "public_dns"},
        },
    }
}
```

### 5.2 `core/infrastructure/infra/providers/gcp/` — Provider GCP (fase 2)

```
infrastructure/infra/providers/gcp/
├── provider.go           # GCP provider
├── compute.go            # gcp_compute_instance
├── sql.go                # gcp_cloud_sql
├── storage.go            # gcp_storage_bucket
├── functions.go          # gcp_cloud_function
├── iam.go                # gcp_iam_*
└── client.go             # GCP SDK client factory
```

### 5.3 `core/infrastructure/infra/providers/azure/` — Provider Azure (fase 3)

```
infrastructure/infra/providers/azure/
├── provider.go
├── vm.go
├── sql.go
├── storage.go
├── functions.go
├── aks.go
└── client.go
```

### 5.4 `core/infrastructure/infra/state/` — Backends de Estado

```
infrastructure/infra/state/
├── backend.go              # Factory function
├── local.go                # Estado em arquivo local (.vyx/terraform.tfstate)
├── s3.go                   # Estado em S3 + DynamoDB locking
├── consul.go               # Estado em Consul KV
├── http.go                 # Estado via API REST customizada
└── backend_test.go
```

### 5.5 `core/infrastructure/infra/templater/` — Exportação para outros formatos

```
infrastructure/infra/templater/
├── terraform.go          # Gera .tf files do modelo vyx (fase 2)
├── cloudformation.go     # Gera templates CloudFormation (fase 2)
└── pulumi.go             # Gera código Pulumi (fase 3)
```

Isso permite que usuários comecem com vyx e migrem para Terraform se necessário, ou usem vyx como um "gerador de configuração" para outras ferramentas.

---

## 6. Scanner & Annotations

### 6.1 `scanner/infra_parser.go` — Parser de Anotações de Infra

```go
// InfraResource representa um recurso descoberto por anotação.
type InfraResource struct {
    Type         ResourceType        `json:"type"`
    ProviderName string              `json:"provider"`
    ID           string              `json:"id,omitempty"` // opcional, default = filename+linha
    Properties   map[string]any      `json:"properties"`
    DependsOn    []string            `json:"depends_on,omitempty"`
    Tags         map[string]string   `json:"tags,omitempty"`
    Outputs      []string            `json:"outputs,omitempty"`
    File         string              `json:"-"`
    Line         int                 `json:"-"`
}

// Annotation format (multi-lang):
//
//   // @Resource(type: "aws_s3_bucket", id: "assets")
//   // @Provider(aws)
//   // @DependsOn(database, cache)
//   // @Tags(env: "production", team: "platform")
//   // @Output(bucket_arn)
//   // @Output(bucket_domain)
//
func ParseInfraFiles(dir string) ([]InfraResource, []AnnotationError) { ... }
```

### 6.2 Formatos por Linguagem

**Go:**
```go
// Package infra defines infrastructure resources.
//
// @Resource(type: "aws_s3_bucket", id: "assets")
// @Provider(aws)
// @Tags(env: "production")
package infra

// @Output(bucket_arn)
// @Output(bucket_domain_name)
type AssetsBucket struct {
    BucketName string `vyx:"bucket"`         // @Input(required: true)
    Versioning bool   `vyx:"versioning"`     // @Input(default: true)
    Encryption  string `vyx:"encryption"`    // @Input(default: "AES256")
}
```

**Python:**
```python
# infra/database.py
# @Resource(type: "aws_rds_instance")
# @Provider(aws)
# @Tags(env: "production")
# @DependsOn(vpc)
# @Output(host), @Output(port), @Output(arn)
def create_database():
    return {
        "engine": "postgres",
        "engine_version": "16.3",
        "instance_class": "db.r6g.large",
        "allocated_storage": 100,
        "db_name": "myapp",
    }
```

**YAML (vyx.yaml):**
```yaml
infrastructure:
  resources:
    - type: aws_s3_bucket
      id: assets
      provider: aws
      tags:
        env: production
      properties:
        bucket: myapp-assets-production
        versioning: true
        encryption: AES256
```

### 6.3 `scanner/generator.go` — Extensão

O `generator.go` existente será estendido para também gerar um `infra_map.json`:

```json
{
  "stacks": [
    {
      "name": "default",
      "resources": [
        {
          "id": "assets",
          "type": "aws_s3_bucket",
          "provider": "aws",
          "properties": { ... },
          "depends_on": [],
          "tags": { "env": "production" },
          "outputs": ["bucket_arn", "bucket_domain_name"]
        }
      ]
    }
  ]
}
```

---

## 7. CLI Integration

### 7.1 `cmd/vyx/infra.go` — Novo Arquivo de Subcomandos

```go
func init() {
    rootCmd.AddCommand(infraCmd)
    infraCmd.AddCommand(infraInitCmd)
    infraCmd.AddCommand(infraPlanCmd)
    infraCmd.AddCommand(infraApplyCmd)
    infraCmd.AddCommand(infraDestroyCmd)
    infraCmd.AddCommand(infraGraphCmd)
    infraCmd.AddCommand(infraOutputCmd)
}
```

### 7.2 Comandos

```bash
vyx infra init     # Inicializa backend de estado
vyx infra plan     # Mostra o plano de mudanças (exit 2 se houver mudanças)
vyx infra apply    # Aplica as mudanças (approval interativo ou -auto-approve)
vyx infra destroy  # Destroi toda a infra (requer confirmação)
vyx infra graph    # Gera grafo Mermaid/Graphviz (--format mermaid|dot)
vyx infra output   # Lista outputs do estado atual

# Flags comuns
--stack=name        # Nome do stack (default: "default")
--state=path        # Caminho do estado (default: .vyx/infra.tfstate)
--auto-approve      # Pula confirmação em apply/destroy
--format=json       # Saída em JSON (para scripting)
--var key=value     # Variáveis de entrada
```

### 7.3 Exemplos de Uso

```bash
# Inicializar
cd myapp
vyx infra init

# Ver o que vai mudar
vyx infra plan

# Aplicar
vyx infra apply

# Usar em CI/CD (non-interactive)
vyx infra plan --format=json
vyx infra apply --auto-approve

# Outputs para usar em outras ferramentas
vyx infra output --format=json
```

---

## 8. Configuração (vyx.yaml)

### 8.1 Extensão do Domain Config

Em `core/domain/config/config.go`, adicionar:

```go
type Config struct {
    Project  ProjectConfig  `yaml:"project"`
    Workers  []WorkerConfig `yaml:"workers"`
    Security SecurityConfig `yaml:"security"`
    IPC      IPCConfig      `yaml:"ipc"`
    Build    BuildConfig    `yaml:"build"`
    Infra    InfraConfig    `yaml:"infrastructure"` // NOVO
}
```

### 8.2 InfraConfig

```go
type InfraConfig struct {
    Backend     BackendConfig     `yaml:"backend"`
    Providers   []ProviderConfig  `yaml:"providers"`
    Stacks      []StackConfig     `yaml:"stacks"`
    DefaultTags map[string]string `yaml:"default_tags"`
}

type BackendConfig struct {
    Type   string         `yaml:"type"`
    Config map[string]any `yaml:"config"`
}

type ProviderConfig struct {
    Name    string         `yaml:"name"`
    Version string         `yaml:"version"`
    Config  map[string]any `yaml:"config"`
}

type StackConfig struct {
    Name    string   `yaml:"name"`
    Sources []string `yaml:"sources"`
}
```

---

## 9. Integração com Sistema Existente

### 9.1 Composição no Core (`core/cmd/vyx/main.go`)

```go
// setupInfraServices wires up the infrastructure module.
func setupInfraServices(cfg *doamincfg.Config, log *zap.Logger) *infraapp.Orchestrator {
    if cfg.Infra == nil {
        return nil // infra não configurada
    }

    // Provider registry
    registry := infra.NewProviderRegistry()
    for _, pcfg := range cfg.Infra.Providers {
        switch pcfg.Name {
        case "aws":
            awsProvider := awsprovider.New(pcfg.Config)
            registry.Register(awsProvider)
        }
    }

    // State backend
    backend := infrastate.NewBackend(cfg.Infra.Backend)

    // App services
    planner := infraapp.NewPlanner(registry, backend)
    applier := infraapp.NewApplier(registry, backend, log)
    destroyer := infraapp.NewDestroyer(registry, backend)

    return infraapp.NewOrchestrator(planner, applier, destroyer, backend, log)
}
```

### 9.2 Pipeline de Build

O `vyx build` ganha um passo extra:

```
vyx build:
  1. Scanner de rotas  → route_map.json
  2. Scanner de infra  → infra_map.json  (NOVO)
  3. Validação cruzada
```

### 9.3 Workspace (`.vyx/` directory)

```
.vyx/
├── infra/
│   ├── terraform.tfstate    # estado local (default)
│   ├── terraform.tfstate.backup
│   └── locks/               # lock files
└── ...
```

---

## 10. Fases de Implementação

### Fase 1 — Fundação (Semanas 1-3)

| Atividade | Artefatos |
|-----------|-----------|
| Domain entities | `resource.go`, `provider.go`, `state.go`, `stack.go`, `plan.go` |
| Provider interface + registry | `provider.go` (interface), `registry.go` |
| State backend interface + local impl | `backend.go`, `local.go` |
| Application planner | `planner.go` (diff engine) |
| Application applier | `applier.go` (apply loop) |
| Application destroyer | `destroyer.go` (reverse-order destroy) |
| CLI infra init/plan/apply/destroy | `cmd/vyx/infra.go` |
| Config extension | `domain/config/config.go` + `InfraConfig` |
| Testes unitários | Cobertura ≥ 70% |

**Entregáveis:** `vyx infra init/plan/apply/destroy` funcionando com backend local e um provider mock.

### Fase 2 — Provider AWS (Semanas 4-6)

| Atividade | Artefatos |
|-----------|-----------|
| AWS provider scaffold | `providers/aws/provider.go`, `client.go` |
| Resource: S3 bucket | `providers/aws/s3.go` |
| Resource: EC2 instance | `providers/aws/ec2.go` |
| Resource: RDS instance | `providers/aws/rds.go` |
| Resource: IAM role/policy | `providers/aws/iam.go` |
| Resource: VPC/subnet/security group | `providers/aws/vpc.go` |
| Resource: Lambda function | `providers/aws/lambda.go` |
| Resource: SQS queue | `providers/aws/sqs.go` |
| Resource: SNS topic | `providers/aws/sns.go` |
| S3 state backend | `state/s3.go` + DynamoDB locking |
| Scanner infra_parser | `scanner/infra_parser.go` |
| Testes de integração AWS (com LocalStack) | `integration/infra/` |
| Documentação | `docs/infra/` |

**Entregáveis:** 10+ recursos AWS, backend S3, scanner funcional, testes com LocalStack.

### Fase 3 — Provider GCP + Recursos Avançados (Semanas 7-8)

| Atividade | Artefatos |
|-----------|-----------|
| GCP provider scaffold | `providers/gcp/provider.go` |
| Resource: Compute Instance | `providers/gcp/compute.go` |
| Resource: Cloud SQL | `providers/gcp/sql.go` |
| Resource: Storage Bucket | `providers/gcp/storage.go` |
| Resource: Cloud Functions | `providers/gcp/functions.go` |
| Resource: IAM | `providers/gcp/iam.go` |
| Consul state backend | `state/consul.go` |
| Graph generator | `graph.go` (Mermaid + Graphviz) |
| Output collector | `output.go` |
| `vyx infra graph/output` CLI | Extensão do `cmd/vyx/infra.go` |

**Entregáveis:** Provider GCP, state backend Consul, CLI graph/output.

### Fase 4 — Provider Azure + Templater (Semanas 9-10)

| Atividade | Artefatos |
|-----------|-----------|
| Azure provider scaffold | `providers/azure/provider.go` |
| Resource: VM | `providers/azure/vm.go` |
| Resource: SQL Database | `providers/azure/sql.go` |
| Resource: Storage Account | `providers/azure/storage.go` |
| Resource: AKS | `providers/azure/aks.go` |
| Terraform HCL templater | `templater/terraform.go` |
| CloudFormation templater | `templater/cloudformation.go` |
| HTTP state backend | `state/http.go` |
| Testes de integração cross-provider | `integration/infra/` |

**Entregáveis:** Provider Azure, exportação para Terraform/CloudFormation, testes cross-provider.

### Fase 5 — Polimento e DX (Semanas 11-12)

| Atividade | Artefatos |
|-----------|-----------|
| Output interativo colorido | Tabelas, diff colorido no terminal |
| Progress bars para apply/destroy | Barra de progresso por recurso |
| Auto-approve mode | `--auto-approve` funcional |
| JSON output mode | `--format=json` para scripting |
| Error recovery | Retry com backoff em falhas de API |
| Import existing resources | `vyx infra import <type> <id>` |
| State migration | `vyx infra state mv` / `vyx infra state rm` |
| Documentação completa | `docs/infra/` |
| Exemplos de projetos | `examples/infra/` |

**Entregáveis:** DX polida, comando `import`, migração de estado, exemplos completos.

---

## 11. Estrutura de Arquivos Final

```
core/
├── domain/
│   └── infra/
│       ├── resource.go
│       ├── provider.go
│       ├── state.go
│       ├── stack.go
│       ├── plan.go
│       ├── config.go
│       ├── errors.go
│       └── resource_test.go
├── application/
│   └── infra/
│       ├── planner.go
│       ├── applier.go
│       ├── destroyer.go
│       ├── graph.go
│       ├── output.go
│       ├── orchestrator.go
│       ├── planner_test.go
│       ├── applier_test.go
│       └── destroyer_test.go
├── infrastructure/
│   └── infra/
│       ├── providers/
│       │   ├── aws/
│       │   │   ├── provider.go
│       │   │   ├── client.go
│       │   │   ├── credentials.go
│       │   │   ├── ec2.go
│       │   │   ├── rds.go
│       │   │   ├── s3.go
│       │   │   ├── lambda.go
│       │   │   ├── iam.go
│       │   │   ├── vpc.go
│       │   │   ├── apigateway.go
│       │   │   ├── sqs.go
│       │   │   ├── sns.go
│       │   │   ├── route53.go
│       │   │   ├── elasticache.go
│       │   │   └── aws_test.go
│       │   ├── gcp/
│       │   │   ├── provider.go
│       │   │   ├── client.go
│       │   │   ├── compute.go
│       │   │   ├── sql.go
│       │   │   ├── storage.go
│       │   │   ├── functions.go
│       │   │   ├── iam.go
│       │   │   └── gcp_test.go
│       │   └── azure/
│       │       ├── provider.go
│       │       ├── client.go
│       │       ├── vm.go
│       │       ├── sql.go
│       │       ├── storage.go
│       │       ├── aks.go
│       │       └── azure_test.go
│       ├── state/
│       │   ├── backend.go
│       │   ├── local.go
│       │   ├── s3.go
│       │   ├── consul.go
│       │   ├── http.go
│       │   └── backend_test.go
│       └── templater/
│           ├── terraform.go
│           ├── cloudformation.go
│           ├── terraform_test.go
│           └── cloudformation_test.go
scanner/
├── infra_parser.go
├── infra_parser_test.go
└── ... (existing files)
cmd/vyx/
└── infra.go (NOVO)
```

---

## 12. Riscos e Mitigações

| Risco | Impacto | Probabilidade | Mitigação |
|-------|---------|---------------|-----------|
| Dependência pesada de SDKs de cloud (AWS SDK Go adiciona ~50MB) | Alto | Alta | Usar build tags (`//go:build with_aws`); providers como plugins separados |
| State locking concorrente | Médio | Média | Usar DynamoDB (AWS), consul session, etc. com timeout configurável |
| API rate limiting das clouds | Médio | Alta | Retry com exponential backoff + jitter; throttle configurável |
| Drift detection complexo | Alto | Média | Refresh antes do plan; `vyx infra refresh` dedicado |
| Segurança de credenciais | Alto | Média | Suporte a AWS STS, GCP workload identity, Azure MSI; variáveis de ambiente; não salvar secrets no state |
| Manutenção de múltiplos providers | Alto | Alta | Foco inicial em AWS; GCP e Azure depois; separação por build tags |
| Compatibilidade com versões de API | Médio | Alta | Versionamento de provider; testes periódicos com clouds reais |

---

## 13. Métricas de Sucesso

1. **Fase 1:** `go test ./domain/infra/... ./application/infra/...` passa com ≥ 80% cobertura
2. **Fase 2:** `vyx infra plan/apply` cria/gerencia bucket S3 real (testado com LocalStack)
3. **Fase 3:** Suporte a 2 cloud providers (AWS + GCP)
4. **Fase 4:** Suporte a 3 cloud providers + exportação Terraform/CloudFormation
5. **Fase 5:** `vyx infra import` funcional para recursos existentes
6. **Documentação:** Guias de início rápido para cada provider
7. **Benchmark:** Plan com 50 recursos em < 1s

---

## 14. Próximos Passos Imediatos

1. ✅ Validar este plano com a equipe
2. 🔲 Criar ADR formal (ADR-006) documentando as decisões arquiteturais
    - Provider model vs. code generation approach
    - State format (JSON versionado)
    - Locking semantics
3. 🔲 Criar GitHub issues para cada fase:
    - `feat(infra): domain layer — resource, provider, state, stack entities`
    - `feat(infra): application layer — planner, applier, destroyer`
    - `feat(infra): CLI — vyx infra init/plan/apply/destroy`
    - `feat(infra): provider AWS — S3, EC2, RDS, IAM, VPC`
    - `feat(infra): scanner infra_parser — @Resource, @Provider, @Tags`
    - ... (etc. para cada fase)
4. 🔲 Implementar Fase 1 (Domain + Application + CLI básica + backend local)
5. 🔲 Implementar Fase 2 (Provider AWS + Scanner)
6. 🔲 Implementar Fases 3-5

---

## 15. Dependências de Go

Para a Fase 1, dependências mínimas (seguindo o princípio zero-dependências do domain):

| Pacote | Uso | Obrigatório |
|--------|-----|-------------|
| `github.com/google/uuid` | IDs de recursos e estado | Sim (já existe no core) |
| `go.uber.org/zap` | Logging (já existe) | Sim |
| (stdlib) `container/list` | Topological sort | Não — implementar manualmente |

Para a Fase 2, dependências adicionais:

| Pacote | Uso |
|--------|-----|
| `github.com/aws/aws-sdk-go-v2` | AWS SDK (ou v1 para compatibilidade) |
| `github.com/aws/aws-sdk-go-v2/config` | AWS config loader |
| `github.com/aws/aws-sdk-go-v2/service/s3` | S3 state backend |
| `github.com/aws/aws-sdk-go-v2/service/dynamodb` | DynamoDB state locking |
| `github.com/aws/aws-sdk-go-v2/service/ec2` | EC2 resources |
| `github.com/aws/aws-sdk-go-v2/service/rds` | RDS resources |
| (more AWS services) | Conforme necessário |

**IMPORTANTE:** Providers serão isolados com build tags para não inflar o binário base:

```go
//go:build with_aws
package aws
```

```bash
# Build com suporte AWS
go build -tags with_aws ./cmd/vyx

# Build sem AWS (core puro, sem dependências extras)
go build ./cmd/vyx
```
