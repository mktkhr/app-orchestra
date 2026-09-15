# What the planner is told — implementation plan

> **For the implementer:** Tasks 1 and 2 touch disjoint files and run in
> parallel; Task 3 follows both. Each task ends green and is committed on its
> own. Steps use `- [ ]`.

**Goal:** the planner's words are a named set selected by
`ORCHESTRA_PLANNER_WORDING`; `v1` is today's text byte for byte; four
candidates exist; `make eval-shortlist --wording` measures them in one run;
the record decides.

**Architecture:** a `wording` package in the toolcall planner's adapter holds
the sets as Go values. The planner applies the selected set when it builds
the request: the system message, the three built-in tools' descriptions, and
each catalogue tool's description (from the endpoint's summary, and for `v5`
its examples). The usecase carries the examples as data on `Tool`; it never
sees prompt text. The runner boots one platform per wording.

**Spec:** `docs/specs/wording.md`. Acceptance criteria: its section 7.

## Global constraints

`docs/plans/shortlisting.md`'s Global constraints hold. Here in particular:

- **`v1` is byte-identical.** A test compares every `v1` string to the
  literals as they stand at `5bf5cf8` (copy them into the test as constants
  with that hash in a comment). AC-Q-101.
- **No mechanics change.** Temperature, `max_tokens`, tool schemas, narrowing,
  K - untouched. A wording changes strings only.
- **`make check` calls no model.** Wording tests check shape (every field
  non-empty, names unique, the catalogue-tool builder produces the expected
  text for a fake endpoint). Verify `docker logs llama-swap 2>&1 | grep -c
'POST /v1/'` before and after.
- Layers: the usecase gains `Tool.Examples []string` (data); the adapter
  renders. `internal/domain` stdlib only.
- One measurement run per wording; the product measurement is deterministic
  (`DECISIONS.md`, 2026-09-15: two runs, 100 of 100 identical).
- `git commit -- <explicit paths>`; another session's `e2e/eval/*.ts` stays.

---

### Task 1: the `wording` package, `v1`, the four candidates, the switch

**Files:**

- Create: `services/platform/internal/adapter/planner/wording/` — `wording.go`
  (the type, `ByName`, `Default`), `v1.go`, `v2_commit.go`,
  `v3_ask_on_collision.go`, `v4_commit_and_ask.go`, `v5_examples_in_tools.go`,
  tests
- Modify: `services/platform/internal/adapter/planner/toolcall/planner.go`
  (take a `wording.Wording`; apply it in `buildMessages` and when serialising
  tools), `internal/usecase/tools.go` (`Tool.Examples`, filled by `ToolsFor`
  from `Endpoint.Examples`), `internal/infra/config/config.go`
  (`ORCHESTRA_PLANNER_WORDING`), `pkg/app/app.go`

**Produces:**

```go
package wording

type Wording struct {
    Name             string
    SystemPrompt     string
    AskUser          string
    ListCapabilities string
    ProposePanel     string
    // CatalogueTool renders one catalogue operation's tool description.
    CatalogueTool    func(summary string, examples []string) string
}

func Default() Wording            // v1
func ByName(name string) (Wording, bool)
func Names() []string
```

- [ ] **Step 1: failing byte-identity test** — `v1.SystemPrompt`,
      `v1.AskUser`, `v1.ListCapabilities`, `v1.ProposePanel` equal the
      literals copied from `toolcall/planner.go` and `usecase/tools.go` at
      `5bf5cf8`; `v1.CatalogueTool("s", examples)` returns `"s"` regardless of
      examples.

- [ ] **Step 2: move the literals** into `v1.go` and make the planner read
      them from its `Wording` — `usecase.AskUserTool()` etc. keep their
      descriptions (the usecase's own text stays for `jsonmode` and for any
      caller that does not go through this adapter), and the toolcall planner
      **overrides** the three built-ins' descriptions by name when it
      serialises tools. Assert in the planner's existing wire-bytes test that
      with `v1` the request is byte-identical to before this change.

