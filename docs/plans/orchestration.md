# Orchestration — implementation plan

> **For the implementer:** one task at a time. Each task ends green and is
> committed on its own. Steps use `- [ ]` for tracking.

**Goal:** A question in Japanese becomes one API call against one of two dummy
services, and its result becomes a rendered screen.

**Architecture:** Go platform with Clean Architecture layers; the LLM sits
behind a `Planner` port and is called exactly once per request. The component
that renders a result is chosen by a pure function over the response schema, not
by a model. A Vite + React Router SPA renders what the platform names.

**Tech stack:** Go 1.27.1, OpenAPI 3.0.3 + oapi-codegen, Vite+ / React 19 /
MUI / Toolpad Core, openapi-typescript + openapi-fetch.

**Spec:** `docs/specs/orchestration.md`. Acceptance criteria: `PRODUCT.md`
section 5.

## Global constraints

- `make check` must never call a real LLM. `planner/stub` is the default in
  every test; the Anthropic adapter runs only when `ORCHESTRA_LIVE_LLM=1`.
- Contract first: change `services/<name>/api/openapi.yaml`, run
  `make generate`, implement against the generated code. Never edit generated
  files.
- Layer order is enforced by `make guard-arch`: domain ← usecase ← adapter ←
  infra ← app ← cmd. Dependencies point inwards only.
- Every hand-written file stays under 1000 lines (`make guard-filelen`).
- Every Go package needs 90% statement coverage (`make guard-coverage`), except
  `cmd/` and `internal/adapter/openapi` which are set to 0.
- Controls come from MUI. Writing `<button>`, `<input>`, `<select>`,
  `<textarea>` or `<option>` as JSX fails `make guard-ui`. A form is
  `<Box component="form">`.
- Every lint suppression must be registered in
  `harness/quality/suppressions.allow`.
- No `any`, no `@ts-ignore`, no `console`, no default exports, no direct
  `fetch` outside `web/src/shared`.
- Every enum in a spec carries `x-enum-labels` covering exactly its values.
- Do not touch anything under `harness/`, `.github/`, `Makefile`,
  `vite.config.ts`, `tsconfig*.json`, `go.work`, `pnpm-workspace.yaml`,
  `AGENTS.md`, `CLAUDE.md` or `.claude/`. Those are the harness.

## File structure

```
services/inventory/            dummy service: stock items
  api/openapi.yaml             3 operations + GET /openapi.yaml
  cmd/api/main.go              entry point
  internal/domain/             Item, Status
  internal/adapter/handler/    implements the generated interface
  internal/adapter/repository/ in-memory fixture data
  internal/infra/              config, httpserver (mirrors platform's shape)
  pkg/app/                     composition root
  acceptance/                  separate module
services/attendance/           dummy service: attendance records — same shape
services/platform/
  internal/domain/             Catalog, Endpoint, Call, Rendering, the render rule
  internal/usecase/            Planner, Invoker, SpecSource ports; Orchestrator
  internal/adapter/planner/stub       deterministic, used by every test
  internal/adapter/planner/anthropic  tool use; live only
  internal/adapter/specsource/http    fetches /openapi.yaml
  internal/adapter/invoker/http       calls a service operation
  internal/adapter/handler/           /api/plan, /api/invoke
  acceptance/                  separate module
web/src/
  app/                         DashboardLayout, router, providers
  pages/chat/                  the one screen
  features/conversation/       turns, welcome state, submit
  features/rendering/          table (+ expand), detail, form, choice, provenance
  shared/api/                  generated types + client
e2e/                           process-level suite
```

---

### Task 1: inventory service

**Files:**

- Create: `services/inventory/api/openapi.yaml`, `cmd/api/main.go`,
  `internal/domain/item.go`, `internal/adapter/handler/api.go`,
  `internal/adapter/repository/memory.go`, `internal/infra/config/config.go`,
  `internal/infra/httpserver/{router,server}.go`, `pkg/app/app.go`,
  `go.mod`, `acceptance/{go.mod,api_test.go}`
