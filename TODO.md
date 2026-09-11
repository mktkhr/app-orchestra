# TODO.md — prioritised work

_Keep three lists. Move items, do not duplicate them._

## In progress

1. The first vertical slice, `docs/plans/orchestration.md`, is complete: all
   seventeen tasks, `make check` fully green, including `acceptance-e2e`,
   `acceptance-browser`, `guard-browser` and (Task 11) the JSON planner.
   Nothing from the plan remains; the slice is done and what follows is
   whatever comes after it (not yet planned).

## Next

1. `harness/guard/exposed-ops.sh` is the natural place to also require
   `title` on every property an exposed operation's schema describes, since
   only an exposed operation's fields are ever shown on screen. Deliberately
   left for a separate decision (`DECISIONS.md`, 2026-09-11).
1. No local model under ~20B parameters was observed to reliably choose
   `ask_user` through the tool-calling planner (`DECISIONS.md`, 2026-09-11).
   Not yet re-measured against the JSON planner's `{"kind": "ask", ...}`
   shape, which may be an easier target for a smaller model than a tool call
   is - none of the questions exercised live for Task 11 were ambiguous
   enough to reach it.
1. Move to TypeScript 7 once `openapi-typescript` supports it. Everything else
   in the repository already passes under 7; only code generation does not.
   orval was measured as a replacement and rejected - it runs under TypeScript 7
   but emits the wrong shape for this product (`DECISIONS.md`, 2026-09-11).

## Done

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
  fixed in `Conversation.test.tsx` regardless.
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
