# Workspaces — implementation plan

> **For the implementer:** one task at a time. Each task ends green and is
> committed on its own. Steps use `- [ ]` for tracking.

**Goal:** An answer worth keeping can be kept. A result in the chat becomes a
panel on a workspace, and opening that workspace asks the same questions again.

**Architecture:** A panel is a saved call — service, operation id, arguments,
component. The platform stores it in SQLite and hands it back; the browser runs
each panel through the `POST /api/invoke` that already exists and draws the
answer with the components the chat already draws. The planner, the catalogue
and the rendering rule are not touched.

**Tech stack:** as the first slice, plus `modernc.org/sqlite` (pure Go, no cgo).

**Spec:** `docs/specs/workspaces.md`. Acceptance criteria: its section 10.

## Global constraints

Everything in `docs/plans/orchestration.md`'s own Global constraints section
still holds, unchanged. In particular:

- `make check` must never call a real LLM, and must need nothing running.
- Contract first: change `services/platform/api/openapi.yaml`, run
  `make generate`, implement against the generated code. Never edit generated
  files.
- Layer order (`make guard-arch`): domain ← usecase ← adapter ← infra ← app ←
  cmd.
- 1000 lines a file; 90% statement coverage per Go package.
- Controls come from MUI (`make guard-ui`). No `any`, no `console`, no default
  exports, no `fetch` outside `web/src/shared`.
- Feature-Sliced Design (`make guard-fsd`): app → pages → features → entities →
  shared. A feature may not import a sibling feature.
- Every enum in a spec carries `x-enum-labels`.
- Do not touch `harness/`, `.github/`, `Makefile`, `vite.config.ts` or the other
  harness files.

And these, learned since that plan was written:

- Run `make -k check`, not `make check`, while work is in progress: `make` stops
  at the first failure and the browser guards sit at the end.
- `make guard-browser` measures the built product. Run `make build` first.
- A disabled MUI contained button has no edge the layout guard can see. Do not
  disable a control because a field is empty.
- MUI 9 takes `alignItems` and `justifyContent` through `sx`, not as props.
- `crypto.randomUUID` exists only in a secure context. Use
  `web/src/shared/lib/turnId.ts`.
- Verify in a browser at the Tailscale address, not at `localhost`: `localhost`
  is a secure context and hides what the person actually sees.
- Every property an exposed operation draws needs a `title`
  (`make guard-exposed-ops`).

## File structure

```
services/platform/
  api/openapi.yaml                    + /api/workspaces, /api/workspaces/{id}/panels
  internal/domain/workspace.go        Workspace, Panel. Pure.
  internal/usecase/workspaces.go      WorkspaceStore port + the usecase over it
  internal/adapter/repository/sqlite/ the store, and the embedded schema
  internal/adapter/handler/workspace.go
  internal/infra/config/              + ORCHESTRA_DB_PATH
web/src/
  entities/workspace/                 Panel card, the panel list, the API shapes
  features/workspaces/                list, create, delete, save-a-result
  pages/workspace/                    the screen
e2e/                                  + a workspace journey
```

---

### Task 0: workspaces have somewhere to live

**Files:**

- Create: `services/platform/internal/domain/workspace.go` and its test
- Create: `services/platform/internal/usecase/workspaces.go` and its test
- Create: `services/platform/internal/adapter/repository/sqlite/{store,schema}.go`
  and tests
- Modify: `services/platform/internal/infra/config/config.go`, `pkg/app/app.go`

**Produces:**

```go
// domain
type Workspace struct { ID, Name, Owner string; Panels []Panel }
type Panel struct {
    ID, WorkspaceID, Service, OperationID, Component, Title string
    Args     map[string]any
    Position int
}

// usecase
type WorkspaceStore interface {
    List(ctx context.Context, owner string) ([]Workspace, error)
    Get(ctx context.Context, id string) (Workspace, bool, error)
    Create(ctx context.Context, owner, name string) (Workspace, error)
    Delete(ctx context.Context, id string) error
    AddPanel(ctx context.Context, workspaceID string, p Panel) (Panel, error)
    DeletePanel(ctx context.Context, workspaceID, panelID string) error
}
```

