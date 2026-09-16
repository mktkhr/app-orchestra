# Staging: pick first, fill second

The planner chooses an operation and fills its arguments in one tool call.
This subproject splits that into two model calls - a pick in the picker's
own format, then a fill against the picked operation only - because the
measurement says the choosing is where the answers are lost, and the
choosing is what the picker does better.

## 1. What it proves

That the 16-point gap between the product's planner and the stand-in picker
is a difference of format, not of model.

Both run `qwen3.5-9b-q8` at temperature 0 with thinking off, on the same
reranker-ordered shortlist of 20 (`docs/specs/shortlisting.md`). Measured
2026-09-16 on the 100-question corpus (scratchpad `variant-A.jsonl` against
`pick-9b-rerank-k20.tsv`, row by row):

- both right 61, **picker right and planner wrong 22**, planner right and
  picker wrong 6, both wrong 11 - 67 against 83.
- Of the 22: **10 are `none`** - b06 to b10 (明細をまとめて見たい / 1件確認 /
  追加 / 修正 / 消したい), b13 to b15 (承認を新規登録 / 直したい / 取り消したい),
  d01 (休みたい). Every one names an operation that exists in two services
  (expense and sales both have 明細; purchasing and expense both have 承認),
  and the planner, offered twenty tools, answers with no tool at all. The
  picker marks most of them `ambiguous` and still names the likelier one,
  which is the answer. 11 pick a different operation (a10 a20 b04 b05 b19
  b24 c08 c09 c23 d08 e02 e03); one (d06) returns `list_capabilities`.
- What differs between the two is only what the model is shown. The picker
  reads a Japanese instruction and twenty tab-separated lines - operation
  id, service name, summary - and writes one line back: an id and `certain`
  or `ambiguous`. The planner reads an English system prompt, twenty JSON
  Schema tool definitions plus `ask_user`, `list_capabilities` and
  `propose_panel`, and must choose and fill arguments in the same breath.

