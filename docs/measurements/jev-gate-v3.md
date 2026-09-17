# Jev picker trial — v3 measurement record ("v3: a noul refusal gate in front of the local pick")

Date: 2026-09-17 (JST). Repository: app-orchestra, branch main. Adapter/
usecase/config commits this round, on top of v2's tree (`70ac2ca`, docs:
record the Jev picker v2 trial):

```
f39cdc8 feat(adapter): jev gains a v3 noul refusal gate port
e8c9068 feat(usecase): planStaged calls the gate before the pick
f8f38dd feat(config): wire ORCHESTRA_GATE and GATE= pass-through
```

Unlike v1/v2, this round does **not** change the picker: `ORCHESTRA_PICKER`
is left unset (local, the default) throughout every run below.
`ORCHESTRA_GATE=jev` is the only new knob exercised — Jev is used for
exactly one typed yes/no decision (`Gate.Gate`, one `noul` question) run
before the local pick, in `planStaged`, after `idAffinity` and before
`o.picker.Pick`. `ORCHESTRA_JEV_GATE_THRESHOLD` was left at its default,
`0.7`, for every run (never overridden). Platform config otherwise as
v1/v2's: narrowing on (`e5-large-q8`+`bge-reranker-v2-m3-q8`, K=20),
wording `v6-unmatched-filter`, thinking off, `ORCHESTRA_PLANNER_STAGES=2`
(now the platform's own default since 2026-09-16, so unset in these
commands), `ORCHESTRA_PLANNER_TODAY=2026-09-16` pinned by every runner.
The API key was exported inline for each command from its local file,
never printed or written into the repo.

## 0. The gate's own shape

`Gate.Gate` (`services/platform/internal/adapter/planner/jev/gate.go`)
sends one `systemone` request: `state` is an **object**
(`{"question": "<query>\n回答: <param>=<value>\n...", "operations":
[{"id","service","summary"}, ...]}`, one entry per shortlist endpoint —
`pickCatalog`, the same post-`idAffinity` catalogue the pick itself sees),
and one `noul` question keyed `"gate"`:

> この質問は、列挙された操作のどれでも実現できないことを求めているか（例:
> 一覧しかない資源の集計・承認・印刷、列挙に無い資源、業務と無関係な話
> 題）。能力を尋ねる質問（何ができる？）は「いいえ」。

`Impossible = noul >= threshold` (0.7 default). On `Impossible`,
`planStaged` answers `none` (`messageNoEndpoint`) without calling the
picker or the fill — the same result `planOrdinary` gives for `none`. On
a `Gate` error, `planStaged` logs a warn and falls through to the pick
unchanged (fail-open; a gate outage never blocks planning).

## 1. One real request/response

