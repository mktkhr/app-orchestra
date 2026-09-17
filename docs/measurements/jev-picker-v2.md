# Jev picker trial — v2 measurement record ("v2: richer criteria")

Date: 2026-09-17 (JST). Repository: app-orchestra, branch main. Adapter
and config/runner commits this round: `8be1b55` (feat(adapter): jev
picker gains a richer v2 criteria style), `3c74497` (feat(config): wire
ORCHESTRA_JEV_CRITERIA and JEV_CRITERIA= pass-through) — both on top of
v1's tree ending at `04f1748` (see `jev-run-v1/RECORD.md`, also committed
as `a72ce72`, docs/measurements/jev-picker-v1.md).

Platform config in effect for every run below, same as v1's: narrowing
on (`e5-large-q8`+`bge-reranker-v2-m3-q8`, K=20 for shortlist/mid; the
eval suite's own fixture), wording `v6-unmatched-filter`, thinking off,
`ORCHESTRA_PLANNER_STAGES=2`, `ORCHESTRA_PLANNER_TODAY=2026-09-16`
(pinned by every runner), `ORCHESTRA_PICKER=jev`,
`ORCHESTRA_JEV_CRITERIA=v2` (new this round — v1's runs never set this,
so they used the default, now formally named `v1`),
`ORCHESTRA_JEV_BASE_URL` unset (default), ambiguity threshold 0.5. The
API key was exported inline for each command, never printed or written
into the repo.

## 0. What changed in the request (v1 → v2)

v1 sent one plain string per candidate:
`"<serviceDisplayName> / <summary>"`. v2 sends an object per candidate
(`criteriaForV2`, `services/platform/internal/adapter/planner/jev/mapping.go`):
`what` (v1's line, plus the operation's own Description's first line
when it adds something), `examples` (the operation's own
`x-orchestra-examples`, omitted when the contract declares none),
`not_for` (the service display names of every _other_ shortlist entry
that collides with this one - same `DisplayName`, or the same leading
noun of `Summary` cut at the first Japanese particle, in a _different_
service; omitted when nothing collides). The three built-ins keep their
v1 phrases as `what`; `none` gets three fixed examples (two out-of-domain
questions, one naming a verb the catalogue has no operation for at all);
`list_capabilities` gets one (「何ができるの？」). `instructions` gains
one sentence explaining what `examples` and `not_for` mean, since Jev is
never told a criterion's field names carry meaning beyond their own
values (per docs.typesafe.ai/primitives/choice).

## 1. Shortlist corpus (100 questions), `PICKER=jev JEV_CRITERIA=v2 STAGES=2`

Command: `PICKER=jev JEV_CRITERIA=v2 ORCHESTRA_JEV_API_KEY=<key> make eval-shortlist STAGES=2`.
Output variant: `on-default-stages2-jev-v2` (the new `-v2` suffix,
`e2e/shortlist/flags.ts`'s `variantSuffix`, only appended when
`ORCHESTRA_JEV_CRITERIA=v2`).

|                     | jev v2 (today)             | jev v1 (2026-09-17, `jev-run-v1/RECORD.md`) | local, STAGES=2 (recorded ref, 78/79) |
| ------------------- | -------------------------- | ------------------------------------------- | ------------------------------------- |
| correct@1           | **69/100**                 | 69/100                                      | 78/100                                |
| correct@shown       | **73/100**                 | 72/100                                      | 79/100                                |
| kinds               | result 75, form 24, none 1 | result 70, none 11, form 19                 | —                                     |
| pick-stage mean/p50 | 227ms / 212ms (max 616ms)  | 231ms / 224ms (max 584ms)                   | not measured                          |

v2's overall correct@1 is **unchanged from v1 (69/100)**; correct@shown
gains one row (72→73). The headline number does not move, but its shape
does:

Per axis, v2 correct@1 / correct@shown: A 72%/76% (18/25, 19/25), B
**84%/88%** (21/25, 22/25), C 60%/64% (15/25, 16/25), D **53%/53%**
(8/15, 8/15), E 70%/80% (7/10, 8/10).
v1: A 76%/80%, B 72%/72%, C 68%/72%, D 40%/40%, E 90%/100%.

B (+12pp) and D (+13pp) — the two axes v1's own record named as its
weakest (D was v1's worst axis at 40%, and B held several of the
承認/明細/注文-style homonym losses) — both improve materially. A, C and
E all move the other way (A −4pp, C −8pp, E −20pp on a 10-question axis,
so E's drop is 2 rows). The net is a wash on correct@1 and +1 row on
correct@shown.

**Most strikingly: v2 answers `none` on only 1 of 100 rows** (d09, 「お金
を返してもらいたい」 - also a `none` miss in v1), down from v1's 11.
Of v1's 17 correct@1 losses, 8 were jev answering `none` rather than
guessing on a cross-service homonym (承認×4, 健康診断, 社員情報, シリア
ル番号, 関税-adjacent) - exactly the failure mode `not_for` targets. In
v2, three of those four 承認 rows (b13, b14, b15) flip to correct; the
fourth (b11, 「承認の状況を知りたい」) stops answering `none` but now
picks a plausible wrong operation instead (`getAttendanceApproval`
singular-get, not one of the three `list*Approvals` in scope) - still
wrong, but a different kind of wrong: v2 traded refusal-on-collision for
occasional wrong-guess-on-collision, not for outright resolution of every
one of v1's own homonym losses.

**18 rows changed correct@1** between v1 and v2 (9 lost, 9 gained, net 0

- exactly matching v1's 69 and v2's own 69; the exact list, from
  `on-default-stages2-jev-v2.jsonl` vs `on-default-stages2-jev.jsonl`
  compared row by row on `operationId ∈ answers`):

LOST (v1 OK, v2 NG): a11 (サンプル依頼って今どうなってる？,
v1=listSalesSampleRequests → v2=getSalesSampleRequest), a15 (輸入申告っ
てどうなってる？, list→get), a21 (苦情申立って今どうなってる？,
search→get), b01 (注文を見たい, v1=listSalesOrders →
v2=getPurchasingOrder - a **new** cross-service miss, the shared "order"
noun this same mechanism is supposed to help with, going the wrong
direction here), c13 (仕入先の評価や契約を知りたい, list→get), c15 (支出
限度額や決裁委任の設定を確認したい, list→get), c24 (経費限度額や予算の
情報を知りたい, list→get), e01 (先月の残業時間, summarize→get), e04 (販
売手数料率で計算された分を確認したい, aggregate→get).

GAINED (v2 OK, v1 NG): a18 (健康診断って受けた人いる？, v1=none →
v2=listAttendanceHealthCheckups), a20 (資格を持ってる人を出したい,
listAttendanceEmployees → searchAttendanceQualifications), b05 (注文を
取り消したい, withdrawSalesOrder → deleteSalesOrder), b13/b14/b15 (承認
系３問, all v1=none → v2 correct), c05 (倉庫やロケーションの情報を知り
たい, searchInventoryWarehouses → listInventoryWarehouses), d05 (品物が
届いたので登録したい, createPurchasingGoodsReceipt →
createInventoryReceiving), d12 (値段を安くしてほしいと頼みたい, v1=none
→ v2=createSalesDiscount).

**A new, list-vs-get failure pattern**: 8 of the 9 losses above
(a11, a15, a21, c13, c15, c24, e01, e04) are v1 correctly naming a
`list*`/`search*`/`summarize*`/`aggregate*` operation and v2 instead
naming the matching `get*` (singular-record) operation on the _same_
endpoint family - not a cross-service confusion at all. (The ninth,
b01, is the exception: v1's `listSalesOrders` → v2's
`getPurchasingOrder` crosses services too.) This is not something `not_for` (a cross-service field) should
touch; the likely cause is `whatForV2`'s added Description-first-line
text or `examples`' presence nudging Jev toward a more specific-sounding
neighbor when the corpus's own phrasing ("〜ってどうなってる？",
"〜を知りたい") is genuinely ambiguous between "the list" and "one
record's state" - a wording effect, not a criteria-collision effect,
and outside this round's own hypothesis. Worth a closer look in any v3,
not investigated further here per the brief's own "no v3" instruction.

Confidence distribution (100 picks, `pick_confidence` from the platform
log): min 0.24, p25 0.44, median 0.65, p75 0.90, max 0.99, mean 0.655.
Histogram in 0.1 buckets:
`[0.2,0.3)=3 [0.3,0.4)=10 [0.4,0.5)=20 [0.5,0.6)=13 [0.6,0.7)=5 [0.7,0.8)=14 [0.8,0.9)=9 [0.9,1.0)=26`.
33/100 picks below the 0.5 ambiguity threshold (v1: 37/100). Both median
and mean confidence are higher than v1's (0.59 median/0.587 mean), and
the below-threshold count is slightly lower, despite v2's own overall
correct@1 being unchanged - a richer criteria set makes Jev _more
confident_ in its answer, not necessarily _more correct_.

Token cost: 247,427 input / 32,101 output tokens over 100 picks - **mean
2,474 input tokens/pick**, about **2.2×** v1's 1,127/pick on this same
corpus (the object-form criteria, plus `examples`/`not_for` text, roughly
double the wire size per candidate).

Zero warn/error lines in the platform log; zero non-200 responses.

Files (`jev-run-v2/`): `on-default-stages2-jev-v2.jsonl`,
`on-default-stages2-jev-v2.log` (platform log, `pick_choice`/
`pick_probabilities` on every line), `misses-default-stages2-jev-v2.txt`,
`corpus-jev-v2-picks.jsonl` (100 rows: id/question/choice/confidence/
probabilities/usage/ms, extracted from the platform log the same way
v1's own `corpus-jev-picks.jsonl` was).

## 2. Mid subset (60 questions, three services), `PICKER=jev JEV_CRITERIA=v2`

Command: `PICKER=jev JEV_CRITERIA=v2 ORCHESTRA_JEV_API_KEY=<key> make eval-mid`
(no `STAGES=`; platform default 2 applies). Output variant:
`mid-default-jev-v2`.

|                           | jev v2 (today)  | jev v1          | local (recorded ref, 38/40·1·17/20·3·0/24) |
| ------------------------- | --------------- | --------------- | ------------------------------------------ |
| answerable (40) correct@1 | **39/40**       | 37/40           | 38/40                                      |
| false refusal             | 1/40            | 3/40            | 1/40                                       |
| impossible (20) refused   | 16/20           | 16/20           | 17/20                                      |
| forced                    | 4/20            | 4/20            | 3/20                                       |
| forms fabricated          | 0/25            | 0/23            | 0/24                                       |
| latency mean/p50          | 1125ms / 1162ms | 1132ms / 1216ms | 1274ms / 1229ms                            |
| pick-stage mean           | 238ms           | 238ms           | not available                              |

**v2 beats both v1 and the local picker on this subset** - 39/40
answerable correct@1, its highest score anywhere in this trial, and its
lowest false-refusal count (1/40, matching local). Impossible/forced is
identical to v1 (jev is no more or less willing to force an answer on an
impossible question under v2 than v1); forms fabricated stays 0
throughout.

v2's 4 misses (down from v1's 7): m29 (att-019の休暇申請の日程を変更し
たい → none - unchanged from v1, a genuine near-miss both versions
share), m41 (受注を集計したい → result listSalesOrders - unchanged from
v1 and shared with local, a real ambiguity in the corpus itself), m42
(取引先を承認したい → form updatePurchasingPartner - unchanged from
v1), m43 (社員情報を一括削除したい → form deleteAttendanceEmployee -
unchanged from v1). v1's other three misses (m34, m39, and the false
refusals beyond m29) are gone under v2 - m34 (注文の内容を修正したい,
v1 answered `list_capabilities`) and m39 (取引先の情報を書き換えたい,
v1 answered `none`) are both now answered correctly.

