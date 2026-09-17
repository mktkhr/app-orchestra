# Chip ordering — would Jev's probabilities rescue more than the reranker?

Date: 2026-09-17 (JST). Repository: app-orchestra, branch main, at
`94ef689` (clean). Offline analysis: no model call of any kind was made to
produce or check this record (see "No model calls" below). This is the
first of three follow-ups to the Jev trial (`docs/measurements/jev-picker-v1.md`,
`jev-picker-v2.md`, `jev-gate-v3.md`, `jev-language-v4.md`, `jev-v5.md`).

## Question

`docs/specs/shortlisting.md` H5: under a `result`, the browser shows the
next two operations of the shortlist as 「違いましたか？」 chips. Today
those two are the shortlist's next two **in reranker order**
(`bge-reranker-v2-m3-q8`). Against the 100-row corpus, `correct@1` (the
chosen operation is right) is 78/100 and `correct@shown` (the chosen
operation or one of the two chips is right) is 79/100 — the chips rescue
exactly one row today.

The Jev picker (v1 and v2 trials) returns a full probability distribution
over the same shortlist candidates, for the same 100 questions. If the
two chips had instead been the two highest-probability candidates from
that distribution (excluding whatever the local pick already chose),
would more rows have been rescued?

## Data and provenance

- `docs/measurements/jev-picker-v2-corpus-picks.jsonl` — 100 rows,
  `{id, question, choice, confidence, probabilities{operationId: p}, usage,
ms}`, the v2 trial's picks over this corpus, using each row's own
  post-narrowing shortlist as the candidate set. `jev-picker-v1-corpus-picks.jsonl`
  is the same corpus under v1's distribution (used only for the
  robustness check in section 6).
- The local run: `nocr-stages2.jsonl` (produced by an earlier session,
  `ORCHESTRA_PICKER` unset — i.e. local, not Jev), 100 rows,
  `{id, text, answers, kind, operationId, alternatives[], latencyMs}`.
  `alternatives` is exactly what the browser rendered as chips, in
  reranker order — the field H5 describes. Kept out of the repo (a
  scratchpad artifact of a prior measurement session); copied into this
  analysis as-is, not regenerated.
- Answer keys: `e2e/narrowing/corpus/*.ts` (via the `answers` field
  already baked into `nocr-stages2.jsonl`).
- All three files cover the identical 100 ids (`a01..e10`-style);
  verified by set equality before any computation (the script asserts
  this and fails loudly otherwise).

All three inputs were already on disk before this analysis began. No
`orchestra` command was run and no HTTP request was made to produce
them or to check them here.

## Method — the exact counterfactual

The local pick's own answer is **not re-run**. For each of the 100 rows,
what changes is only the two chips shown beside the already-fixed local
answer:

1. Take the local pick's `operationId` and `answers` from `nocr-stages2.jsonl`
   as given.
2. **Reranker chips** (today, as shipped): `alternatives` from the same row.
3. **Probability chips**: from the v2 row with the same `id`, sort
   `probabilities` descending (ties broken by operationId string, for
   determinism), drop the local pick's own operationId from the list if
   present, take the top _k_.
4. `correct@shown` for a scheme = local pick is in `answers`, OR any
   shown chip is in `answers`.

**Chips only exist on `kind:"result"` rows.** H5 says the alternatives
turn is attached to a `result` from `/api/plan`; rows of `kind:"form"`
(26) or `kind:"none"` (1) never show a chip UI at all in the browser, so
no ordering scheme can change their outcome — for those 27 rows,
`correct@shown` is fixed at `correct@1` under every scheme, including
today's. 73 of the 100 rows are `kind:"result"` and carry `alternatives`;
all reordering happens only within those 73. This is why re-deriving
`correct@1 = 78` and `correct@shown(reranker) = 79` from the raw data (a
sanity check before anything else) matches the numbers given in the
brief exactly.

Script: `/tmp/claude-1000/-home-takahiro-ghq-github-com-mktkhr-app-orchestra/24470cde-9051-443e-b7b6-397284bcb3c1/scratchpad/chips-order/analyze.py`
(kept in the scratchpad, not the repo, per the task's instruction; a copy
of its full per-row output is committed below as
`jev-chip-order-rows.jsonl` so every number in this document can be
recomputed from that file without rerunning anything or having access to
the script).

