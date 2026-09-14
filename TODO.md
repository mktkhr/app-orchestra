# TODO.md — prioritised work

_Keep three lists. Move items, do not duplicate them._

## In progress

1. `docs/plans/narrowing.md` Task 3 - the lexical baseline
   (`e2e/narrowing/lexical.ts`): a character-bigram index over the catalogue,
   built once, no tokeniser/model/network. Tasks 1 and 2 (the fixture and
   the server that proved the platform builds a 1000-operation catalogue
   from it) are done; see `STATE.md`.

## Next

1. A genre/domain layer above individual services - grouping services by
   what they are for, rather than listing every one flat. Deferred again by
   `docs/specs/picking.md` K4 (2026-09-13): with today's contracts every
   exposed operation in `services/inventory` carries the single tag `items`
   and every one in `services/attendance` carries `records`, so this would
   group nothing beyond what `OperationPicker`'s own `groupBy` (off
   `serviceDisplayName`) already does. Worth building once a service
   carries more than one tag over its own exposed operations.
2. Move to TypeScript 7 once `openapi-typescript` supports it. Everything
   else in the repository already passes under 7; only code generation does
   not. orval was measured as a replacement and rejected - it runs under
   TypeScript 7 but emits the wrong shape for this product (`DECISIONS.md`,
   2026-09-11).
3. **Open defect: a question whose filter word matches no enum value gets
   every row back, silently.** On `qwen3.5-9b-q8`, `no-enum-value` (破損した
   在庫はある？) reaches this outcome 16-20 of 30 runs, the attendance
   variant (有給の勤怠はある？) 5-9 of 10 - both measured three times across
   D15's attempt (see below) and none of it closed the gap. D15
   (`docs/specs/orchestration.md`, an optional enum parameter on a safe
   endpoint offered to the model as required, with a synthetic `__all__`
   value) is tried and withdrawn (`DECISIONS.md`, 2026-09-12, final entry):
   the reject rate never moved outside its noise band in three
   measurements; only the accept rate moved, because `__all__` competes
   with `ask_user` as an easier tool to reach for, not because it closes
   the silent-omission path. A future attempt has to change that
   competition - through the operation's own tool description,
   `ask_user`'s own description, or the decision procedure itself - not add
   another value to the enum.
4. **`harness/quality/file-length.txt` needs `**/src/shared/api/gen/**` (or
   equivalent) added to its `exclude` list**, matching
   `harness/quality/oxfmt/policy.ts` and `harness/quality/oxlint/policy.ts`,
   which already carry it. `docs/plans/dashboard.md` Task 4's contract
   growth pushed `web/src/shared/api/gen/platform.d.ts` from 971 to 1030
   lines, over `guard-filelen`'s 1000-line limit, on a file that is
   entirely generated and never hand-edited - see `STATE.md`'s "Known gaps
   in the harness" for the full account. Left unfixed here per `AGENTS.md`
   rule 2 (harness/quality is not this agent's to reconfigure); the next
   contract change that touches `platform.d.ts` will hit the same wall
   until somebody with standing to edit the harness does.
5. **Uninstall `ollama`.** Left over from before `llama-swap` became the
   local model runtime `make eval`/`ORCHESTRA_LLM_BASE_URL` talk to; nothing
   in this repository or its harness names it any more (`grep -r ollama`
   across the tree turns up nothing but this line). Housekeeping on the
   development machine, not a code change.

6. **`<Typography color="text.secondary">` is a silent no-op almost
   everywhere it is written** - the component's own `color` prop only
   recognises `"textSecondary"` (camelCase, no dot) or a bare palette key
   (`Typography.d.ts`: `` `text${Capitalize<keyof TypeText>}` ``); the
   dot-path form is valid only inside `sx`. `OperationLabel.tsx`,
   `ResultDetail.tsx`, `ResultTable.tsx`, `ResultChart.tsx`,
   `PermissionGrid.tsx`, `WorkspacePicker.tsx`, `TurnList.tsx`,
   `SavedNotice.tsx`, `ExampleQuestions.tsx` and `Provenance.tsx` all pass
   the dot form today, so every one of those "secondary" lines actually
   renders at full `text.primary` opacity (`rgba(0, 0, 0, 0.87)` on light,
   confirmed live) rather than the dimmer `text.secondary` the code reads
   as asking for. Found while building `OperationPicker.tsx`'s new summary
   line (`docs/specs/picking.md` K2, fixed there with `color="textSecondary"` -
   see `DECISIONS.md`, 2026-09-13) and not fixed in the other ten files:
   none of them fail a check (the mistake reads darker, not lower-contrast,
   so `make guard-a11y` sees nothing wrong), and touching ten files outside
   this task's own scope for one commit was the wrong trade. TypeScript
   does not catch it either - the prop's type falls back to `(string & {})`
   for exactly this reason.

## Done

- **`docs/specs/offering.md` closes: a built-in tool declares its own
  condition, and `propose_panel` applies only when the question was asked
  from a workspace** (AC-O-101 through AC-O-105).
  `usecase.BuiltinTool{Tool, Applies func(PlanContext) bool}` and
  `PlanContext{WorkspaceID string}`
  (`internal/usecase/tools.go`); `ToolsFor(catalog, planCtx)` now takes the
  context and skips a builtin whose condition fails - `ask_user`/
  `list_capabilities` carry no condition at all. `PlanRequest.workspaceId`
  (optional, `openapi.yaml`) threads through the handler and
  `Orchestrator.Plan`'s new parameter into `PlanContext`; the browser's
  `ConversationPanel.tsx` already knew this (`defaultWorkspaceId`) and now
  forwards it through `Conversation`/`conversationStore` into `postPlan`.
  `Orchestrator.Plan` refuses a `DecisionProposal` with `ErrToolNotOffered`
  when propose_panel was not in the tools list it itself built, the way an
  unknown operation is refused; `jsonmode.Planner` additionally never tells
  the model about propose_panel at all (two precomputed prompt/schema
  variants) when it does not apply, since that transport has no declared
  function list to lean on the way `toolcall.Planner` does. See `STATE.md`
  and `DECISIONS.md`, 2026-09-14.
- **`docs/specs/conversation-ui.md` closes: a question is a bubble on the
  right, a result keeps the width it always had, a sentence answer is a
  bubble on the left, and a spinner (not a progress claim) holds the
  answer's place while a question is in flight** (AC-C-101 through
  AC-C-106). `web/src/features/conversation/ui/TurnList.tsx` bounds the
  question `Paper` to 75% width and right-aligns it; bounds `kind: "none"`
  and the contract-fallback branch the same way, left-aligned; leaves
  `proposal`/`form`/`ask` and every `kind: "result"` branch (table, detail,
  chart) full width, unchanged. `Conversation.tsx` now forwards its
  existing `pending` into `TurnList`, which draws one more bounded bubble -
  a `CircularProgress aria-label="回答を生成中"` and one honest line, no
  percentage - only while a question is in flight; `pending` already goes
  false on both success and failure, so the spinner's removal needed no new
  wiring. Verified live against `qwen3.5-9b-q8` (`make dev-services`,
  Playwright): the spinner is attached immediately, gone once the answer
  replaces it. Contrast measured by hand in both colour schemes (~15-17:1,
  reusing existing theme tokens); `make guard-a11y`/`make guard-layout`
  stayed green. See `STATE.md` and `DECISIONS.md`, 2026-09-14.
