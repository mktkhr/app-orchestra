# Jev: where it wins, where it is unnecessary — the conditions map

Purpose: this file is the proof-of-concept's own index of **conditions**,
not a recommendation. For each role Jev was put in, it records the exact
setup the number came from and the boundary at which the result flips.
Per-round detail stays in the individual records; this file is what to
read first.

Written 2026-09-18. Rounds covered: `jev-picker-v1.md`, `jev-picker-v2.md`,
`jev-gate-v3.md`, `jev-language-v4.md`, `jev-v5.md`, `jev-chip-order.md`,
`jev-confidence.md`, `jev-thresholds.md`, `latency-bench.md`,
`jev-hybrid.md`, `jev-field-report.md`, `jev-full-catalogue.md`,
`jev-service-router.md`.

## 0. The bench

| item             | value                                                                                                                                                                                     |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| machine          | Linux 7.0.0-30-generic x86_64, Node v24.15.0                                                                                                                                              |
| GPU              | one RTX 4080 SUPER, 16 GB — every local model shares it                                                                                                                                   |
| local models     | `qwen3.5-9b-q8` (planner), `e5-large-q8` (embedding), `bge-reranker-v2-m3-q8` (rerank), all resident together in llama-swap's persistent `narrowing` group on `:11435` (~14 GB)           |
| determinism      | temperature 0, `max_tokens` 1024 (fill) / 200 (pick), thinking off, `--cache-reuse` removed from the qwen entry (2026-09-17), `ORCHESTRA_PLANNER_TODAY=2026-09-16` pinned by every runner |
| planner defaults | two stages (pick then fill), wording `v6-unmatched-filter`, narrowing on at K=20                                                                                                          |
| Jev              | `jev-latest`, `POST https://api.typesafe.ai/v1/systemone`, $0.042/MTok input, output free; client timeout 10 s, ambiguity/route threshold default 0.5                                     |

Every Jev role below is config-gated and off unless named:
`ORCHESTRA_PICKER` (`jev`/`hybrid`), `ORCHESTRA_GATE=jev`,
`ORCHESTRA_SERVICE_ROUTER=jev` (+ `_THRESHOLD`, `_CRITERIA`),
`ORCHESTRA_JEV_CRITERIA` (`v1`/`v2`).

## 1. The four instruments, and what a point on each is worth

| instrument                    | catalogue                                   | questions                                                   | runs each         | what a loss means                                              |
| ----------------------------- | ------------------------------------------- | ----------------------------------------------------------- | ----------------- | -------------------------------------------------------------- |
| corpus (`shortlist/run.ts`)   | 1000 ops, 5 services (fixture)              | 100, axes A-E                                               | 1 (deterministic) | a research signal at a scale the product does not serve yet    |
| mid (`--corpus mid`)          | 30 ops, 3 services (fixture)                | 60 — 40 answerable, 20 impossible                           | 1                 | the middle case: collisions exist, refusal is testable         |
| eval (`eval/run.ts`)          | 6 ops, 2 services (**real** dummy services) | 34 cases                                                    | **10**            | a behaviour the product shows today, pinned in `baseline.json` |
| dialogues (`dialogue/run.ts`) | same 6 ops                                  | 12 conversations, 27 turns, turns chained from real answers | 1                 | a follow-up the product gets wrong today                       |

A `-2/34` on eval is two _kinds_ of question breaking 10 times out of 10,
on the catalogue that actually ships. A `+4/100` on the corpus is four
questions out of a hundred at a scale that does not exist yet. They are
not the same currency.

## 2. Role by role

### 2a. Jev picks the operation — loses at every candidate count

| setup                                                        | corpus correct@1                             |
| ------------------------------------------------------------ | -------------------------------------------- |
| local pick, narrowing K=20                                   | 77-78                                        |
| Jev pick, same 20 candidates, v1 criteria                    | 69                                           |
| Jev pick, same 20, v2 criteria (`what`/`examples`/`not_for`) | 69                                           |
| Jev pick, whole 1000-op catalogue, hierarchical              | 27 as built; **70** with built-ins set aside |

Conditions: two stages, fill always local, one call per question.
Boundary: **candidate count does not move the ranking.** v2's richer
criteria fixed v1's homonym refusals (11→1) and bought an equal number of
new `list*`/`get*` confusions. Translating the 50 most-affected rows to
English scored _lower_ than Japanese (31/30 vs 36/35 over four passes,
200 calls, $0.0185), so the losses are the task's shape, not the language.

### 2b. Jev names the service, local picks inside it — the one win

