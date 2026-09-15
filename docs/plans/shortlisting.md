# The platform narrows before it plans — implementation plan

> **For the implementer:** one task at a time, in order - each depends on
> the one before. Each task ends green and is committed on its own. Steps use
> `- [ ]`.

**Goal:** `/api/plan` narrows the catalogue to a reranked shortlist before the
planner sees it, the answer carries two alternatives, and the product is
measured on the corpus with correct@1 and correct@shown.

**Architecture:** a `Narrower` port in the usecase, called once in
`Orchestrator.Plan` between `catalogFor` and `ToolsFor`; a llama-swap adapter
behind it (embeddings + rerank over HTTP, vectors held in memory from load);
a pass-through when unconfigured. The contract gains `alternatives` on a
result and `preferred` on a request. The browser shows the alternatives as
chips under a result. A runner in `e2e/` measures the running platform.

**Spec:** `docs/specs/shortlisting.md`. Acceptance criteria: its section 8.

## Global constraints

Everything in `docs/plans/proposing.md`'s Global constraints holds. Here in
particular:

- **`make check` calls no model and needs nothing running.** The narrowing
  adapter is tested against an `httptest` fake; the pass-through narrower is
  what every existing test runs with (AC-H-101, AC-H-105). Verify with
  `docker logs llama-swap 2>&1 | grep -c 'POST /v1/'` before and after.
- **Contract first.** `services/platform/api/openapi.yaml`, `make generate`,
  never edit generated files. Every new `enum` needs `x-enum-labels`.
- **Layers.** `domain` imports stdlib only; the usecase may not import
  `net/http` or `encoding/json`; vectors and HTTP live in the adapter
  (`make guard-arch`). 90% statement coverage per Go package.
- **FSD** for the browser (`make guard-fsd`); MUI directly (AGENTS.md rule 6).
- **Order is data.** The shortlist's order reaches `ToolsFor` unchanged; a
  test asserts the tools come out in reranker order (AC-H-102). Measured, the
  picker reads from the top and the order is worth four to five points.
- **Thinking eats `max_tokens`.** Not this subproject's call path - the
  planner's request is unchanged - but the measurement's smoke test still
  asserts the planner's first response was not truncated.
- No suppressions; 1000 lines a file (Go), 300 (TS, oxlint); `make fmt`
  before `make check`; `git commit -- <explicit paths>`; another session's
  `e2e/eval/*.ts` is uncommitted and stays.

---

### Task 1: the `Narrower` port, its llama-swap adapter, and the wiring

**Files:**

- Modify: `services/platform/internal/usecase/ports.go` (the port),
  `orchestrator.go` (call it), `orchestrator_test.go` (+ a fake)
- Create: `services/platform/internal/adapter/narrowing/llamaswap/`
  (`narrower.go`, `embed.go`, `rerank.go`, `index.go`, tests, `testdata`)
- Modify: `services/platform/internal/infra/config/config.go` (+ test),
  `services/platform/pkg/app/app.go` (+ test)

**Consumes:** `domain.Endpoint.Examples` (`3f28509`), `ORCHESTRA_LLM_BASE_URL`.

**Produces:**

```go
// usecase/ports.go
type Narrower interface {
    // Narrow returns at most k endpoints of catalog, best first, for query.
    // A Narrower may return catalog unchanged; the orchestrator does not care.
    Narrow(ctx context.Context, catalog domain.Catalog, query string, k int) (domain.Catalog, error)
}
```

and `PassThroughNarrower{}` in the usecase, which returns its input.

- [ ] **Step 1: failing orchestrator test** - with a fake `Narrower` that
      returns three named endpoints in a given order, `Plan` offers the
      planner exactly those three (plus built-ins) **in that order**; with
      `PassThroughNarrower`, the tools are byte-identical to today's
      (AC-H-101, AC-H-102). Use the existing tools-don't-move test pattern.

- [ ] **Step 2: wire it** - `Orchestrator` gains a `Narrower` field (defaulted
      to pass-through by `NewOrchestrator` so no existing call site changes);
      `Plan` calls `o.narrower.Narrow(ctx, catalog, query, o.narrowK)` after
      `catalogFor` and before `ToolsFor`. Keep the narrowed catalogue in scope:
      Task 2 reads alternatives from it.

- [ ] **Step 3: failing adapter tests** against `httptest.Server` fakes of
      `/v1/embeddings` and `/v1/rerank`: (a) `Load(catalog)` posts every
      operation's text **and each of its examples** as documents once, with
      the `passage: ` prefix, and holds their vectors; (b) `Narrow` posts the
      query once with `query: `, scores each endpoint as the **max** cosine
      over its own vector and its examples' vectors, takes the top 50, posts
      one rerank with documents = the operation's text **plus its examples**
      (spec H1: the reranker reads them), sorts by `relevance_score`
      descending (the API returns them unsorted - assert this), returns the
      top k; (c) an endpoint whose examples are empty still works; (d) an
      error from either endpoint is returned, not swallowed.

- [ ] **Step 4: write the adapter.** `net/http` + `encoding/json` here only.
      Vectors as `[]float32`, normalised at load. The document text is the
      same four fields `e2e/narrowing/lexical.ts`'s `combinedTextOf` joins -
      summary, description, display name, service display name - so the
      product retrieves against what was measured; say so in a comment.

- [ ] **Step 5: config.** `ORCHESTRA_NARROWING_EMBED_MODEL`,
      `ORCHESTRA_NARROWING_RERANK_MODEL`, `ORCHESTRA_NARROWING_K`. All three or
      none; one or two set is `config.ErrNarrowingIncomplete` at startup, the
      way a missing DB path is. Test all four combinations.

