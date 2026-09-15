# The platform narrows before it plans, and offers a second choice

The seventeenth subproject. Everything measured under `narrowing.md`,
`retrieving.md` and `describing.md` happened in `e2e/narrowing/`, against a
fixture, with a stand-in picker. None of it has touched the product. This
subproject puts the measured narrowing in front of the real planner, lets the
answer carry alternatives so a wrong pick costs one click instead of a
round trip, and measures the product - not a stand-in - on the same corpus.

## 1. What it proves

Two numbers, and the gap between them (`DECISIONS.md`, 2026-09-15):

- With `e5-large-q8`, a reranker that reads the written examples, and the
  shortlist cut to 20, the right operation is inside the shortlist for
  **92-96%** of the corpus.
- The local picker, handed that shortlist, picks it **83%** of the time.

Ten points sit between "it is in the twenty" and "the model chose it". The
picker is now the bottleneck, and every lever tried on it - a bigger model,
thinking, showing it the examples - was flat or negative. Two things were
not tried, and this subproject tries both:

1. **The product's own planner.** The 83 is a text-prompt picker built for
   measurement. The product plans with tool calling (`toolcall.Planner`) and
   has never been measured on this corpus. Its number is now known: **65
   correct@1 / 68 correct@shown** with narrowing on, **62 / 62** off
   (`DECISIONS.md`, 2026-09-15, "The product's planner, measured end to
   end") - below the picker, not above it.
2. **Not asking, offering.** `ask_user` asks before answering. If the
   answer carries the next two candidates, a wrong pick costs the person one
   click and a right one costs nothing. The metric for that is not "did it
   pick right" but "was the right one among what was shown" - and the
   shortlist already says that is 92-96%.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **H1** | The platform narrows the catalogue to a shortlist before the planner sees it. The mechanism is the measured one: `e5-large-q8` over the operation's own text and its `x-orchestra-examples`, reranked by `bge-reranker-v2-m3` reading both, cut to 20, in reranker order. Nothing about it is chosen here; it is the row that won.                                                                                                                                                                                                                                                                                                                                                                                                                      |
| **H2** | Narrowing is a stage in the usecase with one interface, and the planner is unchanged. `Orchestrator.Plan` narrows the catalogue and calls `ToolsFor` on the result; the planner receives a shorter list and does not know why.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| **H3** | The catalogue's vectors are computed when the catalogue is loaded and held in memory. A thousand operations with two examples each is a few thousand vectors of a thousand floats: megabytes, seconds, once. The question is embedded per request. No vector store.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| **H4** | Narrowing needs the embedder, the reranker and the chat model resident at once. llama-swap's default swaps them per call at 7-40 seconds a swap (`retrieving.md` section 5); a `groups` block keeps all three loaded, and the measurement asserts that no request paid for a load. This is a deployment precondition and is written down as one.                                                                                                                                                                                                                                                                                                                                                                                                        |
| **H5** | The answer carries alternatives. A `result` from `/api/plan` gains the next two operations of the shortlist, by id and display name, and the browser shows them as their own assistant turn, after the result, reading "違いましたか？" - the same shape an `ask` turn already has, not a row inside the result. Choosing one re-plans with that operation only, and answers that turn: the chosen chip stays visible and marked, every other chip on it is disabled, and the turn cannot be answered a second time. `ask_user` stays for the questions the planner itself flags; alternatives are for the ones it did not. (Revised 2026-09-15: alternatives moved out of the result and into their own turn, and answering locks it - see section 4.) |
| **H6** | The product is measured on the corpus, end to end. A runner asks the running platform - fixture services behind it - the 100 questions through `/api/plan` and reports per axis: **correct@1** (the planned operation is an answer), **correct@shown** (an answer is the planned operation or an alternative), asked-back rate, and end-to-end latency. Beside them, the stand-in picker's 83.                                                                                                                                                                                                                                                                                                                                                          |
| **H7** | Narrowing is on when configured and absent when not. With no narrowing configuration the platform behaves exactly as today. This is what lets the same binary be measured both ways, and what keeps `make check` free of models.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |

## 3. Where it goes

```
Orchestrator.Plan
  catalog   ← catalogFor(user)                (permission, as today)
  catalog   ← narrower.Narrow(ctx, catalog, question, K)   ← new
  tools     ← ToolsFor(catalog, planCtx)      (unchanged)
  decision  ← planner.Plan(question, turns, tools)
  result    ← ... + alternatives(catalog[1..3])            ← new
```

`Narrower` is a usecase port. Its adapter talks to llama-swap's
`/v1/embeddings` and `/v1/rerank` over HTTP; the usecase never sees a
vector. A `Narrower` that returns its input unchanged is what runs when
nothing is configured (H7) and what every existing test keeps using.

The narrowed catalogue keeps the reranker's order, and `ToolsFor` preserves
it - measured, the picker reads from the top and the order is worth four to
five points.

## 4. What the answer carries

`PlanResult` gains `alternatives`: up to two `{operationId, displayName,
service}` taken from the shortlist positions after the chosen operation.
Not the planner's opinion - the shortlist's. They are omitted when narrowing
is off, and the client renders nothing when they are absent.

## Choosing an alternative

The browser shows a non-empty `alternatives` as its own turn after the
answer - reading 「違いましたか？」, one chip per alternative - not as a
row drawn inside the result. Revised 2026-09-15: two chips clicked one
after another, when the chips lived inside the result, appended two
operations under a single question with no way to tell which chip either
belonged to; a separate turn, answered once, is what that fixed.

Choosing a chip sends `/api/plan` again with `preferred: <id>`; the
platform narrows to that one operation and the planner fills its
parameters. D8 is untouched: an unsafe operation still ends at a form and a
button. The click also answers the alternatives turn itself: the chosen
chip stays visible, drawn selected, and every other chip on that turn
becomes disabled. A turn already answered offers no chip a further click
can reach, so a person can no longer choose twice from the same
「違いましたか？」.

## 5. The configuration

Three environment variables, all or none: `ORCHESTRA_NARROWING_EMBED_MODEL`,
`ORCHESTRA_NARROWING_RERANK_MODEL`, `ORCHESTRA_NARROWING_K`. The endpoint is
`ORCHESTRA_LLM_BASE_URL`, already set - the same llama-swap. Half-set is a
startup error, as `ORCHESTRA_DB_PATH` missing is.

The llama-swap side is a `groups` entry in `local-llm`'s config that keeps
the three models resident. That repository is not this one; the record says
what was configured and the measurement says whether it held (H4).

## 6. What is measured

`make eval-shortlist` (name to be settled in the plan): boots the fixture
server and the platform with narrowing on and `ORCHESTRA_SERVICES` pointing
at the five fixture services, signs in, sends the 100 corpus questions to
`/api/plan`, and reports per axis and overall:

- **correct@1**, **correct@shown**, **asked back** (the planner returned
  `ask`), **none**, **error**
- **end-to-end seconds per question**, and separately the narrowing stage's
  own time, so a model load shows up as what it is

Run twice: narrowing on and narrowing off (the whole catalogue, as today).
The off run is the product as it exists; the on run is this subproject.

Beside them: the stand-in picker's 83 / K=20 row and the 92-96 recall, so
the reader sees whether the real planner lands nearer the picker or nearer
the shortlist.

## 7. Deliberately excluded

- **Changing the planner.** Tool-calling stays as it is; the corpus measures
  it. A different prompt to it is a new baseline and a separate decision.
- **The generated utterance layer.** Measured as noise; not read.
- **A vector store, a cache across restarts, incremental re-embedding.** A
  thousand operations embed in seconds at load. The day that is slow is the
  day to build one.
- **Showing alternatives for `ask`.** The planner's own question stays a
  question.
- **Fixture services that answer `/api/invoke`.** Planning is measured;
  invoking a fixture operation is not.

## 8. Acceptance criteria

- **AC-H-101** With narrowing unconfigured, every existing test passes
  unchanged and `/api/plan`'s tool list is byte-identical to today's.
- **AC-H-102** With narrowing configured, the planner receives at most K
  tools plus the built-ins, in reranker order.
- **AC-H-103** A `result` carries up to two alternatives from the shortlist
  when narrowing is on and none when it is off; `preferred` re-plans against
  that operation alone. The browser shows a non-empty `alternatives` as its
  own turn after the answer, not inside it, and answers that turn once:
  after a chip is chosen, the chosen chip stays visible and marked, every
  other chip on that turn is disabled, and no further chip on it can be
  chosen.
- **AC-H-104** Catalogue vectors are computed once at load; a request embeds
  only the question.
- **AC-H-105** `make check` calls no model and needs nothing running; the
  narrowing adapter is tested against a fake.
- **AC-H-106** The measurement runs against the fixture with narrowing on
  and off and reports correct@1, correct@shown, asked-back, latency, per
  axis, with the narrowing stage's time separated.
- **AC-H-107** The measurement records that no request paid for a model
  load (H4), or says which did.
- **AC-H-108** The numbers are recorded in `DECISIONS.md` beside the
  stand-in picker's, and `PRODUCT.md` D2 is revised or explicitly kept by
  whoever owns that decision, with these numbers in front of them.

## 9. What this does not settle

Whether people use the alternatives. correct@shown is the ceiling of a UI
that offers them; how often a person actually clicks the right one is a
question for a person, not a corpus.

And the corpus is still the fixture's. A real service's `x-orchestra-examples`
are written by its owner, who has not read any test; the written layer's 80
is a blind stand-in's number, and the product's will be whatever the owners
write.