No HTTP in this task. The store is exercised directly against a file in
`t.TempDir()`.

- [x] **Step 1** Write the store's test first: create a workspace, add two
      panels, read them back in position order, delete one, delete the workspace.
      Assert a second store opened on the same file sees the same rows. Run it,
      expect failure.
- [x] **Step 2** Add `modernc.org/sqlite` to `services/platform/go.mod`. Write
      the schema as an embedded statement applied at open. Implement the store
      until the test passes.
- [x] **Step 3** Add `ORCHESTRA_DB_PATH` to config, with a test. Wire the store
      into `pkg/app`. An unset path is an error, not a default: a platform that
      silently forgets is worse than one that will not start.
- [x] **Step 4** `make fmt-check services-lint services-test guard-arch
guard-coverage` green.
- [x] **Step 5** Commit: `feat(platform): keep workspaces in a file`

**Satisfies:** the storage half of AC-W-105.

---

### Task 1: the workspace endpoints

**Files:**

- Modify: `services/platform/api/openapi.yaml`
- Create: `services/platform/internal/adapter/handler/workspace.go` and tests
- Create: `services/platform/acceptance/workspace_test.go`

**Consumes:** Task 0.

**Produces:** the contract in `docs/specs/workspaces.md` section 5.

`args` is a free-form object, like `PlanResult.data`. `component` reuses the
existing `Component` enum — it already carries `x-enum-labels`.

- [x] **Step 1** Add the paths and schemas. `make api-lint`, then
      `make generate`.
- [x] **Step 2** Write the acceptance test: create a workspace, add a panel,
      read it back, delete it. Assert a panel for an unknown workspace is a 404 and
      that a panel naming an operation the catalogue does not expose is a 400 — the
      same rule `/api/invoke` follows, for the same reason. Run it, expect failure.
- [x] **Step 3** Implement the handlers until it passes.
- [x] **Step 4** `make -k check` — nothing failing but what the frontend has
      not caught up with.
- [x] **Step 5** Commit: `feat(platform): serve workspaces and their panels`

---

### Task 2: the drawer lists workspaces

**Files:**

- Create: `web/src/features/workspaces/` (list, create, delete), its tests
- Modify: `web/src/app/ui/NavigationDrawer.tsx`, the router

**Consumes:** Task 1, and the generated TypeScript types.

**Produces:** workspaces under チャット in the drawer; a control to make one and
to remove one.

- [x] **Step 1** Write the test: with a scripted API returning two workspaces,
      both names appear in the drawer; creating one posts and appends it. Run it,
      expect failure.
- [x] **Step 2** Implement. Deleting asks first — it takes the panels with it.
- [x] **Step 3** `make web-lint web-test guard-fsd guard-ui guard-duplication`
      green.
- [x] **Step 4** Commit: `feat(web): list workspaces in the drawer`

---

### Task 3: a workspace draws its panels

**Files:**

- Create: `web/src/entities/workspace/` (the panel card), `web/src/pages/workspace/`
- Modify: `web/src/shared/api/client.ts`

**Consumes:** Task 2.

**Produces:** opening a workspace reads its panels, posts each to `/api/invoke`,
and draws each answer with the component the panel names — the same
`ResultTable` and `ResultDetail` the chat uses, in a card with the panel's title
and its provenance.

A panel that fails says so inside its own card. The others still draw.

- [x] **Step 1** Write the test: a workspace of two panels, one whose invoke
      resolves and one whose invoke rejects. Assert the first draws a table and the
      second shows an error, in its own card. Run it, expect failure.
- [x] **Step 2** Implement. The panels load independently; one slow service does
      not hold the others.
- [x] **Step 3** `make web-lint web-test guard-fsd guard-ui guard-duplication`
      green, then `make build` and `make guard-browser`.
