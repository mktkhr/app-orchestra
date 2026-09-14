# Retrieving by meaning

The fifteenth subproject. `docs/specs/narrowing.md` built a thousand-operation
catalogue, a corpus that is hard in five named ways, and measured the cheapest
narrowing there is. This one measures what retrieval by meaning does to those
numbers, which configuration of it, and what it costs to keep loaded.

## 1. What it proves

The lexical floor, measured (`DECISIONS.md`, 2026-09-14, 1000 operations,
K = 10, ties counted against):

| axis | A    | B   | C   | D   | E   |
| ---- | ---- | --- | --- | --- | --- |
|      | 100% | 32% | 36% | 0%  | 50% |

Axis D is zero at every K and every catalogue size. A question whose words the
catalogue never wrote is not merely ranked badly; it is not a candidate.

A throwaway probe on 2026-09-14 (not committed, and the reason this spec
exists) already says roughly what meaning-based retrieval does to that. It also
said something that decides how this subproject has to be built:

> **Ruri v3 310m, CLS pooling: axis B 6/25. The same model, same corpus, mean
> pooling: 22/25.**

A wrong pooling flag makes a good model look worthless. So what gets measured
here is **configurations, not models**, and a model's own contract - how its
vectors are pooled, what prefix its queries and its documents carry - is part
of the measurement rather than a detail assumed correct.

The question is therefore no longer "does meaning help". It is which
configuration, how much of axis D it reaches, and what it costs to keep the
thing loaded.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                                                                                                                           |
| ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **V1** | More than one model. One model's number cannot separate "meaning does not help here" from "this model is wrong for Japanese" - and the probe already caught that mistake once, at the moment `qwen3-embedding-0.6b` scored 3/10 on axis E where `bge-m3` scored 10/10.                             |
| **V2** | A model's contract is measured, not assumed. Pooling and prefixes are declared per model and checked before its numbers are compared with anyone else's. A configuration that has not been shown to work is not evidence about a model.                                                            |
| **V3** | The text that gets embedded is the text the lexical baseline reads - summary, description, display name, service display name. Identical input is what makes the comparison about the mechanism.                                                                                                   |
| **V4** | The catalogue's vectors are computed once and cached on disk; the question's is not. A thousand operations change when the fixture changes; a question arrives one at a time. Caching the first makes the measurement re-runnable in seconds, and not caching the second keeps the cost honest.    |
| **V5** | Every mechanism appears in one table, against the same corpus, including the lexical floor. A number that cannot be put beside the thing it replaces is not a result.                                                                                                                              |
| **V6** | Retrieve-then-rerank counts as one mechanism, not two. Measured, `multilingual-e5-large` puts 11 of the 15 axis-D answers inside the top 50 and only 3 inside the top 10: the answers are found and not ordered, which is what a reranker is for. Measuring the stages separately would hide that. |
| **V7** | What it costs to keep loaded is measured, not argued. Section 5 carries the numbers.                                                                                                                                                                                                               |

## 3. The configurations measured

Five, plus the floor:

| id                        | pooling | query prefix           | document prefix |
| ------------------------- | ------- | ---------------------- | --------------- |
| `lexical`                 | -       | -                      | -               |
| `bge-m3-q8`               | cls     | none                   | none            |
| `e5-large-q8`             | mean    | `query: `              | `passage: `     |
| `ruri-v3-310m-q8-mean`    | mean    | `検索クエリ: `         | `検索文書: `    |
| `qwen3-embedding-0.6b-q8` | last    | `Instruct: …\nQuery: ` | none            |
| `e5-large-q8` + reranker  | mean    | `query: `              | `passage: `     |

The last row is V6: retrieve 50 with `e5-large-q8`, then rerank to K with
`bge-reranker-v2-m3`. `e5` is the retriever for it because it is the one
measured to have the answers inside 50.

`ruri-v3-310m-q8` with CLS pooling stays configured and stays measured, beside
the mean-pooled one. It is the evidence for V2 and deleting it would leave the
decision looking arbitrary.

## 4. What a wrong contract looks like

Two of the configurations above are still suspect, and the spec says so rather
than reporting their numbers as facts about the models:

- **`qwen3-embedding-0.6b-q8`** is the only one pooled on the last token and
  the only one whose query prefix is an instruction sentence. Its weak probe
  numbers may be the model or may be the contract, exactly as Ruri's were. It
  is therefore measured twice, with the instruction and without it
  (`qwen3-embedding-0.6b-q8-plain`), and the recall table decides rather than
  the model card.
- **`bge-m3-q8`** uses CLS, which is right for it, but that was assumed from
  the model card rather than shown.

V2 is what settles these. The cheapest demonstration that does not beg the
question is a **retrieval of a document by its own text**: embed an operation's
text as a query, exactly as the configuration would encode a real question, and
see whether the same operation comes back first.

What that probes is narrower than "is this model configured correctly", and the
spec says which, because the difference was measured on 2026-09-14 and is not
obvious. Encoded symmetrically - the document prefix on both sides - **every**
configuration retrieves itself, including the CLS-pooled Ruri that goes on to
score 6/25 on axis B. The check only has teeth when it encodes the two sides
the way the configuration really does: query prefix on one, document prefix on
the other. So what it measures is whether a configuration's **query encoding
and document encoding land in the same space** - which is what an asymmetric
prefix, or a pooling that disagrees with the one the model was trained under,
actually breaks.