## Results — five orderings

| #   | Scheme                                  | Chips shown | correct@shown | Δ vs correct@1 (78) | Δ vs today (79) |
| --- | --------------------------------------- | ----------- | ------------: | ------------------: | --------------: |
| 1   | 今のまま (reranker order, today)        | 2           |            79 |                  +1 |               — |
| 2   | 確率順に二つ (probability order, top 2) | 2           |            85 |                  +7 |              +6 |
| 3   | 確率順に三つ                            | 3           |            86 |                  +8 |              +7 |
| 4   | 確率順に四つ                            | 4           |            86 |                  +8 |              +7 |
| 5   | 確率順に五つ                            | 5           |            86 |                  +8 |              +7 |

The curve saturates fast: going from 2 to 3 probability-ordered chips
rescues one more row (85 → 86); 4 and 5 add nothing further. All the
saturation gain beyond _k_=2 comes from a single row (`b06`, see below) —
every other miss that probability ordering can catch at all, it already
catches at _k_=2.

## Overlap between the two orderings

Among the 73 chip-bearing rows, comparing the reranker's two shown chips
against the probability order's top two (excluding the local pick):

| Chips in common | Rows |
| --------------- | ---: |
| 0 of 2          |   41 |
| 1 of 2          |   32 |
| 2 of 2          |    0 |

Not one of the 73 rows has the reranker and the probability order
agreeing on both chips; mean overlap is 32/73 ≈ 0.44 chips in common.
The two rankings are, in practice, ordering almost entirely different
candidates — reranker score and Jev's own probability are picking up
different signal.

## Rescued and lost rows

Reranker → probability-order (k=2), keeping the local pick fixed:

**Rescued (6)** — reranker's two chips missed, probability order's two
would have caught it:

| id  | question                                 | correct answer(s)                                                            | local pick             | reranker chips (shown today)                               | probability-order chips                                                |
| --- | ---------------------------------------- | ---------------------------------------------------------------------------- | ---------------------- | ---------------------------------------------------------- | ---------------------------------------------------------------------- |
| a21 | 苦情申立って今どうなってる？             | listAttendanceGrievances, searchAttendanceGrievances                         | getSalesComplaint      | listAttendanceLeaveRequests, getInventoryAllocation        | getAttendanceGrievance, **listAttendanceGrievances**                   |
| b06 | 明細をまとめて見たい                     | listSalesOrderLines, listPurchasingOrderLines, listExpenseLines              | summarizeSalesInvoices | listExpenseSettlements, listExpenseRecurringExpenses       | **listExpenseLines**, propose_panel                                    |
| c02 | 入出庫を一覧したい                       | listInventoryReceivings, listInventoryShipments                              | list_capabilities      | listPurchasingReceivingInspections, listInventoryPickLists | propose_panel, **listInventoryReceivings**                             |
| c13 | 仕入先の評価や契約を知りたい             | listPurchasingContracts, listPurchasingSupplierEvaluations                   | list_capabilities      | listInventorySuppliers, listPurchasingBids                 | getPurchasingSupplierEvaluation, **listPurchasingSupplierEvaluations** |
| d06 | 出荷の準備をしたい                       | createInventoryShipment, createInventoryPickList, createInventoryPackingList | list_capabilities      | createPurchasingDirectShip, createSalesDeliveryNote        | **createInventoryShipment**, propose_panel                             |
| e03 | 日当単価をもとに支給された分を確認したい | listExpensePerDiems, aggregateExpensePerDiems                                | listExpensePayments    | getInventoryCostLayerMethodSetting (1 chip — see note)     | **listExpensePerDiems**, **aggregateExpensePerDiems**                  |

**Lost (0)** — no row where the reranker's two chips caught the answer
and the probability order's two would have missed it.

Note on `e03`: `nocr-stages2.jsonl` records only **one** reranker chip
for this row (`alternatives` has length 1, the only such row in the
corpus), not two — the post-narrowing shortlist evidently had only two
candidates total that round, so there was only one "next" operation
after the local pick to show. `correct_shown` for the reranker scheme
was computed against that single chip as recorded, not padded to two;
it is still a miss either way, so it does not change which rows are
counted rescued.

