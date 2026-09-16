# Evaluation

The fifth subproject, and the only one that is about the tests rather than the
product.

## 1. What it proves

That a change to the prompt did not quietly break a question nobody was
thinking about.

Every decision in `DECISIONS.md` about model behaviour was measured by hand,
once, and thrown away: which local model invents a filter, whether `ask_user`
gets chosen, whether a follow-up stays on its service. Each measurement was
right when it was taken and is evidence of nothing now. The one that found the
dropped filter came from somebody looking at the screen, not from anything this
repository runs.

`make check` cannot fill that gap and should not try - it must not call a model,
because a suite that needs one is a suite that fails when the GPU is busy. So
this is a second suite, run deliberately.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                                                                                                                                                            |
| ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **E1** | A case is a question and the set of decisions that would answer it. Not one decision: several are often defensible, and pretending otherwise makes a passing suite lie.                                                                                                                                                             |
| **E2** | A case is run many times and scored as a rate. A model is not a function, and a suite that asserts one outcome would be red on a coin toss. A case also names which rate it is scored on - `accept` or `reject` - because not every case is asking the same question of its runs (section 3, section 4).                            |
| **E3** | The suite compares its judged rate against a recorded baseline and fails when that rate moves the bad direction by more than its tolerance: down for a case judged on `accept`, up for one judged on `reject`. What counts as a regression is behaviour getting worse at the thing the case watches, not behaviour being imperfect. |
| **E4** | Never part of `make check`. It needs a model, and `make check` needs nothing.                                                                                                                                                                                                                                                       |
| **E5** | The baseline is updated by a person running a command that says so. A suite that rewrites its own expectations records nothing.                                                                                                                                                                                                     |

## 3. What a case is

```yaml
- id: filter-by-label
  question: 検品保留の在庫を見せて
  accept:
    - {
        kind: result,
        service: inventory,
        operationId: ListInventoryItems,
        args: { status: quarantined },
      }

- id: no-matching-value
  question: 破損した在庫はある？
  metric: reject
  accept:
    - { kind: ask, service: inventory, operationId: ListInventoryItems, param: status }
    - {
        kind: result,
        service: inventory,
        operationId: ListInventoryItems,
        args: { status: quarantined },
      }
  reject:
    - { kind: result, service: inventory, operationId: ListInventoryItems, args: {} }
```

`accept` is what would answer the question. `reject` names an outcome that is
specifically wrong and worth watching - here, the silent drop that returns every
row to somebody who asked for one kind.

A run that matches neither is counted apart. It is not a pass, and it is not the
failure the case was written to watch; it is the model doing something new, and
that is worth seeing on its own.

`metric` says which rate the case is judged on, and defaults to `accept` when
left out. Most cases want that default: `accept` names every defensible
answer, so a case that only ever writes `accept` is asking "does one of the
right answers still happen". `no-matching-value` is not asking that - whether
the model asks for the missing enum value or guesses the closest one is not a
question this case has an opinion on, and letting `accept` swing between the
two would make its rate noise, not signal. What this case actually watches is
narrower: does the specific wrong outcome in `reject` - dropping the filter
silently and returning every row - get more common. `metric: reject` says so,
and flips which direction counts as a regression: for an `accept`-judged case
a falling rate is the regression; for a `reject`-judged case a _rising_ one
is, because the outcome it is tracking is the bad one, not a good one.

## 4. What it reports

```
filter-by-label        5/5 accept   0/5 reject
no-matching-value      2/5 accept   3/5 reject (judged: reject)   ← baseline 3/5 reject
follow-up-stays        5/5 accept   0/5 reject
```

Every line prints both counts regardless of which one the case is judged on
(AC-E-203). `(judged: reject)` names the case that is not judged the default
way - an `accept`-judged case (the majority, and the default when a case
leaves `metric` out) prints no such note, so the six cases that have always
looked like this still do.

The judged rate moving against its baseline by more than a tolerance fails
the run - down, for a case judged on `accept`; up, for one judged on
`reject` - and the trailer names which count and which direction it compared
(`baseline 3/5 reject` above, not `accept`). A rate that moved the good
direction passes and says so too: improvements are as worth seeing as
regressions, and a suite that only ever reports bad news gets ignored.

## 4a. What a rate can and cannot settle

