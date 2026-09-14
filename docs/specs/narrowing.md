# Narrowing a catalogue nobody can send whole

The fourteenth subproject. It does not choose how to narrow a catalogue. It
builds the ground that choice has to stand on: a catalogue of a thousand
operations, a corpus that is hard in the ways a real one is, a measurement
that says how well a candidate narrowing did, and the cheapest narrowing
there is, measured first, so every later one has something to beat.

## 1. What it proves

`PRODUCT.md` D2 says the service catalogue is sent whole on every request and
kept warm in the prompt cache, and rejects staged narrowing on cost: several
times the price, three times the latency. That decision was taken against a
catalogue of six operations. At the scale the product is for - five services
of a couple of hundred operations each - sending the catalogue whole is not
an expensive choice. It is not a choice.

What broke first was not cost. It was precision. Measured 2026-09-14
(`DECISIONS.md`): adding a _seventh_ tool moved `qwen38-27b-iq3s` from 10/10
to 7/10 on a question that has nothing to do with it. Seven tools. Whatever
happens at a thousand is not an extrapolation of that curve; it is a
different problem, and today there is no way to look at it.

So the thing that has to be found is a way to put on the order of twenty
operations in front of the model instead of a thousand - cheaply, and
**without the LLM doing the choosing**. An LLM choosing which tools to offer
is D2's cost argument again, one call earlier. The work has to be pushed
outside the model: a lexical index, a vector store, a decided hierarchy, or
something else.

Nothing here picks which. Picking one now would mean picking it against a
catalogue of six, which is the mistake this subproject exists to stop. What
it proves is narrower and comes first: that the question can be asked at all.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                                                                                                                                                                                                                     |
| ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **T1** | The baseline is measured before a mechanism is designed. The cheapest narrowing that could work - lexical, no model - ships and is measured first. A vector store's value is the distance between it and that number, and without the number there is nothing to justify it with.                                                                                                            |
| **T2** | The measurement does not call an LLM. A narrowing is judged on whether the right operation is in the shortlist it produced (recall@K), which is a property of the shortlist. That makes a full run seconds instead of the ten minutes a corpus of LLM calls costs, so a mechanism can be changed and re-measured in a loop.                                                                  |
| **T3** | The difficulty is designed, not generated. A thousand operations produced by permuting nouns and verbs is easy for every mechanism and says nothing. The ways a real catalogue is hard are enumerated in section 4, each is built in deliberately, and **each is measured separately** - a single overall score hides the one axis a mechanism died on.                                      |
| **T4** | The fixture catalogue is not a set of services. It lives outside `services/`, is not built, not tested, and not part of `make check`. Five more Go services would add build, test and acceptance weight to every check in the repository and contribute nothing to what is being measured.                                                                                                   |
| **T5** | The fixture is a definition table, and the contracts are generated from it at serve time. What a person reads and edits is a table of services, resources, verbs and summaries - a few hundred lines. The OpenAPI documents are synthesised in memory by the process that serves them, so tens of thousands of generated lines never enter the repository, the file-length guard, or a diff. |
| **T6** | The scale is a dial. One, two, three or five services - two hundred to a thousand operations - selected at run time. A mechanism's answer is a curve, not a number, and the curve is the interesting part.                                                                                                                                                                                   |
| **T7** | The existing eval corpus is not reused. Its eighteen cases were written against six operations; every one of them is trivially answerable when the catalogue is six and says nothing about a thousand. The new corpus is separate, and `make eval` keeps running the old one against the real services.                                                                                      |

## 3. The fixture catalogue

Five services, chosen so that names collide the way they collide in a real
estate of systems rather than because collisions were added:

| Service | Japanese   | Why it is here                                           |
| ------- | ---------- | -------------------------------------------------------- |
| 在庫    | inventory  | already exists, and owns the noun the corpus starts with |
| 販売    | sales      | 受注 - collides with purchasing on 注文, 明細, 取引先    |
| 購買    | purchasing | 発注 - the other half of that collision                  |
| 勤怠    | attendance | already exists; shares 社員 and 承認                     |
| 経費    | expense    | 申請 and 承認 workflows, shared with purchasing          |

