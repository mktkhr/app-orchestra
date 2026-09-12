# Panels a person builds — implementation plan

> **For the implementer:** one task at a time. Each task ends green and is
> committed on its own. Steps use `- [ ]` for tracking.

**Goal:** A person adds a panel to a workspace without asking a question,
and draws it as a chart over rows the browser grouped first.

**Architecture:** A panel gains a `view` — what to draw, beside the call
that says what to fetch. The browser reads its own catalogue from a new
`GET /api/catalog`, builds the call with the form that already draws a
create, and applies one optional grouping before rendering. The platform
gains an endpoint and a column; it does not learn to compute.

**Spec:** `docs/specs/dashboard.md`. Acceptance criteria: its section 10.

## Global constraints

Everything in `docs/plans/context.md`'s Global constraints section still
holds. The ones this subproject is most likely to meet:

- `make check` must never call a real LLM, and must need nothing running.
  Verify by counting
  `docker logs llama-swap 2>&1 | grep -c 'POST /v1/chat/completions'`
  before and after.
- Contract first: change `services/platform/api/openapi.yaml`, run
  `make generate`. Never edit generated files.
- **Every `enum` in every contract needs `x-enum-labels`**, a Japanese label
  per value — `harness/quality/redocly/enum-labels.js` fails the build
  otherwise, and this plan adds two enums (`aggregate`, `kind`).
- Layer order (`make guard-arch`): domain ← usecase ← adapter ← infra ←
  app ← cmd. `domain` imports stdlib only — no `encoding/json`. FSD for the
  web (`make guard-fsd`): app → pages → widgets → features → entities →
  shared. A feature may not import a sibling feature.
- 1000 lines a file; 90% statement coverage per Go package.
- Controls come from MUI. `@mui/x-charts` is MUI and needs no exception;
  `harness/quality/ui-primitives.txt` is about raw elements, not libraries.
  A disabled contained button has no edge `make guard-layout` can see.
  `TextField size="small"` is under the 44px floor.
- Run `make -k check` while working; `make build` before `make guard-browser`.
- Verify in a browser at the Tailscale address, not at `localhost`.
- **Run `make check` again after the last edit, before reporting.** Green in
  the middle is not evidence.

## The shape everything shares

`view` is one schema, used in three places, and the implementer should add
it once:

```yaml
View:
  type: object
  description: How to draw a result - beside the call that says what to fetch.
  properties:
    transform:
      type: object
      required: [groupBy, aggregate]
      properties:
        groupBy: { type: string }
        aggregate: { type: string, enum: [count, sum, avg] } # x-enum-labels!
        field: { type: string } # absent for count
    chart:
      type: object
      required: [category, value, kind]
      properties:
        category: { type: string }
        value: { type: string }
        kind: { type: string, enum: [bar, line, pie] } # x-enum-labels!
```

- On `Panel`, both halves may be present (section 3 of the spec).
- On `PlanResult` and on a catalogue entry, only `chart` is ever set, and
  only from the endpoint's `x-ui-hint.chart`: a contract declares axes, not
  transforms.

## File structure

```
services/platform/
  api/openapi.yaml                       + View, Component "chart",
                                           GET /api/catalog, Panel.view
  internal/domain/rendering.go           ComponentChart
  internal/domain/view.go                View, Transform, Chart. Pure.
  internal/domain/catalog.go             Endpoint.ChartHint
  internal/domain/workspace.go           Panel.View
  internal/adapter/specsource/http/      x-ui-hint.chart
  internal/adapter/repository/sqlite/    panels.view column
  internal/adapter/handler/catalog.go    GET /api/catalog
  internal/usecase/catalog.go            the usecase behind it
web/src/
  entities/rendering/lib/transform.ts    groupBy + count/sum/avg. Pure.
  entities/rendering/ui/ResultChart.tsx  bar | line | pie
  features/panels/                       the builder
  pages/workspace/ui/PanelResult.tsx     applies the view
e2e/
  src/dashboard.test.ts, browser/dashboard.spec.ts
```

