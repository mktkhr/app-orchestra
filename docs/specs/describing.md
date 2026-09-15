# What the catalogue says about itself

The sixteenth subproject. Every mechanism measured so far - lexical,
embedding, reranking, a local picker, three frontier models, thinking on and
off, the whole catalogue at once - leaves the same questions unanswered, and
they are the same questions every time. This one changes the only thing none
of those touched: the words the catalogue uses to describe an operation.

## 1. What it proves

The residual gap, measured (`DECISIONS.md`, 2026-09-14 and 2026-09-15):

- 「立て替えた分を出したい」→ 経費申請の作成, ranked 385th by `e5-large-q8`
- 「お金を返してもらいたい」→ 精算の作成, 367th
- 「商品が届いたので受け取り処理をしたい」→ 検収の作成, 257th
- 「値段を安くしてほしいと頼みたい」→ 値引の作成, 111th

Widening retrieval to 500 finds them and stops being narrowing. No model -
local 9B, Haiku, Sonnet, Opus, with or without thinking - gets more than 9 of
the 15 axis-D answers to first place from a shortlist of 50, because the
answers are not in the shortlist. A frontier agent reading all thousand
operations gets 14: the mapping exists, in a reader that knows what 立て替え
means. The catalogue does not say it, so nothing that reads the catalogue can
find it.

What this subproject proves is that the catalogue can be made to say it, at
no cost per question, and by how much that moves axis D - and that saying it
does not break the four axes that already work.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                                                                                                                                                        |
| ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **G1** | The gap is closed on the catalogue's side. Retrieval, reranking and the picker do not change. An operation gains **utterances**: things a person might say when they want it.                                                                                                                                                   |
| **G2** | Two layers of utterance, measured separately. Generated: a local model writes them once, offline, from the operation's own text. Written: the service owner puts them in the contract, as `x-orchestra-examples`. The first gives coverage for free; the second gives precision where the first misses, and is the durable one. |
| **G3** | Each utterance is its own vector. An operation's score against a question is the best of its own text and each of its utterances, not the score of one concatenated blob. Five paraphrases folded into a summary dilute it; five vectors beside it do not.                                                                      |
| **G4** | Generation is local, offline, cached and deterministic. The picker's model, thinking off, temperature 0, one call per operation, cached on disk keyed by model, prompt and operation text - exactly as the vectors are. Nothing is generated at question time, and a second run generates nothing.                              |
| **G5** | An utterance has to find its own operation. The contract check of `docs/specs/retrieving.md` V2 is applied to utterances: embedded as a query, each must retrieve its parent operation first. A generator whose utterances cannot find their own operation has produced noise, and its rows say so.                             |
| **G6** | The written layer for the fixture is written blind. Whoever writes `x-orchestra-examples` for the thousand fixture operations has not read the corpus - the same rule as the 2026-09-14 ceiling probe. Examples written with the questions in view would measure the author, not the mechanism.                                 |
| **G7** | Axis D is the number; B, C and E are the guard. Utterances can create matches that should not exist - 発注 and 受注 both paraphrase to 注文. The report shows every axis, and a configuration that lifts D by dropping B is reported as that, not as an improvement.                                                            |

## 3. The two layers

**Generated.** For each exposed operation, the local model is asked, once,
for five things a person who needs that operation would say first - in the
words of their problem, not the system's - with two worked examples from
domains outside the fixture (a meeting-room booking, a lost staff card) and
an explicit ban on the operation's own words. The input is the operation's
summary, description, display name and service display name - the same text
every retriever reads, so the generator knows exactly what the retriever
knows and nothing more. The output is five strings.

The ban and the examples are not decoration. The first prompt tried
(2026-09-15) asked for "short ways of saying this operation" and got, for
経費申請の作成, 「経費申請作って / 経費申請作成 / 経費申請作りたい」: the noun
repeated with verb endings, which the retriever already has, and which
bridges nothing. The words that the residual gap needs - 届いた, 受け取った,
安く - appeared zero times in 4,960 utterances. With the examples and the
ban, the same model writes 「仕入れの荷物が届いたから確認したい」 for 検収の作成
and 「この商品だけ安くしたいんだけど」 for 値引の作成.

**Written.** `x-orchestra-examples` is a vendor extension on an operation in
`openapi.yaml`: a list of strings, each one thing a person might type. It is
optional. It is read by the platform's spec source into the catalogue
endpoint like `x-ui-hint` is, and it is otherwise inert in the product until
the narrowing is wired in - this subproject defines the field and measures
it; it does not change what `/api/plan` does.