- **`docs/specs/storage.md` closes: one `*sql.DB` per database file, WAL, a
  busy timeout, and a 500 that reaches the log** (AC-S-101 through
  AC-S-104). `pkg/app.build` now opens `ORCHESTRA_DB_PATH` once
  (`sqlitestore.Open`) and hands that one connection to `Store`, `Users`,
  `Sessions` and `Permissions` via a `NewFromDB` constructor apiece,
  instead of each opening its own; the DSN also carries
  `_journal_mode=WAL` and `_busy_timeout=5000`. A new
  `internal/infra/httpserver/logging.go` middleware logs any 500 with the
  error that caused it. Forty concurrent requests that each resolve a
  session and write a row failed 8-19 of 40 times before this and ten of
  ten after (`services/platform/acceptance/storage_test.go`);
  `make guard-layout` passed ten consecutive runs afterward. See
  `STATE.md` and `DECISIONS.md`, 2026-09-14 (a correction to 2026-09-13's
  "the browser suite's flake is load, measured", not a deletion of it).
- **`docs/plans/proposing.md` closes: FR-F-5, asking the chat to add a
  panel** (`docs/specs/proposing.md` section 7, AC-N-101 through AC-N-106
  all covered end to end). Task 2's journeys (`e2e/src/proposing.test.ts`,
  `e2e/browser/proposing.spec.ts`) and its measurement of what
  `propose_panel` in every request's tool list costs every other question
  (section 9) are in - see `STATE.md` and `DECISIONS.md`, 2026-09-14.
- **`harness/guard/suppressions.sh` now scans for `istanbul ignore`,
  `c8 ignore` and `v8 ignore`**, alongside the directives it already
  caught - closes the gap this list used to record. See `DECISIONS.md`,
  2026-09-14, "the suppression guard now sees coverage directives".
- **`docs/specs/picking.md`: the panel picker searches more than its own
  label, an option states its summary, and the transform stays a switch
  alone until it is on** (AC-K-101 through AC-K-104). `OperationPicker.tsx`
  gained `filterOptions` (MUI's own `createFilterOptions({ stringify })`
  over a display name, service display name, operation id, summary and the
  titles of the fields the operation returns) and a two-line
  `renderOption`. K3 (the transform starting collapsed) and K4 (the genre
  layer, still deferred) needed no code change - see `STATE.md` and
  `DECISIONS.md`, 2026-09-13, for what was already true before this task
  touched it. Verified live: typing 数量 into a real workspace's picker
  found `ListInventoryItems`, `CreateInventoryItem` and
  `GetInventoryItem` - every operation whose response carries a field
  titled that, none of which have it in their own name.
- **`docs/specs/layout.md` section 5a: a panel's height on the narrow
  breakpoint is a second number, `narrowHeight`, editable by keyboard**
  (AC-L-107, AC-L-108) - closes the gap item 7 above used to record.
  `Panel`/`CreatePanelRequest`/`UpdatePanelRequest` gained `narrowHeight`
  (`openapi.yaml`); `ensurePanelsSizeColumns` (`migrate.go`) adds the third
  column the same idempotent way as `width`/`height`;
  `usecase.Workspaces` clamps only a value the caller actually sent, same
  as `height`; `buildPanelLayout` reads `narrowHeight` only on the narrow
  breakpoint, falling back to `height` when it is absent or non-finite
  (AC-L-108); `useArrangement` gained `narrowResizeBy`, PATCHing only
  `{ narrowHeight }`. Order (`position`) stays arrangeable by keyboard on
  the narrow breakpoint too - a one-column stack still has an order worth
  moving a panel within - only pointer dragging, pointer resizing, and
  changing `width` stay excluded there (`docs/specs/layout.md` section 5,
  L7). Verified live at 375px and 1280px: setting `narrowHeight` on a
  phone left `height` exactly as it was on desktop. See `STATE.md` and
  `DECISIONS.md`, 2026-09-13.

