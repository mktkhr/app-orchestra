# Arranging a workspace — implementation plan

> **For the implementer:** one task at a time. Each task ends green and is
> committed on its own. Steps use `- [ ]` for tracking.

**Goal:** A person decides how wide and how tall each panel is, and what
order the panels come in, and it is still that way after a reload.

**Architecture:** A panel gains `width` and `height`, spans in
`react-grid-layout`'s grid, and `position` (which has existed unused since
`docs/specs/workspaces.md` W5) becomes the order. All three are saved through
the `PATCH` that already exists. `docs/specs/layout.md` section 4 records
that this was first planned without the library and reversed, and what the
reversal owes: **every arrangement a drag can express, a keyboard must
express too**, measured by `make guard-a11y`.

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

- Modify: `web/package.json` (`react-grid-layout` and its types),
  `web/src/pages/workspace/ui/WorkspacePage.tsx`,
  `web/src/entities/workspace/ui/PanelCardShell.tsx`, tests

**Consumes:** Task 0.

**Produces:** panels drawn in `react-grid-layout` — twelve columns where
there is room, one where there is not (L4) — each spanning its own `width`
and `height`, in `position` order. Read-only in this task: nothing is
dragged yet, and nothing is saved.

`react-grid-layout` ships its own CSS. Import it the way the bundler wants;
it is not a CDN request. **Look at what it paints** and confirm
`make guard-layout`'s contrast rules still hold under both colour schemes
(spec section 9) — the design system did not write that stylesheet.

On the narrow breakpoint the grid is static (spec section 5): one column, one
order, nothing to arrange, and no way to drag the page sideways by accident.

- [ ] **Step 1** Add the dependency. Pin it exactly, the way
      `@mui/x-charts` was and for the same reason (`DECISIONS.md`,
      2026-09-12). `make check` must still pass `guard-ui`, `guard-fsd` and
      `guard-duplication`.
- [ ] **Step 2** Write the test: three panels of widths 12, 6 and 6 draw one
      full-width row and two beside each other; at 375px all three span the
      single column; a panel with no width draws full width (AC-L-104,
      AC-L-105). Run it, expect failure.
- [ ] **Step 3** Implement, read-only.
- [ ] **Step 4** Check AC-L-106 by hand and say what you saw: a wider panel
      should draw a _wider chart_, because `ResultChart` takes its size from
      its caller since `docs/plans/dashboard.md` Task 5. If it draws the same
      chart in a wider box, that is this task's bug to fix.
- [ ] **Step 5** Web gates green, then `make build`, `make guard-browser`,
      `make guard-a11y`, `make guard-layout`.
- [ ] **Step 6** Commit: `feat(web): draw a workspace as a grid`

**Satisfies:** AC-L-104, AC-L-105, AC-L-106.

---

### Task 2: a person arranges it, with a mouse or without one

**Files:**

- Modify: `web/src/pages/workspace/`,
  `web/src/entities/workspace/ui/PanelCardShell.tsx`,
  `web/src/shared/api/`, tests
- Modify: `harness/quality/browser/screens.ts` (see Step 4)

**Consumes:** Tasks 0-1.

**Produces:** dragging and resizing, saved through the `PATCH` each panel
already has (L5) — **and a keyboard path to everything a drag can do**.

The keyboard half is not a nicety bolted on at the end; it is what
`docs/specs/layout.md` section 4 says this dependency owes, and AC-L-103 is
the criterion. `react-grid-layout`'s own handles are mouse-first, so decide
how a keyboard moves and resizes a panel and say why you chose that shape.
Whatever it is, `make guard-a11y` has to pass on a workspace **with panels in
it**, which means Step 4.

A drag that ends writes only the panels whose geometry changed — not a
renumbering of the workspace (spec section 6): a request that rewrites six
rows to change one can lose the other five. A drag that is still in flight
writes nothing.

- [ ] **Step 1** Write the test: a drag that ends `PATCH`es the panels that
      moved and no others; a resize `PATCH`es one panel; the arrangement
      survives a reload (AC-L-101, AC-L-102).
- [ ] **Step 2** Implement the pointer half.
- [ ] **Step 3** Implement the keyboard half, and test it by keyboard alone —
      no pointer events in that test at all.
- [ ] **Step 4** `harness/quality/browser/screens.ts` measures an **empty**
      workspace today, so neither browser gate has ever seen a panel. Seed
      one there so AC-L-103 is evidence rather than an assertion. It is a
      harness change: argue it in `DECISIONS.md` and commit with
      `ORCHESTRA_ALLOW_HARNESS_CHANGE=1`. Expect it to find things.
- [ ] **Step 5** Web gates green, then `make build`, `make guard-browser`,
      `make guard-a11y`, `make guard-layout`.
- [ ] **Step 6** Commit: `feat(web): arrange a workspace by drag or by key`

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
- [ ] **Step 3** Confirm Task 2 Step 4 actually landed: both browser gates
      measure a workspace with a panel in it, not an empty one.
- [ ] **Step 4** `make check` in full — every gate green, and no request
      added to the model's log.
- [ ] **Step 5** Commit: `test(e2e): arrange a workspace and reload it`

**Satisfies:** the whole of `docs/specs/layout.md` section 8 end to end.

---

## Order and parallelism

Task 0 blocks everything. Task 1 needs it. Task 2 needs 0 and 1. Task 3
needs all of it. Nothing here runs in parallel, and the subproject is small
enough that it does not need to.

Task 2 is the one that can run long. Its pointer half is what the library
does for you; its keyboard half is what the library does not, and it is the
half `docs/specs/layout.md` section 4 spent the dependency on.

## Done

`make check` is green, every criterion in `docs/specs/layout.md` section 8
has a test that runs in CI, and a workspace looks the way somebody arranged
it rather than the order they happened to build it in.