- [ ] **Step 3: `Tool.Examples`** — `ToolsFor` copies `Endpoint.Examples`
      onto the catalogue tool; a test asserts it; the wire is unchanged under
      `v1` because `v1.CatalogueTool` ignores them.

- [ ] **Step 4: the four candidates** as written in spec section 4. Each
      file's doc comment names the measured miss it targets and quotes the
      sentence it adds or changes. `v5.CatalogueTool` appends
      `\n例: 「…」「…」` after the summary when examples exist. Shape tests:
      names unique, every field non-empty, `ByName` round-trips, unknown →
      `false`.

- [ ] **Step 5: config and app.** `ORCHESTRA_PLANNER_WORDING` unset → `v1`;
      unknown → startup error naming the known names. `pkg/app` passes the
      selected wording to the toolcall planner (the jsonmode planner is out
      of scope - it is not what the product measures).

- [ ] **Step 6: `make fmt && make check`**, call counts unchanged, commit.

---

### Task 2: `--wording` on the runner (parallel with Task 1)

**Files:**

- Modify: `e2e/shortlist/run.ts`, `boot.ts`, `report.ts`, `print-report.ts`,
  `score.ts` (+ tests)
- Modify: `Makefile` (`eval-shortlist` accepts `WORDING=`; protected -
  `ORCHESTRA_ALLOW_HARNESS_CHANGE=1`)

**Consumes:** nothing from Task 1 at build time - only the environment
variable name `ORCHESTRA_PLANNER_WORDING`, agreed here.

- [ ] **Step 1: `run.ts --wording a,b,c`** — one platform boot per name,
      narrowing on, K=20, `ORCHESTRA_PLANNER_WORDING=<name>`; rows written to
      `out/on-<name>.jsonl`; `v1` always included first even if not named.
      `--off-only` and the existing default (no `--wording`) keep working
      and keep writing `on.jsonl` / `off.jsonl`.

- [ ] **Step 2: the miss list per wording** — after each pass, write
      `out/misses-<name>.txt`: every row whose pick is not an answer, with
      kind, pick, alternatives and answers, grouped by axis. This is what the
      record quotes.

- [ ] **Step 3: report** — one block per wording, `v1` first, same columns;
      the stand-in picker's row once at the bottom. `print-report.ts` reads
      whichever `on-*.jsonl` exist.

- [ ] **Step 4: Makefile** — `make eval-shortlist WORDING=v2-commit,v3-ask-on-collision`
      passes through; without `WORDING` the target behaves as today.

- [ ] **Step 5: `make fmt && make check`**, call counts unchanged, commit
      (the Makefile hunk with the flag).

---

### Task 3: the run, `make eval`, the record

**Files:** `DECISIONS.md`, `STATE.md`, `TODO.md`; `docs/specs/wording.md` if a
number in it is now wrong. **`PRODUCT.md` untouched.**

- [ ] **Step 1: `make eval-shortlist WORDING=v2-commit,v3-ask-on-collision,v4-commit-and-ask,v5-examples-in-tools`**
      — about ten minutes per wording, foreground; results and miss lists
      copied to the session scratchpad before anything else, as the previous
      record's discipline requires. The `v1` block must reproduce `65 / 68`.

- [ ] **Step 2: `make eval`** for whichever wording the tables favour (and
      for `v1`, so the two can be compared on the real services).

- [ ] **Step 3: the record** — every wording's table and three quoted misses
      each; what moved on each of the five axes, both directions; `make
eval` for `v1` and the proposed default; the decision, including "keep
      `v1`" if nothing clears it. The decision's grounds are the numbers, not
      the prose of the prompt.

- [ ] **Step 4: if a candidate becomes the default**, change
      `wording.Default()` in one commit that cites the record; `v1` stays in
      the package as the baseline it is.

- [ ] **Step 5: `STATE.md`, `TODO.md`**; `make fmt && make check`; commit.
