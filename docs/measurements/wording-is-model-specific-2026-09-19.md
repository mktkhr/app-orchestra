# The system prompt is a prosthetic, and it fits one model

Measured 2026-09-19 at `be5bdcd`. Two models x two wordings, eval and the
100-question corpus each time, nothing else varied.

## The question

Every wording this repository ships was derived by measuring
`qwen3.5-9b-q8` and patching where it failed: `v2-commit` (commit to a
plausible tool instead of retreating) because that model retreated,
`v6-unmatched-filter` (ask when a restricting word matches no enum value)
because it dropped the filter silently, the pick's three built-in lines
because it escaped into `list_capabilities`, the date line because it
invented dates. The default configuration is therefore not "the model" but
**the model plus its own prosthetics**.

So when `bonsai2-27b` read `make eval` 30/34 against the default's 34/34
(`frontier-full-2026-09-18.md`), the comparison was not between two
models. It was between a model wearing its prosthetics and a model
wearing someone else's. This round measures that directly.

`ORCHESTRA_PLANNER_WORDING=v1` selects the pre-tuning wording (the
`5bf5cf8` literals, still asserted byte-identical in the `wording`
package); unset selects today's `v6-unmatched-filter`.

## Results

| model x wording           | eval      | corpus  | A   | B   | C   | D   | E   |
| ------------------------- | --------- | ------- | --- | --- | --- | --- | --- |
| `qwen3.5-9b-q8` x `v1`    | 32/34     | **65**  | 92  | 56  | 56  | 47  | 70  |
| `qwen3.5-9b-q8` x default | **34/34** | **77**  | 92  | 80  | 64  | 60  | 90  |
| **gain**                  | **+2**    | **+12** | 0   | +24 | +8  | +13 | +20 |
| `bonsai2-27b` x `v1`      | 30/34     | **78**  | 92  | 92  | 56  | 60  | 90  |
| `bonsai2-27b` x default   | 30/34     | **81**  | 92  | 92  | 60  | 73  | 90  |
| **gain**                  | **0**     | **+3**  | 0   | 0   | +4  | +13 | 0   |

## What this says

**1. The tuning is worth +12 corpus points to the model it was tuned on,
and +3 to the other one.** On eval it is worth +2 to `qwen3.5-9b-q8` and
nothing at all to `bonsai2-27b`. The prosthetic fits one model.

**2. Untuned, the newcomer is ahead.** Bare `v1`: `bonsai2-27b` 78,
`qwen3.5-9b-q8` 65 - thirteen points. The default model climbs to 77 by
wearing four rounds of patches, which is where it draws level with the
other model's _starting_ position.

**3. So `make eval` 34/34 against 30/34 is mostly the prosthetic, not the
model.** `qwen3.5-9b-q8` reads 32/34 without it. Nobody has written a
wording for `bonsai2-27b`; one attempt was made
(`ORCHESTRA_PICK_WORDING=v2-strict-capabilities`, one sentence excluding
action-naming questions from `list_capabilities`) and moved nothing, which
is what the first attempt at a wording usually does - `v3`, `v4` and `v5`
were all negative results for `qwen3.5-9b-q8` too.

**4. Not all of the wording is model-specific.** Axis D - the vocabulary
gap - gains +13 on _both_ models. Axis B gains +24 on `qwen3.5-9b-q8` and
0 on `bonsai2-27b`, which already had 92 there. That split matches the
distinction worth keeping: some of the prompt supplies **information**
(today's date, what the catalogue means), and that travels; some of it
**compensates for a weakness** (don't retreat, don't drop the filter), and
that only helps the model with the weakness.

## Consequence for how a model swap should be judged

Requiring a new model to pass the incumbent's recorded baseline, under the
incumbent's wording, measures the wrong thing. The procedure this round
implies:

1. derive a wording for the candidate with the same budget of attempts the
   incumbent got (four rounds, over days, most of them negative);
2. measure both models under their own wordings;
3. re-accept a baseline per model - accepting a baseline is a human act
   (`make eval-accept`), and it is part of a model swap, not an obstacle
   to it.

`ORCHESTRA_PICK_WORDING` (`be5bdcd`) and `ORCHESTRA_PLANNER_WORDING` make
step 1 possible without touching what the incumbent uses: both are named,
selectable sets, and `v1`/the current default stay byte-identical.

## What this round does not settle

- Whether a wording tuned for `bonsai2-27b` reaches 34/34. One attempt
  moved nothing; the incumbent needed four rounds.
- Whether the four eval rows `bonsai2-27b` loses are reachable by wording
  at all. Their shape (three escapes into `list_capabilities`, one form
  for an operation that cannot act) says maybe; the failed first attempt
  says not obviously.
- The corpus numbers here are one run each. The 7-row determinism band
  recorded 2026-09-16 applies: a difference of 3 points is inside it, a
  difference of 12 or 13 is not.

## Raw data

`ablation/out/{model}-{wording}-{eval,corpus}.txt` in the session
scratchpad; the corpus runs also wrote
`e2e/shortlist/out/on-{v1,default}-stages2-model-*.{jsonl,log}`.