Captured with a standalone script calling `jev.NewGate` directly against
the real endpoint (mirrors v1's own section 1), for the mid corpus's m44
question (「休暇申請書を印刷したい」) against a 3-endpoint illustrative
shortlist (m44's own real run, section 3 below, used its actual
post-narrowing 20-endpoint shortlist and scored noul 0.86 — both
comfortably above 0.7, so this shorter example is qualitatively the same
call):

```json
{
  "request": {
    "state": {
      "question": "休暇申請書を印刷したい",
      "operations": [
        {
          "id": "listInventoryPickLists",
          "service": "inventory",
          "summary": "ピッキングリストの一覧を返す"
        },
        {
          "id": "getInventoryPickList",
          "service": "inventory",
          "summary": "ピッキングリストの詳細を返す"
        },
        {
          "id": "createInventoryPickList",
          "service": "inventory",
          "summary": "ピッキングリストを作成する"
        }
      ]
    },
    "model": "jev-latest",
    "questions": {
      "gate": {
        "type": "noul",
        "instructions": "この質問は、列挙された操作のどれでも実現できないことを求めているか（例: 一覧しかない資源の集計・承認・印刷、列挙に無い資源、業務と無関係な話題）。能力を尋ねる質問（何ができる？）は「いいえ」。"
      }
    }
  },
  "response_status": 200,
  "response_body": {
    "model": "jev-1.13.0",
    "answers": { "gate": { "type": "noul", "noul": 0.94 } },
    "usage": { "input_tokens": 521, "output_tokens": 20 }
  },
  "latency_ms": 539
}
```

Full JSON saved beside this file as `example-gate-request-response.json`
(Authorization header redacted before saving). Every gate call observed
this round answered HTTP 200; zero 401/422/429/529.

## 2. Shortlist corpus (100 questions), `GATE=jev STAGES=2`, local pick

Command: `GATE=jev ORCHESTRA_JEV_API_KEY=<key> make eval-shortlist STAGES=2`.
Output variant: `on-default-stages2-gate` (the new `-gate` suffix,
`e2e/shortlist/flags.ts`'s `variantSuffix`, appended when
`ORCHESTRA_GATE=jev`).

|               | gate (today)                          | local, STAGES=2, no gate (today, same tree)       |
| ------------- | ------------------------------------- | ------------------------------------------------- |
| correct@1     | 77/100 (A88 B84 C64 D60 E90)          | 78/100 (A88 B88 C64 D60 E90)                      |
| correct@shown | 79/100 (A88 B88 C68 D60 E90)          | 79/100 (A88 B88 C68 D60 E90)                      |
| latency       | mean 1556ms / p50 1466ms / max 3637ms | mean 1317ms / p50 1209ms / max 3415ms (reference) |

**Every corpus question is answerable by design (`docs/plans/midsizing.md`
section 1), so the expectation going in was ~0 change — and the gate's own
cost on this instrument is exactly zero false refusals.** All 100 gate
calls' `gate_noul` were checked against the 0.7 threshold:
**max noul over the whole corpus was 0.31** (min 0.06, mean 0.122, median
0.11), histogram in 0.1 buckets:
`[0.0,0.1)=35 [0.1,0.2)=56 [0.2,0.3)=8 [0.3,0.4)=1 [0.4,0.5)=0 [0.5,0.6)=0 [0.6,0.7)=0 [0.7,0.8)=0 [0.8,0.9)=0 [0.9,1.0)=0`.
**Zero rows crossed 0.7** — the gate never fired a refusal on this corpus
at all. The one-point correct@1 drop (78→77, axis B 88→84, one row) is
therefore **not gate-caused**: it is the local picker's own run-to-run
sampling variance (a19, 「研修受講の状況が知りたい」, answered `none`
this run with `gate_noul=0.07` — nowhere near the threshold — the same
"none" failure mode the local picker already shows in v1/v2's own
records, unrelated to this round's change). correct@shown is unchanged
(79/100, both runs).

Gate latency: mean 244.6ms, p50 230ms, max 594ms, over 100 calls. Token
cost: 118,857 input / 2,000 output tokens (mean 1,188.6 input / 20 output
per call — `20` output tokens is exact and identical on every single gate
call observed across this entire round, sections 1-4: a `noul` answer's
own fixed-width `{"noul": 0.NN}` shape apparently costs the same output
tokens regardless of the probability's value).

Files (copied into `jev-run-v3/`): `on-default-stages2-gate.jsonl`,
`on-default-stages2-gate.log` (platform log, `gate completed`/`pick
completed` on every line), `misses-default-stages2-gate.txt`.

## 3. Mid subset (60 questions, three services), `GATE=jev`

Command: `GATE=jev ORCHESTRA_JEV_API_KEY=<key> make eval-mid`. Output
variant: `mid-default-gate`.

|                           | gate (today)    | local, no gate (recorded ref) | jev picker v2 (recorded ref) |
| ------------------------- | --------------- | ----------------------------- | ---------------------------- |
| answerable (40) correct@1 | 38/40           | 38/40                         | 39/40                        |
| false refusal             | 1/40            | 1/40                          | 1/40                         |
| impossible (20) refused   | 17/20           | 17/20                         | 16/20                        |
| forced                    | 3/20            | 3/20                          | 4/20                         |
| forms fabricated          | 0/24            | 0/24                          | 0/25                         |
| latency mean/p50          | 1402ms / 1471ms | 1274ms / 1229ms               | 1125ms / 1162ms              |
| pick mean                 | 628ms           | not available                 | 238ms                        |

**This is the instrument the gate was built for — the "impossible half"
midsizing.md itself calls out, and the three forced rows the task brief
named (m41, m42, m44).** Their gate_noul, exactly:

| id  | question               | gate_noul | gated (≥0.7)? | baseline (no gate) outcome                  | this run's outcome                           |
| --- | ---------------------- | --------- | ------------- | ------------------------------------------- | -------------------------------------------- |
| m41 | 受注を集計したい       | 0.16      | no            | forced (result listSalesOrders)             | still forced (result listSalesOrders)        |
| m42 | 取引先を承認したい     | 0.37      | no            | forced (form createSalesPartner)            | still forced (form createPurchasingApproval) |
| m44 | 休暇申請書を印刷したい | 0.86      | **yes**       | forced (result listAttendanceLeaveRequests) | **refused (correctly, `none`)**              |

**m44 (印刷/print) is fixed**: the gate crosses 0.86, well above
threshold, and `planStaged` now answers `none` before the pick ever runs
— exactly the target failure mode. **m41 (集計/aggregate) and m42
(承認/approve) are not fixed**: their own noul (0.16, 0.37) never comes
close to 0.7, so the gate lets both through unchanged, and the local
picker forces an answer on both exactly as it did in every prior round.
The task brief's own instructions text (「一覧しかない資源の集計・承認・
印刷」) names all three verbs as examples in one sentence, but Jev's
actual judgment treats them very differently: 印刷 (print) reads as
unambiguously impossible against a list-only catalogue, while 集計
(aggregate) and especially 承認 (approve, on a resource this subset does
have a whole workflow family for in the full fixture, so "look plausible"
even where the mid subset itself lacks the specific operation) read as
much closer calls.

Net effect on the five headline numbers: identical to the no-gate
baseline on four of five (answerable correct@1 38/40, false refusal
1/40, impossible refused 17/20, fabricated 0/24) and **also identical on
"forced" (3/20)**, but not because nothing moved: m44 flipped from forced
to correctly refused (a real fix), while **m43** (「社員情報を一括削除
したい」, previously correctly refused every prior round including this
round's own shortlist companion) instead became newly forced (`form
deleteAttendanceEmployee`) — m43's own gate_noul was 0.22, far under
threshold, so the gate did not touch it either; this is the local
picker's own independent run-to-run sampling variance (the same kind seen
on the shortlist corpus, section 2), coincidentally landing on the one
row that cancels out m44's fix in the raw "forced" count. The gate's own
contribution, read off gate_noul rather than off the aggregate count, is
unambiguous: it fixed exactly one of the three named forced rows (m44)
and left the other two (m41, m42) exactly as before.

**Gate noul on every mid row that crossed 0.7** (7 of 60): m44 (0.86,
verb-not-there, correct refusal), m47 (0.79, 「在庫の数を確認したい」,
resource-not-there, correct), m48 (0.92, 「倉庫の一覧が知りたい」,
resource-not-there, correct), m49 (0.75, 「請求書を発行したい」,
resource-not-there, correct), m51 (0.96, 「今日の天気を教えて」, out of
domain, correct), m52 (0.93, 「今日って何曜日だっけ」, out of domain,
correct), m54 (0.91, 「123足す456はいくつ？」, out of domain, correct).
**Every one of these 7 gate-triggered refusals was against a genuinely
impossible question — zero false refusals among them.**

**Capability questions, explicitly carved out by the gate's own
instructions, checked directly**: m56 (「何ができるの？」) noul=0.12, m57
(「使える操作は？」) noul=0.09, m58 (「受注で何ができる？」) noul=0.11,
m59 (「発注について何ができる？」) noul=0.12, m60 (「勤怠で何ができる？」)
noul=0.14 — all far under threshold. The instruction's explicit carve-out
("能力を尋ねる質問（何ができる？）は「いいえ」") holds on every capability
row in this corpus; the gate never touches a capability question here.

**m29** (「att-019の休暇申請の日程を変更したい」→`none`, the false refusal
present in every prior round's own baseline too) has gate_noul=0.08 —
confirmed not gate-caused, matching the shortlist corpus's own a19
finding (section 2).

Full noul distribution (60 picks): min 0.05, max 0.96, mean 0.230, median
0.10. Histogram: `[0.0,0.1)=29 [0.1,0.2)=16 [0.2,0.3)=2 [0.3,0.4)=2
[0.4,0.5)=0 [0.5,0.6)=1 [0.6,0.7)=3 [0.7,0.8)=2 [0.8,0.9)=1 [0.9,1.0)=4`.
7/60 at or above 0.7.

Gate latency: mean 233.2ms, p50 216.5ms, max 525ms, over 60 calls. Token
cost: 60,463 input / 1,200 output tokens (mean 1,007.7 input / 20 output
per call).

Files: `mid-default-gate.jsonl`, `mid-default-gate.log`.

## 4. Full eval suite (34 cases), `GATE=jev`, run once

Command: `GATE=jev ORCHESTRA_JEV_API_KEY=<key> make eval` (run once, per
the brief's "one run per instrument"). `make eval`'s own drainStdio
discards the platform's stderr (v2's own note, `e2e/eval/services.ts`),
so the run itself carries no per-call gate_noul; see the supplementary
probe below for that evidence, gathered as nine single-shot calls against
the exact same eval-suite catalogue rather than a second `make eval` run.

**33 of 34 cases held at baseline; one regressed**:

- `real-inventory-list-graph` (「在庫の一覧をグラフで」, accept:
  `result ListInventoryItems`): baseline **10/10 accept** → this run
  **0/10 accept, 10x "none"**. This is a genuine false refusal introduced
  by the gate.
- Every other case — `filter-by-label*` (incl. all seven label variants),
  `no-enum-value`/`no-enum-value-attendance`, `list-everything*`,
  `create*`, `unanswerable`, `capability`, `follow-up-stays`,
  `follow-up-other-service`, and all fifteen other `real-*` cases — held
  exactly at baseline (10/10 or 30/30 as their own `runs` specify).
  Notably `follow-up-other-service` is 10/10 accept this round — the
  known jev-_picker_-only regression v1/v2 both recorded (0/10 under
  `PICKER=jev`) does not appear here, since this round never sets
  `ORCHESTRA_PICKER=jev` at all; the local picker still drives every pick.

**Supplementary probe** (not a second `make eval` run — a standalone
script, `probe-real-cases.ts`, booting the same inventory+attendance
dummy-service catalogue `make eval` uses and sending exactly one request
per case instead of ten, with the platform's stderr captured to a file
this time): the six "refusal-ish" real cases plus the one regression plus
two general capability questions, gate_noul each:

| id                            | question                 | gate_noul | gated?  | outcome this probe                                              |
| ----------------------------- | ------------------------ | --------- | ------- | --------------------------------------------------------------- |
| real-delete-inventory         | 在庫を削除して           | 0.86      | yes     | none (correct — accept includes `none`)                         |
| real-decrease-inventory       | 在庫を減らして           | 0.39      | no      | result list_capabilities (accept)                               |
| real-show-sales               | 売上を見せて             | 0.92      | yes     | none (correct)                                                  |
| real-sum-overtime             | 残業時間を集計して       | 0.72      | yes     | none (correct)                                                  |
| real-what-day                 | 今日は何曜日？           | 0.94      | yes     | none (correct)                                                  |
| real-capability-inventory     | 在庫について何ができる？ | 0.12      | no      | result list_capabilities (correct — capability carve-out holds) |
| **real-inventory-list-graph** | **在庫の一覧をグラフで** | **0.78**  | **yes** | **none (WRONG — this is the false refusal)**                    |
| capability-general-1          | 何ができるの？           | 0.13      | no      | result list_capabilities (correct)                              |
| capability-general-2          | 使える操作は？           | 0.08      | no      | result list_capabilities (correct)                              |

**The false refusal is fully explained**: `real-inventory-list-graph`
scores noul=0.78, just above the 0.7 threshold — Jev reads
「グラフで」(as a graph) as resembling the instruction's own named
example category ("集計・承認・印刷" — none of these operations can
render a chart, only list raw rows), even though the product's own
accepted answer is to just return the plain list and ignore the
decorative "as a graph" request. This is the one place in this entire
round's evidence where the gate is wrong, and it is a near-miss: 0.78 is
the closest any genuinely-answerable question came to the 0.7 line in any
instrument measured (shortlist max 0.31, mid's answerable-half max stayed
well clear too — see section 3's full distribution, no answerable mid row
appears in the ≥0.7 table above).

None of the five refusal-ish real cases actually _needed_ the gate to
pass — `real-decrease-inventory` and `real-capability-inventory` both
score well under threshold and were already correctly handled by the
local picker alone in every prior round's baseline, matching this round's
10/10. The gate is additive on `real-delete-inventory`/`real-show-sales`/
`real-what-day` (redundant safety, all still accept `none`) and on
`real-sum-overtime` (0.72, the only one where crossing 0.7 barely
mattered — its own baseline reject list only forbids naming
`ListAttendanceRecords` directly, and `none` was already an accepted
answer regardless of whether the gate or the pick produced it).

Files: `eval-run.log` (`make eval`'s own report, one run), `probe-real-cases.ts`
(the supplementary script, not part of the repo), `probe-results.jsonl`,
`probe-platform.log` (the probe's own captured platform log, 9
`gate completed` lines).

## 5. Threshold: is 0.7 right?

Per the brief: no second `make eval` run was made to test this — the
recommendation below is read off the two distributions actually gathered
(sections 2-4), not measured directly at a different threshold.

- Every false refusal found this round (exactly one: `real-inventory-
list-graph`, noul=0.78) sits _just_ above 0.7.
- Every correct refusal found this round sits at 0.75 or higher (mid's
  lowest gated row, m49, is 0.75; the eval probe's lowest gated row,
  real-sum-overtime, is 0.72 — and that row's own correctness did not
  depend on being gated, section 4).
- Nothing answerable anywhere in this round's evidence (100 shortlist
  rows, 40 mid answerable rows, 9 eval-catalogue probes) scored above
  0.39 **except** the one false refusal at 0.78 — a real gap, not a dense
  cluster the threshold cuts through.

**Recommendation: raise `ORCHESTRA_JEV_GATE_THRESHOLD` to 0.80–0.85.**
At 0.80: `real-inventory-list-graph` (0.78) drops below the line and is
fixed; `real-sum-overtime` (0.72) also drops below, but section 4 already
showed the gate was not load-bearing there (the local pick already
answers `none` on its own). On the mid instrument, m49 (0.75) and m47
(0.79) would also fall below 0.80 and stop being gated — both are
resource-not-there rows presently caught only by the gate; whether the
local picker alone would still refuse them correctly is untested (they
were not previously measured without a gate in this trial, since the mid
"local, no gate" reference in section 3 predates m47/m49 being singled
out). m44, m48, m51, m52, m54 (0.86-0.96) all stay well clear of 0.80. The
narrowest threshold that both fixes the one known false refusal (0.78)
and keeps every currently-correct gated row (lowest is m49 at 0.75) is
anywhere in (0.78, ~0.86) — e.g. **0.80**, which is this round's
recommendation: it accepts giving up m47/m49's gate coverage (untested
without the gate, so their fallback behavior is unknown) in exchange for
fixing the one confirmed false refusal. A future round should re-measure
mid and the eval suite once at 0.80 rather than trust this extrapolation
further.

## 6. Spend

See `typesafe-spend.json` (scratchpad root) for the full ledger.

- v1: 622 calls, 519,211 input / 123,563 output tokens, $0.0218.
- v2: 880 calls, 1,208,522 input / 135,734 output tokens, $0.0508.
- v3 (this round): 529 calls (100 shortlist gate + 60 mid gate + 360 eval
  gate, estimated from the 9-call probe's own measured 611 input-token
  mean since `make eval`'s own run was not captured + 9 probe calls,
  measured), **404,770 input** (219,960 of which is the eval estimate) /
  **10,580 output** tokens, **$0.0170**.
- **Cumulative this trial: 2,031 calls, 2,132,503 input tokens, $0.0896**
  against the $2 budget — under 5% of it. No run was stopped early for
  budget reasons.

The eval instrument's own 360-call token total is an estimate (the same
caveat v2's own run A carried): `make eval`'s `drainStdio` discards the
platform's stderr, so no per-call gate_noul or token count was captured
from the actual `make eval` run itself; 611 input tokens/call (this
round's own measured mean from the 9-call supplementary probe, run
against the identical eval-suite catalogue) is used as the per-call
estimate. Output tokens are not estimated: every gate call observed this
entire round, with no exception across 169 directly-measured calls
(shortlist 100 + mid 60 + probe 9), answered with exactly 20 output
tokens, so 20/call is used for the eval estimate too, not a guess.

## 7. `make check` and the harness

`make check` was run twice on the committed tree: once by the
implementing session (57 `ok:` steps, exit 0), and once independently
from a clean port by this measurement session (fmt/lint/generate/guards/
test/build/acceptance-services/acceptance-web/acceptance-e2e/
acceptance-browser/guard-a11y/guard-layout, all `ok:`, exit 0). New Go
tests: `internal/adapter/planner/jev/gate_test.go` (exact request body,
threshold both sides, `WithGateThreshold`, empty shortlist, non-200,
missing answer key, log fields), `internal/usecase/
orchestrator_staging_test.go` (+5: no gate unchanged, impossible → none
without picker/planner calls, possible → picker called, error → fail-open
no error, error → warn logged), `internal/infra/config/config_test.go`
(+8: `ORCHESTRA_GATE` default/jev/invalid, `ORCHESTRA_JEV_GATE_THRESHOLD`
default/custom/invalid, `ORCHESTRA_JEV_API_KEY` required for gate too).
New TypeScript tests: `e2e/shortlist/flags.test.ts` (+4, the `-gate`
suffix). No network calls in any test (all Jev interactions go through
`httptest.NewServer`). llama-swap's own model list unchanged from v1/v2:
`bge-reranker-v2-m3-q8`, `e5-large-q8`, `qwen3.5-9b-q8` resident.

## 8. Hashes

Commits this round (`git log --oneline`, top of `main`):

```
f8f38dd feat(config): wire ORCHESTRA_GATE and GATE= pass-through
e8c9068 feat(usecase): planStaged calls the gate before the pick
f39cdc8 feat(adapter): jev gains a v3 noul refusal gate port
70ac2ca docs: record the Jev picker v2 trial - measured, not adopted either
```

SHA-256 of this round's own output files (`jev-run-v3/`):

```
8f603d025ce7f9d9b94eb9045a8752303ffd4c5197910406901527a0a7177f0e  on-default-stages2-gate.jsonl
7aee0f1ad75232b62954e8af03b8dd3c275ba931c6feaca2d84cd06da4ec8345  mid-default-gate.jsonl
38a677c87e31e89bb188510b3219a68db6264e210e596a1d0b7fd591bcfb7ce4  probe-results.jsonl
026750f4c0d1d6793bc7b1e0e9bb3129de1d8b98a23773bc53856b5b447ca8f8  probe-platform.log
```

## 9. Deviations and notes

1. This round's own gate output-token constant: every single `noul`
   answer observed (169 directly-measured calls across sections 2-4) cost
   exactly 20 output tokens, regardless of the `noul` value itself or the
   shortlist size — unlike input tokens, which scale with the shortlist
   (`operations` list) and question length. Worth knowing for costing a
   production gate deployment: output cost per call is fixed, input cost
   is the only lever.
2. Gate latency (233-245ms mean across sections 2-3) is close to but
   slightly under the local pick's own reported "pick mean" (628ms on the
   mid run this round) — the gate adds a real but not dominant sequential
   step ahead of the pick; a production deployment pays roughly gate_ms +
   pick_ms in series for every staged plan call, not gate_ms alone.
3. The one false refusal found (`real-inventory-list-graph`) was located
   only because `make eval`'s own summary line flagged a regression
   (`0/10 accept`, baseline `10/10`) — the supplementary probe (section 4)
   was built specifically to explain it, not planned in advance. This is
   the reason a "no-op on the corpus, net positive on mid, but check the
   full eval suite too" measurement plan is worth its own third instrument
   (`docs/specs/midsizing.md` section 1's own argument, echoed here one
   round later): the shortlist corpus, being uniformly answerable, could
   never have surfaced this failure mode at all.
4. Overall assessment of this round's hypothesis ("gate the one typed
   yes/no decision the local pipeline is worst at, leave the local picker
   choosing"): **partially borne out, same shape as v2's own verdict**.
   The gate is a true no-op on the shortlist corpus (0 false refusals,
   confirmed by evidence not just by the correct@1 number, which moved by
   1 point for an unrelated reason). On mid, it fixes exactly the forced
   row its own instructions describe most literally (m44, 印刷/print) and
   leaves the other two named forced rows (m41 集計/aggregate, m42
   承認/approve) untouched, because Jev's own judgment does not treat all
   three verbs as equally certain — 印刷 against a list-only catalogue
   reads as obviously impossible (0.86), while 集計 and 承認 read as much
   closer to plausible (0.16, 0.37) even in this same catalogue. On the
   full eval suite, one previously-correct answerable case is broken
   (`real-inventory-list-graph`, noul=0.78, a near-miss just above
   threshold) — the first false refusal this entire three-round trial has
   recorded at default settings. Section 5's threshold analysis argues
   0.80-0.85 fixes this specific miss without giving up mid's own three
   strongest fixes (0.86-0.96), but that recommendation is not yet
   measured — a v4 (or a threshold-tuning follow-up to this same v3) would
   need one more `make eval`/`make eval-mid` pass at the new value to
   confirm it, which this round's own "no re-run" instruction defers.
