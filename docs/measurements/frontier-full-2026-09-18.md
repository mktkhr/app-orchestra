# Claude models on all four instruments

Measured 2026-09-18 at `4374c4c`, after the single-shot check
(`frontier-2026-09-18.md`) proved the path. Only the model varies: the
planner's two stages run on the Anthropic Messages API
(`ORCHESTRA_LLM_PROVIDER=anthropic`), local narrowing (`e5-large-q8` +
`bge-reranker-v2-m3-q8`, K=20) is unchanged in front of them, and every
other default is the tree's own (two stages, `v6-unmatched-filter`,
`ORCHESTRA_PLANNER_TODAY=2026-09-16`, all Jev flags unset).

This file fills in as each model finishes. Rows marked _not run_ are
exactly that - no estimate is entered as if it were a measurement.

## Results

| model                           | eval      | corpus      | mid correct / false-refusal / refused | dialogues | corpus ms mean |       cost |
| ------------------------------- | --------- | ----------- | ------------------------------------- | --------- | -------------: | ---------: |
| local `qwen3.5-9b-q8` (default) | **34/34** | 77 / 80     | 37/40 / 2 / 16/20                     | **26/27** |      **1,363** |         $0 |
| `claude-haiku-4-5`              | 29/34     | 60 / 63     | 22/40 / 17 / 19/20                    | 24/27     |          2,089 |      $1.52 |
| `claude-sonnet-5`               | 31/34     | **81 / 83** | 26/40 / 11 / 18/20                    | 22/27     |          4,916 |      $3.23 |
| `claude-opus-5`                 | 31/34     | 75 / 75     | 23/40 / 14 / 18/20                    | 25/27     |          5,314 |      $7.77 |
| `bonsai2-27b` (local, ternary)  | 30/34     | **81 / 81** | **39/40 / 1 / 19/20**                 | 25/27     |          3,579 |         $0 |
| `claude-fable-5-1`              | _not run_ | _pending_   | _not run_                             | _not run_ |                | ~$3.5 est. |

Round total on the Anthropic side: **$12.53** for the three Claude models,
$16.56 cumulative including the earlier rounds.

Corpus per-axis for the models measured so far (A plain lists, B
cross-service homonyms, C near neighbours, D vocabulary gap, E setting
decoys):

| model                 | A   | B      | C   | D      | E       | overall |
| --------------------- | --- | ------ | --- | ------ | ------- | ------- |
| local `qwen3.5-9b-q8` | 92  | 80     | 64  | 60     | 90      | 77      |
| `claude-haiku-4-5`    | 84  | 52     | 48  | 33     | 90      | 60      |
| `claude-sonnet-5`     | 92  | 80     | 68  | **80** | 90      | **81**  |
| `claude-opus-5`       | 88  | 76     | 68  | 47     | **100** | 75      |
| `bonsai2-27b`         | 92  | **92** | 60  | 73     | 90      | **81**  |

## Model by model

**`claude-haiku-4-5` - declines to act.** Five eval cases regress and every
one fails toward `none`: `no-enum-value` (破損した在庫はある？) returns
every row 30/30 where the contract says ask; `real-register-new-item`,
`real-tanaka-attendance` and `real-inventory-list-graph` answer `none`
10/10; `real-register-screws` is half forms, half wrong. The dialogue
losses are the same shape (在庫を登録したい, この人たちの記録を追加したい
→ `none`), and on mid it produced **7 forms against the local model's 23**
with 17 of 40 answerable questions falsely refused. Its 19/20 refusal
column is excellent for the same reason its answerable column is the worst
of any Claude model measured: it says no.

**`claude-sonnet-5` - the best corpus, the worst dialogues.** 81/83 on the
corpus, above the local 77/80, and **axis D 80 against the local 60** - the
vocabulary-gap axis that no configuration had moved before. It still
carries the same reluctance in weaker form: mid 26/40 with 11 false
refusals, 16 forms, and its three eval losses are `real-register-screws`,
`real-tanaka-attendance` and `real-inventory-list-graph`. Dialogues 22/27,
the lowest of the four.

**`claude-opus-5` - 2.4x the price of Sonnet, six points below it.** 75/75
on the corpus against Sonnet's 81/83, axis D 47 against 80. Best axis E of
anything measured (100). eval 31/34, mid 23/40 with 14 false refusals,
dialogues 25/27. The 2026-09-15 pick-only round had Sonnet and Opus tied at
73; running both stages separates them, and not in the direction price
suggests.

