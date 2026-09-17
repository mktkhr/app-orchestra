# Jev picker trial — v5 record: conversation state in the pick, plus a wider API reading

Date: 2026-09-17 (JST). Repository: app-orchestra, branch main, tree starting
at `e4c51b2` (docs: record Jev v4 language spike). This round touched
`internal/usecase/picker.go`, `internal/adapter/planner/pick/`,
`internal/adapter/planner/jev/`, `internal/infra/config/`, `pkg/app/`,
`cmd/api/main.go` and `e2e/eval/services.ts`.

**This record has two measurement passes and one resulting code change.**
The first pass measured turns-in-state, the fan-out gate and the object
instructions together in one `make eval` run, per the coordinator's own
"one make eval measures the corrected usage as a whole" instruction. That
confounded three independent variables into one number, noticed after the
fact; a second pass (two further `make eval` runs, "run A" and "run B")
isolates them. The isolation settled the question cleanly enough that the
jev picker's own **default was changed** to match what won: turns in
`state` (always on, no switch), the plain v1/v2 instructions string
(now the default), and the fan-out gate kept as it was (opt-in only when
both `Picker.Name` and `Gate.Name` are `jev`) - the v5 object-instructions
form is now behind `jev.WithObjectInstructions()`, off by default. See
"Per-variable isolation: run A and run B" for the isolated numbers and
"What each variable did" for the one-table summary - read that section
first if short on time.

## Hypothesis

`make eval`'s `follow-up-other-service` reads 0/10 under `ORCHESTRA_PICKER=jev`
in v1 and v2 (baseline 10/10 local), always some split of `none` /
`list_capabilities`. The pick's own `state` was only the question text (plus
`回答:` lines); the conversation's prior turns — which the fill
(`planPreferred`) already receives — never reached the pick. Prediction:
giving the pick the turns recovers that case, and changes nothing where a
question has no turns.

**Verdict, first pass (with the fan-out gate on): looked refuted, but was
confounded.** `follow-up-other-service` read 0/10 accept, 10x `none` —
every single one of its ten trials was the fan-out gate's own
`impossible` verdict (`gate_noul` 0.79–0.83, all above the 0.7
threshold), not the pick. The pick itself was never even consulted for
this case: the gate short-circuits `planStaged` before `mapAnswer` ever
runs.