- Modify: `go.work`

**Produces:** a process serving `GET /api/inventory/items`,
`GET /api/inventory/items/{id}`, `POST /api/inventory/items`, and
`GET /openapi.yaml` returning its own spec.

The `status` enum and its labels, verbatim:

```yaml
status:
  type: string
  enum: [allocated, staged, quarantined, consigned]
  x-enum-labels:
    allocated: 引当済
    staged: 出荷準備完了
    quarantined: 検品保留
    consigned: 預託在庫
```

`GET /api/inventory/items` takes an optional `status` query parameter of that
type. Fixture data holds at least two items per status so filtering is
observable.

- [ ] **Step 1** Write `api/openapi.yaml`. Every operation needs an
      `operationId`, a `summary`, and a tag with a description (`make api-lint`
      requires them). Model the service on the layer layout above.
- [ ] **Step 2** `make api-lint` — expect it to pass. Fix the spec until it does.
- [ ] **Step 3** Add the module to `go.work`, run `make generate`. Confirm
      `services/inventory/internal/adapter/openapi/openapi.gen.go` now declares
      `ListInventoryItems`, `GetInventoryItem`, `CreateInventoryItem`.
- [ ] **Step 4** Write the acceptance test first: it starts the real object
      graph from `pkg/app` behind `httptest`, asks for `status=allocated`, and
      asserts only allocated items come back. Run it, expect it to fail.
- [ ] **Step 5** Implement domain, repository, handler, app, cmd, infra until
      that test passes.
- [ ] **Step 6** Add a test asserting `GET /openapi.yaml` returns the embedded
      spec and that it parses. Make it pass.
- [ ] **Step 7** `make services-lint services-test guard-arch guard-coverage
acceptance-services` — all green. Coverage is 90%: test the domain and the
      repository directly, not only through HTTP.
- [ ] **Step 8** Commit: `feat(inventory): serve stock items and the contract`

---

### Task 2: attendance service

Identical in shape to Task 1. Do not skim Task 1 — repeat its steps.

**Files:** same layout under `services/attendance/`.

**Produces:** `GET /api/attendance/records`,
`GET /api/attendance/records/{id}`, `POST /api/attendance/records`, and
`GET /openapi.yaml`.

The `kind` enum, verbatim:

```yaml
kind:
  type: string
  enum: [deemed, substitute, compensatory, on_call]
  x-enum-labels:
    deemed: みなし労働
    substitute: 振替休日
    compensatory: 代休
    on_call: 待機
```

`substitute` and `compensatory` are different things in Japanese labour
practice; the fixture data must contain both so a query for one does not
accidentally match the other.

- [ ] **Step 1-8** As Task 1, substituting attendance for inventory.
- [ ] Commit: `feat(attendance): serve attendance records and the contract`

---

### Task 3: platform domain and the rendering rule

**Files:**

- Create: `services/platform/internal/domain/{catalog,call,rendering}.go`
  and their tests
- Modify: `go.work` (platform is already a module)

**Produces:**

```go
type Component string
const (
    ComponentTable  Component = "table"
    ComponentDetail Component = "detail"
    ComponentForm   Component = "form"
    ComponentChoice Component = "choice"
)

type Endpoint struct {
    Service     string
    OperationID string
    Method      string   // GET, POST, ...
    Path        string
    Summary     string
    Parameters  []Parameter
    RequestBody *Schema
    Response    *Schema
    UIHint      Component // empty when the spec gives none
}

type Parameter struct {
    Name        string
    In          string // query, path
    Required    bool
    Schema      Schema
}

type Schema struct {
    Type       string            // object, array, string, integer, boolean
    Items      *Schema
    Properties map[string]Schema
    Enum       []string
    EnumLabels map[string]string
    Title      string
    Description string
}

type Catalog struct{ Endpoints []Endpoint }
func (c Catalog) Find(service, operationID string) (Endpoint, bool)

// Render is a pure function. No I/O, no model.
func Render(e Endpoint) Component

func (e Endpoint) IsSafe() bool // GET, HEAD, QUERY
```

