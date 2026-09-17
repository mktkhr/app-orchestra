# Instruments

The three measurement instruments this repository runs against a real
model, in data terms only - see `docs/specs/narrowing.md`,
`docs/specs/midsizing.md` and `docs/specs/eval.md` for the prose.

| Instrument           | Size                                                                                                            | Files                                                                                                                                                                                                                                                                                                                        | Run with                                                                                                                                                                                             | Current numbers                                                                                                                                            |
| -------------------- | --------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Narrowing corpus** | 100 questions, 5 axes (A/B/C/D/E: 25/25/25/15/10)                                                               | `e2e/narrowing/corpus/index.ts` (assembles), `axis-a.ts` … `axis-e.ts` (questions + answer keys, each `{id, axis, text, answers[], decoy?}`), `corpus.test.ts` (asserts the shape/counts)                                                                                                                                    | `make eval-shortlist` (narrowing on/off, `WORDING=`, `THINKING=`, `REPEAT_PENALTY=`, `STAGES=`; `PICKER=jev` for the Jev trial, `JEV_CRITERIA=v2`, `GATE=jev`)                                       | The 83/100 baseline the local pick was measured against (`docs/specs/staging.md` section 1); per-round numbers live in each `jev-picker-v*.md`             |
| **Mid subset**       | 60 questions (m01-m60), 30 operations, 3 services (sales/purchasing/attendance), half impossible                | `e2e/narrowing/corpus/mid.ts` (assembles), `mid-answerable.ts` (40: m01-m30 one-per-operation + m31-m40 ten collision questions), `mid-impossible.ts` (20: 5 verb-not-there, 5 resource-not-there, 5 out-of-domain, 5 capability), `mid-collision.ts`, `mid.test.ts` (asserts 60/40/20/30 and the 5 capability-flagged rows) | `make eval-mid` (same `WORDING=`/`THINKING=`/`REPEAT_PENALTY=`/`STAGES=`/`PICKER=`/`JEV_CRITERIA=`/`GATE=` knobs as `eval-shortlist`)                                                                | `docs/specs/midsizing.md`'s own baseline numbers; per-round figures in `jev-gate-v3.md`, `jev-language-v4.md`, `jev-v5.md`                                 |
| **Eval suite**       | 34 cases (18 in `e2e/eval/cases.ts` + 16 in `e2e/eval/cases-real.ts`), each run many times, accept/reject rates | `e2e/eval/cases.ts`, `e2e/eval/cases-real.ts` (real-catalogue refusal cases, `docs/specs/eval.md` section 6a), `e2e/eval/baseline.json` (34 recorded `{total, accept, reject}` rows), `e2e/eval/run.ts`, `e2e/eval/match.ts`, `e2e/eval/report.ts`                                                                           | `make eval` (compares against baseline, `ORCHESTRA_EVAL_MODEL` default `qwen3.5-9b-q8`; `PICKER=jev`, `JEV_CRITERIA=v2`, `GATE=jev` for the Jev trials); `make eval-accept` rewrites `baseline.json` | `e2e/eval/baseline.json`'s 34 rows are the current numbers - e.g. `filter-by-label: 10/10 accept, 0/10 reject`, `no-enum-value: 30/30 accept, 0/30 reject` |

A fourth instrument, added 2026-09-18: **Dialogues** - 12 dialogues, 27
questions, real inventory/attendance services, turns chained from the
platform's own answers. Files: `e2e/dialogue/dialogues.ts` +
`dialogues-more.ts` (corpus), `chain.ts` (turns rule, mirrors
`web/src/features/conversation/model/toContextTurns.ts`), `score.ts`,
`report.ts`, `run.ts`. Run with `make eval-dialogue` (same
`PICKER=`/`JEV_CRITERIA=`/`GATE=` knobs as `make eval`); output to
`e2e/dialogue/out/` (untracked). First run: 26/27 turns, 14/15
follow-ups, 11/12 dialogues (`DECISIONS.md`, 2026-09-18).

Counting note: `grep -c "id:"` over `cases-real.ts` returns 18, not 16 -
two of those matches are `args: { id: "att-002" }` / `args: { id: "itm-001" }`
inside a case's own expected outcome, not a second `id:` field on a case
itself. The real count (`grep -oE '^\s*id:\s*"[a-zA-Z0-9-]+"'`, anchored to
the field's own indentation) is 18 in `cases.ts` and 16 in `cases-real.ts`,
34 total - matching `baseline.json`'s own 34 keys.

None of these three ever runs inside `make check` (`docs/specs/eval.md`,
E4/section 6: each needs a GPU and a model, and a suite that needs one
fails when the GPU is busy). All three are `make` targets that start the
real platform and the real dummy services themselves, never the dev
server, so a run is repeatable regardless of what else is running.