It is reported as a rate, not as a gate. Three failures in twenty and one in
twenty are not the same finding, and one of them has an innocent explanation: a
model whose queries carry an instruction its documents do not is being asked to
match a document wrapped in an instruction it has never seen. A configuration's
rate is printed beside its recall so the two can be read together; a
configuration that fails often is not evidence about its model, and one that
fails once is not disqualified.

## 5. Loading, and what it costs

Measured on this machine, 2026-09-14, RTX 4080 SUPER (16 GB), llama-swap with
no `groups` - one model resident at a time, which is llama-swap's default and
what the deployment currently uses:

| call                                             | seconds  |
| ------------------------------------------------ | -------- |
| embedding call, model already resident           | **0.0**  |
| chat call, model already resident                | **0.1**  |
| chat call that first unloads the embedder        | **34.8** |
| embedding call that first unloads the chat model | **4.7**  |

One alternation costs about **40 seconds**. The narrowing it serves takes
0.24 ms. The loading is five orders of magnitude larger than the work.

So a narrowing that embeds the question at the moment the question is asked
cannot share a GPU slot with the model that answers it. Either both stay
resident - `e5-large-q8` is about 0.6 GB against a 9 B chat model's ~9 GB, so
16 GB holds both - or the embedder runs outside llama-swap's rotation
entirely. This is not a deployment note; it is the reason the measurement in
section 6 reports the resident and non-resident cases as different numbers.

Measuring the catalogue is unaffected: it embeds once, calls no chat model, and
V4 caches the result.

## 6. What is measured

Everything `docs/specs/narrowing.md` section 6 defines, unchanged - recall@K
per axis and overall, K = 10/20/50, catalogue sizes 1/2/3/5 services, ties
counted against - for every configuration in section 3, in one table with the
lexical floor.

Two figures are added:

- **Per-question wall-clock with the model resident**, which is what a question
  costs once the loading has happened.
- **The cost of the alternation**, from section 5, reported once rather than
  per row. It does not vary with K or with the catalogue's size, and burying it
  in a per-query average would hide the only number here big enough to decide
  anything.

Embedding vectors are floats, so they do not tie the way bigram counts do. The
pessimistic and optimistic recalls should collapse onto each other; where they
do not, that is worth seeing, and the report keeps both columns for every
mechanism rather than dropping them for the ones that do not need them.

## 7. Deliberately excluded

- **Choosing what the product uses.** This measures; the choice is a later
  decision with these numbers in front of it.
- **Wiring any of it into the platform.** `services/platform` is not touched.
  Nothing here changes what a question to `/api/plan` does.
- **Scoring lexical and vector together.** A hybrid is the obvious next thing
  and it moves two numbers at once. It becomes worth building when the pure
  mechanisms have been measured, not before.
- **A vector database.** A thousand vectors is four megabytes and a dot
  product against all of them is measured in this subproject; a store that
  indexes them solves a problem nobody has yet demonstrated.
- **Fine-tuning or training anything.**
- **The tie problem.** `docs/specs/narrowing.md` section 10's open question was
  what to do about 186 operations sharing a score. The probe suggests meaning-
  based retrieval dissolves most of it by not tying at all. What survives that
  is a smaller question and is not this subproject's.

## 8. Acceptance criteria

- **AC-V-101** Every configuration in section 3 is measured on section 4's
  contract check - how often it retrieves an operation first when queried with
  that operation's own text, encoded the way that configuration encodes a real
  question - and the rate is printed beside its recall, so a configuration that
  fails often is not read as evidence about its model.
- **AC-V-102** The catalogue's vectors are cached on disk, keyed so that a
  change to the fixture or to a configuration invalidates only what it should,
  and a second run reuses them.
- **AC-V-103** A question's vector is computed at question time, not cached.
- **AC-V-104** `make narrowing` reports every configuration and the lexical
  floor in one table, same corpus, same K values, same catalogue sizes, with
  the direction stated in the header.
- **AC-V-105** The report carries per-question wall-clock with the model
  resident, and the alternation cost from section 5, reported once.
- **AC-V-106** Nothing in `services/` changes, and `make check` still calls no
  model of any kind.
- **AC-V-107** The numbers are recorded in `DECISIONS.md`, per axis, per
  configuration, including the configurations that do badly and the ones that
  fail AC-V-101.

## 9. What this does not settle

The probe that prompted this spec found `multilingual-e5-large` reaching 11 of
15 axis-D answers inside the top 50 and 3 inside the top 10. If the reranker
turns that 11 into ten answers inside the top 10, the vocabulary gap is mostly
closed and the next question is what the model does with the shortlist. If it
does not, then the gap is not an ordering problem, and the answer is somewhere
else entirely - in what the catalogue says about itself, rather than in how it
is searched.

Either way the corpus is fifteen questions on that axis, written by the same
hand that wrote the fixture. It can show a mechanism failing. It cannot show
one working.