`Render` rules, in order: the endpoint's `UIHint` when set; then a request body
means `form`; then an object array (directly, or as the single array-valued
property of a wrapper object) means `table`; then an object means `detail`.

- [ ] **Step 1** Write the table test for `Render` covering: hint override,
      request body, bare object array, array wrapped in `{items: [...], total: n}`,
      single object. Run it, expect a compile failure.
- [ ] **Step 2** Define the types above, leave `Render` returning `""`.
      Run: still failing, now on assertions.
- [ ] **Step 3** Implement `Render` and `IsSafe`. Tests pass.
- [ ] **Step 4** `make guard-coverage` — domain must reach 90%. Add cases until
      it does; every branch of `Render` needs one.
- [ ] **Step 5** Commit: `feat(platform): decide the component from the schema`

---

### Task 4: catalogue from running services

**Files:**

- Create: `services/platform/internal/usecase/ports.go`,
  `internal/adapter/specsource/http/{source,parse}.go` and tests
- Modify: `services/platform/internal/infra/config/config.go`

**Consumes:** `domain.Catalog`, `domain.Endpoint`, `domain.Schema` (Task 3).

**Produces:**

```go
// usecase
type SpecSource interface {
    Fetch(ctx context.Context) (domain.Catalog, error)
}
```

The HTTP implementation reads a list of service base URLs from
`ORCHESTRA_SERVICES` (comma separated, `name=url` pairs), GETs
`<url>/openapi.yaml` from each, and converts every operation into a
`domain.Endpoint`. `x-enum-labels` lands in `Schema.EnumLabels`;
`x-ui-hint.component` lands in `Endpoint.UIHint`.

- [ ] **Step 1** Write a test that serves a fixture spec from `httptest` and
      asserts the resulting catalogue: operation count, one endpoint's method, path,
      a parameter's enum values and its labels. Run it, expect failure.
- [ ] **Step 2** Implement the parser and the fetcher until it passes. A
      service that cannot be reached fails the whole fetch — a partial catalogue
      would silently hide endpoints.
- [ ] **Step 3** Add a test that a spec with `x-ui-hint.component: detail` on an
      operation produces that hint. Make it pass.
- [ ] **Step 4** `make services-lint guard-arch guard-coverage` — green.
- [ ] **Step 5** Commit: `feat(platform): build the catalogue from running services`

**Satisfies:** the catalogue half of AC-B-101.

---

### Task 5: catalogue to tool definitions

**Files:**

- Create: `services/platform/internal/usecase/tools.go` and its test

**Consumes:** `domain.Catalog` (Task 3).

**Produces:**

```go
type Tool struct {
    Name        string          // the operation id
    Description string          // the operation summary
    InputSchema map[string]any  // JSON Schema
    Strict      bool            // always true
}

func ToolsFor(c domain.Catalog) []Tool   // one per endpoint, plus AskUserTool
var AskUserTool Tool                      // ask_user(question, param, options)
```

An enum parameter keeps its `enum` list in the JSON Schema and appends its
labels to that property's description as `allocated=引当済 / staged=出荷準備完了`.
This is the only place the Japanese labels reach the model.

- [ ] **Step 1** Write the test: a catalogue with one enum parameter produces a
      tool whose input schema carries the enum values and whose description contains
      both the English value and the Japanese label. Assert `Strict` is true and
      that `ask_user` is present exactly once. Run it, expect failure.
- [ ] **Step 2** Implement `ToolsFor` and `AskUserTool`. Tests pass.
- [ ] **Step 3** `make guard-coverage` — 90%.
- [ ] **Step 4** Commit: `feat(platform): convert the catalogue into tool definitions`

**Satisfies:** the tool-definition half of AC-B-101.

---

### Task 6: planner port, stub, and the safe path of /api/plan

**Files:**

- Create: `services/platform/internal/usecase/{planner.go,orchestrator.go}`,
  `internal/adapter/planner/stub/stub.go`,
  `internal/adapter/invoker/http/invoker.go`,
  `internal/adapter/handler/plan.go`, and tests