Every lever tried on the single call - five wordings (`wording.md`), a
larger model, thinking, examples in the tools - moved it between 56 and 68.
The one thing not tried is letting the model choose the way the 83 was
measured, and only then asking it for arguments.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **S1** | Planning is two model calls. **Pick**: the shortlist is shown in the picker's format and the model names one operation. **Fill**: the planner is offered that operation's tool only, plus `ask_user`, and produces the call, a question, or a form - the path `preferred` already takes (`shortlisting.md`, "Choosing an alternative"; `Orchestrator.planPreferred`). The fill never sees the other nineteen.                                                                                    |
| **S2** | The pick's prompt is the measured one, byte for byte. `e2e/narrowing/pick/client.ts`'s `PICK_SYSTEM_PROMPT` and its candidate line (`operationId \t serviceDisplayName \t summary`) produced the 83; the platform's picker sends the same text so its number is comparable. The examples column is not shown (83 → 77 when it was). A copy in Go is a transcription; the Go and the TypeScript both assert the exact string in a test, and the TypeScript is the source.                         |
| **S3** | The pick offers three fixed lines after the shortlist, so the built-ins keep a way in: `list_capabilities` (使える操作の一覧を知りたい), `propose_panel` (画面に出したい), and `none` (どの候補も質問に合わない（業務と無関係な質問）). The pick is the only place they are offered; the fill does not get them. Their wording is measured by `make eval`'s `capability` and `unanswerable` cases, not guessed; `propose_panel` has no eval case and is covered by the orchestrator's own tests. |
| **S4** | `ambiguous` is recorded, not acted on. The pick's second word is logged and returned on the result for measurement; the alternatives stay the shortlist's next two after the picked one (H5). Asking on ambiguity is the lever `v3-ask-on-collision` measured at 68 → 56.                                                                                                                                                                                                                        |
| **S5** | The pick never thinks; the fill follows the 「思考」 switch. Thinking on the picker was flat or negative (`shortlisting.md` section 1); the fill is where an argument may need it.                                                                                                                                                                                                                                                                                                               |
| **S6** | Staging is a mode, not a replacement. `ORCHESTRA_PLANNER_STAGES=1` is today's single call, byte-identical when set or unset; `2` is this. The default stays `1` until the measurement in section 7 is recorded, and the entry that records it also moves the default or says why not.                                                                                                                                                                                                            |
| **S7** | The pick is a port in the usecase, implemented in an adapter. `usecase.Picker` takes the question and the shortlist and returns one operation id (or a built-in's name) and the ambiguity flag; `internal/adapter/planner/pick` implements it over `chat.Client`. The usecase still imports neither `net/http` nor `encoding/json`.                                                                                                                                                              |

## 3. Where it goes

```
question
   │
   ▼
narrow (H1)  ──▶  shortlist of K
   │
   ▼  STAGES=2
pick  (S1–S5)  ──▶  operationId | list_capabilities | propose_panel | none
   │                        │                 │               │
   │                        ▼                 ▼               ▼
   │                 today's list      today's single   DecisionNone
   │                 (no model call)   call, whole        (no model call)
   │                                   shortlist
   ▼
fill (planPreferred)  ──▶  result | ask | form
   │
   ▼
alternatives (H5): shortlist's next two after the pick
```

`Orchestrator.Plan` gains one branch after narrowing: when `o.stages == 2`
and no `preferred` was given, it calls `o.picker.Pick(ctx, query, catalog)`
and dispatches on the answer. A picked operation id goes down the
`preferred` path with that endpoint - the same code, the same tests, the
same withholding of built-ins (`0840502`). `list_capabilities` answers with
the catalogue list as `listCapabilities` does today, without a second model
call. `propose_panel` falls back to the single call over the whole
shortlist: it is the one built-in that needs an operation and arguments of
its own, and it is rare enough (`make eval` has one case) that a second
format for it is not worth its test surface. `none` is `DecisionNone`.

A `preferred` in the request bypasses the pick, as it bypasses narrowing:
the person already picked.

## 4. The pick

The adapter builds exactly what `pick/client.ts` builds:

- system: `PICK_SYSTEM_PROMPT`, unchanged.
- user: `質問: <question>\n\n候補:\n` then one line per shortlist entry in
  shortlist order, `operationId\tserviceDisplayName\tsummary`, then the three
  fixed lines of S3 in that order.
- `temperature: 0`, `max_tokens: 200`, `chat_template_kwargs:
{enable_thinking: false}`.

Parsing is the measured rule: the first candidate id that appears in the
response, longest id first so one id being a substring of another cannot
steal the match; `ambiguous` if that word appears anywhere. No id at all is
`none`. A response cut off at `max_tokens` is `none` and logged at warn
with its preview, as the planner's own truncation is.

The three fixed lines carry a summary each, in Japanese, and are the only
text in this subproject that is new prose to the model. Their exact words
are chosen by measurement (section 7) and recorded with it. The measured
wording (2026-09-16, fixing the `unanswerable` regression under
`ORCHESTRA_PLANNER_STAGES=2 make eval` without moving `capability`):

```
list_capabilities	platform	使える操作の一覧を知りたい
propose_panel	platform	画面に出したい
none	platform	どの候補も質問に合わない（業務と無関係な質問）
```

The original `list_capabilities` line ("何ができるか知りたい" - "want to
know what can be done") read as a paraphrase of any question the model
had no better place for, including 「今日の天気は？」; renaming it to name
the built-in itself, and giving `none` an explicit exclusion instead of a
bare "no match", fixed `unanswerable` on the first wording tried.

## 5. The fill

`planPreferred` as it stands: the picked endpoint's tool alone, or the
form at once when a required parameter is not in `answers`; anything the
model returns that is not a call to that operation degrades to the form
(`orchestrator_preferred.go`). Three things change:

- `ask_user` is offered alongside the one tool when the endpoint has an
  enum parameter, so an unmatched restricting word (`v6-unmatched-filter`)
  still ends in a question rather than a form. Under `preferred` from a
  chip the person has already chosen, so today's withholding stands there;
  under a pick they have not.
- The fill is where the request's `thinking` applies (S5).
- Under a pick, the required-parameter shortcut above never fires: a
  chip's `preferred` carries no fresh text (the person chose an operation
  and typed nothing new, `0840502`), so skipping the model and going
  straight to the form is exact - but a pick's `preferred` comes from the
  question itself, and the question is where its arguments are
  (create/create-attendance regression, `ORCHESTRA_PLANNER_STAGES=2 make
eval`: 「在庫を登録して。名前はテスト品、数量は5、引当済で」 picks
  `CreateInventoryItem` and must still reach the model to extract `{name:
"テスト品", quantity: 5, status: "allocated"}`). So under a pick the
  fill always calls the model with the one tool (plus `ask_user` per the
  bullet above); the same fallback - anything other than a call to that
  operation, or an honoured ask - still degrades to `formFor`.

## 6. The configuration

| Variable                   | Values   | Default |
| -------------------------- | -------- | ------- |
| `ORCHESTRA_PLANNER_STAGES` | `1`, `2` | `1`     |

`2` requires a narrower or a catalogue small enough to show whole; the pick
reads whatever `Narrow` returns, so with no narrowing configured it reads
the whole catalogue, as the planner does today. Nothing else is added: the
pick's model is the planner's model, its base URL the planner's.

## 7. What is measured

`make eval-shortlist STAGES=2` (a pass-through like `WORDING=` and
`THINKING=`), against the same corpus and fixture, narrowing on, wording
`v6-unmatched-filter`, thinking off. Reported beside the single call's
67 / 71 (2026-09-16, `variant-A-report.txt`) and the picker's 83:

- correct@1, correct@shown, asked, none, per axis; latency mean and p50 with
  the pick's own time separated from the fill's.
- the 22 rows of section 1, id by id: which the pick now gets and which the
  fill then loses.
- `make eval` under `ORCHESTRA_PLANNER_STAGES=2`: `capability`,
  `unanswerable` and the enum cases must hold - these are where S3's three
  lines and S5's `ask_user` are actually tested.

## 8. Deliberately excluded

- **A different pick prompt.** The 83 was measured with this one; changing
  it is a new baseline and a separate decision. Its three added lines are
  the only exception, and they are measured.
- **Asking when the pick is ambiguous.** S4.
- **A second pick when the fill degrades.** A fill that ends in a form is
  the person's to complete.
- **Sending the alternatives through the pick.** The reranker's order is
  measured (H5); the pick's second choice is not.
- **A cache of picks, a batch of picks.** One question, one pick.

## 9. Acceptance criteria

- **AC-S-101** With `ORCHESTRA_PLANNER_STAGES` unset or `1`, every request
  the platform sends to the model is byte-identical to today's; the
  shortlist measurement under `STAGES=1` reproduces 67 / 71.
- **AC-S-102** With `STAGES=2`, a question is answered by exactly two model
  calls when the pick names an operation, one when it names
  `list_capabilities` or `none`, and two when it names `propose_panel`.
- **AC-S-103** The pick's system prompt and candidate line format are
  asserted equal, in a Go test and a TypeScript test, to
  `PICK_SYSTEM_PROMPT` and `candidateLine` in `e2e/narrowing/pick/client.ts`.
- **AC-S-104** The pick sends `temperature: 0`, `max_tokens: 200` and
  thinking off regardless of the request's `thinking`; the fill sends the
  request's `thinking`.
- **AC-S-105** The fill offers the picked operation's tool and, when that
  operation has an enum parameter, `ask_user`; never `list_capabilities`
  or `propose_panel`.
- **AC-S-106** `make check` calls no model; the picker is tested against a
  fake `chat` transport; usecase imports neither `net/http` nor
  `encoding/json`; `guard-arch` holds.
- **AC-S-107** `make eval-shortlist STAGES=2` reports section 7's numbers
  and the 22 rows by id; the result is recorded in `DECISIONS.md` beside
  67 / 71 and 83, and the default in S6 is moved or explicitly kept.

## 10. What this does not settle

Whether the fill loses what the pick wins. The picker's 83 is "named the
right id"; the product's correct@1 also needs the arguments the operation
takes, and a form is not a result. The measurement separates the two so
the next lever is the right one.

And the corpus is still the fixture's, with its blind-written examples and
five look-alike services. The 22 rows of section 1 are what this fixture
makes hard; a real catalogue's collisions are its own.