**`bonsai2-27b` - the surprise, and it is free.** A 27B model ternary-
quantised to **5.95 GB** by PrismML, published the day before this round
and not runnable by stock llama.cpp at all (see below). It ties Sonnet 5 on
the corpus at **81**, takes **axis B at 92** - the cross-service homonyms
every other model finds hardest - and posts **mid 39/40 answerable, 1 false
refusal, 19/20 refused**, which is the best mid line of any of the fifteen
models measured today, local or hosted. eval 30/34 is its weak spot, and it
runs at 3,579 ms a corpus question against the local 9B's 1,363.

## Getting bonsai2-27b to run at all

Stock `llama-server` (build 10920, the one llama-swap ships) **cannot load
either packing**: `tensor 'output.weight' has invalid ggml type 143`
(PTQ1_0) and `type 142` (PQ2_0) - the file is rejected while its tensor
index is read, so it fails loudly rather than producing garbage. That was
measured here, not taken from a blog: an earlier summary of third-party
posts claimed stock llama.cpp "silently outputs garbage", and the actual
failure mode is the opposite.

PrismML's own llama.cpp fork
(`prism-b10685-7dffb15`, linux-cuda-12.8-x64, 167 MB) loads it in **7.3 GB
of VRAM**, answers Japanese cleanly, honours
`chat_template_kwargs.enable_thinking=false` (the same switch the local
Qwen entry uses - thinking is on by default and eats `max_tokens`
otherwise), and returns OpenAI-shaped `tool_calls` with correct arguments
(検品保留の在庫を見せて → `ListInventoryItems {"status":"quarantined"}`).
The fork's binaries now sit in the local-llm repo's mounted `prism/`
directory and the model is a normal llama-swap entry, `bonsai2-27b`.

## Analysis

**1. The fill stage is where this shows.** The 2026-09-15 frontier round
measured the pick alone and read Haiku 67 against the local 69 - close
enough to look like a contender. Running both stages puts it at 60 on the
corpus and 22/40 on mid, because the refusals happen _after_ the operation
has been chosen. A pick-only comparison cannot see a model that picks
correctly and then refuses to fill.

**2. The refusal metric keeps promoting broken models.** Haiku's 19/20
refused is among the best numbers in that column across all fifteen models
measured today; `lfm25-8b-a1b-q8` scored a perfect 20/20 locally by
refusing 39 of 40 answerable questions (`models-2026-09-18.md`). Different
vendors, same failure shape, same misleading column. Refusal and
answerable have to be read as a pair or not at all - and `bonsai2-27b`
shows what a genuinely good line looks like: 19/20 refused **with** 39/40
answered.

**3. Price does not order these models.** Opus 5 costs 2.4x Sonnet 5 and
scores below it on three of four instruments. A 5.95 GB local model ties
the best hosted corpus score, beats every model on mid, and costs nothing
per question. The only column where the price order holds at all is
latency, and there it runs backwards: the cheapest thing here (the local
9B) is also the fastest by 2-4x.

**4. A prompt fix is not model-independent.** `no-enum-value` is the case
`v6-unmatched-filter` was written for (2026-09-16): an unmatched
restricting word must become a question. That wording closed it on
`qwen3.5-9b-q8` and reads 30/30 _wrong_ on Haiku 4.5, while Sonnet 5,
Opus 5 and Fable 5.1 answer it correctly. The wording is tuned, and the
tuning is part of the model choice - any swap has to re-run the wording's
own cases.

**5. Nobody beats the local default on `make eval`.** 34/34 is still the
local model alone; the best hosted score is 31/34 and bonsai2 is 30/34.
Every challenger loses the same kinds of row - create forms, a filtered
list, a chart - which says the gap is in the fill's instruction-following
on the real six-operation catalogue, not in raw capability.

**6. What a pass costs.** Haiku $1.52, Sonnet $3.23, Opus $7.77 for one
pass over four instruments; $12.53 for the three. The number worth carrying
into a "should we use a hosted model" conversation is not the per-token
price but this: what a full pass costs, and what it buys - which here is
+4 corpus points (Sonnet) against -8 mid points and -4 dialogue points.

## Method notes

- eval is 34 cases x 10 runs; corpus 100 questions; mid 60; dialogues 27
  turns. One model at a time, sequential.
- The API key is read from the process environment inside the command and
  never written down; the per-call platform log lines (`anthropic chat
completion`: model, input/output tokens, latency, stop reason) are what
  the cost figures are computed from.
- Costs: corpus and mid are exact (the runners keep the platform log);
  eval and dialogues are computed from their call counts at the measured
  per-question rate, because those runners drain the platform's output
  rather than saving it. Marked as such rather than presented as exact.

## Raw data

Per-model instrument outputs in the session scratchpad
(`claude/out/<model>/{eval,corpus,mid,dialogue}.txt`), the corpus and mid
runs' own `e2e/shortlist/out/*-model-<model>.{jsonl,log}`, and the spend
ledger `anthropic-spend.json`.
