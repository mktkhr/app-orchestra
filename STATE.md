# STATE.md — current implementation state

_Last updated: 2026-09-11_

## Summary

**The harness is complete and the first vertical slice is under construction.**
Eleven of the seventeen tasks in `docs/plans/orchestration.md` are done: the
platform's scaffold, two services that answer real requests, the rendering
rule, the catalogue the platform builds by asking those services what they
offer, the tool definitions built from that catalogue, and all four answers
`/api/plan` can give: a rendered result, a form, a disambiguation question,
and no match - decided by a real model over an OpenAI-compatible endpoint.

The whole backend half of the slice now runs end to end. A question in
Japanese reaches a local model, the model picks one operation out of the
catalogue and fills its arguments, and the platform calls the service and
says what to draw with the answer. Nothing in the path is hard-coded: the
services declare their own contracts, and the model is told about them.

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
a safe endpoint (invoke, render, return `kind: "result"`), a `DecisionCall`
against an unsafe one (build the form from the request body's schema, carry the
planner's arguments as its initial values and the endpoint as its target, and
call nothing at all) and `DecisionNone` (`kind: "none"` with a fixed Japanese
message). `DecisionAsk` (Task 9) is now built too: `Orchestrator.ask` renders
`kind: "ask"` carrying the question, the parameter name and its options - never
from `Decision.Options` directly. Because a `Decision` is ultimately produced
by a model, its own list of candidate values cannot be trusted to exist; `ask`
first resolves `decision.Service`/`decision.OperationID` against the
catalogue (`ErrEndpointNotFound` if that pair does not exist - `ask_user`
names its operation for exactly this reason, DECISIONS.md 2026-09-11), then
`optionsForParam` searches only that one endpoint's parameters and request
body properties for one named `decision.Param` that declares an enum, and
builds the options from that schema's `Enum` and `EnumLabels`. Resolving
`param` within its own endpoint, not the whole catalogue, is what keeps a
parameter name such as `status` from colliding across services. A
param the catalogue does not recognise as an enum returns
`usecase.ErrUnknownParam` (a 500) rather than falling back to the model's own
list. A `DecisionKind` the switch does not recognise at all still returns
`usecase.ErrNotImplemented` (a 501).

The re-post half of the ask flow - `POST /api/plan` with `answers` alongside
the original `query` reaching the planner and producing a different decision -
was already wired: `handler.Plan.PostPlan` converts `PlanRequest.Answers` into
`[]usecase.Answer` unconditionally, and `Orchestrator.Plan` always passes
`answers` through to `Planner.Plan`. What Task 9 added on that side is the stub
planner's ability to act on them: `internal/adapter/planner/stub`'s table is
now keyed by `stub.Key{Query, Answers}`, where `Answers` is
`stub.AnswersKey(answers)` - a canonical, order-independent encoding
(`"param=value"` pairs, sorted, joined with `&`) - so the very same question can
map to an `ask` decision with no answers and a `call` decision once answered,
while staying a pure table lookup.

The form's schema is built by the same `schemaToJSONSchema` the tool
definitions use, so an enum reaches the browser with both its values and its
Japanese labels: the frontend can offer 引当済 and post `allocated`.

`internal/adapter/planner/stub` implements `Planner` as an exact-query-string
table lookup - deterministic, no I/O, the only planner that exists until Task
10 and therefore every test's default. `internal/adapter/invoker/http`
implements `Invoker` by calling a configured service over HTTP, splitting a
decision's `args` across the endpoint's path, query and request body.
`internal/adapter/handler/plan.go` and `invoke.go` implement the generated
`PostPlan`/`PostInvoke`. `Orchestrator.Invoke` is the other mouth: it looks
the endpoint up, checks the arguments against its schema - every required
body field present, every enum value one the enum declares - and calls it.
It never reaches the planner, because a request to /api/invoke is a person
pressing a button rather than a question. The permission check belongs there
once authentication exists; the seat is marked and empty.

`pkg/app.New` now takes a `Config{StaticDir, Services, PlanFixtures}` (its own
exported types, not aliases of anything under `internal/`) and wires the whole
graph, fetching the catalogue at startup. An empty `PlanFixtures` - production's
case, since `cmd/api` never sets one - falls back to a two-entry demo table
(`"在庫の一覧を見せて"` and `"勤怠記録の一覧を見せて"`) hard-coded in `pkg/app/app.go`
(`DECISIONS.md`, 2026-09-11). Verified against inventory and attendance running
on 8081/8082 with `ORCHESTRA_SERVICES` set: `POST /api/plan` with the first demo
query returns `kind: "result"`, `component: "table"` and eight rows; a third
demo query (`"在庫を登録して"`) returns `kind: "form"` with the request body's
schema, its `required` list, the planner's values as `initial` and the
endpoint as `target`, while inventory's row count stays at eight - the write
was described, not performed. An unrecognised query returns `kind: "none"`;
`POST /api/invoke` with a create executes it and returns the created entity
as `component: "detail"`, while an unknown operation, a missing required
field and a value outside an enum each return 400 having called nothing.

