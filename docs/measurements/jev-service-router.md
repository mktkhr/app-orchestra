# Jev as a service router, before local narrowing

Measured 2026-09-18, after `docs/measurements/jev-full-catalogue.md`.
Repository at `0d6dd80` (`010f9dc` the router, `0d6dd80` its second
criteria form).

## The hypothesis this round tests

The whole-catalogue round found Jev's **operation** decision below the
local picker (70 against 78 with built-ins set aside) but its **service**
decision right in 90 of 98 rows and 25 of 25 on axis B, the cross-service
homonyms. So: let Jev name the service, and leave the existing local
narrowing (e5-large + bge-reranker, K=20) and the local pick to work
inside that one service.

## What was built

A `usecase.ServiceRouter` port called in `Orchestrator.Plan` **before**
`Narrow`, off by default:

- `ORCHESTRA_SERVICE_ROUTER=jev` turns it on (unset/`none` is off and
  byte-identical to before).
- `ORCHESTRA_SERVICE_ROUTER_THRESHOLD` (default 0.5) is the confidence
  below which the route is ignored.
- `ORCHESTRA_SERVICE_ROUTER_CRITERIA` picks how a service describes
  itself: `names` (default - display name plus the first few operation
  summaries, the same text the hierarchical request used) or `ops` (the
  display name plus **every** exposed operation's display name).
- One Choice question, one option per service plus a catch-all `other`.
  No `none`/`list_capabilities`/`propose_panel`: this stage cannot answer
  a question, only name a service.
- **Fail open everywhere**: a transport error, `other`, an unknown
  service, or a confidence below the threshold all leave the catalogue
  exactly as it was. The stage can only narrow, never refuse.

## The corpus: the criteria form is what decides it

100 questions, two stages, local pick, threshold 0 (route whenever a
service outranks `other`):

| configuration              | correct@1 | correct@shown | mean ms |
| -------------------------- | --------- | ------------- | ------- |
| local only (the default)   | 78        | 80            | 1290    |
| router, `names` criteria   | 78        | 80            | 1558    |
| **router, `ops` criteria** | **82**    | **84**        | 1775    |

Routing behaviour, same two runs:

| criteria | named a service | of those, correct | mean confidence | tokens/call | route ms |
| -------- | --------------- | ----------------- | --------------- | ----------- | -------- |
| `names`  | 44 / 100        | 43                | 0.751           | 621         | 230      |
| `ops`    | **93 / 100**    | **93**            | 0.851           | 8,133       | 398      |

Not one wrong route in the `ops` run. The seven it declined were five
axis-B and two axis-D questions, answered `other`, which fails open to
today's behaviour.

Per axis, where the route was named and correct:

| axis                      | `names` | `ops`     |
| ------------------------- | ------- | --------- |
| A (plain lists)           | 5/25    | **25/25** |
| B (cross-service homonym) | 5/25    | 20/25     |
| C (near neighbours)       | 18/25   | **25/25** |
| D (vocabulary gap)        | 9/15    | 13/15     |
| E (setting decoys)        | 6/10    | **10/10** |

The `names` form asks Jev which service fits while telling it almost
nothing about what each service can do; it answers `other` 56 times. The
`ops` form is the same question with the evidence attached - and the
service question inside the whole-catalogue request had that evidence
too, which is why that round's service decision read 90/98.

Threshold sweep on the `ops` run (offline, from the logged distribution):
93 routed / 93 correct at 0 or 0.3, 88/88 at 0.5, 84/84 at 0.6, 76/76 at
0.7. Raising it only declines more questions; it corrects nothing,
because nothing was wrong.

## The other instruments: the gain does not survive a small catalogue

| instrument | catalogue       | local only           | router, `ops`                          |
| ---------- | --------------- | -------------------- | -------------------------------------- |
| corpus     | 1000 ops, 5 svc | 78 / 80              | **82 / 84**                            |
| mid        | 30 ops, 3 svc   | 37/40, refused 16/20 | 37/40, refused 15/20 (a wash)          |
| eval       | 6 ops, 2 svc    | **32/34**            | **32/34** at threshold 0, 31/34 at 0.7 |

**Corrected 2026-09-18, after this file was first written.** The eval
column originally read `34/34` for local and blamed the router for two
rows. It was wrong: `make eval` had not been run against the local
default after `eeb23fa` (offering `propose_panel` to the pick only with a
workspace). Run since, with the router off, today's tree reads **32/34** -
`unanswerable` (今日の天気は？) and `real-what-day` (今日は何曜日？) both
answer `list_capabilities` where `baseline.json` records `none`, because
removing one built-in from the pick's candidate list moves those two
near-tie rows. Isolated by hand: the same two questions answer `none`
when the request carries a `workspaceId` (so `propose_panel` is offered)
and `list_capabilities` when it does not.

Against that correct baseline the router costs **one** row, not two:
at threshold 0 it reads 32/34 with a different pair - `real-what-day` and
`real-capability-inventory` (在庫で何ができる？, `list_capabilities` at
baseline, `none` under the router) - and `unanswerable` recovers. At
threshold 0.7 it reads 31/34.

The remaining router-caused row is still the shape described above: with
two services and six operations, narrowing to one service changes what
the fill can see, and a built-in answer flips. The router names a service
confidently for a question that has none, because its only options are
the services and `other`.

## Cost and latency

- `ops` criteria: 8,133 input tokens per question (813,268 for the
  corpus run), **$0.034 per 100 questions**; `names`: 621 per question.
- The route itself costs 398 ms mean (`ops`) or 230 ms (`names`), and
  the end-to-end mean moves 1290 → 1775 ms on the corpus.
- Cumulative Jev spend after this round: about **$0.41** of the $2
  budget (ledger in the session scratchpad).

## Not adopted

`ORCHESTRA_SERVICE_ROUTER` stays unset. On the catalogue the product
actually serves today - two services, six operations - the router costs
one `make eval` row (see the correction above) and gains nothing; the +4
it wins is on the 1000-operation fixture, which is where the product is
meant to end up but is not where it is. Whether that one row is worth +4
is a decision to take once the pick's own built-in wording is settled,
since two of the three rows in this area are moving for a reason that has
nothing to do with the router.

## Open, in `TODO.md`

- Route only above a catalogue size (operations or services), so a small
  catalogue keeps today's behaviour and a large one gets the +4. The
  threshold would have to be chosen from measurement, not guessed.
- Or require a margin over `other` rather than an absolute confidence -
  the three broken rows are questions where `other` should have won.

## Raw data

`jev-service-router-routes.jsonl` - 200 rows, one per question per
criteria form: the question, its answer key, the platform's answer, the
route chosen, its confidence, the full probability distribution over
services and `other`, per-call tokens and latency.