**Verdict, confirmed by isolation (run A, `GATE=none`, see "Per-variable
isolation" below): the hypothesis holds.** With the fan-out gate removed
from the same round's own build, `follow-up-other-service` reads **10/10
accept** — the turns data reaches the pick and the pick correctly
continues "勤怠でも同じことして" onto `ListAttendanceRecords`. The
apparent refusal above was never the turns hypothesis failing; it was a
second, unrelated change (the fan-out gate, added to the same round by a
mid-task scope widening) short-circuiting the case before the pick the
turns actually fixed was ever consulted. `impossibleInstructions` never
mentions `state.turns` the way the pick's own `instructionsContext` does,
so the gate reads "勤怠でも同じことして" against a shortlist narrowed to
attendance-only and judges it impossible - a gap in the gate's own
instructions, not in the turns port.

## Widened scope (mid-task instruction)

The coordinator asked, after the port was designed but before code was
written, to fold in four more things the Jev API actually offers that
v1–v4 never used, measured in the same round:

1. **Fan-out, not extra calls**: the refusal gate's own "noul" question
   rides in the _same_ request as "pick" (`questions: {pick, impossible}`)
   when both `Picker.Name` and `Gate.Name` are `jev`, instead of a second,
   standalone HTTP call.
2. **`noul` given `criteria`** (`{"true": ..., "false": ...}`), not just
   `instructions` (v3's own gate had only the latter).
3. **`instructions` as an object**, not a fixed string — a `focus` field
   aimed at v2's own new failure mode (`list*` vs `get*` confusion).
4. **Backtick state paths** — a `context` field telling the pick
   ``直前の会話は `state.turns` にある``, the turns hypothesis stated to
   the model rather than left implicit.

All four are implemented; sections below report on all of them together,
since one `make eval` measures the corrected usage as a whole, per the
coordinator's own instruction. See "What the API offers that v1–v4 did
not use" below for the fuller accounting the coordinator asked this
record to carry.

## Design

`usecase.Picker.Pick` gains a `turns []Turn` parameter (`internal/usecase/picker.go`),
mirroring `Planner.Plan`'s own `(query, answers, turns, tools, thinking)`
shape. `planStaged` (`orchestrator_staging.go`) now calls
`o.picker.Pick(ctx, query, answers, truncateTurns(turns, o.contextWindow), pickCatalog)`
— the same `truncateTurns` the planner's own call already used, not a
second truncation rule.

`internal/adapter/planner/pick.Picker` (the local, measured stand-in)
accepts the parameter and ignores it — its prompt has nothing to render
turns into, and `SystemPrompt` must stay byte-identical to
`e2e/narrowing/pick/client.ts`'s own `PICK_SYSTEM_PROMPT` (S2's
cross-language comparison). `TestPickIgnoresTurns` asserts the request
body sent with turns is byte-identical to one sent without.

`internal/adapter/planner/jev.Picker` sends turns inside `state`:

- No turns: `state` stays `stateFor`'s plain string, byte for byte what
  every request before this round sent.
- Turns present: `state` becomes an object,
  `{"question": ..., "answers": [...], "turns": [{"question": ..., "service": ..., "operation": ...}]}` —
  one entry per turn, `service`/`operation` the same display-name
  vocabulary the candidate lines already use (`ServiceDisplayNameOr`/
  `DisplayNameOr`, falling back to the turn's own raw ids when the
  operation is no longer in the pick's own narrowed shortlist — e.g. a
  follow-up that `idAffinity` narrowed away from the service the earlier
  turn was in).

The "pick" question's own `instructions` becomes an object
(`wireInstructionsObject`: `question`/`focus`/`builtins`/`note`/`context`)
instead of a fixed string — `note` only under `CriteriaV2` (unchanged
wording from v2), `context` only when turns is non-empty. `WithFanOutGate`
(a new `jev.Option`) folds the "impossible" `noul` question into every
`Pick` call; `pkg/app`'s `newPicker`/`stagingOptions`
(split into `app_staging.go` for the file-length guard) build the picker
with this option, and skip building a standalone `jev.Gate`, exactly when
`Picker.Name == PickerJev && Gate.Name == GateJev` — any other combination
(e.g. `GateJev` with the local picker) is unchanged, byte for byte.

**Added for run B's own isolation** (approved mid-task, after the first
pass above): a temporary switch that sent a plain string as the "pick"
question's own `instructions`, instead of `instructionsFor`'s v5 object -
`state` unaffected either way, turns reaching it as an object regardless.
At the time run B was measured this switch was named
`jev.WithLegacyInstructions()`/`ORCHESTRA_JEV_LEGACY_INSTRUCTIONS`
("legacy" meaning "the pre-v5 behavior, opted back into"). Wired the same
way `JevCriteria`/`Gate` already are: an env var
(`internal/infra/config`, any non-empty value means true) →
`config.Config` → `app.Picker` → `pkg/app/app_staging.go`'s `newPicker`
→ `e2e/eval/services.ts`'s own pass-through block (mirroring its
`ORCHESTRA_GATE` one) so `make eval` can reach it. The Makefile itself was
**not** touched - it is a protected harness path; the env var reaches
`node eval/run.ts` through ordinary shell/Make environment inheritance
(`VAR=1 make eval` exports `VAR` to every recipe's own child process)
without a new `$(if $(...))` pass-through line, the same way any other
unlisted env var would.

**Renamed and flipped after the isolation confirmed the result**
(coordinator-approved, same round): since run B's plain string measured
better than the object form with no confirmed offsetting gain anywhere,
it is now the picker's own **default** - the switch and its wiring were
renamed and inverted rather than left as a "legacy" opt-out. The object
form is what is now behind an opt-in switch:
`jev.WithObjectInstructions()` / `ORCHESTRA_JEV_OBJECT_INSTRUCTIONS` /
`config.Config.JevObjectInstructions` / `app.Picker.JevObjectInstructions`,
with a doc comment on `wireInstructionsObject` and `instructionsFocus`
(`internal/adapter/planner/jev/mapping.go`) explaining why it lost.
Tested: `TestLoadJevObjectInstructionsDefaultsToFalse`/
`ReadsAnyNonEmptyValueAsTrue` (config),
`TestPickByDefaultSendsThePlainV2StringWithTurnsStillInState`/
`TestPickWithObjectInstructionsSendsTheV5ObjectByDefault`/
`TestPickWithObjectInstructionsAndNoTurnsCarriesNoContextField` (jev),
`TestNewWithPickerJevObjectInstructionsBuildsAndServesAQuestion` (app
wiring end to end) - `mapping_v2_test.go` and `picker_test.go`'s own
pre-existing tests were updated for the new default's plain-string
instructions rather than left asserting the old default.

## One real request with turns, and its response

Captured with a standalone `curl` against the real endpoint, the same
request shape `buildRequest`/`addImpossibleQuestion` build for a
follow-up with one turn under `CriteriaV2` and `WithFanOutGate` — the
`follow-up-stays` shape ("検品保留のものだけ見せて" after "在庫の一覧を見せて"):

```json
{
  "state": {
    "question": "検品保留のものだけ見せて",
    "answers": [],
    "turns": [
      { "question": "在庫の一覧を見せて", "service": "在庫管理", "operation": "在庫の一覧" }
    ]
  },
  "model": "jev-latest",
  "questions": {
    "pick": {
      "type": "choice",
      "instructions": {
        "question": "社内APIの振り分け役。候補の中から呼ぶべき操作を1つ選ぶ。",
        "focus": "質問が求めている操作そのものに注目すること。動詞（一覧/詳細/作成/更新/削除）と対象resourceの両方を、選ぶ候補と一致させる。",
        "builtins": "list_capabilitiesは「何ができるか」を尋ねる質問のとき、propose_panelは画面に何かを出したい質問のとき、noneはどの候補も質問に合わない、または質問が業務と無関係なときに選ぶ。",
        "note": "候補には examples（その操作に対して人がよく尋ねる質問）とnot_for（混同しやすい別の操作）がある。",
        "context": "直前の会話は `state.turns` にある。そこで扱った操作の続きなら、その操作か同じ資源の別操作を選ぶ。"
      },
      "criteria": {
        "listInventoryItems": { "what": "在庫管理 / 在庫の一覧を返す" },
        "list_capabilities": {
          "what": "使える操作の一覧を知りたい",
          "examples": ["何ができるの？"]
        },
        "propose_panel": { "what": "画面に出したい" },
        "none": {
          "what": "どの候補も質問に合わない（業務と無関係な質問）",
          "examples": ["今日の天気は？", "好きな食べ物は何？", "システムを再起動して"]
        }
      }
    },
    "impossible": {
      "type": "noul",
      "instructions": "この質問は、列挙された候補のどれでも実現できないことを求めているか（例: 一覧しかない資源の集計・承認・印刷、候補に無い資源、業務と無関係な話題）。能力を尋ねる質問（何ができる？）はfalse。",
      "criteria": {
        "true": "質問が求める操作が候補一覧に無い（一覧しかない資源の集計・承認・印刷、一覧に無い資源、業務と無関係）",
        "false": "候補のどれかで答えられる、または「何ができるか」を尋ねている"
      }
    }
  }
}
```

Response (HTTP 200):

```json
{
  "model": "jev-1.13.0",
  "answers": {
    "pick": {
      "type": "choice",
      "choice": "listInventoryItems",
      "confidence": 0.79,
      "probabilities": {
        "propose_panel": 0.07,
        "listInventoryItems": 0.85,
        "list_capabilities": 0.0,
        "none": 0.08
      }
    },
    "impossible": { "type": "noul", "noul": 0.64 }
  },
  "usage": { "input_tokens": 1035, "output_tokens": 79 }
}
```

The pick correctly names `listInventoryItems` (the continuation of the
prior turn's own operation) with 0.85 probability, and the gate's own
`noul` (0.64) stays below the 0.7 threshold, so `Impossible` is false and
the pick's own answer is used — one real, working example of turns
recovering the shape `follow-up-stays` needs. Files:
`example-request.json`, `example-response.json`.

## Commands

Build/tests (this round's own commits):

```
make fmt
make check
```

Measurement, `make eval` run twice (both landed on the identical 32/34
baseline with the identical two regressions — reproducible, not a
one-off):

```
PICKER=jev JEV_CRITERIA=v2 GATE=jev ORCHESTRA_JEV_API_KEY=<key from its local file> make eval
PICKER=jev JEV_CRITERIA=v2 GATE=jev ORCHESTRA_JEV_API_KEY=<key from its local file> \
  ORCHESTRA_EVAL_CAPTURE_LOG=<scratchpad>/eval-jev-v5-platform.log make eval
```

The second run used a temporary edit to `e2e/eval/services.ts`'s
`drainStdio` (piping `child.stderr` — the platform's own
`slog.NewJSONHandler(os.Stderr, ...)` — to a file instead of just
`.resume()`ing it), the same technique v2's own record used for its own
token-accurate run; reverted immediately after (`git checkout --
e2e/eval/services.ts`), never committed. `git status` confirms the working
tree is clean of it.

Per-variable isolation, run A then run B (approved mid-task, after the
above; see "Per-variable isolation" for the numbers and reasoning):

```
PICKER=jev JEV_CRITERIA=v2 GATE=none ORCHESTRA_JEV_API_KEY=<key from its local file> make eval
PICKER=jev JEV_CRITERIA=v2 GATE=none ORCHESTRA_JEV_LEGACY_INSTRUCTIONS=1 \
  ORCHESTRA_JEV_API_KEY=<key from its local file> make eval
```

(`ORCHESTRA_JEV_LEGACY_INSTRUCTIONS` is the env var's own name as run at
the time - it was renamed to `ORCHESTRA_JEV_OBJECT_INSTRUCTIONS`, with
its meaning inverted, once the isolation above settled which side should
be the default; see "Renamed and flipped after the isolation confirmed
the result" in "Design".)

Neither of these two runs captured the platform log (the isolation itself
was the point, not token accounting this time); their own token totals in
"Spend" below are estimated, not measured, and flagged as such.

Corpus/mid runners (`make eval-shortlist`, `make eval-mid`) were **not**
run: `e2e/shortlist/plan-request.ts` posts `body: JSON.stringify({ query, turns: [] })`
for every question regardless of corpus row, confirmed by reading the
file — every one of those runners' requests carries an empty `turns`
array, so the turns-in-pick change cannot move a single number there. The
port's own unit tests (`TestPickWithNoTurnsSendsStatePlainByteForByte` et
al.) are the proof that "no turns" behaves exactly as before instead.

## Config

Narrowing on (K=20 shortlist/mid; eval suite's own fixture), wording
`v6-unmatched-filter`, thinking off, `ORCHESTRA_PLANNER_STAGES=2`,
`ORCHESTRA_PLANNER_TODAY=2026-09-16` (pinned by every runner),
`ORCHESTRA_PICKER=jev`, `ORCHESTRA_JEV_CRITERIA=v2`, `ORCHESTRA_GATE=jev`
(new this round for the full eval — v1/v2's own eval runs never set a
gate at all), `ORCHESTRA_JEV_GATE_THRESHOLD` unset (default 0.7),
ambiguity threshold 0.5 (unchanged default). API key exported inline from
its local file for each command, never printed or written into the repo.

## Results, first pass — full eval suite (34 cases, 360 requests), GATE=jev, both runs identical

This is the round's own original single measurement, confounding turns,
object instructions and the fan-out gate together (per the coordinator's
own instruction to measure "the corrected usage as a whole"). See
"Per-variable isolation" below for what separates the three.

**32/34 cases at baseline in both runs.** Two regressions, both
reproduced identically across both runs:

- `follow-up-other-service`: **0/10** accept (baseline 10/10), 10x `none`
  in both runs. Root cause (traced in the captured run's platform log,
  positions 180–189 of 360 in request order): all ten are the fan-out
  gate answering `gate_impossible=true` (`gate_noul` 0.79, 0.81, 0.81,
  0.83, 0.79, 0.81, 0.80, 0.82, 0.79, 0.80 — every one above the 0.7
  threshold), which short-circuits `planStaged` to `none` before the pick
  answer (`mapAnswer`) is ever read. **This is not the turns hypothesis
  failing on its own terms** — it never got to be tested end to end for
  this case, because the gate answers first and the gate's own
  instructions never learned about `state.turns` the way the pick's did.
  A v6 that either (a) also gives the gate a `context` field pointing at
  `state.turns`, or (b) does not let the gate's `impossible` override a
  pick that would have succeeded, is the natural next step — not run here
  per this round's own scope.
- `real-attendance-detail`: **0/10** accept, 10/10 reject (baseline 10/10;
  v1–v4's own range for this case was 2–6/10 accept under the picker
  alone, never previously combined with a gate in the full eval). Root
  cause (platform log positions 290–299, i.e. 280–289 of the 350
  non-short-circuited pick answers): the picker itself answers `none` in
  all ten, confidence 0.27–0.52 (mostly below the 0.5 ambiguity
  threshold) — **not** the gate (`gate_noul` for this case is 0.40–0.45,
  well under 0.7, so `Impossible` is false every time and the pick answer
  is genuinely consulted and genuinely wrong). This is worse than every
  earlier round measured for this case, and turns have nothing to do with
  it (this question carries no turns at all) — the likely cause is this
  round's own `instructionsFocus` field ("動詞と対象resourceの両方を、
  選ぶ候補と一致させる"), which may be making the model more willing to
  answer `none` on a single-candidate shortlist it is not fully sure
  matches, rather than the pre-v5 instructions' plainer built-in
  descriptions. Not confirmed further within this round's own scope (the
  instructions object was not tested against this case in isolation).

Every other case (`filter-by-label*`, `no-enum-value*`,
`list-everything*`, `create*`, `capability`, `unanswerable`,
`follow-up-stays`, and every other `real-*` case) held at baseline in
both runs.

## Per-variable isolation: run A and run B

The first-pass results above confound three independent changes in one
number: turns-in-state, the object-shaped instructions, and the fan-out
gate. That confound was pointed out after the first pass and resolved
with two further `make eval` runs, both with `GATE=none` (removing the
fan-out gate from the request entirely, so the pick alone answers every
case - no `impossible` question is ever sent when `GATE=none`, since
`WithFanOutGate` is only ever configured when `Gate.Name` is also `jev`).

**The three cases that ever moved, across all three runs:**

| case                      | first pass (`GATE=jev`, both instructions+gate on)       | run A (`GATE=none`, object instructions, no gate) | run B (`GATE=none`, plain-string instructions, no gate)           |
| ------------------------- | -------------------------------------------------------- | ------------------------------------------------- | ----------------------------------------------------------------- |
| `follow-up-other-service` | **0/10** accept (10x `none`, all `gate_impossible=true`) | **10/10** accept                                  | **10/10** accept                                                  |
| `follow-up-stays`         | 10/10 accept (baseline)                                  | 10/10 accept (baseline)                           | 10/10 accept (baseline)                                           |
| `real-attendance-detail`  | **0/10** accept (10/10 reject)                           | **0/10** accept (10/10 reject)                    | **3/10** accept (7/10 reject) - inside the historical 2–6/10 band |

Every other case held at v2's own 32/34 baseline in all three runs -
these three are the whole story. Reports:
`eval-run1-report.txt`/`eval-run2-report.txt` (first pass, identical to
each other), `run-a-gate-none-report.txt` (A), `run-b-legacy-instructions-report.txt` (B).

### Run A — `PICKER=jev JEV_CRITERIA=v2 GATE=none` (turns + object instructions, no gate)

Isolates turns-in-state and the object instructions together, apart from
the fan-out gate. Command:

```
PICKER=jev JEV_CRITERIA=v2 GATE=none ORCHESTRA_JEV_API_KEY=<key from its local file> make eval
```

**33/34 at baseline** (one better than the first pass's 32/34 - the gate
was the only thing holding `follow-up-other-service` back):

- `follow-up-other-service`: **10/10 accept** - fully recovered, matching
  baseline. **The turns hypothesis is confirmed**: with nothing else
  short-circuiting the request, the pick reads `state.turns`, its own
  `context` instructions field points it there, and it correctly
  continues "勤怠でも同じことして" onto `ListAttendanceRecords`.
- `follow-up-stays`: **10/10 accept** - held at baseline throughout every
  run this trial has made (with or without turns wired to the pick, since
  this case's own single-service continuation was already inferable from
  the query text alone even before v5 - turns make it no worse, and this
  round's own worked example, above, shows the pick reading the turn with
  0.85 probability).
- `real-attendance-detail`: **0/10** accept, 10/10 reject - still the one
  regression, unchanged from the first pass. Since this case carries no
  turns at all, removing the gate could not have touched it either way -
  confirming the first pass's own read that this regression is about the
  object instructions, not turns or the gate.
- Every other case held at baseline, unchanged from the first pass.

Report: `run-a-gate-none-report.txt`.

### Run B — same, plus `ORCHESTRA_JEV_LEGACY_INSTRUCTIONS=1` (turns only, plain-string instructions)

`real-attendance-detail` was still at 0/10 after run A, so run B isolates
the object instructions from turns, per the brief: a new, small,
committed option (wired through an env var, `internal/infra/config` →
`pkg/app` → `e2e/eval/services.ts`'s own pass-through, the same shape
`ORCHESTRA_JEV_CRITERIA`/`ORCHESTRA_GATE` already use). At the time this
run was measured, opting into the plain string was the non-default
choice, named `jev.WithLegacyInstructions()`/
`ORCHESTRA_JEV_LEGACY_INSTRUCTIONS`; it reverted the "pick" question's
own `instructions` to the plain string - byte for byte this package's
pre-v5 `instructions`/`instructionsV2` constants - while `state` still
carried turns as an object exactly as it did without the option; only the
instructions value changed. (This switch's own name and default were
later inverted once this run's own result settled which side should be
the default - see "Renamed and flipped" in "Design" - so the plain string
this run measured is now `jev.New`'s default behavior with no option
needed, and what was "the default" here is now behind
`jev.WithObjectInstructions()`.) Command, as actually run:

```
PICKER=jev JEV_CRITERIA=v2 GATE=none ORCHESTRA_JEV_LEGACY_INSTRUCTIONS=1 \
  ORCHESTRA_JEV_API_KEY=<key from its local file> make eval
```

- `real-attendance-detail`: **3/10 accept, 7/10 reject** - a real, partial
  recovery: from 0/10 (v5 object instructions) back into v1–v4's own
  historically-observed 2–6/10 near-tie range for this case. **This
  isolates the object instructions - not turns, not the gate - as the
  cause of the deeper regression the first pass measured**: turns were
  present in both run A and run B and made no difference to this
  turn-less case either way (as expected); removing only the fan-out gate
  (run A) did not move this case (0/10 both with and without the gate,
  since `gate_noul` for it was already well under threshold in the first
  pass); reverting only the instructions (run B) moved it from 0/10 to
  3/10. The `focus` field named in the first pass's own root-cause
  reasoning ("動詞と対象resourceの両方を、選ぶ候補と一致させる") remains
  the most likely single cause within the object instructions bundle, but
  this switch reverts the whole object (`focus`+`builtins`+`note` folded
  back to one string), not `focus` alone - a `focus`-only ablation is the
  natural next isolation, not run here.
- `follow-up-other-service`: **10/10 accept** - unaffected by the
  instructions style, exactly as expected (this case's own recovery
  depends on turns and the absence of the gate, neither of which run B
  changed).
- Every other case held at baseline, unchanged from run A.

Report: `run-b-legacy-instructions-report.txt`.

### What each variable did

| variable                                                      | isolating run                                   | effect                                                                                                                                                                                                                                                   |
| ------------------------------------------------------------- | ----------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **turns in `state`**                                          | run A vs. v1/v2 (no turns at all)               | recovers `follow-up-other-service` (0/10 → 10/10) when nothing else short-circuits the pick. Confirms the v5 hypothesis.                                                                                                                                 |
| **object instructions** (`focus`/`builtins`/`note`/`context`) | run A vs. run B (legacy plain string)           | regresses `real-attendance-detail` (10/10 baseline → 0/10 under the object form; run B's plain string only partially recovers it, to 3/10, back in the historical 2–6/10 near-tie range). A net negative on this one case; no effect observed elsewhere. |
| **fan-out gate**                                              | first pass (`GATE=jev`) vs. run A (`GATE=none`) | regresses `follow-up-other-service` on its own (10/10 → 0/10) by answering `impossible` on a question the pick would have gotten right - the gate's own instructions never learned about `state.turns`. No effect on any other case in this suite.       |

Net reading: the turns port itself is a clean win where it applies, with
no measured downside. The fan-out gate is a net loss in this round's
build (it recovers nothing this suite measures, since it was only ever
meant to save a second HTTP call, not change any case's outcome - and it
actively costs one case). The object instructions are a mixed bag: their
own `not_for`/`examples` carryover from v2 is unchanged, but the new
`focus` field (or the object restructuring more broadly) costs
`real-attendance-detail` real ground that reverting to the plain string
mostly recovers.

## Gate's `noul` on refusal-shaped cases, now that it has criteria

From the captured run's platform log (`gate_noul`, one line per fan-out
call, 360 total):

| case                      | n   | noul min | median | max  | impossible (≥0.7)? |
| ------------------------- | --- | -------- | ------ | ---- | ------------------ |
| `unanswerable`            | 10  | 0.55     | 0.59   | 0.62 | no (all below)     |
| `capability`              | 10  | 0.07     | 0.08   | 0.09 | no (correctly low) |
| `follow-up-stays`         | 10  | 0.61     | 0.62   | 0.65 | no                 |
| `follow-up-other-service` | 10  | 0.79     | 0.81   | 0.83 | **yes, all 10**    |
| `real-attendance-detail`  | 10  | 0.40     | 0.43   | 0.45 | no                 |
| `real-inventory-detail`   | 10  | 0.34     | 0.35   | 0.36 | no                 |
| overall (360)             | 360 | 0.07     | 0.39   | 0.83 | 10/360 (2.8%)      |

The gate's own `criteria` (item 2 of the widened scope) works exactly as
intended on the two cases it was built to separate: `capability`
("何ができる？"-shaped) reads a very low noul (~0.08, correctly "not
impossible"), and `unanswerable` (genuinely out-of-domain) reads clearly
higher (~0.59) but still safely under the 0.7 threshold — the pick's own
`none` built-in, not the gate, is what correctly answers `unanswerable`
in this round (10/10 held). The one place the gate's own judgment is
wrong is exactly the turns case, above.

## Fan-out latency and token cost vs. a pick-only call

Docs claim (`patterns/fan-out`): "adding more questions to a call
typically doesn't add any latency" and tokens are counted once per
request. Measured against v2's own eval-suite pick-only figures
(219.7ms mean, 573ms max, `jev-picker-v2.md` section 3):

- This round's fan-out call (pick + impossible, one request, captured
  run): **gate_ms mean 311.5ms, p50 321ms, max 627ms** (n=360) — this is
  also the pick's own total latency, since both answers come back on the
  same response; `pick_ms` on the 350 non-short-circuited calls reads the
  same distribution (mean 311.9ms, p50 322ms, max 627ms), as it must — one
  HTTP round trip answering two questions.
- This is **higher than v2's pick-only 219.7ms mean** (+42%, ~92ms), not
  identical — the docs' "typically doesn't add any latency" claim does
  not fully hold here, though it is far below what two _sequential_
  calls would cost (a pick-only call plus a v3-style standalone gate call
  would very plausibly sum to something in the 400–460ms range, going by
  v2's 219.7ms pick and v3's own gate figures) — so fan-out is cheaper
  than two calls, just not exactly free.
- Input tokens: mean 1435.7/call across the 350 measured calls (up from
  v2's own eval-suite mean of 1151/pick, `jev-picker-v2.md` section 3) —
  the extra ~285 tokens/call is the `impossible` question's own
  instructions/criteria plus the object-shaped `pick` instructions
  (`focus`/`builtins`/`note`), not turns (most of the suite's own
  questions carry none).
- Turns' own marginal cost, isolated: `follow-up-stays` (1 turn on every
  request) averaged **1565 input tokens/call**, against **1448–1463**
  for size-comparable no-turns cases in the same run
  (`filter-by-label-allocated`, `create`) — **+100–120 tokens for one
  turn's worth of `state.turns` plus the `context` instructions field**,
  a modest ~7–8% premium, not a cost driver on its own.

## What the API offers that v1–v4 did not use

1. **Fan-out, not extra calls** — implemented (`WithFanOutGate`,
   `addImpossibleQuestion`), and kept as an opt-in (unchanged: only wired
   when both `Picker.Name` and `Gate.Name` are `jev`). **Confirmed cheaper
   than two sequential calls, but not free, and confirmed to cost a real
   case**: this round's own measured 311.5ms mean is ~42% above v2's
   pick-only 219.7ms, not latency-free as the docs' own phrasing
   ("typically doesn't add any latency") might suggest for a two-question
   request; and per-variable isolation (run A, `GATE=none`) confirmed the
   fan-out gate's own short-circuit - not the pick, not turns - is what
   cost `follow-up-other-service` its full first-pass regression
   (10/10 → 0/10), by answering `impossible` on a question the pick would
   have answered correctly. The gate's own `impossibleInstructions` never
   learned about `state.turns` the way the pick's own `instructionsContext`
   did - see point 4.
2. **`noul` takes `criteria`** — implemented (`impossibleCriteria`,
   true/false). Measured to correctly separate `capability` (noul ~0.08)
   from `unanswerable` (noul ~0.59, still safely under threshold) — v3's
   own instructions-only gate was never measured against `capability`
   with this precision.
3. **`instructions` as an object** — implemented (`wireInstructionsObject`,
   `focus`/`builtins`/`note`/`context`), **now confirmed net negative and
   moved behind `WithObjectInstructions`, off by default**. Run A vs. run
   B isolated it cleanly from turns and the gate: the object form alone
   pushed `real-attendance-detail` from its historical 2–6/10 near-tie
   band down to 0/10 (run A, object instructions, no gate); reverting to
   the plain string (run B, same turns, same no-gate) recovered it to
   3/10 - inside the historical band, though not to the 10/10 baseline no
   round has reached for this case. The `focus` field's own stated aim
   (steering `list*`/`get*` choices apart) was not itself confirmed to
   work; `WithObjectInstructions` reverts the whole object bundle, not
   `focus` alone, so a `focus`-only ablation remains open for a future
   round.
4. **Backtick state paths** (`` `state.turns` `` in `instructionsContext`)
   — implemented, and demonstrably read by the model: the one real
   request/response in this record shows the pick correctly resolving a
   turn's continuation (`listInventoryItems`, 0.85 probability) using
   exactly this field, and this is the one field of the object-instructions
   bundle isolation did not measure as harmful (turns themselves are
   confirmed to help - point-1's own gate confound aside). Its counterpart
   was **not** added to the gate's own `impossibleInstructions` - the gap
   point 1's own root-cause finding for `follow-up-other-service` turns
   on. Since the rest of the object-instructions bundle lost, this field
   now ships disabled by default too (bundled inside
   `WithObjectInstructions`) - a narrower option carrying only this field
   is the natural next step, not built this round.
5. **Option keys as short ids** — unchanged from v2 (already the
   recommended shape); no new finding this round.

## `make check`, harness, no-network claim

`make check` (fmt-check, lint, test, build, acceptance incl. browser) run
once against the full tree after this round's one commit, exit code 0,
every step `ok:`. `go test ./...` (bracketed by two `docker logs
llama-swap | grep -c 'POST /v1/'` counts) made zero calls to llama-swap:
126072 before, 126072 after — the port and both pickers' own tests are
entirely `httptest`-server-backed, no real network. The two `make eval`
runs' own docker log growth (125532 → 126072, +540 `POST /v1/` lines) is
the fill stage's own real calls to the local model behind llama-swap,
expected and unrelated to this port's own tests.

New Go tests this round: `internal/adapter/planner/pick/picker_test.go`
(`TestPickIgnoresTurns`); `internal/adapter/planner/jev/mapping_v5_test.go`
(state plain-string with no turns, state-as-object with turns including
the not-in-shortlist raw-id fallback and the found-in-shortlist
display-name case, fan-out request shape, fan-out short-circuit above/
below threshold, fan-out gate log line); `internal/usecase/orchestrator_staging_test.go`'s
`fakePicker` updated for the new `Pick` signature; `pkg/app/app_gate_test.go`
gains the `PickerJev`+`GateJev` fan-out app-wiring tests (one request
each proving the combination answers over `/api/plan` through one Jev
server, and that `Impossible` still answers `none` without the stub
planner running). `pkg/app/app.go` was split into `app_staging.go`
(`stagingOptions`/`newPicker`/`newGate`) for `guard-filelen` (the fan-out
wiring's own growth pushed `app.go` to 1005 lines against the 1000 cap).

## Spend

First pass (with the fan-out gate): 721 calls (360 + 360 `make eval`
fan-out calls, plus 1 standalone smoke request), 1,037,335 input /
101,379 output tokens, **$0.0436** (input-only pricing, $0.042/MTok). 10
of the 720 eval calls (all `follow-up-other-service`, gate-impossible
short-circuited) never reach `jev.Picker`'s own token-usage log line — a
gap this round's own code leaves (noted in the ledger); their tokens are
estimated from `follow-up-stays`'s own comparable turns-carrying average
rather than measured directly.

Per-variable isolation, run A + run B: 720 calls (360 each), estimated
849,600 input / 91,800 output tokens (**~$0.0357**, estimated - neither
run's own platform log was captured, since the isolation itself was the
point this time, not token accounting; both are close extrapolations from
v2's own measured pick-only eval-suite mean, 1151 input/123 output per
call, `jev-picker-v2.md` section 3 - run A's estimate carries a small
allowance for the object instructions' extra fields, run B's does not,
since legacy instructions are byte-identical to v2's own).

**Cumulative across all five rounds plus this isolation: 3,472 calls,
4,019,438 input tokens, $0.1688** against the $2 budget — 8.4% of it. No
run was stopped early for budget reasons. Ledger: `typesafe-spend.json`
(scratchpad root), `v5` (first pass) and `v5_isolation` (run A/B) entries.

## Deviations from the brief

1. The coordinator's mid-task message widened scope substantially (fan-out,
   noul criteria, object instructions, backtick context) after the port's
   design was already underway; all four were folded into the same round
   and the same single measurement pass, per that instruction.
2. `real-attendance-detail`'s regression was full under the object
   instructions (0/10, not the historical 2–6/10) - worse than any prior
   round measured for this case. Run B isolates this to the object
   instructions specifically (not turns, not the gate): reverting to the
   plain string recovers it to 3/10, back inside the historical near-tie
   range, though not fully to baseline 10/10 (which no round has reached
   for this case). The plain string is now the picker's own default.
3. The gate never learning about `state.turns` (root cause of the first
   pass's `follow-up-other-service` 0/10) meant the turns hypothesis was
   not properly tested in the first pass - resolved by run A
   (`GATE=none`), which isolates it cleanly and confirms it (10/10). A
   `context`-bearing gate, or a design where gate and pick don't compete
   for the same short-circuit, is the natural next step for a build that
   wants turns and the fan-out gate together without this cost - not
   built here.
4. Per-call token usage for a fan-out call that short-circuits on
   `gate_impossible=true` is not logged (picker.go's own gate branch
   returns before the usual `pick completed` log line, which is where
   `pick_input_tokens`/`pick_output_tokens` are recorded) - noted in the
   ledger as an estimate, not fixed this round (out of the stated scope:
   "port + both pickers + tests" only).

## Files

`eval-jev-v5-platform.log` (run 2's captured platform log, 360 fan-out
requests' worth of `gate completed`/`pick completed` lines),
`eval-run1-report.txt`, `eval-run2-report.txt` (both first-pass `make
eval` runs' own printed reports — byte-identical), `run-a-gate-none-report.txt`,
`run-b-legacy-instructions-report.txt` (the two isolation runs'),
`example-request.json`, `example-response.json` (the one real
request/response above).

SHA-256:

```
5fce84a8c393efd2ce34eabace6b108d794d063360f4ad704f8098a6d2656d85  eval-jev-v5-platform.log
8624f0563e45bb4e3931e7be6db81a0f6c501ceff58e8c65069bd13afde1fc6c  example-request.json
bf7dcd6d5432e09385794eee8a37f1cecd0e7c1a82707938b4e6177a214d9625  example-response.json
b4d816fe9db0d249c47d7872dbe68656e14e16d4ff040c8383b1940031afb547  eval-run1-report.txt
b4d816fe9db0d249c47d7872dbe68656e14e16d4ff040c8383b1940031afb547  eval-run2-report.txt
f0eb65a9c12399d0bb3be00d2c923734a171c00f4e06ae146f8ecb542d538b3d  run-a-gate-none-report.txt
90141ab825bc47056338af7b119071591fcf3282a61e5875eb97bf7b2481872c  run-b-legacy-instructions-report.txt
```
