## Testes Go — vyx

### Padrões
- Table-driven com `[]struct{ name, input, want }`
- Subtests com `t.Run(tc.name, ...)`
- `httptest.NewRecorder` para testar o HTTP gateway
- Mocks de workers via UDS local em `TestMain`
- `-race` obrigatório em todos os testes

### Cobertura
- Alvo mínimo: 80% em `core/` e `scanner/`
- `go test -coverprofile=coverage.txt ./...`
- `go tool cover -func=coverage.txt`

### Integração
- Tag `//go:build integration` para testes de integração que requerem workers reais
- Rodar separado: `go test -tags=integration ./...`