For the fixture, the definition table gains an `examples` field on a
resource, setting, aggregate and workflow, filled under G6.

## 4. How an utterance is scored

An operation carries one vector for its own text and one per utterance. For a
question, its score is the maximum over all of them. Retrieval, reranking and
everything after are unchanged; they see a better-scored operation and
nothing else.

The reranker sees the operation's own text, not its utterances - except for
the written layer's own examples, which it should read too. Measured
(`DECISIONS.md`, 2026-09-15, "Letting the reranker read the written
examples"): a reranker that scores on `combinedTextOf(operation)` alone
loses seven points of the written layer's own axis D gain at K=10 (80% →
73%) once the two-stage configuration runs, because an operation retrieved
into the fifty by its written example is pushed back down by a reranker
that never read the example that put it there. Appending the operation's
written examples to the reranked document text
(`e2e/narrowing/utterances/reranker-written.ts`) recovers axis D exactly -
80%/93%/93% at K=10/20/50, identical to the written layer's own blind
recall at every K - at zero measured cost to any other axis and about 8%
more reranker latency. Generated utterances stay out of this document: the
generated layer is a source of noise on retrieval (section 3's negative
result), and feeding that same noise to the reranker's document text would
not be a different experiment.

This does not extend to the picker. The same examples shown to the local
9B picker as an extra column, on the same shortlist, make its raw pick
_worse_, not better - overall correct falls further below the plain
`+written` shortlist's own pick score than the reranker's blind spot ever
cost it, and the axis-B flagged rate (desired high, on the genuinely
ambiguous questions) falls rather than rises. The reranker and the picker
read the same information differently; what closes the gap for one does
not close it for the other, and this document does not recommend showing
the examples to the picker.

## 5. Generating them

One call per operation, `qwen3.5-9b-q8`, thinking off, temperature 0,
`max_tokens` enough for five short lines. A thousand operations at the
measured 0.3-0.4 seconds a call is about six minutes once, then never again
until the operation's text changes.

The cache is `e2e/narrowing/.utterances/`, beside `.vectors/`, ignored by
git, keyed by model, prompt and the operation's text. Changing the prompt
regenerates everything; changing one operation regenerates one.

`make check` calls no model. Generation runs from `make narrowing`, which
already needs llama-swap, and is skipped with a reason when it is down,
exactly as the vector rows are.

## 6. The contract check, for utterances

Two rates, printed together, and they pull against each other.

**Retrieval.** For a deterministic sample of operations (every 50th, as
`retrieving.md` does), each utterance is embedded as a query and the parent
operation must come back first among the thousand. A rate, not a gate, for
the reason `retrieving.md` section 4 gives.

**Novelty.** The share of utterances that share **no character bigram** with
their own operation's text - the same definition the corpus's axis D uses
for a question. The first prompt scored perfectly on retrieval precisely
because it repeated the operation's noun; retrieval alone rewards the
failure, which is why novelty stays reported alongside it.

Measured, novelty anti-correlates with what the recall table rewards
(`DECISIONS.md`, 2026-09-15, "The catalogue says it"): the generated layer
scores 43% novel and does not move axis D; the written layer scores 15%
novel and moves axis D from 33% to 80% at K=10. The written layer's own
failing utterances are the ones that dropped the operation's anchor noun
entirely - exactly the utterances a novelty check scores highest, because
losing the noun is one reliable way to stop sharing a bigram with it. A
useful utterance keeps the anchor noun and adds the everyday situation
around it; that is closer to what predicts a layer's value than novelty is,
and this subproject did not build a check for it.

The report still shows both rates, because retrieval and novelty are cheap,
deterministic, and each catches a real failure mode (a layer that cannot
find its own operation; a layer that only repeats the operation's own
words) - but neither rate is what decides a layer's value. Section 7's
recall table is what decides.

## 7. What is measured

`docs/specs/narrowing.md` section 6 unchanged - recall@K per axis and
overall, K = 10/20/50, ties against - with three new rows beside the
existing ones:

- `e5-large-q8` + generated utterances
- `e5-large-q8` + written utterances
- `e5-large-q8` + both

and the two-stage (`e5-large-q8` + reranker), with written and with both -
`e5-large-q8+reranker+written` and `e5-large-q8+reranker+both`.
`e5-large-q8` is the retriever because it is the one measured to hold the
most axis-D answers inside fifty.

