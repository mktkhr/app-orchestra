# Jev language spike v4 — is Japanese the cause of Jev's losses?

Date: 2026-09-17 (JST). Repository: app-orchestra, branch main, tree at
`eab162c` when this spike started. Throwaway measurement scripts (not
committed) live in the session scratchpad, `jev-lang-v4/`:
`build-rows.ts`, `controls.ts`, `build.ts`, `translate-local.ts`,
`jev-call.ts`, `analyze.py` — key logic reproduced in the appendix below.
This record, `translations.jsonl` and `jev-language-v4-picks.jsonl` are
the only artifacts that enter the repository.

## Hypothesis

Jev's losses on the shortlist corpus (`jev-picker-v1.md`: 17 rows the
local picker got right and Jev v1 got wrong) are caused by Japanese —
the corpus's questions and the criteria sent to Jev are all Japanese,
and Jev may be English-centred. If true, translating the same rows to
English should recover most of the 17 losses without breaking rows Jev
already gets right.

## Design

50 rows: the 17 lost + 8 gained (id lists taken verbatim from
`jev-picker-v1.md`'s own tally) + 25 controls. For each row the
shortlist Jev sees is rebuilt from `docs/measurements/jev-picker-v1-corpus-picks.jsonl`
— the keys of that row's `probabilities` object, minus the three
built-ins (`list_capabilities`, `propose_panel`, `none`) — mapped to
their `serviceDisplayName`/`summary`/`description`/`displayName`/
`examples` via `e2e/narrowing/fixture/index.ts`'s `catalogOf(5)` (run
with `node --experimental-strip-types`, importing the fixture module by
absolute path — no package.json in the scratchpad, so this was simpler
than wiring up the e2e project's own module resolution for a one-off
script). Criteria sent to Jev are v2-style (`what`/`examples`/`not_for`),
built by reimplementing `services/platform/internal/adapter/planner/jev/mapping.go`'s
`criteriaForV2`/`whatForV2`/`notForV2`/`noun` in TypeScript — see the
appendix for the reimplementation, and note the deliberate deviation:
using v2 criteria for the JA baseline (not v1, which is what
`jev-picker-v1.md`'s own 17/8 lists were measured against) means this
spike's own JA numbers are not directly the same measurement as v1's —
they are a fresh v2-criteria JA baseline, run specifically so the JA→EN
comparison inside this spike is apples-to-apples.

### Control selection — a deviation from the literal brief, recorded rather than "fixed"

The brief asked for "25 controls: every 4th row of the corpus in id
order that both local and Jev v1 got right." Recomputing "Jev v1 got it
right" directly from `jev-picker-v1-corpus-picks.jsonl`'s own recorded
`choice` field against the current `e2e/narrowing/corpus/*.ts` `answers`
gives only **59/100** Jev-v1-correct — not the 69/100 `jev-picker-v1.md`
reports as Jev v1's headline correct@1. The gap is real and understood,
not a bug in this spike: `jev-picker-v1-corpus-picks.jsonl`'s `choice`
is Jev's **pick-only** answer, while the v1 record's 69/100 is the
platform's **end-to-end** correct@1 — a `propose_panel`/`none` pick
falls back to the local picker's own single-call resolution, which
sometimes lands the right operation. c02 is exactly this case: Jev's raw
pick is `propose_panel` (not in `answers`), yet `jev-picker-v1.md` lists
c02 as "gained" (correct end-to-end). **Pick-only 59 vs end-to-end 69:
the local fallback rescued 10 of Jev v1's 100 rows.** This spike calls
Jev directly and grades the raw pick, so pick-only is the right and
internally consistent metric for everything below — the JA and EN runs
in this spike are both graded the same way, so the comparison between
them is unaffected by which of the two baselines you prefer.

Given that, "both local and Jev v1 got right" (pick-only) leaves only
**51** of the 75 unchanged (non-lost/non-gained) rows — not enough for
a literal "every 4th of the corpus" to yield 25 (it yields 13). Controls
here are instead **the first 25 rows, in corpus id order, of that
51-row pool** — a defensible, deterministic substitute for "every 4th",
recorded here rather than silently redefined.

### Translation

Requested API (Anthropic Haiku 4.5) was blocked: the stored key in its
usual local file was rejected by the API (`authentication_error: API
key is invalid`, confirmed with a bare `curl`, not a script bug) and no
`ANTHROPIC_API_KEY` fallback was set.
Per the coordinator's redirect, translation was done instead with the
**local model, `qwen3.5-9b-q8`, via llama-swap**
(`http://localhost:11435/v1/chat/completions`, `temperature: 0`,
`chat_template_kwargs: {"enable_thinking": false}`) — free, no
Anthropic spend. `translate.ts` (the Haiku path) was written and kept in
the scratchpad but never run beyond nothing (no cost incurred).

1790 items: 50 questions, 563 unique operation `what` lines (across the
union of the 50 shortlists), 1126 example phrases, 51 `not_for` phrases.
Batches of 40 items, numbered JA in / numbered EN out, system prompt:

```
You are translating short pieces of Japanese text from an internal
business-system API catalogue into English, for a research measurement.
The items are: (a) user questions asked of the system, (b) one-line operation
summaries ("what" a candidate API operation does), (c) short example phrases a
person might type, and (d) short "not_for" disambiguation phrases.
Keep terminology consistent across items: 受注 = sales order, 発注 = purchase
order, 承認 = approval, 明細 = line item, 代休 = compensatory leave,
有給 = paid leave. Translate naturally and concisely; do not add quotes,
numbering commentary, or explanations of your own.
Output English only, one line per numbered item, keep the numbering
(e.g. "3. <translation>"). No other text.
```

3-item smoke test passed first (English, correctly numbered, parsed
clean). Full batch: 45 chunks, 0 missing/malformed lines, no retries
needed. The three built-in criteria (`list_capabilities`/`propose_panel`/
`none`, their fixed examples) and the one-sentence `instructions` text
were hand-translated rather than sent through the model — short, fixed
strings not worth a batch item; see the appendix for both language
versions verbatim.

### Jev runs

Four passes over the same 50 rows: **JA-1, JA-2, EN-1, EN-2** (two per
language, to separate a language effect from Jev's own known
run-to-run noise on near-ties). `POST https://api.typesafe.ai/v1/systemone`,
`model: "jev-latest"`, one `choice` question, the API key exported
inline for the call from its own local file, never printed or
written to any file that enters the repo. 200 calls total, zero non-200
responses, zero retries. Correct iff `choice` ∈ the corpus's `answers`
for that row id.

## Results

### 2×2 table — correct count per group × language, both passes

| group          | n   | JA-1   | JA-2   | EN-1   | EN-2   |
| -------------- | --- | ------ | ------ | ------ | ------ |
| lost (17)      | 17  | 8      | 8      | 8      | 8      |
| gained (8)     | 8   | 6      | 5      | 4      | 3      |
| control (25)   | 25  | 22     | 22     | 19     | 19     |
| **total (50)** | 50  | **36** | **35** | **31** | **30** |

Overall, both EN passes score _below_ both JA passes on this 50-row
sample (31/30 vs 36/35) — English is not an improvement here, it is a
regression, on the model's own pick-only accuracy.

### Lost (17): the group the hypothesis is actually about

The 8-vs-8 raw tally hides churn — it is not the same 8 rows in each
language:

- **Net recovery** (wrong in both JA passes, right in both EN passes):
  **1 row** — d09 (`お金を返してもらいたい` / "wants a refund"), which
  both JA passes answered `none` and both EN passes correctly answered
  `createExpenseReimbursement`.
- **Net regression** (right in at least one JA pass, wrong in both EN
  passes): **2 rows** — a20 (`searchAttendanceQualifications` — correct
  in JA-1 only) and c05 (`listInventoryWarehouses` — correct in JA-2
  only); both go wrong in EN-1 and EN-2.
- The other 14 of 17 lost rows are unchanged by language: still wrong
  in all four passes (a02, a06, a16, b11, c16, d04, e08 — 7 rows) or
  still right in all four (b13, b14, b15, b19, d05, d12 — 6 rows,
  a18 also right in all four = 7 rows stable-correct). [7 stable-wrong +
  7 stable-correct + 1 net-recover + 2 net-regress = 17.]

One recovery against two regressions, inside the group the hypothesis
predicts should improve most, is not the signature of a language
effect being fixed.

### Controls (25): what English breaks that Japanese didn't

22 of 25 controls are correct in **both** JA passes (a stable JA
baseline). Of those 22, **3 break in English** (wrong in at least one EN
pass, and in fact both): a01 (`listInventoryBarcodes` → `propose_panel`),
a14 (`listPurchasingBlankets` → `getPurchasingBlanket`, a list→get
slip — the same failure shape `jev-picker-v2.md` names for v1→v2), and
a24 (`aggregateExpenseAllowances` → `listExpensePayments`).

### Gained (8): the group that degrades most

This is where English costs the most: three rows correct in both JA
passes go wrong in both EN passes — a19 (`listAttendanceTrainings` →
`getAttendanceTraining`, another list→get slip), b06
(`listExpenseLines` → `propose_panel`), d11 (`createPurchasingGoodsReceipt`
→ `createInventoryReceiving`, a cross-service near-miss). c13 and e03
stay correct throughout; c02 and c15 are unstable in one language or the
other (noise, see below); a21 is wrong everywhere.

### Noise band: JA-1 vs JA-2, EN-1 vs EN-2

| pair         | n   | same choice | same correctness | choice flip rate |
| ------------ | --- | ----------- | ---------------- | ---------------- |
| JA-1 vs JA-2 | 50  | 47          | 47               | 6% (3/50)        |
| EN-1 vs EN-2 | 50  | 48          | 49               | 4% (2/50)        |

Both languages show the same order of run-to-run noise (~5%, 2–3 rows
out of 50 flip choice between repeated passes) that `jev-picker-v1.md`
already documented for Jev on near-ties. The JA→EN gap (36/35 vs 31/30,
a 5–6 point drop) is roughly the same size as this noise band, but it
is consistently in the same direction across both pass-pairs (every EN
pass scores below every JA pass), which a pure noise explanation would
not predict as reliably.

### Verdict

**Not supported.** Translating to English does not recover Jev's
Japanese losses on this corpus — it recovers 1 of 17 net, regresses 2
of the same 17, breaks 3 of 22 previously-stable controls, and scores
lower overall (31/30 vs 36/35 out of 50) than Japanese on this run,
so the hypothesis "Japanese causes Jev's losses" is not supported by
this test — with the caveat that the translation was machine-made by a
9B local model (`qwen3.5-9b-q8`), not a human or a larger model, so
translation quality itself is a live confound this spike cannot rule
out.

## Costs

**Anthropic: $0.00.** The Haiku 4.5 key was invalid and no request was
ever billed against it (confirmed by the rejected `curl` call before any
retry); translation ran entirely on the local `qwen3.5-9b-q8` via
llama-swap, which is free. Budget (≤ $0.50) was not touched.

**Jev (typesafe): $0.0185**, well inside the ≤ $0.05 budget. 200 calls
(4 passes × 50 rows), 439,578 input tokens / 64,455 output tokens
(`typesafe-spend.json`'s own note: input $0.042/MTok, output free — the
same rate the v1/v2 records used). Zero 4xx/5xx, zero retries.

## Per-row table (50 rows × 4 passes)

`id | group | axis | JA-1 | JA-2 | EN-1 | EN-2` — `OK`/`NG(choice)`.

```
a02 | lost    | A | NG(getInventorySerialNumber) | NG(getInventorySerialNumber) | NG(getInventorySerialNumber) | NG(getInventorySerialNumber)
a06 | lost    | A | NG(getInventoryKit) | NG(getInventoryKit) | NG(getInventoryKit) | NG(getInventoryKit)
a16 | lost    | A | NG(getPurchasingCustomsDuty) | NG(getPurchasingCustomsDuty) | NG(getPurchasingCustomsDuty) | NG(getPurchasingCustomsDuty)
a18 | lost    | A | OK(listAttendanceHealthCheckups) | OK(listAttendanceHealthCheckups) | OK(listAttendanceHealthCheckups) | OK(listAttendanceHealthCheckups)
a20 | lost    | A | OK(searchAttendanceQualifications) | NG(propose_panel) | NG(propose_panel) | NG(propose_panel)
b11 | lost    | B | NG(getAttendanceApproval) | NG(getAttendanceApproval) | NG(getPurchasingApproval) | NG(getPurchasingApproval)
b13 | lost    | B | OK(createAttendanceApproval) | OK(createAttendanceApproval) | OK(createExpenseApproval) | OK(createExpenseApproval)
b14 | lost    | B | OK(updateAttendanceApproval) | OK(updateAttendanceApproval) | OK(updatePurchasingApproval) | OK(updatePurchasingApproval)
b15 | lost    | B | OK(deletePurchasingApproval) | OK(deletePurchasingApproval) | OK(deletePurchasingApproval) | OK(deletePurchasingApproval)
b19 | lost    | B | OK(updateAttendanceEmployee) | OK(updateAttendanceEmployee) | OK(updateAttendanceEmployee) | OK(updateAttendanceEmployee)
c05 | lost    | C | NG(getInventoryWarehouse) | OK(listInventoryWarehouses) | NG(getInventoryWarehouse) | NG(getInventoryWarehouse)
c16 | lost    | C | NG(searchAttendanceRecords) | NG(searchAttendanceRecords) | NG(searchAttendanceRecords) | NG(searchAttendanceRecords)
d04 | lost    | D | NG(createAttendancePaidLeave) | NG(createAttendancePaidLeave) | NG(createAttendancePaidLeave) | NG(createAttendancePaidLeave)
d05 | lost    | D | OK(createInventoryReceiving) | OK(createInventoryReceiving) | OK(createInventoryReceiving) | OK(createInventoryReceiving)
d09 | lost    | D | NG(none) | NG(none) | OK(createExpenseReimbursement) | OK(createExpenseReimbursement)
d12 | lost    | D | OK(createSalesDiscount) | OK(createSalesDiscount) | OK(createSalesDiscount) | OK(createSalesDiscount)
e08 | lost    | E | NG(aggregateExpenseSpendingLimits) | NG(aggregateExpenseSpendingLimits) | NG(aggregateExpenseSpendingLimits) | NG(aggregateExpenseSpendingLimits)
a19 | gained  | A | OK(listAttendanceTrainings) | OK(listAttendanceTrainings) | NG(getAttendanceTraining) | NG(getAttendanceTraining)
a21 | gained  | A | NG(getAttendanceGrievance) | NG(getAttendanceGrievance) | NG(getAttendanceGrievance) | NG(getSalesComplaint)
b06 | gained  | B | OK(listExpenseLines) | OK(listExpenseLines) | NG(propose_panel) | NG(propose_panel)
c02 | gained  | C | NG(propose_panel) | NG(propose_panel) | OK(listInventoryReceivings) | NG(listInventoryTransfers)
c13 | gained  | C | OK(listPurchasingSupplierEvaluations) | OK(listPurchasingSupplierEvaluations) | OK(listPurchasingSupplierEvaluations) | OK(listPurchasingSupplierEvaluations)
c15 | gained  | C | OK(listPurchasingSpendingLimits) | NG(getPurchasingDelegationLimitSetting) | OK(listPurchasingSpendingLimits) | OK(listPurchasingSpendingLimits)
d11 | gained  | D | OK(createPurchasingGoodsReceipt) | OK(createPurchasingGoodsReceipt) | NG(createInventoryReceiving) | NG(createInventoryReceiving)
e03 | gained  | E | OK(listExpensePerDiems) | OK(listExpensePerDiems) | OK(aggregateExpensePerDiems) | OK(aggregateExpensePerDiems)
a01 | control | A | OK(listInventoryBarcodes) | OK(listInventoryBarcodes) | NG(propose_panel) | NG(propose_panel)
a03 | control | A | NG(getInventoryPickList) | NG(getInventoryPickList) | NG(getInventoryPickList) | NG(getInventoryPickList)
a04 | control | A | OK(listInventoryContainers) | OK(listInventoryContainers) | OK(listInventoryContainers) | OK(listInventoryContainers)
a05 | control | A | OK(listInventoryPallets) | OK(listInventoryPallets) | OK(listInventoryPallets) | OK(listInventoryPallets)
a07 | control | A | OK(listSalesReps) | OK(listSalesReps) | OK(listSalesReps) | OK(listSalesReps)
a08 | control | A | OK(listSalesTerritories) | OK(listSalesTerritories) | OK(listSalesTerritories) | OK(listSalesTerritories)
a11 | control | A | NG(getSalesSampleRequest) | NG(getSalesSampleRequest) | NG(getSalesSampleRequest) | NG(getSalesSampleRequest)
a12 | control | A | OK(createSalesBundle) | OK(createSalesBundle) | OK(listSalesBundles) | OK(listSalesBundles)
a13 | control | A | OK(listPurchasingBids) | OK(listPurchasingBids) | OK(listPurchasingBids) | OK(listPurchasingBids)
a14 | control | A | OK(listPurchasingBlankets) | OK(listPurchasingBlankets) | NG(getPurchasingBlanket) | NG(getPurchasingBlanket)
a15 | control | A | NG(getPurchasingImportDeclaration) | NG(getPurchasingImportDeclaration) | NG(getPurchasingImportDeclaration) | NG(getPurchasingImportDeclaration)
a17 | control | A | OK(listPurchasingDirectShips) | OK(listPurchasingDirectShips) | OK(listPurchasingDirectShips) | OK(listPurchasingDirectShips)
a22 | control | A | OK(listExpenseCorporateCards) | OK(listExpenseCorporateCards) | OK(listExpenseCorporateCards) | OK(listExpenseCorporateCards)
a23 | control | A | OK(summarizeExpenseEntertainments) | OK(summarizeExpenseEntertainments) | OK(summarizeExpenseEntertainments) | OK(summarizeExpenseEntertainments)
a24 | control | A | OK(aggregateExpenseAllowances) | OK(aggregateExpenseAllowances) | NG(listExpensePayments) | NG(listExpensePayments)
a25 | control | A | OK(listExpensePerDiems) | OK(listExpensePerDiems) | OK(listExpensePerDiems) | OK(listExpensePerDiems)
b02 | control | B | OK(getSalesOrder) | OK(getSalesOrder) | OK(getSalesOrder) | OK(getSalesOrder)
b03 | control | B | OK(createPurchasingOrder) | OK(createPurchasingOrder) | OK(createSalesOrder) | OK(createSalesOrder)
b07 | control | B | OK(getExpenseLine) | OK(getExpenseLine) | OK(getExpenseLine) | OK(getExpenseLine)
b08 | control | B | OK(createExpenseLine) | OK(createExpenseLine) | OK(createExpenseLine) | OK(createExpenseLine)
b09 | control | B | OK(updateExpenseLine) | OK(updateExpenseLine) | OK(updateExpenseLine) | OK(updateExpenseLine)
b10 | control | B | OK(deleteExpenseLine) | OK(deleteExpenseLine) | OK(deletePurchasingOrderLine) | OK(deletePurchasingOrderLine)
b12 | control | B | OK(getExpenseApproval) | OK(getExpenseApproval) | OK(getExpenseApproval) | OK(getExpenseApproval)
b17 | control | B | OK(getAttendanceEmployee) | OK(getAttendanceEmployee) | OK(getAttendanceEmployee) | OK(getAttendanceEmployee)
b18 | control | B | OK(createAttendanceEmployee) | OK(createAttendanceEmployee) | OK(createAttendanceEmployee) | OK(createAttendanceEmployee)
```

The full 200-row machine-readable version (all fields: `choice`,
`confidence`, top-3 `probabilities`, `latency_ms`, `usage`) is
`jev-language-v4-picks.jsonl` beside this file.

## Appendix: script locations and key logic

Scripts (throwaway, scratchpad, not committed):

- `jev-lang-v4/controls.ts`, `build-rows.ts` — row selection (17 lost +
  8 gained ids from `jev-picker-v1.md`; controls = first 25 of the
  51-row "unchanged & jev-v1-pick-correct" pool, id order).
- `jev-lang-v4/build.ts` — shortlist reconstruction (from each row's
  `probabilities` keys) + v2 criteria (reimplements `mapping.go`).
- `jev-lang-v4/translate-local.ts` — batched local translation via
  llama-swap.
- `jev-lang-v4/jev-call.ts` — the four Jev passes.
- `jev-lang-v4/analyze.py` — the scoring/tables above.

`mapping.go`'s `criteriaForV2` reimplemented in TypeScript (the JS
mirror of `whatForV2`/`notForV2`/`noun`/`criteriaForV2`,
`services/platform/internal/adapter/planner/jev/mapping.go`):

