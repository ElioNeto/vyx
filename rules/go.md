## Regras Go — vyx

### Convenções
- Seguir [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Nomes de pacotes: lowercase sem underscores
- Interfaces: sufixo `-er` quando possível
- Erros sempre tratados — nunca `_` para erros
- `context.Context` sempre como primeiro parâmetro
- Sem `fmt.Println` no código de produção

### Estrutura de pacotes do vyx
- `cmd/` — entry points do CLI `omni`
- `core/` — Core Orchestrator (dispatcher, runtime manager, circuit breaker, gateway)
- `scanner/` — annotation parser, gerador de route_map.json
- `packages/` — bibliotecas compartilhadas

### Build e ferramentas
```bash
go build ./...
go test ./... -race -coverprofile=coverage.txt
go vet ./...
golangci-lint run
govulncheck ./...
```

### Testes
- Table-driven tests obrigatórios para múltiplos casos
- `-race` sempre habilitado
- Mocks via interfaces, nunca structs concretos
- Fixtures em `testdata/`
