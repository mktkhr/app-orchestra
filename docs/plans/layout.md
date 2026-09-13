# Arranging a workspace — implementation plan

> **For the implementer:** one task at a time. Each task ends green and is
> committed on its own. Steps use `- [ ]` for tracking.

**Goal:** A person decides how wide and how tall each panel is, and what
order the panels come in, and it is still that way after a reload.

**Architecture:** A panel gains `width` and `height`, spans in a CSS grid,
and `position` (which has existed unused since `docs/specs/workspaces.md`
W5) becomes the order. All three are saved through the `PATCH` that already
exists. No grid engine and no dragging — `docs/specs/layout.md` section 4 is
the argument.

**Spec:** `docs/specs/layout.md`. Acceptance criteria: its section 8.

## Global constraints

Everything in `docs/plans/dashboard.md`'s Global constraints section still
holds. The ones this subproject is most likely to meet:

- `make check` must never call a real LLM, and must need nothing running.
  Verify by counting
  `docker logs llama-swap 2>&1 | grep -c 'POST /v1/chat/completions'`
  before and after.
- Contract first: change `services/platform/api/openapi.yaml`, run
  `make generate`. Never edit generated files. Every new `enum` needs
  `x-enum-labels`.
- Layer order (`make guard-arch`): domain ← usecase ← adapter ← infra ← app
  ← cmd. `domain` imports stdlib only. FSD for the web
  (`make guard-fsd`).
- 1000 lines a file; 90% statement coverage per Go package.
- **No suppressions of any kind** (AGENTS.md rule 2) — `//nolint`,
  `@ts-ignore`, `istanbul ignore` and every relative. An unreachable branch
  gets deleted, not silenced.
- Controls come from MUI at the point of use. A disabled contained button
  has no edge `make guard-layout` can see.
- **`make guard-a11y` and `make guard-layout` now measure every screen**,
  including a workspace and the panel builder
  (`harness/quality/browser/screens.ts`, restored in `d008dfc`). They were
  measuring only the sign-in screen before that, so "it passed the browser
  gates" is a claim worth more than it used to be — and the gate measures an
  empty workspace, so a panel's own size is not yet covered by it.
- Run `make -k check` while working; `make build` before
  `make guard-browser`.
- `make dev-services` rebuilds and restarts the dummy services if you verify
  live.

## File structure

```
services/platform/
  api/openapi.yaml                        Panel + width/height; PATCH + position
  internal/domain/workspace.go            Panel.Width, Panel.Height
  internal/adapter/repository/sqlite/      two more columns, migrated like view
  internal/usecase/workspaces.go          UpdatePanel takes them; clamping
web/src/
  pages/workspace/ui/WorkspacePage.tsx    the CSS grid
  entities/workspace/ui/PanelCardShell.tsx   the size and move controls
  features/panels/                        reuse for the PATCH it already makes
e2e/
  src/layout.test.ts, browser/layout.spec.ts
```

---

### Task 0: a panel has a size

**Files:**

- Modify: `services/platform/api/openapi.yaml`,
  `internal/domain/workspace.go`,
  `internal/adapter/repository/sqlite/{migrate.go,schema.go,store.go}`,
  `internal/usecase/workspaces.go`, `internal/adapter/handler/workspace.go`,
  tests

**Consumes:** nothing.

**Produces:** `width` and `height` on `Panel` and on the contract, accepted
by `POST` and `PATCH`, and `position` accepted by `PATCH`.