The single row responsible for the 85 → 86 gain between _k_=2 and _k_=3
is `c21` (「経費申請にまつわる書類を確認したい」, correct answers
`listExpenseReceipts`/`listExpenseTravelExpenses`, local pick
`listExpenseClaims`). Its top-2 probability chips are `getExpenseClaim,
propose_panel` (a miss, so `c21` is not in the rescued-at-k=2 table
above); its top-3 chips add `listExpenseReceipts`, which is correct.
`c21`'s reranker chips also miss, so this row is a genuine second rescue
that only a three-chip design would additionally catch — it just is not
in the k=2 table above because k=2 doesn't reach it.

## Robustness check — v1's distribution

Repeating the k=2 probability-order scheme with `jev-picker-v1-corpus-picks.jsonl`
(the same 100 questions and shortlists, a different run of the same
underlying model, without v2's richer criteria):

`correct@shown (probability order, k=2, v1) = 85` — identical to v2's 85.

The rescued set is **identical**, id for id: `a21, b06, c02, c13, d06,
e03`. Zero rows lost under v1 either. The conclusion (probability
ordering rescues 6 more rows than the reranker's current order, at k=2)
does not depend on which of the two Jev trial runs' probabilities are
used.

## No model calls

`docker logs llama-swap 2>&1 | grep -c 'POST /v1/'` was checked before
and after this entire analysis: **127359** both times. No network call
was made by the analysis script (`analyze.py` only opens local files and
does arithmetic); no `orchestra` binary was run.

## Caveats

- **This does not test the local configuration.** The probabilities
  used here came from a Jev call the product does **not** make when
  `ORCHESTRA_PICKER` is unset (the local/default configuration this
  corpus's `nocr-stages2.jsonl` run itself used). "Use Jev's
  probabilities to order the chips" as a real feature would mean one
  extra remote call per question that the local pick did not previously
  make. From the v2 corpus's own recorded `ms` field, the median call
  latency across the 100 rows is in the ~200 ms range (`jev-picker-v2.md`
  documents its own latency figures in full); treat **~230 ms and
  ~$0.00003 per question** (the order of magnitude quoted in the task
  brief, consistent with the trial's own recorded token counts and
  per-token pricing) as the added cost per question, not a new
  measurement produced here.
- **The pick was still Jev's own pick, not the local pick's, when the
  probabilities were generated.** In every row analyzed above, the
  `probabilities` distribution was produced by a call in which Jev
  itself was choosing the operation (`choice` in the v1/v2 files); the
  local pick's own answer (from a completely separate, non-Jev run) was
  substituted in afterward as "the thing that's already chosen" and the
  chips were reordered around it. This is a legitimate counterfactual on
  the rows as they exist, but it is **not** the same as asking "if Jev
  had been given the local pick's already-made choice and asked only to
  rank the remaining candidates, would its distribution look the same."
  The two pickers may attend to different signal specifically because
  one has already committed to an answer and the other has not. This
  analysis does not, and cannot, rule that out — it shows what the
  existing recorded distributions would have done if borrowed for
  reordering, nothing about what a distribution conditioned on the local
  pick's choice would look like.
- 27 of the 100 rows (`kind:"form"` or `kind:"none"`) have no chip UI at
  all under H5's current design; nothing in this analysis bears on
  those rows. Their `correct@1` outcome (21 of 27 already correct) is
  fixed under every scheme, so no ordering, however good, can move
  `correct@shown` for the 100-row total past `21 (fixed, no-chip rows) +
(correct-or-rescuable among the 73 chip-bearing rows)`.

## Files

- `docs/measurements/jev-chip-order-rows.jsonl` — one row per corpus
  question: `{id, question, answers, localChoice, localKind,
localAlternatives, jevChoice, jevConfidence, jevProbabilitiesTop5,
chipsByRerank, chipsByProbability, correctAt1, correctAtShownRerank,
correctAtShownProbability}`. Every number in this document is
  recomputable from this one file (see the "no rerun" checks above:
  `correct@1 = 78`, `correct@shown(rerank) = 79`,
  `correct@shown(probability, k=2) = 85`, all reproduced by summing the
  boolean fields).
- Script: `/tmp/claude-1000/-home-takahiro-ghq-github-com-mktkhr-app-orchestra/24470cde-9051-443e-b7b6-397284bcb3c1/scratchpad/chips-order/analyze.py`
  (scratchpad only, not committed to the repo).
