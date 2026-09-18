# Jev in the fill stage: the first end-to-end speed win, and what it costs

Measured 2026-09-18, after `jev-service-router.md`. Commits: `e221044`
(the two arms), `36e8bbe` (refusal + wide unset), `d67e256` (capabilities
option), `9e56e51` (judgements as separate questions), `f409b1b`
(evidence in those questions).

## Why the fill and not the pick

`jev-conditions.md` section 2e: the local pick is 931 ms p90 of a 3,485 ms
p90 plan at 8 requests in flight, and the fill runs on the same single
GPU - so replacing the pick cannot move end-to-end throughput, and the
hybrid round measured exactly that. The fill is the stage that owns the
GPU. For an operation whose arguments are a **closed set**, filling them
is classification, not generation.

## Two arms, both off by default

- **Arm 1, no Jev at all (`ORCHESTRA_FILL_SKIP_EMPTY=1`)**: when the
  picked operation declares no parameters and no body, skip the fill
  model call entirely.
- **Arm 2 (`ORCHESTRA_FILL_ENUM=jev`)**: when every parameter is
  enum-valued and there is no body, replace the fill model call with one
  Jev request - one Choice question per parameter, options = that enum's
  values (with their `x-enum-labels` Japanese labels) plus `__unset__`
  and `__mismatch__`. `__mismatch__` produces the product's existing ask
  over that parameter. Fail-open to the normal local fill on any error,
  missing answer, or confidence below `ORCHESTRA_FILL_ENUM_THRESHOLD`
  (default 0.5).

Eligibility, counted from the catalogues, not measured: arm 2 covers
**18 of `make eval`'s 34 cases** and **0 of the 100-question corpus**
(the fixture's only enum sits on request bodies, never on a parameter);
arm 1 covers **0 of the eval cases** (every real dummy operation has a
parameter or a body) and most of the corpus.

## Arm 1 - the control, and what it proves

| instrument                | local            | `ORCHESTRA_FILL_SKIP_EMPTY=1` |
| ------------------------- | ---------------- | ----------------------------- |
| corpus correct@1 / @shown | 77 / 80          | **79 / 81**                   |
| corpus latency mean / p50 | 1,363 / 1,257 ms | **731 / 479 ms**              |
| mid answerable            | 37/40            | 37/40                         |
| mid impossible refused    | **16/20**        | **12/20**                     |

Half the latency, no accuracy cost on answerable questions - and four
more impossible questions forced into an operation. **Skipping the fill
is what removes the last gate on a wrong pick**, and that is true with no
hosted model anywhere near it. Everything arm 2 shows below is a variation
on this same trade.

## Arm 2 - five wordings of the same request

All on the real two-service catalogue; eval is 34 cases x 10 runs,
dialogues are 12 conversations / 27 turns with turns chained from real
answers.

| variant                                           | eval      | dialogues | dialogue latency mean / p50 |
| ------------------------------------------------- | --------- | --------- | --------------------------- |
| local fill (the default)                          | **34/34** | **26/27** | 1,062 / 1,081 ms            |
| classifier, no escape hatch                       | 29/34     | 25/27     | 744 / 617 ms                |
| + refusal **as an option** (`_REFUSAL=1`)         | **33/34** | 25/27     | 786 / 613 ms                |
| + capabilities option + wide `__unset__`          | 32/34     | **26/27** | 724 / 559 ms                |
| judgements as **separate** questions, no evidence | **20/34** | -         | -                           |
| separate questions **with operation evidence**    | 32/34     | 25/27     | **684 / 549 ms**            |

### What each row taught

**No escape hatch (29/34).** The five losses are all questions where the
local fill would have answered `none` instead of running the picked
operation (`real-delete-inventory`, `real-decrease-inventory`,
`real-show-sales`, `real-sum-overtime`, `real-capability-inventory`). A
classifier asked only "which enum value" has no way to say "wrong
operation".

**Refusal as an option (33/34).** Adding one option meaning "this
operation cannot answer the question" to each parameter's own question
recovered four of the five.

**Adding a capabilities option too (32/34).** It fixed
`real-capability-inventory` and broke `real-delete-inventory` and
`real-decrease-inventory` back. Captured distributions on
`ListInventoryItems`' single `status` parameter show why:

| question           | `__unset__` | `__mismatch__` | `__refusal__`                 |
| ------------------ | ----------- | -------------- | ----------------------------- |
| 在庫を削除したい   | **0.70**    | 0.19           | 0.11                          |
| 在庫を減らしたい   | **0.81**    | 0.11           | 0.04                          |
| 在庫で何ができる？ | 0.04        | 0.00           | (`__capabilities__` **0.96**) |

"The question does not restrict `status`" is _true_ for the first two, so
it wins. A claim about the whole request cannot compete inside a field's
own option list - it is answering a different question.

**Separate questions, no evidence (20/34).** Moving both judgements out
into their own `noul` questions in the same request made it far worse.
The refusal probability barely moves with the question:

| question               | refusal noul           | correct action |
| ---------------------- | ---------------------- | -------------- |
| 検品保留の在庫を見せて | 0.44                   | call           |
| 在庫を全部見せて       | 0.55 → wrongly refuses | call           |
| 在庫を削除したい       | 0.50                   | refuse         |

The judgement question carried only its instruction sentence: it never
saw **which operation had been picked**, while the Choice questions
beside it carried the enum values and answered at 0.98-0.99.

**Separate questions with evidence (32/34).** Giving those same two
questions the picked operation's own display name, service, id, summary
and parameter titles - as a `criteria` object, the shape the vendor's
noul documentation and this repo's own fan-out gate already use - moved
it from 20/34 back to 32/34, at the lowest latency measured anywhere in
this trial (mean 684 ms against the local fill's 1,062 ms, **-36%**).

## The rule this round confirms for the third time

A Jev question is as good as the evidence inside that question:

1. the service router abstained on 56 of 100 questions with thin service
   descriptions and was right 93 of 93 with every operation name listed
   (`jev-service-router.md`);
2. a whole-request judgement stuffed into a field's option list loses to
   the field's own correct answer (0.11 against 0.70 above);
3. the same judgement as its own question is uninformative without the
   operation's text (0.44-0.55 for everything) and usable with it
   (20/34 → 32/34).

## Where this leaves the fill

Nothing here reaches the local fill's 34/34. The best correctness is
33/34 (refusal as an option); the best latency is 684 ms mean at 32/34
(separate questions with evidence); the two rows that never come back are
`real-sum-overtime` (残業を合計して, wrongly run) and
`real-inventory-list-graph`.

The trade is legible: **a third of the end-to-end latency for one or two
question-kinds out of 34, on a catalogue where 18 of 34 cases are even
eligible.** Arm 1 shows the same trade with no hosted model at all (half
the latency, four more forced answers on mid). Off by default;
`ORCHESTRA_FILL_ENUM`, `ORCHESTRA_FILL_SKIP_EMPTY` unset.

## Cost

About 650-700 input tokens per question, ~180 eligible calls per eval run.
Roughly $0.03 for this round's nine runs. Cumulative Jev spend:
**$0.4299** of the $2 budget.

## Open

- The evidence clause is spliced into the criteria without punctuation
  around the operation name (`選ばれた操作在庫一覧（操作id: ...）は`);
  readable but dense. Whether the wording costs anything is unmeasured.
- Mixed operations (one enum parameter plus one free-text) are out of
  scope: the arm requires every parameter to be enum-valued.
- No concurrency measurement of the fill arm: the numbers above are
  sequential runs. The GPU-freeing effect should be larger under load,
  which is exactly where `latency-bench.md` measured the local stack
  saturating.