- Modify: `services/platform/api/openapi.yaml`, `pkg/app/app.go`

**Consumes:** Tasks 3-5.

**Produces:**

```go
// usecase
type Planner interface {
    Plan(ctx context.Context, query string, answers []Answer, tools []Tool) (Decision, error)
}
type Answer struct{ Param, Value string }

type Decision struct {
    Kind        DecisionKind // call | ask | none
    Service     string
    OperationID string
    Args        map[string]any
    Question    string
    Param       string
    Options     []domain.Option   // value + label
}

type Invoker interface {
    Invoke(ctx context.Context, e domain.Endpoint, args map[string]any) (any, error)
}

type Orchestrator struct{ /* catalog, planner, invoker */ }
func (o *Orchestrator) Plan(ctx context.Context, query string, answers []Answer) (Result, error)
```

`Result` carries the `kind` the HTTP contract returns (`result` / `form` /
`ask` / `none`), the component, the data, and the provenance (service,
operation id, args).

The stub planner maps an exact query string to a `Decision` from a table its
constructor takes. Tests build it explicitly; nothing guesses.

`POST /api/plan` shape is in `docs/specs/orchestration.md` section 6.

- [ ] **Step 1** Add `/api/plan` and `/api/invoke` to the platform's
      `api/openapi.yaml`. `make api-lint`, then `make generate`.
- [ ] **Step 2** Write the acceptance test: two fixture services behind
      `httptest`, a stub planner returning a call to the inventory list endpoint,
      post a question, assert `kind: "result"`, `component: "table"`, and that the
      service actually received the request. Run it, expect failure.
- [ ] **Step 3** Implement the orchestrator's safe path and the handler until
      it passes: safe method → invoke → render → result.
- [ ] **Step 4** Add the `none` case: a stub planner returning
      `DecisionKind("none")` produces `kind: "none"` with a message and calls
      nothing. Make it pass.
