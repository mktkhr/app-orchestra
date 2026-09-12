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

|        | Decision                                                                                                                                                                |
| ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **E1** | A case is a question and the set of decisions that would answer it. Not one decision: several are often defensible, and pretending otherwise makes a passing suite lie. |
| **E2** | A case is run many times and scored as a rate. A model is not a function, and a suite that asserts one outcome would be red on a coin toss.                             |
| **E3** | The suite compares against a recorded baseline and fails on a drop. What counts as a regression is behaviour getting worse, not behaviour being imperfect.              |
| **E4** | Never part of `make check`. It needs a model, and `make check` needs nothing.                                                                                           |
| **E5** | The baseline is updated by a person running a command that says so. A suite that rewrites its own expectations records nothing.                                         |

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

## 4. What it reports

```
filter-by-label        5/5 accept   0/5 reject
no-matching-value      2/5 accept   3/5 reject   ← baseline 4/5 accept
follow-up-stays        5/5 accept   0/5 reject
```

A rate below its baseline by more than a tolerance fails the run. A rate above
it passes and says so: improvements are as worth seeing as regressions, and a
suite that only ever reports bad news gets ignored.

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

## 7. Acceptance criteria

- **AC-E-201** `make eval` runs every case, prints a rate per case, and exits
  non-zero when one has dropped below its baseline beyond the tolerance.
- **AC-E-202** `make check` still calls no model, and `make eval` is not part of
  it.
- **AC-E-203** A recorded reject outcome appearing at all is reported, whether
  or not the accept rate held.
- **AC-E-204** `make eval-accept` rewrites the baseline from the last run, and
  nothing else does.
- **AC-E-205** The corpus covers, at least: a filter named by its label, a
  filter named by a word the enum does not have, a question asking for
  everything, a create, a question no service answers, a capability question,
  and a follow-up that names no service.
