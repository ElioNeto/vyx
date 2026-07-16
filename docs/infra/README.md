# Infrastructure Module

The vyx Infrastructure as Code (IaC) module allows you to define, provision, and manage cloud resources programmatically — similar to Terraform or Pulumi, but fully integrated with the vyx framework.

## Overview

```
vyx infra — manage cloud infrastructure

Usage:
  vyx infra <command> [arguments]

Commands:
  init                   Initialize the state backend
  plan                   Show the plan (diff between desired and current state)
  apply                  Apply the planned changes
  destroy                Destroy all managed resources
  graph                  Generate dependency graph (mermaid or dot format)
  output                 Show output values from the state
  import                 Import existing cloud resource into state
  state                  Manage state (list, mv, rm, refresh)
  export                 Export to Terraform HCL or CloudFormation

Global flags:
  --state-path=<path>    Path to the state file (default: .vyx/infra.tfstate)
  --stack=<name>         Stack name (default: default)
  --dir=<path>           Project root directory (default: .)
```

## Workflow

The standard IaC workflow has 4 steps:

### 1. Define Resources

Resources can be defined in two ways:

**Option A: vyx.yaml (inline)**

```yaml
project:
  name: my-app

infrastructure:
  resources:
    - type: aws_s3_bucket
      id: assets
      provider: aws
    - type: aws_db_instance  
      id: database
      provider: aws
```

**Option B: Annotations (scanned)**

```go
// infra/resources.go
// @Resource(type: "aws_s3_bucket", id: "assets")
// @Provider(aws)
// @Tags(env: "production")
func defineStorage() {}
```

Then run:
```bash
vyx build --infra infra/
```

### 2. Initialize

```bash
vyx infra init
```

This creates the state backend (default: `.vyx/infra.tfstate`).

### 3. Plan

```bash
vyx infra plan
```

Shows what changes will be made:
```
Plan: 2 to create, 0 to update, 0 to delete
  + aws_s3_bucket assets (create)
  + aws_db_instance database (create)
```

Exit code 2 means changes are pending.

### 4. Apply

```bash
vyx infra apply
```

For CI/CD pipelines, use `--auto-approve` to skip confirmation:

```bash
vyx infra apply --auto-approve
```

To destroy all resources:

```bash
vyx infra destroy
vyx infra destroy --auto-approve
```

## State Management

### Backends

```yaml
# Local (default, for development)
backend:
  type: local
  config:
    path: .vyx/infra.tfstate

# S3 (for production on AWS)
backend:
  type: s3
  config:
    bucket: my-state-bucket
    key: infra/state
    region: us-east-1

# Consul (for multi-cloud)
backend:
  type: consul
  config:
    address: http://localhost:8500
    path: vyx/infra/myapp
```

### Commands

```bash
vyx infra state list            # List all resources
vyx infra state mv <from> <to>  # Rename a resource
vyx infra state rm <id>         # Remove from state (no destroy)
vyx infra state refresh         # Update outputs from cloud
```

## Importing Existing Resources

Import resources that were created outside of vyx:

```bash
vyx infra import <resource_type> <resource_id> <cloud_id>

# Examples:
vyx infra import aws_s3_bucket my-bucket my-existing-bucket
vyx infra import aws_instance web-server i-1234567890abcdef0
```

## Outputs

Access output values from provisioned resources:

```bash
# List all outputs
vyx infra output

# Get specific output
vyx infra output bucket.arn

# JSON format
vyx infra output --json
```

## Dependency Graph

Visualize resource dependencies:

```bash
# Mermaid format (for documentation)
vyx infra graph --format mermaid

# Graphviz DOT format
vyx infra graph --format dot

# Pipe to file
vyx infra graph --format dot > infra.dot
```

## Export to Other Formats

Export vyx infrastructure definitions to Terraform HCL or CloudFormation:

```bash
# Terraform
vyx infra export --format terraform --output main.tf

# CloudFormation
vyx infra export --format cloudformation --output template.yaml
```

## Annotation Reference

| Annotation | Format | Description |
|------------|--------|-------------|
| `@Resource` | `@Resource(type: "aws_s3_bucket", id: "my-bucket")` | Declares a resource |
| `@Provider` | `@Provider(aws)` | Specifies the cloud provider |
| `@Tags` | `@Tags(env: "production", team: "platform")` | Resource tags |
| `@DependsOn` | `@DependsOn(database, cache)` | Dependencies |
| `@Output` | `@Output(bucket_arn)` | Declares an output value |

## Provider Reference

### AWS Provider (11 resource types)

Build with: `go build -tags with_aws ./cmd/vyx`

| Resource Type | Description |
|---------------|-------------|
| `aws_s3_bucket` | S3 storage bucket |
| `aws_instance` | EC2 virtual machine |
| `aws_db_instance` | RDS database |
| `aws_iam_role` | IAM role |
| `aws_lambda_function` | Lambda function |
| `aws_vpc` | Virtual Private Cloud |
| `aws_sqs_queue` | SQS message queue |
| `aws_sns_topic` | SNS notification topic |
| `aws_route53_zone` | Route53 DNS zone |
| `aws_elasticache_cluster` | ElastiCache cluster |
| `aws_api_gateway_rest_api` | API Gateway REST API |

## Architecture

```
vyx.yaml / infra_map.json
         │
         ▼
    StackLoader ──► Stack
         │
    ┌────┴────┐
    ▼         ▼
  Planner   Applier
    │         │
    ▼         ▼
  Plan      State
  Result    (local/S3/consul/http)
    │         │
    └──► Provider (AWS/Azure/Mock)
              │
              ▼
          Cloud API
```
