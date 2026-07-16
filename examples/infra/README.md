# Infrastructure Module — Examples

This directory contains example projects demonstrating the vyx IaC module.

## Quick Start

```bash
# 1. Create a vyx.yaml with infrastructure resources
cat > vyx.yaml << 'EOF'
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
EOF

# 2. Initialize the local state backend
vyx infra init

# 3. See what will be created
vyx infra plan

# 4. Apply the changes
vyx infra apply
```

## Annotations Example

Infrastructure resources can also be defined via annotations in Go files (scanned at build time):

```go
// infra/resources.go
// @Resource(type: "aws_s3_bucket", id: "assets")
// @Provider(aws)
// @Tags(env: "production")
// @Output(bucket_arn)
func defineStorage() {}

// @Resource(type: "aws_rds_instance", id: "database")
// @Provider(aws)
// @DependsOn(assets)
// @Output(host), @Output(port), @Output(arn)
func defineDatabase() {}

// @Resource(type: "aws_sqs_queue", id: "orders-queue")
// @Provider(aws)
// @Tags(env: "production", team: "orders")
// @Output(queue_arn)
func defineQueue() {}
```

```bash
# Scan annotations and generate infra_map.json
vyx build --infra infra/

# Plan using the discovered resources
vyx infra plan
```

## Python Example

```python
# infra/database.py
# @Resource(type: "aws_rds_instance", id: "main-db")  
# @Provider(aws)
# @Tags(env: "production")
# @Output(host), @Output(port)

def create_database():
    return {
        "engine": "postgres",
        "engine_version": "16.3",
        "instance_class": "db.r6g.large",
        "allocated_storage": 100,
    }
```

## Full AWS Architecture Example

```go
// infra/architecture.go
// @Resource(type: "aws_s3_bucket", id: "frontend-assets")
// @Provider(aws)
// @Tags(env: "production", team: "frontend")
func defineAssets() {}

// @Resource(type: "aws_ec2_instance", id: "backend-api")
// @Provider(aws)
// @DependsOn(frontend-assets)
// @Tags(env: "production")
func defineAPI() {}
```

## State Management

```bash
# List resources in state
vyx infra state list

# Rename a resource
vyx infra state mv old-name new-name

# Remove a resource from state (keep in cloud)
vyx infra state rm resource-to-remove

# Refresh outputs from cloud
vyx infra state refresh
```

## Export to Terraform

```bash
# Export as Terraform HCL
vyx infra export --format terraform --output main.tf

# Export as CloudFormation
vyx infra export --format cloudformation --output template.yaml
```

## Backend Selection

```yaml
# Local (default, development)
infrastructure:
  backend:
    type: local
    config:
      path: .vyx/infra.tfstate

# S3 (production, AWS)
infrastructure:
  backend:
    type: s3
    config:
      bucket: my-state-bucket
      key: infra/production/state
      region: us-east-1

# Consul
infrastructure:
  backend:
    type: consul
    config:
      address: http://localhost:8500
      path: vyx/infra/myapp

# HTTP (custom)
infrastructure:
  backend:
    type: http
    config:
      address: http://state-api.internal:8080
```
