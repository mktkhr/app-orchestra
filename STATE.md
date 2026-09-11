# STATE.md — current implementation state

_Last updated: 2026-09-12_

## Summary

**Both vertical slices are done, and the third (authentication and
authorisation) has its first three tasks.** `docs/plans/orchestration.md`'s
seventeen tasks, `docs/plans/workspaces.md`'s eight, and now
`docs/plans/auth.md`'s Tasks 0-2: accounts, sessions and permissions exist
in the same SQLite file workspaces already use; the catalogue narrows to
the signed-in person before the planner or `/api/invoke` ever sees it; and
a person can sign in. `internal/adapter/auth/local` checks a name and
password against argon2id-hashed rows and seeds the first admin, once, from
`ORCHESTRA_ADMIN_PASSWORD` (required at startup, the same as
`ORCHESTRA_DB_PATH`). `POST`/`GET`/`DELETE /api/session` sign a person in
(HttpOnly, `SameSite=Lax`, `Secure` cookie carrying an opaque token), report
who is signed in, and sign them out; a session-resolving middleware in
`internal/infra/httpserver` answers every other route 401 without one,
except `GET /api/health`. `make check` is green outside `e2e/`:
`acceptance-e2e` and `acceptance-browser` are red on 401 as of Task 2, on
purpose - Task 6's own job to fix, per `docs/plans/auth.md`.

An answer worth keeping can be kept. A result in the chat can be saved to a
workspace as a panel - service, operation id, arguments, component, title -
and opening the workspace re-runs every panel through the same
`/api/invoke` the chat uses, drawing each answer with the same components.
Workspaces live in a SQLite file (`modernc.org/sqlite`, `ORCHESTRA_DB_PATH`,
required) and survive a restart of the platform process, proven at the
process level, not just in memory. Every criterion in
`docs/specs/workspaces.md` section 10 (AC-W-101 through AC-W-106) has a
test that runs in CI.

The platform's scaffold, two services that answer real
requests, the rendering rule, the catalogue the platform builds by asking
those services what they offer, the tool definitions built from that
catalogue, all four answers `/api/plan` can give - a rendered result, a
form, a disambiguation question, and no match - decided by a real model over
an OpenAI-compatible endpoint (now two planners behind that port: tool
calling and, as of Task 11, JSON for a model that cannot call tools), the
browser drawing all four of those `kind`s, and (Task 16) a process-level
suite and a browser suite that drive the built product end to end.

