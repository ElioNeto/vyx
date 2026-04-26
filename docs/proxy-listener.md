# ProxyListener & RequestLifecycle API

## Visão Geral

O gateway fornece dois sistemas de hooks para estender seu comportamento:

- **RequestLifecycle**: hooks simples para logging, auditoria e transformação de dados. Ideal para casos de uso onde você não precisa interferir no fluxo de requisição.
- **ProxyListener**: pipeline completo com acesso ao `LifecycleContext` mutável. Ideal para middleware que precisa inspecionar, modificar ou abortar requisições/respostas.

Use **RequestLifecycle** quando precisar de hooks de entrada/saída simples que não modificam o fluxo. Use **ProxyListener** quando precisar de controle granular sobre o pipeline (rate limiting, caching, maintenance mode).

---

## RequestLifecycle (hooks simples)

A interface `RequestLifecycle` define três pontos de hooks:

| Método | Quando chamado | Pode abortar? |
|--------|--------------|----------------|
| `OnBeforeDispatch` | Após route lookup e auth, antes de enviar para o worker | Sim (retorna erro) |
| `OnAfterDispatch` | Após receber resposta do worker | Não |
| `OnWorkerError` | Quando o worker retorna erro ou timeout | Não |

### Exemplo de uso: Audit Logger

```go
type AuditHook struct {
    logger *zap.Logger
}

func (a *AuditHook) OnBeforeDispatch(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry) error {
    a.logger.Info("request started",
        zap.String("method", req.Method),
        zap.String("path", req.Path),
        zap.String("user_id", req.Claims.UserID),
    )
    return nil
}

func (a *AuditHook) OnAfterDispatch(ctx context.Context, req *dgw.GatewayRequest, resp *dgw.GatewayResponse) {
    a.logger.Info("request completed",
        zap.String("method", req.Method),
        zap.Int("status", resp.StatusCode),
    )
}

func (a *AuditHook) OnWorkerError(ctx context.Context, workerID string, req *dgw.GatewayRequest, err error) {
    a.logger.Error("worker error",
        zap.String("worker_id", workerID),
        zap.Error(err),
    )
}
```

### Exemplo de uso: Payload Transformer

```go
type PayloadTransformHook struct {
    transformer *mytransformer.Transformer
}

func (p *PayloadTransformHook) OnBeforeDispatch(ctx context.Context, req *dgw.GatewayRequest, route *dgw.RouteEntry error {
    var body map[string]interface{}
    if err := json.Unmarshal(req.Body, &body); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }
    transformed := p.transformer.Transform(body)
    req.Body, _ = json.Marshal(transformed)
    return nil
}
```

---

## ProxyListener (pipeline completo)

A interface `ProxyListener` oferece hooks mais poderosos com acesso ao `LifecycleContext` mutável:

| Fase | Quando chamada | Campos disponíveis no LifecycleContext |
|------|---------------|---------------------------------------|
| `OnRouteMatch` | Após lookup da rota, antes de auth | `Request`, `CorrelationID`, `Route` |
| `OnPreDispatch` | Após validação, antes de enviar para worker | `Request`, `Phase`, `StatusCode` |
| `OnPostDispatch` | Após receber resposta do worker | `Request`, `Response`, `StatusCode` |
| `OnError` | Em qualquer caminho de erro | `Err`, `Phase`, `StatusCode` |

### Exemplo de uso: Prometheus Metrics

```go
type MetricsListener struct {
    histogram *prometheus.HistogramVec
}

func (m *MetricsListener) OnPostDispatch(lc *gateway.LifecycleContext, duration time.Duration) {
    m.histogram.WithLabelValues(lc.Route.WorkerID).Observe(duration.Seconds())
}

func (m *MetricsListener) OnError(lc *gateway.LifecycleContext, phase gateway.Phase) {
    m.errors.WithLabelValues(string(phase)).Inc()
}
```

### Exemplo de uso: Maintenance Mode

```go
type MaintenanceModeListener struct {
    enabled bool
}

func (m *MaintenanceModeListener) OnPreDispatch(lc *gateway.LifecycleContext) {
    if m.enabled {
        lc.RespondBeforeDispatch(&dgw.GatewayResponse{
            StatusCode: 503,
            Body:       []byte(`{"error":"maintenance mode"}`),
        })
    }
}
```

### Exemplo de uso: FuncListener

Para casos simples, use `FuncListener` que converte funções em `ProxyListener`:

```go
listener := gateway.FuncListener{
    OnRouteMatchFn: func(lc *gateway.LifecycleContext) {
        log.Printf("route matched: %s", lc.Route.Path)
    },
    OnPreDispatchFn: func(lc *gateway.LifecycleContext) {
        lc.Request.Headers["X-Request-ID"] = uuid.New().String()
    },
}
```

---

## Registrando hooks no Dispatcher

Use as opções `WithLifecycleHooks` e `WithProxyListeners` ao criar o `Dispatcher`:

```go
d := gateway.NewDispatcher(
    routes,
    transport,
    jwt,
    schema,
    timeout,
    logger,
    nil, // WorkerDrainer
    gateway.WithLifecycleHooks(
        NewAccessLogLifecycle(logger),
        &AuditHook{logger: logger},
    ),
    gateway.WithProxyListeners(
        &MetricsListener{histogram: histogram},
        &MaintenanceModeListener{enabled: false},
    ),
)
```

### AccessLogLifecycle built-in

O gateway já inclui um `AccessLogLifecycle` padrão que emite logs de acesso no formato:

```
access  method=GET path=/api/users user_id=user-42 status=200 latency=1.5ms correlation_id=abc-123
```

Ele é registrado automaticamente no `NewDispatcher` (antes dos hooks do usuário). Para substituí-lo, passe seu próprio hook via `WithLifecycleHooks`.

---

## Ordem de execução

```
OnRouteMatch (ProxyListener)
       ↓
      [auth: JWT validation]
       ↓
      [role check]
       ↓
OnPreDispatch (ProxyListener)
       ↓
   [IPC send to worker]
       ↓
   [IPC receive]
       ↓
OnAfterDispatch (RequestLifecycle + ProxyListener)
```

Em caso de erro:

```
[any error occurred]
       ↓
OnError (ProxyListener)
       ↓
OnWorkerError (RequestLifecycle)
```

Oshooks são executados na ordem em que são registrados. Hooks `RequestLifecycle` executam antes de `ProxyListener` no fluxo feliz.