**The headline row is `e5-large-q8+reranker+written`, not
`e5-large-q8+reranker+both`.** Measured (`DECISIONS.md`, 2026-09-15, "The
catalogue says it"), the written layer is this subproject's result and the
generated layer is a negative one (section 6, and the same entry): unioning
the generated layer's utterances into `+written` lowers axis D at every K
measured. `e5-large-q8+reranker+both` is reported beside the headline as
the trade it actually is - 93% overall against the headline's 89%, at axis
D 67% against 73% - because the reranker filters enough of the generated
layer's noise to let its coverage help axis C, while it still costs axis D.
Neither row is named the winner; a product choosing between them is
choosing between more overall recall and a working vocabulary bridge.

The numbers this subproject was for: axis D at K=10 and K=50, against the
then-current 33% and 73% for `e5-large-q8` alone and 60% / 73% for the
two-stage. Reached: `e5-large-q8+written` alone moves axis D to 80% at
K=10 and 93% at K=50; the headline `e5-large-q8+reranker+written` reaches
73% at K=10. The numbers it must not move - axes A, B, C and E, then 100 /
100 / 72 / 100 for `e5-large-q8` - hold under every row that carries the
written layer alone; the generated layer is the one row that moves axis B
down (100% → 64%), recorded as a regression, not folded into the headline.

The utterance contract-check rate, per layer, is printed with them.

## 8. Deliberately excluded

- **Any change to retrieval, reranking or the picker.** G1.
- **Generation by a frontier model.** It is offline and would cost cents; it
  is excluded because D17 keeps the product local and because a measurement
  of what the local model can write for itself is the one that transfers.
- **Rewriting summaries or descriptions.** An utterance is added beside the
  text, never in place of it; the retriever's existing input is byte-identical.
- **Measuring the pick.** That is `TODO.md` item 2 and it does not depend on
  this.
- **Wiring the narrowing into `services/platform`.** The vendor extension is
  defined and read; nothing behind `/api/plan` changes.
- **Fine-tuning anything.**

## 9. Acceptance criteria

- **AC-G-101** A second `make narrowing` after the first generates nothing:
  every utterance is served from the cache, and changing one operation's text
  regenerates that operation's utterances and no others.
- **AC-G-102** Every utterance is embedded and cached as its own vector, and
  an operation's score is the maximum over its text and its utterances (G3).
- **AC-G-103** The utterance contract check (section 6) - retrieval rate
  and novelty rate - is printed beside each utterance configuration.
- **AC-G-104** `x-orchestra-examples` is defined in the platform's contract
  handling: the spec source reads it, the catalogue endpoint carries it, and
  the fixture's `toOpenAPI` emits it. `make check` is green with no change to
  what `/api/plan` does.
- **AC-G-105** The fixture's written examples were produced without access
  to the corpus (G6), and the record says by whom and how.
- **AC-G-106** The report carries the four new rows of section 7 for all
  four catalogue sizes, and `make check` still calls no model.
- **AC-G-107** The numbers are recorded in `DECISIONS.md` per axis per layer,
  including any axis the utterances made worse.

## 10. What this does not settle

The corpus's fifteen axis-D questions were written by the same hand as the
fixture. Fifteen is enough to see a layer fail and not enough to see it
succeed; a generated layer that reaches 12 of 15 has shown it can bridge
this kind of gap, not that it bridges every kind.

And the written layer is only as good as the service owner's ear for how
their users talk. The fixture's examples, written blind, are a stand-in for
that; a real service's examples will be better or worse than a stand-in, and
the measurement cannot say which.

Whether the reranker should read the written examples is answered
(section 4, `DECISIONS.md` 2026-09-15 "Letting the reranker read the
written examples"): it should, and doing so costs nothing measured on the
axes it already helps. What is not settled is the narrower question that
measurement opened: the same examples shown to the local picker make its
own raw pick worse, and a different presentation of them - not the same
tab-separated column, just relocated - might read differently to a 9B
model. This document does not attempt that; it records only that showing
the picker exactly what fixed the reranker did not fix the picker.

And the generated layer's failure was not a matter of prompting it once and
stopping. Two prompts were tried: the first asked for short paraphrases and
produced the operation's own noun with conjugated endings; the second added
few-shot examples from outside the fixture and banned the operation's own
words, and produced the intended register - genuinely novel, situational
phrasing. Axis D did not move under either one, 33% at K=10 both times. A
third prompt might do better, but this subproject's own working assumption
(section 2, G2 - the generated layer gives "coverage for free") is now
measured, not assumed, and twice over: in two attempts, it did not deliver
coverage this corpus could detect.
