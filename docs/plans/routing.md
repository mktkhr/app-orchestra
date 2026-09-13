# Addresses — implementation plan

> **For the implementer:** one task at a time. Each task ends green and is
> committed on its own. Steps use `- [ ]` for tracking.

**Goal:** A screen's address is a path a person can paste, and it still
works when they reload.

**Architecture:** The platform serves `index.html` for anything that is not
an API route and not a file it has; `react-router` reads the path;
`useHashRoute` goes, and the old hash addresses are not kept working.

**Spec:** `docs/specs/routing.md`. Acceptance criteria: its section 7.

## Global constraints

Everything in `docs/plans/layout.md`'s Global constraints section still
holds. The ones this subproject is most likely to meet:

- `make check` must never call a real LLM, and must need nothing running.
  Verify by counting
  `docker logs llama-swap 2>&1 | grep -c 'POST /v1/chat/completions'`
  before and after.
- **No suppressions of any kind** (AGENTS.md rule 2) — `//nolint`,
  `@ts-ignore`, `istanbul ignore`, `as any` and every relative. An
  unreachable branch gets deleted, not silenced.
- Layer order (`make guard-arch`) and FSD (`make guard-fsd`).
- 90% statement coverage per Go package; 1000 lines a file.
- `make guard-a11y` and `make guard-layout` measure every screen, including
  a workspace with a panel in it — and they navigate by hash today, so this
  subproject moves them (spec section 8, a protected path).
- Run `make -k check` while working; `make build` before
  `make guard-browser`.
- **`make dev-services` rebuilds and restarts the dummy services and the
  platform.** Use it before verifying live: a running process serves the
  contract it was built with, and a stale one produced three false bug
  reports in one day.

---

### Task 0: the server answers for an address it has never heard of

**Files:**

- Modify: `services/platform/internal/infra/httpserver/router.go`, tests

**Consumes:** nothing. Lands before any frontend change and breaks nothing:
the hash router keeps working throughout.

**Produces:** `index.html` for any path that is not `/api/...` and not a
file on disk; 404 for a missing file.

Spec section 4 is the rule and its argument. The distinction that matters:
a path with a file extension is a request for a file and answers 404 when
it is missing; everything else is the application. Handing HTML to a
browser that asked for a missing `.js` produces a syntax error in a file
that appears to exist.

- [x] **Step 1** Write the tests first: `/workspaces/abc` returns the
      index; `/assets/missing-abc123.js` returns 404; an existing asset is
      still itself; `/api/nope` answers as the API (404 from the API, not
      the index); `/` is still the index. Run them, expect failure.
- [x] **Step 2** Implement. `staticDir` empty must behave exactly as it
      does now — the acceptance suites and several tests start a platform
      with no frontend at all.
- [x] **Step 3** Go gates green.
- [x] **Step 4** Commit: `feat(platform): serve the application at any address`

**Satisfies:** AC-R-102, AC-R-103.

---

### Task 1: the browser reads the path

**Files:**

- Modify: `web/package.json` (`react-router`), `web/src/app/`,
  `web/src/app/model/useHashRoute.ts` (deleted), the drawer's links, tests

**Consumes:** Task 0.

**Produces:** `react-router` over the three addresses in spec section 3,
`useHashRoute` gone, and the drawer's `href`s as paths.

Pin the dependency exactly, the way `@mui/x-charts` and `react-grid-layout`
are and for the reason `DECISIONS.md` (2026-09-12) records.

`AuthGate` stays where it is (R5): it decides whether anybody sees a screen,
which is not a routing question.

The conversation's key (`docs/specs/context.md` M6) comes from the route
instead of the hash and is otherwise untouched — `"chat"`, and one per
workspace id.

- [x] **Step 1** Write the test: each address renders its screen; an
      unknown address renders the chat; the drawer's links are paths and
      following one changes the screen without a reload.
- [x] **Step 2** Implement, deleting `useHashRoute` rather than leaving it
      beside the router.
- [x] **Step 3** Web gates green, then `make build` and
      `make guard-browser`.
- [x] **Step 4** Commit: `feat(web): put the screen in the address`

**Satisfies:** AC-R-101 in the browser.

---

### Task 2: end to end, and the gates move

**Files:**

- Modify: `harness/quality/browser/screens.ts` (protected),
  `e2e/browser/*.spec.ts`, `STATE.md`, `TODO.md`, `DECISIONS.md`
- Create: `e2e/src/routing.test.ts`

**Consumes:** everything.

**Produces:** the browser gates and the acceptance suites navigating by
path, and the journey: sign in, open a workspace, reload, still there.

- [x] **Step 1** Move `harness/quality/browser/screens.ts` to paths. Done in
      Task 1: leaving it on the hash for one task meant shipping a redirect
      the spec says does not exist, and a temporary redirect is a permanent
      one with a comment on it.
- [x] **Step 2** `e2e/browser/*.spec.ts` navigate by clicking, not by
      address, so there was nothing to move.
- [ ] **Step 3** Write `e2e/src/routing.test.ts` against the built binary:
      the index for an unknown path, 404 for a missing asset, the API
      unshadowed (AC-R-102, AC-R-103).
- [ ] **Step 4** Write the browser journey: sign in, open a workspace by
      its path, reload, still there; and AC-R-104 — with no session, any
      address reaches the sign-in screen and lands where it was going after
      signing in.
- [ ] **Step 5** `make check` in full — every gate green, and no request
      added to the model's log.
- [ ] **Step 6** Commit: `test(e2e): reload an address and stay there`

**Satisfies:** AC-R-101, AC-R-102, AC-R-103, AC-R-104, and section 7 end
to end.

---

## Order and parallelism

Task 0 is the platform's and lands first, harmless on its own. Task 1 needs
it. Task 2 needs all of it, and is where the gates move — doing that
earlier would leave them pointing at addresses the product does not have
yet.

## Done

`make check` is green, every criterion in `docs/specs/routing.md` section 7
has a test that runs in CI, and a workspace's address can be pasted to
somebody else.