- **`docs/plans/routing.md` Task 0: the platform serves `index.html` for
  any address it does not otherwise answer** (`docs/specs/routing.md`
  section 4, AC-R-102/AC-R-103). `internal/infra/httpserver/router.go`'s
  `spaHandler` replaces the bare `http.FileServer(http.Dir(staticDir))`:
  a request that resolves to a real file under `staticDir` (including
  `/` itself) is served as itself; a request whose path has no file
  extension gets `index.html`, so a reload of a deep link like
  `/workspaces/abc` will keep working once Task 1 puts it in the URL; a
  request with an extension but no matching file still answers 404, not
  HTML. `/api/...` is untouched, `staticDir` empty behaves exactly as
  before, and traversal (plain and percent-encoded) cannot escape
  `staticDir` - see `DECISIONS.md`, 2026-09-13. Frontend untouched; the
  hash router still works.

- **`docs/plans/routing.md` Task 1: the browser reads the path**
  (`docs/specs/routing.md` section 3, AC-R-101 in the browser).
  `react-router` (`8.3.1`, pinned exactly - `DECISIONS.md`, 2026-09-12 and
  2026-09-13) replaces `useHashRoute` (deleted, with its test):
  `BrowserRouter` in `App.tsx`, `Routes`/`Route` in `MainContent.tsx` for
  `/`, `/workspaces/:workspaceId` and `/users`, anything else the chat. The
  drawer's rows are `react-router` `Link`s now, not anchors with
  `href="#..."`, so following one changes the screen without a page load.
  Task 2's first two steps (moving `harness/quality/browser/screens.ts` and
  fixing the one `e2e/browser` assertion that read a hash) landed here
  instead of behind a temporary redirect - see `DECISIONS.md`, 2026-09-13,
  "the routing shim that was not shipped".

- **`docs/plans/routing.md` Task 2, and the subproject closes**
  (`docs/specs/routing.md` section 7, all four criteria). `e2e/src/routing.test.ts`
  proves AC-R-102/AC-R-103 over real TCP against the built platform binary
  serving real `web/dist`: an unknown path answers the built index, a real
  asset is itself, a missing asset 404s and is not HTML, and
  `/api/does-not-exist` still answers as the API. `e2e/browser/routing.spec.ts`
  proves AC-R-101 end to end - open a workspace by clicking, reload, still
  there, same for `/users`. AC-R-104 was checked before a test was written
  for it rather than assumed (`DECISIONS.md`, 2026-09-13): it already held,
  for free, because `BrowserRouter` sits above `AuthGate` and
  `SessionProvider.signIn` never navigates, so the address a person typed
  in survives the sign-in screen being swapped for the shell underneath
  it - `routing.spec.ts`'s second test drives that exact journey. Full
  `make check` green, `docker logs llama-swap` unchanged across it.

- **Defect fix: a panel over an unsafe operation no longer calls
  `/api/invoke` on its own** (`docs/specs/dashboard.md` P14, section 6b,
  AC-P-111). `usePanelInvoke` (`entities/workspace`) takes a new `enabled`
  argument gating both its mount effect and its `refresh`; `PanelResult`
  computes it from a new `useCatalogEntry` lookup (`GET /api/catalog`,
  matched by `service`+`operationId`) rather than from the panel's own
  `component` - see `DECISIONS.md`, 2026-09-13 for why the catalogue entry
  is the only source that cannot drift. An unsafe panel draws
  `entities/rendering`'s `ResultForm` instead (`PanelQuickAddBody`), seeded
  from the panel's saved arguments; submitting is what calls `/api/invoke`.
  Measured live, before and after one submission: `services/inventory`'s
  in-memory store held 8 rows before, 8 after opening the workspace twice
  and pressing refresh, 9 after the one deliberate submit.
- **Defect fix: a panel now has exactly one scroller**
  (`docs/specs/dashboard.md` P15, AC-P-112). `PanelCardShell`'s
  `CardContent` no longer scrolls itself (`overflow: "hidden"`, a flex
  column); `ResultTable`/`ResultTableGrid` give the table's own
  `TableContainer` the scrolling box instead (`flexGrow: 1`, `overflow:
"auto"`, `tabIndex={0}` for `make guard-a11y`'s
  `scrollable-region-focusable`), with the "拡大表示" button and the
  pagination pinned outside it (`flexShrink: 0`) so pagination stays
  reachable. Verified live: a table panel too small for its rows scrolls
  only inside `TableContainer` (`scrollHeight` 334 vs `clientHeight` 108),
  while `CardContent` does not (290/290).
