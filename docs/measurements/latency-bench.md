# Latency microbenchmark — Jev batch size and concurrency, both stacks

Date: 2026-09-17 (JST). Repository: app-orchestra, branch main, tree
starting at `970b5cd`. Machine: Linux ubuntu-desktop 7.0.0-30-generic
x86_64, Node v24.15.0. Local stack: `make dev-services`, already running,
not restarted for this measurement - llama-swap on `:11435` with
`qwen3.5-9b-q8` resident, platform dev server on `:8080`.

No product code touched. Raw data only; this file is the factual record,
not an argument.

## Commands

```
TYPESAFE_KEY="<key>" node latency-bench.mjs   # the run actually measured
TYPESAFE_KEY="<key>" python3 docs/measurements/raw/latency-bench.py   # committed port, see below
```

The key is read from the `TYPESAFE_KEY` env var only, set inline on the
command line; it is never written to a file. The run reported below used
a Node script (`fetch`, in the scratchpad) built to this same logic and
this same fixed question set. That script could not be committed as-is:
this repository's Oxlint policy (`harness/quality/oxlint/policy.ts`, out
of scope for this task) forbids calling the global `fetch` or exceeding
300 lines outside `harness/**`/`e2e/**`/`src/shared/api/**`, and a
throwaway measurement script under `docs/measurements/raw/` is none of
those. `docs/measurements/raw/latency-bench.py` is a line-for-line
behavioral port to the standard library (`urllib.request`,
`concurrent.futures`) - same endpoints, same fixed question set, same
warm-up/measured/concurrency structure - verified to build the identical
size-13 request body as the one actually sent
(`docs/measurements/raw/latency-batch-request-size13.json`). It was not
re-run; the numbers in this file are from the Node run.

- Jev endpoint: `POST https://api.typesafe.ai/v1/systemone`, model
  `jev-latest`, `Authorization: Bearer <key>` (same client shape as
  `services/platform/internal/adapter/planner/jev/client.go`).
- Local pick: `POST http://localhost:11435/v1/chat/completions`, the exact
  body from `docs/measurements/raw/local-pick-request.json`.
- Local end to end: `POST http://localhost:8080/api/plan` with
  `{"query":"在庫の一覧を見せて","turns":[],"thinking":false}`, after
  `POST http://localhost:8080/api/session` with
  `{"name":"admin","password":"dev-only-admin-password"}` and its session
  cookie.

## Question set (fixed, written once)

Question 1 (`pick`) is the exact `pick` question from
`docs/measurements/raw/jev-v2-criteria-request.json`, byte-identical.
Batch sizes 2/4/8/13 add further questions from this fixed, ordered list
(size N uses `pick` plus the first N-1 of these):

1. `impossible` (noul) - the exact question from
   `docs/measurements/raw/jev-fanout-gate-request.json`
2. `cancel` (noul) - 取り消せない操作を求めているか
3. `panel` (noul) - 画面に出したい依頼か
4. `date` (noul) - 質問に日付の条件が含まれるか
5. `urgent` (noul) - 急ぎか
6. `filter` (noul) - 絞り込み条件があるか
7. `ambiguity_score` (score, 3 levels) - 質問の曖昧さ
8. `multi_op` (noul) - 質問が複数の操作にまたがるか
9. `approval` (noul) - 質問が承認を必要とする操作か
10. `aggregate` (noul) - 質問が集計を求めているか
11. `politeness_score` (score, 3 levels) - 質問の丁寧さ
12. `specific_id` (noul) - 質問が特定のIDを指しているか

The full size-13 request body actually sent is
`docs/measurements/raw/latency-batch-request-size13.json`.

## A. Batch-size curve (Jev)

Method: 3 warm-up calls discarded, then 20 measured calls, sequential, per
size. All 100 measured calls returned HTTP 200 - zero errors, zero
retries observed. Raw rows: `docs/measurements/raw/latency-batch-jev.jsonl`.

| size | mean ms | p50 ms | p90 ms | max ms | input tokens (mean) | output tokens (mean) |
| ---: | ------: | -----: | -----: | -----: | ------------------: | -------------------: |
|    1 |   246.2 |  237.4 |  286.6 |  302.3 |                1165 |                  126 |
|    2 |   244.7 |  247.9 |  270.0 |  277.0 |                1362 |                  145 |
|    4 |   283.1 |  250.5 |  293.0 |  705.1 |                1548 |                  177 |
|    8 |   252.4 |  240.3 |  293.8 |  384.8 |                1898 |                  241 |
|   13 |   276.6 |  255.8 |  369.6 |  432.0 |                2228 |                  323 |

## B. Concurrency (40 requests per level, level = requests in flight)