The collisions this produces are the point: 注文 (受注 / 発注), 明細 (販売 /
購買 / 経費), 承認 (経費 / 購買 / 勤怠), 社員 (勤怠 / 経費), 取引先 (販売 /
購買). None of them had to be invented.

Two hundred operations per service, composed rather than drawn at random:

- **150 - core resource CRUD.** Thirty resources, five verbs each
  (list / get / create / update / delete).
- **20 - search and aggregate.** `search*`, `summarize*`, `aggregate*`, over
  the same nouns the CRUD operations use.
- **20 - settings and master data.** `get*Setting`, `update*Threshold`,
  `list*Category` - reusing the transactional nouns, which is where axis E
  comes from.
- **10 - workflow.** `submit*`, `approve*`, `reject*`, `withdraw*`, the same
  shape in every service, which is where axis B comes from.

`operationId` stays unique across services - `listSalesOrders` and
`listPurchaseOrders` - because `harness/guard/operation-ids.sh` requires it
and because real systems do the same thing. **The collision is in the
summaries and display names, not the ids**, which is also true of real
systems and is exactly why an id-matching approach to narrowing would look
better here than it deserves to.

## 4. The five axes of difficulty

Each axis is a category in the corpus and a column in the report.

**A - the verb carries no selectivity.**
一覧 matches 180 operations out of a thousand (measured, 2026-09-14). Every
service lists things. Choosing by verb narrows a catalogue by a fifth and no
further; all of the selectivity is in the noun. The fixture guarantees this
rather than letting it emerge.

**B - the same resource name in more than one service.**
「注文を一覧」 has a defensible answer in 販売 and another in 購買, and the
question does not decide between them. Some of these questions have
`ask_user` as their right answer; a narrowing that returns only one service's
operation has removed the platform's ability to ask.

**C - near-neighbours inside one service.**
在庫品目 / 在庫ロット / 在庫引当 / 棚卸 / 在庫調整. 「在庫を見たい」 has five
roughly equal candidates. Three such groups per service.

**D - vocabulary gap.**
The words the question uses do not appear in the operation's summary.

| Question               | Summary, as the fixture writes it | Operation                  |
| ---------------------- | --------------------------------- | -------------------------- |
| 休みたい               | 有給休暇の一覧                    | `listAttendancePaidLeaves` |
| PO を出したい          | 発注の作成                        | `createPurchasingOrder`    |
| 立て替えた分を出したい | 経費申請の作成                    | `createExpenseClaim`       |

Each of these shares **no character bigram at all** with the operation that
answers it, measured against the fixture rather than asserted. 品切れ → 在庫切れ
品目の一覧 was the obvious fourth and does not qualify: the two share 切れ, so a
lexical mechanism can find it and the question is not measuring this axis. A
question that half-overlaps belongs in axis A or C.

A lexical mechanism cannot answer these; that is not a flaw in the fixture,
it is the measurement. **The distance between a lexical baseline and a
semantic one is this axis and almost nothing else**, which is why it is
built in at a fixed proportion rather than left to chance.

**E - a decoy that is lexically closer than the answer.**

> Question: 先月の残業時間
> Answers: `listAttendanceRecords` (勤怠記録の一覧, which does not contain the
> word 残業 at all) and `listAttendanceOvertimes` (残業の一覧, which contains
> 残業 but not 時間)
> Decoy: `getAttendanceOvertimeThreshold` - summary **残業時間**の上限設定を
> 取得, which carries every character bigram of 残業時間 and answers none of
> them

Settings and master-data APIs are named after the transactions they
configure, so a real catalogue is full of these. A lexical mechanism ranks
the decoy first. A semantic one is not obviously safe either - the decoy is
close in meaning as well as in letters. This is the axis where **both**
candidate mechanisms may fail, and the only way to find that out is to score
it on its own.

## 5. The corpus

One hundred questions, each a question in Japanese and the `operationId`
that answers it (or the set that does, for axis B).

