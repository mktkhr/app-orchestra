# Tuning a prompt for a model that is not the incumbent

Measured 2026-09-19 at `f78d705`, on the model-blind 50-question extension
set (`--corpus ext`, 25 axis D + 25 axis E). All local models, no cost.

## Where this starts

`base-comparison-2026-09-19.md` established that the shipped wording is a
prosthetic fitted to `qwen3.5-9b-q8`: worth +12 to it, +2 to
`bonsai2-27b`, **-4** to `gemma4-12b-q8`. The obvious follow-up is whether
a prompt written for one of the others buys anything - and if so, how many
attempts it takes, since the incumbent's own wording took four rounds of
which three were negative.

Economics first, from the numbers already in hand: the tuning is worth
most to the model that starts lowest. `qwen3.5-9b-q8` climbs 38 → 50 with
four rounds of patches; `bonsai2-27b` starts at 58. **Four rounds of
tuning on the incumbent do not reach the untuned newcomer.** So tuning
looks like a way to rescue a weak model, not to raise a strong one - which
argues for picking two or three finalists on bare numbers and spending the
effort only there.

## `gemma4-12b-q8`: the fix was to remove, not to add

Its two losses under the default wording, read row by row, were both
escapes into `list_capabilities` on questions it answered correctly under
`v1`. Bisecting the wording chain:

| wording                         | overall (50) |
| ------------------------------- | ------------ |
| `v1`                            | **58**       |
| `v2-commit`                     | **58**       |
| default (`v6-unmatched-filter`) | 54           |

The whole of the -4 is `v6`'s own added sentence - the one that tells the
model to ask instead of guessing when a restricting word matches no enum
value. It was written to stop `qwen3.5-9b-q8` silently dropping filters.
`gemma4-12b-q8` does not drop filters, so the instruction only makes it
more hesitant. **A patch for a weakness the model does not have is not
neutral; it is a cost.**

No new wording needed: selecting `v2-commit` restores 58.

## `bonsai2-27b`: reading the misses first

14 of 25 axis-D questions missed, and the misses had shapes the
incumbent's wordings were never written for:

- **wrong verb (3):** 現品を数えて記録に残したい → `updateInventoryStockCount`
  (wanted `create...`); 無料の見本を送ってほしいと頼まれた →
  `getSalesSampleRequest` (wanted `create...`).
- **a neighbouring or more general resource (8):** 取引先の仕事ぶりを点数に
  したい → `...SupplierScorecard` (wanted `...SupplierEvaluation`);
  移動にかかったお金 → `createExpenseClaim`, the generic one (wanted
  `createExpenseMileageClaim`); two different "we are short of parts"
  questions both → `createPurchasingBackorder`.
- escapes to `list_capabilities` (3), which the existing wordings already
  address.

Two sentences were written from that reading, each naming no resource,
service or example question.

## The mistake worth recording: the sentences went to the wrong stage

They were added to the `wording` package - `v7-verb` and `v8-specific`,
`f65cde8` - and measured. Both produced **byte-identical answers to the
default on all 50 questions**. Not "no gain": _identical_, every row.

`ORCHESTRA_PLANNER_WORDING` reaches the **fill** stage. Which operation
gets chosen is decided by the **pick** stage, which carries its own
Japanese system prompt in `internal/adapter/planner/pick/prompt.go` and
had never been parameterised - `ORCHESTRA_PICK_WORDING` (`be5bdcd`)
covered only its three built-in candidate lines. So the experiment could
not have moved anything.

**A run that matches the baseline row for row is not a null result, it is
a wiring check that failed.** A real null result has rows moving in both
directions and netting out.

`f78d705` puts the system prompt inside `pick.Wording`, keeping `v1`
byte-identical and the default request unchanged, and adds two sets that
differ from `v1` by exactly one appended Japanese sentence:

- **`v3-verb`**: 操作を選ぶときは質問が求める動詞に合わせ、まだ存在しない
  ものの記録や申請を求める質問には、それを参照・更新する操作ではなく、
  新しく作成する操作を選ぶこと。
- **`v4-specific`**: 候補に対象を広く扱う操作とより具体的な対象を扱う操作の
  両方があり、どちらも条件に合いそうな場合は、それも該当するだけの広い操作
  ではなく、質問が名指ししている具体的な対象の操作を選ぶこと。

## What the pick-stage sentences did

`bonsai2-27b`, extension set:

| pick wording   | axis D (25) | axis E (25) | overall (50) |
| -------------- | ----------- | ----------- | ------------ |
| `v1` (default) | 44          | 76          | 60           |
| **`v3-verb`**  | 44          | **84**      | **64**       |
| `v4-specific`  | 44          | 80          | 62           |

Four rows changed between the default and `v3-verb`, all four from wrong
to right (checked row by row, not inferred from the total).

**It worked, and not where it was aimed.** The sentence was written for
the three wrong-verb misses on axis D; axis D did not move at all, and
axis E gained eight points. The mechanism is legible after the fact: an
axis-E decoy is a settings operation, `get*Setting` or `update*Threshold`,
and a rule that says "a question asking to record something calls the
operation that creates it, not one that reads or edits" pushes exactly
those away. The reading that produced the sentence was wrong about which
axis it would help, and the sentence was right anyway.

## The same sentence on the other two models

| model           | default | `v3-verb` |
| --------------- | ------- | --------- |
| `bonsai2-27b`   | 60      | **64**    |
| `gemma4-12b-q8` | 54      | 56        |
| `qwen3.5-9b-q8` | 50      | **46**    |

**The model-specificity runs both ways.** Until now the finding was that
the incumbent's wording does not travel to other models. Here a sentence
written from `bonsai2-27b`'s own misses costs the incumbent four points.
A prompt is not a general improvement that some models fail to exploit; it
is a fit to one model's failure modes, and a misfit elsewhere.

## Where `bonsai2-27b` now stands

| configuration            | overall (50) |
| ------------------------ | ------------ |
| bare (`v1` fill wording) | 58           |
| shipped default          | 60           |
| default + pick `v3-verb` | **64**       |

One round of tuning, +4. The incumbent needed four rounds to gain +12 from
a much lower start (38 → 50), and even then sits below this model's
untuned score.

## Open

- Axis D for `bonsai2-27b` has not moved (44 throughout): the eight
  neighbouring-resource misses are untouched, and `v4-specific`, written
  for exactly them, gained two points on axis E instead.
- Whether `v3-verb` holds on the original 100 questions and on mid - it
  has only been measured on the blind 50.
- `gemma4-12b-q8` needs no new wording, only `v2-commit`; that has not
  been measured on the original 100 either.
- Nothing has been adopted: `ORCHESTRA_PICK_WORDING` and
  `ORCHESTRA_PLANNER_WORDING` are both unset by default, and the default
  model is unchanged.

## Raw data

`e2e/shortlist/out/ext-default-stages2-model-*-pick{v3-verb,v4-specific}.{jsonl,log}`
and the `v1`/`v2-commit` runs beside them.
