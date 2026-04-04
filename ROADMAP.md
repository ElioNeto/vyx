# vyx Roadmap

This document maps every open issue to a suggested implementation order based on technical dependencies, risk, and impact. It is a living document — priorities may shift as the project evolves.

See all issues at [github.com/ElioNeto/vyx/issues](https://github.com/ElioNeto/vyx/issues).

---

## How to read this table

| Column | Meaning |
|--------|---------|
| **Order** | Suggested sequence. Lower numbers should be completed first. |
| **Issue** | Linked GitHub issue. |
| **Area** | Core, Observability, DX, Performance, or Ecosystem. |
| **Depends on** | Issues that must be completed before this one. |
| **Complexity** | Rough effort estimate: S (small), M (medium), L (large), XL (extra large). |

---

## Suggested Development Order

| Order | Issue | Title | Area | Depends on | Complexity |
|------:|-------|-------|------|------------|------------|
| 1 | [#55](https://github.com/ElioNeto/vyx/issues/55) | Request Lifecycle Hooks (Middleware / ProxyListener API) | Core | — | M | ✅ done |
| 2 | [#52](https://github.com/ElioNeto/vyx/issues/52) | End-to-End Request Tracing (Correlation IDs) | Observability | #55 | M |
| 3 | [#57](https://github.com/ElioNeto/vyx/issues/57) | Pluggable Client IP Resolver (X-Forwarded-For) | Core | #55 | S |
| 4 | [#56](https://github.com/ElioNeto/vyx/issues/56) | Graceful Worker Drain on Shutdown and Hot Reload | Core | #55 | M |
| 5 | [#10](https://github.com/ElioNeto/vyx/issues/10) | Hot Reload: file watching and zero-downtime worker restart | DX | #56 | M |
| 6 | [#9](https://github.com/ElioNeto/vyx/issues/9) | Worker Pools and Round-Robin Load Balancing | Core | #56 | M |
| 7 | [#8](https://github.com/ElioNeto/vyx/issues/8) | Circuit Breaker: per-route failure detection and traffic diversion | Core | #9 | M |
| 8 | [#53](https://github.com/ElioNeto/vyx/issues/53) | Standardized Structured Logging across Polyglot Workers | Observability | #52 | M |
| 9 | [#11](https://github.com/ElioNeto/vyx/issues/11) | Observability: Prometheus metrics and OpenTelemetry tracing | Observability | #53 | L |
| 10 | [#54](https://github.com/ElioNeto/vyx/issues/54) | CLI / TUI for Real-Time Distributed Log Tailing and Search | DX | #53 | L |
| 11 | [#21](https://github.com/ElioNeto/vyx/issues/21) | CI/CD pipeline: annotation scanning and testing in build | DX | — | S |
| 12 | [#12](https://github.com/ElioNeto/vyx/issues/12) | TLS termination and security hardening | Core | — | M |
| 13 | [#58](https://github.com/ElioNeto/vyx/issues/58) | UDS Connection Warm Pool (Sliding Window) | Performance | #56 | L |
| 14 | [#6](https://github.com/ElioNeto/vyx/issues/6) | Python worker support and annotation scanner | Ecosystem | — | L |
| 15 | [#7](https://github.com/ElioNeto/vyx/issues/7) | Apache Arrow IPC for large dataset transfers | Performance | #6 | XL |
| 16 | [#13](https://github.com/ElioNeto/vyx/issues/13) | Documentation: guides, API reference and annotated examples | DX | #6, #12 | L |
| 17 | [#27](https://github.com/ElioNeto/vyx/issues/27) | Elixir Worker Support (BEAM VM) | Ecosystem | #6 | XL |
| 18 | [#59](https://github.com/ElioNeto/vyx/issues/59) | Worker-Initiated Remote Connection (Pull Model / Multi-Host) | Core | #9, #56, #58 | XL |
| 19 | [#14](https://github.com/ElioNeto/vyx/issues/14) | Remote Workers via gRPC for Multi-Host Deployments | Core | #59 | XL |
| 20 | [#15](https://github.com/ElioNeto/vyx/issues/15) | Kubernetes Operator Integration | Ecosystem | #14 | XL |

---

## Dependency graph

```
#55 Lifecycle Hooks
 ├── #52 Correlation IDs
 │    └── #53 Structured Logging
 │         ├── #11 Prometheus / OTel
 │         └── #54 TUI Log Tailing
 ├── #57 Client IP Resolver
 └── #56 Graceful Drain
      ├── #10 Hot Reload
      ├── #9  Worker Pools
      │    └── #8  Circuit Breaker
      └── #58 UDS Connection Pool
           └── #59 Remote Workers (Pull Model)
                └── #14 gRPC Remote Workers
                     └── #15 Kubernetes Operator

#6  Python Worker SDK
 ├── #7  Apache Arrow IPC
 ├── #16 Documentation
 └── #27 Elixir Worker SDK

#21 CI/CD  (independent)
#12 TLS    (independent)
```

---

## Contributing to a specific issue

If you want to work on any of these issues:

1. Comment on the issue to let others know you are working on it.
2. Read the **Acceptance Criteria** section in the issue before starting.
3. Check the **Depends on** column — make sure those issues are merged first, or coordinate with the maintainer.
4. Open a draft PR early so the community can give feedback.

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full contribution guide.