- `docs/plans/layout.md` Task 0: a panel carries `width` (grid columns,
  1-12) and `height` (grid rows), and `position` is now `PATCH`-writable.
  Defaults (full width, one row) live once in `domain` (`DefaultPanelWidth`/
  `DefaultPanelHeight`), read by both `sqlite.Store` (a `NULL` column - every
  panel saved before this slice - reads back at the default, AC-L-104) and
  `usecase.Workspaces.AddPanel` (a zero `Width`/`Height`, meaning "the
  caller said nothing", gets the same default before it ever reaches the
  store). Clamping (`domain.ClampPanelWidth`/`ClampPanelHeight`) runs in the
  usecase, on both `AddPanel` and a `PATCH`-named `Width`/`Height`, never in
  the browser: 40 clamps down to 12, 0 and -1 clamp up to 1 (height has no
  upper bound - section 5). `position`, `width` and `height` are plain
  `*int` fields on `PanelPatch` - absent means unchanged, the same as
  `Title`/`Args`/`Component` - since an integer has no "explicitly clear it"
  request distinct from "leave it alone" the way naming `view` `null` does;
  `UpdatePanel`'s existing "only the named columns" `SET` builder already
  keeps a position-only `PATCH` from touching any other panel (AC unnamed
  in `docs/specs/layout.md` section 6, tested directly:
  `TestStoreUpdatePanelPositionDoesNotRenumberOthers`). The migration
  (`ensurePanelsSizeColumns`) extends `migrate.go`'s existing
  `pragma_table_info` pattern for `panels.width`/`panels.height`, proven
  against a database file with the schema exactly as it existed after
  `view` but before this slice
  (`TestNewOpensADatabaseFileWrittenBeforeSizeColumnsExisted` - the real
  shape of `~/.local/state/app-orchestra/workspaces.db`). Frontend
  fixtures across `web/src/features/**` and `web/src/pages/workspace/**`
  gained `width`/`height` to satisfy the now-required wire fields; nothing
  draws differently yet (Tasks 1-2).

