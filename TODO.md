# TODO.md — prioritised work

_Keep three lists. Move items, do not duplicate them._

## In progress

Nothing right now. `docs/plans/dashboard.md` (see "Done" below) was the
last plan in flight; `docs/plans/orchestration.md`,
`docs/plans/workspaces.md`, `docs/plans/auth.md` and `docs/plans/context.md`
were already closed. Every task in every plan under `docs/plans/` is done,
every acceptance criterion in `docs/specs/*.md` section 8/9/10 has a test
that runs in CI, and `make check` (not `-k`) is fully green.

What remains of `docs/requirements.md` FR-F is only its layout half -
arranging panels: dragging, resizing, persisting a layout (FR-F-4),
explicitly deferred by `docs/specs/dashboard.md` section 9. No plan exists
for it yet; it would be a new `docs/plans/*.md`, not a reopening of
`dashboard.md`.

## Next

Everything remaining sits outside all four subprojects above:

1. A genre/domain layer above individual services - grouping services by
   what they are for, rather than listing every one flat.
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

## Done

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