- [ ] **Step 6: `pkg/app`.** When configured, build the adapter with the
      catalogue at startup (`Load` before the server listens - AC-H-104), log
      one line with the vector count and the load time; otherwise the
      pass-through. A test asserts the pass-through is used when unconfigured.

- [ ] **Step 7: `make fmt && make check`**, call counts unchanged, commit.

---

### Task 2: alternatives on the result, `preferred` on the request

**Files:**

- Modify: `services/platform/api/openapi.yaml` (`PlanRequest.preferred`,
  `PlanResult` result variant gains `alternatives`), then `make generate`
- Modify: `usecase/orchestrator.go` (+ tests), `adapter/handler/plan.go`
  (+ tests)

**Consumes:** Task 1's narrowed catalogue in `Plan`.

- [ ] **Step 1: contract.** `alternatives`: array (max 2) of
      `{operationId, displayName, service}`, optional, on the `result` kind
      only. `preferred`: optional string on `PlanRequest`. `make generate`.

- [ ] **Step 2: failing tests.** With narrowing returning `[a, b, c, d]` and
      the planner choosing `b`, `alternatives` is `[c, d]` - the shortlist
      positions **after** the chosen one, not the top two (AC-H-103). With
      the pass-through, no alternatives. With `preferred: c`, the narrower is
      bypassed, the catalogue is exactly `[c]`, the planner is offered `c`
      alone, and the result has no alternatives.

- [ ] **Step 3: implement**, thread through the handler, `make fmt && make
check`, commit.

---

### Task 3: 「違いましたか？」

**Files:**

- Modify: `web/src/features/conversation/model/conversationStore.tsx`
  (+ test), the result rendering under `web/src/features/conversation/ui/`
  or `widgets/conversation/` - read where a `result` is drawn before choosing
- Test: vitest beside the component; `web/acceptance` if the existing
  conversation acceptance covers results

**Consumes:** Task 2's contract (`web/src/shared/api/gen/platform.d.ts`).

- [ ] **Step 1: failing store test** - `ask(question, {preferred})` posts
      `preferred`; a result with alternatives keeps them on the message.

- [ ] **Step 2: failing component test** - a result with two alternatives
      renders a line 「違いましたか？」 followed by two MUI `Chip`s (display
      name; the service as secondary text or `title`); clicking one calls
      `ask` with the same question and that `preferred`. No alternatives, no
      line - not an empty row.

- [ ] **Step 3: implement.** MUI `Chip` directly. Keyboard reachable (the
      browser gate measures target size and a11y - `make guard-browser` after
      `make build`).

- [ ] **Step 4: `make fmt && make check`**, commit.

---

### Task 4: measure the product, and record

**Files:**

- Create: `e2e/shortlist/run.ts`, `score.ts`, `report.ts`, `boot.ts` (+ tests
  with fakes for `score`/`report`)
- Modify: `Makefile` (`eval-shortlist`; protected - `ORCHESTRA_ALLOW_HARNESS_CHANGE=1`)
- Write (not commit): a `groups` block for `local-llm/deploy/llama/config.yaml`
  keeping `qwen3.5-9b-q8`, `e5-large-q8` and `bge-reranker-v2-m3-q8`
  resident - hand to the owner of that repository to commit (spec H4)
- Modify: `DECISIONS.md`, `STATE.md`, `TODO.md`

**Consumes:** Tasks 1-3; `e2e/narrowing/serve.ts` (the fixture server);
`e2e/narrowing/corpus/`; `e2e/src/helpers/auth.ts` (sign-in).

- [ ] **Step 1: `boot.ts`** - starts the fixture server and the platform
      binary (built by `make build`) on free ports with `ORCHESTRA_SERVICES`
      naming the five fixture services and narrowing on or off per flag; no
      `ORCHESTRA_LLM_*` beyond what the planner needs. Kills both on exit,
      and verifies with a port probe that nothing is left listening - the
      memory-kill history of this repository is from leftover processes.

- [ ] **Step 2: smoke, three questions**, before a hundred: the planner's
      response is a `result`/`ask`/`none`, not an error; the first response's
      wall-clock is printed; if it exceeds five seconds, say so - that is a
      model load and H4 is not met.

- [ ] **Step 3: `score.ts`** - per question: `correct@1` (result's operation
      ∈ answers), `correct@shown` (that, or any alternative ∈ answers),
      `asked` (`ask`), `none`, `error`; per axis and overall, eligible
      questions only (all 100 at size 5). Latency per question, and the
      narrowing stage's own time if the platform logs it (add a one-line log
      in Task 1's adapter if not: `narrowing: k=20 in 87ms`).

- [ ] **Step 4: run both ways** - narrowing on, narrowing off. The off run
      sends every question against the whole 1000-operation catalogue, which
      is the product today; expect it to be slow and to look like the
      2026-09-15 whole-catalogue frontier run. Record both.

- [ ] **Step 5: `make eval-shortlist`** - depends on `build`; not quiet; not
      in `check`. Prints the table for both runs, with the stand-in picker's
      row (83 / K=20) and the recall@20 row beside them so the reader sees
      where the real planner landed.

- [ ] **Step 6: record.** `DECISIONS.md`: both tables, latencies, whether any
      request paid for a load, the `groups` block that was configured, and -
      plainly - how the product's planner compares to the stand-in picker and
      to the shortlist ceiling. `STATE.md`. `TODO.md`: item 3 closes.
      **Do not edit `PRODUCT.md` D2** - AC-H-108 leaves that to its owner;
      put the numbers where they will be read and stop.

- [ ] **Step 7: `make fmt && make check`**, commit.

---

## What comes after, and is not in this plan

D2's revision, by its owner, with these numbers. A prompt-evaluation loop for
the planner, now that one sentence is known to move a picker six points.
Whether people click the alternatives.
