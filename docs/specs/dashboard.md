# Panels a person builds

The sixth subproject, and the first half of what `docs/requirements.md`
FR-F calls a dashboard. `docs/specs/workspaces.md` built the place panels
live; this one lets a person put one there without asking a question first,
and draw it as a chart.

Arranging them - dragging, resizing, persisting a layout (FR-F-4) - is the
second half and is not here (section 9).

## 1. What it proves

That a workspace is something a person builds, not only somewhere the chat
deposits things.

Today a panel can only be born from an answer: ask a question, like the
answer, press save. That is the whole of W4, and it was the right first
move - it proved a result already carries everything a panel needs. But a
person who knows they want "件数をステータス別に、棒グラフで" has to phrase
it as a question and hope, and if the chart is not what the rendering rule
would have drawn, there is no way to say so.

It also proves the product's own premise, which the obvious version of this
slice would have quietly broken. The point of this platform is to be
pointed at services it does not own. The easy way to get a chart is to ask
each service for a `/summary` endpoint that returns `[{status, count}]` -
and that is an integration that begins by asking every team to change their
API. A dashboard that only works against services built for it has not
proved anything about services that were not.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                                                                                                                                                                                                                    |
| ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **P1** | A panel carries a view as well as a call. W1 said a panel is a saved call; it now also says how to draw it. The call says what to fetch, the view says what to show, and neither stores the answer (W2 holds).                                                                                                                                                                              |
| **P2** | The view's defaults come from the contract and the person overrides them. `x-ui-hint` gains `chart`; where a contract declares one, a chat answer draws as a chart with nothing configured. Where the person disagrees, the panel's own view wins.                                                                                                                                          |
| **P3** | A panel may carry exactly one transformation: group by a field, aggregate another. Not a pipeline. It runs in the browser, so the platform is still a thing that routes and renders and not a thing that computes.                                                                                                                                                                          |
| **P4** | Charts come from `@mui/x-charts` - bar, line and pie. It is MUI's own package, so `AGENTS.md` rule 6 is satisfied without arguing a new dependency past it.                                                                                                                                                                                                                                 |
| **P5** | A chart draws rows, never a scalar. Its input is the same object array a table renders: one field names the category, one holds the value. Statistics tiles are excluded (section 9).                                                                                                                                                                                                       |
| **P6** | A person builds a panel from their own catalogue, read from a new `GET /api/catalog`. `GET /api/operations` stays what it is - the admin's flat list for the permission grid, which is a different question.                                                                                                                                                                                |
| **P7** | Building a panel reuses the form that already draws a create. Arguments come from the same `inputSchemaFor` output an unsafe call's form is built from. A second form would be a second set of bugs.                                                                                                                                                                                        |
| **P8** | A manually built panel never reaches the planner. W4 said a person presses a button and the planner learns nothing about workspaces; here the person also picks the operation, so there is nothing left to decide.                                                                                                                                                                          |
| **P9** | `GET /api/catalog`'s `CatalogEntry` carries `displayName` alongside `summary`, from the contract's `x-ui-hint.displayName` (`docs/specs/orchestration.md` D15) falling back to `summary` when a contract declares none. `OperationPicker` and a panel's default title read `displayName`, never `summary` - `summary` is the model-facing tool description, not a person-facing name (D15). |

## 3. What a panel is now

```
Panel
  id            assigned by the platform
  workspace_id
  service       "inventory"
  operationId   "ListInventoryItems"
  args          {}
  component     "chart"
  title         "ステータス別の在庫件数"
  position      0
  view                                        ← new
    transform   { groupBy: "status", aggregate: "count" }
    chart       { category: "status", value: "count", kind: "bar" }
```

`view` is absent on every panel saved before this slice, and on every panel
whose component needs nothing configured: a table draws every column it has
and a detail draws every property, so neither has anything to say here.

`transform` and `chart` are independent. A table with a transform is a
perfectly good panel - "ステータス別の件数" as three rows rather than three
bars - and a chart over an endpoint that already returns aggregates needs no
transform at all.

