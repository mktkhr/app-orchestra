# Authentication and authorisation — implementation plan

> **For the implementer:** one task at a time. Each task ends green and is
> committed on its own. Steps use `- [ ]` for tracking.

**Goal:** A person signs in, and from then on the platform offers them only
what they may call — to the model, and at the mouth that runs it.

**Architecture:** Identity behind an `Authenticator` port with a local
implementation over the platform's own SQLite file. Permission is a row per
operation. The catalogue is filtered once per request and everything downstream
reads the filtered one, so the tool list and `/api/invoke` cannot disagree.

**Tech stack:** as before, plus `golang.org/x/crypto` (argon2id).

**Spec:** `docs/specs/auth.md`. Acceptance criteria: its section 9.

## Global constraints

Everything in `docs/plans/workspaces.md`'s Global constraints section still
holds, unchanged — including everything it carried forward from the first
slice. The ones this subproject is most likely to meet:

- `make check` must never call a real LLM, and must need nothing running.
  Verify by counting: `docker logs llama-swap 2>&1 | grep -c 'POST /v1/chat/completions'`
  before and after.
- Contract first: change `services/platform/api/openapi.yaml`, run
  `make generate`. Never edit generated files.
- Layer order (`make guard-arch`): domain ← usecase ← adapter ← infra ← app ←
  cmd. Feature-Sliced Design for the web (`make guard-fsd`): app → pages →
  widgets → features → entities → shared. A feature may not import a sibling
  feature; composing two of them is what `widgets` is for.
- 1000 lines a file; 90% statement coverage per Go package.
- Controls come from MUI. A disabled contained button has no edge the layout
  guard can see. `TextField size="small"` is 40px, under the 44px floor.
- MUI 9 takes `alignItems`/`justifyContent` through `sx`, not as props.
- Run `make -k check` while working; `make check` stops at the first failure and
  the browser guards are last. `make build` before `make guard-browser`.
- Verify in a browser at the Tailscale address, not at `localhost`.
- **Run `make check` again after the last edit, before reporting.** Green in the
  middle is not evidence.

## File structure

```
services/platform/
  api/openapi.yaml                    + /api/session, /api/users
  internal/domain/user.go             User, Role, Permission. Pure.
  internal/usecase/auth.go            Authenticator, SessionStore, PermissionStore
  internal/adapter/auth/local/        argon2id over the users table
  internal/adapter/repository/sqlite/ + users, sessions, permissions
  internal/adapter/handler/session.go, users.go
  internal/infra/httpserver/          the middleware that resolves a session
web/src/
  features/session/                   sign in, sign out, who is signed in
  features/permissions/               the admin's grid
  pages/users/                        the admin screen
e2e/                                  + a signed-in journey
```

---

### Task 0: accounts, sessions and permissions exist

**Files:**

- Create: `services/platform/internal/domain/user.go` and its test
- Create: `services/platform/internal/usecase/auth.go` and its test
- Create: `services/platform/internal/adapter/auth/local/` and its tests
- Modify: `services/platform/internal/adapter/repository/sqlite/`,
  `internal/infra/config/config.go`, `pkg/app/app.go`

**Produces:**

```go
// domain
type Role string // "admin" | "user"
type User struct { ID, Name string; Role Role }
type Permission struct { Service, OperationID string }

// usecase
type Authenticator interface {
    Authenticate(ctx context.Context, name, password string) (domain.User, bool, error)
}
type SessionStore interface {
    Create(ctx context.Context, userID string) (token string, err error)
    User(ctx context.Context, token string) (domain.User, bool, error)
    Delete(ctx context.Context, token string) error
}
type PermissionStore interface {
    For(ctx context.Context, userID string) ([]domain.Permission, error)
    Set(ctx context.Context, userID string, permissions []domain.Permission) error
}
```

No HTTP in this task.

- [x] **Step 1** Write the store tests first: a seeded admin authenticates with
      the right password and not the wrong one; a session round-trips and then
      does not after deletion; permissions replace wholesale. Run them, expect
      failure.
- [x] **Step 2** Add the three tables to the embedded schema. Add
      `golang.org/x/crypto` and hash with argon2id. Implement until green.
- [x] **Step 3** Seed the first admin from `ORCHESTRA_ADMIN_PASSWORD` when the
      database is created. An unset or empty one is an error, like the database
      path: a default password is a way of having none while appearing to.
- [x] **Step 4** `make fmt-check services-lint services-test guard-arch
guard-coverage` green.
- [x] **Step 5** Commit: `feat(platform): keep accounts, sessions and permissions`

---

### Task 1: the catalogue narrows to a person

**Files:**

- Modify: `services/platform/internal/domain/catalog.go`,
  `internal/usecase/orchestrator.go`, `internal/usecase/workspaces.go`, tests

**Consumes:** Task 0.

**Produces:** `func (c Catalog) For(permissions []Permission) Catalog`, and a
`*domain.User` argument on `Orchestrator.Plan`, `Orchestrator.Invoke` and every
`Workspaces` method. `stubOwner` goes.

An admin's catalogue is the whole one; a user's is what their rows name.

- [x] **Step 1** Write the test: a catalogue of two services and a permission
      naming one operation yields a catalogue holding only it; `ToolsFor` over it
      names nothing else; `Find` misses the rest. Run it, expect failure.
- [x] **Step 2** Implement `For`. Thread the user through the two usecases.
      Delete the `TODO(auth)` comment — the seat is filled.
