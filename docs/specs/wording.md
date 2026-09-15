# What the planner is told

The eighteenth subproject. The product's planner, handed the same twenty
candidates as a stand-in picker, chooses right 65 times in 100 where the
picker chooses right 83 (`DECISIONS.md`, 2026-09-15, "The product's planner,
measured end to end"). Nothing between them differs except the words: the
system prompt, the descriptions of the built-in tools, and what each
catalogue tool says about itself. This subproject changes only those words,
and measures every version of them the same way.

## 1. What it proves

The eighteen-point gap has a shape. From the miss list beside the picker:

- `none` where the picker commits - on the ambiguous axis and the
  vocabulary axis most.
- `list_capabilities` returned as the answer to a vague question (five
  times in 100). The picker has no such tool and simply picks.
- One ask in 100. The picker flagged half of the ambiguous axis as
  ambiguous; the product's `ask_user` was described in a way that the model
  almost never reaches for.
- On the vocabulary axis, a `form` for a write it has misidentified.

Every one of these is behaviour the words invite or fail to forbid. And the
words are known to matter far more than most other levers: one added
sentence moved the stand-in picker eleven points on a shortlist that did not
change (`DECISIONS.md`, 2026-09-15, "Letting the reranker read…").

That last fact decides the method. A prompt is not edited; a **wording** is
a named, versioned thing, and every candidate is measured beside the
current one on the same run before any of them becomes the default.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Q1** | The planner's words are a named set - system prompt, `ask_user`, `list_capabilities` and `propose_panel` descriptions, and how a catalogue tool's description is built - selected by name. `v1` was the text in the product at launch, byte for byte, and stayed the default until a recorded decision changed it: it did, 2026-09-15 - see Q5 below.                                                                                                                                                                                                                                                       |
| **Q2** | Candidates are written against the measured misses, not against taste. Each names the miss it targets, and the measurement says whether it moved that axis and what it cost elsewhere.                                                                                                                                                                                                                                                                                                                                                                                                                      |
| **Q3** | Every candidate is measured in one run of the product measurement (`make eval-shortlist`, narrowing on, K=20), one pass per wording, beside `v1`. Planning is deterministic at temperature 0 (two runs, 100 of 100 identical), so one pass per wording is a measurement, not a sample.                                                                                                                                                                                                                                                                                                                      |
| **Q4** | The other planner corpus (`make eval`, 18 cases against the real services) is run for the wording that would become the default. A wording that helps the fixture and hurts the real services is not adopted.                                                                                                                                                                                                                                                                                                                                                                                               |
| **Q5** | The default changes by decision, recorded with both tables. The environment variable that selects a wording exists for measurement; the product ships one default. Decided 2026-09-15 (`DECISIONS.md`, "wording: v2-commit becomes the default"): `v2-commit` became `wording.Default()`. Decided again 2026-09-16 (`DECISIONS.md`, "wording: v6-unmatched-filter becomes the default"): `v6-unmatched-filter` is now `wording.Default()` and what `ORCHESTRA_PLANNER_WORDING` unset resolves to; `v1` and `v2-commit` both stay selectable by name, and `v1` is what its byte-identity test still asserts. |

## 3. What a wording is

```
wording
  name                 "v1", "v2-commit", ...
  systemPrompt         the toolcall planner's system message
  askUser              description of ask_user
  listCapabilities     description of list_capabilities
  proposePanel         description of propose_panel
  catalogueTool        how a catalogue operation's tool description is built
                       from the endpoint (today: its summary)
```

Selected by `ORCHESTRA_PLANNER_WORDING`; unset means `v6-unmatched-filter`
(decided 2026-09-16, replacing `v2-commit` decided 2026-09-15 - Q5).
Unknown is a startup error. `v1`'s text is asserted equal
to the literals as of `5bf5cf8` so that the baseline cannot drift under a
refactor - by name (`wording.ByName("v1")`), not via `wording.Default()`,
now that `v1` is no longer the default.

`catalogueTool` is in the set because it is a lever nobody has measured on
this planner: today a tool's description is the operation's summary. The
written examples that lifted retrieval never reach the planner. Showing them
to the stand-in picker made it worse; whether that holds for a tool-calling
planner is a question for the run.

## 4. The candidates

Four to start. Each is a delta from `v1`, small enough to attribute.

- **`v2-commit`** - targets `none` and `list_capabilities`. The prompt says:
  when any offered operation plausibly serves the question, call it;
  `list_capabilities` is for a person asking what the system can do, not for
  a question the tools could answer.
- **`v3-ask-on-collision`** - targets the one ask in 100. The prompt and
  `ask_user`'s description say: when two or more offered operations differ
  only by which service owns them (受注 / 発注, 経費 / 勤怠 の 社員), do not
  guess - ask, naming both.
- **`v4-commit-and-ask`** - both.
- **`v5-examples-in-tools`** - `v1`'s words, with each catalogue tool's
  description carrying its `x-orchestra-examples` after the summary. The
  question section 3 raises.
- **`v6-unmatched-filter`** - built on `v2-commit`, targets TODO.md item 3:
  a question whose restricting word matches no enum value silently drops
  the filter and returns every row. The system prompt and `ask_user`'s own
  description both say: when a restricting word matches none of an enum
  parameter's values, call `ask_user` for that parameter rather than
  dropping the filter. Measured (`DECISIONS.md`, 2026-09-16, "wording:
  v6-unmatched-filter becomes the default"): `make eval`'s two
  `no-enum-value` cases move from 30/30 and 10/10 reject under `v2-commit`
  to 0/30 and 0/10 reject, every accepted outcome an `ask_user` call on the
  parameter; correct@1 on the `make eval-shortlist` corpus costs three
  points against `v2-commit` (67 vs 68), traced mostly to a pre-existing
  repetition-loop truncation rather than the new rule. Adopted as the
  default despite that cost - closing a real service defect outweighs it.

More can be added; each is a name and a delta and gets its own row.

## 5. What is measured

`make eval-shortlist` gains `--wording <name>[,<name>…]`; the report prints
one block per wording, `v1` first, with the same columns as today - correct@1,
correct@shown, asked, none, error, latency, per axis and overall - and the
stand-in picker's row beside them. The miss list per wording is written next
to the rows so a candidate's failures can be read, not only counted.

For the wording that would become the default, `make eval` is run against
the real services and its table is recorded beside the fixture's.

## 6. Deliberately excluded

- **Changing the planner's mechanics.** Tool calling, strict schemas,
  temperature, `max_tokens`, the K, the narrowing - all fixed. If a
  candidate's numbers seem to want a mechanical change, that is a finding and
  a separate subproject.
- **Chain-of-thought or thinking.** Measured flat on the local model, twice.
- **Per-service wording.** One set for the product.
- **Tuning on the corpus by iteration.** Four candidates, one run, decide.
  A fifth candidate written after seeing the four's results is a new
  decision with its own record, not a loop.

## 7. Acceptance criteria

- **AC-Q-101** With `ORCHESTRA_PLANNER_WORDING` unset, every request to the
  model is byte-identical to today's; a test asserts `v1` against the
  literals at `5bf5cf8`.
- **AC-Q-102** Each candidate in section 4 exists as a named set and is
  selectable; an unknown name fails at startup.
- **AC-Q-103** `make eval-shortlist --wording` runs one pass per named
  wording in one invocation and prints one block per wording beside `v1`.
- **AC-Q-104** `make check` calls no model; wording sets are tested for
  shape, not for effect.
- **AC-Q-105** The record carries every candidate's table and miss list,
  `make eval` for the proposed default, and the decision - including "keep
  `v1`" if that is what the numbers say.

## 8. What this does not settle

The corpus is one hundred questions written against a fixture; a wording
that gains eight points here has gained eight points here. `make eval`'s
eighteen cases against the real services are the only check that it did not
lose something the fixture cannot see, and eighteen is not many.
