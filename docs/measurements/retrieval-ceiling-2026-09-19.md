# The ceiling was in the retriever, and three prompt rounds were spent below it

Measured 2026-09-19 at `d1af9b9`, on the model-blind 50-question extension
set. Free - local models and the local embedding/rerank stack only.

## How this was arrived at

Three pick-stage wordings were written for `bonsai2-27b` from its own
misses and measured (`tuning-per-model-2026-09-19.md`): `v3-verb` +4,
`v4-specific` +2, `v5-commit-to-a-candidate` +2 - **and all of the gain
was on axis E every time, while axis D sat at 44 through all three.** The
sentences were written for axis-D failures. Three rounds, none of them
touching what they were aimed at.

The question not asked until then: **is the right answer even in the
twenty candidates the pick stage is shown?** No prompt can choose what it
cannot see.

## First measurement, and why it was wrong

Running the extension questions through `e5-large-q8` + `bge-reranker-v2-m3-q8`
and checking whether any answer lands in the top 20: axis D **18/25**,
axis E 24/25.

That number was taken with the **wrong pipeline**. `embedCatalogue`
embeds an operation's own text only; the product's narrowing
(`internal/adapter/narrowing/llamaswap`) additionally embeds every
`x-orchestra-examples` utterance and reranks on the operation's text plus
its examples. The fixture does carry examples - two per verb per resource,
written blind for the describing subproject - and the first measurement
ignored all of them.

## Second measurement, with the product's own shape

Re-run through `twoStageWithWrittenRerankerNarrowerOf` (examples embedded
_and_ read by the reranker, the configuration the product ships):

| retrieval           | axis D    | axis E    |
| ------------------- | --------- | --------- |
| no examples, K=20   | 18/25     | **24/25** |
| with examples, K=10 | 17/25     | 22/25     |
| with examples, K=20 | **19/25** | 22/25     |
| with examples, K=50 | **19/25** | 22/25     |

Three things fall out of that table.

**1. The examples buy almost nothing here: 18 → 19 on axis D.** And the
_identity_ of the misses changes completely - `d2-06`, `d2-09`, `d2-19`,
`d2-22`, `d2-25` come in; `d2-16`, `d2-17`, `d2-18`, `d2-20` drop out.
The top-20 is a fixed number of seats: an example pulls one operation in
and pushes another out.

**2. Widening the shortlist does not help.** K=20 and K=50 are identical,
19/25 and 22/25. So the answers that are missing are not ranked 21st to
50th - they are **not in the first stage's retrieval at all**. The
embedding retriever drops them before the reranker ever sees them, and no
amount of room downstream recovers that.

**3. The examples cost axis E: 24/25 → 22/25.** Axis E's decoy is a
settings operation whose summary reuses the question's own words. Adding
example utterances to every operation gives the decoy more surface to
match on too. The product ships this configuration, so it pays this
whenever a settings decoy is in play.

## What this means for the three prompt rounds

**Axis D's ceiling on this question set is 19/25 = 76%.** `bonsai2-27b`
reads 44 (11/25). So eight rows are genuinely the pick stage's to win, and
**six can never be won by any prompt** - the answer is not on the page.

That reframes the three rounds. They were not wrong to gain nothing on
axis D; they were aimed at a target that was 24% unreachable and already
28 points short for reasons the prompt could address. And it explains why
they all landed on axis E instead: axis E's answers _are_ retrieved
(22/25), so there the pick stage has something to get right or wrong, and
a sentence about verbs can move it.

## The narrowing-off run, read again

Measured earlier the same day: `bonsai2-27b`, extension set, whole
1000-operation catalogue instead of a shortlist.

|                          | axis D | axis E | overall |
| ------------------------ | ------ | ------ | ------- |
| narrowing on, K=20       | 44     | 76     | 60      |
| narrowing off (1000 ops) | 40     | **92** | **66**  |

Consistent with the ceiling reading, and sharper than it looked at the
time:

- **Axis E gains 16 points with no narrowing at all.** The retriever was
  the problem there - it puts the decoy in front of the answer. Show the
  model everything and it picks correctly. The narrowing stage is _costing_
  this axis.
- **Axis D loses 4.** With a thousand candidates the pick's own precision
  falls faster than the extra recall helps.

So narrowing is not uniformly good: it is a large win on the axes where
lexical similarity tracks the answer, and a net loss on the axis built out
of decoys that lexical similarity loves.

## Where the real work is

For axis D, in order of what the measurements support:

1. **The first-stage retriever** drops six of twenty-five answers before
   anything downstream can help. That is the binding constraint. Neither a
   bigger K, nor the examples as they stand, nor any prompt touches it.
2. **The pick stage** has eight retrievable rows it currently gets wrong -
   worth having, and the only part a wording can reach.
3. The examples layer helps by one row on this set. The 2026-09-15 result
   that made it look decisive (axis D 33% → 80%) was measured at **K=10**
   on the original corpus, where the shortlist was tight enough for
   better ranking to matter. At K=20 on these questions it has run out of
   room.

## Method note

The first ceiling figure in this file was wrong and is kept rather than
deleted: measuring recall through a pipeline that is not the one the
product runs produces a number that looks authoritative and is not. The
check that caught it was reading what `embedCatalogue` actually embeds,
against what `llamaswap`'s own index builds.

## Raw data

Scratchpad: `recall-ext.ts` (first, examples-blind) and `recall2.ts`
(product-shaped, `K=` sweep). The narrowing-off run is
`e2e/shortlist/out/ext-default-stages2-model-bonsai2-27b-narrowing-off.jsonl`.