---

### Task 0: rows can be grouped

**Files:**

- Create: `web/src/entities/rendering/lib/transform.ts` and its test

**Consumes:** nothing. This can run first and beside anything.

**Produces:**

```ts
export type Aggregate = "count" | "sum" | "avg";
export interface Transform {
  readonly groupBy: string;
  readonly aggregate: Aggregate;
  readonly field?: string;
}
export function applyTransform(
  rows: readonly Record<string, unknown>[],
  transform: Transform,
): Record<string, unknown>[];
```

Rows in, rows out. The output's keys are `transform.groupBy` and the
aggregate's own name (`count`, `sum`, `avg`), so a chart's `category` and
`value` can name fields that exist (spec section 4).

- [x] **Step 1** Write the tests first, and make them the specification:
      grouping by a field with three distinct values yields three rows;
      `count` counts; `sum` and `avg` read `field`; a row missing `groupBy`
      groups under its own bucket rather than being dropped silently; a
      non-numeric value under `field` does not produce `NaN`; an empty input
      yields an empty output. Decide what each edge does and write it down
      here — this function is pure, so its tests are the whole contract.
- [x] **Step 2** Implement. No date handling, no sorting beyond a stable
      order, no second step.
- [x] **Step 3** Web gates green.
- [x] **Step 4** Commit: `feat(web): group rows before drawing them`

**Satisfies:** AC-P-104's pure-function half.

---

### Task 1: a chart draws rows

**Files:**

- Modify: `web/package.json` (`@mui/x-charts`), the pnpm catalog
- Create: `web/src/entities/rendering/ui/ResultChart.tsx` and its test

**Consumes:** nothing. Beside Task 0.

**Produces:** a component taking rows, a `category`, a `value`, a `kind`
and a title, drawing one of `BarChart`/`LineChart`/`PieChart`.

The title is the chart's text alternative: a picture with none is a picture
a screen reader cannot read, and `make guard-a11y` runs against a real
screen (spec section 11).

- [x] **Step 1** Add `@mui/x-charts` at the version the catalog pins for the
      other MUI packages. `make check` must still pass `guard-ui`.
- [x] **Step 2** Write the test: three rows and a category/value draw three
      bars; the same rows draw as a line and as a pie when `kind` says so; a
      value that is not a number is skipped rather than drawn as `NaN`.
- [x] **Step 3** Implement.
- [x] **Step 4** Web gates green, then `make build` and
      `make guard-browser`, `make guard-a11y`, `make guard-layout` — a chart
      is the first thing in this repository that is a picture.
- [x] **Step 5** Commit: `feat(web): draw a chart of a result's rows`

**Satisfies:** AC-P-103's rendering half.

---

### Task 2: a panel carries a view

**Files:**

- Modify: `services/platform/api/openapi.yaml`,
  `internal/domain/view.go` (new), `internal/domain/rendering.go`,
  `internal/domain/workspace.go`,
  `internal/adapter/repository/sqlite/`, `internal/usecase/workspaces.go`,
  tests

**Consumes:** nothing on the web side.

**Produces:** `View` on the contract, `Panel.View`, `chart` as a
`Component`, and a `view` column the sqlite adapter marshals like `Args`
already is — JSON lives inside that adapter and may not reach `domain` or
`usecase` (depguard).

A panel saved before this slice has no `view` and must still read back and
still draw (AC-P-106): the column is nullable and `View` is a pointer.

- [x] **Step 1** Add `View` and `Component: chart` to the contract, and
      `view` to `Panel` and `CreatePanelRequest`. Remember `x-enum-labels`
      on both new enums. `make api-lint`, `make generate`.
- [x] **Step 2** Write the store test: a panel with a view round-trips; a
      panel without one round-trips as nil; an existing row with a NULL
      `view` reads back as nil rather than erroring. Run it, expect failure.
