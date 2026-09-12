# Orchestration — turning a question into an API call and a screen

_The first vertical slice. The decisions in `PRODUCT.md` section 2 (D1-D4) are
assumed and not repeated here; this document says how the slice is built._

## 1. What it proves

A user types a question in Japanese. One LLM call decides which endpoint of
which service answers it. The platform calls that endpoint and returns the
result together with the component that should render it. The browser renders
it. Nothing about the rendering is decided by a model.

## 2. Decisions taken here

|         | Decision                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **D5**  | The LLM is reached through a `Planner` port in `usecase`. The transport is an OpenAI-compatible chat endpoint - llama.cpp behind llama-swap, vLLM, LM Studio and the hosted providers all speak it - and the model is named per request, so switching model switches backend. Two adapters implement the port: one using tool calling, one asking for JSON in the prompt for models that cannot call tools.                                                                                                                                                                                                                                                                                                                     |
| **D6**  | The platform fetches each service's contract over HTTP from `GET /openapi.yaml`. Services are processes, not files on a shared disk.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| **D7**  | `x-ui-hint.component` exists but only as an override. The rendering rule reads the response schema; the hint wins when present.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| **D8**  | The API result never goes back to the LLM. One request is one LLM call. Both still hold with multi-turn context (`docs/specs/context.md`): what goes back on the next request is the earlier question and the decision made for it - service, operation, arguments - never a row of the answer, and rendering that conversation into the prompt is part of the same one call, not a second one.                                                                                                                                                                                                                                                                                                                                 |
| **D9**  | The component is chosen by the Go platform, not by the browser. The rule lives in `domain` as a pure function.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| **D10** | Enum parameters carry Japanese labels (`x-enum-labels`) and are sent to the model with `strict: true`, so a value outside the enum cannot be returned at all.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| **D11** | An `ask_user` tool lets the model say "I cannot tell which value you mean" and hand the choice back to the person, by picking from that parameter's declared enum values. In practice the model also reaches for `ask_user` when a required parameter has no enum at all - a free-text field it was never told a value for. There is nothing to pick from in that case, so it degrades to the same form an unsafe call already produces (carrying the endpoint's whole argument schema and whatever arguments the model did fill in), rather than an error: the model was still asking a genuine question, just through a tool shaped for an enum it did not have.                                                              |
| **D12** | The shell is `AppBar` + `Drawer` + `List` from Material UI directly. Results render inline in the conversation; a table can be expanded to a full-screen modal. Every result carries its provenance.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| **D13** | An operation reaches the model only when its contract marks it `x-orchestra-expose: true` (default off). The mark is read once, in `specsource/http`, as the catalogue is built - not later, in `ToolsFor` or at `/api/invoke` - so the tool list and the operations `/api/invoke` will answer can never disagree, and a crafted POST cannot reach an operation the model was never offered.                                                                                                                                                                                                                                                                                                                                    |
| **D14** | `list_capabilities` is a further built-in tool, alongside `ask_user`, that answers "what can this do?" from the catalogue itself rather than from the model's own description of it - the same reasoning as D8, applied to a question about the catalogue instead of a question about one service's data.                                                                                                                                                                                                                                                                                                                                                                                                                       |
| **D15** | An optional enum parameter on a safe operation is offered to the model as _required_, carrying one value its contract does not have: `__all__`, labelled すべて. Leaving a filter out and asking for everything then look different to the model, where before they were the same answer. This is measured, not assumed (`DECISIONS.md`): the failure it closes is the model dropping a filter word it could not match and returning every row, which a person reads as the answer to the question they asked. The synthetic value never leaves the platform - the orchestrator strips it before an argument is validated or a call is made - so no service's contract changes and `__all__` appears in no result's provenance. |

## 3. Architecture

```
services/platform/
  internal/domain      Catalog, Endpoint, Call, Rendering. Pure; no I/O.
  internal/usecase     Ports: Planner, Invoker, SpecSource. Orchestrator.
  internal/adapter     planner/anthropic, planner/stub, invoker/http,
                       specsource/http, handler
  internal/infra       config, httpserver
  pkg/app              composition root
services/inventory     dummy service
services/attendance    dummy service
web/                   chat screen, four render components
e2e/                   process-level acceptance
```

