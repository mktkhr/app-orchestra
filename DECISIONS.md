# DECISIONS.md — design decision log

_Append only. One entry per decision: context, decision, consequences. Newest last._

## 2026-01-16 Automatic Playwright browser installation via postinstall hook

**Context.** The `acceptance-browser` target in the Makefile runs Playwright tests
against a built product. These tests require Chromium to be installed in
`.cache/ms-playwright/`. However, this directory is `.gitignore`d and doesn't
persist between container restarts. When the harness runs `make check` in a fresh
environment, Chromium isn't installed, causing `acceptance-browser` to fail with
"Executable doesn't exist" errors.

**Decision.** Add a `postinstall` script to `package.json` that runs
`cd e2e && pnpm exec playwright install chromium`. This hook runs
automatically after `pnpm install`, ensuring browsers are installed as part of
the standard setup flow (`make setup` which runs `pnpm install --frozen-lockfile`).

**Consequences.** Browsers are now installed automatically during `pnpm install`,
eliminating the need for manual `make browsers` invocation before `make check`.
This is a product-side fix (package.json is not a protected path) rather than
changing harness behavior.

## 2026-09-07 Guards over rules

**Context.** The repository will be driven by an autonomous LLM for long stretches. Instructions decay; tools do not.
**Decision.** `AGENTS.md` holds five principles. Everything else (style, safety, architecture, suppression discipline) is a static check that fails the build. Warnings are not used anywhere: every enabled rule is an error.
**Consequences.** Adding a rule means adding a check under `harness/quality/` or `harness/guard/`, not a sentence in a Markdown file.

## 2026-09-07 Vite+ as the only frontend toolchain

**Context.** Vite+ (`vp`) bundles Vite, Vitest, Oxlint, Oxfmt and tsgolint with one config file.
**Decision.** No ESLint, no Prettier, no separate `tsc` step. `vp check` (format, type-aware lint, type check) is the frontend gate; policies live in `harness/quality/oxlint` and `harness/quality/oxfmt` and are imported by the root `vite.config.ts` (monorepo layout recommended by Vite+).
**Consequences.** `baseUrl` is not used in tsconfigs (tsgolint does not support it). Lint suppressions and disable comments are policed by `harness/guard/suppressions.sh` since Oxlint has no built-in registry.

## 2026-09-07 golangci-lint v2 with an explicit linter list

**Context.** Presets change between releases; an agent must not get a different policy after an upgrade.
**Decision.** `default: none` plus an explicit `enable` list; `nolintlint` requires a specific linter and an explanation; every `//nolint` must also be registered in `harness/quality/suppressions.allow`. Formatting is `gofmt` + `goimports` + `gci` through `golangci-lint fmt`.
**Consequences.** Upgrading golangci-lint is a deliberate harness change (`harness/quality/toolchain.mk`).

## 2026-09-07 Architecture is data, guards are code

**Context.** Layer rules written in prose are ignored under pressure.
**Decision.** `harness/quality/architecture.json` declares the backend layers (domain → usecase → adapter → infra → app → cmd) and the frontend FSD layers. `harness/guard/archcheck` (Go, over `go list`) and `harness/guard/fsd.ts` (oxc-parser) enforce them independently of the linters; `depguard` adds package-level denials inside golangci-lint.
**Consequences.** A new layer or slice is a JSON change plus, for the backend, possibly a `depguard` rule.

## 2026-09-07 Harness files are protected, not read-only

**Context.** A stuck agent's cheapest move is to relax a rule. OS-level read-only mounts are a later step.
**Decision.** `harness/quality/protected-paths.txt` lists the harness. `harness/guard/protected-paths.sh` fails the pre-commit hook and the pull-request CI job when any of them changes, unless the change is explicitly acknowledged (`ORCHESTRA_ALLOW_HARNESS_CHANGE=1` locally, the `harness` label on a PR).
**Consequences.** Product work and harness work are separate changes by construction. The directory split (`harness/quality/`, `harness/githooks/`, `harness/guard/`) makes a future read-only mount a one-liner.

## 2026-09-07 OpenAPI first

**Context.** The user wants every implementation to start from the API definition, with both the Go server and the TypeScript client generated.
**Decision.** `services/platform/api/openapi.yaml` is the single source of truth. Go: `oapi-codegen` (pinned via a `tool` directive in `harness/gen/go.mod`) generates models, a strict server interface and the embedded spec; requests are validated against the spec by `nethttp-middleware` before any handler runs. TypeScript: `openapi-typescript` generates the types, `openapi-fetch` provides the typed client. Generated files are committed and `make guard-generated` fails when they are stale; `redocly lint` gates the spec itself.
**Consequences.** Adding an endpoint without updating the spec is impossible: the Go build fails (interface not implemented) and the TypeScript build fails (path not in `paths`). The contract error envelope is the only error shape clients see, whether the rejection comes from the validator or the domain.

