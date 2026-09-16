# Staging: pick first, fill second — implementation plan

> **For the implementer:** Tasks 1 and 2 touch disjoint files and can run in
> parallel; Task 3 follows both; Task 4 is the measurement and is run by the
> session owner, not delegated. Each task ends green and is committed on its
> own. Steps use `- [ ]`.

**Goal:** with `ORCHESTRA_PLANNER_STAGES=2` the platform plans in two model
calls - a pick in the measured picker's format, then a fill against the
picked operation only - and `make eval-shortlist STAGES=2` measures it
beside the single call's 67 / 71 and the picker's 83.

**Architecture:** a `usecase.Picker` port (question + shortlist → one
operation id or a built-in's name, plus an ambiguity flag) implemented by
`internal/adapter/planner/pick` over `chat.Client`. `Orchestrator.Plan`
gains one branch after narrowing that calls the picker and dispatches: an
operation id goes down the existing `planPreferred` path (with `ask_user`
offered when the operation has an enum parameter), `list_capabilities`
answers from the catalogue, `none` is `DecisionNone`, `propose_panel` falls
back to the single call. `STAGES=1` (default) is byte-identical to today.

**Spec:** `docs/specs/staging.md`. Acceptance criteria: its section 9.

## Global constraints

`docs/plans/shortlisting.md`'s and `docs/plans/wording.md`'s Global
constraints hold. Here in particular:

- **The pick prompt is the TypeScript one, byte for byte** (S2, AC-S-103).
  The Go constant is asserted against `e2e/narrowing/pick/client.ts` by a
  test that reads that file and checks the Go string appears in it verbatim
  (the template literal holds the exact text). The TypeScript side gets the
  mirror test: it reads the Go file. Neither copies by hand into a test;
  each reads the other's source.
- **`STAGES=1` unchanged.** No request the platform sends under `1` differs
  from today's; the existing planner tests are the proof and none is
  edited to pass.
- **`make check` calls no model.** The picker is tested through
  `chat.Client`'s fake transport, as `toolcall` is. Verify
  `docker logs llama-swap 2>&1 | grep -c 'POST /v1/'` before and after.
- Layers: `usecase` gains a port and data types only; the adapter renders
  and parses. No `net/http`, no `encoding/json` in `usecase`.
- Coverage 90%; `guard-arch`; files under 1000 lines (`orchestrator.go` is
  at 942 - new code goes in `orchestrator_staging.go`).
- `git commit -- <explicit paths>`; subjects ≤ 72 characters; no push.

---

### Task 1: the `Picker` port and the `pick` adapter

**Files:**

- Create: `services/platform/internal/usecase/picker.go`,
  `services/platform/internal/adapter/planner/pick/picker.go`,
  `.../pick/prompt.go`, `.../pick/parse.go`, tests beside each
- Test (cross-language): `.../pick/prompt_test.go` reads
  `e2e/narrowing/pick/client.ts`; `e2e/narrowing/pick/client-prompt.test.ts`
  gains a case that reads `.../pick/prompt.go`

**Produces:**

```go
package usecase

// PickKind is what the picker named: an operation of the shortlist, or one
// of the three fixed lines the pick offers after it (docs/specs/staging.md, S3).
type PickKind string

const (
	PickOperation        PickKind = "operation"
	PickListCapabilities PickKind = "list_capabilities"
	PickProposePanel     PickKind = "propose_panel"
	PickNone             PickKind = "none"
)

type Pick struct {
	Kind        PickKind
	Service     string // set for PickOperation
	OperationID string // set for PickOperation
	Ambiguous   bool   // S4: recorded, not acted on
}

// Picker names one operation of the shortlist for the question, in the
// measured picker's format (docs/specs/staging.md, section 4).
type Picker interface {
	Pick(ctx context.Context, query string, shortlist domain.Catalog) (Pick, error)
}
```

```go
package pick

const SystemPrompt = `あなたは社内APIの振り分け役。...` // byte-identical to PICK_SYSTEM_PROMPT

// Fixed candidate lines after the shortlist (S3). operationId column is the
// built-in's name; serviceDisplayName column is "platform"; summary is the
// Japanese phrase. Their wording is what Task 4 measures.
const (
	lineListCapabilities = "list_capabilities\tplatform\t何ができるか知りたい"
	lineProposePanel     = "propose_panel\tplatform\t画面に出したい"
	lineNone             = "none\tplatform\t該当なし"
)

func New(client *chat.Client, model string) *Picker
func (p *Picker) Pick(ctx context.Context, query string, shortlist domain.Catalog) (usecase.Pick, error)
```

- [ ] **Step 1: write the failing tests**

`pick/picker_test.go`, through `chat.Client` with a fake transport (see
`toolcall/planner_test.go` for the pattern):

- sends `model`, `temperature: 0`, `max_tokens: 200`,
  `chat_template_kwargs: {"enable_thinking": false}`, no tools, exactly two
  messages: system = `SystemPrompt`, user = `質問: <query>\n\n候補:\n` + one
  line per shortlist endpoint in shortlist order
  (`operationId\tserviceDisplayName\tsummary`) + the three fixed lines in
  S3's order. Assert the whole user string.
- `Endpoint.Summary` is the summary column; when it is empty, the
  description's first line (the same fallback `toolFor` uses - read
  `tools.go` and reuse the helper rather than write a second one).
