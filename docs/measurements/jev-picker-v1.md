# Jev picker trial — measurement record

This is the raw scratchpad record of the first Jev picker trial
(2026-09-17), copied here verbatim, unedited. The 100 raw corpus picks it
references sit beside it as `jev-picker-v1-corpus-picks.jsonl`. See
`DECISIONS.md`, 2026-09-17 ("Jev as the pick stage, v1: measured, not
adopted") for the decision this record backs.

Date: 2026-09-17 (JST). Repository: app-orchestra, branch main. All runs
against today's tree, commit history ending at (in order) cb59717
(adapter), ffc1704 (probabilities log), 37f0842 (variantSuffix -jev),
6618f78 (config/app wiring), c1e1c47 (Makefile/eval pass-through), 8ffcdfe
(pick_choice log), 04f1748 (eval stdio-drain fix).

Platform config in effect for every run below unless noted: narrowing on
(`e5-large-q8` embed + `bge-reranker-v2-m3-q8` rerank, K=20 for the
shortlist/mid runs; the eval suite's own fixture, no narrowing override),
wording default (`v6-unmatched-filter`, the platform's own default since
2026-09-16), thinking off (the default), `ORCHESTRA_PLANNER_STAGES=2`
(pick-then-fill), `ORCHESTRA_PLANNER_TODAY=2026-09-16` (pinned by every
runner: `e2e/shortlist/boot.ts` and `e2e/eval/services.ts` both set it, so
every planning call's date line is fixed regardless of the day the
process actually ran on — this is why today's numbers are comparable to
the ones recorded 2026-09-16), `ORCHESTRA_PICKER=jev`,
`ORCHESTRA_JEV_BASE_URL` unset (default `https://api.typesafe.ai`),
ambiguity threshold 0.5 (the adapter's own default; never overridden in
any of these runs). The API key was exported inline for each command from
its local file, never printed, never written into the repo or this record.

## 1. The Jev request/response shape (one real call)

Captured with a standalone script hitting the real endpoint with the
adapter's exact request shape (`internal/adapter/planner/jev/mapping.go`),
for corpus question a03 ("ピッキングリストが知りたい") against a
three-endpoint shortlist:

```json
{
  "request": {
    "state": "ピッキングリストが知りたい",
    "model": "jev-latest",
    "questions": {
      "pick": {
        "type": "choice",
        "instructions": "社内APIの振り分け役。質問に対して、候補の中から呼ぶべき操作を1つ選ぶ。list_capabilitiesは「何ができるか」を尋ねる質問のとき、propose_panelは画面に何かを出したい質問のとき、noneはどの候補も質問に合わない、または質問が業務と無関係なときに選ぶ。",
        "criteria": {
          "listInventoryPickLists": "在庫管理 / ピッキングリストの一覧を返す",
          "getInventoryPickList": "在庫管理 / ピッキングリストの詳細を返す",
          "createInventoryPickList": "在庫管理 / ピッキングリストを作成する",
          "list_capabilities": "使える操作の一覧を知りたい",
          "propose_panel": "画面に出したい",
          "none": "どの候補も質問に合わない（業務と無関係な質問）"
        }
      }
    }
  },
  "response_status": 200,
  "response_body": {
    "model": "jev-1.13.0",
    "answers": {
      "pick": {
        "type": "choice",
        "choice": "listInventoryPickLists",
        "confidence": 0.35,
        "probabilities": {
          "none": 0.02,
          "listInventoryPickLists": 0.46,
          "propose_panel": 0.14,
          "list_capabilities": 0.01,
          "createInventoryPickList": 0,
          "getInventoryPickList": 0.37
        }
      }
    },
    "usage": { "input_tokens": 594, "output_tokens": 89 }
  },
  "latency_ms": 598
}
```

Full JSON saved beside this file as `example-request-response.json`.

**Deviation from the documented contract, noted here rather than treated
as a bug**: `confidence` (0.35) is not the probability of the chosen
`choice` (`listInventoryPickLists` = 0.46, the actual max of the
distribution). The contract describes `confidence` and `probabilities` as
two separate fields without saying they must agree, and the adapter never
assumed they would — `Ambiguous` is computed from `confidence` alone, as
specified — but a reader expecting "confidence = probability of the
answer given" will be surprised. `min(0.22,...)`/`p25`/etc. distributions
below are all built from `confidence`, not from `max(probabilities)`; the
two are close on most rows in this corpus but not identical (see the
`corpus-jev-picks.jsonl` file, which carries both).

The API answered every one of the 622 real calls made across every run in
this trial with HTTP 200; zero 401/422/429/529 were seen anywhere.
`model` in the response was always `"jev-1.13.0"` (the request always
asked for `"jev-latest"`).

## 2. Shortlist corpus (100 questions), `ORCHESTRA_PICKER=jev STAGES=2`

Command: `PICKER=jev ORCHESTRA_JEV_API_KEY=<key from its local file> make eval-shortlist STAGES=2`
Run at 2026-09-17T14:43–14:44 JST (a first attempt at 14:37 produced
identical numbers before `pick_choice` was added to the log; both are
kept — see files below).

|                     | jev (today)                  | local picker, STAGES=2 (today, same tree)                    | recorded ref (`nocr-stages2.jsonl`, 2026-09-16) |
| ------------------- | ---------------------------- | ------------------------------------------------------------ | ----------------------------------------------- |
| correct@1           | 69/100                       | 78/100                                                       | 78/100                                          |
| correct@shown       | 72/100                       | 79/100                                                       | 79/100                                          |
| kinds               | result 70, none 11, form 19  | (not recomputed; report only gives correct@1/@shown)         | —                                               |
| latency mean/p50    | 1131ms / 1056ms (max 3428ms) | 1776ms / 1344ms (max 26746ms)                                | —                                               |
| pick-stage mean/p50 | 231ms / 224ms (max 584ms)    | not measured (no per-pick timing logged by the local picker) | —                                               |

Today's local-picker re-run reproduced the recorded 78/79 exactly — no
drift from the corpus, the model, or the date pin.

Per axis, jev correct@1 / correct@shown: A 19/25 (76%/80%), B 18/25
(72%/72%), C 17/25 (68%/72%), D 6/15 (40%/40%), E 9/10 (90%/100%).
Local (today): A 88/88, B 88/88, C 64/68, D 60/60, E 90/90 (overall
78/79).

**25 rows changed correct@1** between the local picker and jev (17 lost,
8 gained, net −9, matching 78−9=69):

Lost (local OK, jev NG): a02 (シリアル番号ってどうなってる？, jev→none),
a06 (キット構成を確認したい, jev→getInventoryKit vs answer
listInventoryKits), a16 (関税, jev→getPurchasingCustomsDuty),
a18 (健康診断, jev→none), a20 (資格を持ってる人, jev→listAttendanceEmployees),
b11 (承認の状況, jev→none), b13 (承認を新規登録, jev→none), b14
(承認の内容を直したい, jev→none), b15 (承認を取り消したい, jev→none),
b19 (社員情報を直したい, jev→list_capabilities), c05 (倉庫やロケーション,
jev→searchInventoryWarehouses vs answer listInventoryWarehouses), c16
(残業や欠勤, jev→searchAttendanceRecords), d04 (有給を使いたい,
jev→createAttendancePaidLeave vs answer createAttendanceLeaveRequest),
d05 (品物が届いたので登録したい, jev→createPurchasingGoodsReceipt vs
answer createInventoryReceiving), d09 (お金を返してもらいたい, jev→none),
d12 (値段を安くしてほしい, jev→none), e08 (出張承認の上限,
jev→listExpenseTravelExpenses vs answer listAttendanceBusinessTrips).

Gained (jev OK, local NG): a19 (研修受講の状況), a21 (苦情申立て),
b06 (明細をまとめて見たい), c02 (入出庫を一覧したい), c13 (仕入先の評価や
契約), c15 (支出限度額や決裁委任), d11 (商品が届いたので受け取り処理),
e03 (日当単価をもとに支給された分).

8 of the 17 losses resolve to jev choosing `none` (a02, a18, b11, b13,
b14, b15, d09, d12) — the same shape as the local picker's own known weak
spot for multi-service homonyms (S1's finding in `docs/specs/staging.md`
section 1, e.g. "承認" existing in three services at once). jev appears
more willing to answer `none` on these than to guess, where the local
picker guesses and is right more often on this particular corpus.

Confidence distribution (100 picks, from the platform's own `pick_completed`
log, `pick_confidence` field): min 0.22, p25 0.41, median 0.59, p75 0.72,
max 0.98, mean 0.587. Histogram in 0.1 buckets:
`[0.2,0.3)=8 [0.3,0.4)=15 [0.4,0.5)=14 [0.5,0.6)=17 [0.6,0.7)=14 [0.7,0.8)=13 [0.8,0.9)=8 [0.9,1.0)=11`
(nothing below 0.2). 37/100 picks fell below the 0.5 ambiguity threshold
(`pick_ambiguous=true` on every one of them — the adapter's own
`Ambiguous = confidence < threshold` is exactly what fired).

Zero warn/error lines in the platform log for this run (0 retries, 0
unknown-choice warnings, 0 401/429/529).

**On "the 43 rows the local picker flagged ambiguous" (requested for
comparison)**: not reconstructable from the scratchpad. `stages2-run3.jsonl`
carries no `ambiguous` field per row (only `id`/`axis`/`text`/`answers`/
`kind`/`operationId`/`alternatives`/`via`/`latencyMs`), and
`stages2-run3-platform.log` is exactly two lines (`narrowing loaded`,
`platform listening`) — it never captured a single `pick completed` line,
confirming the brief's own warning that this file "may lack pick lines".
Whatever produced the "43" figure elsewhere is not in this scratchpad; I
cannot say which 43 ids they are, or what jev did on them specifically,
without that source. What I can and did compare instead: jev's own
confidence/ambiguous numbers on today's 100-question run, above.

Files: `on-default-stages2-jev.jsonl`, `on-default-stages2-jev.log`
(platform log, includes `pick_choice`/`pick_probabilities` on every
line), `local-on-default-stages2.jsonl` (today's local-picker run, for
comparison), `misses-default-stages2-jev.txt`,
`corpus-jev-picks.jsonl` (100 rows: id/question/choice/confidence/
probabilities/usage/ms, extracted from the platform log by pairing each
question with its one `pick completed` line in request order — the eval
harness runs one question at a time, sequentially, so this pairing is
exact for this run).

## 3. Mid subset (60 questions, three services), `ORCHESTRA_PICKER=jev`

Command: `PICKER=jev ORCHESTRA_JEV_API_KEY=<key from its local file> make eval-mid`
(no `STAGES=` given — the platform's own default, 2, applies; the output
file is named `mid-default-jev` — no `-stages2` suffix, since
`variantSuffix` only adds that suffix when `--stages` was explicitly
passed to `run.ts`, which this command does not do).
Run at 2026-09-17T14:46–14:47 JST.

|                           | jev (today)     | local picker (today, same tree)                            | recorded ref |
| ------------------------- | --------------- | ---------------------------------------------------------- | ------------ |
| answerable (40) correct@1 | 37/40           | 38/40                                                      | 38/40        |
| false refusal             | 3/40            | 1/40                                                       | 1            |
| impossible (20) refused   | 16/20           | 17/20                                                      | 17/20        |
| forced                    | 4/20            | 3/20                                                       | 3            |
| forms fabricated          | 0/23            | 0/24                                                       | 0/24         |
| latency mean/p50          | 1132ms / 1216ms | 1274ms / 1229ms                                            | —            |
| pick-stage mean           | 238ms           | not available (no `pick_ms` in the local picker's own log) | —            |

Today's local re-run again reproduced the recorded reference exactly
(38/40 · 1 · 17/20 · 3 · 0/24), confirming the date pin holds.

jev's misses (7): m29 (att-019の休暇申請の日程を変更したい → none),
m34 (注文の内容を修正したい → result list_capabilities — new miss,
not in the local picker's own miss list), m39 (取引先の情報を書き換え
たい → none — new miss), m41 (受注を集計したい → result
listSalesOrders — shared with local), m42 (取引先を承認したい → form
updatePurchasingPartner — local picked createSalesPartner, both wrong but
differently), m43 (社員情報を一括削除したい → form
deleteAttendanceEmployee — new miss), m44 (休暇申請書を印刷したい →
result listAttendanceLeaveRequests — shared with local). Local's own
m24 (att-102の連絡先を直して) is not a miss for jev.

Confidence distribution (60 picks): min 0.29, p25 0.64, median 0.88, p75
0.98, max 1.0. 9/60 (15%) below the 0.5 ambiguity threshold. Notably
higher-confidence overall than the 100-question shortlist corpus (median
0.88 vs 0.59) — the mid subset's three-service, thirty-operation
catalogue gives jev far less cross-service homonym collision to be
uncertain about.

Zero warn/error lines in the platform log.

Files: `mid-default-jev.jsonl`, `mid-default-jev.log`,
`local-mid-default.jsonl` (today's local-picker run, for comparison).

## 4. Full eval suite (34 cases, `real-*` and enum cases), `ORCHESTRA_PICKER=jev`

Command: `PICKER=jev ORCHESTRA_JEV_API_KEY=<key from its local file> make eval`

**First two attempts failed** (2026-09-17T14:48–14:55 and 14:56–15:03,
~6m48s each): `node:internal/modules/run_main` uncaught `TypeError: fetch
failed`, cause `HeadersTimeoutError` / `UND_ERR_HEADERS_TIMEOUT` — a
single `/api/plan` request never answered within undici's default 300s
headers timeout. A same-day control run with `PICKER` unset (the local
picker) completed cleanly in 5m19s with every case at baseline, isolating
the cause to the jev picker rather than the environment.

**Root cause**: `e2e/eval/services.ts` spawns the platform with
`stdio: ["ignore", "pipe", "pipe"]` (`startBinary`,
`e2e/src/helpers/process.ts`) and never reads either piped stream. A
piped stream nobody reads stays paused; once its OS pipe buffer (64KiB on
Linux) fills, the child process's own write to that fd blocks — and,
because the platform's structured logger (`slog`) writes synchronously to
stdout, so does whatever request happened to be logging at that moment,
forever, since nothing was ever going to drain the rest. The jev picker
logs one extra `"pick completed"` line per pick (with `pick_probabilities`,
a per-candidate map — the largest field in the line) beside the
orchestrator's own generic one; across the eval suite's ~360 requests
that is enough extra volume to fill the buffer within a run, where the
local picker's much shorter lines never did. This is not a bug in the
adapter's request/response handling — every call still completed inside
its own 10s bound — it is a pre-existing gap in the harness (unconsumed
piped stdio) that a more talkative picker was the first to actually hit.

**Fix**: `e2e/eval/services.ts` now drains (`.resume()`) stdout/stderr for
every spawned service (inventory, attendance, platform), committed as
`04f1748`. Verified: `make check` (full suite, including
`acceptance-browser`) still green afterward; the eval suite itself no
longer hangs under `PICKER=jev`.

**Post-fix runs** (two, one plain and one with a temporary platform-log
capture added just for this measurement's exact token accounting, then
reverted — the capture is not part of the committed fix):

- Run A, 2026-09-17T15:11:33–15:15:51 (4m18s): 32/34 cases at baseline
  (10/10 or 30/30 `accept`); two regressions:
  - `follow-up-other-service`: **0/10** accept (baseline 10/10) — 5x
    `result:list_capabilities()`, 5x `none`. jev never once continued the
    follow-up onto the other service's operation in ten tries.
  - `real-attendance-detail`: **2/10** accept, 8/10 reject (baseline
    10/10).
- Run B, 2026-09-17T15:20:06–15:24:27 (4m21s), with platform-log capture:
  same 32/34 at baseline; `follow-up-other-service` again exactly 0/10
  (5x none / 5x list_capabilities — same split both times);
  `real-attendance-detail` this time **6/10** accept, 4/10 reject —
  different from run A's 2/10, confirming this is a genuinely unstable,
  near-tie case under jev (matching how `docs/plans/staging.md` already
  describes `real-attendance-detail` as a near-tie row for the local
  picker too), not a one-off fluke.

**`real-attendance-detail` root cause, checked directly against run B's
captured platform log**: this is not a shortlist-narrowing bug.
`idAffinity` (`internal/usecase/orchestrator_affinity.go`, shared code
run before either picker is ever called) fired for all 10 runs of this
case — the log shows exactly 10
`"pick affinity narrowed shortlist to one service"` /
`pick_affinity_service=attendance` lines, one per run, so jev's shortlist
was narrowed to attendance-only endpoints every single time, identically
to what the local picker would have received. Pairing each of those 10
narrowing lines with the jev pick that immediately followed it gives the
full picture:

```
GetAttendanceRecord  confidence 0.40  ambiguous
GetAttendanceRecord  confidence 0.37  ambiguous
GetAttendanceRecord  confidence 0.45  ambiguous
GetAttendanceRecord  confidence 0.50  (not ambiguous - exactly at threshold)
none                 confidence 0.35  ambiguous
none                 confidence 0.42  ambiguous
GetAttendanceRecord  confidence 0.39  ambiguous
none                 confidence 0.46  ambiguous
none                 confidence 0.45  ambiguous
GetAttendanceRecord  confidence 0.41  ambiguous
```

6 correct / 4 `none`, matching run B's reported 6/10 exactly. Even on the
6 it gets right, confidence sits in a tight 0.37–0.50 band — jev is never
confident about this question even with a single-service, narrowed
shortlist naming the operation outright; on 4 of 10 tries that same
uncertainty crosses into answering `none` instead of naming the (correct
and only sensible) operation. `real-inventory-detail` (`itm-001の詳細`),
narrowed the same way to inventory-only in all 10 of its own runs, held
10/10 - so the instability is specific to how jev reads
`<id>の内容` ("<id>'s content/details") against `GetAttendanceRecord`'s
own criteria description, not a general weakness of narrow shortlists or
of ID-detail questions as a class.

Every other case — both `filter-by-label*` families, `no-enum-value` (30
runs) and `no-enum-value-attendance`, `capability`, `unanswerable`,
`create*`, `list-everything*`, and every other `real-*` case — held at
baseline in both post-fix runs.

Confidence distribution, run B's 360 picks (exact, from the captured
platform log): min 0.30, p25 0.70, median 0.92, p75 0.99, max 1.0; 46/360
(12.8%) below the 0.5 ambiguity threshold. Materially higher-confidence
than the shortlist corpus (median 0.92 vs 0.59) and similar in shape to
the mid subset — the eval suite's cases are individually narrower
(smaller shortlists, more clear-cut questions) than the 100-question
corpus's cross-service homonyms.

pick-stage latency (run B, 360 picks): mean 230.5ms, p50 218ms, max 614ms.

Zero warn/error lines in either post-fix platform log; zero
401/422/429/529 anywhere in this trial.

Files: `eval-jev-report-run2.txt` (run A's full case report),
`eval-jev-report-run3-with-log.txt` (run B's, token-accurate),
`eval-jev-platform.log` (run B's captured platform log, 360
`pick completed` lines), `eval-local-control-stdout.txt` (the local-picker
control run that isolated the hang to the picker choice).

## 5. Spend

Ledger (`typesafe-spend.json`, cumulative across every real call made in
this trial, including the very first smoke test from before this task
and the two failed eval attempts that still cost real tokens before they
hung):

- 622 calls total, 519,211 input tokens, 123,563 output tokens.
- At the blog's stated price ($0.042/MTok input, output free):
  **$0.0218 total** — a rounding error against the $2 budget.
- Average per pick: ~835 input tokens overall (519,211 / 622), but this
  varies a lot by shortlist size: ~1127 tokens/pick on the 20-candidate
  shortlist corpus (112,661 / 100), ~967 tokens/pick on the mid subset's
  thirty-operation catalogue (58,043 / 60), ~652 tokens/pick on the eval
  suite's own, typically smaller, per-case shortlists (234,640 / 360).

No run was stopped early for budget reasons; nothing approached even 1%
of the $2 ceiling.

## 6. `make check` and the harness

`make check` (full suite: fmt-check, lint, test, build, acceptance
including browser) was run twice against the final tree and passed both
times, exit code 0, after the adapter existed and again after the
config/app wiring and the eval stdio-drain fix. `llama-swap`'s own model
list (`/v1/models`) was checked before and after the whole trial: 31
models declared, the same three loaded both times
(`bge-reranker-v2-m3-q8`, `e5-large-q8`, `qwen3.5-9b-q8`) — nothing this
trial did changed which models llama-swap was running.

## 7. Deviations from the documented contract, summarized

1. `confidence` is not always `max(probabilities)` for the chosen answer
   (section 1) — not necessarily wrong, but worth knowing before treating
   the two as interchangeable.
2. Pick latency was consistently faster than the contract's own ~600ms
   figure: measured means were 231ms (shortlist corpus), 238ms (mid),
   230.5ms (eval), with maxima never exceeding 614ms across 520 real
   picks. The one single ad-hoc call in section 1 (598ms) is close to the
   documented figure; the much larger sample from the actual runs is not.
   Possibly the documented ~600ms describes a different load pattern, a
   cold start, or a larger/different criteria set than this trial's.
3. No 401/422/429/529 was seen in 622 calls, so the retry-on-429/529 path
   (`internal/adapter/planner/jev/client.go`) was exercised only by its
   own unit tests, never for real in this trial.
4. The harness bug in section 4 (unconsumed piped stdio hanging a
   request under enough log volume) is not a Jev API deviation at all,
   but is the one genuine defect this trial's measurement work turned up
   and fixed.