Confidence distribution (60 picks): min 0.32, p25 0.69, median 0.96, p75
0.99, max 1.0. 7/60 (11.7%) below threshold (v1: 9/60, 15%) - again
higher and more confident than v1.

Token cost: 132,455 input / 15,127 output over 60 picks - **mean 2,208
input tokens/pick**, about **2.3×** v1's 967/pick on this subset.

Zero warn/error lines in the platform log.

Files: `mid-default-jev-v2.jsonl`, `mid-default-jev-v2.log`.

## 3. Full eval suite (34 cases), `PICKER=jev JEV_CRITERIA=v2`

Command: `PICKER=jev JEV_CRITERIA=v2 ORCHESTRA_JEV_API_KEY=<key> make eval`,
run twice (as v1's own record did): run A with no token capture, run B
with one added just for this measurement (`ORCHESTRA_EVAL_CAPTURE_LOG`,
a temporary edit to `e2e/eval/services.ts`'s `drainStdio`, piping
`child.stderr` - the platform's `slog.NewJSONHandler(os.Stderr, ...)`
writes there, not stdout, the one thing that differs from v1's own
attempt at this - to a file, then reverted immediately after via `git
checkout -- e2e/eval/services.ts`; not part of any commit).

Both runs: **32/34 cases at baseline**, the same two regressions v1
found, at closer-to-baseline severity on one of them:

- `follow-up-other-service`: **0/10** accept (baseline 10/10) in both
  runs - identical to v1. Run A: 10x `result:list_capabilities()`. Run
  B: also 10x `result:list_capabilities()` (v1 saw a 5/5 split between
  `none` and `list_capabilities` across its own two runs; v2 lands on
  `list_capabilities` every time in both runs here). jev v2 still never
  continues a follow-up onto the other service's operation.
- `real-attendance-detail`: run A **2/10** accept, 8/10 reject; run B
  **5/10** accept, 5/10 reject. Both inside v1's own observed range
  (2/10, 6/10) for this case - still the same genuinely unstable,
  near-tie row `docs/plans/staging.md` already describes for the local
  picker too, not something v2's richer criteria moved either way in any
  clear direction (2→5 vs v1's 2→6 is noise on a 10-sample near-tie, not
  a trend).

Every other case (`filter-by-label*`, `no-enum-value*`,
`list-everything*`, `create*`, `capability`, `unanswerable`,
`follow-up-stays`, and every other `real-*` case) held at baseline in
both runs, exactly as under v1.

Confidence distribution (run B, 360 picks, exact from the captured
platform log): min 0.30, p25 0.87, median 0.98, p75 0.99, max 1.0; 23/360
(6.4%) below the 0.5 ambiguity threshold - materially higher-confidence
than v1's own eval-suite run (median 0.92, 46/360 = 12.8% below
threshold), continuing the pattern from sections 1 and 2: v2 is more
confident everywhere it is measured, whether or not it is more correct
there.

