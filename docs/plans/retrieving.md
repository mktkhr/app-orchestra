# Retrieving by meaning — implementation plan

> **For the implementer:** one task at a time. Each task ends green and is
> committed on its own. Steps use `- [ ]` for tracking.

**Goal:** `make narrowing` reports the lexical floor and five meaning-based
configurations in one table, against the same 100 questions, with the cost of
keeping a model loaded reported beside them.

**Architecture:** the measurement already exists
(`e2e/narrowing/measure.ts`). What this adds is a second kind of narrower - one
that calls llama-swap for vectors instead of counting bigrams - and the machinery
that makes calling it cheap enough to re-run: a disk cache for the catalogue's
vectors, and a contract check that refuses to report a misconfigured model's
numbers as if they were the model's.

**Spec:** `docs/specs/retrieving.md`. Acceptance criteria: its section 8.

## Global constraints

`docs/plans/narrowing.md`'s Global constraints still hold. Added or changed:

- **`make check` still calls no model of any kind** (AC-V-106). Every unit test
  uses a fake client; none of them reach `localhost:11435`. Verify with
  `docker logs llama-swap 2>&1 | grep -c 'POST /v1/'` before and after —
  note `/v1/`, not `/v1/chat/completions`: embeddings and reranking are
  different endpoints and the old count would miss them.
- **`make narrowing` now needs llama-swap for the vector rows.** The lexical
  row must still run without it. When llama-swap is unreachable, print the
  lexical table, name the rows that were skipped and say why — do not fail, and
  do not silently print a short table.
- `services/` is not touched at all. Nothing in this subproject changes what a
  question to `/api/plan` does.
- No suppressions. Import test helpers from `vite-plus/test`, not `"vitest"`;
  `vitest/no-conditional-in-test` forbids `if`/`continue` in a test body;
  `vitest/valid-expect` takes one argument; `no-unsafe-type-assertion` forbids
  `as` on a loosely-typed value.
- 1000 lines a file. `make fmt` before `make check`; if a file is still
  unformatted after `make fmt`, simplify it rather than fighting the formatter
  (a multi-paragraph markdown list item is one known way to trigger that).
- Commit with `git commit -- <explicit paths>`. Another session's
  `e2e/eval/*.ts` is uncommitted and must stay that way.

## What is already configured in llama-swap

Added and verified working on 2026-09-14, all reachable at
`http://localhost:11435`. You do not need to configure llama-swap; you need to
call it.

| model id                  | endpoint         | pooling | query prefix           | document prefix |
| ------------------------- | ---------------- | ------- | ---------------------- | --------------- |
| `bge-m3-q8`               | `/v1/embeddings` | cls     | none                   | none            |
| `e5-large-q8`             | `/v1/embeddings` | mean    | `query: `              | `passage: `     |
| `ruri-v3-310m-q8-mean`    | `/v1/embeddings` | mean    | `検索クエリ: `         | `検索文書: `    |
| `ruri-v3-310m-q8`         | `/v1/embeddings` | cls     | `検索クエリ: `         | `検索文書: `    |
| `qwen3-embedding-0.6b-q8` | `/v1/embeddings` | last    | `Instruct: …\nQuery: ` | none            |
| `bge-reranker-v2-m3-q8`   | `/v1/rerank`     | -       | -                      | -               |

`ruri-v3-310m-q8` and `ruri-v3-310m-q8-mean` return **768** dimensions; the
others return 1024. Nothing may assume a fixed dimension. `/v1/rerank` takes
`{model, query, documents}` and returns `results: [{index, relevance_score}]`,
**not sorted** — sort by `relevance_score` descending yourself.

The CLS-pooled Ruri is deliberately kept: spec section 4 uses it as the
evidence that a wrong contract, not a weak model, produces a bad number.

---

### Task 1: the embedding client, its cache, and the contract check

**Files:**

- Create: `e2e/narrowing/embedding/client.ts`, `configs.ts`, `cache.ts`,
  `contract.ts`, `index.ts`
