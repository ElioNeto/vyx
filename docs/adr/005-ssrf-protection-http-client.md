# ADR-005: Proteção SSRF no Client HTTP Padrão

## Status
Aceito

## Contexto
Workers em Go, Python e Node.js frequentemente precisam fazer chamadas HTTP para serviços externos (APIs de terceiros, webhooks, integrações). Sem proteção adequada, um worker comprometido ou mal configurado pode ser usado para ataques SSRF (Server-Side Request Forgery), acessando:
- Serviços internos na nuvem (AWS Metadata Service em 169.254.169.254)
- Serviços internos da rede (Redis, MySQL, etc. em 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
- Localhost (127.0.0.1, ::1)

Todos os três runtimes (Go, Python, Node.js) precisam ter o MESMO comportamento de proteção.

## Decisão

### Client HTTP Seguro (Obrigatório)

O VYX framework fornece um client HTTP padronizado que implementa proteção SSRF por padrão:

```yaml
# vyx.yaml — configuração global do client HTTP
http_client:
  block_private_ips: true          # Bloqueia RFC 1918, RFC 6598, RFC 3927
  block_localhost: true            # Bloqueia 127.0.0.1, ::1, localhost
  block_cloud_metadata: true       # Bloqueia 169.254.169.254, 169.254.170.2
  allowed_schemes: ["https"]       # Apenas HTTPS (não http, file, ftp, dict, gopher)
  max_redirects: 5                 # Limite de redirects
  timeout: 10s                     # Timeout obrigatório
  keepalive: 30s                   # Keepalive
  max_idle_conns: 100              # Pool de conexões
  tls_min_version: "1.2"           # TLS mínimo
  validate_url: true               # Valida URL contra allowlist de domínios
```

### Comportamento por Runtime

| Comportamento | Go | Python | Node.js |
|--------------|----|--------|---------|
| Bloquear 127.0.0.0/8 | `net.ParseIP` + deny | `ipaddress` module | `net.isIP` |
| Bloquear 10.0.0.0/8 | `net.ParseIP` + deny | `ipaddress` module | `net.isIP` |
| Bloquear 172.16.0.0/12 | `net.ParseIP` + deny | `ipaddress` module | `net.isIP` |
| Bloquear 192.168.0.0/16 | `net.ParseIP` + deny | `ipaddress` module | `net.isIP` |
| Bloquear 169.254.0.0/16 | `net.ParseIP` + deny | `ipaddress` module | `net.isIP` |
| Bloquear ::1/128 | `net.ParseIP` + deny | `ipaddress` module | `net.isIP` |
| Bloquear metadata cloud | String match em resolved IP | String match | String match |
| Apenas https | `url.Parse` scheme check | `urlparse` scheme check | `URL` scheme check |
| Timeout | `context.WithTimeout` | `asyncio.timeout` | `AbortController` |
| Redirect limit | `http.Client.CheckRedirect` | `aiohttp.ClientSession` | `fetch` redirect |

### Mecanismo de Bloqueio

O bloqueio acontece em DUAS camadas para evitar bypass por DNS rebinding:

```
1. ANTES da resolução DNS:
   - Valida URL (scheme, formato)
   - Verifica allowlist de domínios (se configurada)
   - Rejeita IPs literais em range bloqueado

2. DEPOIS da resolução DNS:
   - Resolve o hostname para IP
   - Verifica se o IP resolvido está em range bloqueado
   - Se estiver → bloqueia com erro SSRF_BLOCKED
   - Se não → permite a requisição
```

### Estrutura de Erro

```json
{
  "error": {
    "code": "VYX-HTTP-001",
    "title": "SSRF blocked",
    "detail": "A requisição para 'http://169.254.169.254/latest/meta-data/' foi bloqueada por proteção SSRF (endereço IP privado)."
  }
}
```

### Exceções (Allowlist)

Para casos legítimos que precisam acessar serviços internos:

```yaml
# vyx.yaml — por worker
workers:
  - id: go:internal-api
    http_client:
      allowed_private_networks:
        - "10.100.0.0/16"    # Acesso ao cluster Kubernetes
        - "db.internal:5432"  # Acesso ao banco
```

### Implementação nos SDKs

```go
// Go — comportamento padrão
client := vyx.NewHTTPClient()
resp, err := client.Get("https://api.externa.com/data")
// Se tentar http://169.254.169.254/ → erro SSRF_BLOCKED

// Configuração por worker (permite rede interna)
client := vyx.NewHTTPClient(vyx.HTTPClientConfig{
    AllowedPrivateNetworks: []string{"10.100.0.0/16"},
})
```

```python
# Python — mesmo comportamento
client = vyx.HTTPClient()
resp = await client.get("https://api.externa.com/data")
```

```typescript
// Node.js — mesmo comportamento
const client = new vyx.HTTPClient();
const resp = await client.get('https://api.externa.com/data');
```

### Testes de Conformidade

TODO SDK DEVE PASSAR ESTES TESTES:

1. `http://127.0.0.1:8080/admin` → SSRF_BLOCKED
2. `http://10.0.0.1:27017/` → SSRF_BLOCKED (MongoDB)
3. `http://192.168.1.1/` → SSRF_BLOCKED
4. `http://169.254.169.254/latest/meta-data/` → SSRF_BLOCKED (AWS metadata)
5. `http://[::1]:3000/` → SSRF_BLOCKED
6. `http://localhost:9200/` → SSRF_BLOCKED
7. `ftp://files.internal/data` → SSRF_BLOCKED (scheme não permitido)
8. `file:///etc/passwd` → SSRF_BLOCKED
9. `https://api.externa.com/data` → ✅ ALLOWED
10. URL com redirect para IP privado → SSRF_BLOCKED (proteção DNS rebinding)

## Consequências

### Positivas
1. Proteção SSRF nativa e consistente entre todos os runtimes
2. Bloqueio em duas camadas (pré-DNS + pós-DNS) previne DNS rebinding
3. Configurável por worker (allowlist para casos legítimos)
4. Erro claro e código específico (VYX-HTTP-001)

### Negativas
1. Pode quebrar integrações legítimas com serviços internos (exige allowlist)
2. A resolução DNS adicional adiciona ~1-5ms de latência
3. DNS rebinding é difícil de mitigar completamente (exige TTL check)

### Riscos
- Se a allowlist de redes privadas for muito permissiva, anula a proteção
- Se o DNS da máquina estiver comprometido, a proteção pós-DNS pode ser enganada
- Serviços cloud com metadata endpoints não padronizados (ex: GCP usa 169.254.169.254 igual AWS)

## Referências
- SECURITY_ARCHITECTURE.md — Seção 7.1 (SSRF), Seção 10
- OWASP API Security Top 10 — API6:2023 (Unrestricted Access to Sensitive Business Flows)
- OWASP Cheat Sheet: Server-Side Request Forgery Prevention