pick-stage latency (run B, 360 picks): mean 219.7ms, p50 209ms, max
573ms - close to v1's own 230.5ms/218ms/614ms; the richer criteria did
not slow picks down measurably.

Token cost (run B, the only one of the two runs actually captured - see
section 5): 414,320 input / 44,253 output over 360 picks - **mean 1,151
input tokens/pick**, about **1.76×** v1's 652/pick on this suite's own
(typically small) shortlists. Run A's tokens were not captured (its
`drainStdio` call ran before this measurement's temporary patch existed)
and are assumed equal to run B's in the ledger (section 5) - the eval
corpus is deterministic (same 34 cases, same shortlists every run), and
both runs' accept/reject counts agree at 32/34 baseline, so their token
totals should be close even though only run B's were actually counted.

Zero warn/error lines in either platform log; zero non-200 responses in
either run.

Files: `eval-jev-v2-report-run-a.txt`, `eval-jev-v2-report-run-b.txt`,
`eval-jev-v2-platform.log` (run B's captured platform log, 360
`pick completed` lines), `eval-run-a.log`, `eval-run-b.log` (the full
`make eval` stdout, build steps included).

## 4. Spend

See `typesafe-spend.json` (scratchpad root) for the full ledger.

- v1: 622 calls, 519,211 input / 123,563 output tokens, $0.0218.
- v2 (this round): 880 calls (100 shortlist + 60 mid + 360 eval run A +
  360 eval run B), 1,208,522 input / 135,734 output tokens (eval run A's
  are the estimate described in section 3), **$0.0508**.