- [x] **Step 3** Add the test that `Invoke` refuses an operation the person may
      not call with the same error an unknown one gets, and calls nothing.
- [x] **Step 4** Go gates green.
- [x] **Step 5** Commit: `feat(platform): offer only what the person may call`

**Satisfies:** AC-A-103, AC-A-104 at the usecase level.

---

### Task 2: signing in

**Files:**

- Modify: `services/platform/api/openapi.yaml`
- Create: `internal/adapter/handler/session.go`,
  `internal/infra/httpserver/session.go` (the middleware), tests
- Create: `services/platform/acceptance/session_test.go`

**Consumes:** Tasks 0-1.

**Produces:** `POST`/`DELETE`/`GET /api/session`, and a middleware that resolves
the cookie into a user for every other route. `GET /api/health` stays open.

- [x] **Step 1** Add the paths. `make api-lint`, then `make generate`.
- [x] **Step 2** Write the acceptance test: signing in sets a cookie and
      returns the user; a wrong password is 401 with no cookie; every other
      endpoint is 401 without one; signing out ends it. Run it, expect failure.
- [x] **Step 3** Implement. The cookie is HttpOnly and SameSite; the token is
      random and opaque.
- [x] **Step 4** `make -k check` — nothing failing but what the frontend has not
      caught up with.
- [x] **Step 5** Commit: `feat(platform): sign a person in`

**Satisfies:** AC-A-101, AC-A-102, AC-A-107 at the platform level.

---

### Task 3: the admin's endpoints

**Files:**

- Modify: `services/platform/api/openapi.yaml`
- Create: `internal/adapter/handler/users.go`, tests, acceptance

**Consumes:** Task 2.

**Produces:** `GET /api/users`, `GET`/`PUT /api/users/{id}/permissions`, refused
with 403 to anybody who is not an admin.

- [x] **Step 1** Add the paths. `make api-lint`, `make generate`.
- [x] **Step 2** Write the acceptance test: an admin reads the accounts and sets
      another person's permissions; that person's next question is answered from
      the service they were just granted; a non-admin gets 403 from both. Run it,
      expect failure.
- [x] **Step 3** Implement.
- [x] **Step 4** Go gates green.
- [x] **Step 5** Commit: `feat(platform): let an admin set what a person may call`

**Satisfies:** AC-A-106.

---

### Task 4: the sign-in screen

**Files:**

- Create: `web/src/features/session/`, `web/src/pages/signin/`
- Modify: `web/src/app/`, `web/src/shared/api/client.ts`

**Consumes:** Task 2.

**Produces:** a sign-in screen when nobody is signed in, the person's name in
the bar, and a control to sign out. A 401 from anywhere returns to the screen.

- [x] **Step 1** Write the test: with a scripted API answering 401, the shell
      shows the sign-in screen; signing in shows the chat. Run it, expect failure.
- [x] **Step 2** Implement. The session is read once at startup and after a
      sign-in; nothing polls.
- [x] **Step 3** Web gates green, then `make build` and `make guard-browser`.
- [x] **Step 4** Commit: `feat(web): sign in before anything else`

**Satisfies:** AC-A-107 end to end.

---

### Task 5: the admin's screen

**Files:**

- Create: `web/src/features/permissions/`, `web/src/pages/users/`
- Modify: the drawer

**Consumes:** Tasks 3-4.

**Produces:** a users entry in the drawer for an admin, a list of accounts, and
a per-account grid of operations grouped by service, with a control that grants
or revokes a whole service at once.

The entry is hidden from a non-admin, and the endpoints refuse them anyway.

- [x] **Step 1** Write the test: an admin sees the entry and a non-admin does
      not; checking a service's box grants every operation it holds; saving puts
      them. Run it, expect failure.
- [x] **Step 2** Implement.
- [x] **Step 3** Web gates green, then `make build` and `make guard-browser`.
- [x] **Step 4** Commit: `feat(web): grant and revoke what a person may call`

**Satisfies:** AC-A-106 end to end.

---

### Task 6: end to end

**Files:**

- Create: `e2e/src/auth.test.ts`, `e2e/browser/auth.spec.ts`
- Modify: the existing suites, which now have to sign in first

**Consumes:** everything.

**Produces:** the journey: sign in as an admin, grant a person one service, sign
in as them, ask a question, see it answered from that service and from no other.
And a workspace one person makes that another cannot see.

- [x] **Step 1** Make the existing e2e suites sign in. They will be failing on
      401 by now; this is the task that fixes them.
- [x] **Step 2** Write the process-level journey. Run it, expect failure, make it
      pass.
- [x] **Step 3** Write the browser journey.
- [x] **Step 4** `make check` in full — every gate green, and the model's log no
      longer than it was.
- [ ] **Step 5** Commit: `test(e2e): answer only what the person may ask`

**Satisfies:** AC-A-105, and the whole of section 9 end to end.

---

## Order and parallelism

Task 0 blocks everything. Task 1 needs only Task 0 and touches no HTTP, so it
can run beside Task 2. Task 3 needs Task 2. Task 4 needs Task 2; Task 5 needs
both 3 and 4. Task 6 needs all of it, and is where the existing suites are
repaired.

## Done

`make check` is green, every criterion in `docs/specs/auth.md` section 9 has a
test that runs in CI, and a person signed in as themselves cannot reach a
service nobody granted them — through the model or around it.
