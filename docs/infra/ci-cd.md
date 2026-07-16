# Infrastructure CI/CD Guide

## Local Testing

### Prerequisites

- Docker (for LocalStack)
- AWS CLI or awslocal (for LocalStack interaction)
- Go 1.25+

### Running Integration Tests

```bash
# Start LocalStack
docker compose -f core/integration/infra/docker-compose.yml up -d

# Wait for LocalStack to be ready
sleep 10

# Run integration tests
go test -tags integration ./core/integration/infra/... -v

# Run AWS provider tests (with build tag)
go test -tags with_aws ./core/infrastructure/infra/... -v

# Run all infra tests
go test ./core/domain/infra/... ./core/application/infra/... -v

# Stop LocalStack
docker compose -f core/integration/infra/docker-compose.yml down
```

### Skipping Integration Tests

```bash
SKIP_INTEGRATION=1 go test ./... -v
```

## CI Pipeline

### GitHub Actions

The CI workflow runs:

1. **Unit tests** — all packages without build tags
2. **Race detection** — all tests with `-race`
3. **Build verification** — compilation without build tags
4. **AWS build verification** — compilation with `-tags with_aws`

### Example: .github/workflows/infra.yml

```yaml
name: Infrastructure Tests

on:
  push:
    branches: [main]
  pull_request:
    paths:
      - 'core/domain/infra/**'
      - 'core/application/infra/**'
      - 'core/infrastructure/infra/**'
      - 'scanner/infra_parser*'

jobs:
  infra-unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go test ./core/domain/infra/... -race -count=1
      - run: go test ./core/application/infra/... -race -count=1
      - run: go test ./core/infrastructure/infra/state/... -race -count=1
      - run: go vet ./core/domain/infra/...

  infra-build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go build ./cmd/vyx/...
      - run: go build -tags with_aws ./core/infrastructure/infra/...
      - run: go vet ./core/domain/infra/...

  infra-integration:
    runs-on: ubuntu-latest
    services:
      localstack:
        image: localstack/localstack:latest
        ports:
          - 4566:4566
        env:
          SERVICES: s3,dynamodb,ec2
          DEFAULT_REGION: us-east-1
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go test -tags integration ./core/integration/infra/... -v
```

## Build Tags Reference

| Tag | Effect | Binary Size |
|-----|--------|-------------|
| *(none)* | Core + domain + local state | ~15 MB |
| `with_aws` | + AWS provider + S3 backend | ~65 MB |
| `with_aws,with_azure` | + AWS + Azure providers | ~80 MB |
