## Testes Go

### Padrões
- Table-driven tests com `[]struct{ name, input, want }`
- Subtests com `t.Run(tc.name, ...)`
- Mocks com interfaces, não com structs concretos
- Fixtures em `testdata/`

### Cobertura
- Alvo mínimo: 80% em pacotes de lógica de negócio
- `go test -coverprofile=coverage.txt ./...`
- `go tool cover -func=coverage.txt`

### Integração
- Tag `//go:build integration` para testes de integração
- Rodar separado: `go test -tags=integration ./...`
