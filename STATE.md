# STATE.md — current implementation state

_Last updated: 2026-09-11_

## Summary

**The harness is complete and the first vertical slice is under construction.**
Seven of the seventeen tasks in `docs/plans/orchestration.md` are done: the
platform's scaffold, two services that answer real requests, the rendering
rule, the catalogue the platform builds by asking those services what they
offer, the tool definitions built from that catalogue, and the safe-call/`none`
paths of `/api/plan`.

Every gate of `make check` passes except `acceptance-e2e`, which has no test
files: the end-to-end suite is Task 16. Until then the honest statement is that
the Go and web halves are green and the cross-process half does not exist.

## What works

**Two services, each its own Go module, each a whole contract-first stack.**
`services/inventory` serves stock items on port 8081 and `services/attendance`
serves attendance records on port 8082. Both expose four operations - list with
an enum filter, create, get by id, and `GET /openapi.yaml` - over eight fixture
rows apiece, and both carry an enum whose values a Japanese speaker cannot guess
from the English (`quarantined` = 検品保留, `substitute` = 振替休日). Each labels
every enum value with `x-enum-labels`; `make api-lint` fails a spec that does
not.

Each service serves its own contract from the spec oapi-codegen embedded in its
generated code, so the document it serves and the code that serves it cannot
drift. The embedding normalises operation ids to PascalCase and changes nothing
else, which is deliberate (`DECISIONS.md`, 2026-09-11).

**The platform's domain**, `services/platform/internal/domain`, imports nothing
outside the standard library and is fully covered. It holds the catalogue types
and `Render`, the pure rule that picks a component from a response schema: an
explicit `x-ui-hint` wins, then a request body means `form`, an array of objects
means `table`, an object means `detail`.

**The catalogue**, `services/platform/internal/adapter/specsource/http` behind
the `usecase.SpecSource` port. It reads `ORCHESTRA_SERVICES` (`name=url` pairs),
fetches each service's `/openapi.yaml`, and converts every operation into a
`domain.Endpoint`, carrying `x-enum-labels` through to `Schema.EnumLabels` -
the only route by which the Japanese label for an enum value reaches the model.
A service that cannot be reached fails the whole fetch rather than yielding a
partial catalogue. Verified against both services running: eight endpoints, both
enums labelled, every component the rule picks correct.

**The harness**, imported from takamai and renamed (`DECISIONS.md`,
2026-09-10). `harness/` holds all of it - `quality/` (policy), `guard/`
(implementations), `githooks/`, `claude/`, `gen/` and `quiet.sh`. Every target,
guard, hook and CI job discovers services rather than naming them: a service is
a directory under `services/` with a `go.mod`, and nothing declares the list.

Two guards were added for this design: `guard-generated-ops` fails when a
generated file is missing an operation id its spec declares (oapi-codegen drops
OpenAPI 3.2 `query` operations silently), and a Redocly plugin fails a spec
whose `enum` carries no `x-enum-labels`.

**Tool definitions**, `services/platform/internal/usecase/tools.go`. `ToolsFor`
turns a `domain.Catalog` into one `Tool` (a plain JSON-Schema map, spec-agnostic
so both the tool-calling and JSON planners of Task 10/11 can render it their
own way) per endpoint that has a request body or a response schema, plus the
fixed `AskUserTool`. An enum parameter's Japanese labels are appended to its
property description (`allocated=引当済 / staged=出荷準備完了`) - the only route
by which they reach a model.

**The safe path of `/api/plan`.** `services/platform/internal/usecase/planner.go`
and `orchestrator.go` add the ports and types Task 6 calls for:

```go
type Planner interface {
    Plan(ctx context.Context, query string, answers []Answer, tools []Tool) (Decision, error)
}
type Invoker interface {
    Invoke(ctx context.Context, e *domain.Endpoint, args map[string]any) (any, error)
}
type Orchestrator struct{ /* catalog, planner, invoker */ }
func NewOrchestrator(catalog domain.Catalog, planner Planner, invoker Invoker) *Orchestrator
func (o *Orchestrator) Plan(ctx context.Context, query string, answers []Answer) (Result, error)
```

`Invoker.Invoke` and `Orchestrator.call`'s `Decision` take a pointer rather than
the value the plan's sketch shows, matching `domain.Render`'s and
`Endpoint.IsSafe`'s existing precedent for the same gocritic `hugeParam` reason
(`DECISIONS.md`, 2026-09-11). `Orchestrator.Plan` handles `DecisionCall` against
a safe endpoint (invoke, render, return `kind: "result"`) and `DecisionNone`
(`kind: "none"` with a fixed Japanese message, nothing invoked); a
`DecisionCall` against an unsafe endpoint or a `DecisionAsk` returns an error
wrapping `usecase.ErrNotImplemented` - Task 7 and Task 9's seats, left open on
purpose rather than silently degrading to `none`.

`internal/adapter/planner/stub` implements `Planner` as an exact-query-string
table lookup - deterministic, no I/O, the only planner that exists until Task
10 and therefore every test's default. `internal/adapter/invoker/http`
implements `Invoker` by calling a configured service over HTTP, splitting a
decision's `args` across the endpoint's path, query and request body.
`internal/adapter/handler/plan.go` and `invoke.go` implement the generated
`PostPlan`/`PostInvoke`; `PostInvoke` always answers 501 today (Task 8's job).

`pkg/app.New` now takes a `Config{StaticDir, Services, PlanFixtures}` (its own
exported types, not aliases of anything under `internal/`) and wires the whole
graph, fetching the catalogue at startup. An empty `PlanFixtures` - production's
case, since `cmd/api` never sets one - falls back to a two-entry demo table
(`"在庫の一覧を見せて"` and `"勤怠記録の一覧を見せて"`) hard-coded in `pkg/app/app.go`
(`DECISIONS.md`, 2026-09-11). Verified against inventory and attendance running
on 8081/8082 with `ORCHESTRA_SERVICES` set: `POST /api/plan` with the first demo
query returns `kind: "result"`, `component: "table"`; an unrecognised query
returns `kind: "none"`; `POST /api/invoke` returns 501.

## What does not exist yet

Tasks 7 to 16 of `docs/plans/orchestration.md`: the form path for unsafe calls,
`/api/invoke`'s real execution, `ask_user`, the two planner adapters, and the
whole of `web/src` beyond the shell. `make check` is green because the guards
report on the code that is there, not because the product is finished.

## Known gaps in the harness

- **TypeScript is held at 6.0.3 by a dependency, not by choice.**
  `openapi-typescript` builds its output with the TypeScript Compiler API, which
  TypeScript 7's native implementation does not provide, so `make generate` fails
  under 7. Everything else - including type-aware Oxlint - passed under 7. Go and
  pnpm are current (`DECISIONS.md`, 2026-09-10).