- `docs/plans/layout.md` Task 1: the workspace draws as a
  `react-grid-layout` grid, read-only. `react-grid-layout` `1.5.4` +
  `@types/react-grid-layout` `1.3.6`, both pinned exactly. New
  `pages/workspace/ui/WorkspaceGrid.tsx` (one `useMediaQuery` picks 12 or 1
  columns, MUI's own `sm`) and `pages/workspace/model/buildPanelLayout.ts`
  (pure: sorts by `position`, packs left to right, clamps every width to
  `columns` - `columns: 1` alone gives the narrow breakpoint AC-L-105 with
  no separate branch). Every grid item is `static`, and
  `isDraggable`/`isResizable` are both `false` - Task 2's job. Also
  landed, both forced by AC-L-106: `PanelCardShell.tsx` fills its grid
  item's box (`height: "100%"`, a scrolling `CardContent`) instead of
  sizing to content, and `PanelResult.tsx`'s chart size now comes from a
  new `shared/lib/useElementSize.ts` (`ResizeObserver`, callback-ref
  backed) measuring the chart's own rendered box, replacing a viewport
  media query that could not tell a wide panel from a narrow one on the
  same screen. Confirmed by hand against a running platform: two chart
  panels, `width: 12` and `width: 6`, measured 896×626 and 416×626 and
  drew `<svg>`s at exactly those sizes - see `STATE.md` for the bug this
  caught (a plain `useRef` effect that ran before the measured element
  ever mounted) and the fix.

- `docs/plans/layout.md` Task 2: dragging and resizing by pointer, on the
  wide breakpoint, plus a keyboard path to both (AC-L-103) -
  `pages/workspace/model/arrangement.ts`'s four pure functions
  (`positionChanges`/`sizeChange` for the pointer half, `moveChanges`/
  `resizeChange` for the keyboard one) `PATCH` only the panels whose
  geometry actually changed, never a renumbering of the workspace (section
  6). The keyboard control lives in every panel's own header
  (`PanelActions.tsx`): arrow keys move, `Shift`+arrow resizes, tested by
  keyboard alone. The harness change this owed:
  `harness/quality/browser/playwright.config.ts` now runs the inventory
  dummy service too, so `screens.ts`'s "a workspace" can seed one real
  panel - `AddPanel` refuses any operation outside the catalogue, so there
  was no way to seed one without a real service. That seeding found three
  real bugs (a sideways-scroll flash from `react-grid-layout`'s own
  `WidthProvider`, `isDraggable` eating every panel button's click, and an
  overlay keyboard control sitting on top of the header it now lives in
  instead) - all fixed, all argued in `DECISIONS.md`. The stylesheet's own
  drag placeholder and resize handle, live for the first time, changed
  nothing about `guard-layout`'s contrast checks in either colour scheme.

- `docs/plans/layout.md` Task 3: end to end - the subproject closes.
  `e2e/src/layout.test.ts` (process-level: three panels, three `PATCH`es
  give one of them a wider, taller, first row and the other two a later
  `position`, a fresh `GET` on the same process proves it, AC-L-101/102)
  and `e2e/browser/layout.spec.ts` (real pointer drags in headless
  Chromium: resize two panels narrower by their own handle, resize and
  drag a third to the front, reload, then narrow to 375px and confirm the
  order survives with no panel wider than the viewport, AC-L-105) - the
  latter's geometry helpers live in `e2e/browser/helpers/layout.ts` for
  `max-lines`. AC-L-103's own two claims were confirmed rather than
  retested: the keyboard-only test already in
  `WorkspaceGrid.test.tsx` (Task 2) covers the first, and running
  `make guard-a11y`/`make guard-layout` directly against
  `screens.ts`'s "a workspace with a panel in it" screen (also Task 2)
  covers the second. Found two things along the way, neither a product
  bug: `AddPanel` never assigns a fresh panel an ascending `position` (it
  is always the domain's zero value, matching `docs/specs/workspaces.md`
  W5's original column - a workspace looks arranged only once something
  actually arranges it), and `.react-grid-item.cssTransforms`'s 200ms
  transition means a bounding box read immediately after a resize that
  moved another panel can read a mid-slide position - both argued in
  `DECISIONS.md`.

- `docs/specs/dashboard.md` section 6a, P11-P13: a panel can be changed
  after it is made. `PATCH /api/workspaces/{id}/panels/{panelId}`
  (`UpdatePanelRequest` - `title`/`args`/`component`/`view` all optional,
  `service`/`operationId` not properties at all), `usecase.Workspaces.UpdatePanel`
  (narrows through the same `catalogFor` `AddPanel` uses, re-checked against
  the panel's own fixed operation rather than trusted from save time),
  `sqlite.Store.UpdatePanel` (writes only the named columns), and the
  builder's own form reused for editing (`usePanelFields` gained a `seed`
  parameter; `usePanelEditor`/`EditPanelControl`, beside the refresh control
  in a new `PanelActions.tsx`). "Remove the view" vs "leave it alone" is
  `nullable.Nullable[View]` on the wire, `**domain.View` (pointer-to-pointer)
  in `domain.PanelPatch` - domain may not depend on the adapter's own
  nullable type. Found and fixed a real gap along the way:
  `PanelArguments.tsx` never accepted seeded values at all, so an edited
  panel's own arguments would have come back empty - only the browser
  journey caught it. See `DECISIONS.md`, 2026-09-13 ("A panel can be
  changed after it is made"). `e2e/browser/dashboard.spec.ts` gained the
  edit journey; `e2e/src/dashboard-update-permissions.test.ts` is AC-P-109's
  own process-level test. `make check` is fully green;
  `docker logs llama-swap`'s request count did not move.
- Follow-up fix, one level up: a service (`inventory`/`attendance`) had no
  Japanese name either, shown raw in `OperationPicker`'s group headers,
  `list_capabilities`' サービス column, a result's provenance
  (`Provenance.tsx`) and the admin's permission grid (`ServiceCard.tsx`).
  `x-ui-hint.displayName` (D15) now also sits on a service's own `info`
  object (`docs/specs/orchestration.md` D16); `service` itself - the
  identifier `ORCHESTRA_SERVICES`, `source.service`, a `Permission` and a
  `Panel` all carry - never changes. See `STATE.md` and `DECISIONS.md`,
  2026-09-13. `make check` is green; `make eval` was not run and no
  operation's `summary` changed.
- Follow-up fix: English showing through the panel builder. A bug
  (`fieldOptionsFor` in `usePanelFields.ts` threw away the `title` `fields`
  already carries for the chart/transform pickers) and a pre-existing
  design gap (no operation had a Japanese name; `summary` is the
  model-facing tool description and stays untranslated on purpose - see
  `usecase.ToolsFor`). `x-ui-hint` gained `displayName`
  (`docs/specs/orchestration.md` D15), read through
  `domain.Endpoint.DisplayNameOr`; both dummy services' exposed operations
  now declare one. See `STATE.md` and `DECISIONS.md`, 2026-09-13. `make
check` is green; `make eval` was not run and no operation's `summary`
  changed.
- `docs/plans/dashboard.md`, Task 7: end to end - closes the whole
  dashboard subproject (all seven tasks). Checked every AC-P-101..107
  against what already runs in CI first, per the task's own instruction,
  and found two real gaps closed here rather than worked around: a
  chart-hinted chat answer never drew at all (`TurnList.tsx` had no
  `component === "chart"` branch, and `SaveToWorkspaceControl` dropped a
  result's own `view`), and a chart built on top of a transform could not
  be built through the panel builder (`usePanelFields.ts`'s axis pickers
  never offered the transform's own output keys) - plus a third found
  chasing the second against the real inventory binary, an untouched
  optional argument posted as `""` and rejected on every refresh
  (`usePanelBuilder.ts`'s new `compactArgs`). See `STATE.md` and
  `DECISIONS.md`, 2026-09-13 ("Dashboard Task 7"). `e2e/src/dashboard.test.ts` /
  `dashboard-permissions.test.ts` and `e2e/browser/dashboard.spec.ts` are
  the new journeys - the stub planner is never wired in for either, since
  P8 means nothing either one does ever asks a question. `make check` is
  fully green and `docker logs llama-swap`'s request count did not move.
- Fixed: an `ask_user` naming an **unsafe** operation (e.g. `CreateInventoryItem`)
  whose parameter happens to be a real enum (`status`) used to reach the
  wire as `kind: "ask"` - 「ステータスを選んでください」 - instead of the
  create form, because `optionsForParam` searched an unsafe endpoint's
  request body properties too. `Orchestrator.ask` now degrades to the
  operation's form whenever the endpoint is not `IsSafe()`, before the
  parameter is looked at at all; `optionsForParam`'s request-body branch
  had no caller left and is deleted. A safe operation's ask is unchanged.
  See `DECISIONS.md`, 2026-09-12 ("An unsafe operation is answered by its
  form, not a question"), and `docs/specs/orchestration.md` D11 (amended)
  and section 8b. `web/` needed no change - verified, not assumed.
- D15 (an optional enum parameter on a safe endpoint offered to the model
  as required, with a synthetic `__all__` value appended so the model
  cannot silently drop a filter it could not match): tried, measured three
  times against the eval corpus, and reverted - see `DECISIONS.md`,
  2026-09-12, and its final entry, and `STATE.md`'s own paragraph. The
  reject rate never moved outside its noise band; only the accept rate
  moved, because the synthetic value competed with `ask_user` rather than
  reinforcing it. The defect itself is open again - see item 3 under
  "Next". One piece of this work survives independent of D15:
  `jsonmode.renderParam` now renders a parameter's own contract
  `Description`, a real gap this work happened to find.
- `docs/plans/context.md`, Task 4: end to end - closes the multi-turn
  context subproject. `e2e/src/context.test.ts` proves AC-M-101 at the
  process level against the built platform: ask about inventory (or
  attendance), then ask the exact same service-less follow-up wording, and
  land back on the same service - and with no turns at all, the same
  wording answers `kind: "none"`. `e2e/browser/context.spec.ts` proves the
  same journey in headless Chromium. Both fix the follow-up's answer
  through `internal/adapter/planner/stub`, extended to key its table on the
  conversation too: `stub.Key` gained a `Turns` field
  (`stub.TurnsKey([]usecase.Turn) string`, `service/operationId` pairs,
  order-dependent, joined by `|`), so the very same `Query` can map to two
  different fixture rows depending on what came before it - the stub still
  performs no reasoning over turns, only a table lookup, so `make check`
  still never calls a real LLM. `pkg/app.PlanFixture` and
  `internal/infra/config.PlanFixture` both gained `Turns []TurnFixture`
  (`{service, operationId}`) to carry this through `ORCHESTRA_PLAN_FIXTURES`.
  D8 (`docs/specs/orchestration.md`) now says explicitly that a follow-up's
  question and decision go back on the next request, never a row of the
  answer, and that rendering them is still part of the same one LLM call.
  A real model's own ability to do this was measured by hand, not asserted
  by a test - five follow-ups against `qwen3.5-9b-q8`, 5/5 stayed on the
  right service - recorded in `DECISIONS.md`, 2026-09-12 ("qwen3.5-9b-q8
  carries a follow-up's context, measured by hand"). See `STATE.md` for the
  whole subproject's summary (Tasks 0-4).
- `docs/plans/auth.md`, Task 6: end to end. Every existing e2e/browser
  suite now signs in first; `e2e/src/auth.test.ts` proves AC-A-103,
  AC-A-104 and AC-A-105 at the process level (grant one service, ask a
  question, see it answered from only that service; a workspace one person
  makes is invisible to another) and `e2e/browser/auth.spec.ts` proves
  AC-A-107 in headless Chromium (sign in, chat, sign out, back to the
  sign-in screen, even after a reload). New: `ORCHESTRA_SEED_ACCOUNTS`
  (`internal/infra/config`), a non-admin account seed for a built binary,
  mirroring `ORCHESTRA_PLAN_FIXTURES`; `ORCHESTRA_SECURE_COOKIE=false` in
  every e2e/browser platform-starting env, since these suites run over
  plain HTTP. `make check` (not `-k`) is fully green - this closes
  `docs/plans/auth.md`: every task done, every criterion in
  `docs/specs/auth.md` section 9 has a test running in CI. See
  `DECISIONS.md`, 2026-09-12 ("Auth Task 6: end to end").
- Closed the nil-store auth bypass: `pkg/app.New` now refuses to build a
  handler when `Config.DBPath` is empty (`app.ErrMissingDBPath`), so
  `requireSession`'s old "no store means run every request as a fixed
  admin" branch could never fire again - deleted, along with
  `newStubAdmin`. Test-first (`TestNewFailsWhenDBPathIsEmpty`). See
  `DECISIONS.md`, 2026-09-12 ("Closing the nil-store auth bypass").
- `docs/plans/workspaces.md`, Task 7: end to end. `e2e/src/workspaces.test.ts`
  proves AC-W-105 (a workspace survives a restart of the platform) at the
  process level - create a workspace and a panel over HTTP, stop the built
  platform binary, start a fresh one on the same `ORCHESTRA_DB_PATH` file,
  read the workspace back. `e2e/browser/workspace.spec.ts` drives the built
  product in headless Chromium through the whole journey: ask, save, reopen
  (a real page reload), see the panel draw, refresh it (AC-W-101, AC-W-102,
  AC-W-103). Both suites keep their own temporary database file and feed the
  stub planner through `ORCHESTRA_PLAN_FIXTURES` - no real LLM is ever
  called by `make check`. See `DECISIONS.md`, 2026-09-11 ("Workspaces
  Task 7"). This closes `docs/plans/workspaces.md`: every criterion in
  `docs/specs/workspaces.md` section 10 now has a test that runs in CI.
- `docs/plans/workspaces.md`, Task 6: asking from the workspace screen.
  `web/src/pages/workspace/ui/WorkspacePage.tsx` carries
  `widgets/conversation`'s `ConversationPanel`, passed the current
  `workspaceId` as `defaultWorkspaceId` so a result's save control defaults
  to the workspace it was asked from (AC-W-104).
- `docs/plans/workspaces.md`, Task 5: a panel can be run again.
  `web/src/entities/workspace/model/usePanelInvoke.ts` exposes `refresh`; the
  refresh control never disables itself, only swaps its icon/label while in
  flight, since a disabled `contained` `Button` fails `make guard-layout`
  (AC-W-103).
- `docs/plans/workspaces.md`, Task 4: a result can be kept.
  `web/src/features/workspaces/ui/SaveToWorkspaceControl.tsx` posts a
  result's `source`/`component` as a panel, with an editable title
  defaulting to the question that produced it, and can create a workspace
  on the spot (AC-W-101).
- `docs/plans/workspaces.md`, Task 3: a workspace draws its panels.
  `web/src/pages/workspace/ui/PanelResult.tsx` composes
  `entities/workspace`'s card shell with `entities/rendering`'s provenance
  and result widgets; each panel loads independently, so one unreachable
  service only shows up in its own card (AC-W-102, AC-W-106).
- `docs/plans/workspaces.md`, Task 2: the drawer lists workspaces.
  `web/src/features/workspaces` (list, create, delete) and
  `NavigationDrawer`.
- `docs/plans/workspaces.md`, Task 1: the workspace endpoints -
  `POST`/`GET /api/workspaces`, `GET`/`DELETE /api/workspaces/{id}`,
  `POST /api/workspaces/{id}/panels`, `DELETE
/api/workspaces/{id}/panels/{panelId}` - against Task 0's store, rejecting
  an unexposed operation with the same 400 `/api/invoke` gives.
- `docs/plans/workspaces.md`, Task 0: workspaces have somewhere to live.
  `internal/domain/workspace.go` (`Workspace`, `Panel`, stdlib only);
  `internal/usecase/workspaces.go` (`WorkspaceStore` port); the SQLite
  implementation in `internal/adapter/repository/sqlite` (`modernc.org/sqlite`,
  pure Go, schema embedded via `go:embed` and applied at open, every
  statement `IF NOT EXISTS`); `ORCHESTRA_DB_PATH` in `internal/infra/config`,
  required, no default (`config.ErrMissingDBPath`); wired into `pkg/app.New`
  as a startup-time open-then-close check (`checkWorkspaceStore`) since
  nothing speaks HTTP to workspaces yet. IDs are `crypto/rand.Text()`
  (Go 1.24+, no error return), tests never assert a specific one - they
  capture what `Create`/`AddPanel` return and check reads echo it back.
  `Args` (`map[string]any`) is JSON only inside the sqlite adapter -
  `encoding/json` may not reach `domain` or `usecase` (depguard) - marshalled
  on write, unmarshalled on read. `WorkspaceStore.AddPanel` takes `*domain.Panel`,
  not the value shown in the plan's pseudocode, for the same `gocritic`
  hugeParam reason `pkg/app.Config` is already a pointer (112 bytes each).
  91.5% package coverage (`t.TempDir()`, plus a second store opened on the
  same file proving persistence - AC-W-105's storage half). See
  `DECISIONS.md`, 2026-09-11.
- `harness/guard/exposed-ops.sh` now also requires `title` on every property
  an exposed operation actually draws on screen (response fields, an object
  request body's fields, and parameters - `ask` can degrade any exposed
  operation into a form built from `inputSchemaFor`, which merges
  parameters in). Bundling switched to `--dereferenced` so a shared schema's
  `title` is visible through every `$ref` to it. Added the missing `title`
  on `getInventoryItem`'s and `getAttendanceRecord`'s `id` path parameter
  (`DECISIONS.md`, 2026-09-11).
- Measured the two planners against the same ambiguous questions: the JSON
  planner reaches `ask` where the tool-calling one guesses, and answers
  `none` where it talks itself into a guess (`DECISIONS.md`, 2026-09-11).

- Imported the repository harness from takamai at HEAD, renamed every
  identifier, and removed all product code (`DECISIONS.md`, 2026-09-10).
- Rebuilt `DECISIONS.md`: kept the eleven entries that justify the harness,
  dropped the ten that describe a product this repository does not have.
- Rewrote `PRODUCT.md`, `STATE.md` and `TODO.md` for app-orchestra, and fixed
  the documentation drift inherited from takamai.
- Made `.gitignore` deny by default, then allow by path rather than by
  extension (`DECISIONS.md`, 2026-09-11).
- Added the Claude Code layer: `CLAUDE.md`, the permission allowlist and the
  after-edit hook.
- Reorganised the repository into `harness/`, `services/<name>/`, `web/` and
  `e2e/` (`DECISIONS.md`, 2026-09-10), and removed the NO_COLOR machinery
  (`.env`, `.npmrc`, and the `postinstall` rewrite of `node_modules/.bin/vp`).
- Made every target, guard, hook and CI job discover services instead of naming
  one, and verified it against a temporary second service.
- Upgraded the toolchain: Go 1.27.1, pnpm 12.3.4 and the whole catalog. Found
  that golangci-lint must be rebuilt by the Go it analyses, and tied that to
  `toolchain.mk`. TypeScript 7 was tried and reverted (`DECISIONS.md`).
- Designed the first vertical slice (`docs/specs/orchestration.md`) and wrote
  its acceptance criteria into `PRODUCT.md`.
- Closed the two harness gaps the design exposed: `guard-generated-ops` fails on
  an operation id a generated file dropped, and a Redocly rule fails an `enum`
  with no `x-enum-labels`.
- Built the slice's foundation: the platform's shell and health endpoint, the
  `inventory` and `attendance` services, the domain's rendering rule, and the
  catalogue the platform fetches from the running services.
- Converted the catalogue into tool definitions (`usecase.ToolsFor`,
  `usecase.AskUserTool`), enum labels folded into each property's description.
- Added `/api/plan` and `/api/invoke` to the platform's contract, the
  `Planner`/`Invoker`/`Orchestrator` ports and the stub planner, and wired the
  safe-call and `none` paths of `/api/plan` end to end against the running
  services (`DECISIONS.md`, 2026-09-11, three entries).
- Built the form path for an unsafe call and `/api/invoke`'s real execution
  with argument validation against the catalogue.
- Built `ask_user`: `Orchestrator.ask` renders `kind: "ask"` from the
  catalogue's own enum, never from a Decision's own (model-supplied) options;
  the stub planner routes on `{query, answers}` so a re-posted answer reaches
  a different decision (`DECISIONS.md`, 2026-09-11).
- Built the OpenAI-compatible chat transport (`internal/adapter/planner/chat`)
  and the tool-calling planner (`internal/adapter/planner/toolcall`), wired
  `pkg/app` to select it whenever `ORCHESTRA_LLM_BASE_URL` is configured, and
  deleted `defaultPlanFixtures` now that a real planner exists. Measured four
  local models against the same tool definitions and set `qwen3.5-9b-q8` as
  the default (`DECISIONS.md`, three entries, 2026-09-11).
- Added the public mark: `x-orchestra-expose` (default off), read once in
  `internal/adapter/specsource/http.parseSpec` as the catalogue is built, so
  `ToolsFor` and `/api/invoke` read the same filtered set and can never
  disagree. Deleted `ToolsFor`'s shape-based exclusion now that exposure is
  a declared intent. Marked `listInventoryItems`/`createInventoryItem`/
  `getInventoryItem` and their attendance equivalents exposed; left both
  services' `GET /openapi.yaml` and the platform's own contract unmarked.
  Added `harness/guard/exposed-ops.sh` (`make guard-exposed-ops`), which
  fails on an exposed operation nothing can render, or a service exposing
  nothing at all (`DECISIONS.md`, 2026-09-11;
  `docs/specs/orchestration.md` D13).
- Built `choice` (Task 15): `entities/rendering/ui/ResultChoice.tsx` renders
  a `kind: "ask"` answer's question and Japanese-labelled options, re-posts
  `/api/plan` with the original query (found by walking `TurnList`'s turn
  array back to the nearest question turn) and the chosen answer, and wires
  the result into `TurnList` in place of the last placeholder. Extracted
  `entities/rendering/model/useSubmission.ts` out of `ResultForm` so the two
  components' submit/error handling is written once. Verified live: 1 `ask`
  in 5 attempts against `qwen3.5-9b-q8` (matching the prior measurement),
  fixed in `Conversation.test.tsx` regardless of what any particular live
  run does.
- Added `list_capabilities` (`service?: string`), a built-in tool alongside
  `ask_user` that answers "what can this do?" from the catalogue itself:
  `Orchestrator.listCapabilities` renders it as `kind: "result"` /
  `component: "table"` without calling any service, `toolcall/planner.go`
  routes the tool name before it would reach `resolveService`, and
  `kind: "none"`'s message now points at it instead of being a dead end. No
  `openapi.yaml` or frontend change was needed - verified live and with
  `web/src/entities/rendering` unit tests unchanged (`DECISIONS.md`,
  2026-09-11; `docs/specs/orchestration.md` D14).
- Fixed a 500: an `ask_user` naming a parameter with no declared enum (a
  free-text required field, such as `CreateInventoryItem`'s `name`, when the
  question never said what to call the thing) now degrades to `kind: "form"`
  instead of `usecase.ErrUnknownParam` (deleted). `Orchestrator.ask`'s form
  and `Orchestrator.call`'s unsafe-call form both now build their schema from
  `inputSchemaFor` (`tools.go`) instead of the deleted `formSchema`, so a
  form also covers path/query parameters, not just a request body.
  `askUserDescription` was sharpened after measuring. Measured live against
  `qwen3.5-9b-q8`, before and after (`DECISIONS.md`, 2026-09-11).
- Task 16, the end-to-end suite: `e2e/src/orchestration.test.ts` (process
  level, built binaries, free ports) and `e2e/browser/chat.spec.ts`
  (headless Chromium against the platform serving `web/dist` via
  `ORCHESTRA_STATIC_DIR`). Both drive the stub planner through the new
  `ORCHESTRA_PLAN_FIXTURES` environment variable rather than a real LLM
  (`DECISIONS.md`, 2026-09-11). `make check` is now fully green, including
  `guard-a11y`/`guard-layout` against a real screen for the first time.
- Task 11, the JSON planner: `internal/adapter/planner/jsonmode.Planner`, a
  second `usecase.Planner` for models that cannot call tools - renders the
  catalogue as text, asks for one JSON object (`kind`: `call`/`ask`/
  `list_capabilities`/`none`), validates a `call`/`ask` answer against
  `domain.Catalog` (unknown operation, or an argument outside its
  parameter's enum), and retries once, quoting the failure back, before
  giving up. `ORCHESTRA_LLM_MODE` (`toolcall`/`json`) selects the adapter in
  `pkg/app.newPlanner`; an unknown value fails startup. Verified live:
  twelve runs (four questions, three times each) through
  `ORCHESTRA_LLM_MODE=json` against `qwen3.5-9b-q8`, twelve clean answers,
  zero retries - after discovering and fixing that `response_format`'s JSON
  Schema must preserve the prompt's own field order, not the alphabetical
  order a `map[string]any` marshals to (`DECISIONS.md`, 2026-09-11). This
  closes the first vertical slice: all seventeen tasks in
  `docs/plans/orchestration.md` are done.

- The eval suite (docs/specs/eval.md) is done: `e2e/eval/`, `make eval` /
  `make eval-accept`, baseline committed (`DECISIONS.md`, 2026-09-12). Not
  yet covered: attendance-specific cases (the corpus is inventory-only,
  since that is where the dropped-filter behaviour docs/specs/eval.md exists
  for was actually observed), and a wider `ORCHESTRA_EVAL_N` for anyone
  willing to spend more wall time on a tighter measurement of the
  enum-less-filter case's true rate.
