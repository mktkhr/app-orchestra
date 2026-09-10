# Architecture

This describes the shape the code takes, not code that exists: the product was
removed when the harness was imported (`DECISIONS.md`, 2026-09-10). The layer
declarations in `harness/quality/architecture.json` and the guards that read them are
live, and apply to the first file written into either tree.

## Backend (Go, Clean Architecture)

```
cmd/api            reads the environment, runs the server            ─┐
pkg/app            composition root: wires adapters into use cases   ─┤ outer
internal/infra     config, HTTP server, request validation, routing  ─┤
internal/adapter   handler (generated server interface), repository  ─┤
internal/usecase   application logic, orchestrates the domain        ─┤
internal/domain    entities, value objects, repository interfaces    ─┘ inner
```

Dependencies point inwards only. `harness/quality/architecture.json` lists the layers;
`harness/guard/archcheck` verifies every import edge (`make guard-arch`), and
`depguard` in `harness/quality/go/golangci.yml` adds package-level denials (the domain
may not import `net/http`, `encoding/json`, `log`, ...).

The HTTP surface is generated from `services/platform/api/openapi.yaml`:

- `internal/adapter/openapi/openapi.gen.go` (models, `StrictServerInterface`, router, embedded spec)
- `internal/adapter/handler` implements the interface; the compiler proves completeness.
- `internal/infra/httpserver` validates every request against the spec before dispatch and maps every failure to the contract's error envelope.

## Frontend (React, Feature-Sliced Design)

```
app        composition, providers, entry point
pages      route-level screens
widgets    composite blocks
features   user actions
entities   domain objects derived from the contract
shared     generated API client, config, cross-slice helpers
```

Rules, verified by `harness/guard/fsd.ts` (`make guard-fsd`):

1. import lower layers only;
2. never import a sibling slice;
3. reach a slice through its `index.ts`;
4. relative imports stay inside the slice.

Network access exists only in `shared/api` (`openapi-fetch` over the generated
`paths`). Oxlint forbids the global `fetch` everywhere else.

## Contract flow

```
services/platform/api/openapi.yaml ──make generate──▶ Go strict server + validator
                 └──────────────▶ TypeScript paths/components + client
make guard-generated: fails when either output is stale
redocly lint:         fails when the spec breaks the rules in redocly.yaml
```
