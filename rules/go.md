## Regras Go

### Convenções
- Seguir o [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Nomes de pacotes: lower case, sem underscores
- Interfaces: sufixo `-er` quando possível (`Reader`, `Writer`, `Handler`)
- Erros: sempre tratados, nunca `_`
- `context.Context` sempre como primeiro parâmetro

### Estrutura de pacotes
- `internal/` para código não exportado
- `cmd/` para entry points
- `pkg/` para bibliotecas exportadas (quando aplicável)

### Testes
- Arquivos de teste: `*_test.go` no mesmo pacote
- Table-driven tests para múltiplos casos
- `testify` permitido; prefer `t.Fatal` sobre `t.Error` quando o estado é inválido

### Build e ferramentas
```bash
go build ./...
go test ./... -race -coverprofile=coverage.txt
go vet ./...
golangci-lint run
govulncheck ./...
```

### CI jobs locais
- `go-test`: `go build ./...` + `go test ./...`
- `security-go`: `govulncheck ./...`