## 2026-09-07 Public composition root in the backend

**Context.** Acceptance tests live outside the backend module and cannot import `internal/`.
**Decision.** `services/platform/pkg/app` is the only public package; it wires the graph and returns an `http.Handler`. `cmd/api` only reads the environment and runs it.
**Consequences.** Acceptance tests and the binary run the identical object graph.

## 2026-09-10 Harness imported from takamai, product code removed

**Context.** app-orchestra needs the repository harness that takamai carries:
static guards, protected harness paths, a one-line-on-success output contract,
and OpenAPI-first code generation. Every version of that harness, from its
first commit onwards, shipped together with a sample product - a greeting API,
and later a household expense tracker. There is no commit in which the harness
stands alone.

**Decision.** Copy the harness at takamai's HEAD verbatim, rename the
identifiers (`github.com/mktkhr/takamai` to `github.com/mktkhr/app-orchestra`,
`@takamai/` to `@app-orchestra/`, `TAKAMAI_` to `ORCHESTRA_`), then delete
every line of product code. The references to the observed failures that
justify `harness/quality/file-length.txt` and `harness/githooks/commit-msg` were generalised
rather than renamed: those failures happened in another repository, and
renaming them would claim they happened here.

**Consequences.** `make check` fails until a product exists - the guards have
nothing to measure and the acceptance suites are empty. The policy in
`harness/quality/` is unchanged and applies in full to the first product code written
here, including the 90% coverage floor. Note that takamai's HEAD does not
itself pass `make check`: `dupl` rejects its handler layer, where 43 of 59
handlers repeat the same authorisation block. The harness was never inherited
in a green state.

## 2026-09-10 .gitignore denies by default

**Context.** The conventional `.gitignore` tracks everything and lists the junk.
That fails in one direction only: whatever nobody thought to list gets committed,
silently. Build output, caches, downloaded browsers, an agent's scratch note and
a stray secret all arrive the same way, and an agent adds new kinds of file
faster than anyone updates the list.

**Decision.** Deny everything (`*`), re-enter directories so the rules below can
match (`!*/`), exclude the directories that must never be indexed, then allow
tracked file types by extension plus the handful of extensionless files by name.

**Consequences.** The failure mode moves to the other side: something that should
be tracked silently is not - which `git status` shows and a build catches.
Adding a file type is a one-line change. Verified against the 76 files the
repository holds: all 76 pass, while `node_modules`, build output, scratch
directories, `.env.local` and editor state are refused.

## 2026-09-10 A Claude Code layer that nothing depends on

**Context.** The harness enforces itself through git hooks, `make check` and CI,
which is what lets any agent runtime drive it. But a commit is dozens of edits
away from the edit that broke something, and an agent that finds out at commit
time has already built on top of the mistake.

**Decision.** Add a Claude Code layer that reports the same failures sooner and
does nothing else. `.claude/settings.json` allows the command surface `AGENTS.md`
names (`make`, the read-only `go` and `git` subcommands, file reading) without a
prompt, and denies `git commit --no-verify` and `git push --no-verify` outright.
`harness/claude/post-edit.sh` runs after each Edit or Write, checks only the file
that changed, and stays silent unless it is unformatted or part of the harness.
Unlike `harness/quiet.sh` it prints nothing on success: a confirmation after
every edit would fill the model's context with noise, which is the opposite of
the output contract's purpose.

**Consequences.** Deleting the entire layer changes nothing about what can be
committed - `harness/githooks/pre-commit`, `make check` and CI make every one of these
checks again. The layer is itself a protected path, so an agent cannot switch off
the thing that reports on it. `CLAUDE.md` imports `AGENTS.md` with `@AGENTS.md`
rather than restating it, so the two cannot drift.

## 2026-09-10 Repository layout: harness, services, web, e2e

**Context.** The imported layout assumed one backend and one contract:
`api/openapi.yaml` sat at the root, `acceptance/` was a sibling of the thing it
tested, and `quality/`, `scripts/` and `hooks/` were three more root entries
beside the product. app-orchestra has a platform service plus several
microservices, each with its own contract, so a single root `api/` cannot
express it and `acceptance/backend` stops naming anything in particular.