| Axis | Questions |
| ---- | --------- |
| A    | 25        |
| B    | 25        |
| C    | 25        |
| D    | 15        |
| E    | 10        |

D and E are the minority because they are the minority in real use. They are
not diluted by that, because they are scored separately: fifteen questions
is enough to see a mechanism answer none of them.

## 6. What is measured

**recall@K** - the share of questions whose right answer is somewhere in the
K operations the narrowing returned. Reported per axis and overall, for
K = 10, 20 and 50, at each of the four catalogue sizes.

K is not a constant to be chosen in advance. How small K can get before
recall falls is half the result: it is the number that says how many
operations the model has to be handed, and therefore whether any of this
helps.

Wall-clock time per query is reported alongside, because a narrowing that
costs more than the call it is narrowing has not narrowed anything.

No LLM is called. The end-to-end question - does the model then pick
correctly out of K - is a later measurement against the real corpus, and it
only becomes worth running once a narrowing survives this one.

## 7. The baseline

Character-bigram lexical scoring over each operation's summary, description,
display name and service display name. No model, no embedding service, no
index to keep warm; the index is built when the catalogue is read and is a
map.

It is chosen for being the weakest thing that is not a strawman. Japanese
without a tokeniser scores acceptably on character bigrams, so this is a real
baseline rather than one built to lose. What it cannot do is axis D, by
construction.

If it reaches usable recall at a usable K, a vector store is unnecessary and
the catalogue stays a file. If it does not, the shortfall is measured, per
axis, and that measurement is the argument for whatever comes next.

## 8. Deliberately excluded

- **Choosing the narrowing mechanism.** That is the next subproject, and its
  input is this one's output.
- **Rewriting D2.** D2 rejects staged narrowing _by the LLM_ on cost grounds,
  and nothing here contradicts it. When a mechanism is chosen, `PRODUCT.md`
  will need to say what the catalogue now is; that is a product decision and
  is not taken here.
- **The fixture answering calls.** The catalogue is what is measured. The
  fixture serves contracts; it does not implement the operations in them, and
  `/api/invoke` against a fixture operation is not part of this.
- **`make check`.** The fixture, the corpus and the measurement are run
  deliberately, like `make eval`. None of them join the gate.
- **Permission narrowing.** `catalogFor` (`docs/specs/auth.md` A4) already
  narrows a catalogue by what a person may see. It is a different filter for
  a different reason and is not touched.

## 9. Acceptance criteria

- **AC-T-101** The fixture serves five OpenAPI contracts the platform's
  existing spec source can read, with no change to `specsource/http`.
- **AC-T-102** The catalogue the platform builds from the fixture holds 1000
  exposed operations, and each of the four sizes holds 200, 400, 600 and 1000.
- **AC-T-103** Every fixture contract passes the same contract rules the real
  services do - unique operation ids across services, `x-enum-labels` on every
  enum, an `x-ui-hint.displayName` on every exposed operation.
- **AC-T-104** The repository holds the definition table, not the generated
  contracts: no generated OpenAPI document is committed.
- **AC-T-105** The corpus holds 100 questions in the 25/25/25/15/10 split, and
  every answer names an operation the fixture actually serves.
- **AC-T-106** The measurement runs against a fixture catalogue without a
  running LLM and reports recall@K per axis, overall, for K = 10/20/50 at each
  of the four sizes, with per-query wall-clock.
- **AC-T-107** The lexical baseline is measured and its numbers recorded in
  `DECISIONS.md`, per axis - including the axes it fails.

## 10. What this does not settle

The fixture is a guess at what a real estate of systems looks like. Its five
services are plausible and its collisions are real, but a company's actual
catalogue will be harder in ways nobody listed here - inconsistent naming
between teams, operations nobody can explain, three generations of the same
API alive at once. A mechanism that works here has not been proven; it has
stopped being disproven.

The corpus is likewise written by the same hand that wrote the fixture, which
is the oldest problem in evaluation. What limits the damage is that the five
axes were named before the questions were, and that a mechanism is scored on
each of them separately - it is hard to accidentally write a hundred
questions that flatter a mechanism that does not exist yet.