`pkg/app.PlanFixture` grew `Answers []Answer`, `Ask bool`, `Question` and
`Param` alongside its existing `Service`/`OperationID`/`Args`, so a fixture can
describe either half of the ask flow; `defaultPlanFixtures()` now has a fourth
demo pair (`"破損した在庫を見せて"` asks about `status`, then the same query with
`{status: quarantined}` answered calls `ListInventoryItems`). Verified against
the running platform: the bare query returns `kind: "ask"` with all four
`ItemStatus` values and their Japanese labels (`allocated`=引当済,
`staged`=出荷準備完了, `quarantined`=検品保留, `consigned`=預託在庫); resubmitted
with the answer it returns `kind: "result"`, `component: "table"`, only the
quarantined rows.

Task 10 replaced the stub's hard-coded default with a real planner:
`internal/adapter/planner/chat` is an OpenAI-compatible chat-completions
transport (`Client.Complete`, httptest-only in `make check`), and
`internal/adapter/planner/toolcall` implements `usecase.Planner` over it,
sending `usecase.ToolsFor(catalog)` as `tools` and mapping the one tool call
back onto a `Decision` - `ask_user` to `DecisionAsk`, any other name to
`DecisionCall` with its `Service` resolved from the catalogue by operation id
(first match wins on a collision, `DECISIONS.md`), no call to `DecisionNone`.
A previous ask's answers are folded into the user turn's own text (there is
no stored conversation - one call per request, D8). Each tool's schema is
shaped for strict mode per tool (`additionalProperties: false` always;
`strict: true` only when every declared property is required), rather than
sent as `usecase.ToolsFor` emits it (`DECISIONS.md`, two entries). A live
test against a real endpoint exists and `t.Skip()`s unless
`ORCHESTRA_LIVE_LLM=1` - confirmed skipped by default.

`pkg/app.Config` gained `LLM{BaseURL, APIKey, Model}` (read from
`ORCHESTRA_LLM_BASE_URL`/`ORCHESTRA_LLM_API_KEY`/`ORCHESTRA_LLM_MODEL` by
`internal/infra/config`); `newPlanner` selects the tool-calling planner when
`LLM.BaseURL` is set and falls back to the stub over `PlanFixtures` otherwise.
`defaultPlanFixtures()` is deleted - an empty `PlanFixtures` with no LLM
configured now means every question comes back `kind: "none"`, not two
hard-coded Japanese questions. `services/platform/.air.toml`'s `full_bin` now
also sets `ORCHESTRA_LLM_BASE_URL=http://localhost:11435/v1` (llama-swap) and
`ORCHESTRA_LLM_MODEL=qwen3.5-9b-q8`, the default local model
(`DECISIONS.md`: the only one of four measured that neither fabricates an
unrequested filter nor drops a requested one).

Verified live (a temporary instance on a spare port, so the running
`make dev-platform` process and its port were never touched) against
`qwen3.5-9b-q8`: `検品保留の在庫を見せて` → `ListInventoryItems {"status":"quarantined"}`,
table, 3 rows; `みなし労働の勤怠を見せて` → `ListAttendanceRecords {"kind":"deemed"}`,
table; `在庫を登録して。名前はテスト品、数量は5、引当済で` → `kind: "form"` with the
parsed values as `initial`; `今日の天気は？` → `kind: "none"`; `在庫を全部見せて` →
`ListInventoryItems` called with **no** `args` at all - confirming this model
does not invent a filter when none was asked for.

## What does not exist yet

Task 11 of `docs/plans/orchestration.md`, the JSON planner for models without
tool calling, and Tasks 12-16, the whole of `web/src` beyond the shell.
`ask_user` is implemented and unit-tested (a fixed tool-call fixture maps to
`DecisionAsk`), but no local model under about 20B parameters was observed to
choose it reliably live - `qwen3.5-9b-q8` picked it 1 run in 5 against a
genuinely ambiguous query, guessing a value the other 4 (`DECISIONS.md`).
`make check` is green because the guards report on the code that is there,
not because the product is finished.

## Known gaps in the harness

- **TypeScript is held at 6.0.3 by a dependency, not by choice.**
  `openapi-typescript` builds its output with the TypeScript Compiler API, which
  TypeScript 7's native implementation does not provide, so `make generate` fails
  under 7. Everything else - including type-aware Oxlint - passed under 7. Go and
  pnpm are current (`DECISIONS.md`, 2026-09-10).
