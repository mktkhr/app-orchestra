# STATE.md — current implementation state

_Last updated: 2026-09-14 (`docs/specs/storage.md` closed - one `*sql.DB`
per database file, WAL, a busy timeout, and a 500 that reaches the log; see
below and `DECISIONS.md`)_

## Summary

**`docs/specs/storage.md` is closed - the eleventh subproject, and the
correction to 2026-09-13's "load, measured" reading.** Forty concurrent
requests that each resolve a session and write a row (`TestConcurrentSessionReadsAndWritesAllSucceed`,
`services/platform/acceptance/storage_test.go`) failed 8-19 of 40 times
against the code as it stood, with `SQLITE_BUSY` in `requireSession`'s own
session read - confirmed by running it, not assumed. Three causes, all in
`internal/adapter/repository/sqlite`: `Store`, `Users`, `Sessions` and
`Permissions` each opened their own `*sql.DB` on the same file (`New`,
`NewUsers`, `NewSessions`, `NewPermissions` each called `openDB`
independently); nothing set a `busy_timeout`; nothing set `journal_mode`.

**S1 (one `*sql.DB`, AC-S-102).** `openDB` is unchanged as the private
worker every path-taking constructor calls, but a new exported `Open(path)
(*sql.DB, error)` returns the same kind of handle for a caller that needs
to share it, and each store gained a second constructor -
`NewFromDB`/`NewUsersFromDB`/`NewSessionsFromDB`/`NewPermissionsFromDB` -
that wraps an already-open `*sql.DB` instead of opening its own.
`pkg/app.build` now calls `sqlitestore.Open(cfg.DBPath)` once and threads
that one `db` through `newWorkspaceHandler`, `newPermissionStore` and
`newAuth`, which no longer take a path or return an error for opening (only
`newAuth` can still fail, seeding the admin account). No package-level
mutable state: a `sync.Mutex`-guarded registry keyed by path was the first
design, and `gochecknoglobals` (`harness/quality/go/golangci.yml`'s own
"guardrails against agent shortcuts") correctly refused it - the two extra
constructors per store, called once from `pkg/app.build`, replace it
without any global. Every existing test that calls `New`/`NewUsers`/
`NewSessions`/`NewPermissions` with its own path is untouched (AC-S-104):
they still open their own private connection, exactly as before.

**S2/S3 (WAL and a five-second busy timeout).** Both are DSN query
parameters modernc.org/sqlite documents and validates itself -
`?_journal_mode=WAL&_busy_timeout=5000`, appended to the path `openDB`
already opened - confirmed directly against the pinned driver version
(v1.58.0) by opening a file with this DSN and reading `PRAGMA journal_mode`
/ `PRAGMA busy_timeout` back (`wal`, `5000`); also pinned as a test
(`TestOpenAppliesThePragmasEveryStoreNeeds`,
`internal/adapter/repository/sqlite/store_internal_test.go`).

**S4 (a 500 reaches the log, AC-S-103).** A new `internal/infra/httpserver/logging.go`:
`logInternalServerErrors` wraps the whole `/api/` handler chain (outside
`requireSession`, so it also catches that middleware's own 500) in a
`responseRecorder` that captures the status and, only for a 5xx, the body -
every 500 this codebase writes already carries its cause as
`openapi.ErrorResponse.Message` (`writeSessionError`, and every
`*500JSONResponse` `plan.go`/`invoke.go`/`workspace.go`/`users.go` write) -
and logs it through `slog.Default()`. `cmd/api/main.go` now calls
`slog.SetDefault(logger)` once, at startup, so that default is the same
JSON handler cmd/api already logs with, rather than adding a logger
parameter to `pkg/app.Config`.

**Measured, not assumed, in both directions.** The acceptance test above
failed reliably before this change and passed ten times in a row after it;
`make guard-layout` - the intermittent 500 this spec exists to remove - was
run ten consecutive times after the fix and passed every time. See
`DECISIONS.md`, 2026-09-14, for the correction to the 2026-09-13 "load,
measured" entry this result overturns.

**`docs/plans/proposing.md` is closed - FR-F-5, asking the chat to add a
panel** (`docs/specs/proposing.md` section 7, AC-N-101 through AC-N-106).
Three tasks:

- **Task 0** (`a349fb3`) gave the model a fifth built-in tool,
  `propose_panel(service, operationId, args, component?, chart?,
transform?, title?)`, alongside `ask_user` and `list_capabilities`
  (`usecase.ToolsFor`), and `/api/plan` a fifth result `kind`,
  `"proposal"` (`usecase.ResultKindProposal`), beside `result`/`form`/
  `ask`/`none`. `Orchestrator.propose` never calls the invoker (N1): it
  resolves a `DecisionProposal` into a proposed panel, filling in from the
  catalogue whatever the model left unset - the component from
  `domain.Render`, the view from the operation's own `x-ui-hint.chart`,
  the title from the operation's display name - the model's own values
  winning where given. An operation outside the caller's catalogue cannot
  be proposed at all (AC-N-105), the same refusal every other decision
  kind already gets. Both the tool-calling and jsonmode planners map onto
  the same `Decision` shape; the stub planner answers a `propose_panel`
  fixture the same table-lookup way as every other `DecisionKind`
  (AC-N-106).
- **Task 1** (`64a2e83`) drew a proposal as `features/panels`' own
  `AddPanelForm`, opened over the proposal instead of over nothing
  (`ProposalControl`, `usePanelProposal`) - one control, labelled "配置"
  (not "追加"/"保存"), places it via the unchanged `POST
/api/workspaces/{id}/panels`; editing a field before pressing it places
  the edit (AC-N-103). Only `widgets/conversation/ui/ConversationPanel`
  on the workspace screen ever draws one - the chat screen's own
  conversation draws no proposal turn at all even when handed one
  (AC-N-104, N4).
- **Task 2** (`0d96ce9`) is the end-to-end journey and the measurement.
  `e2e/src/proposing.test.ts` drives the built platform binary over real
  TCP: ask, get back `kind: "proposal"` with the panel the question
  described, confirm the workspace holds no panel until one is explicitly
  posted (AC-N-101, AC-N-102). `e2e/browser/proposing.spec.ts` drives the
  same journey through Chromium: ask the workspace's chat for a chart,
  press "配置", reload, see the panel drawn. Driving either through the
  _built binary_ needed a gap closed first: `ORCHESTRA_PLAN_FIXTURES`'s
  pipeline (`internal/infra/config.PlanFixture` → `cmd/api.
toAppPlanFixtures` → `pkg/app.PlanFixture` → `pkg/app.toDecision`) had
  no way to produce a `DecisionProposal` - only `ask`/`call` - so it
  gained `Propose`/`Component`/`Chart`/`Title` fields, mirrored across all
  three types, the same way `Ask` already does.

  **The measurement (section 9).** `propose_panel` rides in every
  request's tool list, on every question, whether or not a workspace is
  open - `make eval`'s corpus is what says whether that changed anything
  about a question with nothing to do with panels. Run before this
  subproject's commits (worktree at `0d2a5bf`) and after (`0d96ce9`, plus
  a full `make check`): `no-enum-value` (judged on `reject` at n=30, band
  15-22 over nine samples, `docs/specs/eval.md` section 4a) read
  **17/30 reject before, 13/30 after - outside the band**. See
  `DECISIONS.md`, 2026-09-14, for the reading: every other case held
  (`accept`/`reject` unchanged), so this is not noise across the whole
  corpus, but `no-enum-value` was already the corpus's least stable case
  before this subproject touched anything, and one before/after pair is
  one sample, not three. Recorded, not corrected: `e2e/eval/baseline.json`
  is untouched (`make eval-accept` was not run - that is a human's act).

**A screen's address is a path, read by `react-router`** (`docs/plans/routing.md`
Task 1, `docs/specs/routing.md` section 3, AC-R-101 in the browser;
`DECISIONS.md`, 2026-09-13, two entries). `web/src/app/App.tsx` wraps the
whole tree in `BrowserRouter`; `MainContent.tsx` (`app/ui`) reads the path
with `Routes`/`Route` instead of `useHashRoute` (deleted, along with its
test): `/` is the chat, `/workspaces/:workspaceId` a workspace (keyed by
that id in `ConversationProvider`, unchanged otherwise -
`docs/specs/context.md` M6), `/users` the admin's screen, and anything else
the chat, same as the hash router before it. `AuthGate` did not move (R4).
The drawer's rows (`DrawerNavItem`, `WorkspaceListItem`) render `react-router`'s
`Link` in place of a plain `href="#..."` anchor, so following one changes
the screen without a page load. `react-router` is pinned exactly (`8.3.1`),
the way `@mui/x-charts` and `react-grid-layout` are, for the same reason.

`harness/quality/browser/screens.ts` navigated to `/#users` and
`/#workspace-{id}`; moving it to `/users` and `/workspaces/{id}` was Task
2's Step 1, done here instead of behind a temporary hash-redirect shim that
was built, found to need a `hashchange` listener to work at all (a
`page.goto` differing only by fragment is a same-document navigation, not a
reload, so a mount-only effect never sees it), and then dropped rather than
finished - `docs/specs/routing.md` section 3 already says nothing redirects
the old addresses. `e2e/browser/*.spec.ts` navigate by clicking rather than
by address, so Task 2's Step 2 needed only one fix: `layout.spec.ts` read a
workspace id back out of `page.url()` by splitting on `"workspace-"`.
`docs/plans/routing.md` Task 2 keeps only the end-to-end journey (Steps
3-6).

**The platform serves `index.html` for any address that is not `/api/...`
and not a file it has** (`docs/plans/routing.md` Task 0,
`docs/specs/routing.md` section 4, AC-R-102/AC-R-103; `DECISIONS.md`,
2026-09-13). `internal/infra/httpserver/router.go`'s `spaHandler` replaces
the bare `http.FileServer(http.Dir(staticDir))`: a real file under
`staticDir` is served as itself; an extensionless path (an application
address) gets `index.html`; a path with an extension but no matching file
still 404s, so a missing build asset is never handed HTML to parse.
`staticDir` empty is unchanged - no fallback is registered at all, as
before.