- [x] **Step 3** Implement, including the migration — the schema is
      embedded and applied at open with `IF NOT EXISTS`, so adding a column
      to an existing file needs an `ALTER TABLE` guarded the same way.
      **Open a database file created before this change and read it**, in a
      test; a migration nobody ran against old data is a migration nobody
      tested.
- [x] **Step 4** Add the test that `AddPanel` refuses an operation the
      person may not call, with the same error an unknown one gets
      (AC-P-107) — `Workspaces` already takes the user.
- [x] **Step 5** Go gates green.
- [x] **Step 6** Commit: `feat(platform): let a panel say how to draw itself`

**Satisfies:** AC-P-106, AC-P-107.

---

### Task 3: a contract can declare its axes

**Files:**

- Modify: `internal/adapter/specsource/http/parse.go`,
  `internal/domain/catalog.go`, `internal/usecase/orchestrator.go`,
  `services/platform/api/openapi.yaml`, tests

**Consumes:** Task 2 (the `View` schema).

**Produces:** `x-ui-hint.chart` parsed into `domain.Endpoint`, and a
`PlanResult` that carries `view.chart` when the endpoint declares one — so
an answer the chat produced draws as a chart with nothing configured, and
saving it copies the axes into the panel's own view (AC-P-105).

`x-ui-hint.component` already parses (`parse.go`'s `uiHint`); this is the
sibling field beside it. No service in this repository declares one — use
`internal/adapter/specsource/http/testdata/fixture.yaml`, which is where
`x-ui-hint` is exercised today.

- [x] **Step 1** Add `view` to `PlanResult` on the contract.
      `make api-lint`, `make generate`.
- [x] **Step 2** Write the parse test against the fixture: an operation
      declaring `x-ui-hint.chart` yields an `Endpoint` carrying it; one
      declaring only `component` still parses; a malformed one is an error,
      not a silent empty. Run it, expect failure.
- [x] **Step 3** Implement the parse, then thread it onto the result in
      `Orchestrator`'s result path.
- [x] **Step 4** Go gates green.
- [x] **Step 5** Commit: `feat(platform): let a contract name a chart's axes`

**Satisfies:** AC-P-105's platform half.

---

### Task 4: a person can read their own catalogue

**Files:**

- Modify: `services/platform/api/openapi.yaml`
- Create: `internal/usecase/catalog.go`,
  `internal/adapter/handler/catalog.go`, tests,
  `services/platform/acceptance/catalog_test.go`

**Consumes:** Tasks 2-3.

**Produces:** `GET /api/catalog` — every operation the signed-in person may
call, each with `service`, `operationId`, `summary`, `component`, `schema`
(arguments, from `inputSchemaFor`), `fields` (response fields, from
`FieldsSchema`) and `view` when the contract declared axes.

Built over the same `catalogFor(ctx, user)` the planner's tool list is
(`docs/specs/auth.md` A4), so the two sets are the same by construction and
not by agreement.

`GET /api/operations` is not touched. It answers a different question for a
different audience (spec section 5).

- [ ] **Step 1** Add the path and its schemas. `make api-lint`,
      `make generate`.
- [ ] **Step 2** Write the acceptance test: a person granted one service
      sees only its operations and none of the other's; an admin sees every
      one; no session is 401 (AC-P-101). Run it, expect failure.
- [ ] **Step 3** Implement.
- [ ] **Step 4** Go gates green.
- [ ] **Step 5** Commit: `feat(platform): tell a person what they may call`

**Satisfies:** AC-P-101.

---

### Task 5: a workspace draws a panel's view

**Files:**

- Modify: `web/src/pages/workspace/ui/PanelResult.tsx`,
  `web/src/entities/workspace/`, `web/src/shared/api/`, tests

**Consumes:** Tasks 0, 1, 2.

**Produces:** a saved panel whose view names a transform gets grouped rows,
and one whose view names a chart gets a chart. A panel with no view draws
exactly as it does today.

`ResultChart` currently draws at a fixed 320x240, because `@mui/x-charts`
measures its container with `getComputedStyle` and a test environment with
no layout engine returns 0 (Task 1's own comment says so). That is the test
environment deciding the product's layout, which is backwards, and a panel
is where it stops being tolerable: a 320px chart in a wide card looks like
a mistake. Give `ResultChart` the size its caller wants - optional props,
defaulting to what it has now so Task 1's tests keep working - and have the
panel pass what the panel knows. Check it at 375px too; `make guard-layout`
will.

- [ ] **Step 1** Write the test: a panel with `view.chart` renders the chart
      component; one with `view.transform` renders the grouped rows; one
      with neither renders the table it renders today (AC-P-106). Run it,
      expect failure.
- [ ] **Step 2** Implement. The transform is applied where the rows arrive,
      before the component is chosen — `transform.ts` has two callers, this
      and Task 6's preview, which is why it lives in `entities/rendering`
      and not in the builder.
- [ ] **Step 3** Web gates green, then `make build` and
      `make guard-browser`.
- [ ] **Step 4** Commit: `feat(web): draw a panel the way it says to`

**Satisfies:** AC-P-103, AC-P-104 end to end.

---

### Task 6: building one

**Files:**

- Create: `web/src/features/panels/`
- Modify: `web/src/pages/workspace/ui/WorkspacePage.tsx`

**Consumes:** Tasks 0, 1, 4, 5.

**Produces:** a control on the workspace screen that adds a panel: pick an
operation (grouped by service), fill its arguments, pick a component, and —
for a chart — pick the category, the value and the kind, and optionally a
transform.

One form, not a wizard (spec section 6): every control on screen at once,
and the ones that do not apply are absent. Arguments reuse
`entities/rendering`'s `ResultForm` rather than a second form (P7) — check
what it needs to be reusable here and change it there, once, rather than
copying it.

The chart's axes default from the catalogue entry's `view` when it has one
(P2), and the person may change them.

- [ ] **Step 1** Write the test: the control lists only the operations the
      catalogue returned; picking one shows its arguments; picking `chart`
      shows the axes and offers only fields `fields` describes; saving posts
      the panel with its view (AC-P-102). Run it, expect failure.
- [ ] **Step 2** Implement.
- [ ] **Step 3** Web gates green, then `make build`, `make guard-browser`,
      `make guard-a11y`, `make guard-layout`.
- [ ] **Step 4** Commit: `feat(web): build a panel without asking a question`

**Satisfies:** AC-P-102.

---

### Task 7: end to end

**Files:**

- Create: `e2e/src/dashboard.test.ts`, `e2e/browser/dashboard.spec.ts`
- Modify: `STATE.md`, `TODO.md`, `DECISIONS.md`

**Consumes:** everything.

**Produces:** the journey against the built product — sign in, open a
workspace, add a panel over a list operation with a group-by and a bar
chart, see it draw, reload, see it draw again from the saved view.

- [ ] **Step 1** Write the process-level journey: `GET /api/catalog`,
      `POST` a panel carrying a view, read the workspace back, confirm the
      view survived. The stub planner is not involved — nothing here asks a
      question, which is P8's whole point.
- [ ] **Step 2** Write the browser journey.
- [ ] **Step 3** `make check` in full — every gate green, and no request
      added to the model's log.
- [ ] **Step 4** Commit: `test(e2e): build a panel and draw it`

**Satisfies:** the whole of `docs/specs/dashboard.md` section 10 end to end.

---

## Order and parallelism

Tasks 0 and 1 are the web's own and touch no contract; they can run first,
together, before anything else exists. Task 2 is the platform's and can run
beside them. Task 3 needs Task 2's `View` schema. Task 4 needs 2 and 3.
Task 5 needs 0, 1 and 2. Task 6 needs 4 and 5. Task 7 needs all of it.

## Done

`make check` is green, every criterion in `docs/specs/dashboard.md` section
10 has a test that runs in CI, and a person can put a chart on a workspace
without asking a question — over a service that was not changed to make it
possible.