- [ ] **Step 5** `make check`'s Go half: `services-lint services-test guard-arch
guard-coverage acceptance-services` — green.
- [ ] **Step 6** Commit: `feat(platform): answer a question with a rendered result`

**Satisfies:** AC-B-102, AC-B-106.

---

### Task 7: unsafe methods return a form

**Files:**

- Modify: `services/platform/internal/usecase/orchestrator.go`,
  `internal/adapter/handler/plan.go`, plus tests

**Consumes:** Task 6.

**Produces:** `Result` with `kind: "form"`, carrying the request body schema and
the arguments the planner filled in as initial values.

- [ ] **Step 1** Write the acceptance test: a stub planner returning a call to
      the inventory _create_ endpoint with `{name: "…", status: "allocated"}`. Assert
      the response is `kind: "form"`, that the schema describes the request body,
      that the initial values are present, **and that the service received no
      request at all**. Run it, expect failure.
- [ ] **Step 2** Implement: `IsSafe()` false → build the form, do not invoke.
- [ ] **Step 3** Green on the Go gates.
- [ ] **Step 4** Commit: `feat(platform): return a form instead of writing`

**Satisfies:** AC-B-103.

---

### Task 8: /api/invoke

**Files:**

- Create: `services/platform/internal/adapter/handler/invoke.go` and tests
- Modify: `internal/usecase/orchestrator.go`

**Consumes:** Tasks 3-7.

**Produces:**

```go
func (o *Orchestrator) Invoke(ctx context.Context, service, operationID string, args map[string]any) (Result, error)
```

It looks the endpoint up in the catalogue, rejects an unknown one, validates the
arguments against the endpoint's schema, invokes, and renders. It never calls
the planner. This is where the permission check goes when authentication
arrives; leave the seat, do not build it.

- [ ] **Step 1** Write the acceptance test: post a create to `/api/invoke`,
      assert the service received it and the created entity comes back rendered as
      `detail`. Run it, expect failure.
- [ ] **Step 2** Add a test that an unknown operation id returns 400 and calls
      nothing.
- [ ] **Step 3** Implement until both pass.
- [ ] **Step 4** Green on the Go gates.
- [ ] **Step 5** Commit: `feat(platform): execute a confirmed call`

**Satisfies:** AC-B-104.

---

### Task 9: ask_user

**Files:**

- Modify: `services/platform/internal/usecase/orchestrator.go`,
  `internal/adapter/handler/plan.go`, plus tests

**Consumes:** Tasks 5-6.

**Produces:** `Result` with `kind: "ask"` carrying the question, the parameter
name, and the options with their Japanese labels taken from
`Schema.EnumLabels`.

- [ ] **Step 1** Write the acceptance test: a stub planner returning an
      `ask` decision for the `status` parameter. Assert the response lists all four
      values with their Japanese labels and that no service was called. Run it,
      expect failure.
- [ ] **Step 2** Add a second test: posting the same question again with
      `answers: [{param: "status", value: "allocated"}]` reaches the planner with
      those answers and produces a result.
- [ ] **Step 3** Implement until both pass.
- [ ] **Step 4** Green on the Go gates.
- [ ] **Step 5** Commit: `feat(platform): hand an ambiguous value back to the user`

**Satisfies:** AC-B-105.

---

### Task 10: OpenAI-compatible transport and the tool-calling planner

**Files:**

- Create: `services/platform/internal/adapter/planner/chat/{client,config}.go`,
  `internal/adapter/planner/toolcall/planner.go`, and tests
- Modify: `services/platform/internal/infra/config/config.go`, `pkg/app/app.go`

**Consumes:** Tasks 5-6 (`Tool`, `ToolsFor`, `Planner`, `Decision`).

**Produces:**

```go
// adapter/planner/chat — transport shared by both planner adapters
type Config struct {
    BaseURL string // ORCHESTRA_LLM_BASE_URL, e.g. http://localhost:8080/v1
    APIKey  string // ORCHESTRA_LLM_API_KEY, may be empty for a local runtime
    Model   string // ORCHESTRA_LLM_MODEL, overridable per request
}
type Client struct{ /* http.Client, Config */ }
func New(cfg Config) *Client
func (c *Client) Complete(ctx context.Context, req Request) (Response, error)
```

`Request` carries messages, an optional `tools` array and an optional
`response_format`. This is the OpenAI chat-completions shape, which llama.cpp
behind llama-swap, vLLM, LM Studio and the hosted providers all accept. The
model travels in the request, so naming a different model is how a different
backend gets used — llama-swap switches on exactly that.

`toolcall.Planner` implements `usecase.Planner`: it sends `ToolsFor(catalog)`
as `tools`, asks for one call, and maps the returned tool call onto `Decision`.
`ask_user` maps to `DecisionKind("ask")`; any other name maps to a call; no tool
call at all maps to `none`.

- [ ] **Step 1** Write the transport test against `httptest`: assert the request
      body carries the model, the messages and the tools, and that a canned
      response decodes. Run it, expect failure.
- [ ] **Step 2** Implement `chat.Client`. Test passes.
- [ ] **Step 3** Write the mapping test for `toolcall.Planner` with a canned
      tool-call response as a fixture — no network. Assert operation id and
      arguments land in `Decision`, and that an `ask_user` call maps to `ask`. Run
      it, expect failure.
- [ ] **Step 4** Implement the planner. Tests pass.
- [ ] **Step 5** Write the live test. It must `t.Skip()` unless
      `ORCHESTRA_LIVE_LLM=1`. Confirm with `go test -v` that it skips by default.
- [ ] **Step 6** Wire adapter selection in `pkg/app` from configuration: the
      stub unless a base URL is configured.
- [ ] **Step 7** `make services-lint services-test guard-arch guard-coverage`
      green, and the test output shows the live test skipped.
- [ ] **Step 8** Commit: `feat(platform): plan through an OpenAI-compatible endpoint`

---

### Task 11: JSON planner for models without tool calling

**Files:**

- Create: `services/platform/internal/adapter/planner/jsonmode/planner.go`
  and tests
- Modify: `services/platform/internal/infra/config/config.go`, `pkg/app/app.go`

**Consumes:** Task 10 (`chat.Client`), Task 5 (`ToolsFor`).

**Produces:** a second `usecase.Planner` for models that cannot call tools.

It renders the same catalogue into the prompt as text — one line per endpoint
with its operation id, summary and parameters, enum values with their Japanese
labels — and asks for a single JSON object:

```json
{ "kind": "call", "service": "inventory", "operationId": "listInventoryItems",
  "args": { "status": "allocated" } }
{ "kind": "ask", "param": "status", "question": "…" }
{ "kind": "none" }
```

It sets `response_format` to a JSON schema when the endpoint accepts one and
falls back to asking for JSON in the prompt when it does not. Either way the
answer is parsed, then **validated against the endpoint's parameter schema** —
an enum value outside the list is rejected here, because nothing enforces it on
the model's side the way `strict: true` does for tool calling. A rejected or
unparseable answer is retried once with the failure quoted back; a second
failure returns an error.

- [ ] **Step 1** Write the test: a canned response containing a valid JSON
      object produces the matching `Decision`. Run it, expect failure.
- [ ] **Step 2** Implement rendering, parsing and mapping. Test passes.
- [ ] **Step 3** Write the validation test: a response naming an enum value the
      parameter does not allow is rejected, retried once, and the second answer is
      used. Assert the retry prompt quotes the rejected value. Make it pass.
- [ ] **Step 4** Write the give-up test: two bad answers produce an error, and
      no service is called.
- [ ] **Step 5** Add a live test guarded by `ORCHESTRA_LIVE_LLM=1`, skipped by
      default.
- [ ] **Step 6** Select the adapter from configuration: `ORCHESTRA_LLM_MODE` is
      `toolcall` or `json`, defaulting to `toolcall`.
- [ ] **Step 7** Go gates green; live tests skipped.
- [ ] **Step 8** Commit: `feat(platform): plan with JSON for models without tool calling`

---

### Task 12: web shell, conversation, welcome state

**Files:**

- Create: `web/src/app/{App.tsx,router.tsx,theme.ts}`,
  `web/src/pages/chat/`, `web/src/features/conversation/`,
  `web/src/shared/api/client.ts`, `web/index.html` entry
- Modify: `web/package.json` (add `@toolpad/core`)

**Consumes:** the platform's generated TypeScript types
(`web/src/shared/api/gen/platform.d.ts`, produced by `make generate`).

**Produces:** one screen inside a Toolpad Core `DashboardLayout`: a turn list, a
question input, and — when there are no turns — three example questions, one per
shape (a list, a create, a status the enum does not have). Clicking an example
submits it.

Controls come from MUI. The form is `<Box component="form">`. `make guard-ui`
fails on a raw `<button>` or `<input>`.

- [ ] **Step 1** Write the test: render `App` with a scripted API, assert the
      three examples are present. Run it, expect failure.
- [ ] **Step 2** Add `@toolpad/core`, build the shell and the conversation until
      it passes.
- [ ] **Step 3** Add a test that submitting a question posts to `/api/plan` and
      appends an assistant turn.
- [ ] **Step 4** `make web-lint web-test guard-fsd guard-ui guard-duplication`
      — green. FSD: a page may import features, a feature may not import a page.
- [ ] **Step 5** Commit: `feat(web): chat shell with an example-led empty state`

**Satisfies:** AC-F-104.

---

### Task 13: provenance, table, expand

**Files:**

- Create: `web/src/features/rendering/{Provenance,ResultTable}.tsx`,
  `web/src/features/rendering/index.ts`, plus tests

**Consumes:** Task 12.

**Produces:** a `Provenance` header showing `service / operationId` with the
arguments revealed on expansion, and a `ResultTable` rendering rows from
`kind: "result"` with `component: "table"`, paginated, with an expand control
that opens the same rows in a full-screen MUI `Dialog`.

Columns come from the keys of the first row; headers use the schema `title`
when the platform supplies one.

- [ ] **Step 1** Write the test: given a table result of 30 rows, the first page
      renders 10, the provenance shows `inventory / listInventoryItems`, and
      expanding the arguments reveals `status=allocated`. Run it, expect failure.
- [ ] **Step 2** Implement `Provenance` and `ResultTable`.
- [ ] **Step 3** Add the expand test: clicking expand opens a dialog containing
      the same rows; closing returns.
- [ ] **Step 4** Web gates green.
- [ ] **Step 5** Commit: `feat(web): render a table with its provenance`

**Satisfies:** AC-F-101, AC-F-105, AC-F-106.

---

### Task 14: detail and form

**Files:**

- Create: `web/src/features/rendering/{ResultDetail,ResultForm}.tsx` and tests

**Consumes:** Tasks 12-13.

**Produces:** `ResultDetail` for `component: "detail"`, and `ResultForm` for
`kind: "form"` — inputs derived from the schema (string → `TextField`, enum →
`TextField select` with `MenuItem` per option using its Japanese label, boolean
→ `Switch`, integer → numeric `TextField`), prefilled with the planner's values,
and a submit that posts to `/api/invoke` and appends the result as a new turn.

- [ ] **Step 1** Write the form test: given a form result whose schema has a
      `status` enum with labels and an initial value of `allocated`, the select
      shows the Japanese labels and `引当済` is selected. Run it, expect failure.
- [ ] **Step 2** Implement both components.
- [ ] **Step 3** Add the submit test: submitting posts to `/api/invoke` with the
      edited values and appends a turn.
- [ ] **Step 4** Web gates green. Watch `guard-duplication`: the two components
      share shape, so extract what repeats rather than copying it.
- [ ] **Step 5** Commit: `feat(web): render a detail card and a submit form`

**Satisfies:** AC-F-102.

---

### Task 15: choice

**Files:**

- Create: `web/src/features/rendering/ResultChoice.tsx` and its test

**Consumes:** Tasks 12-14.

**Produces:** for `kind: "ask"`, the question plus one control per option
showing its Japanese label. Choosing one re-posts the original question to
`/api/plan` with `answers: [{param, value}]` appended.

- [ ] **Step 1** Write the test: an ask result with four options renders all
      four labels; clicking one posts the original question plus that answer. Run
      it, expect failure.
- [ ] **Step 2** Implement.
- [ ] **Step 3** Web gates green.
- [ ] **Step 4** Commit: `feat(web): let the user resolve an ambiguous value`

**Satisfies:** AC-F-103.

---

### Task 16: end to end

**Files:**

- Create: `e2e/src/orchestration.test.ts`, `e2e/browser/chat.spec.ts`
- Modify: `e2e/playwright.config.ts` if more than one service must start

**Consumes:** everything.

**Produces:** a process-level test that starts both dummy services and the
platform (with the stub planner) serving the built web app, asks a question, and
asserts the rendered table.

The platform binary must serve `web/dist` for this to work — check
`ORCHESTRA_STATIC_DIR` is honoured, as the harness's browser gates rely on the
same mechanism.

- [ ] **Step 1** Write the test. Run it, expect failure.
- [ ] **Step 2** Make it pass, starting the processes from the built binaries.
- [ ] **Step 3** `make check` in full — every gate green, including
      `guard-a11y` and `guard-layout`, which now have a real screen to measure.
      Expect work here: contrast and target size are measured, not asserted.
- [ ] **Step 4** Commit: `test(e2e): answer a question against the built product`

**Satisfies:** AC-E-101.

---

## Order and parallelism

Tasks 1 and 2 are independent of everything else and of each other. Task 3 is
independent of 1-2. Tasks 4-9 are a chain. Tasks 10 and 11 depend on 5-6 but not
on each other beyond the shared transport, and neither blocks the web work.
Tasks 12-15 need the platform's generated types (Task 6) but not its behaviour —
they can start once `/api/plan` exists in the spec. Task 16 needs all of it.

## Done

`make check` is green and every criterion in `PRODUCT.md` section 5 has a
passing test named after it.