Two more nullable columns, migrated exactly the way `view` was in
`docs/plans/dashboard.md` Task 2 — `modernc.org/sqlite` rejects
`ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, so `migrate.go` checks
`pragma_table_info` first. **Read that file before writing this one**, and
extend its test that opens a database written before the column existed
(AC-L-104): a panel with no width or height must read back as the default.

Clamping belongs in the usecase, not the browser: a `width` of 40 or 0 is
not a thing the grid can draw, and a caller that is not this repository's own
frontend will send one.

- [x] **Step 1** Add `width`/`height` to `Panel`, `CreatePanelRequest` and
      `UpdatePanelRequest`, and `position` to `UpdatePanelRequest`.
      `make api-lint`, `make generate`.
- [x] **Step 2** Write the store test: a panel with a size round-trips; one
      without reads back as the default; a database file written before
      these columns existed opens and its panels read back as the default.
      Run it, expect failure.
- [x] **Step 3** Implement, including the migration and the clamp. Decide
      what the defaults are (the spec says full width, one row) and where
      they live — the domain, not three callers.
- [x] **Step 4** Go gates green.
- [x] **Step 5** Commit: `feat(platform): let a panel carry its own size`

**Satisfies:** AC-L-104 at the store level.

---

### Task 1: the workspace is a grid

**Files:**

- Modify: `web/src/pages/workspace/ui/WorkspacePage.tsx`,
  `web/src/entities/workspace/ui/PanelCardShell.tsx`, tests

**Consumes:** Task 0.

**Produces:** panels drawn in a CSS grid — twelve columns where there is
room, one where there is not (L4) — each spanning its own `width` and
`height`, in `position` order.

MUI's `Box` with `display: "grid"`, not a third-party grid. A panel's span is
clamped to the columns that exist, so a `width: 12` panel on a phone is one
full-width panel and not a sideways scroll.

- [ ] **Step 1** Write the test: three panels of widths 12, 6 and 6 draw one
      full-width row and two beside each other; at 375px all three span the
      single column; a panel with no width draws full width (AC-L-104,
      AC-L-105). Run it, expect failure.
- [ ] **Step 2** Implement.
- [ ] **Step 3** Check AC-L-106 by hand and say what you saw: a wider panel
      should draw a _wider chart_, because `ResultChart` takes its size from
      its caller since `docs/plans/dashboard.md` Task 5. If it draws the same
      chart in a wider box, that is this task's bug to fix.
- [ ] **Step 4** Web gates green, then `make build`, `make guard-browser`,
      `make guard-a11y`, `make guard-layout`.
- [ ] **Step 5** Commit: `feat(web): draw a workspace as a grid`

**Satisfies:** AC-L-104, AC-L-105, AC-L-106.

---

### Task 2: a person changes the size and the order

**Files:**

- Modify: `web/src/entities/workspace/ui/PanelCardShell.tsx`,
  `web/src/pages/workspace/`, `web/src/shared/api/`, tests

**Consumes:** Tasks 0-1.

**Produces:** on each panel's card, a control for its width, one for its
height, and two that move it earlier and later — saved through the `PATCH`
the panel already has (L5).

Moving a panel swaps `position` with its neighbour: two `PATCH`es, not a
renumbering of the workspace (spec section 6). Moving the first panel earlier
does nothing and says nothing (AC-L-103) — there is no error to show for
something that cannot happen, so the control is simply absent at the ends
rather than present and inert. A disabled contained button has no edge
`make guard-layout` can see, which is the other reason.

- [ ] **Step 1** Write the test: changing a width `PATCH`es that panel and
      redraws it; moving a panel down `PATCH`es two panels and swaps them on
      screen; the first panel has no "earlier" control and the last none for
      "later" (AC-L-101, AC-L-102, AC-L-103). Run it, expect failure.
- [ ] **Step 2** Implement.
- [ ] **Step 3** Web gates green, then `make build`, `make guard-browser`,
      `make guard-a11y`, `make guard-layout` — every new control is measured
      by both, at 375px among others.
- [ ] **Step 4** Commit: `feat(web): size and order a workspace's panels`

**Satisfies:** AC-L-101, AC-L-102, AC-L-103.

---

### Task 3: end to end

**Files:**

- Create: `e2e/src/layout.test.ts`, `e2e/browser/layout.spec.ts`
- Modify: `STATE.md`, `TODO.md`, `DECISIONS.md`, and
  `harness/quality/browser/screens.ts` only if Step 3 decides it

**Consumes:** everything.

**Produces:** the journey — sign in, open a workspace with three panels,
make one wide, move it first, reload, see it wide and first.

- [ ] **Step 1** Write the process-level journey: create three panels, PATCH
      sizes and positions, read the workspace back, confirm the geometry
      survived.
- [ ] **Step 2** Write the browser journey, including a 375px pass
      (AC-L-105).
- [ ] **Step 3** Decide whether `harness/quality/browser/screens.ts` should
      seed a panel so the browser gates measure a workspace with something in
      it rather than an empty one (`DECISIONS.md`, 2026-09-13 records that it
      does not today). If yes, it is a harness change: explain it in
      `DECISIONS.md` and commit with `ORCHESTRA_ALLOW_HARNESS_CHANGE=1`. If
      no, say why in your report.
- [ ] **Step 4** `make check` in full — every gate green, and no request
      added to the model's log.
- [ ] **Step 5** Commit: `test(e2e): arrange a workspace and reload it`

**Satisfies:** the whole of `docs/specs/layout.md` section 8 end to end.

---

## Order and parallelism

Task 0 blocks everything. Task 1 needs it. Task 2 needs 0 and 1. Task 3
needs all of it. Nothing here runs in parallel, and the subproject is small
enough that it does not need to.

## Done

`make check` is green, every criterion in `docs/specs/layout.md` section 8
has a test that runs in CI, and a workspace looks the way somebody arranged
it rather than the order they happened to build it in.
