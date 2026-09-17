# Hybrid Jev/local picker — corpus, mid, eval suite and concurrency

Date: 2026-09-17 (JST). Repository: app-orchestra, branch main, tree
starting at `6d79378`. Local stack: `make dev-services`, already running,
not restarted for this measurement - llama-swap on `:11435` with
`qwen3.5-9b-q8`/`e5-large-q8`/`bge-reranker-v2-m3-q8` resident, dummy
services on `:8081`/`:8082`, dev platform on `:8080`.

No product code touched. Raw data only; this file is the factual record,
not an argument.

## What the hybrid picker does

`internal/adapter/planner/hybrid.Picker` composes the Jev picker and the
local picker behind one `usecase.Picker`. It calls Jev first (v2
criteria, turns carried in state, plain instructions), under its own
timeout (`ORCHESTRA_HYBRID_JEV_TIMEOUT`, default 800ms). Jev's answer is
used only when it names a catalogue operation (`Kind == PickOperation`)
and its own reported confidence is at or above
`ORCHESTRA_HYBRID_THRESHOLD` (default 0.7, the same band
`docs/measurements/jev-thresholds.md` found to be calibrated). Every
other outcome - Jev answering with a built-in (`none` /
`list_capabilities` / `propose_panel`), confidence below the threshold, a
timeout, or any error - falls back to the local picker, which always
succeeds or degrades to `PickNone` rather than propagating an error. One
`"pick completed"` line is logged per hybrid decision, carrying
`pick_provider`, `pick_fallback_reason` (`low_confidence` / `builtin` /
`timeout` / `error` / empty), `pick_confidence`, `pick_jev_ms` and
`pick_local_ms` - on top of, not instead of, Jev's and/or the local
picker's own "pick completed" lines from the same call, so a platform log
can carry up to three "pick completed" lines per decision.

Jev is called first on every decision regardless of whether its answer
ends up used - the timeout only bounds how long hybrid waits for it, it
does not skip the call. That matters for the Jev call counts below: every
one of the corpus/mid questions made a real Jev API call, not just the
ones whose answer was kept.

## Commands

The Jev API key is read from its local key file inline on each command
(`<key>` below), never exported or written to a file.

```
PICKER=hybrid ORCHESTRA_JEV_API_KEY="<key>" make eval-shortlist STAGES=2
PICKER=hybrid ORCHESTRA_JEV_API_KEY="<key>" make eval-mid
PICKER=hybrid ORCHESTRA_JEV_API_KEY="<key>" make eval
```

plus, for the concurrency step, a second platform process on `:8095`
built from the same `dev-services` recipe (`Makefile`), with
`ORCHESTRA_PORT=8095 ORCHESTRA_PLANNER_STAGES=2 ORCHESTRA_PICKER=hybrid
ORCHESTRA_JEV_API_KEY=...` and its own `ORCHESTRA_DB_PATH`, pointed at the
same already-running dummy services and llama-swap - never a restart of
the shared `:8080`/`:8081`/`:8082` dev stack.

## Step 1 — corpus eval, STAGES=2

Raw rows: `docs/measurements/raw/hybrid-corpus.jsonl` (copied from
`e2e/shortlist/out/on-default-stages2-hybrid.jsonl`, 100 rows).

| variant                                     | correct@1 | correct@shown |
| ------------------------------------------- | --------: | ------------: |
| local-only (`on-default-stages2`, baseline) |    78/100 |        79/100 |
| hybrid (`on-default-stages2-hybrid`)        |    78/100 |        80/100 |

Row-by-row diff against the baseline's own raw run
(`nocr-stages2.jsonl`, this session's scratchpad copy of
`on-default-stages2.jsonl`): only 3 of 100 rows differ.

| id  | question                             | baseline pick                 | baseline correct | hybrid pick                       | hybrid correct      |
| --- | ------------------------------------ | ----------------------------- | ---------------- | --------------------------------- | ------------------- |
| b18 | 社員を新規登録したい                 | form createExpenseEmployee    | yes              | form createAttendanceEmployee     | yes                 |
| b19 | 社員情報を直したい                   | form updateExpenseEmployee    | yes              | result list_capabilities          | **no (regression)** |
| d11 | 商品が届いたので受け取り処理をしたい | form createInventoryReceiving | no               | form createPurchasingGoodsReceipt | **yes (fix)**       |

