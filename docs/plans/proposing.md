# Asking for a panel — implementation plan

> **For the implementer:** one task at a time. Each task ends green and is
> committed on its own. Steps use `- [ ]` for tracking.

**Goal:** A person on a workspace screen says 「在庫をステータス別に棒グラフで
置いて」 and gets a filled-in panel to accept with one press.

**Architecture:** `propose_panel` is a tool the model **answers with**, like
`ask_user`. The platform turns the call into a fifth result kind, fills in
what the model left out from the catalogue, and the browser draws the
builder's own form already filled. Nothing is written until a person presses
the control that places it, through the endpoint that already exists.

**Spec:** `docs/specs/proposing.md`. Acceptance criteria: its section 7.

## Global constraints

Everything in `docs/plans/routing.md`'s Global constraints section still
holds. The ones this subproject is most likely to meet:

- `make check` must never call a real LLM, and must need nothing running.
  Verify by counting
  `docker logs llama-swap 2>&1 | grep -c 'POST /v1/chat/completions'`
  before and after.
- Contract first: change `services/platform/api/openapi.yaml`, run
  `make generate`. Never edit generated files. Every new `enum` needs
  `x-enum-labels`.
- Layer order (`make guard-arch`): `domain` imports stdlib only; the usecase
  may not import `net/http` or `encoding/json`. FSD (`make guard-fsd`).
- 90% statement coverage per Go package; 1000 lines a file.
- **No suppressions of any kind** (AGENTS.md rule 2) — `//nolint`,
  `@ts-ignore`, `istanbul ignore`, `as any` and every relative. An
  unreachable branch gets deleted, not silenced.
- **Run `make check`, not `make -k check`, before reporting.** `-k`
  continues past failures and is a weaker claim; four agents have reported
  under it.
- `make build` before `make guard-browser`. `make dev-services` rebuilds and
  restarts the dummy services and the platform.
- On a flaky browser run, check `uptime` and say what it was
  (`DECISIONS.md`, 2026-09-13, "the browser suite's flake is load,
  measured"). Licence to look at the machine, not to rerun until green.

---

### Task 0: the model can answer with a panel

**Files:**

- Modify: `services/platform/api/openapi.yaml`,
  `internal/usecase/tools.go`, `internal/usecase/orchestrator.go`,
  `internal/adapter/planner/toolcall/planner.go`,
  `internal/adapter/planner/jsonmode/planner.go`,
  `internal/adapter/planner/stub/`, `internal/adapter/handler/plan.go`,
  tests

**Consumes:** nothing.

**Produces:** `propose_panel` in the tool list, a `DecisionProposal` the two
planners map onto, and `kind: "proposal"` on `/api/plan`'s response.

```
propose_panel(service, operationId, args, component?, chart?, transform?, title?)
```

Spec section 3 and 4. Three things worth getting right:

- **It is a fifth `kind`, not a `result` with an extra field** (section 4).
  A browser that told them apart by looking for a field is one refactor from
  drawing the wrong thing.
- **The platform fills in what the model left out**, from the catalogue: the
  component from `domain.Render`, the axes from `x-ui-hint.chart` when the
  contract declares them, the title from the operation's display name. The
  model's own values win where it gave them. The browser never has to.
- **Refusal is free.** A proposal naming an operation the person may not
  call cannot happen, because the catalogue the planner was offered never
  held it (`docs/specs/auth.md` A4) — but write the test that says so
  (AC-N-105), because "cannot happen" is a claim about code somebody will
  change.

The stub planner answers with whichever decision its fixture names, so
`make check` drives this path without a model (AC-N-106).

- [ ] **Step 1** Add the `proposal` kind and its payload to the contract.
      `make api-lint`, `make generate`.
- [ ] **Step 2** Write the tests: the tool is offered; a `propose_panel`
      call becomes a proposal with the model's own values; one that omits
      the view gets the catalogue's; one naming an operation outside the
      person's catalogue is refused; the JSON planner reaches the same
      decision shape as the tool-calling one. Run them, expect failure.
- [ ] **Step 3** Implement.
- [ ] **Step 4** Go gates green.
- [ ] **Step 5** Commit: `feat(platform): let the model answer with a panel`

**Satisfies:** AC-N-105, AC-N-106, and AC-N-101 at the platform level.

---

### Task 1: the person sees it and places it

**Files:**

- Modify: `web/src/entities/rendering/`, `web/src/features/conversation/ui/TurnList.tsx`,
  `web/src/features/panels/`, `web/src/pages/workspace/`, tests

**Consumes:** Task 0.

**Produces:** a proposal drawn in the conversation as the builder's own
form, filled in, with one control that places it.

Spec section 5. The form is `features/panels`' own (`docs/specs/dashboard.md`
P12, P7) — **opened over a proposal rather than over nothing**, which is the
same thing Task 2 of the dashboard plan did for editing a panel. Do not
write a third form.

**Only the workspace screen's conversation offers one** (N4, AC-N-104). The
chat screen has no workspace to put anything on. Work out where that
decision belongs — the component that draws a turn, or the thing that
decides what a turn can be — and say why you put it there.

Placing it is `POST /api/workspaces/{id}/panels`, unchanged. The model is
not in that request.

- [ ] **Step 1** Write the test: a proposal turn draws the form filled in;
      pressing the control posts the panel and the workspace shows it;
      editing before placing places the edit (AC-N-103); the chat screen's
      conversation draws no proposal even when handed one (AC-N-104).
- [ ] **Step 2** Implement.
- [ ] **Step 3** Web gates green, then `make build`, `make guard-browser`,
      `make guard-a11y`, `make guard-layout`.
- [ ] **Step 4** Commit: `feat(web): accept a panel the model filled in`

**Satisfies:** AC-N-102, AC-N-103, AC-N-104.

---

### Task 2: end to end, and what the tool costs

**Files:**

- Create: `e2e/src/proposing.test.ts`, `e2e/browser/proposing.spec.ts`
- Modify: `e2e/eval/cases.ts` if Step 3 decides it, `STATE.md`, `TODO.md`,
  `DECISIONS.md`

**Consumes:** everything.

**Produces:** the journey — ask a workspace's chat for a chart of something,
accept it, reload, see the panel — and a measurement of what offering one
more tool did to every other answer.

Step 3 is the one that cannot be skipped. `docs/specs/proposing.md`
section 9: `propose_panel` rides in **every** request's tool list, on every
question, whether or not a workspace is open. The corpus is the only thing
that says whether that changed how the model answers questions with nothing
to do with panels.

- [ ] **Step 1** Write the process-level journey with the stub planner.
- [ ] **Step 2** Write the browser journey.
- [ ] **Step 3** **Measure.** Run `make eval` before this subproject's
      commits and after — `git stash` or a worktree, whichever is honest —
      and report both. `no-enum-value` is judged on `reject` at n=30 and its
      band is 15-22 (`docs/specs/eval.md` section 4a), so a move inside that
      says nothing and a move outside it says something. If it moved, say so
      plainly and do not adjust the corpus to make it look flat.
- [ ] **Step 4** `make check` in full — every gate green, and no request
      added to the model's log by `make check` itself.
- [ ] **Step 5** Commit: `test(e2e): ask for a panel and accept it`

**Satisfies:** AC-N-101 end to end, and section 9's own question.

---

## Order and parallelism

Task 0 blocks everything. Task 1 needs it. Task 2 needs both. Nothing runs
in parallel.

## Done

`make check` is green, every criterion in `docs/specs/proposing.md` section
7 has a test that runs in CI, and the eval corpus has a before and after for
the tool this subproject adds to every request.