| criteria form                                             | routed | of those correct | corpus correct@1  | tokens/call | route ms |
| --------------------------------------------------------- | ------ | ---------------- | ----------------- | ----------- | -------- |
| `names` (display name + a few operation summaries)        | 44/100 | 43               | 77 (= local)      | 621         | 230      |
| `ops` (display name + **every** operation's display name) | 93/100 | **93**           | **81** (local 77) | 8,133       | 398      |

Conditions: `ORCHESTRA_SERVICE_ROUTER=jev`, threshold 0 (route whenever a
service outranks the catch-all `other`); one Choice question holding the
5 services plus `other`, no built-ins; fail-open on error, on `other` and
below the threshold; narrowing and pick unchanged behind it. Corpus, 100
questions, one call each, $0.034 per run.

Per-axis routing accuracy under `ops`: A 25/25, B 25/25 (cross-service
homonyms - the local pick's own weakest axis), C 25/25, D 13/15, E 10/10;
the 7 it declined were `other`, which costs nothing.

**Boundaries found:**

- **Evidence in the options is the switch.** Same question, same model,
  same everything: with thin service descriptions Jev abstains 56 times
  and the gain is zero; with every operation name listed it commits 93
  times and is never wrong. The hierarchical whole-catalogue request's
  service decision (90/98) had the same evidence attached, which is why it
  read high there too.
- **Catalogue size decides whether the win exists.** 1000 ops / 5 services:
  +4. 30 ops / 3 services: -1 (36/40 against 37/40), refusals unchanged.
  6 ops / 2 services: -2 on eval, both rows 10/10 consistent.
- **A conversation lowers the confidence, and the threshold is the safety
  valve.** In dialogue d04 (`att-003を見せて` → `itm-004は？`) the router
  answered `attendance` at confidence 0.29 on the second turn, the
  catalogue lost the inventory operation, and the turn answered `none`.
  At threshold 0 the dialogues read 25/27; at the default 0.5 the same
  route is declined and they read **26/27**, equal to local. Turn one of
  the same dialogue routed at 0.60.
- **The remaining eval losses are not confidence failures.** At threshold
  0.5, `real-capability-inventory` (在庫で何ができる？) and
  `no-enum-value-attendance` (有給の勤怠はある？) still break. Narrowing
  to the _right_ service is what changes their answer: with 3 of 6
  operations visible, the built-in choice and the unmatched-enum ask both
  move. Nothing about the route is wrong.

### 2c. Jev as a refusal gate (`noul`) — partial, and only where the verb is missing

Conditions: `ORCHESTRA_GATE=jev`, threshold 0.7, fail-open, local pick
behind it. Corpus: a true no-op (max noul 0.31, zero refusals). mid: the
印刷/print row correctly refused at 0.86, while 集計/aggregate (0.16) and
承認/approve (0.37) stayed far below the line. eval: one false refusal
(`real-inventory-list-graph`, noul 0.78).
Boundary: it detects "this verb does not exist in the catalogue" only when
the verb is lexically absent; a noun/verb collision (承認 as a state) is
invisible to it.

### 2d. Confidence-routed hybrid — no end-to-end gain, for a structural reason

Conditions: `ORCHESTRA_PICKER=hybrid`, Jev first at an 800 ms timeout,
used only above 0.7 confidence, fail-open. Correctness held (corpus
78/78, mid a wash, eval 34/34) and delegation ran at 27-45% rather than
the predicted ~50%.
Boundary: **the pick is not the bottleneck.** See 2e.

### 2e. Latency and concurrency — measured, with the parallelism written down

Method (`latency-bench.md`, 2026-09-17): four levels of **requests in
flight** — 1, 2, 4, 8 — **40 requests per level per target**, sent in
back-to-back batches of `level` via `Promise.all`; throughput is that
level's 40 requests divided by its own wall-clock. Zero failures or
timeouts at any level. Targets: `jev` (the pick call alone), `local_pick`
(the local pick call alone), `local_end_to_end` (a full `/api/plan`).

| target           | level 1 p90 | level 2 p90 | level 4 p90 | level 8 p90 | throughput 1 → 8 (req/s) |
| ---------------- | ----------- | ----------- | ----------- | ----------- | ------------------------ |
| jev              | 292.4       | 270.3       | 313.1       | 359.0       | 3.90 → **18.70**         |
| local_pick       | 138.2       | 245.7       | 470.6       | **931.4**   | 7.70 → 8.48              |
| local_end_to_end | 728.6       | 1118.3      | 1899.2      | 3485.6      | 1.39 → 2.29              |

Batch size (several questions in one Jev call, sizes 1/2/4/8/13): mean
244-283 ms throughout — **adding questions to a call is nearly free**.

Boundaries:

- Jev's latency is flat in candidate count (20 → 1000 options: 1,131 ms →
  715 ms mean in the same round) and nearly flat in concurrency, because
  nothing local is contended.
- The local pick's p90 overtakes Jev's between level 2 and level 4 on one
  GPU, and its throughput plateaus at ~8.5 req/s from level 2 - the single
  4080 SUPER is the shared resource.
- Replacing only the pick cannot move end-to-end throughput: at level 8
  the full plan costs 3,485 ms p90 while the pick is 931 ms of it, and the
  fill still runs on the same GPU. The hybrid round measured exactly that
  - both builds held ~0.7 req/s end to end on the real services.

### 2f. Display-only uses — the cheapest place a win costs nothing

Ordering the 「違いましたか？」 chips by Jev's own recorded probability
distribution (offline, zero new calls) moves `correct@shown` 79 → 85 at
two chips, 86 at three or with built-ins excluded, with zero rows lost.
Not measured live; it needs one call per question and changes display
order only, never the answer.

## 3. The map, in one table

| condition                   | Jev helps                                                | Jev is unnecessary                                                                      |
| --------------------------- | -------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| granularity of the decision | coarse (which of 5 services)                             | fine (which of 20 operations)                                                           |
| evidence in the options     | every operation name listed                              | a name plus a few summaries                                                             |
| catalogue size              | hundreds/thousands of operations, several services       | a handful of operations, two services                                                   |
| question shape              | single question, names its domain                        | a follow-up that leans on prior turns (confidence drops; the 0.5 threshold declines it) |
| what is being asked of it   | name a class, rank candidates, judge yes/no              | produce the answer itself                                                               |
| load                        | many requests in flight (flat p90, throughput scales)    | one request at a time (local pick is faster: 138 ms p90 vs 292 ms)                      |
| which stage                 | a stage that does not own the GPU                        | any single stage, if the goal is end-to-end speed — the fill dominates                  |
| language                    | Japanese is fine (English scored lower on the same rows) | —                                                                                       |

## 4. Costs actually spent

| round                                                       |  calls | input tokens |             USD |
| ----------------------------------------------------------- | -----: | -----------: | --------------: |
| v1-v5 + follow-ups (2026-09-17)                             | ~1,500 |            — |          0.2057 |
| whole catalogue, hierarchical (2026-09-18)                  |    100 |    3,394,368 |          0.1428 |
| service router, `names` + smoke                             |    102 |       63,310 |          0.0027 |
| service router, `ops` (corpus)                              |    100 |      813,268 |          0.0342 |
| service router, `ops` (mid, eval ×2, dialogues ×2, re-runs) |   ~800 |     ~450,000 |          ~0.019 |
| **cumulative**                                              |        |              | **~0.46 of $2** |

### 2g. Jev fills the enum arguments — the only end-to-end speed win

Measured 2026-09-18, full record in `jev-fill-enum.md`. Replacing the
fill model call (not the pick) with one Jev request for an all-enum
operation: dialogues 25-26/27 against local's 26/27 at **684-744 ms mean
against 1,062 ms**, `make eval` 29-33/34 depending on which escape hatches
the request offers. The control arm with no Jev at all (skip the fill for
a parameterless operation) reads the same trade: corpus latency 1,363 →
731 ms, mid refusals 16/20 → 12/20.

Boundaries: the fill is the stage that owns the GPU, so this is where
latency moves; whatever replaces it must be able to answer "wrong
operation" or it forces every wrong pick; and a whole-request judgement
needs its own question _with the operation's own text in it_ (0.44-0.55
without, usable with - see `jev-fill-enum.md`). Eligibility is a hard
gate: 18 of 34 eval cases, 0 of the 100-question corpus.

## 5. Not measured yet (the PoC's open ground)

1. **The fill arm under concurrency.** Every fill figure above is a
   sequential run; `latency-bench.md` measured the local stack saturating
   at 4-8 requests in flight, which is where removing a GPU call should
   matter most.
2. **Impossible-question detection with the evidence attached.** The v3
   gate saw only instructions; 2b showed that listing every operation is
   what made a decision reliable. mid's 20 impossible questions are the
   instrument.
3. **Fabrication checking** (a non-id field holding an id, a value the
   question never gave) - the vendor's own "guardrail another model"
   pattern, and an open defect from the dialogue instrument.
4. **Chip ordering, live** (2f, offline only so far).
5. **Multi-turn routing, separated** - today's dialogue reading is "equal
   because it abstained", not "equal because it decided well".