- Test: `e2e/narrowing/embedding/client.test.ts`, `cache.test.ts`,
  `contract.test.ts`
- Modify: `.gitignore`

**Consumes:** `e2e/narrowing/fixture/` (`catalogOf`, `FixtureOperation`).

**Produces:** `embedCatalogue(config, catalog)` returning one normalised vector
per operation, served from disk on a second call; `embedQuestion(config, text)`;
`checkContract(config, catalog)`.

- [ ] **Step 1: write the failing client test** — the client must be usable
      with a fake transport, because `make check` may not call llama-swap. Give
      it a `fetch`-shaped function as a parameter, default the real one, and
      test against a fake that records what it was asked for. Assert: documents
      get the document prefix and questions get the query prefix, batching
      splits a long input list, vectors come back normalised (length 1).

- [ ] **Step 2: run it and watch it fail**

- [ ] **Step 3: write `configs.ts` and `client.ts`** — `configs.ts` is the
      table above as data: id, model, endpoint, query prefix, document prefix.
      Nothing computed, no logic. `client.ts` posts batches of 64 to
      `/v1/embeddings` and normalises.

- [ ] **Step 4: write the failing cache test** — a second `embedCatalogue` with
      the same config and the same catalogue must return the same vectors
      **without calling the transport at all**; changing one operation's text,
      or the config, must miss. Assert the miss, not only the hit.

- [ ] **Step 5: write `cache.ts`** — vectors as a `Float32Array` in one binary
      file per config, beside a JSON manifest holding the operation ids in
      order, the dimension, and a hash of the exact text that was embedded.
      A manifest whose hash does not match is a miss and the file is rewritten.
      Put the files under `e2e/narrowing/.vectors/` and add that to
      `.gitignore`: they are derived, four megabytes each, and
      `docs/specs/narrowing.md` T5 already says generated things do not enter
      the repository.

- [ ] **Step 6: write the failing contract test** (AC-V-101) — `checkContract`
      embeds a sample of operations' own text as **queries** and requires each
      to come back first among the catalogue. Test it against a fake transport
      two ways: vectors that satisfy it, and vectors that do not.

- [ ] **Step 7: write `contract.ts`** — sample 20 operations spread across the
      five services, deterministically (every 50th), not at random: a check
      that differs between runs cannot be cited in `DECISIONS.md`. Return which
      operations failed, not a boolean.

- [ ] **Step 8: `make fmt && make check`**, confirm the llama-swap call count is
      unchanged, then commit.

---

### Task 2: the vector configurations in the report

**Files:**

- Create: `e2e/narrowing/embedding/narrower.ts`
- Modify: `e2e/narrowing/measure.ts`, `e2e/narrowing/report.ts`
- Test: `e2e/narrowing/embedding/narrower.test.ts`, and the existing
  `measure.test.ts`

**Consumes:** Task 1.

**Produces:** one table holding the lexical floor and every embedding
configuration (AC-V-104).

- [ ] **Step 1: find the seam.** `measure.ts` currently builds a lexical index
      and calls `narrow`/`rankRangeOf` directly. Read it before changing
      anything and introduce the smallest interface that both kinds of narrower
      satisfy — something that, given a question, yields ranked operation ids
      with scores, from which the existing `rankRangeOf` logic can compute a
      worst and a best rank. **Keep the lexical numbers byte-identical across
      this refactor**: run `make narrowing` before and after and diff the
      lexical rows. A refactor that moves them has changed the measurement.

- [ ] **Step 2: write the failing narrower test** — a vector narrower over a
      fake, tiny catalogue returns operations ordered by cosine similarity, and
      returns fewer than K when the catalogue is smaller than K.

- [ ] **Step 3: write `narrower.ts`** — dot product against the cached
      normalised vectors. Do not add a vector store; a thousand dot products is
      the measurement (spec section 7).

- [ ] **Step 4: widen the report** — a row per (configuration, size, K). The
      header already states the direction; extend it to say what the
      configuration column is. Keep the worst/best columns for every
      configuration, including the ones that never tie (spec section 6).