**Decision.** Four top-level homes. `harness/` holds every part that decides
what code is acceptable - policy, guards, git hooks, the Claude hook, the output
contract, the pinned generator. `services/<name>/` is one directory per backend,
each its own Go module, holding its own `api/` contract and its own
`acceptance/` module beside the code. `web/` is the frontend. `e2e/` is the only
acceptance suite that stands alone, because it is the only one that belongs to
no single service.

**Consequences.** Adding a service is adding a directory. `protected-paths.txt`
covers the whole harness in one line instead of four. Two things had to follow:
`services/platform/acceptance` is nested inside `services/platform`, so the
pre-commit hook now claims staged files deepest-module-first instead of relying
on the two being siblings; and `services/*/api/` is deliberately _not_ a
protected path, because a generator config per service would mean amending the
protected list every time a service is added.

## 2026-09-10 The NO_COLOR machinery is gone

**Context.** Three separate mechanisms suppressed ANSI colour: a committed
`.env` holding `NO_COLOR=1`, `.npmrc` setting `env.NO_COLOR=1`, and a
`postinstall` script that rewrote `node_modules/.bin/vp` with `sed` to inject an
`export`. All three existed so that a verifier at `/harness/bin/ac` could grep
vitest output for `Tests N passed`.

**Decision.** Remove all three. That verifier is not part of this repository and
never was; `harness/quiet.sh` already strips ANSI escapes from every command it
wraps, which is every command the Makefile, the hooks and CI run.

**Consequences.** One less committed `.env` - a file whose name invites secrets
and which only held a display setting - one less root config, and `pnpm install`
no longer edits a file inside `node_modules`. The `postinstall` script stays,
reduced to installing the pinned Chromium.

## 2026-09-10 Every target discovers its services

**Context.** Moving to `services/<name>/` shaped the tree for several backends,
but nothing else had followed: the `Makefile` listed one module, `archcheck`
read a single `backend` object from the policy, `coverage.sh` did `cd
services/platform`, the pre-commit hook held a hand-sorted module list, and CI
named `services/platform/go.mod`. A second service would have been invisible to
all of them - the tree would have said "several services" while the harness
still checked one.

**Decision.** A service is a directory under `services/` with a `go.mod`, and
that is the only declaration. `make` derives the module list, the specs and the
generated-file list from `$(wildcard services/*/go.mod)`; `archcheck` walks the
same directories and applies one layer declaration to each; `coverage.sh` and
the pre-commit hook discover modules the same way; CI reads `go.work` for the Go
version and `**/go.sum` for the cache. The generator configuration moved back to
`harness/gen/oapi-codegen.yaml` as a single shared file, with `make` passing the
spec and the output path per service, and `redocly.yaml` lost its `apis` block
for the same reason. The `backend-*` and `frontend-*` targets became
`services-*` and `web-*`, since they no longer describe one of each.

**Consequences.** Adding a service is adding a directory; no list anywhere needs
editing. This was verified rather than assumed: a second service was added
temporarily, and each guard was made to report on it - `archcheck` rejected a
`usecase` package importing `adapter`, `guard-coverage` failed its untested
packages, `guard-suppressions` caught an unregistered `//nolint`, `make
generate` produced its Go and TypeScript output, and the pre-commit hook ran
`go vet` against it under its own module. The service was then removed.

Two smaller things fell out. `go test ./...` and `go vet ./...` exit non-zero on
a module with no packages, which is not a finding, so every loop skips a module
whose `go list` is empty. And the lint policy rejected `checkServices` twice -
`gocritic` wanted the three results named, `nonamedreturns` forbade naming them -
which is a fair push towards a struct that says what the pair means.

## 2026-09-10 Toolchain upgrade: Go 1.27 and pnpm 12, TypeScript held at 6

**Context.** The harness arrived pinning Go 1.26, pnpm 10.33.2 and TypeScript
6.0.3, all behind current. The upgrade was deliberately deferred until the layout
and multi-service work was finished, so that anything breaking afterwards would
be attributable to the versions rather than to the move.

**Decision.** Go 1.27.1 and pnpm 12.3.4, and the catalog moved to current
versions (Vite+ 0.3.1, React 19.3.0, `@types/react` 19.3.0, oxc-parser 0.149.0,
happy-dom 20.14.3, `@vitejs/plugin-react` 6.1.1, `@testing-library/user-event`
14.6.7, `@types/node` 24.13.4). TypeScript stays at 6.0.3.

**Consequences.** Two findings, one of which changed the harness.

