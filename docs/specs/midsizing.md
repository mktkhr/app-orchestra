# Midsizing: a catalogue that can say no

A measurement subproject. It adds no capability: it adds the one number
the last two days of decisions were missing.

## 1. What it proves

That a change to the planner can be judged on one catalogue for both of
the things a person notices: whether the right operation is called, and
whether an impossible request is refused instead of answered with
something else.

Today there are two instruments and neither measures both:

- The shortlist corpus (`narrowing.md`, `shortlisting.md`): five services,
  a thousand operations, a hundred questions, every one of them
  answerable. It measures picking - and only picking. It cannot reward a
  refusal, so any change that lets the planner refuse reads as a loss on
  it (2026-09-16, "A adopted over B and E": 79 → 76, all four real losses
  escapes to `list_capabilities`).
- The real-catalogue cases in `make eval` (`eval.md`, section 6a): the two
  dummy services, six operations, sixteen questions. It measures refusal
  and fabrication on a catalogue so small that the pick has nowhere to be
  wrong - `att-002の内容` is its only pick failure, and that is a service
  mix-up, not a choice among look-alikes.

Between them there is nothing: no catalogue large enough for the pick to
matter and lacking enough for refusal to matter. Every A/B decision so far
has read one instrument, then the other, and reconciled them by hand.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **M1** | The mid catalogue is a subset of the shortlisting fixture, not a new service. Three of its five services, ten operations each, chosen by hand so that each service keeps two resources with list / get / create / update / delete, and so that the three services collide the way the corpus's axis B does (注文 in both sales and purchasing; 社員 in attendance). The fixture server already serves schema-valid bodies for every operation and every written example already exists; a subset costs a list of thirty operation ids and nothing else.                                   |
| **M2** | The mid fixture's ids carry a `pattern`. Each service's item ids follow one prefix (`so-`, `po-`, `att-` style) declared with OpenAPI's standard `pattern` on the id parameters, so the id affinity rule (`TODO.md`, 2026-09-17; the platform reads `Schema.Pattern`) is exercised on this catalogue too. The full thousand-operation fixture stays as it is.                                                                                                                                                                                                                             |
| **M3** | Sixty questions, in two halves that are scored differently. **Forty answerable**: each names one operation of the thirty as its answer (several where the corpus's axis B would), a third of them carrying an argument (an id, an enum value, a name and a quantity). **Twenty impossible**: an operation the catalogue lacks (delete where only list exists; aggregate; a resource of a service not in the subset), a question outside the domain, and capability questions, general and scoped. The impossible half is what neither instrument has.                                     |
| **M4** | Five numbers, one run. Over the answerable half: **correct@1** and **false refusal** (answered `none` or `list_capabilities`). Over the impossible half: **refused** (`none`, or `list_capabilities` for a capability question) and **forced** (a catalogue operation called or a form offered). Over every form in the run: **fabricated** (an initial string value that appears neither in the question nor in an answer). Latency mean and p50 beside them. The report prints all five; a change is judged on all five, and a record that quotes one without the others is incomplete. |
| **M5** | The runner is the shortlist runner's, with a corpus switch. `e2e/shortlist/run.ts` gains `--corpus mid`, which boots the fixture server on the subset and reads the mid questions; output files carry a `mid-` prefix; `print-report.ts` prints the five numbers for a mid pass. `make eval-mid` is the target. No second runner: the boot, the sign-in, the resumable jsonl and the noise-band caveat (2026-09-16, "determinism band") are already right, and a second copy would drift.                                                                                                 |
| **M6** | The numbers are recorded, not gated. `make eval-mid` is never part of `make check` (it needs the three models resident) and has no `baseline.json`; the current numbers live in `DECISIONS.md` beside the corpus's and the real-catalogue cases', and every planner decision from now on quotes all three instruments.                                                                                                                                                                                                                                                                    |

## 3. The catalogue

Three services from the fixture: **sales**, **purchasing**, **attendance**.
Sales and purchasing collide on 注文 (受注 / 発注) and 取引先; attendance
collides with nothing in the subset, which is deliberate - it gives the
impossible half a service to name a resource _from_ (経費, expense, is not
in the subset, so 経費の明細を見たい is impossible here).

Per service, ten operations: two resources × (list, get, create, update,
delete). The exact thirty ids are listed in the plan, chosen from the
fixture's existing operations so that every one keeps its written
examples (`x-orchestra-examples`, two per operation) - the reranker reads
them (`retrieving.md`), so a subset that dropped them would measure a
different narrower.

The subset's `get`, `update` and `delete` operations declare a `pattern`
on their id parameter: one prefix per service. The fixture generator
(`e2e/narrowing/fixture/openapi.ts`) emits it for the subset only, from
the same table that names the thirty ids.

Narrowing runs as it does on the corpus (K=20 over 30 operations: the
shortlist is most of the catalogue, and the pick still has twenty lines to
choose from - the instrument measures the pick and the fill more than the
narrower, and says so).

## 4. The questions

Sixty, in `e2e/narrowing/corpus/mid.ts`, the corpus's own `Question`
shape plus one field:

```ts
interface MidQuestion extends Question {
  /** "answerable": answers names the operations; "impossible": answers is empty and refusal is the expectation. */
  readonly expect: "answerable" | "impossible";
  /** For "impossible" capability questions: list_capabilities is the refusal, not none. */
  readonly capability?: boolean;
}
```

Answerable (40): written blind to the operations' summaries the way the
corpus was (`narrowing.md`, section 4), spread over the thirty operations,
at least one per operation, with the axis-B collisions represented
(「注文の内容を変えたい」 has two answers). A third carry an argument: an
id in the service's prefix (`so-0012の内容`), an enum value by its Japanese
label, a create with a name and a quantity in the sentence.

Impossible (20): five "the verb is not there" (削除 / 集計 / 承認 on a
resource that only lists), five "the resource is not there" (経費, 在庫,
倉庫 - services outside the subset), five outside the domain (天気, 曜日,
雑談), five capability questions - two general, three scoped to one of
the three services (`capability: true`).

The answer key is reviewed the way the corpus's was: an answerable
question's `answers` lists every defensible operation, and the review
widens it rather than the question.

## 5. What the report says

```
## mid (three services, thirty operations, sixty questions)
answerable (40)   correct@1  ..   false refusal  ..
impossible (20)   refused    ..   forced         ..
forms (n)         fabricated ..
latency (ms)      mean  ..   p50  ..   pick mean ..
misses: <id> <question> → <kind> <operation>   (every answerable miss and every forced impossible)
```

Beside it, unchanged: the corpus's correct@1 / correct@shown and the real
cases' pass count, so the three instruments are read together.

## 6. Deliberately excluded

- **New dummy services in Go.** The fixture server answers every
  operation with a schema-valid body already; planning is what is
  measured, and a real handler adds nothing to it.
- **A baseline gate.** M6. The instrument informs a record; `make eval`
  guards.
- **Alternatives, thinking, wording sweeps on the mid corpus.** They are
  the runner's existing flags and work unchanged; none is measured here
  as part of this subproject.
- **Growing the subset.** Thirty is enough for the pick to have
  look-alikes and for the impossible half to have things to lack. A
  bigger mid corpus is a new decision.

## 7. Acceptance criteria

- **AC-M-101** `make eval-mid` boots the fixture server on exactly thirty
  operations of three services, asks sixty questions through `/api/plan`,
  and prints section 5's report; `make check` calls no model and needs
  nothing running.
- **AC-M-102** The thirty operations each carry their two written
  examples, and every `get` / `update` / `delete` among them declares a
  `pattern` on its id; a unit test asserts both from the generated
  OpenAPI.
- **AC-M-103** The sixty questions type-check against `MidQuestion`; a
  unit test asserts 40 / 20, at least one answerable question per
  operation, every `answers` id in the subset, and every impossible
  question's `answers` empty.
- **AC-M-104** The five numbers are computed by `score.ts` from the jsonl
  and unit-tested on hand-made rows: an answerable `none` counts as a
  false refusal; an impossible `form` counts as forced; a capability
  question's `list_capabilities` counts as refused and a non-capability
  question's `list_capabilities` also as refused; a form's initial with a
  string absent from the question and answers counts as fabricated.
- **AC-M-105** The first run's numbers are recorded in `DECISIONS.md`
  beside the corpus's 77 / 79 and the real cases' 15 / 16, with the row
  ids of every miss.

## 8. What this does not settle

Whether sixty questions written by the same hands as the corpus measure
what a person would ask. The impossible half in particular is easy to
write too obviously (「天気」) and hard to write like a real mistake (「在庫
を減らして」 - a verb the catalogue almost has). The first record will
say which kind it leaned to, and the set is meant to be widened when a
real question exposes a gap, the way the real-catalogue cases were.
