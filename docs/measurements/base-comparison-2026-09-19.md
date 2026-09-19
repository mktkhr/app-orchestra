# Measuring models bare: the axis D/E extension, and what it is for

Started 2026-09-19 at `81c27d3`. This file fills in as runs finish; rows
marked _running_ or _not run_ are exactly that.

## Why this instrument exists

Three findings from the preceding rounds forced it:

1. **The shipped wording is a prosthetic fitted to `qwen3.5-9b-q8`**
   (`wording-is-model-specific-2026-09-19.md`): worth +12 corpus points to
   that model and +3 to `bonsai2-27b`; on `make eval`, +2 and 0. So every
   cross-model number taken under the default wording compares a model
   wearing its own prosthetics against one wearing someone else's.
2. **`make eval` is itself qwen-shaped.** Its cases were added when
   `qwen3.5-9b-q8` failed something, its baseline is that model's own
   accepted run, and 23 of its 34 cases admit a single answer. It is a
   regression gate for the incumbent, not a neutral comparison.
3. **The axes that separate models are the smallest ones.** In the
   100-question corpus, axis D (vocabulary gap) has 15 questions and axis
   E (setting decoys) has 10 - so one question moves 6.7 or 10 points,
   while the measurement's own determinism band is about 7 rows. Those are
   exactly the axes where the biggest spreads appeared (axis D: Sonnet 5
   at 80 against Opus 5 at 47; axis E: Fable 5.1 at 60 against everyone
   else at 90).

So: **25 new axis-D and 25 new axis-E questions** (`7a1a69a`, sharpened in
`60988bc`), held apart from the original 100 so the recorded numbers stay
comparable, and run with `--corpus ext` (`81c27d3`).

## How the extension set was built

Written by a subagent that was **forbidden to read any measurement
result** - no `docs/measurements/**`, no `DECISIONS.md`, no
`baseline.json`, no `out/` - so no question could be shaped by knowing
which model gets what right. Both sets are checked structurally by
`corpus-extension.test.ts`, the same way the original corpus is: axis D
questions share no character bigram with any of their answers' combined
text (`bigramsOf`), axis E decoys strictly out-score every answer
(`scoreOperation`) and are settings operations (`isSetting`) while the
answers never are.

Reviewed by hand afterwards; four axis-D questions were sent back and
replaced or widened (a "who is here" question that admitted only clock
events, and three whose answer needed a leap - repacking → packing list,
stacking → pallet, a payment method → corporate card). The replacements
each rule out their neighbours in the question text itself: 倉庫から倉庫へ
(transfer, not shipment or receiving), 倉庫の中でどこに (storage location,
not the warehouse), 先方に渡した (payment, not settlement or
reimbursement).

## Results - extension set (25 axis D + 25 axis E)

Two wordings per model: `v1` is the pre-tuning wording, `default` is
today's `v6-unmatched-filter`. Two stages, narrowing on at K=20,
everything else at its default. Local models only - the Claude models were
measured on the original 100 questions under the default wording and are
not being re-run here.

| model                 | wording | axis D (25) | axis E (25) | overall (50) | ms mean |
| --------------------- | ------- | ----------- | ----------- | ------------ | ------- |
| `qwen3.5-9b-q8`       | `v1`    | **16**      | 60          | **38**       | 1,511   |
| `qwen3.5-9b-q8`       | default | 40          | 60          | 50           | 1,623   |
| `qwen3.5-9b` (Q4_K_M) | `v1`    | 28          | 64          | 46           | 1,920   |
| `qwen3.5-9b` (Q4_K_M) | default | 44          | 64          | 54           | 1,610   |
| `bonsai2-27b`         | `v1`    | 40          | **76**      | **58**       | 3,657   |
| `bonsai2-27b`         | default | 44          | **76**      | **60**       | 3,701   |
| `gemma4-12b-q8`       | `v1`    | **48**      | 68          | **58**       | 4,478   |
| `gemma4-12b-q8`       | default | 40          | 68          | 54           | 2,465   |
| `gemma4-12b` (q4 QAT) | `v1`    | 32          | 64          | 48           | 2,277   |
| `gemma4-12b` (q4 QAT) | default | 36          | 68          | 52           | 1,913   |

What the wording is worth, per model, on this set: `qwen3.5-9b-q8`
**+12**, `qwen3.5-9b` +8, `gemma4-12b` +4, `bonsai2-27b` +2,
`gemma4-12b-q8` **-4**.

## The original 100 questions, for comparison

Measured earlier the same day, same conditions apart from the question
set (`wording-is-model-specific-2026-09-19.md` for the first two rows):

| model           | wording | corpus (100) | axis D (15) | axis E (10) |
| --------------- | ------- | ------------ | ----------- | ----------- |
| `qwen3.5-9b-q8` | `v1`    | 65           | 47          | 70          |
| `qwen3.5-9b-q8` | default | 77           | 60          | 90          |
| `bonsai2-27b`   | `v1`    | 78           | 60          | 90          |
| `bonsai2-27b`   | default | 81           | 73          | 90          |

## What to read out of it

**1. The two question sets agree, and that is the first thing to say.**
Fifty questions written today, model-blind, by someone forbidden to read
any result, reproduce what the original hundred said: bare, the incumbent
is last (38) and `bonsai2-27b` and `gemma4-12b-q8` lead (58); the wording
is worth +12 to the incumbent and +2 to `bonsai2-27b` (the original
corpus said +12 and +3). Two independently written question sets, the same
verdict.

**2. Bare, the shipped model is the weakest of the five.** 38 against 58

- twenty points. On the original corpus it was 65 against 78. The default
  configuration's competence is substantially the prompt's, not the model's.

**3. The wording can be negative.** `gemma4-12b-q8` reads **58 under `v1`
and 54 under the default**: the prosthetic written for another model costs
it four points. That did not show on the original hundred, where no model
lost ground. It is the sharper form of the same finding - a prompt tuned
against one model's failures is not neutral for the others, it is a bet
that they fail the same way.

**4. All of the wording's effect is on axis D. None of it is on axis E.**
Axis E moves by zero for four of the five models between `v1` and the
default (60/64/76/68 unchanged; `gemma4-12b` alone moves 64→68). Axis D
moves +24, +16, +4, -8, +4. So the tuning is entirely a vocabulary-gap
patch - it teaches the model to bridge 立て替え → 経費申請, and it does
nothing at all about a settings decoy that out-scores the answer
lexically.

**5. Axis E is where the model itself shows.** Nothing in the prompt
moves it, and the spread is wide and stable: `bonsai2-27b` 76,
`gemma4-12b-q8` 68, `qwen3.5-9b` 64, `qwen3.5-9b-q8` 60. If a single
number had to stand for "how good is this model at this task, bare", this
is the least contaminated candidate the instruments currently produce.

**6. Resolution improved as intended.** At 15 and 10 questions an axis, a
single row moved 6.7 or 10 points; at 25 each it moves 4. The -4 on
`gemma4-12b-q8` is one row on axis D (48 → 40 is two rows); it would have
been invisible or indistinguishable from noise at the old sizes.

## Raw data

`ablation/ext-out/<model>-<wording>.txt` in the session scratchpad; the
runs themselves write
`e2e/shortlist/out/ext-{v1,default}-stages2-model-<model>.{jsonl,log}`
and `misses-ext-*.txt`.