Layer order is enforced by `make guard-arch`, which already applies to every
service under `services/`.

## 4. Data flow

1. **Startup.** The platform GETs `/openapi.yaml` from each configured service,
   builds the `Catalog` from only the operations marked `x-orchestra-expose`
   (D13), and converts each into one tool definition. Enum parameters carry
   their `x-enum-labels` in the parameter description.
2. The browser posts the question to `POST /api/plan`.
3. **One LLM call.** Tools (the whole catalogue, cache-friendly because it sits
   at the front of the prompt) plus the question, with `strict: true`.
4. The reply is a `tool_use` block: an operation id and its arguments, already
   structured. Nothing is parsed out of prose.
5. **Safe method** (GET, HEAD, QUERY) — the platform calls the service and
   returns the result plus the component.
6. **Unsafe method** (POST, PUT, PATCH, DELETE) — nothing is called. The form
   definition comes back with the arguments the model filled in as initial
   values.
7. **`ask_user`** — the question and the candidate values come back; the browser
   renders a choice. The answer is posted to `/api/plan` again alongside the
   original question.

`POST /api/invoke` is the separate execution mouth: it takes a service, an
operation id and arguments, calls it, and returns the result. It never calls the
LLM. Every method goes through it, because a person pressed the button.

The property "the model cannot change anything on its own" rests on exactly one
thing: `/api/plan` does not execute unsafe methods.

## 5. Contract of the platform

```
POST /api/plan     { query, answers? }
  -> { kind: "result", component, data, source, fields? }
  -> { kind: "form",   schema, initial, target }
  -> { kind: "ask",    question, param, options }
  -> { kind: "none",   message }

POST /api/invoke   { service, operationId, args }
  -> { component, data, fields? }
```

Stateless: no plan is held server-side, so there is no plan id and no expiry.
`/api/invoke` re-validates its arguments against the catalogue, which is also
where the permission check will go when authentication arrives.

## 6. Rendering rule

A pure function of the response schema, in `domain`:

- object array (optionally wrapped in a paginated envelope) -> `table`
- single object -> `detail`
- the operation has a request body -> `form`
- the model called `ask_user` -> `choice`

`x-ui-hint.component` on the operation overrides the result. Labels come from
the schema's `title` and `description`; no separate label vocabulary.

A `kind: "result"` response also carries `fields`: the per-property JSON
Schema for a table's columns (the row schema) or a detail's own properties,
built by the same conversion the model's tool definitions use
(`schemaToJSONSchema`), so an enum property's `x-enum-labels` are never
derived twice. `fields` exists for two reasons: someone asking a question in
Japanese should not be shown an enum's raw English value (`quarantined`
instead of `検品保留`), and a form's select control needs the value/label
pairs to offer, not just the value. `fields` is absent — not an empty
object — when the result renders as neither `table` nor `detail`.