**`docs/plans/routing.md` is closed** (`docs/specs/routing.md` section 7,
all four criteria; `DECISIONS.md`, 2026-09-13). Task 2's remaining steps:

- `e2e/src/routing.test.ts` proves AC-R-102/AC-R-103 the way a unit test
  cannot - over real TCP, against the built platform binary serving the
  real `web/dist` (not a `t.TempDir()` fixture): an unknown path answers
  the built index, a real built asset is itself, a missing asset 404s and
  is not HTML, and `/api/does-not-exist` answers as the API (404, signed
  in) rather than as the application.
- `e2e/browser/routing.spec.ts` proves AC-R-101 end to end: sign in, open
  a workspace by clicking its row, note the address, reload, and the same
  workspace draws again (not the chat `useHashRoute` would have fallen
  back to) - and the same for `/users`.
- AC-R-104 was checked, not assumed, before writing its test
  (`DECISIONS.md`, 2026-09-13): it already holds, for free, from how
  `App.tsx` is built. `BrowserRouter` sits above `AuthGate`
  (`docs/plans/routing.md` Task 1, R5) and reads `window.location.pathname`
  once, independent of which of `SignInPage`/`Shell` `AuthGate` renders
  under it; `SessionProvider.signIn` only sets `user` in React state and
  never navigates. So a person who types `/workspaces/{id}` in with no
  session sees the sign-in screen at that same address, and signing in
  swaps `AuthGate`'s output without the router ever having moved off it -
  `MainContent` resolves the still-current path to the workspace on the
  next render. `e2e/browser/routing.spec.ts`'s own second test drives this
  exact journey (sign in, open a workspace, sign out, `page.goto` its
  address directly, sign in again from the form, land on it) rather than
  arguing it from the source alone.

