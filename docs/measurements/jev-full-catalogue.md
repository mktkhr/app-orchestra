# Jev with the whole catalogue: the vendor's own recommended shape

Measured 2026-09-18. Repository at `eeb23fa` (`3c3f7b9` hierarchical
request, `d9168ea` runner flag, `eeb23fa` the propose_panel fix below).

Every Jev round before this one (v1-v5 and the follow-ups) gave Jev the
local narrowing stage's own 20-candidate shortlist. TypeSafe's
documentation recommends the opposite: "giving the model the full list of
teams, categories, or products rather than a shortlist"
(`docs.typesafe.ai/primitives/choice`, read 2026-09-18), with a ceiling of
255 options per Choice question and, above that, the hierarchical
classification cookbook's staged Choice questions scored by the geometric
mean of each edge's probability
(`docs.typesafe.ai/cookbooks/hierarchical_classification.md`). The
255 figure had been recorded as **unverified** in `jev-field-report.md`;
it is now confirmed from the primary source.

This round asks the one question no earlier round asked: what does Jev do
when it is given the whole 1000-operation catalogue instead of a
shortlist?

## What was built

`internal/adapter/planner/jev/hierarchical.go` (`3c3f7b9`): when a
shortlist's endpoints plus the built-in options exceed 255, one request
carries

- a `service` Choice question - one option per service present, plus the
  same built-ins the flat request offers, and
- one `op_<service>` Choice question per service - that service's own
  endpoints only, no built-ins,

all in one HTTP call. An endpoint's score is `sqrt(P(service) *
P(op|service))`; a built-in's is `P(builtin)` off the service question;
the maximum wins. At or below 255 options the flat request is unchanged,
byte for byte.

## A defect found by the two-question smoke test, and fixed first

The first two questions sent through the hierarchical path both came back
`propose_panel`, at p=0.79 and p=0.70, from the service question. A pick of
`propose_panel` falls back to `planOrdinary` over the whole catalogue
(`orchestrator_staging.go`), so the answer came from the local model, not
from the pick under test - and the first such question took 27.6 s.

Cause: the pick stage offered `propose_panel` on every question, while
`usecase.ToolsFor` offers it only when the request carries a workspace
(`docs/specs/offering.md`). Every picker had the same blindness - the Jev
flat and hierarchical paths and the **local** picker
(`internal/adapter/planner/pick`). Fixed in `eeb23fa`: `usecase.Picker.Pick`
now takes `PlanContext`, and each adapter omits `propose_panel` - from its
options and from its instructions text - when no workspace is present. A
workspace-present request is asserted byte-identical to before.

Measured effect of the fix alone, on the 100-question corpus (two stages,
narrowing on, K=20):

| picker                | before the fix | after the fix |
| --------------------- | -------------- | ------------- |
| local (the default)   | 78 / 79        | **78 / 80**   |
| Jev, flat, 20 options | 69 / 72        | **69 / 71**   |

(correct@1 / correct@shown.) The fix moves no score outside the
measurement's own near-tie band; it removes a path by which the local
model was silently answering some of the rows a Jev run was crediting to
Jev.

## The full-catalogue run

`node shortlist/run.ts --stages 2 --narrowing off` with
`ORCHESTRA_PICKER=jev`; 100 questions, one call each, v1 criteria.

| configuration             | correct@1 | correct@shown |
| ------------------------- | --------- | ------------- |
| local, narrowing on, K=20 | 78        | 80            |
| Jev, narrowing on, K=20   | 69        | 71            |
| **Jev, whole catalogue**  | **27**    | **27**        |

Answer kinds: `none` 58, `result` 32, `form` 10.

**Why 27.** Jev chose the built-in `none` at the service level in 58 of
100 questions, at a mean probability around 0.85. A built-in is one
decision and an operation is two, so the built-in's raw probability is
compared against a geometric mean of two probabilities - the cookbook's
own depth-fair formula, which here systematically favours the built-in.
This is a property of the shape as built, not evidence about Jev's
ability to find the operation.

## What the same run says with the built-ins set aside

Every pick logs its top three paths (`pick_top3_paths`), so the ranking
below the built-in is recoverable without another call. Taking the best
**non-built-in** path as the answer (exact for 98 of 100 rows; 2 rows had
only built-ins in the top three):

| axis                      | Jev, whole catalogue | local, K=20 | Jev, K=20 |
| ------------------------- | -------------------- | ----------- | --------- |
| A (plain lists)           | 16/25                | 22/25       | 19/25     |
| B (cross-service homonym) | **23/25**            | 21/25       | 18/25     |
| C (near neighbours)       | 14/25                | 16/25       | 17/25     |
| D (vocabulary gap)        | **11/15**            | 10/15       | 7/15      |
| E (setting decoys)        | 6/10                 | 9/10        | 8/10      |
| overall                   | 70                   | 78          | 69        |

(The per-axis columns for the two K=20 configurations are this same run's
own report, converted from its percentages.)

**The service question is where the signal is.** The chosen path's
service matched the answer's own service in **90 of 98** rows, and in
**25 of 25** on axis B - the cross-service collisions (受注/発注 both
displayed as 注文) that the local pick gets wrong and that a 20-candidate
shortlist can silently drop. Axis D, the vocabulary gap, also reads above
both K=20 configurations.

Where the whole catalogue loses is axes A and E, exactly where the
reranked 20-candidate shortlist hands the picker an easy, pre-filtered
list.

## Cost and latency

- 33,944 input tokens per call (mean), 3,394,368 for the run;
  **$0.143** at $0.042/MTok. Output tokens are free.
- Jev's own pick latency: mean 715 ms, p50 707 ms - against 1,131 ms
  mean for the same adapter on 20 candidates in the pre-fix run and
  244-283 ms measured in `latency-bench.md` on small requests. Fifty
  times the candidates costs well under double the time, and the request
  itself (72,698 bytes at v1 criteria, 225,296 at v2 - measured offline
  before the run) is nowhere near a limit the API documents; no size or
  rate error occurred.
- Cumulative Jev spend after this round: **$0.3558** of the $2 budget
  (session ledger in the scratchpad; $0.2057 carried in).

## Not adopted

`ORCHESTRA_PICKER` stays unset: the local picker remains the pick stage.
The hierarchical request stays in the tree behind the same config, used
only when a shortlist exceeds 255 options.

## What this round did not measure

- The mid instrument and `make eval` under the whole catalogue (mid's 30
  operations fit one flat question; the eval suite's 6 already do).
- v2 criteria under the hierarchical shape.
- A run with the built-ins removed from the service question, which is
  what the 70 above estimates offline.

## Raw data

`jev-full-catalogue-picks.jsonl` - one row per question: the question, its
answer key, the platform's answer, Jev's chosen option, the service and
operation probabilities, the path score, the top three paths, per-call
tokens and latency.