- parse: response `listInventoryItems certain` → `PickOperation`, service
  and id of that endpoint, `Ambiguous: false`; `... ambiguous` → true;
  longest id first (`listInventoryItems` vs `listInventoryItem` present
  together); `list_capabilities certain` → `PickListCapabilities`;
  `propose_panel` → `PickProposePanel`; `none` or no id at all →
  `PickNone`; `finish_reason: length` → `PickNone` and a warn log with the
  preview (same shape as `toolcall`'s truncation log).
- an empty shortlist → `PickNone` without calling the model.

`pick/prompt_test.go`: reads `../../../../../e2e/narrowing/pick/client.ts`
(compute the path from the test file's location), asserts
`strings.Contains(ts, SystemPrompt)` and that the candidate-line format
string matches `candidateLine`'s (assert on the `\t` join and the
`質問: ` / `\n\n候補:\n` framing literally present in the TS).

`e2e/narrowing/pick/client-prompt.test.ts`: reads
`../../../services/platform/internal/adapter/planner/pick/prompt.go`,
asserts the Go file contains `PICK_SYSTEM_PROMPT`'s text verbatim.

- [ ] **Step 2: run them, see them fail** (`cd services/platform && go test
./internal/adapter/planner/pick/...`; `cd web && ...` is not involved -
      the TS test runs under `make check`'s e2e unit target).
- [ ] **Step 3: implement** `usecase/picker.go`, `pick/prompt.go`
      (constants + `userMessage(query, shortlist)`), `pick/parse.go`
      (`parse(content string, candidates []candidate) usecase.Pick` -
      longest id first, `ambiguous` regexp case-insensitive), `pick/picker.go`.
- [ ] **Step 4: run the tests, green.** `make check` green; llama-swap
      count unchanged.
- [ ] **Step 5: commit** `feat(planner): add the pick stage - port and adapter`.

---

### Task 2: the orchestrator's staged branch

**Files:**

- Create: `services/platform/internal/usecase/orchestrator_staging.go`,
  `orchestrator_staging_test.go`
- Modify: `orchestrator.go` (`WithPicker(p Picker)`, `WithStages(n int)`,
  the one branch in `Plan`), `orchestrator_preferred.go` (`planPreferred`
  gains a `fromPick bool` - when true and the endpoint has an enum
  parameter, `AskUserTool()` is offered beside `toolFor(endpoint)` and a
  `DecisionAsk` naming that operation is honoured through `o.ask`; when
  false, today's behaviour byte for byte)

**Consumes:** `usecase.Picker`, `usecase.Pick` (Task 1).

**Produces:** `Orchestrator.planStaged(ctx, catalog, query, answers, turns, thinking) (Result, error)`.

- [ ] **Step 1: write the failing tests** (`orchestrator_staging_test.go`,
      fake picker + the existing fake planner that records its tool list):

- `WithStages(2)` + picker names an operation → planner called once with
  exactly that operation's tool (and `AskUserTool()` iff it has an enum
  parameter); result is the call's; `alternatives` are the shortlist's
  next two after it (the existing `alternativesFor` on the narrowed
  catalogue, not the one-endpoint catalogue - assert two alternatives).
- picker names an operation with a required parameter not in `answers` →
  planner is still called with that operation's tool (amended
  2026-09-16, create/create-attendance regression,
  `docs/specs/staging.md` section 5: a pick's question is the source of
  its own arguments, so `planPreferred`'s required-parameter shortcut
  applies only when `fromPick` is false); a call to that operation forms
  or invokes normally, anything else degrades to the form
  `requiredParamsKnown` used to return without calling the planner at
  all.
- picker → `PickListCapabilities` → the same `Result` `listCapabilities`
  returns today, planner not called.
- picker → `PickNone` → `kind: none`, planner not called.
- picker → `PickProposePanel` → planner called once with the full
  `ToolsFor(shortlist)` (today's single call); result is whatever it
  returns.
- `fromPick` + planner returns `DecisionAsk` for that operation's enum
  parameter → `kind: ask` with the options (not a form).
- request with `preferred` set + `WithStages(2)` → picker not called.
- `WithStages(1)` or no `WithPicker` → picker not called; the existing
  single-call tests still pass untouched.
- `WithStages(2)` without `WithPicker` → `NewOrchestrator` panics or
  returns an error at construction, never at request time (pick one, say
  which in the doc comment).
- the pick's `Ambiguous` and the picked id are logged at info
  (`slog` with `pick.operationId`, `pick.ambiguous`, `pick.ms`) - assert
  through a captured handler as other tests do, if any do; otherwise
  assert the log call is made through the orchestrator's logger field.

- [ ] **Step 2: run, fail.**
- [ ] **Step 3: implement.** `Plan`: after `Narrow`, before `ToolsFor`:
      `if o.stages == 2 { return o.planStaged(...) }`. `planStaged` in
      `orchestrator_staging.go`; `planPreferred` gains the flag; the
      request-`preferred` call site passes `false`.
- [ ] **Step 4: green; `make check`; count unchanged.**
- [ ] **Step 5: commit** `feat(usecase): plan in two stages when configured`.

---

### Task 3: configuration, wiring, and the measurement pass-through

**Files:**

- Modify: `services/platform/internal/infra/config/config.go` (+ test):
  `ORCHESTRA_PLANNER_STAGES` = `1` (default) | `2`, anything else fails
  startup; `pkg/app/app.go`: build `pick.New(chatClient, cfg.LLM.Model)`
  and pass `usecase.WithPicker` + `usecase.WithStages` when `2`;
  `e2e/shortlist/boot.ts` (`stages?: 1 | 2` → `ORCHESTRA_PLANNER_STAGES`),
  `e2e/shortlist/run.ts` (`--stages 1|2`, variant suffix `-stages2` on the
  output file and report section), `Makefile` `eval-shortlist` gains
  `STAGES=` pass-through (protected path: `ORCHESTRA_ALLOW_HARNESS_CHANGE=1`,
  that one line only); `e2e/eval/services.ts` passes
  `ORCHESTRA_PLANNER_STAGES` through when set (as it does `WORDING`).
- Document the variable where `ORCHESTRA_PLANNER_THINKING` is documented.

- [ ] **Step 1: tests** - config parsing (`1`, `2`, unset, `3` → error);
      app wiring covered by the existing app tests' pattern (a `2` config
      builds without error); `run.ts`'s flag parsing test if one exists for
      `--thinking`.
- [ ] **Step 2: implement; `make check` green; count unchanged.**
- [ ] **Step 3: commit** (a) `feat(config): ORCHESTRA_PLANNER_STAGES`,
      (b) `feat(shortlist): STAGES= pass-through`.

---

### Task 4: measure and record (session owner)

- [x] `make eval-shortlist STAGES=2` (narrowing on, wording
      `v6-unmatched-filter`, thinking off - the defaults). Copy the jsonl
      and report to the scratchpad. Never delete `e2e/shortlist/out/`. 79
      correct@1 / 80 correct@shown, mean 893ms/p50 989ms - scratchpad
      `stages2-fixed-report.txt`/`stages2-fixed.jsonl`.
- [x] `ORCHESTRA_PLANNER_STAGES=2 make eval` from the repo root: `capability`,
      `unanswerable`, `no-enum-value`, `no-enum-value-attendance` must
      hold; report every case that moved. All eighteen cases at baseline
      after two fixes (`5031418`, `555c485`) - scratchpad
      `eval-stages2-fixed.txt`. Re-run once more with `STAGES` unset
      (the new default): still every case at baseline, scratchpad
      `eval-default-stages2.txt`.
- [x] From the platform log of the shortlist pass: mean and p50 of `pick.ms`
      against the total latency; the `ambiguous` rate. Pick alone: mean
      378ms, p50 379ms; 43 of 100 picks flagged `ambiguous` (recorded
      only, S4).
- [x] The 22 rows of the spec's section 1, id by id: pick right → fill
      right / fill lost; pick wrong. 15 recovered, 7 still missed by the
      pick itself, 6 lost against the single call, 3 more gained beyond
      the original 22 - see `DECISIONS.md`.
- [x] Record in `DECISIONS.md` beside 67 / 71 and 83; move the default in
      S6 or say why not; `STATE.md`, `TODO.md`. AC-S-107. Default moved to
      `2` (`DECISIONS.md`, 2026-09-16, "Planning in two stages: pick first,
      fill second - now the default").