- [x] **Step 4** Commit: `feat(web): draw a workspace's panels`

**Satisfies:** AC-W-102, AC-W-106.

---

### Task 4: a result can be kept

**Files:**

- Modify: `web/src/features/conversation/ui/TurnList.tsx`,
  `web/src/entities/rendering/`
- Create: the save control and its test

**Consumes:** Task 3.

**Produces:** a result turn carries a control that saves it as a panel. It posts
the `source` it is already showing — service, operation id, arguments — plus the
component it was drawn with and a title the person can edit, defaulting to the
question that produced it.

- [x] **Step 1** Write the test: a table result offers the control; using it
      posts a panel whose service, operation id and args match the result's
      provenance. Run it, expect failure.
- [x] **Step 2** Implement. Which workspace to save to is chosen from the ones
      that exist; if there are none, the control offers to make one.
- [x] **Step 3** Web gates green.
- [x] **Step 4** Commit: `feat(web): save a result to a workspace`

**Satisfies:** AC-W-101.

---

### Task 5: a panel can be run again

**Files:**

- Modify: `web/src/entities/workspace/`, plus tests

**Consumes:** Task 3.

**Produces:** a refresh control on each panel card that posts the panel to
`/api/invoke` again and replaces what it draws.

- [x] **Step 1** Write the test: a panel whose second invoke returns a row the
      first did not; refreshing shows the new row. Run it, expect failure.
- [x] **Step 2** Implement. The control does not disable itself while in flight
      — it shows that it is working (`make guard-layout`).
- [x] **Step 3** Web gates green.
- [x] **Step 4** Commit: `feat(web): run a panel again`

**Satisfies:** AC-W-103.

---

### Task 6: asking from the workspace

**Files:**

- Modify: `web/src/pages/workspace/`
- Reuse: `web/src/features/conversation/`

**Consumes:** Tasks 3-4.

**Produces:** the workspace screen carries the same conversation the chat does.
A question asked there is answered there, and the answer offers the same save
control, defaulting to the workspace it was asked from.

Nothing about the planner changes. It is the chat, on another screen.

- [x] **Step 1** Write the test: asking on a workspace screen appends a turn,
      and the result's save control defaults to that workspace. Run it, expect
      failure.
- [x] **Step 2** Implement. If the conversation feature needs a prop to know
      which workspace it sits on, that is the whole change.
- [x] **Step 3** Web gates green, then `make build` and `make guard-browser`.
- [x] **Step 4** Commit: `feat(web): ask a question from a workspace`

**Satisfies:** AC-W-104.

---

### Task 7: end to end

**Files:**

- Modify: `e2e/src/orchestration.test.ts` or a sibling, `e2e/browser/`

**Consumes:** everything.

**Produces:** the journey, against the built product: ask, save, reopen, refresh.
And a restart: the platform is stopped and started on the same database file,
and the workspace is still there.

- [ ] **Step 1** Write the process-level test: create a workspace and a panel
      over HTTP, stop the platform, start it again on the same file, read the
      workspace back. Run it, expect failure.
- [ ] **Step 2** Make it pass. The e2e database is a file in a temporary
      directory, so the suites cannot see each other's workspaces.
- [ ] **Step 3** Write the browser journey: ask a question, save the result,
      open the workspace, see the panel.
- [ ] **Step 4** `make check` in full — every gate green.
- [ ] **Step 5** Commit: `test(e2e): keep an answer and find it again`

**Satisfies:** AC-W-105, and the whole of section 10 end to end.

---

## Order and parallelism

Task 0 blocks everything. Task 1 blocks all the web work. Tasks 2 and 3 are a
chain. Task 4 needs 3 only for somewhere to save to, and Task 5 needs 3 for a
card to put the control on; they are independent of each other. Task 6 needs 4.
Task 7 needs all of it.

## Done

`make check` is green, every criterion in `docs/specs/workspaces.md` section 10
has a test that runs in CI, and a workspace assembled before a restart is still
assembled after one.