## 4. The transformation

One step, with three fields:

```
transform
  groupBy     "status"          a field of the response's rows
  aggregate   "count"           count | sum | avg
  field       "quantity"        which field to sum or average; absent for count
```

Rows in, rows out: `[{id, name, status, quantity}, ...]` becomes
`[{status, count}, ...]`, and whatever drew the first can draw the second.
The output's field names are `groupBy`'s own name and the aggregate's
(`count`, `sum`, `avg`), so a chart's `category` and `value` name fields
that exist.

**Not a pipeline.** Grafana's transformations are a list of steps, and the
list is the feature: ordering, the intermediate shape after each step, the
controls to add and remove and reorder them. That is a subproject of its
own, and the thing it buys over one step is combinations nobody has asked
for yet. One step fits in the same form as the chart's axes - three
controls beside three more - and a second step can be argued for on its own
once something needs it.

**In the browser.** The rows are already going there to be drawn, the view
is already a browser concern, and the alternative is the platform learning
to compute over data it currently only forwards. `docs/specs/orchestration.md`
D8's reasoning is about the model, but the line it draws - the platform
routes and renders - is worth keeping for the same reason: a platform that
transforms is a platform with opinions about what the data means.

### 4a. What this cannot do, and where it will hurt

Grouping sees what the call returned. An operation that paginates returns a
page, and grouping a page produces the counts for that page, presented as
though they were the counts. The dummy services return everything they
have, so this never shows here - and it is the first thing that will go
wrong against a real service.

Nothing in this slice fixes that, and pretending otherwise by hiding the
transform would be worse: the honest shape is that a service's own
aggregate endpoint is always the better answer where one exists (P2's
`x-ui-hint.chart` is how such an endpoint says so), and the transform is
what a person has when it does not. `docs/requirements.md` FR-B's catalogue
has no notion of "this operation returns everything" to check against, so
this is a limit, not a validation.

## 5. The catalogue a person browses

```
GET /api/catalog -> [
  {
    service:     "inventory",
    operationId: "ListInventoryItems",
    summary:     "List stock items, optionally filtered by status.",
    displayName: "在庫一覧",  // x-ui-hint.displayName, falling back to summary (P9)
    component:   "table",
    schema:      { ... },        the arguments, as /api/plan's form `schema`
    fields:      { ... }         the response's fields, as /api/plan's `fields`
  },
  ...
]
```

Every operation the signed-in person may call, and nothing else: the same
`Catalog.For(permissions)` the planner's tool list is built from
(`docs/specs/auth.md` A4), so the operations a person can build a panel
from and the operations they can ask a question about are the same set by
construction.

`schema` and `fields` are shapes the contract already carries, for the same
two jobs: `schema` is what a form is built from, and this screen builds a
form; `fields` is what a table's columns are described by, and this screen
offers those fields as a chart's axes and a transform's `groupBy`.

`GET /api/operations` is left alone. It answers "what does this deployment
have", admin only, for the permission grid to draw itself; this answers
"what may I call, and what shape is it". Merging them would mean one
endpoint answering two questions with a role check in the middle.

## 6. Building one

On the workspace screen, beside the panels: a control that adds one.

1. Pick an operation from the catalogue, grouped by service.
2. Fill in its arguments - `ResultForm`, from `schema` (P7).
3. Pick a component. The rule's own answer (`component`) is selected
   already; `chart` is offered whenever `fields` describes rows.
4. For a chart, pick the category and the value from `fields`, and the
   kind. For a transform, pick `groupBy`, the aggregate and its field.
5. Name it, or accept the operation's summary as the name.

Steps 4 and 5 are one form, not a wizard: every control is on screen at
once, and the ones that do not apply are not there. A person who picked
`table` sees no axes.

The panel is created through the endpoint that already exists
(`POST /api/workspaces/{id}/panels`), which gains `view`.

## 7. Contract