`POST /api/invoke`'s response carries the same `fields`, built the same way
(`usecase.fieldsFor`, shared by both `Plan`'s safe-call path and `Invoke`).
A form submission's result is exactly as much a `kind: "result"` answer as
one `/api/plan` produced directly — the detail turn it lands in shows a
Japanese title and enum label either way, not just the first time a table or
detail happened to come from `/api/plan`.

## 7. User interface

**Layout.** A bar, a drawer and a list, straight from Material UI. The
navigation holds one entry now and workspaces will be the second, so the shell
is built to take more from the start - but it is built here rather than taken
from a framework, because the one on offer trails Material UI by two majors
(`DECISIONS.md`, 2026-09-11).

**Empty state.** Before the first question the conversation shows example
questions, one per shape the slice can answer - a list, a create, and one that
names a status the enum does not have. They are the demo script, made visible.

**Results are inline.** Each answer renders inside the assistant's turn, so the
conversation and its results form one timeline. That is also what makes saving a
result into a workspace natural later.

**A table can be expanded.** Inline width is tight for a table, so the table
component carries an expand control that opens the same rows in a full-screen
modal. Nothing else expands.

**Every result shows where it came from.** Above the rendered result sit the
service and the operation id that produced it (`inventory / listItems`), with
the arguments the model chose available on expansion (`status=allocated`). One
LLM call means there are no intermediate steps to show; what matters is which
endpoint was picked and with what, because that is what a reviewer of this PoC
judges.

**Components.** `table` (with expand), `detail`, `form`, `choice`, and the
provenance header shared by all four.

## 8. Catalogue and tool definitions

A service's operation only reaches the catalogue at all when its contract
marks it `x-orchestra-expose: true` (default off, D13); everything else -
a health check, an internal admin call, a batch trigger, `GET
/openapi.yaml` itself - is dropped in `specsource/http` before a
`domain.Endpoint` for it ever exists. This is the only filter: it runs
once, at the same place `Catalog.Find` and `ToolsFor` both read from
afterwards, so the tool list offered to the model and the operations
`/api/invoke` will actually answer cannot drift apart. Filtering later
(inside `ToolsFor`, or inside the `/api/invoke` handler) would leave the
mark decorative - a POST straight to `/api/invoke` would still reach an
operation the model was never shown. An operation marked exposed that no
component could ever render (no request body, no 2xx JSON response) is a
mistake in the spec, not a shape to drop quietly; `harness/guard/
exposed-ops.sh` fails the build on it, and on a service that exposes
nothing at all.

One operation becomes one tool. The tool name is the operation id; the
description is the operation summary; the input schema is built from the
parameters and the request body.

An enum parameter is emitted with its `enum` list intact and its Japanese labels
appended to the description, e.g. `allocated=引当済 / staged=出荷準備完了`. With
`strict: true` the model cannot answer with a value outside the list, so an
invented status is structurally impossible rather than merely unlikely.

Models that cannot call tools get the same catalogue as text in the prompt and
answer with JSON. The schema of that JSON is the tool definition by another
name, so the catalogue is built once and rendered two ways; only the adapter
differs. `strict: true` has no equivalent there, so the JSON adapter validates
the answer against the endpoint's parameter schema itself and retries once.

`ask_user` is one further tool, always present, not derived from any spec:

```
ask_user(question: string, service: string, operationId: string, param: string, options: [{ value, label }])
```

`service` and `operationId` name the operation the model was stuck on - the
one it would have called instead of `ask_user`, had the parameter's value
been clear. They are required because a parameter name such as `status` or
`type` is not unique across a catalogue of many services: without naming the
operation, resolving `param` against the catalogue could surface another
service's enum entirely.

`list_capabilities` is a second further tool, also always present, also not
derived from any spec:

```
list_capabilities(service?: string)
```

It answers "what can this do?" - in general, or, with `service`, for one
named service - by listing the catalogue's own operations
(service/operation/summary), rendered as `kind: "result"` /
`component: "table"` exactly like any endpoint's own result. The answer is
built from `domain.Catalog`, not asked of the model as prose: a model's own
description of what it can do can name an operation that does not exist or
miss one that does, and D8 already settled that a result comes from the API,
not from the model's telling - that holds just as much for a question about
the catalogue as for a question about one service's data. `service` is a
plain string rather than an enum, because the set of services is not fixed
the way an endpoint's own declared enum values are; the tool's description
tells the model to use one of the service names it already sees elsewhere in
the catalogue. A `service` that matches nothing in the catalogue renders as
a table with zero rows, not a silent fallback to every service's operations.

### 8a. Asking for everything is an answer

A list operation's filter is optional in its contract, and that is correct:
`GET /api/inventory/items` with no `status` returns every item, and every item
is a thing a person asks for. But an optional parameter gives the model two
ways to say nothing, and they are the same answer to the schema and different
answers to a person. 在庫を全部見せて means leave it out. 破損した在庫はある？
also leaves it out, whenever the model cannot match 破損 against any declared
status - and the person is handed every row, presented as the answer to a
question about one kind of row. Nothing on the screen says a filter went
missing.

That is measured rather than supposed. On the default model it happened in 4
of 5 runs by hand, and the eval corpus puts it at 18 of 30 for inventory and 9
of 10 for attendance, where 有給 has no near neighbour among みなし労働 /
振替休日 / 代休 / 待機 for the model to guess at instead (`DECISIONS.md`,
`docs/specs/eval.md`). The worse number is the telling one: the model is not
guessing badly, it is declining to answer in the one way the schema lets it
decline silently.

So the fix is not a list of words to watch for - a list would have to be
written in Japanese, per service, and would be wrong the first time somebody
added a status. It is to take away the option of saying nothing. On a safe
operation, an enum parameter is offered to the model as required, and its enum
carries one value the contract does not: `__all__`. Omitting is then not a
legal answer at all, and the model has exactly three left - name a declared
value, name `__all__`, or reach for `ask_user` (D11). The silent drop has no
shape to take.

`__all__` is a request for every row, which is what an absent filter already
meant, so the platform removes it rather than passing it on: the argument is
stripped after the catalogue lookup and before anything is validated or
called, which is also before a result's provenance is built - `source.args`
reads `{}`, exactly as it did when the model omitted the parameter. A service
never learns the value exists, and a workspace panel saved from such a result
holds no `__all__` to replay.

Safe operations only, and parameters only - never a request body's own
properties. A create's `status` is a field of the thing being created, and
"all" is not a status an item can be in.

## 9. Error handling

- No endpoint fits -> `kind: "none"`, whose message points at `list_capabilities`
  ("何ができるの？") rather than leaving the question at a dead end.
- A value outside the enum -> impossible; `strict: true` rejects it at the API.
- A value the model cannot pin down -> `ask_user` -> `kind: "ask"`.
- A question about what the platform can do -> `list_capabilities` ->
  `kind: "result"` / `component: "table"`, built from the catalogue.
- The service returns an error -> its error envelope is passed through with
  `component: "error"`.
- The LLM call itself fails -> 500 after one retry.

## 10. Testing

**`make check` does not call an LLM.** `planner/stub` is the default adapter in
tests: it maps a fixed question to a fixed `Call`. The real adapter is exercised
only when `ORCHESTRA_LIVE_LLM=1` is set, which CI never sets.

- The rendering rule and the catalogue conversion are pure functions covered by
  unit tests in `services/platform`.
- Acceptance tests drive the real object graph over HTTP with the stub planner.
- The browser suite drives the built product.

## 11. Dummy services

Both carry enums whose meaning cannot be guessed from the English word, which is
what makes the label mechanism worth testing.

**inventory** — `status`: `allocated` (引当済), `staged` (出荷準備完了),
`quarantined` (検品保留), `consigned` (預託在庫).

**attendance** — `kind`: `deemed` (みなし労働), `substitute` (振替休日),
`compensatory` (代休), `on_call` (待機).

`substitute` and `compensatory` are two different things in Japanese labour
practice and neither word carries that meaning on its own; a model without the
labels gets them wrong.

Each service exposes three operations - list, detail, create - plus
`GET /openapi.yaml`.

## 12. Acceptance criteria

They live in `PRODUCT.md` section 5, which is where `docs/acceptance.md` says
they belong: `AC-B-101` to `AC-B-106` for the platform, `AC-F-101` to
`AC-F-103` for the web app, `AC-E-101` end to end.

## 13. Deliberately excluded

- Authentication and authorisation. A stub user; the seat where the permission
  filter belongs is left open in the catalogue and in `/api/invoke`.
- Workspaces, multi-turn context, and the genre layer between service and API.
- Summarising the result in prose (it would double the LLM calls).
- The QUERY method. `oapi-codegen` v2.8.0 accepts a valid OpenAPI 3.2 document
  containing a `query` operation and silently generates nothing for it - no
  error, no warning, an empty `ServerInterface`. Search endpoints use POST until
  the generator catches up.

## 14. Harness work this implies

- A guard that fails when the generated code is missing an operation id present
  in the spec. `guard-generated` only checks freshness, so today's silent drop
  of a QUERY operation passes it.
- A Redocly rule that fails a spec where an `enum` has no `x-enum-labels`, so
  the labels the model depends on cannot be forgotten.
- A guard (`harness/guard/exposed-ops.sh`) that fails a spec where an
  `x-orchestra-expose: true` operation cannot be rendered by any component,
  or a service marks nothing exposed at all.
