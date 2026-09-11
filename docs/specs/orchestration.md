# Orchestration — turning a question into an API call and a screen

_The first vertical slice. The decisions in `PRODUCT.md` section 2 (D1-D4) are
assumed and not repeated here; this document says how the slice is built._

## 1. What it proves

A user types a question in Japanese. One LLM call decides which endpoint of
which service answers it. The platform calls that endpoint and returns the
result together with the component that should render it. The browser renders
it. Nothing about the rendering is decided by a model.

## 2. Decisions taken here

|         | Decision                                                                                                                                                                                                                                                                                                                                                                                                    |
| ------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **D5**  | The LLM is reached through a `Planner` port in `usecase`. The transport is an OpenAI-compatible chat endpoint - llama.cpp behind llama-swap, vLLM, LM Studio and the hosted providers all speak it - and the model is named per request, so switching model switches backend. Two adapters implement the port: one using tool calling, one asking for JSON in the prompt for models that cannot call tools. |
| **D6**  | The platform fetches each service's contract over HTTP from `GET /openapi.yaml`. Services are processes, not files on a shared disk.                                                                                                                                                                                                                                                                        |
| **D7**  | `x-ui-hint.component` exists but only as an override. The rendering rule reads the response schema; the hint wins when present.                                                                                                                                                                                                                                                                             |
| **D8**  | The API result never goes back to the LLM. One request is one LLM call.                                                                                                                                                                                                                                                                                                                                     |
| **D9**  | The component is chosen by the Go platform, not by the browser. The rule lives in `domain` as a pure function.                                                                                                                                                                                                                                                                                              |
| **D10** | Enum parameters carry Japanese labels (`x-enum-labels`) and are sent to the model with `strict: true`, so a value outside the enum cannot be returned at all.                                                                                                                                                                                                                                               |
| **D11** | An `ask_user` tool lets the model say "I cannot tell which value you mean" and hand the choice back to the person.                                                                                                                                                                                                                                                                                          |
| **D12** | The shell is `AppBar` + `Drawer` + `List` from Material UI directly. Results render inline in the conversation; a table can be expanded to a full-screen modal. Every result carries its provenance.                                                                                                                                                                                                        |

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
   builds the `Catalog`, and converts every operation into one tool definition.
   Enum parameters carry their `x-enum-labels` in the parameter description.
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
  -> { kind: "result", component, data, source }
  -> { kind: "form",   schema, initial, target }
  -> { kind: "ask",    question, param, options }
  -> { kind: "none",   message }

POST /api/invoke   { service, operationId, args }
  -> { component, data }
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

## 9. Error handling

- No endpoint fits -> `kind: "none"`.
- A value outside the enum -> impossible; `strict: true` rejects it at the API.
- A value the model cannot pin down -> `ask_user` -> `kind: "ask"`.
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