**A screen's address can be pasted to somebody else and survives a
reload** - the subproject's own "Done" line (`docs/plans/routing.md`).
Every criterion in `docs/specs/routing.md` section 7 has a test that runs
in `make check`; nothing calls a real LLM to do it
(`docker logs llama-swap`'s count is unchanged across the run).

**A panel over an unsafe operation never calls `/api/invoke` on its own,
and a panel has exactly one scroller** (`docs/specs/dashboard.md` P14/P15,
AC-P-111/AC-P-112; `DECISIONS.md`, 2026-09-13, two entries). `PanelResult`
(`pages/workspace/ui`) looks the panel's operation up in `GET /api/catalog`
(`entities/workspace/model/useCatalogEntry.ts`) - the only source that
cannot drift, since a `Panel`'s own `component` is a person's editable
display choice, not a safety signal - and gates `usePanelInvoke`'s new
`enabled` argument on that lookup resolving to something other than
`component: "form"`. An unsafe panel draws `PanelQuickAddBody.tsx`
(`entities/rendering`'s `ResultForm`, seeded from the panel's saved args)
instead, and its own refresh control clears a submitted result back to a
blank form rather than calling anything. `PanelCardShell`'s `CardContent`
no longer scrolls itself; `ResultTable`/`ResultTableGrid`'s
`TableContainer` does, with `tabIndex={0}` for `make guard-a11y` and
`flexShrink: 0` on the pagination control and button row so pagination
never gets squeezed into scrolling itself.

**A panel can be changed after it is made** (`docs/specs/dashboard.md`
section 6a, P11-P13; see `DECISIONS.md`, 2026-09-13). Three pieces:

1. `PATCH /api/workspaces/{id}/panels/{panelId}` (`UpdatePanelRequest`):
   `title`/`args`/`component`/`view` are all optional and only the ones
   named change; `service`/`operationId` are not properties of this schema
   at all (P13). `view` is the one field with a third state - absent
   leaves it alone, `null` removes it - carried on the wire as
   `nullable.Nullable[View]` (per-property `x-go-type`/
   `x-go-type-skip-optional-pointer`, not the harness-owned
   `oapi-codegen.yaml`'s own `nullable-type` option) and, below the
   handler, as `domain.PanelPatch.View`, a `**View` (domain may not import
   the adapter's own nullable type).
2. `usecase.Workspaces.UpdatePanel` narrows through the same `catalogFor`
   `AddPanel` already uses, re-checked against the panel's own (fixed)
   operation rather than trusted from save time - permissions can change
   after a panel is made. Refused exactly the way `AddPanel` is (AC-P-109):
   `ErrEndpointNotFound` for an operation no longer callable,
   `ErrWorkspaceNotFound` for a workspace or panel id that does not exist
   or belongs to somebody else - one sentinel for all three, so a 404 never
   says which is true. `sqlite.Store.UpdatePanel` writes only the named
   columns.
3. The edit form is the builder's own (P12): `usePanelFields` gained a
   `seed` parameter rather than a second hook (its pure rules split into
   `panelFieldRules.ts`/`panelFieldValues.ts` to stay under
   `max-lines`/`max-lines-per-function` once seeding was added);
   `AddPanelForm` gained `operationLocked`, which states the fixed
   operation as plain text (`OperationLabel.tsx`) rather than a disabled
   `Autocomplete` - `make guard-layout`'s selector matches a disabled
   input exactly as an enabled one, and a disabled MUI input's own lighter
   border is the low-contrast edge that guard exists to catch, so the
   control is removed from that selector entirely instead of betting it
   clears the threshold; `usePanelEditor`/`EditPanelControl` (with
   `EditPanelDialogContent`, split out to stay under
   `import/max-dependencies`) sit beside `PanelResult`'s refresh control in
   a new `PanelActions.tsx`. The edit form always restates every field it
   shows (title/args/component/view) rather than omitting untouched ones -
   it is the whole panel's own state once open, so there is no "leave
   alone" case for a control already on screen; `view` is sent explicit
   `null` whenever no chart/transform applies.

Found and fixed a real gap along the way: `PanelArguments.tsx` never
accepted seeded values - its `useFormValues(schema)` call always reset to
each field's own empty default, so an edited panel's arguments would have
silently come back blank. Fixed by threading an `initialValues` prop
through to `useFormValues`'s existing `initial` parameter (already used by
`ResultForm`). Only the browser journey caught this - a good argument for
`e2e/browser/dashboard.spec.ts`'s new edit test, not just the unit-level
ones. `e2e/src/dashboard-update-permissions.test.ts` is AC-P-109's own
process-level test, its own file (not a `describe` added to
`dashboard-permissions.test.ts`, already at its `max-lines` budget).
`make check` is fully green; `docker logs llama-swap`'s request count did
not move (the planner is never involved here).

**Follow-up fix, one level up: a service has no Japanese name either, and
that is closed too** (see `DECISIONS.md`, 2026-09-13). Same shape as the
entry below, one level up: `inventory`/`attendance` showed raw wherever a
_service_ appeared - `OperationPicker`'s group headers, `list_capabilities`'
サービス column, a result's provenance (`Provenance.tsx`), the admin's
permission grid (`ServiceCard.tsx`). `x-ui-hint` (already on an operation,
D15) now also sits on a service's own `info` object
(`docs/specs/orchestration.md` D16); `specsource/http` reads it once per
service and copies it onto every one of that service's endpoints, the same
way `Service` itself already is. `domain.Endpoint.ServiceDisplayNameOr(fallback)`
mirrors `DisplayNameOr`; `CatalogEntry.serviceDisplayName`,
`Operation.serviceDisplayName` and `Source.serviceDisplayName` (a
`/api/plan` result's provenance) each fall back to `service` - the
identifier - exactly where that screen already showed it, so a contract
declaring none changes nothing. `service` itself never changes anywhere:
`ORCHESTRA_SERVICES`, `source.service`, a `Permission` row, a `Panel`, and
what `/api/invoke` resolves are all untouched. One exception: a saved
`Panel` stores only the identifier and `POST /api/invoke` returns no
`Source` at all, so `PanelResult.tsx`'s own hand-built provenance still
reads the identifier there, same as `operationId` already does in that
spot. `services/inventory` and `services/attendance` each declare one
(在庫管理, 勤怠管理). Verified `make eval` was not run, no operation's
`summary` changed, and `docker logs llama-swap`'s
`POST /v1/chat/completions` count is unchanged across a full `make check`
run. `make check` is green.

**Follow-up fix: English showing through the panel builder is closed** (see
`DECISIONS.md`, 2026-09-13). Two causes. (1) A bug: `usePanelFields.ts`'s
`fieldOptionsFor` returned bare property names for the chart's axis pickers
and the transform's `groupBy`/aggregate-field pickers, throwing away the
`title` a table already draws from the same `fields` shape
(`entities/rendering/model/rows.ts`'s `columnTitle`) - fixed by having
`fieldOptionsFor` return `{value, label}` pairs, and a new
`AGGREGATE_LABELS` (`entities/rendering/lib/transform.ts`) labelling
`applyTransform`'s own output keys (`count`/`sum`/`avg`), which name no
contract property. (2) A design gap: no operation had a Japanese name
anywhere - `OperationPicker`, a panel's default title, and
`list_capabilities`' 操作 column all read either `summary` (a contract's
English tool description, `usecase.ToolsFor`'s `Tool.Description` -
deliberately not translated, since that would move `make eval`'s baseline)
or the raw operation id. `x-ui-hint` gains `displayName`
(`docs/specs/orchestration.md` D15), read the same lenient way as
`component`, carried as `domain.Endpoint.DisplayName` and read through the
new `DisplayNameOr(fallback)` - `GET /api/catalog`'s `CatalogEntry.displayName`
falls back to `summary`, `list_capabilities`' 操作 column falls back to the
operation id, each the value that screen already showed. Both dummy
services' `x-orchestra-expose: true` operations now declare one
(`docs/specs/dashboard.md` P9). `make check` is green.

**`docs/plans/dashboard.md` is done - all seven tasks.** Task 7 is the
whole subproject's own end to end journey plus the check `docs/plans/dashboard.md`
asked for first: that every acceptance criterion in `docs/specs/dashboard.md`
section 10 (AC-P-101 through AC-P-107) already had a test running in CI, and
that the person's own path - sign in, open a workspace, add a panel over a
list operation with a group-by and a bar chart, see it draw, reload, see it
draw again - actually works against the built product, not only against
each layer in isolation.

That check found two real gaps, both closed here (see `DECISIONS.md`,
2026-09-13, "Dashboard Task 7"): a chart-hinted chat answer never drew at
all (`features/conversation/ui/TurnList.tsx` had no branch for
`component === "chart"`, and `SaveToWorkspaceControl`/`useSaveToWorkspace`
dropped a result's own `view` on the floor when saving it - AC-P-105's own
wording, "a panel saved from that answer carries the contract's axes",
never held); and a chart built on top of a transform could not be built
through the panel builder at all - `usePanelFields.ts`'s axis pickers
always offered the raw response's own fields, never the two keys
`applyTransform` actually leaves in a grouped row (`groupBy` and the
aggregate's own name), so picking any real field as the value axis drew
nothing. A third gap turned up chasing the second one against the real
inventory binary: an untouched _optional_ argument (`ListInventoryItems`'s
own `status`) was posted as `""` and rejected by `usecase.validateEnumArg`
on every later refresh - `usePanelBuilder.ts`'s new `compactArgs` drops it
instead, the same way an omitted argument would be. All three are fixed in
product code, not worked around in the tests.

`e2e/src/dashboard.test.ts` and its sibling `e2e/src/dashboard-permissions.test.ts`
(split apart at the 300-line budget) are the process-level suite:
`GET /api/catalog`, a panel posted with a caller-built `view` (a transform
and a chart), a read-back that proves it survived, and AC-P-107's own
refusal against a narrowly-permissioned seeded account
(`ORCHESTRA_SEED_ACCOUNTS`, the same pattern `auth.test.ts` uses).
`e2e/browser/dashboard.spec.ts` drives the built product in headless
Chromium: create a workspace from the drawer directly (no question asked -
P8), build a panel over the real, unmodified `ListInventoryItems` with a
status group-by and a bar chart chosen entirely from the catalogue, see
four bars draw (`.MuiBarChart-element`, one per status the service's own
seed data holds), reload, and see the same four bars draw again from the
saved panel. Neither suite wires `ORCHESTRA_PLAN_FIXTURES` - nothing either
journey does asks a question, so the stub planner is never involved, which
is exactly P8's point.

Every criterion in section 10 now has a test and a place it runs: AC-P-101
(`services/platform/acceptance/catalog_test.go`, plus the process suite),
AC-P-102 (`features/panels/ui/AddPanelControl.test.tsx`, plus the browser
spec), AC-P-103/AC-P-104 (`pages/workspace/ui/PanelResult.test.tsx` and
`entities/rendering/lib/transform.test.ts` for the pure function, plus both
new e2e suites), AC-P-105 (`internal/usecase/orchestrator_test.go` and
`adapter/handler/plan_test.go` for the platform half, `ConversationChart.test.tsx`
and `SaveToWorkspaceControl.test.tsx` for the two chat halves this task
closed), AC-P-106 (`adapter/repository/sqlite/store_test.go` and
`PanelResult.test.tsx`), AC-P-107 (`internal/usecase/workspaces_test.go`
and the new `dashboard-permissions.test.ts`).

What is left of `docs/requirements.md` FR-F is only its second half:
arranging panels - dragging, resizing, persisting a layout (FR-F-4),
explicitly deferred by `docs/specs/dashboard.md` section 9. No plan exists
for it yet.

**`docs/plans/dashboard.md` Tasks 0-6 are done.** Task 6 adds
`features/panels/`, the workspace screen's own "add a panel" control
(`docs/specs/dashboard.md` section 6, P7/P8, AC-P-102): a toggle button
(`ui/AddPanelControl.tsx`) opens one form, not a wizard - every control for
the current choice on screen at once, absent when it does not apply. Step 1
(`ui/OperationPicker.tsx`) is an MUI `Autocomplete` over `GET /api/catalog`
(new `shared/api/catalog.ts`, lazily loaded on first open by
`model/useCatalog.ts`), grouped by service via the component's own
`groupBy`. Step 2 (`ui/PanelArguments.tsx`) reuses `entities/rendering`'s
`useFormValues`/`ResultFormFields` - both pulled out of `ResultForm.tsx`
unchanged in behaviour (it now composes them plus its own submit) so a
second caller could draw the identical controls over a catalogue entry's
`schema` without a second form; `PanelArguments` is remounted
(`key={service:operationId}`) whenever the operation changes, so a fresh
`useFormValues` reseeds instead of carrying over the previous operation's
values. Step 3 (`ui/ComponentPicker.tsx`) offers the entry's own `component`
plus `chart` whenever `fields` is present at all. Step 4 is
`ui/ChartFields.tsx` (category/value/kind, shown only for `component ===
"chart"`, defaulting from the entry's own `view.chart` when it has one) and
`ui/TransformFields.tsx` (an optional groupBy/aggregate/field, independent
of `component` - section 4's "a table with a transform is a perfectly good
panel"). Step 5 (`ui/PanelSaveFields.tsx`) is the title, defaulted to the
entry's `summary`, and the save button - never disabled at rest, only while
submitting; an incomplete choice makes `handleSave`
(`model/usePanelBuilder.ts`) silently no-op, the same shape
`useSaveToWorkspace` already uses. `usePanelBuilder` composes `useCatalog`
(step 1) and a new `model/usePanelFields.ts` (steps 2-5's state) to stay
under `max-lines-per-function`; saving without a chart or a transform posts
no `view` at all. `pages/workspace/ui/WorkspacePage.tsx` hosts the control
through a new `model/useWorkspacePage.ts` (wrapping `useWorkspace` and a
new `useAddedPanels.ts`, to stay under `import/max-dependencies`) and shows
a panel just added by appending it to local state - the panel the POST
already returns is everything `PanelResult` needs, so no second
`GET /api/workspaces/{id}` round trip. See `DECISIONS.md`, 2026-09-13
("Dashboard Task 6") for the judgement calls (which components a chart is
offered alongside; the save button's silent no-op). `make check` is green.

**`docs/plans/dashboard.md` Tasks 0-5 are done.** Task 5 makes
`pages/workspace/ui/PanelResult.tsx` the seam that draws a saved panel the
way its `view` says to, reusing Task 0's `applyTransform` and Task 1's
`ResultChart` rather than a second grouping function or a second chart: the
transform runs on the rows as they arrive from `usePanelInvoke` -
`rowsFromData(result.data)` then, when `panel.view.transform` is present,
`applyTransform` over it - before the component is chosen, exactly where
`transform.ts`'s own comment says its two callers (this one and Task 6's
builder preview) sit. `panel.view.chart` present means a chart, drawn with
`ResultChart` over those same (possibly grouped) rows, regardless of what
`result.component` says; its absence means `RenderedResult` as before,
fed the grouped rows re-wrapped as `{ rows: … }` (an envelope
`rowsFromData` can still pull a sole array property out of) when a
transform ran, or `result.data` completely untouched when neither half of
`view` is present - the AC-P-106 regression the plan calls out by name,
since panels saved before this slice carry no `view` at all. `ResultChart`
gained optional `width`/`height` props, defaulting to its old fixed
320×240 so Task 1's own tests keep seeing exactly that; `PanelResult`
passes its own two sizes instead, chosen by `useMediaQuery("(min-width:600px)")`
(MUI's own `sm` breakpoint, named directly rather than through `useTheme`
to stay under `import/max-dependencies`) - 260×200 narrow, small enough to
sit inside a `PanelCardShell`'s `CardContent` padding at 375px without
overflowing, and 560×320 wide, filling a panel card instead of looking like
a mistake in one. `entities/rendering`'s barrel now also exports
`rowsFromData`/`Row` (already existed, was only reachable from inside the
slice) and `shared/api/client.ts` exports a `View` type alias - both pure
reuse, no new logic, so `PanelResult` could reach across the FSD boundary
without a second copy of either. A chart whose rows carry none of its
named fields draws `ResultChart`'s own existing empty state (`結果は0件です。`)
rather than a second, invented answer - pinned by test. `make check` is
green, including `guard-a11y`/`guard-layout` at 375px.

**`docs/plans/dashboard.md` Tasks 0-4 are done.** Task 4 adds `GET
/api/catalog`: `usecase.NewCatalog(catalog, permissions)` narrows through
the same `catalogFor(ctx, catalog, permissions, user)` `Orchestrator` and
`Workspaces` already call (`internal/usecase/auth.go`) - the seat
`docs/specs/auth.md` section 5 names - and builds one `usecase.CatalogEntry`
per surviving endpoint by reusing exactly the conversions a `/api/plan`
result already goes through: `inputSchemaFor` for `Schema`, `fieldsFor` for
`Fields` (nil, not an empty map, whenever `domain.FieldsSchema` finds
nothing - which includes every chart-hinted endpoint, since `FieldsSchema`
only ever describes a table's or a detail's own properties and
`RenderResult` picks `ComponentChart` ahead of both for one), `chartViewFor`
for `View`, and `domain.Render` for `Component`. Both `inputSchemaFor` and
`fieldsFor` stayed unexported: `internal/usecase/catalog.go` lives in the
same package as `orchestrator.go`, so no port-boundary export was needed.
`internal/adapter/handler/catalog.go` is a thin wire conversion, reusing
`toAPIView` (`workspace.go`) as-is. `GET /api/operations` (`Admin.Operations`)
is untouched - it still answers the admin's unfiltered "what does this
deployment have", a different question for a different audience
(`docs/specs/dashboard.md` section 5) - and this task did not merge them.
Acceptance: `services/platform/acceptance/catalog_test.go` - an admin sees
every operation across two services; a person granted one sees only its
one; no session is 401; a chart-hinted operation carries `view`, one
without omits it. `make check` is green except `guard-filelen`, a
pre-existing harness gap this task's contract growth exposed but did not
cause - see "Known gaps in the harness" below.

**`docs/plans/dashboard.md` Tasks 0-3 are done.** Task 3 adds
`x-ui-hint.chart` parsing, the sibling of the `x-ui-hint.component` `parse.go`
already read: `uiHint` (`internal/adapter/specsource/http/parse.go`) now
returns `(domain.Component, *domain.Chart, error)`, and `parseChartHint`
converts the raw `{category, value, kind}` map into a `domain.Chart`.
`component` keeps the leniency it always had (absent, non-object, or a
non-string `component` all mean "no override", never an error); `chart`
does not - a missing `category`/`value`, a `kind` outside `bar`/`line`/`pie`,
or a `chart` that is not an object at all is `errMalformedChartHint`, which
fails `Source.Fetch` outright (see `DECISIONS.md`, 2026-09-12, "Dashboard
Task 3"). `domain.Endpoint` gains `ChartHint *domain.Chart`, and
`Render`/`RenderResult` (`internal/domain/rendering.go`) both gained a new
second rule: `ChartHint` alone - with no `x-ui-hint.component` at all -
is now enough to choose `ComponentChart`, ranked directly under the
existing `UIHint` override and above the request-body/response-schema
rules. The contract's `PlanResult` gains `view` (the same `View` schema
Task 2 added); `usecase.Result` gains `View *domain.View`, filled in by
`chartViewFor` in `Orchestrator.invokeAndRender` - only `View.Chart` is
ever set from a contract, never `View.Transform` ("The shape everything
shares": a contract declares axes, never a transform). `handler.Plan`
carries it onto the wire with the same `toAPIView` Task 2 already built for
`Panel.View` (`workspace.go`), reused as-is. No dummy service declares
`x-ui-hint.chart` - `docs/specs/dashboard.md` section 1/7 keep them
unchanged - so every test exercising it uses
`internal/adapter/specsource/http/testdata/fixture.yaml`'s new
`countWidgets` operation (chart hint only, no `component`) plus inline
malformed specs in `source_test.go`.

**`docs/plans/dashboard.md` Tasks 0, 1 and 2 are done.** Task 2 adds the
contract's `View` schema (`transform`/`chart`, both optional, per
"The shape everything shares") and `chart` as a fifth `Component`, wires
`view` onto `Panel` and `CreatePanelRequest`, and adds `internal/domain/view.go`
(`View`, `Transform`, `Chart`, `Aggregate`, `ChartKind` - pure, stdlib only)
and `internal/domain.ComponentChart` (in `rendering.go`; never returned by
`Render`/`RenderResult` - a chart is asked for, not implied by a response
shape). `internal/adapter/repository/sqlite` marshals `Panel.View` to and
from the nullable `panels.view` TEXT column the same way `Panel.Args`
already is (`store.go`'s `marshalView`/`unmarshalView`, JSON types private
to that adapter). The migration itself is `ensurePanelsViewColumn`
(`internal/adapter/repository/sqlite/migrate.go`): modernc.org/sqlite's
parser rejects SQLite's own `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`
syntax outright (a genuine syntax error, not a version gap - the driver's
`sqlite_version()` is 3.53.4), so idempotence is done in Go instead - check
`pragma_table_info('panels')` for a `view` column first, `ALTER TABLE` only
when it is missing - and is run unconditionally right after `schemaSQL`, on
every open. `TestNewOpensADatabaseFileWrittenBeforeViewExisted`
(`store_test.go`) builds a database file with the pre-Task-2 schema
verbatim (no `view` column), inserts a workspace and a panel directly with
`database/sql`, then opens it with `sqlite.New` and asserts the panel reads
back with a nil `View` and its other fields intact, and that the file is
still writable afterward. `usecase.Workspaces.AddPanel` did not already
refuse an operation the caller may not call (only one the whole catalogue
does not expose at all) - `docs/specs/auth.md`'s section 7 had deliberately
deferred that check to `/api/invoke`, for workspaces. AC-P-107 changes that:
`Workspaces` now takes a `usecase.PermissionStore` and narrows its catalogue
per call the same way `Orchestrator.catalogFor` does (admin bypasses it;
anybody else is narrowed by `PermissionStore.For` before `catalog.Find`),
so a panel over an unpermitted operation gets the same `ErrEndpointNotFound`
an unknown one does. `pkg/app.build` now opens the permission store before
the workspace handler instead of after, to thread it through. See
`DECISIONS.md`, 2026-09-12 (two entries: the migration, and the permission
narrowing).

**`docs/plans/dashboard.md` Tasks 0 and 1 are done.** `web/src/entities/rendering/lib/transform.ts`
exports a pure `applyTransform(rows, transform)`: groups rows by
`transform.groupBy` and reduces each group to `count`, `sum` or `avg`,
producing rows whose keys are `groupBy`'s own name and the aggregate's -
`[{status}, ...]` becomes `[{status, count}, ...]`, so a chart's
`category`/`value` can name fields that exist. Its own tests
(`transform.test.ts`) are the whole of its contract, per the plan: a row
whose `groupBy` value is missing, `null`, or not a string is grouped under
a shared `null` bucket rather than dropped; a non-numeric value under
`field` is skipped rather than producing `NaN`; `sum` of an all-skipped
group is `0`, `avg` of one is `null`; output order is first-seen order of
the group key. It has no callers yet - Task 5 adds one - and no
other file changed, on purpose (the function is pure, no React/MUI/I/O).

`web/src/entities/rendering/ui/ResultChart.tsx` draws a `data`/`category`/
`value`/`kind`/`title` result as a bar, line or pie chart, via `@mui/x-charts`
pinned at `9.4.0` (exact - the version installed for `@mui/material` and
`@mui/icons-material`, both `^9.4.0` but resolved to `9.4.0` in the
lockfile; a caret range on `@mui/x-charts` alone resolves to whatever is
newest, so it is pinned exact rather than relying on the lockfile never
being regenerated). It does not call `applyTransform` - it draws whatever
rows it is handed, grouped or not (Task 5 decides which). A row whose
`value` is not a finite number is dropped, not drawn as `NaN`, the same
call `transform.ts` makes for a non-numeric aggregate input; a row whose
`category` is not a string draws under the shared placeholder label
`"null"`, consistently with `transform.ts`'s `UNGROUPED` bucket, without
merging such rows into one mark (this component draws one mark per row).
Empty input, or every row filtered out, shows `ResultTable`'s own "結果は
0件です。" message rather than a blank chart. The chart's title is a
visible `<figcaption>`, not `@mui/x-charts`' own `title` prop: that prop
lands as `aria-label` on a `role="none"` container, and axe's
`aria-prohibited-attr` (WCAG 4.1.2) rejects that combination outright -
confirmed by rendering the built chart's HTML through axe-core directly,
not assumed from the library's docs.

**All four subprojects are done: `docs/plans/orchestration.md`'s seventeen
tasks, `docs/plans/workspaces.md`'s eight, `docs/plans/auth.md`'s seven, and
now every one of `docs/plans/context.md`'s five (Tasks 0-4).** A second
question in the same conversation is answered with the first one's turns in
hand: the browser keeps a conversation - one per screen, ended explicitly,
not by a component unmounting - and sends every earlier question and what
the platform decided for it (never a row of any answer) alongside the next
one; both planner adapters render that conversation into the prompt, after
the catalogue so the cache stays warm for the part that never changes; and a
follow-up phrased with no service name is measurably, if not perfectly,
answered from the service the question before it used. Accounts, sessions and
permissions live in the same SQLite file workspaces already use; the
catalogue narrows to the signed-in person before the planner or
`/api/invoke` ever sees it; a person signs in, and an admin reads every
account and grants or revokes what each one may call, per operation, from
a screen in the drawer only an admin sees (the endpoints refuse anybody
else regardless). `internal/adapter/auth/local` checks a name and password
against argon2id-hashed rows and seeds the first admin, once, from
`ORCHESTRA_ADMIN_PASSWORD` (required at startup, the same as
`ORCHESTRA_DB_PATH`). `POST`/`GET`/`DELETE /api/session` sign a person in
(HttpOnly, `SameSite=Lax`, `Secure` cookie carrying an opaque token,
configurable off via `ORCHESTRA_SECURE_COOKIE` for a link TLS does not
secure), report who is signed in, and sign them out; a session-resolving
middleware in `internal/infra/httpserver` answers every other route 401
without one, except `GET /api/health`. The web shell shows a sign-in
screen until a session exists, the signed-in person's name and a
sign-out control in the bar afterwards, and returns to the sign-in screen
on any 401 from anywhere.

**Task 6 closed the subproject end to end.** Every existing e2e/browser
suite now signs in first (`e2e/src/helpers/auth.ts`'s `signIn`/`withSession`
for the process-level suites, `e2e/browser/helpers/auth.ts`'s
`signIn`/`signInAsAdmin` for the browser ones), and both needed
`ORCHESTRA_SECURE_COOKIE=false` added to their platform-starting env - these
suites run over plain HTTP, where a `Secure` cookie is stored by nobody.
`e2e/src/auth.test.ts` is the new process-level journey (AC-A-103, AC-A-104,
AC-A-105): sign in as admin, seed two non-admin accounts through the new
`ORCHESTRA_SEED_ACCOUNTS` environment variable (mirrors
`ORCHESTRA_PLAN_FIXTURES` exactly - JSON, decoded in
`internal/infra/config`, never set in production), grant one of them a
single service, prove their next question is answered from it and not the
other, prove `/api/invoke` refuses the other operation the same way an
unknown one would, and prove a workspace either of them makes is invisible
to the other. `e2e/browser/auth.spec.ts` drives the same product's sign-in
→ chat → sign-out → sign-in-screen journey in headless Chromium (AC-A-107).
`make check` (not `-k`) is fully green, including `acceptance-e2e` and
`acceptance-browser` - the only two targets that had been allowed to fail
since Task 2. See `DECISIONS.md`, 2026-09-12 ("Auth Task 6: end to end") for
every design choice this task made and why, including a `pkill` mistake
made and corrected while verifying an operation id's casing.

**The nil-store auth bypass is closed.** `requireSession` used to treat a
nil `SessionUsers` store as "run every request as a fixed stub admin" -
harmless only because `cmd/api` always builds one, since
`internal/infra/config.Load` refuses to start without `ORCHESTRA_DB_PATH`.
Any other caller of `pkg/app.New` that forgot to set `Config.DBPath` got
that bypass for free: a wide-open admin backdoor on every route.
`pkg/app.New` now refuses to build a handler at all when `DBPath` is empty
(`app.ErrMissingDBPath`) - there is no legitimate use of this platform
without a database, workspaces and accounts both need one - so the nil case
`requireSession` used to handle can no longer be reached, and that branch
(and the `newStubAdmin` helper it used) was deleted outright rather than
left as unreachable dead code.

There is a design decision worth its own note: the shared acceptance
sign-in helper (`newTestApp`, `services/platform/acceptance/helpers_test.go`)
lives in the `acceptance` package rather than duplicated per file - see
`DECISIONS.md`, 2026-09-12 ("Closing the nil-store auth bypass").

**`docs/plans/context.md`, all five tasks, are done - multi-turn context is
complete.** `web/src/features/conversation/model/conversationStore.ts` holds
conversations above `Conversation` itself, keyed - `"chat"`, and one per
workspace id - with a control that ends one and empties its turns
(AC-M-107, AC-M-108). `usecase.Turn{Question, Kind, Service, OperationID,
Args}` and `PlanRequest.turns` (contract-first: `openapi.yaml`, `make
generate`) carry a conversation from the browser to
`Orchestrator.Plan`, truncated to the most recent `ORCHESTRA_CONTEXT_TURNS`
turns, oldest dropped first, before either planner ever sees it
(AC-M-104, AC-M-105); both `toolcall` and `jsonmode` render the same turns
into the prompt after the catalogue (M3), never a row of any answer
(AC-M-102, AC-M-103, AC-M-106); `web/src/features/conversation`'s
`toContextTurns` builds them from what the browser already has - a
question's text and the following answer's `source`/`target` - so no new
state is invented client-side either.

Task 4 closed it end to end, and made one decision this task's plan asked
for explicitly: `internal/adapter/planner/stub.Key` now carries a third
field, `Turns` (canonicalised by the new `stub.TurnsKey`, order-dependent,
`service/operationId` pairs joined by `|`), alongside `Query` and `Answers`.
The stub still performs no reasoning over turns - it stays a pure table
lookup, so `make check` still never calls a real LLM - but a fixture table
can now key the very same `Query` on the conversation that came before it,
which is what lets `e2e/src/context.test.ts` and `e2e/browser/context.spec.ts`
fix a follow-up question's answer without a model at all: ask about
inventory, then ask the same follow-up wording after an attendance turn
instead, and get attendance back - proof the mechanism carries the turns
through, not that any particular fixture happened to match. `pkg/app.PlanFixture`
and `internal/infra/config.PlanFixture` both grew a `Turns []TurnFixture`
field (`{service, operationId}`) for exactly this, decoded from
`ORCHESTRA_PLAN_FIXTURES` the same way `Answers` already was.

Whether a real model actually carries a follow-up's context is a different
question, answered by hand rather than by any test: five `POST /api/plan`
calls against the running `qwen3.5-9b-q8` (port 8080, `make dev-platform`),
each carrying one earlier inventory turn and a follow-up ("他にはある？")
phrased to give no service clue at all, went 5/5 to the inventory service -
four resolved outright, one came back `kind: "ask"` for a missing `status`
value but still on the right operation. Recorded in `DECISIONS.md`,
2026-09-12 ("qwen3.5-9b-q8 carries a follow-up's context, measured by
hand"), per this task's own instruction that a model unable to do this
would be a finding, not a failure.

D8 (`docs/specs/orchestration.md`) now says explicitly what
`docs/specs/context.md` argued it always meant: the API result never goes
back to the LLM and one request is still one LLM call, and what goes back
on the _next_ request is the earlier question and the decision made for
it, never a row of the answer - rendering that conversation into the same
one call's prompt is not a second call.

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

**Asking for a panel (`docs/specs/proposing.md`), Task 0 of
`docs/plans/proposing.md`.** A fifth built-in tool, `propose_panel(service,
operationId, args, component?, chart?, transform?, title?)`, alongside
`ask_user` and `list_capabilities` (`usecase.ProposePanelTool`,
`usecase.ToolsFor`): the model answers with a panel it composed rather than
doing anything (N1). `usecase.Orchestrator.propose` maps a
`DecisionProposal` onto `kind: "proposal"` - a fifth `PlanResult` kind
beside `result`/`form`/`ask`/`none`, carrying a `panel` (the new
`ProposedPanel` contract schema, reusing `View` for its chart axes and
transform rather than inventing a second shape) - and fills in whatever the
model left zero-valued from the catalogue: the component from
`domain.Render`, the chart axes from `endpoint.ChartHint` when the contract
declares them, the title from `DisplayNameOr`. A proposal naming an
operation outside the caller's own narrowed catalogue fails with the same
`ErrEndpointNotFound` every other unknown-or-forbidden operation already
does (AC-N-105). Both planners reach `DecisionProposal`:
`toolcall.Planner` maps the `propose_panel` tool call directly;
`jsonmode.Planner` adds a fifth `kind: "propose_panel"` JSON shape,
validated the same way a `call` answer's args already are. The stub planner
needed no change - a table lookup already returns whichever `Decision` its
fixture names, `DecisionProposal` included (AC-N-106). See `DECISIONS.md`,
2026-09-13, "propose_panel: a fifth `kind`, filled in from the catalogue".

**Task 1 of `docs/plans/proposing.md`: the person sees the proposal and
places it.** A `proposal` answer turn draws `features/panels`' own form -
`AddPanelForm`, the same component `AddPanelControl` (create) and
`EditPanelControl` (edit) already open - a third time, over what the
platform filled in rather than over nothing or an existing panel
(`docs/specs/dashboard.md` P12, P7; `docs/specs/proposing.md` section 5).
The new `ProposalControl` (`features/panels/ui`) matches the proposal's
`service`/`operationId` against the loaded catalogue exactly as
`EditPanelDialogContent` does for an existing panel, then hands
`AddPanelForm` a new `usePanelProposal` hook - `usePanelEditor`'s own shape,
seeded from the proposal instead of a saved panel, posting through
`POST /api/workspaces/{id}/panels` (`addPanel`) instead of `PATCH`. The
operation is stated rather than offered (`operationLocked`, P13's own
reasoning), and the save control reads "配置" - placing a proposal, not
merely editing a panel already on the workspace - via a new `saveLabel`
override on `AddPanelForm`. Editing any field before pressing it changes
exactly what gets posted, so a proposal edited before being placed is
placed as edited (AC-N-103); once placed, the form is replaced by a plain
confirmation so the same proposal cannot be placed twice.

**Only a workspace's own conversation offers one (N4, AC-N-104), decided in
`widgets/conversation`'s `ConversationPanel`, not in `TurnList`.**
`TurnList`/`Conversation` (`features/conversation`) gained a `renderProposal`
slot, the same pattern `renderSaveControl` already established for the same
reason: the form comes from `features/panels`, a sibling feature
`features/conversation` cannot import (`make guard-fsd`). `TurnList` itself
carries no notion of a workspace at all - it only ever asks "was I handed
something to draw a proposal with", and draws nothing (not a broken
control, not the generic "missing information" fallback) when it was not.
`ConversationPanel` is where a workspace's presence or absence is actually
known - `pages/chat` calls it with no id, `pages/workspace` always with one

- so it supplies `renderProposal` (wired to the new `ProposalControl`) only
  when its own `defaultWorkspaceId` is defined; `pages/chat` therefore never
  offers one, without `Conversation`/`TurnList` needing to know why.
  `WorkspacePage` passes `onPanelPlaced={addedPanels.add}` - the same
  callback `AddPanelControl` already uses - so a panel placed from the chat
  appears in the grid immediately, the same as one added through the
  button.

**`usePanelSave` (`features/panels/model/panelSubmit.ts`) is a new hook
shared by `usePanelBuilder`, `usePanelEditor` and `usePanelProposal`.**
All three built (checking what's missing, assembling the request, posting
it, calling back on success) the same way past the one call each of them
makes; pulling that logic into `PanelPayload`/`buildPayload`/`usePanelSave`
(plus `buildAddPanelRequest`, shared by the two hooks that `POST` rather
than `PATCH`) is what keeps `usePanelProposal` - structurally almost
identical to `usePanelEditor` otherwise - from tripping
`make guard-duplication` (`harness/quality/duplication.txt`, `minNodes 60`).
Verified: `make guard-duplication` passes.

Tests: `ProposalControl.test.tsx` (the form seeded from the proposal, the
operation stated not offered, placing it as filled in or as edited
(AC-N-103), and the catalogue's own load error); `ConversationProposal.test.tsx`
(split out of `Conversation.test.tsx`, at its own `max-lines` budget) for
the slot itself and AC-N-104's "no renderProposal, no proposal, no throw";
`ConversationPanel.test.tsx` and `WorkspacePage.test.tsx` gained one test
each for the composed, end-to-end shape - asking from a workspace's chat,
pressing 配置, and the panel showing in the grid.

`make guard-a11y`/`make guard-layout` were run against the built product
with a proposal turn on screen (a workspace with a panel already in it, per
the task's own instruction) - both green with no changes needed: the new
furniture is `AddPanelForm`'s existing JSX subtree, already measured
wherever `AddPanelControl`/`EditPanelControl` open it, drawn inline in a
`Paper` the same way a `form`/`ask` answer turn already is - no new control,
no new colour, nothing this pair had not already checked.

Task 2 (the end-to-end journey through `e2e/`, and measuring what offering
`propose_panel` in every request costs every other question) is not
started.

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
names its operation for exactly this reason, DECISIONS.md 2026-09-11). If
that endpoint is **unsafe**, `ask` degrades straight to `kind: "form"`
before `decision.Param` is looked at at all - the same reasoning `call`
already applies to a `DecisionCall` against an unsafe endpoint (D8): an
unsafe operation is answered by a form in every case, so an `ask_user`
naming one is the model saying "I could not fill this in" about a call
that was always going to be handed over for filling in, and there is
nothing to disambiguate before running because nothing runs
(`DECISIONS.md`, 2026-09-12; `docs/specs/orchestration.md` D11 and section
8b - this is the fix for 「在庫を登録したい」 landing on
「ステータスを選んでください」 instead of the create form). Only for a
**safe** endpoint does `optionsForParam` still run: it searches that one
endpoint's parameters (no longer its request body - a request body only
ever appears on an unsafe endpoint's call, which now never reaches this
function) for one named `decision.Param` that declares an enum, and builds
the options from that schema's `Enum` and `EnumLabels`. Resolving `param`
within its own endpoint, not the whole catalogue, is what keeps a parameter
name such as `status` from colliding across services. A safe endpoint's
param the catalogue does not recognise as an enum at all - a plain string
parameter with no declared enum values - degrades to `kind: "form"` instead
of an error (`DECISIONS.md`, 2026-09-11: this used to be a 500,
`usecase.ErrUnknownParam`, which is now deleted). A `DecisionKind` the
switch does not recognise at all still returns `usecase.ErrNotImplemented`
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
`POST /api/workspaces/{id}/panels`, `PATCH`/`DELETE
/api/workspaces/{id}/panels/{panelId}` from Task 1 (`PATCH` added this task,
`docs/specs/dashboard.md` section 6a - see the Summary above), rejecting an operation
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

**`docs/plans/layout.md` Task 0 (a panel has a size).** `domain.Panel`
gains `Width`/`Height int`; `domain.PanelPatch` gains `Width`/`Height`/
`Position *int`, plain "was this named" pointers like `Title`/`Args`/
`Component` - no third state the way `View` needs one, since an integer has
no "explicitly clear it" request distinct from "leave it alone"
(`domain.PanelPatch`'s own doc comment; `DECISIONS.md`). The defaults - full
width (`DefaultPanelWidth` = 12) and one row (`DefaultPanelHeight` = 1,
`docs/specs/layout.md` section 3) - and the clamp bounds
(`MinPanelWidth`/`MaxPanelWidth` = 1/12, `MinPanelHeight` = 1, no upper
bound) live once as `domain` constants and `ClampPanelWidth`/
`ClampPanelHeight` functions, so `sqlite.Store` (reading a `NULL` column)
and `usecase.Workspaces.AddPanel`/`UpdatePanel` (deciding what a zero or
out-of-range value means) all read the same numbers. `AddPanel` treats a
zero `Width`/`Height` - what a caller that named neither leaves, and what
nothing could mean on purpose anyway - as "unset", defaults it, then clamps
unconditionally (a defaulted value is already in range; 40 clamps to 12, 0
and -1 clamp to 1); `UpdatePanel` clamps only a `Width`/`Height` a `PATCH`
actually named. `sqlite.Store.AddPanel` mirrors the same "0 means unset"
convention when writing (`marshalPanelSize`, storing `NULL` rather than a
literal 0) so a store-level test can prove AC-L-104 without going through
the usecase; `UpdatePanel`'s existing "only the named columns" `SET`
builder (`updatePanelSets`) already keeps a `position`-only `PATCH` from
touching any other panel's row, proven directly
(`TestStoreUpdatePanelPositionDoesNotRenumberOthers`, section 6). The
migration (`migrate.go`'s `ensurePanelsSizeColumns`) extends the existing
`pragma_table_info`-then-`ALTER TABLE` pattern (`ensurePanelsViewColumn`)
for two more nullable columns, `schema.sql` unchanged - the same
"never in `CREATE TABLE`, always through the migration function, even for a
fresh file" precedent `view` already set. Proven against a database file
built with the schema exactly as it existed after `view` but before this
slice (`TestNewOpensADatabaseFileWrittenBeforeSizeColumnsExisted`) - the
real shape of `~/.local/state/app-orchestra/workspaces.db` on this machine.
The contract (`openapi.yaml`): `Panel` gains required `width`/`height`;
`CreatePanelRequest` gains optional `width`/`height`; `UpdatePanelRequest`
gains optional `position`/`width`/`height`, plain integers (no
`x-go-type`/`nullable.Nullable` the way `view` needs). Frontend test
fixtures across `web/src/features/**` and `web/src/pages/workspace/**`
gained `width: 12, height: 1` to satisfy the now-required wire fields;
nothing draws differently yet - `WorkspacePage.tsx`'s grid and
`PanelCardShell.tsx`'s controls are Tasks 1-2.

**`docs/plans/layout.md` Task 1 (the workspace is a grid, read-only).**
`react-grid-layout` (`1.5.4`, pinned exactly like `@mui/x-charts` -
`DECISIONS.md`, 2026-09-12) plus `@types/react-grid-layout` (`1.3.6`, the
last real definitions before the package became a deprecated stub for the
library's own-typed `2.x` line). `pages/workspace/ui/WorkspaceGrid.tsx`
wraps `WidthProvider(GridLayout)` from the library, picking `12` or `1`
columns off a single `useMediaQuery("(min-width:600px)")` read (MUI's own
`sm`, named directly the way `PanelResult` already does, not through
`useTheme`, for `import/max-dependencies`) rather than the library's own
multi-breakpoint `Responsive` component - `docs/specs/layout.md` section 5
asks for exactly one breakpoint. `pages/workspace/model/buildPanelLayout.ts`
is the pure function that turns `panels` + `columns` into a `Layout[]`: sorts
by `position` first (never the order the API returned them in), then packs
left to right, clamping each panel's `width` to `columns` and wrapping to a
new row when the next one would not fit - passing `columns: 1` alone gives
the narrow breakpoint every panel spanning the single column (AC-L-105),
with no separate branch. Every item comes out `static: true`, and the grid
itself is `isDraggable={false}`/`isResizable={false}` - Task 1 is read-only
by both belt and suspenders; Task 2 turns both on and adds the keyboard
half `docs/specs/layout.md` section 4 owes for taking the dependency.
`react-grid-layout/css/styles.css` (a real bundler import, not a CDN
request) paints no colour of its own - checked against the built CSS
directly: transitions, an absolute-position rule, a translucent red drag
placeholder and grey resize-handle corner arrows, all inert while
`isDraggable`/`isResizable` are false, so `make guard-layout`'s contrast
checks (both colour schemes) have nothing new to see from it yet; that
changes in Task 2 once handles actually render.

Two more things this task's own AC-L-106 forced: `PanelCardShell.tsx`
(`entities/workspace`) now fills its box (`height: "100%"`, a flex column,
`CardContent` as the `flexGrow` scroll region) instead of sizing to its
content, because a `react-grid-layout` item is already the exact pixel box
`width`/`height` say it should be and the old shell left the rest of a
short panel's cell blank while a tall one would have spilled past it
(`docs/specs/layout.md` section 7's "scrolls inside its own card" needed
this scroll region to exist at all). And `PanelResult.tsx`'s chart size,
which used to come from one viewport media query (`WIDE_BREAKPOINT_QUERY`,
560×320 or 260×200 - unable to tell a `width: 12` panel from a `width: 6`
one on the same screen), now comes from a new
`shared/lib/useElementSize.ts` hook that measures the chart's own rendered
box with `ResizeObserver` and feeds that straight to `ResultChart`
(`docs/plans/dashboard.md` Task 5 already made it take its size from its
caller). Confirmed by hand against a running platform, not merely built:
two chart panels of the same catalogue operation, `width: 12` and `width:
6`, ended up measured at 896×626 and 416×626 respectively, and the
`<svg>` each `BarChart` drew matched those numbers exactly. That check
first failed - both panels drew the fixed 320×240 default - because
`useElementSize`'s first version used a plain `useRef` with an effect run
once (`[]` deps): `PanelResult` does not render the measured `Box` until a
result has loaded, so the ref the effect read was still `null` at the time
it ran and the observer was never created. The fix was a callback ref
backed by state (`setNode` on attach) so the effect's own dependency is
"the node changed," not "the component mounted once."

**`docs/plans/layout.md` Task 2 (arrange it, with a mouse or without one).**
`WorkspaceGrid.tsx` turns `isDraggable`/`isResizable` on for the wide
breakpoint (still `static`, both off, on the narrow one - AC-L-105 has
nothing to arrange there). `pages/workspace/model/arrangement.ts` is the
pure core, all four functions unit-tested directly: `positionChanges`
derives every panel's `position` from `react-grid-layout`'s own final
`x`/`y` and returns only the ones whose value actually differs from what
is on screen (section 6 - no renumbering the workspace to move one);
`sizeChange` reads one resized item's own `w`/`h`; `moveChanges`/
`resizeChange` are the keyboard half's identical two operations, swapping
`position` with a neighbour or nudging `width`/`height` by one. All four
are used by both halves, so a drag-driven swap and a keyboard-driven one
`PATCH` the same shape of change. `useArrangement.ts` layers the result on
the loaded panels as a per-id local override (the same split
`PanelResult`'s own `current` makes for a saved edit) and fires
`patchPanel`; nothing is written for a drag still in flight, since only
`onDragStop`/`onResizeStop` are wired.

**The keyboard shape.** Every panel's header (`PanelActions.tsx`) carries
one `IconButton` ("...をキーボードで並べ替え・サイズ変更", with a
`Tooltip` naming the keys), reachable by tabbing to it: arrow keys move the
panel one step earlier/later in `position` order, `Shift`+arrow resizes by
one column or row. Tested by keyboard alone - `WorkspaceGrid.test.tsx`
tabs to the button and drives it with `userEvent.keyboard`, no pointer
event in that test at all (AC-L-103).

**Two real bugs the harness change (Step 4) found, beyond the stylesheet
(still clean in both colour schemes - see `DECISIONS.md`).**
`WorkspaceGrid` no longer uses `react-grid-layout`'s own `WidthProvider`:
its unmeasured first-render guess, animated into the real width over the
stylesheet's own 200ms transition, was wide enough on the narrow
breakpoint to fail `guard-layout`'s sideways-scroll check every time once a
real panel existed to measure. Replaced with `useElementSize` (the same
hook `PanelResult` already uses) feeding `width` straight to a plain
`GridLayout`, plus `workspaceGrid.css` narrowing the item's own transition
to `transform` only. Separately, `isDraggable` going live meant
`mousedown` on any panel button (refresh, edit, arrange, pagination)
started a drag before its `click` fired - found by `e2e/browser/dashboard.spec.ts`'s
edit journey, whose "編集" click stopped opening the dialog. Fixed with
`draggableCancel="button, a, input, select, textarea"`. Full account,
including why `measureBeforeMount` was tried and reverted (it hangs every
unit test in `happy-dom`, which never fires the measurement it waits for),
in `DECISIONS.md`.

**The harness change itself.** `harness/quality/browser/playwright.config.ts`
now also runs the inventory dummy service and names it in
`ORCHESTRA_SERVICES`, because `AddPanel` refuses any operation the
catalogue does not expose - a panel cannot be seeded at all otherwise, and
faking one directly in the database would be evidence of a state the
product can never actually reach. `screens.ts`'s "a workspace" is now "a
workspace with a panel in it," carrying one real `ListInventoryItems`
table panel through `createPanel` (mirroring `createWorkspace`'s own
page-context `fetch`).

**`docs/plans/layout.md` Task 3 (end to end) - the subproject closes.**
`e2e/src/layout.test.ts`: three panels created over real HTTP against the
built platform binary (no width/height/position named, so all three read
back at Task 0's own defaults - full width, one row, `position: 0`), three
`PATCH`es give two of them a later `position` and the third a wider,
taller, first row, and a fresh `GET` on the same process proves the
geometry landed exactly there - `second`'s own `PATCH` named only
`position`, so its width/height read back as the untouched default
(AC-L-104) rather than anything the test itself set. `e2e/browser/layout.spec.ts`
drives the same thing through real pointer events in headless Chromium:
sign in, add three panels (every builder-made panel starts full width, one
row - the builder has no width/height field), narrow two of them by
dragging their own resize handle, grow the third's height by the same
handle, drag it to the front by its own header, reload, and read the
result back both through the DOM (`.MuiCardHeader-title` order) and
through `/api/workspaces/{id}` (via `page.request`, which shares the
browser's own session cookie - no bare `fetch` from Node, and nothing runs
inside the page itself) - then narrows the viewport to 375px and confirms
the same order survives with no panel wider than the viewport (AC-L-105).
Geometry helpers live in `e2e/browser/helpers/layout.ts` (kept out of the
spec file for `max-lines`).

**Two real things this drag exposed, neither a product bug.** First, a
newly created panel is not just full-width by default - it is
`position: 0` for every panel, the same zero value that predates this
whole subproject (`docs/specs/workspaces.md` W5's own column, "added...and
excluded the editing"): `AddPanel` never assigns a fresh, ascending
position, so three panels made in a row all start at `position: 0` and
draw in creation order only because `Array.prototype.toSorted` is stable.
A workspace only looks arranged once a drag, a resize, or (in the e2e
test) an explicit `PATCH` actually arranges it - exactly this plan's own
closing line. Second, `.react-grid-item.cssTransforms`'s 200ms transition
(`workspaceGrid.css`) means a resize that moves another panel leaves that
panel's own screen position mid-slide for up to 200ms - a bounding box
read immediately afterward (or a scroll triggered by a huge tall panel's
own resize handle, since `boundingBox()` does not scroll and a `y` that
lands off-screen cannot be clicked) reads a stale position. The spec
settles 300ms after every drag/resize and sizes its own viewport tall
enough that the whole arrangement never needs to scroll, rather than
fight either one mid-gesture.

**AC-L-103's own two claims, confirmed rather than re-tested.** "Moving a
panel and resizing it are both operable without a pointer" is
`web/src/pages/workspace/ui/WorkspaceGrid.test.tsx`'s own keyboard-only
test (Task 2): focus is moved with `Tab`, every action after that is
`userEvent.keyboard`, no pointer event in the test at all - re-read here
rather than duplicated. "`make guard-a11y` passes on a workspace that has
panels in it" was confirmed by running `make guard-a11y` (and
`make guard-layout`) directly: `harness/quality/browser/screens.ts`'s "a
workspace with a panel in it" screen (Task 2 Step 4) is in `SCREENS`, and
both gates pass green against it.

**`docs/specs/layout.md` section 5a (a panel's height on the narrow
breakpoint is a second number).** `narrowHeight` joins `width`/`height` on
`Panel`, `CreatePanelRequest` and `UpdatePanelRequest` - optional and
nullable on the wire (`openapi.yaml`), unlike `width`/`height`, which
always resolve to a default: `domain.Panel.NarrowHeight` is a `*int`, nil
meaning "no narrow height of its own", and stays nil rather than being
defaulted the way `AddPanel` defaults `Width`/`Height` (AC-L-108).
`ensurePanelsSizeColumns` (`migrate.go`) is extended with a third column,
`narrow_height`, added the same idempotent way as `width`/`height` -
`TestNewOpensADatabaseFileWrittenBeforeSizeColumnsExisted` (renamed in
spirit, not in name) now also proves a pre-existing file's panel reads
back with a nil `NarrowHeight`. `usecase.Workspaces.AddPanel`/`UpdatePanel`
clamp only a `NarrowHeight` the caller actually sent, through the same
`domain.ClampPanelHeight` `Height` uses (extracted into `clampPatchSize` to
keep `UpdatePanel` under golangci-lint's `gocyclo` limit).

`web/src/pages/workspace/model/buildPanelLayout.ts`'s `resolvedHeight`
reads `narrowHeight` only on the narrow breakpoint (`interactive: false`,
the same flag `WorkspaceGrid` already passes as `wide`), falling back to
`height` exactly the way a missing `height` itself falls back to `1` -
`finiteOr`'s own rule, applied twice. `arrangement.ts` gained
`narrowResizeChange` (starts from `height` when `narrowHeight` is unset,
the same fallback) and `useArrangement` a `narrowResizeBy`, PATCHing only
`{ narrowHeight }` - never `width` or `height`.

**Order was kept on the narrow breakpoint; width and pointer resizing were
not.** Section 5/5a/L7 say the narrow breakpoint's grid is static and has
"no order that differs" - but that reads as "no _separate_ narrow order",
not "reordering is meaningless there": a stack of one column still has an
order, moving a panel changes what a person sees on their phone exactly as
it does on a desktop, and `position` is already a single field shared
across both breakpoints (L7). So `WorkspaceGrid` now wires `onMove` (the
keyboard move control) on both breakpoints unconditionally, and only
`onResize` differs: on narrow it is a handler that reads only
`deltaHeight` and calls `narrowResizeBy`, so Shift+Left/Right (which would
change `width`, a field the narrow breakpoint has nothing to set, L7) is a
no-op there rather than reaching for a column that does not exist.
`isDraggable`/`isResizable` on `GridLayout` itself stay `wide`-only -
pointer dragging and resizing-by-handle are still exactly what section 5
excludes; only the keyboard control's own reach changed.

Tests: `buildPanelLayout.test.ts` (AC-L-107/108, null and non-finite
`narrowHeight`), `arrangement.test.ts` (`narrowResizeChange`),
`useArrangement.test.ts` (`narrowResizeBy` PATCHes only `narrowHeight`),
`WorkspaceGrid.test.tsx` (a narrow-breakpoint keyboard test - `matchMedia`
mocked to `matches: false` - moves by `ArrowRight` and resizes only
`narrowHeight` by `Shift+ArrowDown`), and on the Go side unit tests in
`usecase/workspaces_test.go`, `adapter/handler/workspace_test.go` and the
repository layer (`store_test.go`, extended;
`store_panelsize_test.go`, new - `store_test.go` passed `guard-filelen`'s
1000-line limit once this slice's tests were added, so panel
view/size/migration tests were split out into their own file, the same
kind of split `permissions_test.go`/`sessions_test.go`/`users_test.go`
already are for their own sources).

**Verified live**, not just by test: a phone-width (375px) panel's
`narrowHeight` was set to 4 via the keyboard control's Shift+ArrowDown
(three nudges from the height-1 starting point `resolvedHeight`'s own
fallback draws), read back through `/api/workspaces/{id}` as
`{ height: 1, narrowHeight: 4 }`, and the same workspace at 1280px drew
the panel at its original one row tall - the desktop untouched by the
phone-only edit, screenshotted both ways.

**Storage (`docs/specs/storage.md`), the eleventh subproject: one `*sql.DB`
per database file, not four.** `internal/adapter/repository/sqlite`'s
`Store`, `Users`, `Sessions` and `Permissions` used to each open their own
connection to the same `ORCHESTRA_DB_PATH` file - `pkg/app.build` now opens
it once (`sqlitestore.Open`) and hands that one `*sql.DB` to each store's
`NewFromDB` constructor, alongside WAL (`_journal_mode=WAL`) and a
five-second busy timeout (`_busy_timeout=5000`), both DSN parameters
modernc.org/sqlite validates itself. A new `internal/infra/httpserver/logging.go`
middleware logs any 500 - from any handler, `requireSession`'s own included -
with the body that named its cause, through `slog.Default()`
(`cmd/api/main.go` now calls `slog.SetDefault`). Forty concurrent requests
that each resolve a session and write a row failed 8-19 of 40 times before
this change and ten of ten times after
(`services/platform/acceptance/storage_test.go`); `make guard-layout` - the
symptom that motivated the spec - passed ten consecutive runs afterward.
See `DECISIONS.md`, 2026-09-14, correcting 2026-09-13's "the browser
suite's flake is load, measured".

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

Authentication/authorisation and multi-turn conversational context are both
done (`docs/plans/auth.md`, `docs/plans/context.md`, see the Summary above).
What remains is a genre/domain layer above individual services, and the
move to TypeScript 7 once `openapi-typescript` supports it (see "Known gaps
in the harness" below).

**D15 - asking for everything is a thing the model must say - tried and
withdrawn.** An optional enum parameter on a safe endpoint was offered to
the model as required, with a synthetic `__all__` value appended, to close
a defect the eval corpus measures directly: on `qwen3.5-9b-q8`,
`no-enum-value` (破損した在庫はある？, a filter word matching no enum value)
silently drops the filter and returns every row. Measured three times -
plain, then with an instruction added to `__all__`'s description telling
the model when it is and is not the right answer:

```
                       before D15   D15      D15 + instruction
no-enum-value reject     18/30      20/30    16/30    (n=30 noise band measured at 16-19)
no-enum-value accept     10/30       5/30    10/30    (observed range 7-11)
attendance    reject      9/10       5/10     8/10    (n=10, band 0.40 wide - not decisive)
every other case         10/10      10/10    10/10    (no side effects, throughout)
```

The reject rate never moved outside its own noise band, in either version;
only the accept rate moved, and only because `__all__` competes with
`ask_user` as a way to answer a call the model was already going to make -
adding a value to the operation's own enum made calling that operation
easier, not harder, which made `ask_user` rarer, not more likely. **D15 is
reverted** (`DECISIONS.md`, 2026-09-12, final entry): the synthetic value,
its stripping, and both planners' handling of it are gone. The defect
itself is open again - see TODO.md - and a future attempt has to change the
competition between `ask_user` and the operation's own tool, not the enum.

One piece survives, independent of D15: `jsonmode.renderParam`
(`internal/adapter/planner/jsonmode/planner.go`) now renders a parameter's
own contract `Description` in the rendered catalogue text, which it never
did before this work found the gap.

## Known gaps in the harness

- **TypeScript is held at 6.0.3 by a dependency, not by choice.**
  `openapi-typescript` builds its output with the TypeScript Compiler API, which
  TypeScript 7's native implementation does not provide, so `make generate` fails
  under 7. Everything else - including type-aware Oxlint - passed under 7. Go and
  pnpm are current (`DECISIONS.md`, 2026-09-10).
- **`harness/quality/file-length.txt` does not exempt the per-service generated
  `.d.ts` files, only `schema.d.ts`.** `oxfmt`'s and `oxlint`'s own policies
  (`harness/quality/oxfmt/policy.ts`, `harness/quality/oxlint/policy.ts`) already
  exempt the whole `**/src/shared/api/gen/**` directory, and `DECISIONS.md`'s
  2026-09-11 orval entry already assumes `guard-filelen` does too - but the glob
  in `file-length.txt` was never updated when `openapi-typescript` moved from one
  shared `schema.d.ts` to one file per service (`attendance.d.ts`,
  `inventory.d.ts`, `platform.d.ts`). `web/src/shared/api/gen/platform.d.ts` was
  already at 971 of 1000 lines before Dashboard Task 4 (`docs/plans/dashboard.md`)
  added `GET /api/catalog` and `CatalogEntry`, which pushed it to 1030 -
  `guard-filelen` now fails on a file nobody hand-edits, for a reason
  `file-length.txt`'s own header says should not apply to generated code.
  `AGENTS.md` rule 2 forbids reconfiguring anything under `harness/quality/`, so
  this was left as-is rather than patched: whoever picks this up should add
  `**/src/shared/api/gen/**` (or one `exclude *.d.ts` line scoped the same way
  `*.gen.go` is) to `file-length.txt`, matching the two sibling policies. Task 4
  is otherwise fully green - see `TODO.md`.

## Picking something to put on a dashboard (docs/specs/picking.md), a ninth subproject

**The operation picker matches what a person types, not only what it
displays** (K1, section 3; AC-K-101). `OperationPicker.tsx`
(`features/panels/ui`) builds one searchable string per catalogue entry -
its display name, its service's display name, its operation id, its
summary, and the titles of the fields it returns (`fieldOptionsFor`, the
same rule `TransformFields`' own dropdowns already used) - and hands it to
MUI's `createFilterOptions({ stringify })`. That function's own defaults
(`ignoreCase: true`, `matchFrom: "any"`, i.e. plain `String.includes`) are
already exactly the plain substring, case-insensitive match section 3 asks
for, so this is `Autocomplete`'s own `filterOptions` prop, not a second
filter written over the array upstream. Verified live against the running
inventory/attendance contracts (not only the unit fixtures): typing 数量
into a workspace's picker found `ListInventoryItems`, `CreateInventoryItem`
and `GetInventoryItem` - every operation whose response schema carries a
field titled 数量 - none of which have 数量 in their own display name.

**An option shows its name and its summary underneath** (K2, section 4;
AC-K-102). `renderOption` draws two lines - `variant="body2"` for the name,
`variant="caption" color="textSecondary"` for the summary - inside the
option `<li>`'s own padding, so the row grows taller rather than the row's
own hit target shrinking. Checked live in both colour schemes rather than
assumed: the option is ~54px tall (well past the 44px floor
`layout.spec.ts` enforces elsewhere), and the summary's colour measures
`rgba(0, 0, 0, 0.6)` on light and MUI's default `text.secondary` on dark -
both comfortably past 4.5:1 on their own default backgrounds, since this
app's theme (`app/theme.ts`) does not override either palette. The summary
stays untranslated English, unchanged, as section 4 requires - the
contract's wart, not this screen's.

**The transform was already a switch alone** (K3, section 5; AC-K-103).
Reading `TransformFields.tsx` and `AddPanelForm.tsx` before touching
either: `usePanelFields`'s `transformEnabled` already starts `false`
(`emptyFieldValues`, `panelFieldValues.ts`), and `TransformFields` already
renders its own three fields only inside `{enabled && (...)}`. Section 5's
claim that "the transform is the one that does not" start collapsed did not
hold against this codebase's own state - a real gap between the spec's
narrative and the code it describes, not a defect this task introduced or
had to fix (`DECISIONS.md`, 2026-09-13). Left unchanged; a dedicated test
(`AddPanelControl.test.tsx`, "shows only the picker before an operation is
picked...") now pins AC-K-103 explicitly, which nothing did before this.

**The genre layer (K4) stays deferred**, unchanged from section 2/6: every
exposed operation in `services/inventory` carries `items` and every one in
`services/attendance` carries `records`, so grouping by tag would still
produce exactly one group per service - what `OperationPicker`'s own
`groupBy` already does off `serviceDisplayName`.

Three tests already querying an option by its visible text
(`AddPanelControl.test.tsx`, `AddPanelControlArgs.test.tsx`,
`AddPanelControlTransform.test.tsx`, `WorkspacePage.test.tsx`) moved from
`findByText(summary)` to `findByRole("option", { name: new RegExp(summary,
"u") })`: these fixtures set `summary` equal to `displayName`, so an
option's now-two-line text duplicates it and a plain text query throws on
"more than one match" rather than picking the wrong one. A query fix, not a
behaviour change - `docs/specs/dashboard.md` section 10's own criteria
(AC-P-101 through AC-P-112) still pass unedited otherwise (AC-K-104).

## The eval suite (docs/specs/eval.md), a fifth subproject about the tests

`e2e/eval/` measures the real planner as a rate against a committed
baseline - never inside `make check` (AC-E-202, verified: `docker logs
llama-swap`'s `POST /v1/chat/completions` count is unchanged across a full
`make check` run). `make eval` starts both dummy services and the platform
from their built binaries with `ORCHESTRA_LLM_BASE_URL`/`ORCHESTRA_LLM_MODEL`
set to a real local model (`ORCHESTRA_EVAL_MODEL`, default `qwen3.5-9b-q8`),
signs in as admin, and runs every case in `e2e/eval/cases.ts`
(`ORCHESTRA_EVAL_N`, default 10, times each), matching each response against
the case's `accept`/`reject` outcomes (`e2e/eval/match.ts`) and printing one
line per case (`e2e/eval/report.ts`). The corpus covers all seven kinds
AC-E-205 asks for: a label-named filter, an enum-less filter (the one this
suite exists for - qwen3.5-9b-q8 was measured, by hand, to sometimes drop
the filter silently), "everything", a create, an unanswerable question, a
capability question, and a follow-up naming no service (fixed `turns`, not a
live first call, so only the follow-up itself is under measurement). `make
eval` exits non-zero when a case's accept rate falls more than
`ORCHESTRA_EVAL_TOLERANCE` (default 0.3) below `e2e/eval/baseline.json`;
`make eval-accept` is the only thing that rewrites that file (AC-E-204).
Both are new `Makefile` targets, neither a dependency of `check` or
`acceptance`; see `DECISIONS.md`, 2026-09-12, for the location, N/tolerance
and operationId-casing decisions.
