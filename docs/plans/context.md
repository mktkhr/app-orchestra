# Multi-turn context — implementation plan

> **For the implementer:** one task at a time. Each task ends green and is
> committed on its own. Steps use `- [ ]` for tracking.

**Goal:** A second question knows what the first one asked.

**Architecture:** The browser keeps the conversation, keyed and held above the
screen that draws it, and sends the turns with each question. The platform
renders them into the prompt after the catalogue and never stores them. The
model sees what was asked and what was decided, never a row of any answer.

**Spec:** `docs/specs/context.md`. Acceptance criteria: its section 8.

## Global constraints

Everything in `docs/plans/auth.md`'s Global constraints section still holds.
The ones this subproject is most likely to meet:

- `make check` must never call a real LLM, and must need nothing running.
  Verify by counting `docker logs llama-swap 2>&1 | grep -c 'POST /v1/chat/completions'`
  before and after.
- Contract first: change `services/platform/api/openapi.yaml`, run
  `make generate`. Never edit generated files.
- Layer order (`make guard-arch`) and Feature-Sliced Design (`make guard-fsd`):
  app → pages → widgets → features → entities → shared. A feature may not
  import a sibling feature; composing two of them is what `widgets` is for.
- 1000 lines a file; 90% statement coverage per Go package.
- Controls come from MUI. A disabled contained button has no edge the layout
  guard can see. `TextField size="small"` is under the 44px floor.
- Run `make -k check` while working; `make build` before `make guard-browser`.
- Verify in a browser at the Tailscale address, not at `localhost`.
- **Run `make check` again after the last edit, before reporting.** Green in the
  middle is not evidence.

---

### Task 0: a conversation knows what it is

**Files:**

- Create: `web/src/features/conversation/model/conversationStore.ts` and tests
- Modify: `web/src/features/conversation/ui/Conversation.tsx`,
  `web/src/widgets/conversation/ui/ConversationPanel.tsx`,
  `web/src/app/`, `web/src/pages/chat/`, `web/src/pages/workspace/`

**Produces:** conversations held above the screen, keyed — `"chat"`, and one per
workspace id — and a control that ends one.

This is `docs/specs/context.md` section 3a, and it lands before anything is sent
anywhere. Nothing about the contract changes in this task.

- [ ] **Step 0** Confirm the bug first: open the chat, ask something, open a
      workspace, come back. Write the test that fails because the turns are gone.
- [ ] **Step 1** Lift the turns out of `Conversation`'s own `useState` into
      something the app holds, keyed per screen. Make the test pass.
- [ ] **Step 2** Add the control that ends a conversation, and the test that the
      next question starts from nothing.
- [ ] **Step 3** Web gates green, then `make build` and `make guard-browser`.
- [ ] **Step 4** Commit: `fix(web): give a conversation a life of its own`

**Satisfies:** AC-M-107, AC-M-108.

---

### Task 1: the platform takes the turns

**Files:**

- Modify: `services/platform/api/openapi.yaml`,
  `internal/usecase/planner.go`, `orchestrator.go`, tests

**Consumes:** nothing; it can run beside Task 0.

**Produces:** `turns` on `POST /api/plan`, a `[]Turn` argument on
`usecase.Planner.Plan`, and the window from `ORCHESTRA_CONTEXT_TURNS`.

```go
type Turn struct {
    Question    string
    Kind        DecisionKind
    Service     string
    OperationID string
    Args        map[string]any
}
```

The platform truncates to the most recent `ORCHESTRA_CONTEXT_TURNS`, oldest
dropped first (section 6). No planner renders them yet.

- [ ] **Step 1** Add `turns` to the contract. `make api-lint`, `make generate`.
- [ ] **Step 2** Write the test: more turns than the window leaves the most
      recent, oldest first dropped; no turns behaves exactly as today. Run it,
      expect failure.
- [ ] **Step 3** Implement. Thread the turns to the planner port.
- [ ] **Step 4** Go gates green.
- [ ] **Step 5** Commit: `feat(platform): carry a conversation to the planner`

**Satisfies:** AC-M-104, AC-M-105.

---

### Task 2: both planners read them

**Files:**

- Modify: `internal/adapter/planner/toolcall/planner.go`,
  `internal/adapter/planner/jsonmode/planner.go`, tests

**Consumes:** Task 1.

**Produces:** the turns rendered into the prompt, after the catalogue (M3), by
both adapters.

**No row of any answer is rendered.** A `Turn` has no data field to render, and
a test asserts the prompt contains nothing from one.

- [ ] **Step 1** Write the test for the tool-calling planner against `httptest`:
      the request body's messages carry the earlier question and the operation it
      resolved to, after the tools, and nothing else. Run it, expect failure.
- [ ] **Step 2** Implement for `toolcall`.
- [ ] **Step 3** The same for `jsonmode`, whose catalogue is already text — the
      turns go after it.
- [ ] **Step 4** Add the test that the tool list is byte-identical with and
      without turns (AC-M-102).
- [ ] **Step 5** Go gates green; the live tests still skip.
- [ ] **Step 6** Commit: `feat(platform): let the model read the conversation`

**Satisfies:** AC-M-102, AC-M-103, AC-M-106.

---

### Task 3: the browser sends them

**Files:**

- Modify: `web/src/shared/api/client.ts`,
  `web/src/features/conversation/`, tests

**Consumes:** Tasks 0-2.

**Produces:** every question carries the conversation it belongs to.

A turn is built from what the browser already has: the question it asked and the
`source` or `target` the platform answered with.

- [ ] **Step 1** Write the test: a second question posts the first one's
      question and operation in `turns`; a first question posts none. Run it,
      expect failure.
- [ ] **Step 2** Implement.
- [ ] **Step 3** Web gates green, then `make build` and `make guard-browser`.
- [ ] **Step 4** Commit: `feat(web): ask the next question in context`

---

### Task 4: end to end

**Files:**

- Create: `e2e/src/context.test.ts`, and a browser journey
- Modify: `docs/specs/orchestration.md` (D8's wording), `PRODUCT.md` if needed

**Consumes:** everything.

**Produces:** the journey against the built product: ask about one service, then
ask a follow-up that names no service, and watch it answered from the same one.

D8 says "The API result never goes back to the LLM. One request is one LLM call."
Both halves are still true and the first one now has a companion sentence: the
question and the decision do go back. Say so where D8 is written.

- [ ] **Step 1** Write the process-level journey with the stub planner, whose
      fixture table can key on the turns. Run it, expect failure, make it pass.
- [ ] **Step 2** Write the browser journey: two questions, the second phrased
      with no service name.
- [ ] **Step 3** Measure it against the local model by hand, the way the planner
      comparisons were measured: five runs of a follow-up, recorded in
      `DECISIONS.md` whether it resolves or not. **A model that cannot do this is
      a finding, not a failure** — the platform's part is what these tests fix.
- [ ] **Step 4** `make check` in full — every gate green, and no request added
      to the model's log.
- [ ] **Step 5** Commit: `test(e2e): ask a second question`

**Satisfies:** AC-M-101, and the whole of section 8 end to end.

---

## Order and parallelism

Task 0 is the browser's own and Task 1 is the platform's; they touch nothing in
common and can run together. Task 2 needs Task 1. Task 3 needs 0 and 2. Task 4
needs all of it.

## Done

`make check` is green, every criterion in `docs/specs/context.md` section 8 has
a test, and a follow-up question is answered from the service the question
before it used — or `DECISIONS.md` records which local models can and cannot do
that, measured rather than assumed.