A rate is evidence about a regression, not about an improvement, and the
corpus is honest about which it is being asked for.

`no-enum-value`'s reject count was measured nine times at n=30 under changes
that all turned out to be the same behaviour: 16, 19, 19, 19, 18, 20, 16, 15, 22. That band is 0.23 wide. `ORCHESTRA_EVAL_TOLERANCE` at 0.3 sits just
outside it, which is what makes the case work as intended - a filter that
began to drop outright would clear the tolerance and fail the run - and it is
also why nothing smaller can be read out of it. An intervention that moves the
true rate by a tenth is invisible here, in either direction.

So a case's rate answers "did this get worse in the way the case watches".
It does not answer "did this change help", and a run whose number moved the
good direction by less than the tolerance is reported (section 4) without
being evidence of anything. Settling a question that small would need n in
the hundreds - tens of minutes of GPU for one measurement - which is a price
no suite should charge on every change (`DECISIONS.md`, 2026-09-12).

## 5. What it runs against

The platform, started by the suite with the real planner and the model
`ORCHESTRA_EVAL_MODEL` names. Not the dev server: a suite that measures whatever
happens to be running measures nothing repeatable.

The services are the same dummy pair, seeded the same way, because a case's
expected operation is meaningless against a different catalogue.

## 6. Deliberately excluded

- **Judging the answer's prose.** There is none; the platform answers with a
  call and a component.
- **Grading with a model.** The expected outcomes here are a service and an
  operation id and a few arguments. Comparing them is `==`, and a judge would
  add a second model's opinions to a measurement of the first one's.
- **Running in CI.** It needs a GPU. The baseline in the repository is how a
  reviewer sees the numbers without one.

## 6a. Real-catalogue cases

`e2e/eval/cases-real.ts` (appended into `cases` by `cases.ts`) is a second
family of cases, run alongside section 5's fixture corpus rather than
replacing it. It exists because refusal is only measurable on a catalogue
that lacks the thing being asked for - a delete, a sales figure, an overtime
total - and the fixture corpus, built to exercise every operation the
platform has, cannot lack anything by construction. The real dev
inventory/attendance services can, so this is where "does the model refuse
what it should" gets checked.

The families:

- **Refusals.** A question the catalogue has no operation for
  (delete/decrease/sales/overtime, and a plain "what day is it") - `accept`
  is a plain `none` or a `list_capabilities` redirect (either counts as
  refusing), `reject` is a `result` naming the nearest real operation
  anyway.
- **A capability question scoped to one service** ("what can I do with
  inventory?"), distinct from the unscoped `capability` case in the fixture
  corpus.
- **No fabrication.** A create request missing a required field ("I want to
  log being late", "register a new item", "apply for paid leave") - `accept`
  is a form (or an `ask`, or a `none`) that leaves the missing field out of
  `initial`; `reject` is a form that filled it in with a guess.
  `ExpectedOutcome.argsAbsent`/`argsPresent` (`match.ts`) check a field's
  presence regardless of what value would have been guessed, which `args`
  alone cannot: `args` only checks values a case pins down, not the absence
  of one it does not name.
- **Right answers that must hold.** Questions the catalogue does answer, run
  against the real seed data (an id lookup, a status/kind filter, a create
  with values named in the question, a follow-up with no filter the
  operation supports) - watching that the honest answer keeps happening
  once refusal is also being asked of the same model.

Like every case in this suite, a real-catalogue case's expectations are the
product's decisions, not a transcript of what the platform answers today: a
case can fail on the day it is added and still be worth keeping, recorded as
failing by the baseline exactly as `no-enum-value` was (section 3).

## 7. Acceptance criteria

- **AC-E-201** `make eval` runs every case, prints a rate per case, and exits
  non-zero when one has moved against its baseline, on the rate it is judged
  on (`metric`), by more than the tolerance.
- **AC-E-202** `make check` still calls no model, and `make eval` is not part of
  it.
- **AC-E-203** A recorded reject outcome appearing at all is reported, whether
  or not the judged rate held.
- **AC-E-204** `make eval-accept` rewrites the baseline from the last run, and
  nothing else does.
- **AC-E-205** The corpus covers, at least: a filter named by its label, a
  filter named by a word the enum does not have, a question asking for
  everything, a create, a question no service answers, a capability question,
  and a follow-up that names no service.