golangci-lint links the standard library of the Go that built it, so the binary
built under Go 1.26 reported phantom type errors inside Go 1.27's own sources
(`unknown field rfd in struct literal of type splicePipe`). The fix was not a
version bump - v2.13.2 is current - but a rebuild under the new Go. The Makefile
now makes `.tools/bin/golangci-lint` depend on `harness/quality/toolchain.mk` and
passes `GOTOOLCHAIN=go$(GO_VERSION)`, so changing the pinned Go rebuilds the
linter with it. `GO_VERSION` gained its patch level to make that possible.

TypeScript 7 cannot be adopted yet, and the reason is not TypeScript. It is a
native reimplementation that does not ship the Compiler API, and
`openapi-typescript@7.13.0` builds its output with `ts.factory`, so `make
generate` dies on `Cannot read properties of undefined (reading
'createKeywordTypeNode')`. Everything else passed under TypeScript 7 - the
type-aware Oxlint through tsgolint included - and only code generation broke. No
released version of openapi-typescript supports it. The pin carries the reason so
that the next person does not have to rediscover it.

pnpm 12 kept `lockfileVersion: '9.0'`, so the lockfile did not change shape.

## 2026-09-11 openapi-typescript stays; orval measured and rejected

**Context.** TypeScript 7 is blocked by `openapi-typescript@7.13.0`, which builds
its output with the TypeScript Compiler API that TypeScript 7's native
implementation does not ship. orval was raised as a replacement.

**Decision.** Keep `openapi-typescript`. orval does work under TypeScript 7 -
measured rather than assumed: it depends on no `typescript` package at all,
generating code as text through acorn and esbuild, and its output type checks
clean under `strict` with `noUncheckedIndexedAccess` and
`exactOptionalPropertyTypes`. It was rejected on what it emits, not on whether it
runs.

**Consequences.** Four things decided it, the first on its own:

- `openapi-typescript` emits a `paths` map type - path string to method to
  operation - which lets `openapi-fetch` be called with the path as a _variable_
  and still be type checked. orval emits named functions (`listItems`,
  `createItem`), so choosing an endpoint at runtime needs a hand-written map of
  operation id to function. This product exists to let an LLM pick the endpoint
  at runtime, so that map is the load-bearing part, and orval does not generate
  it.
- orval emits runtime code, not just types: 129 lines against 95 for two
  endpoints, and the gap grows per endpoint because each one gets a function and
  a URL builder. The first vertical slice is sized at 100-200 endpoints.
- `openapi-fetch` - a generic client of about 1.5 kB with no generated code -
  would be dropped.
- The generated-output exclusions in `oxfmt`, `oxlint` and `guard-filelen` assume
  `**/src/shared/api/gen/**` and `schema.d.ts`; orval's multi-file modes would
  need them reworked. Its output is also poorly formatted (`;;` appears in it).

Worth recording for the vertical-slice design rather than for this decision:
`orval --client mcp` generates a complete Model Context Protocol server from a
spec, registering every operation as a tool and exposing them as
`Record<string, RegisteredTool>` - which answers the first objection by turning
the endpoint list into a runtime-addressable map. Adopting it would change the
shape of the catalogue and the orchestration decided in `PRODUCT.md` (D2), so it
belongs to that design, not to a generator swap.

## 2026-09-11 Two guards the orchestration design asked for

**Context.** Designing the first vertical slice turned up two things the harness
could not see. `oapi-codegen` v2.8.0 accepts a valid OpenAPI 3.2 document
containing a `query` operation and generates nothing for it - no error, no
warning, an empty `ServerInterface` - and `guard-generated` only compares the
generated files against a fresh run, so a silently dropped operation passes it.
Separately, the whole enum-label mechanism the planner depends on rests on spec
authors remembering to write `x-enum-labels`, which is exactly the kind of rule
that decays when it lives in prose.

**Decision.** Two static checks. `harness/guard/generated-ops.sh` extracts every
operationId a spec declares and fails when it is absent from the Go or the
TypeScript output, per service, discovered the same way every other target
discovers them. A Redocly plugin at `harness/quality/redocly/enum-labels.js`
fails a spec whose `enum` carries no `x-enum-labels`, or whose labels do not
cover exactly the enum's values.

**Consequences.** Both were verified by making them fail on purpose: a probe
service with a 3.2 `query` operation reproduced the silent drop and was
reported, and three spec variants confirmed the label rule accepts a complete
mapping and rejects both a missing one and a mismatched one.

One thing had to give. Redocly loads plugins as JavaScript, and `.gitignore`
denies by default with no `.js` among the tracked extensions, so the plugin was
invisible to git and `guard-ignored` failed on it. Rather than tracking `.js`
wholesale - the rest of the workspace is TypeScript, and a blanket allow would
let build output in - the one file is named individually in the `.gitignore`
section meant for exactly that.