```ts
function summaryFor(e) {
  return e.summary || (e.description.split("\n")[0] ?? "");
}
function whatForV2(e) {
  const what = e.serviceDisplayName + " / " + summaryFor(e);
  const descLine = e.description.split("\n")[0] ?? "";
  if (descLine === "" || what.includes(descLine)) return what;
  return what + "。" + descLine;
}
function noun(e) {
  if (e.displayName) return e.displayName;
  const summary = summaryFor(e);
  const m = summary.match(/[のをにがはで]/);
  if (m && m.index > 0) return summary.slice(0, m.index);
  return summary;
}
function notForV2(e, shortlist) {
  const own = noun(e);
  const seen = new Set();
  const siblings = [];
  for (const other of shortlist) {
    if (other.service === e.service || noun(other) !== own) continue;
    const name = other.serviceDisplayName;
    if (seen.has(name)) continue;
    seen.add(name);
    siblings.push(name + "の" + own + "ではない");
  }
  return siblings.join("、");
}
```

English instructions and built-in criteria (hand-translated, not sent
through the translation model):

```
Instructions (EN): The internal API router. Given a question, pick
exactly one operation to call from the candidates. Choose
list_capabilities when the question asks what this system can do,
propose_panel when the question wants something shown on screen, and
none when no candidate matches the question, or the question is
unrelated to work. Candidates carry examples (questions people often
ask for that operation) and not_for (other operations that are easily
confused with it).

list_capabilities: "wants to know what operations are available",
  examples: ["What can this do?"]
propose_panel: "wants something shown on screen"
none: "no candidate matches the question (the question is unrelated to
  work)", examples: ["What's the weather today?",
  "What's your favorite food?", "Restart the system"]
```