The whole slice now runs end to end, twice over: once against a real local
model (the platform's own `dev-platform`/`ORCHESTRA_LLM_BASE_URL` path,
proven by the acceptance suites' fixtures standing in for it), and once, for
`make check` itself, against the stub planner - which is the only planner
`make check` is ever allowed to exercise. `e2e/src/orchestration.test.ts`
starts both dummy services and the platform from their built binaries on
free ports and asks `/api/plan` directly; `e2e/browser/chat.spec.ts` drives
the same built product (platform serving `web/dist` via
`ORCHESTRA_STATIC_DIR`) in headless Chromium, clicking the list example
question through to a rendered table. Both feed the stub planner the one
question they ask through `ORCHESTRA_PLAN_FIXTURES`, a new environment
variable `internal/infra/config` decodes - see `DECISIONS.md`, 2026-09-11,
for why: `pkg/app.Config.PlanFixtures` is a Go API a separate OS process
cannot reach, and `make check` must never call a real LLM.

Every gate of `make check` passes, including `guard-a11y` and `guard-layout`
(`make guard-browser`), which measure a real, populated screen - both passed
unchanged, so no accessibility or layout defect was found. Nothing from the
plan remains; `TODO.md` tracks the continuing work under "Next".

**Task 11, the JSON planner, closes the slice.**
`internal/adapter/planner/jsonmode.Planner` is the port's second
implementation: it renders the catalogue as text in the system prompt (one
line per endpoint, one per parameter, an enum's Japanese labels inline -
`renderCatalog`), asks for a single JSON object shaped by `kind` (`call`,
`ask`, `list_capabilities` or `none` - the plan's own JSON examples predate
`list_capabilities`, D14, so they show only three; the implementation
carries all four), and both sets `response_format` (a `json_schema`) and
instructs the same shape in the prompt, so a backend that ignores
`response_format` still has something to answer from. `Planner.complete`
falls back to a bare request, once, only when the endpoint's own HTTP
response is non-2xx - not every OpenAI-compatible backend is documented to
accept `response_format` at all, though llama.cpp/llama-swap, the only
backend measured, does. A `call` or `ask` answer is validated against
`domain.Catalog` (unknown service/operationId, or an argument naming a value
outside its parameter's enum - the check that stands in for tool calling's
`strict: true`, which this transport has no equivalent of) and a bad or
unparseable answer is retried once, quoting the failure back, before giving
up with an error. `Decision.Service`: unlike `toolcall.Planner`, `jsonmode`
needs no `resolveService`-style guess at all - a JSON answer names its own
`service` field directly, so `domain.Catalog` is kept only to validate that
service/operationId pair, not to resolve an ambiguity a tool call's bare
operation-id name would have left open. `ORCHESTRA_LLM_MODE` (`toolcall`,
the default, or `json`) selects the adapter in `pkg/app.newPlanner`; an
unrecognised value fails `internal/infra/config.Load` at startup rather than
silently defaulting.

The one finding worth carrying forward: `response_format`'s JSON Schema
must be built with its properties in the same order the prompt's own
examples already use them in, not whatever order `encoding/json` produces
for a `map[string]any` (alphabetical) - a grammar-constrained local model
(`qwen3.5-9b-q8`) was observed, by hand, to corrupt its own JSON under the
alphabetical order and answer cleanly, every time, once the schema's
`properties` were reordered to match. `jsonmode.decisionSchemaJSON` is a
hand-written `json.RawMessage` for exactly this reason - see `DECISIONS.md`,
2026-09-11, for the full account and the live verification (twelve runs,
four questions three times each, all through `ORCHESTRA_LLM_MODE=json`,
zero retries needed after the fix).

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
fetches each service's `/openapi.yaml`, and converts every operation marked
`x-orchestra-expose: true` (default off - an unmarked, falsely-marked, or
non-boolean-marked operation is dropped, not carried through as something
`ToolsFor` might later exclude) into a `domain.Endpoint`, carrying
`x-enum-labels` through to `Schema.EnumLabels` - the only route by which the
Japanese label for an enum value reaches the model. A service that cannot be
reached fails the whole fetch rather than yielding a partial catalogue.
Verified against both services running: six exposed endpoints (both services'
`GET /openapi.yaml` unmarked and excluded), both enums labelled, every
component the rule picks correct; a POST to `/api/invoke` naming an unexposed
operation id (`GetInventorySpec`) returns 400, identical to an operation id
that does not exist.

**The harness**, imported from takamai and renamed (`DECISIONS.md`,
2026-09-10). `harness/` holds all of it - `quality/` (policy), `guard/`
(implementations), `githooks/`, `claude/`, `gen/` and `quiet.sh`. Every target,
guard, hook and CI job discovers services rather than naming them: a service is
a directory under `services/` with a `go.mod`, and nothing declares the list.

Four guards were added for this design: `guard-generated-ops` fails when a
generated file is missing an operation id its spec declares (oapi-codegen drops
OpenAPI 3.2 `query` operations silently), `guard-operation-ids` fails when two
services claim the same operation id (a tool call carries only the name, so the
name has to mean one thing), `guard-exposed-ops` fails when an
`x-orchestra-expose: true` operation cannot be rendered by any component or a
service exposes nothing at all, and a Redocly plugin fails a spec whose `enum`
carries no `x-enum-labels`.

**Tool definitions**, `services/platform/internal/usecase/tools.go`. `ToolsFor`
turns a `domain.Catalog` into one `Tool` (a plain JSON-Schema map, spec-agnostic
so both the tool-calling and JSON planners of Task 10/11 can render it their
own way) per endpoint the catalogue carries, plus the fixed `AskUserTool`.
It no longer excludes an endpoint by the shape of its schemas - that filter
moved upstream, into `specsource/http.parseSpec`'s `x-orchestra-expose` check
(`DECISIONS.md`, 2026-09-11), which is also why `ToolsFor` and `/api/invoke`
(`Catalog.Find`) can never disagree about which operations exist. An enum
parameter's Japanese labels are appended to its property description
(`allocated=引当済 / staged=出荷準備完了`) - the only route by which they reach a
model.

**"What can this do?"** `ListCapabilitiesTool` (`tools.go`) is a second
built-in tool, alongside `AskUserTool`, that answers a question about the
catalogue itself ("何ができるの？", "在庫について、どういう操作ができる？")
rather than one about a service's data. It carries one optional string
argument, `service` - deliberately not an enum, since the set of services
grows - and `Orchestrator.listCapabilities` answers it entirely from
`catalog.Endpoints` already in memory (no service is ever called): every
endpoint, or only the named service's, as `service`/`operation`/`summary`
rows sorted by `(service, operationId)` for a deterministic table, rendered
as `kind: "result"` / `component: "table"` through the same
`{items: [...]}` envelope shape a real endpoint's list result uses, so
neither `domain.soleArrayProperty` nor the frontend's `rowsFromData` needed
to change. A `service` that matches nothing renders as zero rows rather
than falling back to the whole catalogue. `DecisionListCapabilities` is its
own `DecisionKind`, resolved in `toolcall/planner.go` before the tool name
would otherwise reach `resolveService` (which cannot find it - it is not a
catalogue operation), mirroring how `ask_user` is intercepted first.
`kind: "none"`'s message now also names `list_capabilities`, so a question
the catalogue genuinely cannot answer no longer reads as a dead end
(`DECISIONS.md`, 2026-09-11).

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
parameter name such as `status` from colliding across services. A param the
catalogue does not recognise as an enum - most often a free-text required
field, such as `name` on `CreateInventoryItem`, that the model reached
`ask_user` for anyway because it had nothing to fill in - degrades to
`kind: "form"` instead of an error (`DECISIONS.md`, 2026-09-11: this used to
be a 500, `usecase.ErrUnknownParam`, which is now deleted). A `DecisionKind`
the switch does not recognise at all still returns `usecase.ErrNotImplemented`
(a 501).

The form both this degraded `ask` and an unsafe `DecisionCall` (above) build
is now the same `inputSchemaFor` (`tools.go`) that already builds a tool's
`InputSchema` for the model - an endpoint's parameters and request body
merged into one JSON Schema - not the deleted `formSchema`, which converted
only the request body and left a form empty for a GET-shaped endpoint (query
parameters, no body) such as `GetInventoryItem`. There is now exactly one
converter from a `domain.Endpoint`'s arguments to JSON Schema, not two.
`askUserDescription` (`tools.go`) now says explicitly not to call `ask_user`
for a free-text parameter - measured before and after, alongside the fix
itself, against the running `qwen3.5-9b-q8`: before, `"在庫を登録したい"`
("I want to register some inventory") 500'd 3-4 times in 5; after, every run
returns `kind: "form"` or `kind: "ask"` (about `status`, once the model
reaches a genuine enum question) or `kind: "result"` (`list_capabilities`),
never a 500.

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

**The chat screen**, `web/src/features/conversation` inside the MUI shell.
An empty conversation offers three example questions - a list, a create, and
a status the enum does not have - written out as the questions themselves
rather than labels for them, because an example question is there to show
what a question may look like. Clicking one submits it; so does the form.
Each answer becomes a turn.

A `table` answer is drawn: `entities/rendering` shows the rows ten to a
page, under a header naming the service and operation that produced them and
revealing the arguments the model chose, and an expand control opens the same
rows full-screen. The slice sits in `entities` rather than the `features`
the plan named, because `conversation` has to render it and Feature-Sliced
Design forbids one feature importing another; these components draw the
contract's own shapes, which is what `entities` is for.

The `status` column reads 検品保留, not `quarantined`. A result carries
`fields`, the schema of one row built by the same converter that builds the
tool definitions, with each enum's Japanese labels structured rather than
folded into the model-facing description - a person who asked in Japanese
should not be shown the code.

Rows are found the way the platform found them: the sole array-valued
property of the response object, mirroring `soleArrayProperty` in
`internal/domain/rendering.go`, so inventory's `items` and attendance's
`records` both work without either name appearing in the frontend.

A `detail` answer is a list of the object's fields. A `form` answer is the
inputs its schema describes - a select for an enum, showing the Japanese
label and posting the value, a switch for a boolean, a numeric field for an
integer - prefilled with what the planner chose, and submitting posts to
`/api/invoke` and appends what comes back as another turn.

**`choice`** (Task 15), `web/src/entities/rendering/ui/ResultChoice.tsx`: for
`kind: "ask"`, the planner's disambiguation question plus one MUI `Button`
per option showing its Japanese label. Picking one re-posts `/api/plan` with
the _original_ query (what the person typed, not `PlanResult.question`,
which is the planner's own prompt) alongside `answers: [{param, value}]`, and
reports whatever comes back through the same `onFormSubmitted`/`onSubmitted`
path `ResultForm` already uses to turn a follow-up response into a new turn.

The original query has no home on `Turn` itself - an `ask` answer turn does
not carry a copy of what the person typed, and threading a `query` field
through every producer of an answer turn (including `ResultForm`'s
`/api/invoke` path, which has no query to give) would mean one more thing
each of them has to remember. `TurnList` instead walks the turn array
backward from an `ask` turn's own index to the nearest preceding `role:
"question"` turn - correct even after a choice has already been answered
once, since the turn immediately before a second `ask` may be another
answer, not the question.

Double-submission is guarded by a `useRef` flag set the instant a click is
accepted and cleared once the request settles - not by React's `submitting`
state, which is a render behind the event and let two clicks landing before
a re-render both through in testing. The option row is visually locked (`
opacity`/`pointerEvents: none` in `sx`) once an answer is chosen - in flight
or already answered - rather than through MUI's `disabled` prop: a disabled
`contained` `Button` has no border of its own and fails `make
guard-layout`'s WCAG 1.4.11 contrast check at rest (measured), the same
constraint `ResultForm`'s submit button already works around. The chosen
option stays visible afterwards, drawn `contained` against its `outlined`
siblings, rather than being hidden or disabled further, because the answer
it produced appears as a new turn right below it.

`ResultForm` and `ResultChoice` share the submit-and-report shape - one
async call turned into `submitting`/`error` state and the same Japanese
failure message - through a new `entities/rendering/model/useSubmission.ts`
hook, pulled out to keep `guard-duplication` from flagging the two `try`/
`catch`/`finally` blocks as one repeated structure.

`PlanResult.options` (and any other concrete array-of-object property
`openapi-fetch`'s `MethodResponse` mapping produces) type-checks with its own
prototype methods (`.map`, `.filter`, ...) as `{}` - calling them directly on
it is a compile error, not just an unsafe read. `ResultChoice` reads it the
same defensive way `ResultForm`/`rows.ts` already read a schema whose
declared shape cannot simply be trusted (`Array.isArray` + a `Record<string,
unknown>` narrow, rather than iterating the generated type's own methods).

Verified live against `http://100.75.74.118:5173/` (Tailscale) and the
running `qwen3.5-9b-q8`: of five attempts at `破損した在庫はある？`, four had
the model guess a status on its own (`ListInventoryItems` with no args, or
with `status: quarantined` directly - confirming `DECISIONS.md`'s prior
measurement), and the fifth produced `kind: "ask"` with all four labelled
options; picking 検品保留 re-posted the original query with the answer and
appended a new table turn filtered to the two quarantined items. Fixed
against this behaviour in `Conversation.test.tsx` regardless of what any
particular live run does.

**Task 16, the end-to-end suite, closes the slice.**
`e2e/src/orchestration.test.ts` picks three free TCP ports, starts
`services/inventory/bin/api`, `services/attendance/bin/api` and
`services/platform/bin/api` (`ORCHESTRA_SERVICES` pointed at the first two,
`ORCHESTRA_PLAN_FIXTURES` carrying one fixture), waits for each to answer,
posts `在庫の一覧を見せて` to `/api/plan`, and asserts `kind: "result"`,
`component: "table"`, the inventory source, and a non-empty `data.items` -
then kills all three. `e2e/browser/chat.spec.ts` drives the same three
processes (now started by `e2e/playwright.config.ts`'s array `webServer`,
fixed ports 18083/18084/18080, chosen not to collide with a developer's own
8080/8081/8082 or the harness's own browser gate on 18081) through headless
Chromium: click the list example question, see the question echoed, see
`inventory / ListInventoryItems`, see a `<table>` containing `itm-001`. See
`DECISIONS.md`, 2026-09-11, for `ORCHESTRA_PLAN_FIXTURES` itself.

**Workspaces (`docs/plans/workspaces.md`, all eight tasks) close the second
slice.** `services/platform/internal/domain/workspace.go` (`Workspace`,
`Panel`, stdlib only), `internal/usecase/workspaces.go` (the
`WorkspaceStore` port and the usecase over it), and
`internal/adapter/repository/sqlite` (the store, embedded schema, IDs via
`crypto/rand.Text()`) from Task 0; `POST /api/workspaces`,
`GET /api/workspaces`, `GET`/`DELETE /api/workspaces/{id}`,
`POST /api/workspaces/{id}/panels`, `DELETE
/api/workspaces/{id}/panels/{panelId}` from Task 1, rejecting an operation
the catalogue does not expose with the same 400 `/api/invoke` gives, and an
unknown workspace with 404. `web/src/features/workspaces` (list, create,
delete, save-a-result), `web/src/entities/workspace` (the panel card shell,
`usePanelInvoke`), `web/src/pages/workspace` (the screen) from Tasks 2-6: a
workspace under チャット in the drawer per row with a delete control that
asks first; opening one posts each panel to `/api/invoke` independently -
one unreachable service shows its error in its own card, the rest still
draw (AC-W-106); a result turn in the chat carries a "ワークスペースに保存"
control that copies its `source`/`component` and lets its title be edited,
creating a workspace on the spot if none exists yet (AC-W-101); each panel
card carries its own refresh control, which never disables itself, only
swaps its icon and label while in flight (AC-W-103); the workspace screen
carries the same conversation the chat does, and a result asked from there
defaults its save control to that workspace (AC-W-104).

Task 7 closes it end to end. `e2e/src/workspaces.test.ts` creates a
workspace and a panel over real HTTP against the built platform binary,
stops that process, starts a fresh one on the same `ORCHESTRA_DB_PATH`
file (inventory kept running throughout, since the platform re-fetches its
catalogue at startup and will not start without a configured service
answering), and reads the workspace and its panel back unchanged - AC-W-105,
proven at the process level, not just against an in-memory store.
`e2e/browser/workspace.spec.ts` drives the built product in headless
Chromium: ask the list example question, save the result to a new
workspace, reload (a person returning later does the same), open the
workspace from the drawer, see the same table draw again from a fresh
`/api/invoke` call, then refresh the panel and see the table still draw
(AC-W-101, AC-W-102, AC-W-103). Both suites use their own `mkdtempSync`
database file, so neither sees another suite's workspaces, and both feed
the stub planner through `ORCHESTRA_PLAN_FIXTURES` - no real LLM is ever
called. See `DECISIONS.md`, 2026-09-11 ("Workspaces Task 7"), for why the
process-level suite duplicates rather than imports
`orchestration.test.ts`'s helpers, and why the browser journey reopens the
workspace with a real page reload.

**`docs/plans/auth.md` Task 0 (accounts, sessions and permissions exist).**
`internal/domain/user.go` adds `Role` (`admin`/`user`), `User` and
`Permission` - pure data, no logic, standard library only.
`internal/usecase/auth.go` adds the three ports the plan specifies
(`Authenticator`, `SessionStore`, `PermissionStore`), interfaces only, no
`net/http`/`encoding/json`/`golang.org/x/crypto` in this layer.
`internal/adapter/repository/sqlite` gains three tables (`users`,
`sessions`, `permissions`) in the same embedded `schema.sql`, and three new
store types - `Users`, `Sessions`, `Permissions` - each independently
openable over the same file path. They are separate Go types from the
existing `Store` (not new methods on it) because `usecase.SessionStore.Delete`
and `Store`'s own workspace-deleting `Delete` would otherwise be two
methods of the same name on the same receiver, which Go does not allow;
`openDB` (factored out of `Store.New`) is the one place that opens a
connection and applies the schema, shared by all four types.
`internal/adapter/auth/local` implements `Authenticator`: argon2id (64 MiB,
t=1, p=4, 32-byte key - OWASP's argon2id baseline for a single server with
no secondary defence, RFC 9106 §4) over `sqlite.Users`, encoded as a PHC
string (`$argon2id$v=...$m=...,t=...,p=...$salt$hash`) so a hash carries
its own parameters. `local.New` seeds the first admin - name `"admin"`,
from `ORCHESTRA_ADMIN_PASSWORD` - exactly once: `sqlite.Users.SeedAdminIfNone`
checks `COUNT(*) FROM users` and inserts only when it is zero, so a restart
against a database that already has accounts never overwrites one.
`ORCHESTRA_ADMIN_PASSWORD` is now required at startup, refused empty or
unset the same way `ORCHESTRA_DB_PATH` is (`internal/infra/config`); the
seeding call lives in `pkg/app.build` (`seedAdmin`, skipped when `DBPath`
is empty, mirroring `newWorkspaceHandler`) - nothing consumes the resulting
`Authenticator` yet, since no HTTP handler exists before Task 2, so the
function returns only its error. Verified live, not just in the test
suite: `services/platform/bin/api` started twice against the same
`ORCHESTRA_DB_PATH` file with different `ORCHESTRA_ADMIN_PASSWORD` values
the second time - the admin seeded on the first run is still the one whose
original password authenticates. No HTTP in this task (`docs/plans/auth.md`
says so explicitly); a person still cannot sign in, and the catalogue did
not yet filter by permission (Task 1, done since).

**`docs/plans/auth.md` Task 1 (the catalogue narrows to a person).**
`domain.Catalog.For(permissions []Permission) Catalog` keeps only the
endpoints a permission names; `Orchestrator.Plan` and `Orchestrator.Invoke`
now take `*domain.User` as a parameter, not a context value (A6: a
parameter cannot be forgotten, because the code does not compile without
it), and build the catalogue an admin's request sees as the whole one
(`PermissionStore` is never read for them) or a user's as `For` their own
permissions. `Invoke` refuses an operation the person may not call with the
exact same error `usecase.ErrEndpointNotFound` an unknown one gets, so a
403 never tells a caller a thing exists that they cannot reach. `stubOwner`
is gone from `usecase/workspaces.go` and the `TODO(auth)` comment from
`Orchestrator.Invoke` - both seats Task 0 left open are filled. Until Task
2, every request still resolves to one fixed admin
(`adapter/handler/user.go`'s `currentUser` stub), so this was invisible
from the outside; `make check` stayed green throughout.

**`docs/plans/auth.md` Task 2 (signing in).** `POST /api/session`
(`{name, password}` -> the user, 401 on a wrong one, no cookie set either
way), `GET /api/session` (the signed-in user, or 401) and
`DELETE /api/session` (idempotent sign-out) in
`internal/adapter/handler/session.go`. The session cookie
(`orchestra_session`) is `HttpOnly`, `SameSite=Lax` and `Secure` - the last
one unconditionally, not only over TLS: gosec's `G124` (part of the fixed
lint policy) requires a literal `true`, and every place this runs today -
`make check`'s own `httptest` servers, `harness/quality/browser`'s
Playwright guard, a developer at `localhost` - is a loopback address
`net/http/cookiejar` and every real browser already treat as a secure
origin regardless of scheme; reaching the platform at a non-loopback,
non-HTTPS address (a Tailscale IP) is real but nothing signs in through a
browser yet (Task 4), and TLS termination is the answer for when that
changes, not a weaker cookie (`DECISIONS.md`, 2026-09-12). A new middleware,
`internal/infra/httpserver.requireSession`, resolves the cookie into a user
once per request (`adapter/handler.WithUser`/`currentUser`, replacing the
Task 0/1 fixed-admin stub's body only, as that stub's own doc comment
promised) and answers 401 for everything but `GET /api/health` and
`/api/session` itself without one. `NewRouter` and `pkg/app.build` thread a
`usecase.SessionStore` (nil-able) through; nil - what every `Config` with an
empty `DBPath` builds, `pkg/app`'s own pre-auth tests included - makes every
request run as a fixed stub admin and refuses nothing, matching the
pre-Task-2 behaviour exactly, the same convention `newWorkspaceHandler` and
`newPermissionStore` already use for an empty `DBPath`. Static files
(`ORCHESTRA_STATIC_DIR`) stay outside the authenticated `/api/` mux
entirely, unauthenticated by construction, not by a special case - so a
sign-in screen (Task 4) will still load. `harness/quality/browser/a11y.spec.ts`
and `layout.spec.ts` sign in first now (`harness/quality/browser/session.ts`,
new; a harness change, committed with `ORCHESTRA_ALLOW_HARNESS_CHANGE=1`,
see `DECISIONS.md`) - the screen they measure at `"/"` is still the chat
screen (Task 4 has not built a sign-in one), and every one of its own API
calls would otherwise 401. `e2e/src/*.test.ts`, `e2e/browser/*.spec.ts`
and `services/platform/acceptance`'s pre-existing suites all now 401 on
every protected route without a session; `services/platform/acceptance`
(part of `make check`) was updated to sign in first (`workspace_test.go`)
and gained its own `session_test.go` proving AC-A-101, AC-A-102 and
AC-A-107 at the platform level - `e2e/` was deliberately left red, Task 6's
job per the plan.

## What does not exist yet

Past `docs/plans/auth.md` Task 2: there is no `/api/users` yet (Task 3), no
sign-in screen or admin screen (Tasks 4-5) - a person can sign in over HTTP
but has nowhere in the UI to do it - and `e2e/`'s suites do not sign in yet
(Task 6, expected red until then).

Nothing from `docs/plans/orchestration.md`'s first vertical slice or
`docs/plans/workspaces.md`'s second; all twenty-five tasks between them are
done. `ask_user` is implemented and unit-tested for both planners (a fixed
tool-call fixture, and a fixed `kind: "ask"` JSON fixture, each map to
`DecisionAsk`), but no local model under about 20B parameters was observed
to choose it reliably live - `qwen3.5-9b-q8` picked it roughly 1 run in 5
against a genuinely ambiguous query through the tool-calling planner,
guessing a value the rest of the time (`DECISIONS.md`); not re-measured for
the JSON planner, since none of the questions exercised live for Task 11
were ambiguous enough to reach it.

Everything past the two vertical slices remains future work: authentication
and authorisation (workspaces carry a stub owner column, W6), multi-turn
conversational context (one call per request, D8), a genre/domain layer
above individual services, and the move to TypeScript 7 once
`openapi-typescript` supports it (see "Known gaps in the harness" below).

## Known gaps in the harness

- **TypeScript is held at 6.0.3 by a dependency, not by choice.**
  `openapi-typescript` builds its output with the TypeScript Compiler API, which
  TypeScript 7's native implementation does not provide, so `make generate` fails
  under 7. Everything else - including type-aware Oxlint - passed under 7. Go and
  pnpm are current (`DECISIONS.md`, 2026-09-10).