b18: both picks are in the row's own `answers` list, so both count as
correct despite naming different operations. b19 is a genuine regression
(hybrid picked a refusal where the local baseline's form pick was
accepted). d11 is a genuine improvement (hybrid's pick is the accepted
answer, the local baseline's pick was not). Net effect on the corpus
score: a wash - one regression, one fix, correct@1 unchanged at 78/100.

Delegation, from grepping `on-default-stages2-hybrid.log` for
`"pick completed"` lines that also carry `pick_jev_ms` (hybrid's own
line - Jev's own "pick completed" line for the same call has no
`pick_jev_ms` field, so this grep isolates hybrid's decision line
cleanly):

| pick_provider | pick_fallback_reason | count |
| ------------- | -------------------- | ----: |
| jev           | (used directly)      |    27 |
| local         | builtin              |    30 |
| local         | low_confidence       |    43 |

100 decisions total, matching the 100-question corpus. No `timeout` or
`error` fallbacks were observed. **Deviation from plan**: only 27% of
decisions used Jev directly, well under the roughly-half rate
`docs/measurements/jev-thresholds.md`'s threshold analysis predicted for
the 0.7 band. Of the 70 decisions where Jev answered a catalogue
operation at all (100 - 30 builtin), 27 (39%) cleared the confidence
threshold - closer to, but still under, the ~50% figure. This is stated
plainly as a deviation, not explained further; the evidence here does not
say why.

## Step 2 — mid corpus (three services, thirty operations, sixty questions)

Raw rows: `docs/measurements/raw/hybrid-mid.jsonl` (copied from
`e2e/shortlist/out/mid-default-hybrid.jsonl`, 60 rows).

Report row format is (answerable correct@1, answerable false refusal,
impossible refused, impossible forced, forms fabricated) -
`e2e/shortlist/report-mid.ts`'s own five-number layout.

| variant               | answerable correct@1 | false refusal | impossible refused | impossible forced | forms fabricated |
| --------------------- | -------------------: | ------------: | -----------------: | ----------------: | ---------------: |
| local-only (baseline) |                38/40 |          1/40 |              17/20 |              3/20 |             0/24 |
| hybrid                |                39/40 |          1/40 |              16/20 |              4/20 |             0/25 |

One more answerable question correct, one more impossible question
"forced" (answered instead of refused) - a small, roughly offsetting
move, not a clear win or loss either way.

Delegation:

| pick_provider | pick_fallback_reason | count |
| ------------- | -------------------- | ----: |
| jev           | (used directly)      |    25 |
| local         | builtin              |    17 |
| local         | low_confidence       |    18 |

60 decisions total, matching the 60-question mid corpus. Again no
`timeout`/`error` fallbacks. Delegation rate 25/60 = 42% of all
decisions, or 25/43 = 58% of decisions where Jev answered a catalogue
operation - closer to the ~half figure than step 1's, but the two steps
together (52 jev / 108 local out of 160) land at 33% overall, still below
the ~50% predicted.

## Step 3 — eval suite baseline

Full runner output: `docs/measurements/raw/hybrid-eval-report.txt`
(copied from this run's captured stdout/stderr, not retyped).

| variant                                      | cases at baseline | regressions | improvements |
| -------------------------------------------- | ----------------: | ----------: | -----------: |
| baseline (`e2e/eval/baseline.json`)          |             34/34 |           - |            - |
| hybrid (`PICKER=hybrid make eval`, 360 runs) |             34/34 |           0 |            0 |

No line moved. Every case's accept/reject/total in the hybrid run is
identical to its `baseline.json` entry (checked field by field, not just
by the absence of `REGRESSION` markers): 33 cases at 10/10 accept and
0/10 reject, `no-enum-value` at 30/30 accept and 0/30 reject, and
`no-enum-value-attendance` (judged on reject) at 0/10 reject.

**No delegation breakdown for this step.** `e2e/eval/services.ts` drains
the platform's stdout/stderr and never writes it anywhere
(`drainStdio`), and this measurement does not change that file, so the
`"pick completed"` lines for these 360 decisions were not kept. The
runner reports only final outcomes, so it cannot tell whether a case was
answered by Jev or by the local fallback.

## Step 4 — concurrency, end to end

Targets, same machine, run back to back (all four hybrid levels, then all
four local levels, about ten minutes in total):

- `hybrid-8095`: a second platform process started for this measurement
  with the `dev-services` recipe's environment plus `ORCHESTRA_PORT=8095`,
  `ORCHESTRA_PLANNER_STAGES=2`, `ORCHESTRA_PICKER=hybrid`, the Jev key
  (inline) and its own `ORCHESTRA_DB_PATH` in the scratchpad. It used the
  same dummy services (`:8081`/`:8082`) and llama-swap as the dev stack.
  It was stopped by its own pid afterwards.
- `local-8080`: the already-running dev platform, not restarted. Its
  process environment has no `ORCHESTRA_PICKER`, `ORCHESTRA_PLANNER_STAGES`
  or `ORCHESTRA_GATE` (read from `/proc/<pid>/environ`, names only), so it
  runs the default STAGES=2 path with the local picker.

Method: `POST /api/plan` with `{"query": q, "turns": [], "thinking":
false}` after `POST /api/session`, following the `local_end_to_end` mode in
`docs/measurements/raw/latency-bench.py`. The client was a scratchpad
port of that script and is not committed. Each level sent 40 requests in
back-to-back batches of `level`. The question set is the 10
catalogue-targeting questions from `e2e/eval/cases-real.ts`, cycled four
times per level: 遅刻を記録したい, 新しい在庫を登録したい, 有給の申請,
att-002の内容, itm-001の詳細, 預託在庫ある？, 代休を取った人,
ネジを100個入庫, 田中さんの勤怠, 在庫の一覧をグラフで. The six refusal
and capability cases were left out. Throughput is completed requests
divided by that level's wall-clock seconds. Raw rows:
`docs/measurements/raw/hybrid-concurrency.jsonl` (320 rows).

All 320 requests returned HTTP 200.

| target      | level | mean ms | p50 ms |  p90 ms |  max ms | failures | throughput (req/s) |
| ----------- | ----: | ------: | -----: | ------: | ------: | -------: | -----------------: |
| hybrid-8095 |     1 |  1651.3 | 1221.3 |  2767.5 |  4294.3 |        0 |               0.61 |
| hybrid-8095 |     2 |  2400.9 | 1784.7 |  3962.2 |  5365.4 |        0 |               0.68 |
| hybrid-8095 |     4 |  3831.8 | 3669.3 |  6555.7 |  8475.9 |        0 |               0.72 |
| hybrid-8095 |     8 |  6839.8 | 7187.3 | 10989.8 | 12047.2 |        0 |               0.75 |
| local-8080  |     1 |  1472.8 |  990.9 |  2367.1 |  4029.2 |        0 |               0.68 |
| local-8080  |     2 |  2343.7 | 1797.7 |  4249.6 |  5522.8 |        0 |               0.70 |
| local-8080  |     4 |  3721.6 | 3074.9 |  7009.3 |  8612.0 |        0 |               0.70 |
| local-8080  |     8 |  7391.2 | 6818.2 | 11427.2 | 12516.0 |        0 |               0.72 |

Hybrid decisions on `:8095` per level, from its own captured log (160
hybrid `"pick completed"` lines for 160 requests):

| level | jev | local / builtin | local / low_confidence | local / timeout |
| ----: | --: | --------------: | ---------------------: | --------------: |
|     1 |  19 |               6 |                     15 |               0 |
|     2 |  18 |               7 |                     15 |               0 |
|     4 |  18 |               6 |                     15 |               1 |
|     8 |  17 |               6 |                     17 |               0 |

72 of 160 decisions (45%) used Jev directly. Per level, the counts are
exact. Each row's `provider`/`fallback_reason` in the jsonl was matched to
a request by completion order within its level. Under concurrency that
matching is best-effort, not a guaranteed per-request link. For
`local-8080`, `provider` is `local` by construction; that platform's log
was not read. Jev's own time per decision (`pick_jev_ms`) had a p50 of
250ms across the 160 decisions. The single `timeout` fallback hit the
800ms bound (`pick_jev_ms` 802) and then spent 1327ms in the local pick:

```
{"time":"2026-09-17T23:31:53.781899565+09:00","level":"INFO","msg":"pick completed","pick_provider":"local","pick_fallback_reason":"timeout","pick_confidence":0,"pick_jev_ms":802,"pick_local_ms":1327}
```

What the table shows:

- End to end, the two configurations are within noise of each other at
  every level. Hybrid is slower at level 1 (p50 1221 vs 991ms), about the
  same at level 2, and slightly faster on p90 at levels 2, 4 and 8. Its p50
  at level 8 is higher (7187 vs 6818ms). Neither is consistently ahead.
- Throughput is flat at about 0.6-0.75 req/s for both targets at every
  level, so neither one scales with concurrency. Both still send every
  question's fill step (and narrowing) to the same single llama-server,
  and hybrid also sends a local pick for the 55% of decisions that fell
  back. Handing 45% of picks to Jev did not change the end-to-end
  saturation point in this measurement.
- These requests are much heavier than `latency-bench.md`'s single
  「在庫の一覧を見せて」: at level 8, local p90 is 11427ms here against
  3486ms there. Most of these questions end in a form or a detail lookup
  that needs a fill call. The two files' absolute numbers are not
  comparable. Within this file, the two targets are.

## Log excerpt

One `pick_provider=jev` line and one fallback line, verbatim, from step
1's platform log (`on-default-stages2-hybrid.log`):

```
{"time":"2026-09-17T23:17:46.168259178+09:00","level":"INFO","msg":"pick completed","pick_provider":"jev","pick_fallback_reason":"","pick_confidence":0.79,"pick_jev_ms":515,"pick_local_ms":0}

{"time":"2026-09-17T23:17:47.680271431+09:00","level":"INFO","msg":"pick completed","pick_provider":"local","pick_fallback_reason":"builtin","pick_confidence":0,"pick_jev_ms":291,"pick_local_ms":430}
```

(saved verbatim at `docs/measurements/raw/hybrid-log-excerpt.txt`)

## Deviations from plan, summarized

- Step 1's Jev-delegation rate (27%) came in well under the ~half figure
  `jev-thresholds.md` predicted for the 0.7 confidence band; step 2's
  (42%) was closer but still under. Stated plainly above, not
  editorialized on.
- Jev is called on every hybrid decision, not only ones whose answer gets
  used - so the real Jev call count for steps 1 and 2 is 100 and 60
  respectively (matching the question counts), not the ~27/~25 that
  actually used Jev's answer.
- Step 3 has no delegation breakdown: the eval runner discards the
  platform log (see step 3).
- Step 4 had the only non-`builtin`/`low_confidence` fallback of the
  round: one `timeout` at level 4. Steps 1 and 2 had none, and step 3
  cannot say.
- Step 4 did not show the end-to-end concurrency win this picker was
  built for. Both targets stayed at about 0.7 req/s, with p90 within
  noise of each other at every level (see step 4).

## Jev spend

680 Jev calls in total: 100 corpus, 60 mid, 360 eval and 160 concurrency.
Every hybrid decision calls Jev, including decisions where the answer is
discarded. Measured from `jev.Picker`'s own token log lines: corpus
112,661 in / 31,996 out, mid 58,043 / 15,122, and concurrency 101,304 /
18,732 over 159 calls. Two parts are estimates. The eval run's 360 calls
had no log and are estimated at the concurrency run's per-call mean
(637 / 118), because both use the same six-operation real catalogue. The
one timed-out concurrency call logged no tokens and is estimated at the
same mean. Total about 502k input / 108k output tokens, about $0.021 at
$0.042/MTok input (output is not billed).

- The two concurrency targets run different builds. The shared `:8080`
  process was started at 22:14, before the hybrid commits (`5e1b6cc`,
  `6d79378`, 23:13), and was not restarted. `:8095` runs the binary
  `make eval` rebuilt from `6d79378`. Those commits leave
  `internal/adapter/planner/pick` and `usecase/orchestrator_staging.go`
  untouched, but they do change shared wiring (`pkg/app/app.go`,
  `pkg/app/app_staging.go`, `internal/infra/config/config.go`,
  `internal/usecase/picker.go`). The `:8080` numbers therefore describe
  the local picker as built before those changes. This was not
  re-checked against a rebuilt local-only instance.