- [ ] **Step 5: handle llama-swap being down** — the lexical row runs anyway;
      the vector rows say they were skipped and why. Test this with a transport
      that refuses to connect.

- [ ] **Step 6: mark a configuration that failed its contract check** — its
      rows are still printed, labelled as misconfigured, so nobody reads them
      as a fact about the model (AC-V-101).

- [ ] **Step 7: `make fmt && make check`**, then `make narrowing` with
      llama-swap up. Paste the whole table in the commit body only if it is
      short; otherwise say where it is. Commit.

---

### Task 3: the reranking stage

**Files:**

- Create: `e2e/narrowing/rerank/client.ts`, `narrower.ts`, `index.ts`
- Test: `e2e/narrowing/rerank/client.test.ts`, `narrower.test.ts`
- Modify: `e2e/narrowing/measure.ts`

**Consumes:** Tasks 1 and 2.

**Produces:** a sixth configuration — retrieve 50 with `e5-large-q8`, rerank to
K with `bge-reranker-v2-m3-q8` — measured as one mechanism (spec V6).

- [ ] **Step 1: write the failing client test** against a fake transport.
      `/v1/rerank` returns `results: [{index, relevance_score}]` in **input
      order**; the client sorts descending by score and maps back to operation
      ids. Test that the mapping survives an unsorted response — that is the
      bug this test exists for.

- [ ] **Step 2: write `client.ts`**

- [ ] **Step 3: write the two-stage narrower** — retrieve 50, rerank, take K.
      The 50 is a constant with a reason: measured, `e5-large-q8` puts 11 of the
      15 axis-D answers inside 50 and 3 inside 10 (spec section 9). Name it and
      say so where it is defined.

- [ ] **Step 4: add it to the report** as one configuration, not two rows.

- [ ] **Step 5: record the per-question rerank cost** — measured on the probe
      at 70–210 ms per question with the model resident, against 0 ms for a
      resident embedding call. It belongs in the wall-clock column, not in a
      footnote.

- [ ] **Step 6: `make fmt && make check`**, then `make narrowing`. Commit.

---

### Task 4: what it costs to keep loaded, and the record

**Files:**

- Create: `e2e/narrowing/loading.ts`
- Modify: `e2e/narrowing/measure.ts`, `DECISIONS.md`, `STATE.md`, `TODO.md`

**Consumes:** Tasks 1–3.

**Produces:** the alternation cost measured rather than quoted, and every
number recorded.

- [ ] **Step 1: measure the alternation** — with llama-swap in its default
      one-model-at-a-time mode: an embedding call with the model resident, a
      chat call that first unloads it, an embedding call that unloads the chat
      model again, and each of those repeated once while resident. Report
      seconds. This is the one place in the subproject that calls a chat model;
      it calls it with `max_tokens: 1` and does not look at what it says,
      because what is being measured is the loading, not the answer. Keep it
      out of `make check` and out of every other target.

- [ ] **Step 2: print it once in the report**, not per row (spec section 6): it
      does not vary with K or catalogue size, and averaging it into a per-query
      figure would hide the largest number here.

- [ ] **Step 3: run the whole thing** — `make narrowing`, all configurations.

- [ ] **Step 4: record it** — a `DECISIONS.md` entry dated 2026-09-14 with
      recall per axis per configuration, the contract-check results including
      any failure, the rerank cost and the alternation cost. Include the
      configurations that do badly. Do not editorialise the axis-D column: it
      went 0% → 20% → 47% across lexical, embedding and two-stage on the probe,
      and if it lands under half again that is the finding, not a defect.

- [ ] **Step 5: update `STATE.md` and `TODO.md`** — what exists now, and what
      the next decision has in front of it.

- [ ] **Step 6: `make fmt && make check`**, then commit.

---

## What comes after, and is not in this plan

Choosing what the product uses, and wiring it in. Both wait for these numbers.
A hybrid of lexical and vector scoring is the obvious next mechanism and is
deliberately excluded (spec section 7) — it moves two numbers at once, and
neither pure mechanism has been measured yet when this plan starts.