```
GET  /api/catalog                        new (section 5)
POST /api/workspaces/{id}/panels         + view
GET  /api/workspaces/{id}                panels carry view
```

`Component` gains `chart`. `x-ui-hint` gains `chart`, alongside the
`component` it already has:

```yaml
x-ui-hint:
  component: chart
  chart: { category: status, value: count, kind: bar }
```

No service in this repository declares one yet, and none has to: the
dummies are unchanged by this slice, on purpose (section 1).

## 8. Where it sits

```
services/platform/
  internal/domain/rendering.go     ComponentChart
  internal/domain/workspace.go     Panel.View
  internal/adapter/specsource/     x-ui-hint.chart
  internal/adapter/handler/        GET /api/catalog
  internal/usecase/                the catalogue usecase, over Catalog.For
web/src/
  entities/rendering/ui/ResultChart.tsx    bar | line | pie, from @mui/x-charts
  entities/rendering/lib/transform.ts      groupBy + count/sum/avg, pure
  features/panels/                         the builder
  pages/workspace/                         hosts it
```

`transform.ts` is pure and lives in `entities/rendering`, not in the
builder: the workspace screen applies it when it draws a saved panel, and
the builder applies it to show a preview. Two callers, one function.

## 9. Deliberately excluded

- **Arranging panels** (FR-F-4). Dragging, resizing, a persisted layout -
  the second half of the dashboard, and the half that needs a grid library
  MUI does not have. That argument is worth having on its own, against a
  screen that already has something worth arranging.
- **A statistics tile.** One big number is a different component with a
  different input: a scalar, which no operation here returns. It would need
  either an endpoint that returns one or a transform that reduces rows to
  one, and the second is a second transform (section 4).
- **A transformation pipeline.** Section 4 argues it.
- **Editing a panel's view in place.** Rebuilding is the same form and the
  same five steps. This is the weakest of the exclusions here - picking one
  wrong axis costs a whole panel - and it is the first thing to add if it
  is annoying in practice.
- **Asking the chat to add a panel** (FR-F-5). The workspace screen has a
  conversation already, and it answers questions; making it manipulate the
  workspace is a planner change, which P8 is specifically not.
- **The chat drawing a chart it chose the axes for.** A chart a chat
  produces is drawn from `x-ui-hint.chart` or not at all. Letting the model
  pick axes is `docs/specs/orchestration.md` D3 in reverse - the rendering
  rule is deterministic, and this is a rendering decision.

## 10. Acceptance criteria

- **AC-P-101** `GET /api/catalog` lists exactly the operations the signed-in
  person may call, each with its arguments schema and its response fields;
  a person granted one service sees no operation of the other.
- **AC-P-102** A panel can be created from the workspace screen without a
  question being asked, and it draws the same result an equivalent saved
  answer would.
- **AC-P-103** A panel whose component is `chart` draws a bar, line or pie
  chart of its rows, with the category and value its view names.
- **AC-P-104** A panel whose view carries a transform draws the grouped and
  aggregated rows, and one without a transform draws the rows as they came.
  The same transform over the same rows produces the same output, tested as
  a pure function.
- **AC-P-105** An operation whose contract declares `x-ui-hint.chart` draws
  as a chart from the chat, with no view configured, and a panel saved from
  that answer carries the contract's axes as its own view.
- **AC-P-106** A panel saved before this slice - no `view` at all - still
  draws.
- **AC-P-107** A person may not build a panel over an operation they may
  not call, through this endpoint any more than through `/api/invoke`.

## 11. Harness work this implies

One new dependency, `@mui/x-charts`, which `harness/quality/ui-primitives.txt`
has nothing to say about: that file forbids raw `<button>`/`<input>`/`<select>`
elements, not libraries. `AGENTS.md` rule 6 is the rule that would bite, and
it asks for MUI - which this is.

`make guard-a11y` and `make guard-layout` run against a real screen, so a
chart has to pass both. A chart is a picture, and a picture with no text
alternative is a picture a screen reader cannot read: the panel's title is
the text, and the chart carries it.