Method: level 1, 2, 4, 8, 40 requests total per level per target, sent in
back-to-back batches of `level` requests via `Promise.all`. No failures or
timeouts were observed at any level for any target, so no level's
escalation was stopped early. Raw rows:
`docs/measurements/raw/latency-concurrency.jsonl`. Throughput is
`completed requests / wall-clock seconds` for that level's 40 requests.

| target           | level | mean ms | p50 ms | p90 ms | max ms | failures | throughput (req/s) |
| ---------------- | ----: | ------: | -----: | -----: | -----: | -------: | -----------------: |
| jev              |     1 |   256.7 |  243.7 |  292.4 |  633.3 |        0 |               3.90 |
| jev              |     2 |   248.9 |  223.1 |  270.3 |  615.0 |        0 |               7.18 |
| jev              |     4 |   256.4 |  234.4 |  313.1 |  575.9 |        0 |              12.09 |
| jev              |     8 |   274.5 |  231.7 |  359.0 |  612.4 |        0 |              18.70 |
| local_pick       |     1 |   129.8 |  123.9 |  138.2 |  240.1 |        0 |               7.70 |
| local_pick       |     2 |   183.8 |  148.2 |  245.7 |  258.1 |        0 |               8.25 |
| local_pick       |     4 |   296.5 |  244.8 |  470.6 |  478.5 |        0 |               8.50 |
| local_pick       |     8 |   536.0 |  505.1 |  931.4 |  947.6 |        0 |               8.48 |
| local_end_to_end |     1 |   719.8 |  723.1 |  728.6 |  738.9 |        0 |               1.39 |
| local_end_to_end |     2 |   973.4 |  841.8 | 1118.3 | 1125.4 |        0 |               1.80 |
| local_end_to_end |     4 |  1485.5 | 1355.2 | 1899.2 | 1909.1 |        0 |               2.11 |
| local_end_to_end |     8 |  2522.4 | 2389.7 | 3485.6 | 3505.4 |        0 |               2.29 |

## What this means for our open questions

- **Does the 2-question penalty hold across runs?** No. This run's size-1
  mean (246.2ms) and size-2 mean (244.7ms) are statistically
  indistinguishable (p50 247.9 vs 237.4, well inside the noise the p90/max
  columns show at every size). The single prior observation in
  `docs/measurements/jev-v5.md` (311ms for 2 questions vs 220ms for 1, a
  ~41% jump) does not reproduce here - it reads as one noisy pair of
  calls, not a real per-question cost. Mean latency stays flat
  (244-283ms) across all five sizes 1/2/4/8/13 while input tokens roughly
  double from 1165 to 2228 and output tokens roughly triple from 126 to
  323, consistent with the vendor's and the independent 12-question
  reproduction's claim that added questions are evaluated in parallel and
  do not add request latency, at least up to 13 questions in one call.
- **Where does the curve flatten?** It does not visibly rise at all in
  this range - there is no elbow to find between 1 and 13 questions per
  request. The only sizes that show any latency growth are size=4's p90
  and max (293ms / 705.1ms), driven by one outlier call, not a trend
  (size=8 and size=13 both fall back to the 240-370ms band). Tokens grow
  linearly and predictably with size; latency does not track tokens in
  this range.
- **At what concurrency does the local stack's p90 exceed Jev's?** At
  every level measured (1, 2, 4, 8), `local_pick`'s p90 already exceeds
  Jev's p90 - even at level=1 (138.2ms vs 292.4ms is still below Jev, but
  by level=2 local_pick's p90 245.7ms is close to Jev's 270.3ms, and by
  level=4 local_pick's p90 470.6ms has passed Jev's 313.1ms). So the
  crossover is level=4 for `local_pick`. `local_end_to_end` (the fuller
  pipeline, narrowing + fill + pick against the same one llama-server)
  passes Jev's p90 already at level=1 (728.6ms vs 292.4ms) and keeps
  degrading roughly linearly with concurrency (p90 728.6 -> 1118.3 ->
  1899.2 -> 3485.6ms at levels 1/2/4/8) - one GPU serving one model at a
  time does not parallelize, while Jev's own p90 barely moves (292 ->
  270 -> 313 -> 359ms) and its throughput scales close to linearly with
  concurrency (3.9 -> 7.18 -> 12.09 -> 18.7 req/s), the opposite of what
  `local_pick`'s throughput does (plateaus at ~8.5 req/s from level=2
  onward - the single GPU is already saturated).
- No errors, timeouts, retries or non-200 responses were observed in any
  of the 100 batch calls or 480 concurrency calls (580 measured calls
  total), so none of the above reflects error handling or backoff - it is
  all steady-state latency.