- **Cumulative this trial: 1,502 calls, 1,727,733 input tokens, $0.0726**
  against the $2 budget - still well under 4% of it. No run was stopped
  early for budget reasons.

Per-pick input tokens by instrument, v2 vs v1 (v2 is consistently ~1.8–
2.3× v1, largest on the instruments with the biggest shortlists, where
`not_for`'s cross-checking has the most sibling endpoints to name):
shortlist corpus 2,474 vs 1,127 (2.2×), mid subset 2,208 vs 967 (2.3×),
eval suite 1,151 vs 652 (1.76×).

## 5. `make check` and the harness

`make check` (fmt-check, lint, test, build, acceptance incl. browser) was
run once against the full tree after both commits (adapter + config/
runners/Makefile) and passed, exit code 0 - 32 guard/test/build/
acceptance steps, all `ok:`. New Go tests added this round: 2 in
`internal/adapter/planner/jev/mapping_v2_test.go` (the v2 request body,
exact, for a two-endpoint shortlist with a `DisplayName`-collision and
`x-orchestra-examples`; and a v1-unchanged-byte-for-byte guard), 3 in
`internal/infra/config/config_test.go` (`ORCHESTRA_JEV_CRITERIA` default/
v2/invalid), 1 in `pkg/app/app_picker_test.go`
(`TestNewWithPickerJevCriteriaV2BuildsAndServesAQuestion`). New/changed
TypeScript tests: `e2e/shortlist/flags.test.ts` gains 3 cases for
`variantSuffix`'s new `-v2` suffix. llama-swap's own model list was
checked before this round's runs: 31 models, same three loaded
(`bge-reranker-v2-m3-q8`, `e5-large-q8`, `qwen3.5-9b-q8`) - unchanged
from v1.

## 6. Hashes

Commits this round (`git log --oneline`, top of `main`):

```
3c74497 feat(config): wire ORCHESTRA_JEV_CRITERIA and JEV_CRITERIA= pass-through
8be1b55 feat(adapter): jev picker gains a richer v2 criteria style
a72ce72 docs: record the Jev picker v1 trial - measured, not adopted
04f1748 fix(eval): drain spawned services' stdout/stderr
```

SHA-256 of this round's own output files (`jev-run-v2/`):

```
602ce8ffe03fb77c480c97a817935366c0b92943bf337345e4c0ee2012af8b27  on-default-stages2-jev-v2.jsonl
0e8fa7be3c2539cdcc872e1cd174eeab5162eedf705636fee5c07c9a4a2ad2c3  mid-default-jev-v2.jsonl
a01439dbd209bc977bc265cc3afbb3e178bc9678ad14b36276108733fecd6809  corpus-jev-v2-picks.jsonl
3c5f1c94ae6714a03f3047e4530e06c544c41062619b7af945624f0d8e86d323  eval-jev-v2-platform.log
```

## 7. Deviations and notes

1. Same `confidence`-not-`max(probabilities)` note v1 already recorded
   (`jev-picker-v1.md`, section 1) - unaffected by the criteria style,
   not re-verified line by line this round.
2. Pick latency continues to sit well under the ~600ms contract figure
   (219–238ms means throughout this round, 573–616ms maxima), matching
   v1's own finding.
3. No 401/422/429/529 was seen in any of this round's 880 calls.
4. This round's temporary platform-log capture for the eval suite
   (section 3) needed `child.stderr`, not `child.stdout` as a first
   attempt assumed - `services/platform/cmd/api/main.go` builds its
   logger over `os.Stderr` (`slog.NewJSONHandler(os.Stderr, nil)`). The
   first attempt (piping stdout) produced an empty capture file; this is
   noted here rather than silently discarded, since a future capture
   attempt over this same codebase would hit the same thing. Not a
   defect in the adapter or the harness - the capture code itself was
   never committed either way.
5. **Overall assessment of this round's hypothesis** ("the cross-service
   homonym collision is where the losses concentrate; a `not_for` field
   naming siblings should recover them"): partially borne out. The
   targeted failure mode - answering `none` on a homonym collision
   rather than guessing - dropped from 11/100 to 1/100 on the shortlist
   corpus, and the mid subset's own answerable correct@1 rose to this
   trial's best score anywhere (39/40, beating both v1 and the local
   picker). But the shortlist corpus's own overall correct@1 did not
   move (69/100 both rounds), because a new, unhypothesized failure mode
   appeared alongside the fix: v2 several times names a `get*`
   (singular-record) operation where v1 correctly named the matching
   `list*`/`search*`/`summarize*`/`aggregate*` one, on phrasings like
   「〜ってどうなってる？」 that are genuinely ambiguous between "the
   list" and "one record's state" - not a cross-service collision at
   all, so `not_for` was never going to touch it. The two eval-suite
   regressions v1 found (`follow-up-other-service`,
   `real-attendance-detail`) are both still present, essentially
   unchanged. Confidence is higher across every instrument under v2,
   independent of whether accuracy moved - a caution against reading
   Jev's own confidence as a proxy for correctness gained from richer
   criteria.
