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

The lint policy needed the same treatment. Redocly calls the default export of
its plugin, which `import/no-default-export` forbids, and JavaScript leaves the
type-aware rules nothing to narrow, so every value reads as `any`. Both are
switched off for `harness/quality/redocly/*.js` alone, next to the existing
exemption that lets Vite configs be default exports for the same reason: the
tool decides the shape, not the author.

## 2026-09-11 Toolpad Core adopted, then dropped

**Context.** The first design named Toolpad Core as the shell, reasoning that
workspaces would need a navigation frame later and that retrofitting one is
expensive. The scaffold was built on it. The browser then showed the app name
sitting off-centre in the header, and the console explained why: `React does not
recognize the 'justifyContent' prop on a DOM element`, four times, from Material
UI `Stack` elements inside Toolpad's own header.

`@toolpad/core@0.16.0` is the latest release and its peer range for
`@mui/material` is `^7.0.0`. This repository is on `^9.4.0`. MUI 9's `Stack` no
longer turns those props into styles, so Toolpad emits them as DOM attributes
and the layout they were meant to produce never happens. No Toolpad release
supports MUI 8 or 9, and the gap has been widening: 0.13 accepted MUI 6, 0.14
onwards only 7.

**Decision.** Drop it. The shell is `AppBar` + `Drawer` + `List` from Material
UI directly. What Toolpad supplied here was a layout frame, and that frame is
tens of lines of MUI.

**Consequences.** The alternative was holding Material UI two majors back to
keep one dependency, which is the wrong trade for a frame this thin - and the
MUI 9 pin was inherited from takamai rather than chosen, so nothing here argued
for it either. Moving now cost little because the shell was all that existed;
after workspaces it would not have been. The 44px minimum touch target that
`harness/quality/browser/layout.spec.ts` enforces stays in the theme: it was
added for Toolpad's 40px icon buttons but MUI's own default is the same 40px, so
it is still load-bearing.

## 2026-09-11 .gitignore allows by path, not by extension

**Context.** The deny-by-default `.gitignore` of 2026-09-10 allowed files by
extension: any `.md`, any `.yml`, any `.json`, wherever it sat. That reads like
deny-by-default and behaves like the opposite. Three holes had already been
patched by hand - `tmp/`, `scratch/`, and then `.playwright-mcp/`, which a
browser tool created at the repository root and whose `page-*.yml` was tracked
until `make fmt-check` tripped over it. Each patch was reactive, and the list
would have kept growing: every tool that writes a file writes it somewhere new.

Two more exceptions had accumulated for the opposite reason. A Redocly plugin
must be JavaScript, and `.js` was not a tracked extension, so it needed naming
individually; air's config is TOML, same story.

**Decision.** Allow by path. Section 3 names the trees the repository is made of
(`harness/`, `services/`, `web/`, `e2e/`, `docs/`, `.github/`, `.claude/`),
section 4 names the root-level files, and section 5 lists what stays out even
inside an allowed tree - build output, caches, test artefacts.

**Consequences.** An unknown directory is out from the start rather than in
until someone notices. All three hand-patched holes and both individual file
exceptions disappeared: `.playwright-mcp/` is out because nothing allows it, and
the Redocly plugin and `.air.toml` are in because they live under `harness/` and
`services/`. Adding a source tree is one line; adding a file type is nothing at
all. Verified over every file on disk: the only ones ignored are build output,
test artefacts and `docs/requirements.md`, which is deliberate.

## 2026-09-11 The catalogue reflects the served contract, casing included

**Context.** Each service answers `GET /openapi.yaml` by rendering YAML from the
spec oapi-codegen embedded in its generated code, so the served contract can
never drift from the code that serves it. The platform builds its catalogue from
that document.

Building the catalogue for real revealed that oapi-codegen normalises operation
ids when it embeds: `api/openapi.yaml` on disk says `listInventoryItems`, and
every service serves `ListInventoryItems`. Nothing else changes - descriptions,
summaries, `x-enum-labels` and `x-ui-hint` all survive the round trip, verified
by decoding the embedded blob with a temporary extension in place.

**Decision.** Leave it. The catalogue reflects what the running service says it
is called, and that name is the one the planner puts in its tool call and the
one `/api/invoke` looks up. The string is chosen and consumed in the same place.

**Consequences.** A contract file and the document its service serves differ in
the casing of four identifiers. Nothing reads both: the generated TypeScript
gives the frontend a `paths` map keyed by path, not by operation id, so no
second spelling of the name exists in the frontend. Tests that name an operation
id must take it from the catalogue's own vocabulary - `ListInventoryItems`, not
`listInventoryItems` - or build their catalogue by hand.

The alternative, serving `api/openapi.yaml` through `go:embed`, was rejected: it
would make the document a service serves and the document it validates requests
against two separate artefacts, to fix a difference in capitalisation.

## 2026-09-11 One flat PlanResult schema instead of a `kind`-discriminated oneOf

**Context.** `/api/plan` (docs/specs/orchestration.md, section 6) answers with
one of four shapes depending on `kind`: `result`, `form`, `ask` or `none`. OpenAPI
models a "one of these shapes" contract with `oneOf` plus a `discriminator`, and
oapi-codegen's Go output for a discriminated union is a union type with its own
`As<Variant>`/`From<Variant>` accessors on every response - a second vocabulary
layered over the one the strict server interface already gives each status code
its own named response type.

**Decision.** Model `PlanResult` as one flat object: `kind` plus every field any
variant can carry (`component`, `data`, `source`, `message`, `schema`, `initial`,
`target`, `question`, `param`, `options`), all but `kind` optional. The schema's
description states which fields are populated for which `kind`. Both new enums
this task adds (`DecisionKind` and `Component`) carry `x-enum-labels`, per the
harness's Redocly rule - `kind`'s labels (`結果`/`フォーム`/`質問`/`該当なし`) read
naturally in Japanese, so it stayed an enum rather than becoming a plain string.

**Consequences.** `internal/adapter/handler/plan.go` populates only the fields
its current `kind` needs and leaves the rest zero-valued; Task 7 (form) and Task
9 (ask) add fields to an existing struct rather than a new response type. The
generated Go and TypeScript types allow a client to read a field that `kind`
does not populate - not compile-time enforced. Given the four variants share
`Task 6`'s two-of-four-implemented status (`call`/`none` today, `form`/`ask`
belonging to Task 7 and Task 9), this reads as a stable contract to build a
frontend against sooner, at the cost of the stronger guarantee `oneOf` would
have given once every variant exists.

## 2026-09-11 domain.Endpoint and usecase.Decision travel by pointer through the Invoker port

**Context.** Task 6's plan sketch for `usecase.Invoker` reads
`Invoke(ctx, e domain.Endpoint, args map[string]any) (any, error)` - `e` by
value. `domain.Endpoint` is 136 bytes; golangci-lint's gocritic `hugeParam`
check (part of the fixed harness policy) already forced `domain.Render` and
`Endpoint.IsSafe` to take a pointer for the same reason (see the doc comments on
both), and it rejects `Invoker.Invoke`'s value parameter identically.

**Decision.** Declare `Invoker.Invoke` with `e *domain.Endpoint`, matching
`Render`'s and `IsSafe`'s existing precedent rather than the plan's literal
snippet. The same reasoning applies to `Orchestrator.call`'s `Decision`
parameter (112 bytes) and `stub.New`'s `notFound` parameter: both took a
pointer for the same hugeParam reason, with `stub.New(table, nil)` treated as
`&usecase.Decision{Kind: usecase.DecisionNone}`.

**Consequences.** The port's signature differs from the plan's code block by
one `*`; every caller and test double follows it. No behavioural difference -
`domain.Endpoint` and `usecase.Decision` are read-only inputs to `Invoke` and
`call` in every implementation.

## 2026-09-11 The stub planner's production table lives in pkg/app, keyed by exact Japanese query

**Context.** Task 6 requires the running platform to answer `/api/plan` without
ever calling a real LLM (`planner/stub` is every test's default, and until Task
10 it is the platform's _only_ Planner). `pkg/app.New` is the one place
acceptance tests and `cmd/api` both go through, and per its own doc comment
neither caller should need to see `internal/`.

**Decision.** `pkg/app` gained its own exported `Config`, `Service` and
`PlanFixture` types - not aliases of `internal/infra/config.Service` or
`internal/usecase.Decision` - so a caller builds a stub table without importing
anything under `internal/`. `Config.PlanFixtures` empty (production's default,
since nothing sets `ORCHESTRA_PLAN_FIXTURES` or similar) falls back to
`defaultPlanFixtures()`, a two-entry demo table (`"在庫の一覧を見せて"` →
`inventory/ListInventoryItems`, `"勤怠記録の一覧を見せて"` →
`attendance/ListAttendanceRecords`) hard-coded in `pkg/app/app.go`.

**Consequences.** A person can drive the running platform to a real rendered
table without any configuration beyond `ORCHESTRA_SERVICES`. The two demo
queries are the only Japanese the platform "understands" until Task 10 replaces
the stub with a real planner; anything else answers `kind: "none"`. `cmd/api`'s
`main.go` never sets `PlanFixtures`, so this table is what production runs with
today - a fact this decision records precisely so it is not mistaken for
intended product behaviour later.

## 2026-09-11 An ask offers the catalogue's values, never the model's

**Context.** Task 9 (`docs/plans/orchestration.md`) has `Orchestrator.Plan`
build a `kind: "ask"` result from a `DecisionAsk`, which carries `Question`,
`Param` and `Options` (`[]domain.Option`). `Decision` is ultimately produced by
a `Planner` - by Task 10, a real model reading the `ask_user` tool's schema -
so `Decision.Options` is exactly as trustworthy as any other tool-call
argument: nothing stops a model from inventing a value the catalogue does not
declare, or misspelling one that exists.

**Decision.** `Orchestrator.ask` ignores `Decision.Options` entirely.
`optionsForParam` searches the catalogue's own endpoints - each parameter,
and each unsafe endpoint's request body properties - for the first schema
named `decision.Param` that declares a non-empty `Enum`, and builds the
options from that schema's `Enum` and `EnumLabels` alone. A parameter no
endpoint declares as an enum returns `ok=false`, which `Plan` reports as
`usecase.ErrUnknownParam` (mapped to a 500, the same as any other
`Orchestrator.Plan` error the handler does not special-case) rather than
falling back to the model's own list or silently degrading to `kind: "none"`.
The reasoning: a disambiguation question exists specifically to replace a
model's guess with a person's answer; presenting options nobody vetted would
undermine the one property `ask_user` is for. An enum value with no entry in
`EnumLabels` - not something a spec-conformant service can produce, since
`x-enum-labels` coverage is a harness lint, but not ruled out by the type
system - gets an empty label rather than being dropped, the same defensive
choice `tools.go`'s `enumLabels` already makes for the model-facing
description.

**Consequences.** A `Decision.Ask` naming a real parameter always returns
options a person can actually submit back (Step 1's acceptance test asserts
all four `ItemStatus` values and their labels, sourced from the catalogue
fixture, not from the stub's decision literal, which deliberately names a
`bogus` option that never reaches the wire). A `Decision.Ask` naming a
parameter with a typo, or one belonging to no configured service, fails
loudly instead of asking a question with no good answers.

## 2026-09-11 The stub planner's table keys on {query, answers}, not query alone

**Context.** Task 9 Step 2 requires that posting the same question a second
time, now with `answers`, reach the planner and produce a different decision
than the bare question did (the ask, then the call it resolves to) - and the
stub planner must stay a pure, deterministic table lookup (the `make check`
constraint that no test ever calls a real LLM).

**Decision.** `internal/adapter/planner/stub`'s table is now
`map[Key]usecase.Decision`, where `Key{Query, Answers}` and `Answers` is built
by the new exported `AnswersKey(answers []usecase.Answer) string`: each answer
rendered as `"param=value"`, sorted, joined with `&`, so the same set of
answers canonicalises to the same key regardless of slice order, and no
answers canonicalises to `""` - the same key a bare question already used, so
every fixture from before Task 9 keeps working unchanged once its literal
`map[string]Decision` becomes `map[Key]Decision`. `pkg/app.PlanFixture` grew
`Answers []Answer` (its own type, mirroring `usecase.Answer`, for the same
reason `Service`/`PlanFixture` already avoid aliasing `internal/` types) and
an `Ask bool` (plus `Question`/`Param`) to describe an ask fixture without
repurposing `Service`/`OperationID`/`Args`.

**Consequences.** `stub.New`'s signature changed (`map[string]Decision` to
`map[stub.Key]Decision`); every caller and test updated in the same commit. A
fixture author writes two `PlanFixture` entries for one ask/answer pair - the
bare `Query` and the `Query`+`Answers` combination - rather than one entry
with conditional behaviour, which keeps the stub itself free of any decision
logic beyond the lookup.

## 2026-09-11 ask_user names the operation it was stuck on

**Context.** Task 9's `ask_user(question, param, options)` did not say which
operation it was standing in for, so `optionsForParam` resolved `param`
against the whole catalogue by name alone - a linear search returning the
first endpoint that declared an enum with that name. This does not collide
today because inventory's only enum parameter is `status` and attendance's
is `kind`, but the product is meant to grow to ten services and one to two
hundred operations (`PRODUCT.md`), where `status` or `type` colliding across
services is close to certain. A collision means a person is shown another
service's options for a question about theirs - the wrong values, offered
with a straight face.

**Decision.** `ask_user`'s input schema gains `service` and `operationId`,
both required, alongside the existing `question`, `param` and `options`
(`services/platform/internal/usecase/tools.go`). `Decision` already carried
`Service`/`OperationID` for `DecisionCall`; a `DecisionAsk` now populates the
same two fields with the operation the model was stuck on. `Orchestrator.ask`
looks that endpoint up with `catalog.Find(decision.Service,
decision.OperationID)` first - an unknown pair is `ErrEndpointNotFound`, the
same sentinel a bad `DecisionCall` already produces - and `optionsForParam`'s
signature changes from `(catalog domain.Catalog, name string)` to
`(endpoint *domain.Endpoint, name string)`: it now searches one endpoint's
parameters and request body properties, never the catalogue as a whole. A
parameter absent from that one endpoint's enums is still `ErrUnknownParam`.
`Decision.Options` remains untrusted, unchanged from Task 9 - the fix is
entirely about which endpoint `param` is resolved against, not about where
the offered values come from. `strict: true` (D10) means `operationId` is
not itself constrained to an enum of real operation ids the way a catalogue
parameter is, but that is no looser than `service`/`operationId` already are
on every other tool call: `catalog.Find` is exactly the existence check a
`DecisionCall` already relies on, so a model naming a nonexistent operation
fails the same way here as it always has.

**Consequences.** An ask about one service's `status` can never surface
another service's `status` values, however many services and operations the
catalogue grows to hold. Every `ask_user` caller - the stub planner's table
via `pkg/app.PlanFixture` (already had `Service`/`OperationID`, unused by an
ask fixture until now) and, later, the real planner adapters of Task 10/11 -
must supply the operation a `DecisionAsk` names, not only its parameter.

## 2026-09-11 A tool is strict only when every property it declares is required

**Context.** `usecase.Tool.Strict` is always `true` (D10), but `usecase.ToolsFor`
deliberately emits a plain JSON Schema with no `additionalProperties` and a
`required` list that only names the properties an endpoint's OpenAPI contract
actually requires - `ListInventoryItems`'s optional `status` filter is the
example (`internal/usecase/tools.go`). OpenAI's strict function calling
requires `additionalProperties: false` on every object _and_ every declared
property to be listed as required, so sending that schema with `strict: true`
verbatim is not valid strict mode for an endpoint with any optional
parameter - `status` would have to be both declared and, dishonestly,
`required`.

**Decision.** `internal/adapter/planner/toolcall/planner.go`'s `shapeTool`
adds `additionalProperties: false` to every object schema unconditionally -
top level, `items`, and every `properties` entry - regardless of whether the
tool ends up strict, since forbidding an invented extra argument is harmless
on every backend this adapter targets today (verified by hand against
llama-swap, which does not enforce strict mode at all) and is exactly the
harmless half of strict mode when a backend does enforce it. Whether a given
tool is sent with `strict: true` or `strict: false` is decided per tool by
`everyPropertyRequired`: strict only when the (already-shaped) schema's
`required` list names every one of its top-level `properties`. `CreateInventoryItem`
and `ask_user` qualify (every field their input schema declares is required);
`ListInventoryItems` does not, because `status` is optional. The alternative
the plan raised - widening an optional property's type to include `null` so
it can stay both `required` and strict - was rejected: it would require the
model to pass `status: null` explicitly to mean "no filter", which no
prompt in this codebase asks for and no model was observed to do, whereas
`strict: false` for that one tool costs nothing against llama.cpp-family
backends and is honest about what the schema actually promises.

**Consequences.** Every tool's `additionalProperties` is `false`; `strict` is
computed per tool, not copied from `usecase.Tool.Strict`, and a tool
gains or loses strictness automatically as its schema's required set changes

- no adapter code has to be updated by hand when a service adds or removes an
  optional filter. `internal/adapter/planner/jsonmode` (Task 11) does not use
  this shaping at all, by design: it has no `strict` mode to satisfy, and
  `usecase.ToolsFor`'s unshaped output is what it needs instead (see the doc
  comment on `Tool.Strict` in `internal/usecase/tools.go`).

## 2026-09-11 An operation id that names two services picks the first catalogue match

**Context.** A tool call in the OpenAI wire format carries only a function
name - `usecase.ToolsFor` names each tool after its operation id alone, never
qualified by service (`Name: e.OperationID`, `internal/usecase/tools.go`), so
nothing in the request or the response says which service a `DecisionCall`'s
operation belongs to. `internal/adapter/planner/toolcall.Planner` has to
resolve that itself, by searching the catalogue it was built with
(`resolveService` in `planner.go`). Every operation id in this product's
catalogue is unique today (`ListInventoryItems` vs. `ListAttendanceRecords`,
not a bare `List`), but nothing enforces that as more services are added.

**Decision.** `resolveService` returns the service of the _first_ endpoint in
`catalog.Endpoints` whose operation id matches - deterministic, because the
catalogue is built by iterating configured services in a fixed order
(`internal/adapter/specsource/http`), but not necessarily correct if two
services ever do collide. An operation id present in no endpoint at all is a
harder failure: `Plan` returns `ErrUnknownOperation` rather than guessing,
since there is no catalogue entry to fall back to. Rejected alternatives:
failing the whole `Plan` call on any ambiguity (this would make the planner
brittle to a naming collision the catalogue's own author caused, not the
model), and threading the tool's originating service through the wire
format (the OpenAI chat-completions tool-call shape has no field for it, and
inventing one is not portable to any other endpoint speaking the same
protocol).

**Consequences.** Keeping operation ids unique across the whole catalogue is
now a real correctness requirement for this planner, not just a stylistic
one - a future service must not reuse another's operation id, or a person
could silently see the wrong service's data. This is judged acceptable
because the alternative (qualifying every tool name with its service) makes
every tool name longer and uglier for every model, to guard against a
collision that a naming convention already avoids. `docs/plans/orchestration.md`
Task 11's JSON planner, which puts `service` and `operationId` in the same
JSON object the model returns, does not have this problem at all - see its
own mapping code once it exists.

## 2026-09-11 qwen3.5-9b-q8 is the default local model for the tool-calling planner

**Context.** No local model under about 20B parameters was observed to
reliably choose `ask_user` for an ambiguous enum value ("破損した在庫はある？"
against `qwen3.5-9b-q8` picked `ask_user` only 1 run in 5, guessing
`{"status":"quarantined"}` the other 4). Given that, the choice of default
model was made on a different, more decisive axis instead: what a model does
with a parameter the question does not mention at all. Four models were
measured against the same tool definitions and the same unfiltered question
("在庫を全部見せて"), five runs each: `gpt-oss-20b` invented a nonexistent
filter in `args` on 5 of 5 runs - the result silently misses rows the person
never asked to exclude, and nothing in the response says so. `gemma4-26b-a4b-qat`
never invents a filter, but was separately observed to drop a filter the
question _did_ specify ("破損した在庫はある？" calling `ListInventoryItems {}`
instead of asking or filtering). `qwen3.5-9b-q8` and `qwen38-27b-iq3s` both
returned empty `args` on 5 of 5 unfiltered runs - neither fabricates nor
drops.

**Decision.** `services/platform/.air.toml`'s `full_bin` sets
`ORCHESTRA_LLM_MODEL=qwen3.5-9b-q8` as the platform's default local model.
Between the two models that pass the no-fabrication test, `qwen3.5-9b-q8` (9B)
is preferred over `qwen38-27b-iq3s` (27B) purely on footprint: this machine
has 16GB of VRAM, and the smaller model loads faster and leaves headroom
(measured at 12.9GB resident, ~3GB free) without a measured behavioural cost
against the larger one. Of the three ways a local tool-calling model was
observed to fail on this catalogue - fabricating a filter, dropping one, or
guessing at an ambiguous value instead of asking - fabrication is treated as
disqualifying and the other two are not: a fabricated filter changes the
answer while looking exactly like a correct one, which the person has no way
to notice from the response alone, whereas a dropped filter or a guessed
value is at least visible in `source.args` next to the question that was
asked.

**Consequences.** The default answers `検品保留の在庫を見せて` /
`みなし労働の勤怠を見せて` / `在庫を全部見せて` correctly and returns `kind: "form"`
/ `kind: "none"` for a create and an out-of-catalogue question respectively -
all verified live against the running platform. It does not reliably use
`ask_user`; a question with a genuinely ambiguous enum value is more likely to
be silently guessed than asked about, which is a real gap `docs/plans/orchestration.md`
does not yet have a task to close. The model is a one-line change
(`ORCHESTRA_LLM_MODEL` in `.air.toml`, or `ORCHESTRA_LLM_MODEL` in the
environment for any other deployment) - swapping it requires no code change,
which is exactly what D5's per-request model field was for.

## 2026-09-11 An operationId names one operation across every service

**Context.** A tool definition is named by its operationId alone
(`services/platform/internal/usecase/tools.go`), and a tool call carries only
that name back, so the platform resolves the service by searching the catalogue
for it (`internal/adapter/planner/toolcall`). Two services declaring the same
operationId therefore break twice: the model is offered two tools with one name,
which a tool-calling API rejects and a JSON planner cannot tell apart, and a
call that does arrive is routed to whichever service the catalogue lists first.

Nothing was checking. Redocly's `operation-operationId-unique` reads one
document at a time, and each service is generated, linted and compiled on its
own, so the collision exists only in the platform, only at runtime, and only
once both services are running. Task 10 recorded first-match-wins as an accepted
limitation; that is the kind of requirement this repository enforces rather than
writes down.

**Decision.** `make guard-operation-ids` (`harness/guard/operation-ids.sh`)
bundles every `services/*/api/openapi.yaml`, reads their operationIds, and fails
when one name is claimed by more than one service. A duplicate inside a single
spec stays Redocly's job.

The guard is deliberately stricter than the platform strictly needs: `ToolsFor`
excludes an endpoint that has neither a request body nor a JSON response, so the
spec-serving `GET /openapi.yaml` never becomes a tool and could safely share a
name. Encoding that exclusion in shell would copy a Go rule into a second place
and let the two drift. A guard with no business logic in it is worth more than
the names it costs.

**Consequences.** It found a collision on its first run: `getSpec`, declared by
both services, now `getInventorySpec` and `getAttendanceSpec`. The generated
method names moved with them, which also removes a confusion the old name had -
oapi-codegen emits its own package-level `GetSpec()` for the embedded document,
so a service had two different `GetSpec`s in one package.

Every new service pays one line of naming convention: prefix, suffix, or
anything else that makes the name its own. The alternative was paying it in a
planner that sometimes calls the wrong service.

## 2026-09-11 PlanResult.fields structures enum labels for the UI

**Context.** `x-enum-labels` reaches `domain.Schema.EnumLabels`, and
`schemaToJSONSchema` (`services/platform/internal/usecase/tools.go`) already
folded it into a property's `description` as `"allocated=引当済 / staged=..."`
— but that string exists to be read by the model, not parsed by a UI. Task
13 shipped a `/api/plan` `kind: "result"` response with no column
information at all, so a person asking "検品保留の在庫を見せて" in Japanese
got a table whose `status` column showed the raw English enum value
(`quarantined`) — there was nowhere else in the response for a label to
come from, and splitting the model-facing description string apart in the
browser was not an option.

**Decision.** `schemaToJSONSchema` now also emits a structured `enumLabels`
(value -> label) map alongside `enum`, in addition to — not instead of —
the folded description; the model still reads the description, the UI reads
`enumLabels`. `PlanResult` gained a `fields` property: per-column schema for
a table's rows or a detail's own properties, built by calling
`schemaToJSONSchema` on the schema `domain.FieldsSchema` (`internal/domain/
rendering.go`) returns — a new exported function added next to `RenderResult`
rather than reimplementing its wrapper/`soleArrayProperty` judgment a second
time in `usecase`. One conversion function, one row-schema judgment; `fields`
is omitted (not an empty object) whenever the result renders as neither
`table` nor `detail`, or the row schema has no properties to describe.

**Consequences.** `web/src/entities/rendering/model/rows.ts` reads
`fields[column].enumLabels[value]` for a cell and `fields[column].title` for
a header, falling back to the raw value/key when either is absent — checked
against the running inventory and attendance contracts, neither of which
declares a property `title` today, so headers still show the raw key until
a spec adds one. Every endpoint gains one more thing `Render`'s rules decide
for it instead of the browser guessing: the UI no longer needs, and must
never regain, its own copy of "which array property holds the rows".

## 2026-09-11 Contracts carry a Japanese `title`; `/api/invoke` gains `fields`

**Context.** `PlanResult.fields`'s `enumLabels` (the previous entry) fixed an
enum's own values, but a table header or a form label is a field's _name_,
not one of its values, and `x-enum-labels` says nothing about that: none of
`services/inventory/api/openapi.yaml` or `services/attendance/api/openapi.yaml`
declared a `title` on any property, so `fields[column].title` — already
implemented and tested in `web/src/entities/rendering/model/rows.ts`'s
`columnTitle` — always fell through to the raw key (`name`, `quantity`,
`status`). Separately, `InvokeResult` (`services/platform/api/openapi.yaml`)
never gained a `fields` property when `PlanResult` did: `usecase.Orchestrator
.Invoke` already called the same `invokeAndRender` → `fieldsFor` path `call`
does, so `usecase.Result.Fields` was populated correctly, but `handler.Invoke
.PostInvoke` never copied it onto the wire response — so a detail turn
appended after a form submission showed `status` as `allocated`, the one
path Task 14 did not carry `fields` through.

**Decision.** Every property a screen renders in the inventory and
attendance contracts now carries a short Japanese `title`, alongside — not
replacing — its existing English `description`: `title` is for the person
looking at the screen, `description` is for the model reading the tool
definition, and the two are kept apart on purpose. `InvokeResult` gained a
`fields` property, the same shape as `PlanResult.fields`; `PostInvoke` now
copies `result.Fields` onto it exactly as `PostPlan` already did, rather
than adding a second way to build it.

**Consequences.** A table, a form and a detail card now all show Japanese
field names, not just Japanese enum values, and a detail turn built from a
form submission is indistinguishable from one `/api/plan` produced directly
— both carry `fields`, read through the same `columnTitle`/`cellText`. Any
new property on these two contracts needs a `title` to show up correctly on
screen; nothing enforces that today beyond this file recording it as the
convention.

## 2026-09-11 x-orchestra-expose gates the catalogue; ToolsFor drops its shape-based filter

**Context.** Every operation a service's contract declared became a tool:
`usecase.ToolsFor` excluded one only when its shape had neither a Response
nor a RequestBody schema, which happened to catch `GET /openapi.yaml` but
was never a rule about what should reach the model — a health check, an
internal admin call, a batch trigger, a webhook receiver, all had a
renderable shape and all passed straight through. `/api/invoke` read the
same catalogue `ToolsFor` built tools from, but nothing tied the two
together: an exclusion inside `ToolsFor` alone would have left `/api/invoke`
still willing to call an operation the model was never offered — a person
with a browser console could reach it directly, making the mark decorative.

**Decision.** A new vendor extension, `x-orchestra-expose` (boolean,
default `false`), marks an operation as one the platform may show the model
and answer at `/api/invoke`. It is read exactly once, in
`internal/adapter/specsource/http.parseSpec`, as `domain.Endpoint`s are
built from a service's spec — before `ToolsFor` and `Catalog.Find` both read
`domain.Catalog.Endpoints`, not after either of them. An unexposed operation
is not filtered later; it is simply never in the catalogue, so
`ErrEndpointNotFound` (400) is what `/api/invoke` returns for it, identical
to an operation id that does not exist. The default is `false`, not `true`:
a service nobody has reviewed yet should be invisible by construction, not
exposed until someone remembers to hide the parts that should not be. Any
value other than the literal boolean `true` — absent, `false`, a quoted
`"true"` — counts as unexposed; a typo fails closed. `ToolsFor`'s
shape-based exclusion is deleted along with this: once exposure is a
declared intent, an operation marked exposed that nothing can render is a
spec mistake, and silently dropping it would hide that mistake instead of
surfacing it. `harness/guard/exposed-ops.sh` fails the build on exactly that
case, and on a service that marks nothing exposed at all.

**Consequences.** `services/inventory/api/openapi.yaml` and
`services/attendance/api/openapi.yaml` mark their three catalogue operations
(list, create, get) `x-orchestra-expose: true`; `GET /openapi.yaml` in both,
and the platform's own contract (never fetched by `specsource/http` at
all — see `harness/guard/exposed-ops.sh`), carry no mark. Adding an
operation to either service now defaults to invisible until someone opts it
in, which is the point: `docs/specs/orchestration.md` D13, section 8.

## 2026-09-11 ResultChoice finds its original query by walking the turn list, not by carrying it

**Context.** Task 15's `ResultChoice` (AC-F-103) resubmits `/api/plan` with
the person's original question alongside the chosen `answers`, but a
`kind: "ask"` `PlanResult` carries no copy of it — `PlanResult.question` is
the planner's own disambiguation prompt ("「破損」に近いステータスはどれですか？"),
not what the person typed. The obvious fix, a `query` field on
`AnswerTurn`, would need every producer of an answer turn to remember to set
it, including `ResultForm`'s `/api/invoke` submission path, which has no
query to give at all.

**Decision.** `TurnList` derives the original query at render time by
walking `turns` backward from an `ask` turn's own index to the nearest
preceding `role: "question"` turn, rather than storing it on the turn.
Walking backward instead of reading `turns[index - 1]` directly matters once
a choice has already been answered once: the turn immediately before a
second `ask` may be another answer turn, not the question, and the nearest
_question_ is still the one `/api/plan` needs back.

Separately, `PlanResult.options` — the one place a schema-declared
array-of-objects property reaches the browser through openapi-fetch's
`MethodResponse` mapping — type-checks with every one of its own prototype
methods (`.map`, `.filter`, `.flatMap`, ...) as `{}`; calling them directly
is a compile error, not an unsafe read. `ResultChoice` reads it the same
defensive way `ResultForm`/`rows.ts` already read a schema property whose
declared type cannot be trusted as-is: `Array.isArray` (which narrows via
its own `any[]` signature, sidestepping the broken methods) followed by a
`Record<string, unknown>` guard per element.

The option row locks against a second click through a `useRef` flag set the
instant a click is accepted, not through the `submitting` state `useSubmission`
(new: `entities/rendering/model/useSubmission.ts`, extracted out of
`ResultForm` to keep the two components' `try`/`catch`/`finally` from
tripping `guard-duplication`) already exposes for the row's visual lock —
`submitting` lags one render behind the click, which measurably let two
fast clicks both reach `postPlan` in testing. The chosen option is drawn
`contained` against its `outlined` siblings rather than disabled through
MUI's `disabled` prop: a disabled `contained` `Button` has no border of its
own and fails `make guard-layout`'s WCAG 1.4.11 contrast check at rest
(measured), so the lock is `opacity`/`pointerEvents: none` on the row via
`sx` instead.

**Consequences.** `Turn`/`AnswerTurn` (`features/conversation/model/turn.ts`)
stays unchanged — nothing needs to remember a query it does not have.
`TurnList.tsx` grew a `nearestQuestion` helper and an `AnswerResult`
component split out of `TurnItem` to stay under `max-lines-per-function`
once the fifth `kind`/`component` branch (`ask`) was added. Verified live
against `http://100.75.74.118:5173/` and `qwen3.5-9b-q8`: 1 `ask` in 5
attempts at `破損した在庫はある？` (the other 4 guessed a status directly,
matching the measurement above), picking an option correctly re-posted the
original query and appended a filtered table turn. Fixed independently of
that variance in `Conversation.test.tsx`.

## 2026-09-11 list_capabilities answers "what can this do" from the catalogue, as its own decision kind

**Context.** The platform could not answer a question about itself: "何が
できるの？" or "在庫について、どういう操作ができる？" fell through to
`kind: "none"`, because nothing in the catalogue's own tools describes the
catalogue - every tool is one endpoint, and the model has nothing to call
when the question is about the set of endpoints rather than any one of
them. The answer to that question already exists, in full, as
`domain.Catalog` - the same structure `ToolsFor` already builds the model's
tools from - so the fix is to expose it, not to ask the model to describe
it from memory: a model's own prose summary of "what it can do" can name an
operation that does not exist or miss one that does, which is exactly what
D8 (`docs/specs/orchestration.md`) already ruled out for every other
answer.

**Decision.** `list_capabilities(service?: string)` is a new built-in tool,
alongside `ask_user`: not derived from any service's spec
(`usecase.ListCapabilitiesTool`), always offered. `service` is a plain
string, not an enum - the harness's own `x-enum-labels` lint would force
every enum to carry Japanese labels for a fixed value set, but the set of
services grows as new ones are configured, so the tool's description tells
the model to use the same service names it already sees named among the
other tools instead.

It gets its own `DecisionKind` (`DecisionListCapabilities`) rather than
being folded into `DecisionCall` and switched on `OperationID` inside
`Orchestrator.call`: `call` resolves its `Service`/`OperationID` against
`Catalog.Find` and would fail outright, because `list_capabilities` is not
a catalogue endpoint - the same reason `ask_user` already has its own
`DecisionAsk` path rather than being a `DecisionCall` in disguise. The new
`Orchestrator.listCapabilities` never touches the invoker; it filters and
sorts `catalog.Endpoints` in memory and renders straight to
`ResultKindResult` / `component: "table"`, with `data: {items: [...]}` so
the existing `soleArrayProperty` (`domain`) / `rowsFromData` (`web`)
envelope convention renders it without any new frontend component.
`internal/adapter/planner/toolcall/planner.go` routes the tool name to
`decisionFromListCapabilities` before it would otherwise reach
`resolveService`, mirroring how `ask_user` is intercepted first.

An unmatched `service` filter (a typo, or a service that genuinely does not
exist) renders as a `table` with zero rows, not a silent fallback to the
whole catalogue: the person asked about one named service, and answering
with every service's operations instead would misrepresent what was asked

- the same reasoning `ErrEndpointNotFound` already applies to a
  `DecisionCall` naming an operation the catalogue does not have, just
  rendered as an empty result instead of an error, since "this service has
  nothing" is a legitimate answer where "this operation does not exist" is
  not. Rows are sorted by `(service, operationId)` rather than left in
  `catalog.Endpoints`' own order, so the order on screen cannot depend on
  service configuration order.

`kind: "none"`'s message now also points at `list_capabilities`
("「何ができるの？」と聞くと、できることの一覧を確認できます。"): a
question the catalogue genuinely cannot answer ("今日の天気は？") still
lands on `none`, but it no longer reads as a dead end for someone who
simply did not know what to ask.

**Consequences.** `services/platform/api/openapi.yaml` needed no change:
`PlanResult`'s `kind`/`component`/`data`/`source`/`fields` already cover a
`list_capabilities` result exactly as they cover any other table.
`web/src/entities/rendering` needed no change either, verified live.
`ToolsFor` now returns one more tool than before for every catalogue;
`services/platform/internal/usecase/tools_test.go`'s fixed-count
assertions were updated alongside it.

## 2026-09-11 An ask over a parameter with no enum degrades to a form, not a 500

**Context.** `POST /api/plan` with `{"query":"在庫を登録したい"}` ("I want to
register some inventory") returned a 500 roughly 3 times in 5:
`parameter not found in catalogue: "name" on inventory/CreateInventoryItem`.
"登録したい" names no item, so the model - reasonably - reached for
`ask_user` naming `name`, the one required field it had nothing to fill in.
D11 built `ask_user` around an enum (a list of values to pick from), and
`optionsForParam` (`orchestrator.go`) only ever looks for one; `name` is a
free-text string with no enum, so `optionsForParam` reported it unknown and
`Orchestrator.ask` turned that into `ErrUnknownParam`, which
`handler/plan.go` mapped straight to 500. This is a platform defect, not a
model misbehaviour: the model used `ask_user` exactly as its own
description told it to ("call this when the question does not tell you
which value to use"), for a case the tool's shape did not anticipate. A
model choosing a documented tool for a documented reason must never crash
the response.

**Decision.** `Orchestrator.ask` no longer returns `ErrUnknownParam` when
`optionsForParam` cannot find an enum for the named parameter. It degrades
to `ResultKindForm` instead - the same shape an unsafe `DecisionCall`
already produces - naming the endpoint the model was stuck on and carrying
`decision.Args` (whatever the model did fill in) as the form's initial
values. `ErrUnknownParam` is deleted; nothing outside its own doc comment
and two other comments referenced it (checked, `grep -rn ErrUnknownParam`).
A person cannot pick a free-text value from a list, so a form - type it in
yourself - is the only thing left an ask can degrade to; this was the
model's genuine question, just asked through a tool built for an enum it
did not have (D11, `docs/specs/orchestration.md`, updated alongside).

The form's schema is now built by `inputSchemaFor` (`tools.go`) - the same
function that already builds a tool's `InputSchema` for the model, merging
an endpoint's parameters and request body into one JSON Schema - rather
than `formSchema`, which converted only the request body and left a form
empty for a GET-shaped endpoint (query parameters, no body) such as
`GetInventoryItem`. `formSchema` is deleted; there is now exactly one
converter, not two, so a form's fields can never drift from what the model
was told about the same endpoint. This changes the exact JSON a form's
`schema.properties` carries for an endpoint with no properties at all (an
explicit `"properties": {}` where none was sent before, since
`inputSchemaFor` always initialises the map) - not a behaviour change,
since an absent and an empty `properties` render identically, but
`orchestrator_test.go`'s fixed-value assertions for the existing create
form were updated to match.

`askUserDescription` (`tools.go`) was also sharpened - "ONLY" for an enum
parameter, an explicit "do NOT call this for a free-text parameter" - but
only after measuring, not as the fix itself: a description is a request a
model can still ignore, and this one already was (D11's original wording
already said "for an enum" without stopping the model reaching for it over
a free-text field). The platform-side degradation above is what makes the
outcome safe regardless of what the model does next.

**Measurement**, `qwen3.5-9b-q8`, 5 runs per query, before and after (recorded
in the PR/report rather than duplicated here in full): before the fix, the
four registration-intent queries 500'd 3-4 times in 5 with
`ErrUnknownParam`, the rest landing on `list_capabilities` or a correct
`ask` about `status`; after, every run of those four queries returned
`kind: "form"` for `CreateInventoryItem`, and the unrelated
`list_capabilities`/`result`/`none` queries were re-measured alongside and
were unaffected by either change.

**Consequences.** `ErrUnknownParam` is gone from `usecase`'s exported
surface; a caller that matched on it (none exists in this repository -
checked) would need updating. `formSchema` is gone; `inputSchemaFor` is now
called from both `Orchestrator.call` (the unsafe-call form) and
`Orchestrator.ask` (the new degraded form), so the two forms' schemas are
built identically. No `openapi.yaml` change was needed: `PlanResult.schema`
was already an untyped JSON object.

## 2026-09-11 ORCHESTRA_PLAN_FIXTURES feeds the stub planner from outside the process

**Context.** Task 16 (`docs/plans/orchestration.md`) needed a process-level
suite (`e2e/src/orchestration.test.ts`) and a browser suite
(`e2e/browser/chat.spec.ts`) that ask a question against the platform's
_built binary_, started as a separate OS process. Both must go through the
stub planner - `make check` must never call a real LLM - but
`pkg/app.Config.PlanFixtures` is a Go API: a test driving `services/platform/bin/api`
over TCP cannot reach it, and the stub's table is empty by default since
`defaultPlanFixtures` was removed (2026-09-11, above): every question would
come back `DecisionNone` with no way to fixture one from outside the process.

**Decision.** Added `ORCHESTRA_PLAN_FIXTURES`: a JSON array, read once by
`internal/infra/config.Load` (the one seam allowed to read an environment
variable) into `[]config.PlanFixture`, converted by `cmd/api/main.go` into
`[]app.PlanFixture` exactly the way `ORCHESTRA_SERVICES` already becomes
`[]app.Service`. It reaches `pkg/app.Config.PlanFixtures` unchanged from
there - no new plumbing inside `pkg/app` itself. Production never sets it:
an operator sets `ORCHESTRA_LLM_BASE_URL` instead, which makes
`pkg/app.newPlanner` pick the tool-calling planner and ignore
`PlanFixtures` outright, so this variable has no effect once a real LLM is
configured. `e2e/playwright.config.ts`'s `webServer.env` and
`e2e/src/orchestration.test.ts`'s spawned platform process both set it to
one fixture: `"在庫の一覧を見せて"` against `inventory`'s
`ListInventoryItems`, the same query `web/acceptance/src/App.test.tsx`
already exercises against a scripted `fetch`.

**Consequences.** The stub planner's escape hatch for driving a built
binary from outside the process is now a first-class, tested part of
`config.Load` (`config_test.go` covers parsing, defaulting and malformed
JSON), not a one-off. `e2e/playwright.config.ts`'s `webServer` became an
array of three (inventory, attendance, platform) on fixed ports
(18083/18084/18080) distinct from a developer's own running processes and
from the harness's own browser gates (port 18081); Playwright 1.63 supports
an array there. `guard-a11y`/`guard-layout` (`make guard-browser`) now run
against a real, populated screen for the first time and passed unchanged.

## 2026-09-11 The JSON planner's response_format schema must preserve field order, not just contents

**Context.** Task 11 (`docs/plans/orchestration.md`) adds `jsonmode.Planner`,
a second `usecase.Planner` for models that cannot call tools: it renders the
catalogue as text and asks for one JSON object back. `response_format`
(OpenAI's `json_schema` mode) was unverified against llama.cpp/llama-swap
before this task; tested by hand first (`curl` straight to
`http://localhost:11435/v1/chat/completions`), it was accepted and honoured -
a `json_schema` response_format around a flat object produced clean,
schema-matching JSON. On that basis the planner was built to always send
`response_format`, falling back to a plain request only when the endpoint
answers with a non-2xx status (`Planner.complete`, checking
`errors.Is(err, chat.ErrRequestFailed)`) - the literal reading of "sets
response_format when the endpoint accepts one and falls back... when it does
not", since acceptance is a runtime property of the endpoint, not something
knowable from configuration.

That much worked - until it was exercised against the full catalogue prompt
live, where `qwen3.5-9b-q8` corrupted its own JSON on every `kind: "call"`
attempt (`{"kind":"call","service":"inventory\",\"operationId\":...`,
consistently breaking right after "service"), while `kind: "none"` and
`kind: "list_capabilities"` - the two shapes with the fewest fields - came
back clean every time. The `response_format`'s JSON Schema was built as a Go
`map[string]any`, the way every other schema in this codebase is; `json_schema` grammar-constrains
the model's token generation to the schema's _declared_ property order, and `encoding/json`
always marshals a map's keys alphabetically. That put `"args"` before
`"kind"` and `"service"` in the wire request - a property order that does
not match the order the model's own chain-of-thought reaches them in (it
reasons about `kind`, then `service`+`operationId`, then `args` last),
confirmed by hand: the exact same system prompt and question, with the
schema's `properties` reordered to `kind, service, operationId, args, param,
question` (the order those fields already appear in in
`systemPromptHeader`'s own four examples), produced clean JSON on every
attempt against a model that had just failed at it three times running.

**Decision.** `jsonmode.decisionSchemaJSON` is a hand-written
`json.RawMessage` constant, not a `map[string]any` - the one schema in this
package built that way, called out in its own doc comment so it is not
"fixed" back to a map later. Its property order matches the prompt's own
example order. `buildResponseFormat` assigns it directly as the `"schema"`
value inside the (otherwise ordinary, alphabetised-by-`encoding/json`)
outer `map[string]any` `ResponseFormat.JSONSchema` - `json.RawMessage`
implements `json.Marshaler`, so `encoding/json` emits its bytes verbatim
wherever it sits, regardless of the surrounding map's own key order. The
schema stays deliberately permissive (every field optional but `kind`, no
stricter `oneOf` keyed on `kind`): `docs/specs/orchestration.md` already
notes `strict: true` has no equivalent for this transport, so the real
validation is `jsonmode.parse`/`validateArgs` in Go, and a plainer grammar
is also less for the model to fight.

Verified live end to end after the fix: `検品保留の在庫を見せて` and
`在庫を全部見せて` (both `kind: "call"`), `何ができるの？` (`kind:
"list_capabilities"`) and `今日の天気は？` (`kind: "none"`), three times
each against `qwen3.5-9b-q8` through `ORCHESTRA_LLM_MODE=json` on a
separate port - 12 for 12, no retry ever needed. The tool-calling planner
answered the same `検品保留の在庫を見せて` identically (same `args`, same
rows) over the already-running dev instance, for comparison.

**Consequences.** A JSON Schema built for a grammar-constrained local model
is not just a validation contract - its _property order_ is part of the
prompt, in the same way message order already is. Any future schema built
for `response_format` against a small local model should default to
matching whatever order the model is already being told about the fields in
(the prompt's own examples, here), not whatever order `encoding/json` or a
schema library happens to produce. `jsonmode.Planner`'s enum validation
(`validateArgs`) and its one-retry-then-error behaviour were exercised
separately, with `httptest` fixtures (`planner_test.go`), and never needed
against the live model in these dozen runs - the ordered schema alone was
enough to stop the corruption that would have triggered them.

## 2026-09-11 The JSON planner asks where the tool-calling one guesses

**Context.** `DECISIONS.md` recorded that no local model under about 20B
parameters was observed to reliably choose `ask_user` through tool calling:
`qwen3.5-9b-q8` guessed a status instead, four runs in five. `TODO.md` asked
whether a JSON object shaped `{"kind": "ask", ...}` is an easier target for a
small model than a tool call is, and Task 11's live questions were never
ambiguous enough to answer it.

**Decision.** Measured, five runs each, same model, the two planners side by
side:

| question             | tool calling                                       | JSON       |
| -------------------- | -------------------------------------------------- | ---------- |
| 破損した在庫はある？ | guessed `quarantined` 4/5, `list_capabilities` 1/5 | `none` 5/5 |
| あやしい在庫を見せて | `ask` 2/5, guessed 3/5                             | `ask` 4/5  |

The JSON planner reaches `ask` where the value genuinely maps to nothing
(あやしい - "suspicious", which no status means), and answers `none` where
the tool-calling one talks itself into a guess (破損 - "damaged", which
検品保留 nearly means).

**Consequences.** Neither is right about 破損: one filters by a status the
person did not ask for, the other refuses a question it could have asked
about. But the JSON planner fails towards saying nothing rather than towards
saying something wrong, and a wrong filter is the failure a person cannot
see. Nothing changes in the code; this is what the two adapters do, recorded
so the next person choosing between them is choosing with numbers.

## 2026-09-11 guard-exposed-ops also requires title on every drawn property

**Context.** A screen field's label comes from the schema's `title`
(`web/src/entities/rendering/model/rows.ts`, `columnTitle`); with none, the
raw English JSON key (`name`, `quantity`) leaks through. Nothing fails when
`title` is missing - Task 13/14 only turned this up once a UI existed to
look at it. `x-orchestra-expose: true` now marks exactly the operations a
person can see, so "every property an exposed operation draws" is a bounded,
checkable set rather than "every property everywhere."

**Decision.** A third failing condition in `harness/guard/exposed-ops.sh`:
for every `x-orchestra-expose: true` operation, every property the platform
actually draws must carry a `title`. "Draws" is read off the platform's own
rendering code rather than redefined: the response side mirrors
`domain.FieldsSchema` (`internal/domain/rendering.go`) and
`specsource/http.convertResponse`'s first-2xx-json-response rule; the
request body side is an object body's own properties
(`usecase.mergeRequestBody`); parameters count too, because
`usecase.inputSchemaFor` merges every parameter into the same form schema
`ask` degrades to whenever a stuck argument has no enum
(`usecase/orchestrator.go`, `ask`), for any exposed operation, safe or not -
verified by reading `ask` before deciding, not assumed.

Bundling switched from a plain `redocly bundle` to `--dereferenced`: a plain
bundle leaves an internal `$ref` (a parameter's schema pointing at
`#/components/schemas/ItemStatus`, say) exactly as written, so the guard's
`jq` would see the `$ref` object itself, never the shared schema's `title`.
Dereferencing resolves every `$ref` before `jq` reads the file, which is
also what "a shared schema's `title` covers every `$ref` to it" depends on
being true.

The two service specs were missing exactly one `title` each:
`getInventoryItem`'s and `getAttendanceRecord`'s path parameter `id` had no
`title` (every other property already carried one).

**Consequences.** Verified by breaking each of the guard's three conditions
in turn (an unrenderable exposed operation, a service with none exposed, a
drawn property with no `title`) and confirming the guard names exactly what
is wrong, then restoring. `make check` stays green.

## 2026-09-11 Workspaces Task 0: IDs, the JSON boundary, and a required `ORCHESTRA_DB_PATH`

**Context.** `docs/plans/workspaces.md` Task 0 asks for a SQLite-backed
`usecase.WorkspaceStore`, storing `domain.Workspace`/`domain.Panel` (section
3 of `docs/specs/workspaces.md`), with three things left to the implementer:
how IDs are generated (deterministically testable), where `Panel.Args`
(`map[string]any`) is converted to and from JSON given `domain` and
`usecase` may import nothing but the standard library and never
`encoding/json` (`harness/quality/go/golangci.yml`, depguard), and what
happens when `ORCHESTRA_DB_PATH` is unset.

**Decision.** IDs: `crypto/rand.Text()` (Go 1.24+), which has no error
return - unlike reading `crypto/rand.Reader` directly - so `Create` and
`AddPanel` never carry an unreachable-in-tests error branch for it.
Deterministic testing does not mean a fixed ID: every test captures what
`Create`/`AddPanel` returns and asserts a later read echoes the same value
back, never a hard-coded string.

JSON: only `internal/adapter/repository/sqlite` marshals/unmarshals `Args`

- `json.Marshal` on write (defaulting a nil map to `map[string]any{}` first,
  so a panel with no args round-trips as `{}`, not `null`), `json.Unmarshal`
  on read. `domain.Panel.Args` stays `map[string]any` all the way through
  `usecase`, exactly like `domain.Endpoint`'s schema-less counterparts.

`ORCHESTRA_DB_PATH`: `internal/infra/config.Load` returns
`ErrMissingDBPath` when it is unset or empty - `os.LookupEnv`, not
`os.Getenv`, so "unset" and "set to empty" are both rejected the same way.
No default, per the plan: "a platform that silently forgets is worse than
one that will not start." `pkg/app.Config.DBPath`, one layer in, stays
optional though - empty skips `pkg/app.checkWorkspaceStore` entirely - so
every `pkg/app` test written before workspaces (which builds `Config`
directly, never through `config.Load`) keeps passing unmodified;
`cmd/api` always has a non-empty one, because it will already have failed
in `config.Load` otherwise.

`WorkspaceStore.AddPanel(ctx, workspaceID string, p *domain.Panel)` takes a
pointer, not the value the plan's pseudocode shows - `domain.Panel` is 112
bytes, and `gocritic`'s `hugeParam` check rejects passing it by value, the
same reason `pkg/app.Config` (also 112 bytes) is already a pointer where an
earlier plan showed a value. The interface and its one implementation
(`internal/adapter/repository/sqlite.Store.AddPanel`) must agree exactly -
Go's structural typing leaves no way to keep the interface itself at a
value type.

`internal/adapter/repository/sqlite.Store.Delete` runs both `DELETE`s in a
transaction; a failing one calls `rollbackAndWrap(tx, cause)`, which returns
`errors.Join(cause, tx.Rollback())` rather than a `defer tx.Rollback()` that
discards the error - `errcheck`'s `check-blank: true` flags
`_ = tx.Rollback()`, correctly, since `Rollback` (unlike `Close`) is not
covered by errcheck's default exclusions. `errors.Join(cause, nil)` is just
`cause`, so the common case (rollback succeeds) reads the same as it would
unwrapped, and the whole function is one statement - no branch that only
the rare "rollback itself failed" case would reach and a test could not.
`Store.Close` similarly returns `errors.Join(s.db.Close())` rather than an
`if err != nil { return fmt.Errorf(...) }`, for the same reason and because
a bare `return s.db.Close()` is an unwrapped external error (`wrapcheck`).

**Consequences.** 91.5% statement coverage in
`internal/adapter/repository/sqlite` (guard minimum 90%), reached largely by
opening a second, raw `database/sql` connection to the same file in tests to
put the database into shapes the `Store`'s own API can never produce itself
(a dropped table, a row with malformed JSON) - the only way to exercise a
database error path without a fake driver. `TestStorePersistsAcrossOpens`
proves AC-W-105's storage half: a second `Store` opened on the same
`t.TempDir()` file sees the workspace and panel the first one wrote.

Making `ORCHESTRA_DB_PATH` required broke two harness-owned browser gates
that were not part of this task's file list and cannot be edited
(`harness/quality/browser/playwright.config.ts` starts
`services/platform/bin/api` without it): `guard-a11y` and `guard-layout`.
`e2e/playwright.config.ts` and `e2e/src/orchestration.test.ts` - both
unprotected - were updated to set it (a file in a fresh `mkdtempSync`
directory each run, so no suite sees another's workspaces), which keeps
`acceptance-e2e` and `acceptance-browser` green. The harness file needs the
same one-line addition, but that is a protected path change this agent is
not authorised to make; see `TODO.md`.

## 2026-09-11 Workspaces Task 7: restarting the platform mid-test, and the drawer's own staleness

**Context.** AC-W-105 (a workspace survives a restart of the platform) needs a
process-level test that stops and starts the built `platform` binary itself,
not just the browser or the page - `e2e/src/orchestration.test.ts`'s helpers
(`freePort`, `startBinary`, `stop`, `waitForReady`) start their processes once
in `beforeAll` and stop them once in `afterAll`; nothing there restarts one
mid-test. Separately, the browser journey (ask → save → reopen) hit a real
product behaviour: `web/src/features/workspaces/model/useWorkspaces.ts`, which
the drawer uses to list workspaces, loads once on mount and is never told
about a workspace `SaveToWorkspaceControl` created through its own, separate
`useSaveToWorkspace` hook - so a freshly-saved workspace does not appear in the
drawer without some kind of reload.

**Decision.** `e2e/src/workspaces.test.ts` is a sibling file, not a change to
`orchestration.test.ts`: it duplicates the same handful of small process
helpers rather than importing them, because this suite's shape is
genuinely different (it starts a _second_ platform process against the
_same_ `ORCHESTRA_DB_PATH` file mid-test, after stopping the first), and
`make guard-duplication` only scans `web/src` - there is no guard this
would fail either way. It keeps its own `inventory` service running
across both platform starts, since `pkg/app.build` fetches the catalogue
at startup and fails to start at all if a configured service is
unreachable - so what survives the restart is the workspace's row in
SQLite, not the platform having to run without its catalogue.

For the browser journey (`e2e/browser/workspace.spec.ts`), "reopen the
workspace" is written as a real `page.reload()` after saving, the same
thing a person returning later would do, rather than reaching into the
app's state to force the drawer to refetch. This is the honest way to
prove AC-W-101/102 end to end without changing product code that Task 7
was not scoped to touch; `useWorkspaces` re-fetching live is not a defect
this task's acceptance criteria ask for.

**Consequences.** `e2e/src/workspaces.test.ts` only pushes its _second_
platform process onto the array `afterAll` cleans up - the first is
stopped inline, mid-test, and `stop`'s `once("exit", ...)` listener never
fires for a process that has already exited by the time it is attached,
which hung the suite for the full hook timeout before this was noticed.

## 2026-09-11 Auth Task 0: three store types instead of three methods on one, and where the admin gets seeded

**Context.** `docs/plans/auth.md` Task 0 asks for `usecase.SessionStore` and
`usecase.PermissionStore`, implemented by `internal/adapter/repository/sqlite`
alongside the existing `WorkspaceStore` implementation - the same package,
the same SQLite file. `usecase.SessionStore` names a method `Delete(ctx,
token string) error`; the existing `Store` (workspaces) already has a
`Delete(ctx, id string) error` of its own. Go does not allow two methods of
the same name on one receiver, so `Store` itself cannot implement both
`WorkspaceStore` and `SessionStore` no matter how the file is organised.

Separately: `local.New` (the admin-seeding entry point) is called from
`pkg/app.build`, but nothing in the object graph consumes the
`Authenticator`, `SessionStore` or `PermissionStore` it could hand back
until Task 2 wires `/api/session` - `docs/plans/auth.md` says explicitly
that Task 0 has no HTTP. A Go local variable that is never read is a
compile error, not a lint warning, so `build` cannot simply construct and
discard these values by name.

**Decision.** Three new types in the `sqlite` package - `Users`, `Sessions`,
`Permissions` - each with its own `New(path string)`, distinct from `Store`.
`openDB` (the "open, set the connection limit, apply the schema" sequence,
previously inlined in `Store.New`) is factored out and shared by all four
constructors, so each type still opens its own `*sql.DB` against the same
file path rather than sharing one connection object - simpler than plumbing
a shared handle through four constructors, and modernc.org/sqlite already
serialises writes at the file level regardless (see `Store`'s own doc
comment on `maxOpenConns`). `Users.SeedAdminIfNone` generates the admin's id
itself (via the package's existing `newID`, unexported, so only this
package can call it) rather than taking one as a parameter - keeping id
generation in one place, the same as every other row this package creates.

`pkg/app.seedAdmin` is a small function called from `build`, right after
`newWorkspaceHandler`, that opens `sqlite.Users` and calls `local.New` -
returning only the error. It exists purely for the side effect of having
accounts ready once a person can sign in in Task 2; nothing keeps the
`*local.Authenticator` it builds, because nothing yet has anywhere to put
it. `AdminPassword` on `pkg/app.Config` mirrors `DBPath`'s own optionality
(empty skips seeding) rather than being independently required at this
layer - `internal/infra/config.Load` is the one seam that enforces
`ORCHESTRA_ADMIN_PASSWORD` non-empty, the same as it already does for
`ORCHESTRA_DB_PATH`.

argon2id parameters (64 MiB, t=1, p=4, 32-byte output) come from OWASP's
password-storage cheat sheet's argon2id baseline, which RFC 9106 §4
recommends for the same profile this platform is in: a single server with
no secondary defence (no separate pepper, no rate-limiting hardware). Not
argon2's own package-level defaults, which target interactive key
derivation rather than a password store. Salt comes from `crypto/rand.Text()`
(Go 1.24+, no error return), the same source `sqlite.newID` already uses,
rather than `crypto/rand.Read` into a byte slice - one less error path to
handle or test, for the same amount of entropy.

**Consequences.** `local.Authenticator`, `sqlite.Sessions` and
`sqlite.Permissions` are all fully implemented and unit-tested against a
real SQLite file (`t.TempDir()`), but nothing routes an HTTP request to any
of them yet - `docs/plans/auth.md` Task 2 is what will change `seedAdmin`'s
shape, most likely into something that keeps the `Authenticator` and hands
it to a new session handler instead of discarding it. Verified live outside
the test suite too: `services/platform/bin/api` started twice against the
same `ORCHESTRA_DB_PATH` file with two different `ORCHESTRA_ADMIN_PASSWORD`
values on the two runs - the admin seeded on the first run is still there,
under its first password, on the second.

## 2026-09-12 Signing in (docs/plans/auth.md, Task 2)

**Context.** Every route but `GET /api/health` must answer 401 without a
session (docs/specs/auth.md, A2, section 6). That includes every route
`harness/quality/browser/a11y.spec.ts` and `layout.spec.ts` measure: both
navigate to `"/"` before a11y-scanning or layout-measuring whatever loaded
there. `"/"` itself is the built frontend, served unauthenticated by
`internal/infra/httpserver.NewRouter`'s static-file branch (only `/api/...`
goes through the new session middleware) - so the HTML, JS and CSS still
load fine. But the chat screen that HTML boots - the only screen there is;
Task 4 has not built a sign-in screen yet - immediately calls the platform's
API to do anything useful, and every one of those calls now comes back 401.
Nothing in `docs/plans/auth.md` Task 2's own scope fixes that: Task 4 builds
the sign-in screen, Task 6 repairs `e2e/`. Left alone, these two harness
gates would measure a screen stuck in a 401 error state instead of the real
one.

Three ways to keep them measuring something real:

1. Have the gates sign in first, so the screen they measure is the same one
   a signed-in person already sees today.
2. Exempt static file serving from auth - already true; it does not help,
   since the 401s come from the API calls the loaded page makes, not from
   loading the page itself.
3. Something else - skip the gates for Task 2, or gate a different path.

**Decision.** Option 1. Added `harness/quality/browser/session.ts`,
exporting `signInAsAdmin(page)`: one `page.request.post("/api/session")`
with `{ name: "admin", password: "guard-admin-password" }` - the same
account `playwright.config.ts`'s own `ORCHESTRA_ADMIN_PASSWORD` already
seeds for these gates - using Playwright's `APIRequestContext`, which
shares its cookie jar with `page`, so the session cookie the response sets
is already attached to every following `page.goto`. Both spec files call it
as the first line of every test, before their own `page.goto("/")`. This is
a harness change (`harness/quality/browser/` is a protected path) -
committed with `ORCHESTRA_ALLOW_HARNESS_CHANGE=1`, flagged in Task 2's own
report rather than made silently.

Not option 3: skipping a required gate is a weaker answer than keeping it
green on the real screen, and `a11y.spec.ts`'s `PUBLIC_PAGES` already names
`"/"` `"sign in"` - forward-looking to Task 4, where a visitor with no
session really does reach a sign-in screen there. Task 2 does not build
that screen, so signing in first is the only way to keep measuring the
screen that exists today rather than one that does not yet.

**Consequences.** Temporary, and said so in `session.ts`'s own doc comment:
once Task 4 lands a sign-in screen at `"/"`, this call stops being "sign in
before measuring the public page" and starts being "measure a screen nobody
without a session sees" - the opposite of what `PUBLIC_PAGES` asks for.
Whoever lands Task 4 should remove `signInAsAdmin` from both files'
`page.goto("/")` calls, and add a deliberately-named gate for any
signed-in-only screen that still needs one.

## 2026-09-12 Closing the nil-store auth bypass

**Context.** `internal/infra/httpserver.requireSession` had a branch: when
built with a nil `SessionUsers` store, it ran every request as a fixed
stub admin instead of ever refusing one. `pkg/app.build` only ever passed
a nil store when `Config.DBPath` was empty (`newAuth`'s own former nil
case) - and the shipped binary was safe only because
`internal/infra/config.Load` already refuses to start `cmd/api` without
`ORCHESTRA_DB_PATH` (`config.ErrMissingDBPath`). Nothing enforced the same
rule on `pkg/app.New` itself: any other caller - a future `cmd/`, a test
harness, anything that builds a `Config` without going through
`config.Load` first - that forgot to set `DBPath` got a wide-open admin
backdoor on every route, silently. `docs/specs/auth.md` section A6 already
rejected this shape of reasoning once: "a check that can be forgotten will
be forgotten."

**Decision.** `pkg/app.New` (via `build`) now refuses to build a handler at
all when `Config.DBPath` is empty, returning the new `app.ErrMissingDBPath`
sentinel - written test-first
(`services/platform/pkg/app/app_test.go`'s `TestNewFailsWhenDBPathIsEmpty`,
confirmed red before `ErrMissingDBPath` existed, green after). There is no
legitimate use of this platform without a database: workspaces and
accounts both need one. With that guaranteed, `requireSession`'s
`store == nil` branch became unreachable and was deleted outright, along
with the `newStubAdmin` helper it alone used - not left in place as
defensive dead code, since a check that can never fire is exactly the kind
of thing that invites someone to reintroduce a caller that skips it.
`newWorkspaceHandler`, `newPermissionStore` and `newAuth` in `pkg/app/app.go`
lost their own `dbPath == ""` branches for the same reason: DBPath is now
always non-empty by the time they run.

This touched roughly fifteen `app.New(&app.Config{...})` call sites across
`pkg/app/app_test.go` (7), `acceptance/api_test.go` (1),
`acceptance/invoke_test.go` (3) and `acceptance/plan_test.go` (5), plus
`pkg/app/internal_test.go`'s `TestBuildWrapsRouterError` and three
`httpserver.NewRouter(..., nil)` calls in
`internal/infra/httpserver/router_test.go`. Every acceptance test that now
drives a route other than `GET /api/health` needs a session first
(`docs/specs/auth.md`, AC-A-102) - `session_test.go` and
`workspace_test.go` already had that pattern (a `t.TempDir()` database,
sign in as admin, carry the cookie in a jar), so it was extracted into
`acceptance/helpers_test.go` as `newTestApp` (builds the platform, seeds
and signs in as the shared `adminPassword`) rather than copied into
`invoke_test.go` and `plan_test.go` separately.

**Where the shared helper lives, and why.** `acceptance/helpers_test.go`,
a new file in the `acceptance_test` package, rather than a new subpackage
or a `t.Helper()` added to an existing file. Go test files in one package
already share unexported declarations freely (`doJSON`, defined in
`workspace_test.go`, was already being called from `session_test.go`
without any import) - so a same-package file, not a new module or package,
is the natural home once more than one file needs the same setup. It is
not folded into `workspace_test.go` (where `adminPassword` used to live)
because that file is about workspace behaviour specifically, and every
other acceptance file would then depend on a name owned by an unrelated
test file's own concerns; a dedicated `helpers_test.go` says "this is
shared plumbing" the way `session_test.go` and `plan_test.go` say "this is
what I'm testing." `workspace_test.go`'s own `newWorkspaceTestApp` was
rewritten to call the new `newTestApp` too, so the sign-in pattern now has
exactly one implementation, not two that happen to agree.

`api_test.go`'s `TestHealthEndpointReportsOK` deliberately does _not_ use
`newTestApp`: it only ever calls the one route exempt from `requireSession`
(`GET /api/health`), so forcing it through a sign-in it does not need would
make the test depend on session machinery it is not about.

`newTestApp` takes `*app.Config`, not `app.Config`, to satisfy
golangci-lint's `gocritic` `hugeParam` check (`harness/quality/go/golangci.yml`)

- the same reason `app.New` itself takes a pointer.

Also removed: `TestRequireSessionWithNoStoreRunsEveryRequestAsTheStubAdminAndNeverBlocks`
(`session_internal_test.go`) - it tested exactly the bypass this decision
closes, so keeping it green would have meant keeping the bypass. The three
`router_test.go` cases that used to pass a nil store now pass a small
`fakeSessionUsers` fixture instead (`NewRouter`'s `sessions` parameter must
never be nil in production now); `TestNewRouterRejectsUnknownRoute` signs
in with it first, since an unknown `/api/` route is gated by
`requireSession` like any other route now (it used to reach the mux's own
404 only because the old nil-store bypass let it through unauthenticated).

**Consequences.** `e2e/` was not touched (someone else's task, and its
existing suites already sign in or are expected red per Task 2/Task 6) -
`acceptance-e2e` and `acceptance-browser` remain the only two `make check`
targets allowed to fail, unchanged from before this decision. No real LLM
call is made by any of this: `docker logs llama-swap`'s
`POST /v1/chat/completions` count was identical before and after every
test run this decision's tests exercise.

## 2026-09-12 Auth Task 6: end to end

**Context.** `docs/plans/auth.md`'s last task: make the existing e2e/browser
suites sign in (they were failing on 401 since Task 2), and write the
process-level and browser journeys proving AC-A-103, AC-A-104 and AC-A-105
end to end. Two things this task needed had no answer yet: how a
process-level suite driving a _built binary_ carries a session cookie
across requests (Go's acceptance tests already had `net/http/cookiejar`;
Node's `fetch` keeps none), and how to get a non-admin account into that
binary's database at all, since `docs/specs/auth.md` section 8 keeps
account creation out of both the UI and the API.

**Decision 1: a cookie-carrying `fetch` helper.** `e2e/src/helpers/auth.ts`
exports `signIn(baseUrl, name, password)` (posts `/api/session`, reads the
`Set-Cookie` response header by hand, returns `{ cookie }`) and
`withSession(session, init?)` (merges that cookie into a `RequestInit` via
`new Headers(init.headers)` - spreading `init.headers` directly trips
`no-misused-spread`, since `HeadersInit` can also be an array of tuples or
a `Headers` instance, neither of which spreads onto a plain object the way
a caller would expect). One implementation, shared by `orchestration.test.ts`,
`workspaces.test.ts` and the new `auth.test.ts`, rather than three copies of
the same `Set-Cookie` parsing.

**Decision 2: `ORCHESTRA_SEED_ACCOUNTS`, a new environment variable, mirroring
`ORCHESTRA_PLAN_FIXTURES` exactly.** `pkg/app.Config.SeedAccounts` already
existed (Task 3's own acceptance tests use it directly, being Go code that
can build a `pkg/app.Config` by hand) with a doc comment saying cmd/api
never sets it. A process-level e2e suite starts the _built_ `services/platform/bin/api`
binary, though, which has no access to `pkg/app.Config` at all - only to
`cmd/api`'s environment. The same gap already existed for the stub
planner's fixture table, and `ORCHESTRA_PLAN_FIXTURES` is exactly the
precedent this follows: a JSON-encoded array, decoded in
`internal/infra/config.Load` into a new `config.SeedAccount` slice,
wired through `cmd/api/main.go`'s `toAppSeedAccounts` into
`app.Config.SeedAccounts`, and never set by anything other than a test.
Test-first: `config_test.go` gained
`TestLoadSeedAccountsDefaultsToEmpty`/`TestLoadParsesSeedAccounts`/
`TestLoadRejectsMalformedSeedAccounts` before the field, the parser and the
wiring existed, confirmed red, then green.

This does **not** reopen `docs/specs/auth.md` section 8's exclusion ("no
account creation through the UI"). Nothing over HTTP creates an account -
`ORCHESTRA_SEED_ACCOUNTS` is read once, before the process ever serves a
request, the same moment `ORCHESTRA_ADMIN_PASSWORD` already seeds the first
admin. Section 8 was reworded slightly ("through the UI or the API") to
name what actually stays excluded, since the previous wording ("through the
UI") could be misread as leaving a backdoor through the API instead - it
never did, and now says so. No behaviour changed; the doc comments on
`pkg/app.Config.SeedAccounts` and `seedAccounts` (`pkg/app/app.go`) were
updated to describe the new cmd/api path alongside the existing direct one.

**Decision 3: the browser suites share one sign-in helper and one admin
password constant.** `e2e/browser/helpers/auth.ts`'s `signIn`/`signInAsAdmin`
drive the real sign-in form (fill name/password, click submit, wait for
the チャット heading) rather than reaching around it - these are
browser-driven specs, and the sign-in screen is exactly what Task 4 built
for a person to use. `e2e/browser/helpers/constants.ts` holds
`ADMIN_PASSWORD` once, imported by both `e2e/playwright.config.ts`'s
`webServer` env and `signInAsAdmin`, so the two cannot drift into two
different literals that happen to still match by luck. `chat.spec.ts` and
`workspace.spec.ts` were each missing exactly one line: their opening
`page.goto("/")` became `await signInAsAdmin(page)`, since `AuthGate` now
shows the sign-in screen instead of the chat until a session exists
(AC-A-102). The new `auth.spec.ts` drives the journey Task 6's own plan
names for the browser: sign in, see the chat, sign out, and a reload still
shows the sign-in screen (AC-A-107).

**Decision 4: `ORCHESTRA_SECURE_COOKIE=false` reaches every e2e/browser
process.** `525da74` (`docs/plans/auth.md`, "let the session cookie reach a
link TLS did not secure") made the session cookie's `Secure` attribute
configurable, default on. Every e2e/browser suite runs the platform over
plain `http://127.0.0.1`, where a `Secure` cookie is stored by nobody - a
sign-in would appear to succeed and every subsequent request would arrive
with no session at all, a much more confusing failure than 401 from the
first request. Added to `e2e/playwright.config.ts`'s `webServer` env and to
both process-level suites' own platform-starting env
(`orchestration.test.ts`, `workspaces.test.ts`, `auth.test.ts`).
`harness/quality/browser/playwright.config.ts` (the harness's own `guard-a11y`/
`guard-layout` gates) was deliberately left alone: those gates measure the
sign-in screen itself (nobody signs in there, on purpose, per
`docs/plans/auth.md`'s own Task 4 note), so no session is ever set for a
`Secure` attribute to matter to.

**Decision 5: the three process-level suites now share `helpers/process.ts`
and `helpers/wire.ts`.** `orchestration.test.ts` and `workspaces.test.ts`
each carried their own copy of `freePort`/`waitForReady`/`startBinary`/`stop`;
a comment in `workspaces.test.ts` had argued this was fine for two files.
Adding a third (`auth.test.ts`) made that argument's own math work out
differently - three copies is real duplication, and `auth.test.ts` on its
own tripped `eslint(max-lines)` (300) besides. Extracted verbatim into
`e2e/src/helpers/process.ts`; all three suites import it now.
`e2e/src/helpers/wire.ts` extracts `isRecord`/`asRecord`, a runtime
narrowing pattern each of these three files already used privately, into
a fourth, small module - which is what let `auth.test.ts` fit under the
line-count ceiling without giving up how it parses a JSON response body.

**Verified.** `make -k check` was run to iterate, and `make check` (no `-k`,
after the last edit) was run once at the end and reported every gate
green, including `acceptance-e2e` and `acceptance-browser`, which had been
the only two allowed to fail since Task 2. `docker logs llama-swap 2>&1 |
grep -c 'POST /v1/chat/completions'` read the same count before this task's
first edit and after its last `make check` run - no real LLM call was ever
made. Ports 8080/8081/8082/5173 (the developer's own running processes)
and 11435 (llama-swap) were still listening afterwards.

**A mistake made and corrected during this task.** While confirming a
served operation id's casing, `pkill -f "bin/api"` was run to stop a
throwaway `curl` probe's own background process - `pkill` by pattern
matched every process with `bin/api` in its command line, which also
killed the developer's own inventory (8081) and attendance (8082)
processes, in violation of this task's own instruction never to touch
them. Both were restarted immediately, on the same ports, from the same
built binaries, before continuing. Noted here, and in the task's final
report, rather than left silent: the fix for next time is to `kill` a
specific PID captured at spawn time, never a broad `pkill -f` pattern,
anywhere near ports something else depends on.

## 2026-09-12 The conversation rides in the user message, not a second system one

**Context.** The turns had to go somewhere in the prompt, after the catalogue
so the cache stays warm for the part that does not change
(`docs/specs/context.md`, M3). A second `system` message between the fixed one
and the question is the obvious place: it is not the question, and it is not
the instructions.

It is also a 500. `qwen3.5-9b-q8`'s chat template refuses any system message
that is not the first, and llama.cpp answers the whole request with
`Jinja Exception: System message must be at the beginning`. Every run failed,
and `make check` never saw it - the transport tests talk to `httptest`, which
accepts any shape at all.

**Decision.** Render the turns into the user message, ahead of the question.
The fixed system prompt and the tools stay byte-identical across every request
in a conversation either way, so M3 holds; and one system message is what every
chat template tolerates.

**Consequences.** The prompt reads as one person speaking - what they asked
before, what the platform did about it, and what they are asking now - which is
arguably what it always was. The cost is that a template quirk of one model
shaped the design; the benefit is that it is the shape with the fewest ways to
be wrong elsewhere.

It is also the second time a live check caught what the test suite could not,
after the fifteen-second write timeout. A mocked transport proves the bytes are
assembled; only a model proves they are accepted.

## 2026-09-12 qwen3.5-9b-q8 carries a follow-up's context, measured by hand

**Context.** `docs/plans/context.md` Task 4, Step 3: `make check` never calls a
real LLM (`internal/adapter/planner/stub` answers every test and every
`acceptance-e2e`/`acceptance-browser` fixture), so AC-M-101 - a follow-up
question naming no service is answered from the service the previous one used

- has no automated proof against an actual model. It can only be measured by
  hand against the platform already running under `make dev-platform` (port
  8080, `qwen3.5-9b-q8` via llama-swap on 11435), the same way the planner
  comparisons in this file's earlier entries were.

Five `POST /api/plan` calls were made against the running platform, each
carrying one earlier turn (`{"service": "inventory", "operationId":
"ListInventoryItems"}`, from having just asked "在庫の一覧を見せて") and a
follow-up phrased to give the model no service clue at all: "他にはある？"
("anything else?" - no item name, no status word, nothing an operation's
description could match on its own).

**Decision.** No code change - this is a measurement, recorded because
`docs/plans/context.md` asks for one, not a fix. The result: **5/5** stayed on
the inventory service the previous turn named. Four of the five resolved
outright (`kind: "result"`, `source: {service: "inventory", operationId:
"ListInventoryItems"}`); the fifth came back as `kind: "ask"` for the `status`
parameter, with a question in Japanese about which _inventory_ status to
show - still routed to the right service and the right operation, just short
one argument the model judged genuinely missing. None of the five degraded to
`list_capabilities` or answered from `attendance`, the wrong-service failure
`docs/plans/orchestration.md`'s own planner comparisons had seen on a
context-free question with this same model.

A second, easier phrasing ("検品保留のものだけ見せて", which names an enum
label - "quarantined" - that only `inventory`'s `status` property has) was
tried first and went 5/5 to a clean `result` every time; it is a weaker test
of context specifically, since the wording alone narrows the service, so the
harder, clue-free phrasing above is the one this entry counts.

**Consequences.** `qwen3.5-9b-q8` carries a one-turn conversation well enough
for AC-M-101's own example (a follow-up after an inventory question) to work
in production, without inventing a service or falling back to
`list_capabilities` the way the leaner comparisons in this file's earlier
entries warned it could on a context-free question. This is a finding about
today's model, not a guarantee: a longer conversation, a follow-up further
from ORCHESTRA_CONTEXT_TURNS's edge, or a different model swapped in later
each want their own measurement here rather than an assumption borrowed from
this one. The platform's own part - carrying `turns` from the wire to the
prompt untouched - is what `e2e/src/context.test.ts` and
`e2e/browser/context.spec.ts` fix with the stub planner instead, since that
part does not need a model to be right.

## 2026-09-12 The eval suite (docs/specs/eval.md) lives in e2e/eval/, N=10, tolerance=0.3

**Context.** docs/specs/eval.md asks for a fifth subproject that measures the
real planner's behaviour as a rate, against a committed baseline, without
ever running inside `make check` (E4). Building it meant deciding where the
code lives, how many times to run each case, and how large a drop from the
baseline counts as noise rather than a regression - none of which the spec
settles, and none of which should be guessed.

**Location.** `e2e/eval/`, not a new top-level `eval/` package. Reasons: (1)
`.gitignore` is deny-by-default (section 3 lists the trees that are tracked
at all) - a new top-level directory would need a new allow rule, and this
repository's own history has one story of an allow-rule accident swallowing
a source tree (`**/users/`, `.gitignore`'s own comment). `e2e/**` is already
allowed. (2) The suite needs exactly what `e2e/src/*.test.ts` already have -
`freePort`/`startBinary`/`waitForReady`/`stop` (`e2e/src/helpers/process.ts`)
and `signIn`/`withSession` (`e2e/src/helpers/auth.ts`) - and importing them
from a sibling package would need a second pnpm workspace entry for no
benefit. (3) `harness/quality/oxlint/policy.ts` already turns
`eslint/no-console` off for `files: ["harness/**", "e2e/**"]` - console
output being pre-approved for this tree is a signal that tooling living
under `e2e/` (not test files) was already anticipated. The one thing kept
separate from `e2e/src/`: `e2e/vite.config.ts`'s `test.include` is
`src/**/*.test.ts`, so nothing under `e2e/eval/` is ever collected by
`acceptance-e2e` (`make check`) no matter its filename - AC-E-202 does not
depend on a human remembering not to name a file `*.test.ts`.

`e2e/eval/*.ts` are plain scripts run directly by `node` (`node
e2e/eval/run.ts`), the same way `harness/guard/fsd.ts` and
`harness/guard/duplication.ts` already are - Node 24's unflagged type
stripping, already relied on there, needing no new tooling. `e2e/tsconfig.json`
gained `"eval"` in its `include` so `make check`'s own `web-lint` (Oxlint,
type-aware) still catches a mistake in this suite by static analysis alone -
static analysis is not "calling a model" (AC-E-202).

**Operation ids.** The corpus (`e2e/eval/cases.ts`) writes `ListInventoryItems`/
`CreateInventoryItem`, not the `listInventoryItems`/`createInventoryItem` that
`services/inventory/api/openapi.yaml` declares on disk. Traced by decoding
`services/inventory/internal/adapter/openapi/openapi.gen.go`'s embedded
spec directly: oapi-codegen renames every operationId in its _embedded_ copy
of the spec to the Go identifier it generated (`ListInventoryItems`), without
touching the source file. The platform's catalogue is built from `GET
/openapi.yaml` - what a service actually serves - never from the file on
disk, so the renamed spelling is what a real run resolves against. This
also resolves what looked like a mismatch between the source `.yaml` and
every existing fixture in `services/platform/internal/**/*_test.go` (all of
which already write the capitalised form) - they were right, and reading
only the source file was the error.

**N and tolerance, measured, not guessed.** Every case in the corpus was run
ten times against `qwen3.5-9b-q8` (2026-09-12, this instance's llama-swap),
twice, from a fresh platform each time. Six of the seven held at 10/10 or
5/5 across every sample (`filter-by-label`, `list-everything`, `create`,
`unanswerable`, `capability`, `follow-up-stays`). The seventh, the enum-less
filter (`破損した在庫はある？`), is the one docs/specs/eval.md exists for
and swung: reject (the silent "no filter, every row" drop this suite watches
for, per section 3's own example) came out 2/10 and then 3/10 across two
ten-run samples of the same unchanged binary, with accept correspondingly
4/10 and 5/10-7/10 depending on the sample. A binomial rate this uncertain
at n=10 has roughly a 30-point-wide two-sample-deviation band around its own
true value - smaller than that and a run that changed nothing would fail
`make eval` on pure noise; catching a real regression (the filter starting
to drop outright rather than merely wobbling) does not need much finer than
that. `ORCHESTRA_EVAL_N=10` and `ORCHESTRA_EVAL_TOLERANCE=0.3` are the
result, both overridable by environment variable for whoever wants to spend
more wall time on a tighter measurement later. One full `make eval` run (70
calls: 7 cases x 10) took about three minutes end to end including `make
build`, comfortably inside "realistic to run deliberately".

**Consequences.** `Makefile` gained `eval`/`eval-accept` (harness change,
`ORCHESTRA_ALLOW_HARNESS_CHANGE=1`) - neither is a dependency of `check` or
`acceptance`. `e2e/eval/baseline.json` is committed (AC-E-204's baseline a
reviewer can read without a GPU, docs/specs/eval.md section 6) and was
verified, by dry-run `git add`, to not fall through `.gitignore`'s
deny-by-default rules the way an unnoticed new directory could.

## 2026-09-12 no-enum-value judged on reject, and run at n=30

**Context.** The previous entry's tolerance (0.3) was sized from two ten-run
samples of `no-enum-value`'s accept rate. A third, unrelated observation
(5/10, 4/10, then 2/10 accept across three ten-run samples of the same
unchanged binary) put a sample outside that band - the tolerance was too
tight for the thing it was actually measuring, and `make eval` would either
cry wolf on a run that changed nothing or, sized wider to compensate, miss a
real regression. What `no-enum-value` exists to watch (docs/specs/eval.md
section 3) is not which defensible answer the model picks - asking for the
missing enum value versus guessing the closest one - it is whether the
specific wrong outcome, silently dropping the filter and returning every
row, gets more common. Judging the case by `accept` was measuring the wrong
split.

**Decision.** Cases now name which rate they are judged on: `Case.metric`
(`e2e/eval/types.ts`), `"accept"` (default, unchanged for the other six
cases) or `"reject"`. `report.ts` compares whichever rate `metric` names
against its own baseline count, in the direction that means "worse" for that
metric - down for `accept`, _up_ for `reject` - and marks the line
`(judged: reject)` so the report says which comparison produced a
REGRESSION/improved trailer without changing the six unaffected lines'
shape. `no-enum-value` is now `metric: "reject"` (`e2e/eval/cases.ts`).

Measured before deciding, not assumed: switching to `reject` alone did not
fix the noise. Six ten-run samples of `no-enum-value` (2026-09-12, same
unchanged binary, fresh platform each time) read reject 6, 5, 7, 5, 6, 9 out
of 10 - a 5-9/10 band exactly as wide as accept's own wobble, because accept
and reject are complements of each other once "neither" is rare. Raising N
is what narrowed it: three thirty-run samples read reject 16, 19, 19 out of
30 (0.53-0.63), less than half the earlier band's width, matching
`sqrt(p(1-p)/n)` shrinking with n. `Case.runs` (`e2e/eval/types.ts`) lets one
case override `run.ts`'s default N; `no-enum-value` sets `runs: 30`
(`e2e/eval/cases.ts`) and nothing else does, because the other six already
hold to 10/10 or 5/5 - paying 3x the wall time on every case to fix the one
that needed it would be the wrong trade. `ORCHESTRA_EVAL_TOLERANCE` stays a
single number (0.3) rather than one per case: it was sized generously enough
in the original measurement that it still comfortably covers the tighter
n=30 band above, with room before a real regression would need to clear it,
and a second per-case knob was not worth adding without evidence the shared
one had stopped working - which, at n=30, it had not.

`e2e/eval/baseline.ts`'s `BaselineEntry` now records both `accept` and
`reject` counts unconditionally (`e2e/eval/baseline.json` rewritten by
`make eval-accept` to match), so a case is free to switch which one it is
judged on later without losing the count it had been ignoring.

**Consequences.** `e2e/eval/types.ts`, `cases.ts`, `baseline.ts`, `run.ts`,
`report.ts` changed; `docs/specs/eval.md` sections 2-4 and 7 (E2/E3,
AC-E-201/AC-E-203) describe `metric` and the flipped regression direction.
No harness or `Makefile` change was needed. Verified: two consecutive
`make eval` runs against the rewritten baseline both passed with no
regression reported (same unchanged code); forcing `no-enum-value`'s
baseline reject to 0 made the next `make eval` fail with `← REGRESSION,
baseline 0/30 reject` and a non-zero exit, then the baseline was restored.
`make check` was re-run after the last edit and stayed green with zero real
model calls - `make eval` is still not one of its targets.

## 2026-09-12 asking for everything is a thing the model must say (D15)

**Context.** A list operation's enum filter is optional in its contract,
and that is correct - `GET /api/inventory/items` with no `status` returns
every item, and every item is a legitimate answer to "在庫を全部見せて". But
an optional parameter gives the model two ways to say nothing at all, and
the schema cannot tell them apart: "the person asked for everything" and "I
could not match the word they used against any declared value" both come
out as the same omitted argument. 破損した在庫はある？ has no declared
inventory status anywhere close to 破損, and when the model cannot match it
the schema lets it decline silently by leaving `status` out - the person is
handed every row, presented as the answer to a question about one kind of
row, with nothing on screen saying a filter went missing. Measured, not
assumed: on the default local model (`qwen3.5-9b-q8`), the eval corpus's
`no-enum-value` case (破損した在庫はある？) reached that wrong outcome 18 of
30 runs, and `no-enum-value-attendance` (有給の勤怠はある？) 9 of 10 - the
worse number is the telling one, since 有給 has no near neighbour among
attendance's own enum values (みなし労働/振替休日/代休/待機) for the model to
guess at instead, so the model is not guessing badly, it is declining to
answer in the one way the schema lets it decline silently.

**Decision.** A list of Japanese words to watch for was rejected outright -
it would have to be written per service, would need updating every time an
operator added a status, and would still miss whatever word the corpus
had not thought of. Instead, take away the option of saying nothing at all:
on a safe endpoint (`domain.Endpoint.IsSafe()` - GET, HEAD, QUERY), an
optional enum parameter is offered to the model as _required_, with one
value its contract does not have, `domain.EnumAllValue` (`__all__`),
labelled `domain.EnumAllLabel` (すべて) the same way every other enum value
is labelled - both in the description suffix and in the structured
`enumLabels` map. Omitting the parameter is then not a legal answer at all;
the model has exactly three choices left - name a declared value, name
`__all__`, or reach for `ask_user` (D11) - and the silent drop has no shape
left to take. `__all__` is a request for every row, which is what an absent
filter already meant, so the platform strips it rather than passing it on:
`usecase.stripSyntheticAll` (`internal/usecase/orchestrator.go`) removes
the argument after the catalogue lookup and before anything is validated
or called - which is also before a result's provenance is built - on both
paths that reach a service (`call`'s safe branch and `Invoke`), so
`source.args` reads `{}` exactly as it did when the model omitted the
parameter, no service ever learns the synthetic value exists, and a
workspace panel saved from the result holds no `__all__` to replay.

Two further boundaries, both load-bearing: the synthetic value is added
only to `e.Parameters`, never to a request body's own properties (a
create's `status` is a field of the thing being created, and "all" is not
a status an item can be in), and building the emitted JSON Schema never
mutates the `domain.Schema` the catalogue holds - `usecase.WithSyntheticAll`
returns a copy - because `domain.Catalog.Find` and the `ask_user` path
(`optionsForParam`/`optionsFromSchema`) read that same value directly and
must never offer a person a "すべて" choice alongside the contract's own
declared ones.

Both planners needed checking, not just one, because the catalogue is
built once but rendered two ways (`docs/specs/context.md` M5): `toolcall`
reads `usecase.ToolsFor`'s schemas directly and inherited the fix for free.
`jsonmode` does not - it renders its own catalogue text and validates enum
values straight from `domain.Endpoint` (`argSchemas`,
`internal/adapter/planner/jsonmode/planner.go`), so it needed its own call
into the same `usecase.EnumParamGetsSyntheticAll`/`usecase.WithSyntheticAll`
helpers `tools.go` uses, exported from `usecase` specifically so the two
planners could not independently decide this differently. `jsonmode` has no
structural enforcement of "required" at all (no schema-level required list
reaches its model), so the synthetic value's practical effect there is
narrower than for `toolcall`'s `strict: true` tool calls - it offers `__all__`
as a nameable value and accepts it in `validateArgs`, but nothing stops the
model from omitting `status` anyway the way it always could. That gap is
inherent to the transport, not something this change could close without
redesigning `jsonmode` itself, so it is recorded here rather than papered
over.

One side effect, not a regression: `ListInventoryItems`'s optional `status`
parameter becoming required means every property that tool declares is now
required, which flips its `strict` field from `false` to `true` in
`toolcall.shapeTool`'s output (`everyPropertyRequired` had nothing left to
find missing) - `internal/adapter/planner/toolcall/planner_test.go`'s
assertion was updated to match, since this is the schema doing exactly what
D15 asked of it.

**Consequences.** `internal/domain/catalog.go` (the two constants),
`internal/usecase/tools.go` (`EnumParamGetsSyntheticAll`,
`WithSyntheticAll`, and `inputSchemaFor`'s new branch),
`internal/usecase/orchestrator.go` (`stripSyntheticAll`/`isEnumParam`, and
the two call sites in `call` and `Invoke`), and
`internal/adapter/planner/jsonmode/planner.go` (`argSchemas`) changed.
`docs/specs/orchestration.md` D15 and section 8a describe the design.
Tests added across `internal/usecase/tools_test.go`,
`orchestrator_test.go`, `orchestrator_invoke_test.go`,
`internal/adapter/planner/jsonmode/planner_test.go`, and one
acceptance test (`services/platform/acceptance/plan_test.go`,
`TestPlanCallNamingTheSyntheticAllValueCarriesNoArgumentsInProvenance`)
covering the end-to-end shape: a planner decision naming `__all__` produces
a result whose provenance carries no arguments and whose actual HTTP
request to the service carries no `status` query parameter at all. `make
-k check` was run to completion and is green; `docker logs llama-swap
2>&1 | grep -c 'POST /v1/chat/completions'` read the same count before and
after this work - no real model call was made anywhere in `make check`.
The before/after eval measurement against `no-enum-value` and
`no-enum-value-attendance` is being run separately, outside this session.

**Correction, 2026-09-12, after the measurement came back.** The
before/after numbers this entry was waiting on:

```
                          before          after
no-enum-value             18/30 reject    20/30 reject     (n=30 noise band measured at 16-19)
no-enum-value-attendance   9/10 reject     5/10 reject     (n=10 noise band is 0.40 wide - not decisive)
no-enum-value (accept)    10/30           5/30
every other case          10/10 accept    10/10 accept     (no side effects at all)
```

**D15 did not reduce the defect.** `no-enum-value`'s reject rate (18→20)
sits inside its own noise band and is not a change; `no-enum-value-attendance`
moved but its n=10 band is 0.40 wide, too coarse to call decisive either
way. What did move, cleanly, is the accept rate: 10/30 down to 5/30. The
mechanism this entry describes is real - the model stopped omitting the
parameter - but what it says instead, half the time, is `__all__`, and the
person sees the same screen either way: every row, presented as the answer
to a question about one kind of row. Adding a value to the list tool made
calling that tool easier to reach for; `ask_user` is a different tool, and
got reached for _less_, not more. The reading is that `__all__` competes
with `ask_user` (D11) rather than reinforcing it, and this entry's original
reasoning - that taking away the silent-omission option would push the
model toward `ask_user` - did not hold.

**Next tried:** giving `__all__` a description that tells the model when it
is, and is not, the right answer (`usecase.syntheticAllInstruction`,
appended in `usecase.WithSyntheticAll`) - see `docs/specs/orchestration.md`
section 8a for the exact text and where it lands in both planners' rendered
output. If that description does not move the accept-rate number, **D15 is
reverted**: the synthetic `__all__` value, its stripping, and both
planners' handling of it come back out, and the silent-omission case goes
back to being handled some other way (a fresh decision, not this one).

**Third measurement, and withdrawal.** The instruction did not move the
number either:

```
                       before D15   D15      D15 + instruction
no-enum-value reject     18/30      20/30    16/30    (n=30 noise band measured at 16-19)
no-enum-value accept     10/30       5/30    10/30    (observed range 7-11)
attendance    reject      9/10       5/10     8/10    (n=10, band 0.40 wide - not decisive)
every other case         10/10      10/10    10/10    (no side effects, throughout)
```

The instruction brought the accept rate back to where it started (10/30,
recovering from the 5/30 the unqualified value caused) and left the reject
rate exactly where it was before D15 ever landed - inside the same noise
band, with the same third case (`attendance`) too coarse at n=10 to read
either way. Three measurements land on the same conclusion: adding the
value moves the accept rate around and never touches the reject rate. **D15
is reverted** - the synthetic `__all__` value, `stripSyntheticAll`,
`isEnumParam`, `EnumParamGetsSyntheticAll`, `WithSyntheticAll`, and both
planners' handling of it, along with section 8a of
`docs/specs/orchestration.md`, are removed. The one piece that survives is
independent of D15: `jsonmode.renderParam` now renders a parameter's own
`Description`, a real gap the D15 work happened to find.

What this failed experiment establishes: `ask_user` is a separate tool
competing with the operation's own tool, not a fallback the model reaches
for only when it has run out of other options. Adding a value to an
operation's own enum makes calling that operation _easier_, and every time
it got easier, `ask_user` got called _less_ - the accept rate is what moved,
in both directions, exactly in step with whether `__all__` was offered
unqualified or qualified. The reject rate never moved because the defect
those cases measure - the model returning every row when it cannot match a
filter word at all - was never actually about the enum lacking a value for
"everything"; it is about which tool the model reaches for when a word
matches nothing, and `__all__` never changed that choice. A future attempt
at this defect has to change the competition between `ask_user` and the
operation's own tool - through the tool's own description, through
`ask_user`'s own description, or through the decision procedure itself -
not add another value to the enum.

## 2026-09-12 An unsafe operation is answered by its form, not a question

**Context.** 「在庫を登録したい」 names `CreateInventoryItem` and no
arguments. The model reaches for `ask_user` on the first required field it
cannot fill in, and when that field happens to be `status` - a real enum -
the person is shown 「ステータスを選んでください」 instead of the create
form they asked for. The ask is not wrong so much as beside the point: it
offers one of the three fields a create needs, `name` and `quantity` were
never asked about so answering it cannot complete anything, and the form
that appears afterwards repeats `status` as a select over the same values
with the same labels - the same question, twice, the second time in a
control that could have been the whole interaction.

The mechanism was `Orchestrator.ask` degrading a `DecisionAsk` into a form
only when `optionsForParam` found no enum for the named parameter, and
`optionsForParam` searched an unsafe endpoint's request body properties as
well as its declared parameters - so `status` on `CreateInventoryItem` was
found there, options were built, and the result was `kind: "ask"` rather
than a form.

**Decision.** D8 already settled that an unsafe operation is never run by
the model: it is answered by a form in every other case, and a person
presses the button. `ask` now applies that before it ever looks at the
parameter: an endpoint that is not `Endpoint.IsSafe()` degrades straight to
`kind: "form"` (the same `Schema`/`Initial` the existing degradation path
already produced), whether or not the named parameter has an enum. There is
nothing left to disambiguate before running, because nothing runs. An ask
over a **safe** endpoint is a different question, unchanged: that one runs
immediately, so a wrong value is already on the screen before anybody could
object, which is what D11 was for - enum found still means `kind: "ask"`,
no enum still degrades to a form.

With every caller of `optionsForParam`'s request-body branch now routed
around it (an unsafe endpoint never reaches `optionsForParam` at all), the
branch had nothing left to search - deleted, along with its doc comment's
claim that it searches "for an unsafe endpoint, its request body's own
properties". `docs/specs/orchestration.md` D11 is amended with this
reasoning in place, and a new section 8b walks through the defect itself.

**Consequences.** The two degradation paths (an unsafe `DecisionCall`, and
now an `ask_user` naming an unsafe operation) are one idea rather than two:
a form is what an unsafe operation is answered with, full stop, and the
same form is what a safe operation's un-listable parameter degrades to.
`web/` needed no change - `TurnList.tsx` already dispatches purely on
`result.kind`, `ResultForm` already renders an enum request-body property
as a select, and no existing frontend test asserted `kind: "ask"` for an
unsafe/create operation (the one `"ask"` fixture in `Conversation.test.tsx`
and `ResultChoice.test.tsx` is `ListInventoryItems`, a safe read, and is
unaffected). Verified with `docker logs llama-swap`'s
`POST /v1/chat/completions` count unchanged across `make check`.

## 2026-09-12 — the eval corpus cannot adjudicate a prompt-sized change

**Context.** Three separate attempts were made at the defect where a filter
word matching no enum value is dropped silently and every row comes back:
D15's synthetic `__all__` value (withdrawn, see above), an instruction
appended to that parameter's own description (withdrawn with it), and a
paragraph added to both planners' system prompts saying what omitting an
optional parameter means ("Leaving an optional parameter out is itself an
answer: it means the user asked for every row"). Each was measured against
the corpus, before and after.

**What the numbers actually say.** `no-enum-value`'s reject count, at
n=30, across every measurement taken that day — under D15, under the
description experiment, after the revert, and under the system-prompt
paragraph, all of which measured as the same behaviour:

    16, 19, 19, 19, 18, 20, 16, 15, 22   / 30

Nine samples of what is, as far as any of them can tell, one condition.
The band is 15–22, a width of 0.23. An earlier note in this file recorded
0.10 from three samples; three samples were not enough to say, and this
supersedes it.

**Decision.** Stop tuning the prompt for this defect, and record why: the
instrument cannot resolve a difference smaller than about 0.25, so an
intervention of prompt size is neither confirmed nor refuted by it. The
three attempts above were reported at the time as "no effect"; the honest
reading is "not decidable with this corpus at this n". The system-prompt
paragraph is reverted on the same grounds the other two were — no evidence
of benefit, and it produced the worst sample of the nine — but "it made it
worse" is exactly as unsupported as "it made it better" would have been.

**Consequences.** `no-enum-value` remains a useful _regression_ detector:
`ORCHESTRA_EVAL_TOLERANCE` at 0.3 sits just outside the observed band, so
the filter beginning to drop outright would still fail the run, which is
what the tolerance was sized for. It is not an improvement detector, and
`docs/specs/eval.md` now says so. Deciding a prompt-sized effect would need
n far above 30 — several hundred runs, tens of minutes of GPU per
measurement — which is not a price a PoC's suite should ask anybody to pay
on every change. The defect itself stays open. What is left to try is
structural rather than textual: `ask_user` is a separate tool competing
with the operation's own tool, so every attempt so far has been arguing
with the model about which tool to pick, and the thing to change is that it
has to pick.

## 2026-09-12 — @mui/x-charts, pinned exactly, captioned rather than titled

**Context.** `docs/plans/dashboard.md` Task 1 adds the first chart. Two
things about the dependency were worth settling rather than rediscovering.

**Decision, the version.** `@mui/x-charts` is pinned exactly (`9.4.0`),
where `@mui/material` and `@mui/icons-material` beside it carry `^9.4.0`.
The carets are held down by a lockfile that already resolved them to
9.4.0; a third caret had nothing holding it and resolved fresh to the
latest (9.13.0), which is not the same line as the other two. An exact pin
says what the other two mean rather than depending on a lockfile never
being regenerated. MUI's packages are not in `pnpm-workspace.yaml`'s
catalog - that file is a protected harness path, and which packages belong
in it is not a question this task gets to answer.

**Decision, the accessibility tree.** A chart is the first thing in this
repository that is a picture, and `make guard-a11y` runs axe against a
real screen. `@mui/x-charts`' own `title` prop is not the way to give one
a text alternative: it lands as an `aria-label` on a `role="none"`
container, which axe's `aria-prohibited-attr` rule (WCAG 4.1.2) rejects
outright. The browser's own accessibility tree recovers - `role="none"`
loses to the presentational-conflict rule and the container resolves to
`generic` carrying the label as its name - but axe reports the authored
markup, not the resolved tree, so the gate fails either way. `ResultChart`
draws its title as a visible `<figure>`/`<figcaption>` instead, which is
both violation-free and readable by someone who can see it. Verified by
running the built chart's HTML through axe-core in Chromium, before the
component was written that way, not read out of the library's docs.

**Consequences.** Any future chart in this repository gets its text
alternative the same way. A caption a person can read is a better answer
than a label only a screen reader gets, so this is not a workaround being
tolerated - it is the thing that should have been done first.

## 2026-09-12 — panels.view's migration checks the column in Go, not in SQL

**Context.** `docs/plans/dashboard.md` Task 2 adds a `view` column to the
`panels` table, which already ships rows on disk at `ORCHESTRA_DB_PATH`.
`schema.sql`'s tables are all `CREATE TABLE IF NOT EXISTS`, which does
nothing for a column added to a table that already exists, so an existing
file needed an `ALTER TABLE` run against it once, safely, on every open.
The obvious spelling is SQLite's own `ALTER TABLE panels ADD COLUMN IF NOT
EXISTS view TEXT` - upstream SQLite has supported that syntax since 3.35
(2021), and `modernc.org/sqlite` v1.58.0's `sqlite_version()` reports
3.53.4, well past it.

**What actually happened.** That exact statement fails at parse time
against this driver: `SQL logic error: near "EXISTS": syntax error`. A
plain `ALTER TABLE panels ADD COLUMN view TEXT` (no `IF NOT EXISTS`) works
fine, and fails on the specific error `duplicate column name: view` when
run a second time. This was verified directly (not inferred from a
changelog) with two throwaway `go run` programs against the pinned driver
version before writing the real code: modernc.org/sqlite's own SQL parser
does not yet accept the trailing clause on `ALTER TABLE ... ADD COLUMN`,
whatever the reported `sqlite_version()` says about the C library it is a
port of.

**Decision.** Do the idempotence check in Go instead of in SQL:
`internal/adapter/repository/sqlite/migrate.go`'s `panelsHasViewColumn`
queries `SELECT 1 FROM pragma_table_info('panels') WHERE name = 'view'`
(the same `QueryRowContext`+`Scan`+`sql.ErrNoRows` shape `AddPanel`'s own
existence check already uses, not a hand-rolled loop over
`PRAGMA table_info`'s full row shape, which needlessly enlarges the
untestable-error-branch surface for no benefit), and
`ensurePanelsViewColumn` runs the plain `ALTER TABLE` only when that comes
back false. `store.go`'s `openDB` calls it unconditionally, right after
`schemaSQL`, on every open - fresh file or old one, the two cases end up
identical the moment the column exists.

**Consequences.** `TestNewOpensADatabaseFileWrittenBeforeViewExisted`
builds a file with the schema exactly as it existed before this task (no
`view` column, a workspace and a panel inserted with raw `database/sql`,
independent of whatever `schema.go`'s embedded SQL says today) and proves
`sqlite.New` opens it, the old panel reads back with a nil `View`, and the
file accepts a new panel with a view afterward - the test the plan calls
the one that actually matters here, since a migration nobody ran against
old data is a migration nobody tested. Any future column added to an
existing table in this package should follow the same check-then-`ALTER`
shape rather than reach for `IF NOT EXISTS` on `ADD COLUMN` again.

## 2026-09-12 — AddPanel narrows the catalogue by permission (AC-P-107)

**Context.** `docs/specs/auth.md` section 7, written for the workspaces
subproject, deliberately left `Workspaces.AddPanel` checking only whether
an operation exists in the whole configured catalogue, not whether the
calling person holds a permission for it - "a panel naming an operation the
person may no longer call fails in its own card", at `/api/invoke` time,
was the chosen point, so the check would not be made twice. `workspaces.go`
said so directly in `Workspaces`' own doc comment.

**Decision.** `docs/specs/dashboard.md`'s AC-P-107 supersedes that for
`AddPanel` specifically: "a person may not build a panel over an operation
they may not call, through this endpoint any more than through
`/api/invoke`." `Workspaces` now takes a `usecase.PermissionStore` (a third
constructor argument) and `AddPanel` calls a `catalogFor` narrowing the
same way `Orchestrator.catalogFor` already does - the whole catalogue for
an admin, `catalog.For(permissions.For(ctx, user.ID))` for anybody else -
before `catalog.Find`, so an operation that exists but that the caller
cannot reach gets the same `ErrEndpointNotFound` an operation that does not
exist anywhere gets. `pkg/app.build` now opens the permission store before
`newWorkspaceHandler` rather than after, so it has one to pass in.

**Consequences.** Every existing `Workspaces` test needed a third
constructor argument; `workspaces_test.go`'s `grantedPermissions()` grants
`exposedCatalog()`'s one operation regardless of which user is asked about,
so every test that is not itself about permission narrowing keeps its
prior behaviour unchanged. Two new tests
(`TestWorkspacesAddPanelRejectsAnOperationTheUserMayNotCall`,
`TestWorkspacesAddPanelNeverConsultsThePermissionStoreForAnAdmin`) pin
AC-P-107 and the admin exception directly. `docs/specs/auth.md` section 7's
prose about `/api/invoke` being the one place this is checked is now true
of every endpoint except `AddPanel`; a future reader should treat AC-P-107
as the narrower, later word on this one path.

## 2026-09-12 — Dashboard Task 3: a chart hint is strict, and stands alone

**Context.** `docs/plans/dashboard.md` Task 3 asks for `x-ui-hint.chart`,
the sibling of `x-ui-hint.component` (`parse.go`'s `uiHint`), and leaves
two judgement calls open: what a malformed hint does, and whether a
`chart` block by itself - with no `component: chart` alongside it - makes
`Render`/`RenderResult` choose the chart component.

**Decision 1: `chart` is strict where `component` is lenient, and the two
are deliberately made to differ.** `uiHint`'s existing handling of
`x-ui-hint.component` was already lenient before this task: an absent
extension, a non-object `x-ui-hint`, or a non-string `component` all
degrade silently to "no override" - never an error - because a missing
override just falls back to deciding from the response schema, which is
always a safe answer to fall back to. A malformed `x-ui-hint.chart` has no
such safe fallback: there is no "guess the axes" default, and a service
maintainer who mistypes `category` or spells a `kind` wrong needs the
fetch to fail where they can see it, not have the hint silently vanish
from a result nobody looks at hard enough to notice the chart never drew.
`parseChartHint` (`internal/adapter/specsource/http/parse.go`) therefore
returns `errMalformedChartHint` - wrapped with the reason - for anything
present but not right: not an object, missing `category` or `value`, or a
`kind` outside `bar`/`line`/`pie`; `Source.Fetch` propagates it and fails
the whole fetch, the same way an unparsable OpenAPI document already does.
This is a considered asymmetry, not an oversight: the task's own prompt
asked for it to be named explicitly if the two diverged, and they do.

**Decision 2: a `chart` block alone means chart, with no `component: chart`
needed beside it.** `docs/specs/dashboard.md` P2 says "where a contract
declares [chart axes], a chat answer draws as a chart with nothing
configured" - not "draws as a chart when it also says `component: chart`".
Read literally, declaring `x-ui-hint.chart` is itself the declaration of
how the result draws; requiring a second, redundant field to say the same
thing would make a contract author write the same fact twice to get one
outcome. `Render` and `RenderResult` (`internal/domain/rendering.go`) both
gained the same second rule, ranked directly beneath the existing
`UIHint` override and above everything else: `Endpoint.ChartHint != nil`
means `ComponentChart`. An explicit `UIHint` (say, `detail`) still wins
outright over a `ChartHint` on the same operation - `UIHint` is a stated
override of "how this draws", and stays first - but absent that, a
`ChartHint` no longer needs a `UIHint` of `chart` riding along with it.
Pinned by `TestRenderChartHintAloneMeansChart`,
`TestRenderUIHintWinsOverChartHint`, `TestRenderResultChartHintAloneMeansChart`
(`internal/domain/rendering_test.go`) and the fixture's `countWidgets`
operation, which declares `chart` with no `component` at all.

**Consequences.** `usecase.Result` gained `View *domain.View`, filled in
by `chartViewFor` in `Orchestrator.invokeAndRender` from
`endpoint.ChartHint` alone - `View.Transform` is never set from a
contract, matching "The shape everything shares"'s "a contract declares
axes, not transformations." `handler.Plan.toAPIPlanResult` carries it onto
the wire with the same `toAPIView` Task 2 already wrote for `Panel.View`
(`workspace.go`), unchanged: one conversion function for both directions
`domain.View` reaches the wire from. No service in this repository
declares `x-ui-hint.chart` (`docs/specs/dashboard.md` sections 1 and 7 keep
the dummies unchanged on purpose), so every test exercising the parse or
the render rule uses `internal/adapter/specsource/http/testdata/fixture.yaml`'s
new `countWidgets` operation and, for the malformed cases, inline spec
snippets built in `source_test.go` rather than the shared fixture (adding a
failing operation to the shared fixture would have broken every other test
that fetches it successfully).

## 2026-09-12 — Dashboard Task 4: one narrowing function, no port-boundary export, and a stale filelen glob left alone

**Context.** `docs/plans/dashboard.md` Task 4 asks for `GET /api/catalog`:
every operation the signed-in person may call, each with enough to build a
panel over it - narrowed through the same rule `docs/specs/auth.md` section
5 already names as the one seat both the planner's tool list and
`/api/invoke`'s lookup share, `catalogFor` (`internal/usecase/auth.go`).

**Decision: `internal/usecase/catalog.go` calls `catalogFor` directly, adding
no new port.** `Catalog` (the usecase) takes exactly what `Orchestrator` and
`Workspaces` already take - `domain.Catalog` and `usecase.PermissionStore` -
and its `For` method is `catalogFor(ctx, c.catalog, c.permissions, user)`
plus a conversion loop. No third implementation of the narrowing rule was
written; the task's own framing ("a rule applied in two places is a rule
that will disagree with itself") is why this reuses the existing
package-private function rather than exporting a fresh copy of its nine
lines.

**Decision: `inputSchemaFor` and `fieldsFor` stayed unexported - no port
boundary was crossed.** The plan's own text asked to report anything added
to `usecase` to expose these through a port boundary "if the layering
forced it." It did not: `internal/usecase/catalog.go` is a file in the same
package as `internal/usecase/tools.go` (`inputSchemaFor`,
`schemaToJSONSchema`) and `internal/usecase/orchestrator.go` (`fieldsFor`,
`chartViewFor`), so `toCatalogEntry` calls all three directly, the same way
`Orchestrator.formFor` and `Orchestrator.invokeAndRender` already do for a
`/api/plan` result. A catalogue entry's `schema`/`fields`/`view` are
supposed to be exactly the same shapes a plan result's `schema`/`fields`/
`view` are (`docs/specs/dashboard.md` section 5's whole point), so sharing
the same private functions is not a convenience - it is what keeps the two
from drifting apart the way a second, hand-written conversion eventually
would.

**Decision: `GET /api/operations` (`usecase.Admin.Operations`) is
untouched, on purpose.** It answers "what does this deployment hold",
unfiltered, admin only, for the permission grid; `GET /api/catalog` answers
"what may I call, and what shape is it", for whoever is signed in. Section
5 of the spec argues why merging them would be one endpoint answering two
questions with a role check in the middle - this task did not merge them,
and flags this here rather than doing it, per the task's own instruction.

**Decision: `CatalogEntry.Fields` is absent for a chart-hinted endpoint,
same as a `/api/plan` result's `Fields` already is, and the acceptance
test asserts that rather than fighting it.** `domain.FieldsSchema` only
describes a table's row schema or a detail's own properties
(`RenderResult` picking `ComponentTable`/`ComponentDetail`); a chart-hinted
endpoint's `RenderResult` is `ComponentChart` (Task 3's own rule, ranked
above the table/detail rules), so `FieldsSchema` returns `nil` for it, and
`fieldsFor` - reused as-is - carries that through to the catalogue exactly
as it already does for a `/api/plan` result over the same endpoint. A
service wanting its chart's axes offered as `fields` too would need its
response schema to independently support a table or detail rendering,
which is outside this task's scope to change.

**Left alone, not fixed: `harness/quality/file-length.txt`'s exclude list
does not cover the per-service generated `.d.ts` files.** Adding
`GET /api/catalog` and `CatalogEntry` to `services/platform/api/openapi.yaml`
grows `web/src/shared/api/gen/platform.d.ts` (openapi-typescript output, one
file per service, never hand-edited) from 971 to 1030 lines - past
`guard-filelen`'s 1000-line limit. `file-length.txt`'s own header says
"generated code is exempt," and `harness/quality/oxfmt/policy.ts` and
`harness/quality/oxlint/policy.ts` both already exclude the whole
`**/src/shared/api/gen/**` directory for that reason (and
`DECISIONS.md`'s 2026-09-11 orval entry already assumed `guard-filelen` did
too) - but `file-length.txt` itself only excludes `schema.d.ts`, a filename
that predates the per-service split and does not exist anywhere in this
repository any more. This is a real, pre-existing gap, not a rule this task
disagrees with, and the honest fix is a one-line addition to
`file-length.txt`. `AGENTS.md` rule 2 forbids reconfiguring anything under
`harness/quality/` regardless of how clearly justified the change looks
from inside a task, so it was left as-is: `make check` is green except this
one gate, `guard-filelen`, failing on a file this task did not write by
hand and could not shrink without dropping a field the spec requires. See
`STATE.md`'s "Known gaps in the harness" and `TODO.md`'s "Next" list.
`allOf`-based reuse of the `Operation` schema was tried first (`CatalogEntry:
allOf: [Operation, {...}]`) and reverted: it saved only 8 of the 30 lines
needed, and it silently turned `summary` optional (`Operation.summary` is
not `required`), which this contract does not want for `CatalogEntry`.

## 2026-09-13 — a file-length exclude can name a directory

**Context.** `docs/plans/dashboard.md` Task 4 added `GET /api/catalog` to
the contract, and `make generate` grew `web/src/shared/api/gen/platform.d.ts`
from 971 lines to 1030. `make guard-filelen` failed on it, and `main` was
red on a generated file.

`harness/quality/file-length.txt` says in its own comment that generated
code is exempt - "its size is decided by
`services/platform/api/openapi.yaml`, not by anyone editing it, and it is
never hand-edited". Its exclude list did not do that. It held `*.gen.go`
and `schema.d.ts`, and `schema.d.ts` is a file this repository no longer
has: `harness/guard/filelen.sh` matched every glob against the basename
alone, so when the generated TypeScript moved to
`web/src/shared/api/gen/*.d.ts` the exclude stopped matching anything. It
kept looking like an exclude. `harness/quality/oxlint/policy.ts` and
`oxfmt/policy.ts` both already name `**/src/shared/api/gen/**`, and a
2026-09-11 entry in this file assumed `guard-filelen` did too.

**Decision.** A glob containing a `/` is matched against the
repository-relative path; one without is still matched against the
basename. The stale `schema.d.ts` becomes
`web/src/shared/api/gen/*.d.ts`.

This narrows the exemption rather than widening it: the obvious one-line
alternative, `exclude *.d.ts`, would have exempted every `.d.ts` in the
repository including the hand-written `web/src/vite-env.d.ts`, and would
have left the basename-only matching in place to go stale again the next
time a generator moves its output. Raising `limit` was the other
alternative and is worse still - the limit's own comment is an account of
a 1,622-line hand-written file being rewritten eleven times, which is
about hand-written files and unaffected by how long a generated one is.

**Consequences.** Verified both ways rather than assumed: with the change
in place a 1,001-line hand-written file under `web/src/shared/lib/` still
fails the guard, and removing it passes. `make check` is green again, with
no request added to the model's log.

A guard reports what it failed and never what it skipped, which is why
this rotted invisibly for as long as it did. Nothing here fixes that
general problem - a guard that printed its exclusions would have caught it
the day the file moved - and it is worth doing if another exclude ever
goes stale.

## 2026-09-13 Dashboard Task 5: a panel's chart size, and what a transform-only view draws through

**Context.** `docs/plans/dashboard.md` Task 5 has `pages/workspace/ui/PanelResult.tsx`
draw a saved panel by its `view`: a transform before the component is
chosen, a chart when `view.chart` names one. Two things the plan leaves to
the implementer: what size `ResultChart` gets now that it takes one, and
what a `view.transform`-only panel (no chart) actually renders through.

**Decision, sizing.** `ResultChart` keeps its old fixed 320×240 as the
default for its own two `width`/`height` props (so Task 1's tests, which
pass neither, see exactly what they always have) and `PanelResult` passes
two concrete sizes of its own, picked with `useMediaQuery("(min-width:600px)")`

- MUI's own default `sm` breakpoint, named directly as a media-query string
  rather than through `useTheme().breakpoints.up("sm")` (the form
  `app/ui/Shell.tsx` already uses) because importing `@mui/material/styles`
  on top of everything else this file already imports trips
  `import/max-dependencies` (max 10; the file was already at 10 without it).
  Narrow is 260×200 - comfortably under a `PanelCardShell`'s `CardContent`
  content width at a 375px viewport (viewport minus `WorkspacePage`'s own
  `p:3` and the card's default `CardContent` padding leaves roughly 295px);
  wide is 560×320 - big enough that a chart reads as the panel's content
  rather than a decoration in the corner of one, small enough to stay well
  inside any workspace layout this slice draws. Both are named constants a
  future task can revisit once an actual wide layout (Task 6, or FR-F-4's
  arrangement) gives a panel a real width to measure instead of two guesses;
  a `ResizeObserver`-based measurement was considered and rejected for now as
  more machinery than two fixed, breakpoint-picked numbers earn today.

**Decision, transform-only rendering.** `view.transform` with no
`view.chart` still goes through `RenderedResult`/`ResultTable`, exactly as
an untransformed result does - but the grouped rows `applyTransform`
returns are a bare array, not the `{items: [...]}`-shaped envelope
`rowsFromData` was written to unwrap, so `PanelResult` re-wraps them as
`{ rows: groupedRows }` before handing them down. `rowsFromData` only ever
needs a single array-valued property to find rows in, regardless of what
that property is called, so this costs nothing beyond the wrapping call.
The alternative - giving `ResultTable`/`RenderedResult` a second, rows-only
input shape - was rejected as a change to Task 1/entities/rendering code
this task's Files list does not name, for a benefit (skipping one object
literal) too small to justify it. This only covers a transform paired with
a table-shaped result, which is the only combination `docs/specs/dashboard.md`
section 3's own example pairs (transform with a table; chart, with or
without a transform, separately) - a transform saved against a `detail`
component is not a case this task builds for or tests.

**Decision, a chart over rows missing its fields.** `ResultChart` already
answers this: a row with no finite value at `view.chart.value` is dropped,
one with no string at `view.chart.category` draws under a shared `"null"`
label, and zero surviving rows draws its own `結果は0件です。` message.
`PanelResult` adds no second opinion here - it hands `ResultChart` the same
rows regardless of whether they carry the named fields, and lets that
existing behaviour decide, pinned by a `PanelResult` test with a row
carrying neither `category` nor `value`.

**Consequences.** `entities/rendering`'s barrel gained `rowsFromData`/`Row`
exports (the functions already existed; they were only reachable from
inside the slice) and `shared/api/client.ts` gained a `View` type alias -
both existing shapes made reachable across the FSD boundary, not new
logic. `web/src/shared/api/client.ts` was at exactly its 300-line
`max-lines` ceiling before this; the new export forced two nearby
paragraph comments to be rewrapped to fewer, fuller lines to stay under it

- their wording is otherwise unchanged.

## 2026-09-13 Dashboard Task 6: building a panel without a question

**Context.** Task 6 (`docs/plans/dashboard.md`) is the workspace screen's
own "add a panel" control - pick an operation from `GET /api/catalog`, fill
its arguments, pick a component, and for a chart pick its axes, all as one
form (`docs/specs/dashboard.md` section 6, P7/P8). It asked for `ResultForm`
to be made reusable "there, once" rather than copied, and left a few shapes
open: what "reusable" actually means, which components a chart is offered
alongside, and what an incomplete form does when saved.

**Decision, reusing `ResultForm`.** `ResultForm` did two things at once:
seed and hold a schema's values (`useState`, keyed by `schema.properties`),
and draw one control per property. Both moved out, unchanged in behaviour,
into `entities/rendering`: `model/useFormValues.ts` (the state) and
`ui/ResultFormFields.tsx` (the controls) - `ResultForm` itself is now a thin
composition of the two plus `postInvoke`/the submit button, and its own
tests were not touched (same public props, same behaviour). `features/panels/ui/PanelArguments.tsx`
calls the same two exports directly - the identical controls over a
catalogue entry's `schema`, no invocation, no second form. Its parent
(`AddPanelForm`) remounts it with `key={service:operationId}` whenever the
chosen operation changes, so a fresh `useFormValues` reseeds from the new
schema; this was chosen over teaching `useFormValues` to watch its own
`schema` argument for changes, which would have added a `useEffect` (and a
new failure mode: silently resetting a person's half-typed values whenever
a parent re-render happened to hand it a new-but-equal schema object) for a
behaviour a `key` gives for free.

**Decision, which components a chart is offered alongside.** The spec says
`chart` "is offered whenever `fields` describes rows" but a `CatalogEntry`
carries no flag saying its `fields` describe _rows_ specifically, as
opposed to one object's own properties (`detail`). Rather than infer that
from `component === "table"` - which would silently exclude a `detail`-shaped
operation whose fields happen to work fine as chart axes - `usePanelFields.componentOptionsFor`
offers `chart` whenever `fields` is present at all (and the entry's own
`component` is not already `chart`). This is a small overreach in the
person's favour: they can pick `chart` for an operation the rule did not
intend to chart, and get `ResultChart`'s own existing "0件です" or
dropped-row behaviour if the axes do not fit, rather than the platform
guessing that they cannot.

**Decision, an incomplete form's save button.** `handleSave` (`usePanelBuilder`)
silently no-ops when `usePanelFields.canSave()` says the choice is
incomplete (no operation, no title, a chart with an unset axis, a transform
switched on with an unset field), and the "追加" button is never disabled
for that reason - only while `submitting`. This mirrors `SaveToWorkspaceControl`/`useSaveToWorkspace`'s
existing `handleSave` exactly, for the same reason its own comments give:
`make guard-layout` rejects a `contained` button disabled at rest (no
visible edge), so "grey it out until valid" was never an option; a `helperText`
per control was considered and rejected as a second form on top of the
first for validation nobody has asked for yet, since every control here is
a closed choice (a select or an enabled schema field) rather than free text
that can be malformed.

**Decision, the transform is independent of the component.** Per section 4
("a table with a transform is a perfectly good panel"), `TransformFields`
is shown whenever the chosen operation has any `fields` to group by at all,
regardless of whether `component` is `table` or `chart` - only the chart
axes (`ChartFields`) are gated on `component === "chart"`.

**Consequences.** `entities/rendering`'s barrel gained `useFormValues`,
`ResultFormFields`, `fieldEntries` and `Fields` - all pre-existing logic
made reachable across the FSD boundary. `web/src/shared/api/catalog.ts` is
a new, small client module (`getCatalog`), split out for the same
`max-lines` reason `users.ts` already documents for itself.
`features/panels/model/usePanelBuilder.ts` composes two smaller hooks -
`useCatalog` (step 1's lazy load) and `usePanelFields` (steps 2-5's state) -
to stay under `max-lines-per-function`; `WorkspacePage.tsx` gained a single
`useWorkspacePage` hook (wrapping `useWorkspace` + a new `useAddedPanels`)
for the same reason on `import/max-dependencies`, now that it also hosts
`AddPanelControl`. A panel added through the control is shown by appending
it to local state (`useAddedPanels`) rather than re-fetching the whole
workspace - the panel `POST /api/workspaces/{id}/panels` returns is already
everything `PanelResult` needs to draw it.

## 2026-09-13 Dashboard Task 7: closing the two gaps end to end found

**Context.** `docs/plans/dashboard.md` Task 7 asked for the whole
subproject's journey against the built product, and to check first that
every acceptance criterion in `docs/specs/dashboard.md` section 10 had a
test actually running in CI - "this is the task that covers it" if one did
not. Two real gaps turned up doing that check, both because Tasks 2-6's own
unit/acceptance tests each exercised their own layer in isolation and never
drove the whole stack from the chat screen or the panel builder the way a
person would.

**Gap 1: a chart-hinted chat answer never drew.** AC-P-105 has two halves -
"draws as a chart from the chat" and "a panel saved from that answer
carries the contract's axes". `TurnList.tsx`'s `renderResultAnswer`
(`features/conversation`) only ever matched `component === "table"` or
`"detail"`; a `chart` component fell through to the generic "this answer
carries nothing to draw" fallback, and `SaveToWorkspaceControl`/
`useSaveToWorkspace` never accepted a `view` at all, so even a chart that
did draw would save a panel with none. **Decision.** Add a third branch to
`renderResultAnswer` for `component === "chart"` with `view.chart` set,
drawing `ResultChart` over `rowsFromData(result.data)` the same way
`PanelResult.tsx` already does for a saved panel; thread the result's own
`view` through `SaveControlSlotProps` and into `useSaveToWorkspace`/
`SaveToWorkspaceControl` as an optional prop, copied onto
`POST /api/workspaces/{id}/panels` exactly as `source`/`component` already
are. **Consequences.** `Conversation.test.tsx`'s own chart coverage lives in
a sibling file, `ConversationChart.test.tsx` - the parent file was already
at its 300-line budget - and `SaveToWorkspaceControl.test.tsx` gained a
case pinning the view round trip.

**Gap 2: a transform's own chart could not be built at all.** `usePanelFields.ts`'s
chart axis pickers (`category`/`value`) always offered `fieldOptionsFor(entry)`

- the raw response's own fields - never aware that `applyTransform`
  (`entities/rendering/lib/transform.ts`) replaces every row with exactly two
  keys, `transform.groupBy` and the aggregate's own name. Manually verified
  against the built product (`services/platform/bin/api`, real
  `ListInventoryItems`): building a panel with a transform (`groupBy: status,
aggregate: count`) and a chart whose value axis named any real field (there
  is no field literally called `count` on an inventory item) invoked fine at
  save time but failed every later refresh with `usecase.validateEnumArg`-style
  "is not a valid value" from `ResultChart` silently skipping every row as
  "not a number" - the exact journey this task's own browser spec needs
  ("a list operation with a group-by and a bar chart, see it draw") could not
  be built through the UI at all. **Decision.** `chartFieldOptionsFor`
  (`usePanelFields.ts`) narrows the chart's own axis options to
  `[groupBy, aggregate]` once a transform is enabled, leaving `fieldOptions`
  itself (still used for the transform's own `groupBy`/`aggregateField`
  pickers) untouched. **Consequences.** `AddPanelControlTransform.test.tsx`
  (a new file, kept separate from `AddPanelControl.test.tsx` for the same
  line-budget reason as gap 1's test) pins both the narrowed option list and
  a full save with a working transform+chart combination.

**Gap 2b, found alongside it: an untouched optional argument broke every panel.**
While chasing gap 2 with the real inventory service, `POST`ing a panel over
`ListInventoryItems` with its optional `status` argument left untouched
also failed every refresh: `useFormValues`'s own `seedValue` seeds an
untouched optional string/enum field to `""`, and that empty string was
posted as the argument's own value, which `usecase.validateEnumArg` (a real
enum's `""` is never one of its declared values) rejects outright - a panel
built without ever touching an optional filter could never draw, which is
the common case, not an edge one. **Decision.** `compactArgs` in
`usePanelBuilder.ts` drops any argument whose value is still `""` and whose
key is not in the entry's own `schema.required`, right before
`POST /api/workspaces/{id}/panels` - an omitted optional argument, not an
explicit empty one. A required field's own `""` is left alone: that gap
belongs to `missingBeforeSave`, not to this. **Consequences.**
`AddPanelControl.test.tsx`'s pre-existing "posts no transform" test changed
its own pinned expectation from `args: { keyword: "" }` to `args: {}` -
`keyword` was never a real backend field, so the old assertion was pinning
the bug, not a real behaviour - and a new `AddPanelControlArgs.test.tsx`
pins the fix against a schema shaped like a real enum parameter. Verified
directly against the built platform and inventory binaries
(`POST /api/invoke` with `args: {}` returns the full item list; with
`args: {"status": ""}` it 400s) before and after the fix, not only through
the mocked unit tests.

**Where the journey itself lives.** `e2e/src/dashboard.test.ts` (the
catalogue → panel → read-back round trip, AC-P-101/102/103/104) and
`e2e/src/dashboard-permissions.test.ts` (AC-P-107, split out once the first
file reached its own line budget) are the process-level suite;
`e2e/browser/dashboard.spec.ts` is the browser one - sign in, create a
workspace from the drawer directly (no question asked, per P8), build a
panel over `ListInventoryItems` with a group-by-status transform and a bar
chart entirely from the catalogue, see four bars draw, reload the page, and
see the same four bars draw again from the saved panel. Neither suite wires
`ORCHESTRA_PLAN_FIXTURES`: nothing in either journey asks a question, so
the stub planner has no part in it, which is P8's own point made
executable.

## 2026-09-13 English showing through the panel builder: a real bug and a real gap, and x-ui-hint.displayName

**Context.** Using the panel builder found English text on screens meant to
read in Japanese. Two separate causes, not one.

**Cause 1, a bug: the axis pickers threw away the labels the contract
already carries.** `fieldOptionsFor` (`usePanelFields.ts`) returned
`Object.keys(entry.fields)` - raw property names (`status`, `quantity`,
`employee`) - for the chart's category/value axes and the transform's
`groupBy`/aggregate-field pickers. `entry.fields` is the same shape a table
already reads a `title` from (`columnTitle`,
`entities/rendering/model/rows.ts`) to draw 「ステータス」 instead of
`status`; the picker just never called it. **Decision.** `fieldOptionsFor`
now returns `FieldOption[]` (`{value, label}`): `value` is still the bare
property name (what a panel posts, unchanged), `label` is `columnTitle`'s
own answer. `chartFieldOptionsFor`'s transform-on branch needed a label for
`applyTransform`'s own output keys too (`count`/`sum`/`avg`), which name no
contract property and so have no `title` to look up - `AGGREGATE_LABELS`,
a `Record<Aggregate, string>` beside `applyTransform`
(`entities/rendering/lib/transform.ts`, the one place that already knows
what those keys mean), fills that gap and is reused by `TransformFields`'s
own aggregate-method picker so the label exists in exactly one place.
**Consequences.** `ChartFields.tsx` and `TransformFields.tsx` render
`option.label`/key `option.value` instead of the bare string.
`AddPanelControlTransform.test.tsx`'s pinned option list changed from
`["status", "count"]` to `["status", "件数"]` (the fixture's `status` field
still carries no `title`, so it still falls back to the property name -
only the aggregate's own name gained a label); `e2e/browser/dashboard.spec.ts`
picks the real `ListInventoryItems.status` field by its real title,
`ステータス` (`services/inventory/api/openapi.yaml`'s `ItemStatus` already
declared one), and the value axis by `件数`, not `count`.

**Cause 2, a design gap: an operation has no Japanese name anywhere.**
`OperationPicker` and the default panel title both read `entry.summary`;
`list_capabilities`' 操作 column read the raw operation id. Every
contract's `summary` is English (it is written for a person, but the
_model_ is that person - `usecase.ToolsFor` sends it verbatim as
`Tool.Description`) and this is a pre-existing product-wide gap
(`list_capabilities`' answer to 「何ができるの？」 shows the same English) that
the builder only made visible. **Decision: `summary` is not translated, and
does not become the display name.** It is the tool description the planner
sends the model; changing it changes how the model chooses operations,
which would move `make eval`'s measured baseline (checked: `usecase.ToolsFor`,
`internal/adapter/planner/toolcall/planner.go`, and
`internal/adapter/planner/jsonmode/planner.go`'s `renderCatalog` are the
only readers of `Endpoint.Summary`/`CatalogEntry.Summary`, and none of them
changed). A display name is a different thing from a model-facing
description and gets its own field: `x-ui-hint.displayName`, beside
`component` and `chart` (D7's own extension point). `displayName` is
chosen, not `title` or `name`, because both of those already mean something
else nearby (a JSON Schema property's `title`, an operation's own
`operationId`/tool `Name`) and this needed to read unambiguously as "what a
_person_ calls this," never confusable with either.

It flows through exactly as D7's own hint does: `specsource/http.uiHint`
reads it (leniently - absent, non-object, non-string all mean "no display
name," never a fetch error, matching `component`'s own leniency, not
`chart`'s); `domain.Endpoint.DisplayName` carries it; a new
`domain.Endpoint.DisplayNameOr(fallback)` method is the one place that
decides what a contract's silence means - because that silence means a
_different_ thing depending on who is asking. `usecase.toCatalogEntry` asks
for `e.DisplayNameOr(e.Summary)` (`CatalogEntry.DisplayName`, required,
never blank) since `Summary` is what `OperationPicker` and the default
panel title already showed; `orchestrator.capabilitiesItems` asks for
`e.DisplayNameOr(e.OperationID)` since the operation id is what its own 操作
column already showed. Neither caller's existing behaviour changes for a
contract that declares nothing.

**The dummy services get one.** `docs/specs/dashboard.md` sections 1/7 keep
`services/inventory`/`services/attendance` untouched for aggregate
endpoints - adding one would mean every real team must add one too,
undercutting the whole premise that the platform draws from what a service
already exposes. A display name is the opposite case: presentation
metadata `x-ui-hint` already exists to carry (D7), `x-enum-labels` already
sets the precedent that a contract carries its own Japanese labels for what
it draws, and a service that declares none still works exactly as before.
Every `x-orchestra-expose: true` operation in both dummy services' contracts
now carries an `x-ui-hint.displayName` (在庫一覧, 在庫アイテムの作成,
在庫アイテムの詳細, 勤怠記録一覧, 勤怠記録の作成, 勤怠記録の詳細).

**Consequences.** `GET /api/catalog`'s `CatalogEntry` gained `displayName`
(required); `OperationPicker.tsx` and `usePanelFields.ts`'s `selectEntry`
(the default panel title) now read `entry.displayName` instead of
`entry.summary`. `orchestrator_test.go` crossed `guard-filelen`'s 1000-line
budget adding this case alongside gap 1's; its `list_capabilities` tests
(and the `twoServiceCatalog` fixture they share) moved to a new sibling
file, `orchestrator_capabilities_test.go`, the same split gap 1's own tests
already used this subproject for `AddPanelControlTransform.test.tsx`.
`e2e/browser/dashboard.spec.ts` picks the operation by 在庫一覧, not by its
English summary. Verified `make eval` was not run and no operation's
`summary` changed (see above); verified `docker logs llama-swap`'s
`POST /v1/chat/completions` count is unchanged across a full `make check`
run, matching the existing AC-E-202 guarantee this subproject does not
touch.

## 2026-09-13 A service has no Japanese name either, and info.x-ui-hint.displayName

**Context.** The panel builder's own follow-up: `inventory` and `attendance`
are shown raw wherever a _service_ appears, not just wherever an _operation_
does - `OperationPicker`'s group headers, `list_capabilities`' サービス
column, a result's provenance, and the admin's permission grid, all grouped
or labelled by the bare identifier. The previous entry (above) gave an
_operation_ a name; this does the same one level up, for the _service_ that
holds it, and extends D15 rather than inventing a parallel mechanism
(`docs/specs/orchestration.md` D16).

**Where the name comes from, and why not the alternatives.** Three
candidates were checked:

- `ORCHESTRA_SERVICES` - rejected. It is where an operator points the
  platform at a URL; naming a Japanese label there conflates "where do I
  find you" with "what do I call you for a person," and every other label
  in this product already lives in the contract instead
  (`x-enum-labels`, a JSON Schema property's `title`, D15's own
  `x-ui-hint.displayName`).
- `info.title` - checked and rejected. `services/inventory/api/openapi.yaml`'s
  `info.title` is `Inventory API` today: an OpenAPI-conventional API name,
  not a Japanese label, and `grep -rn "\.Title\b" services/platform/internal`
  turns up nothing that reads `openapi3.T.Info.Title` anywhere in this
  repository - adopting it as the display name would mean either renaming
  it away from an "API name" (which is what `info.title` is _for_,
  everywhere else this format is read) or living with "Inventory API"
  showing up as a group header, neither of which is what a person wants
  to read.
- `info.x-ui-hint.displayName` - chosen. It sits exactly one level up from
  an operation's own `x-ui-hint.displayName` (D15), the same extension,
  the same object shape, read the same leniently: absent, non-object or
  non-string all mean "no display name," never a fetch error, matching
  `component`'s own leniency and D15's before it. A service that declares
  none behaves exactly as it did before this field existed.

**What must not change, verified.** `service` remains the identifier -
`ORCHESTRA_SERVICES`' own key, what `source.service` carries in a plan
result's provenance, what a `Permission` row and a `Panel` name, and what
`/api/invoke` resolves against. None of those readers changed; only what a
screen shows _alongside_ the identifier does. Checked separately (the same
way the previous entry checked `summary`): no operation's `summary`
changed, and neither `usecase.ToolsFor`, `internal/adapter/planner/toolcall/planner.go`,
nor `internal/adapter/planner/jsonmode/planner.go`'s `renderCatalog` were
touched - the model still sees the same tool descriptions it always did.
`docker logs llama-swap`'s `POST /v1/chat/completions` count was checked
before and after a full `make check` run and is unchanged, matching the
existing AC-E-202 guarantee.

**The carriers.** `specsource/http.parseSpec` reads
`info.x-ui-hint.displayName` once per service (`serviceDisplayName`,
sharing `uiHint`'s own extraction of the `x-ui-hint` object rather than
duplicating it) and sets it on every one of that service's
`domain.Endpoint`s - the same way `Service` itself is already set once per
loop and copied onto each endpoint. `domain.Endpoint.ServiceDisplayNameOr(fallback)`
mirrors `DisplayNameOr` exactly. From there, three callers each ask for
their own fallback, one per place "The defect" named:

- `usecase.toCatalogEntry` asks for `e.ServiceDisplayNameOr(e.Service)`
  (`CatalogEntry.ServiceDisplayName`, required, never blank) - read by
  `OperationPicker.tsx`'s `groupBy` (was `entry.service`, now
  `entry.serviceDisplayName`) and, through `usePermissionGrid`'s
  `Operation`, the same field name and fallback, `ServiceCard.tsx`'s card
  header and its "すべて許可" checkbox's `aria-label` (was `group.service`).
- `orchestrator.capabilitiesItems` asks for `e.ServiceDisplayNameOr(e.Service)`
  for the サービス column's _value_ - the filter itself still matches
  `decision.Service` against the identifier `e.Service`, unchanged, so a
  person naming a service by its Japanese name in a follow-up is a
  question this entry does not answer (D11's own territory, untouched).
  `list_capabilities`' synthetic `"platform"` pseudo-service (D14) names no
  real contract to read a hint from, so its `ServiceDisplayName` just
  repeats `"platform"` - the same value the サービス column already showed
  for it.
- `usecase.Result` (from `orchestrator.invokeAndRender` and `formFor`)
  carries `ServiceDisplayName` the same way `Service` already does, into
  `openapi.Source` (`toAPIPlanResult`'s `source` and `target`, both), which
  `Provenance.tsx` reads (`source.serviceDisplayName`, never
  `source.service`) for a chat/plan result's provenance and a submitted
  form's own re-post of its target.

**The one place that still reads the identifier: a saved panel.** `Panel`
stores only `service`/`operationId` (W2, `docs/specs/workspaces.md`) and
`POST /api/invoke` - what `PanelResult.tsx` calls to redraw a saved panel -
answers with no `Source` at all (`InvokeResult` carries `component`/`data`/
`fields`, nothing naming the endpoint). `PanelResult.tsx` already built its
own `Source` literal by hand from the panel's own identifiers before this
field existed; it now sets `serviceDisplayName` to that same identifier,
which is not a fallback hack bolted on top - it is the honest answer to
"what name is available here," the same one `operationId` already gives in
that exact spot, and the smallest change that keeps `Source.serviceDisplayName`
uniformly required rather than introducing an optional field whose absence
every other reader would need to handle. Threading the catalogue (or the
contract's own hint) into `usecase.Workspaces`/`GET /api/workspaces` just to
enrich this one provenance line would be a materially larger carrier than
anything "The defect" named as broken, and the identifier is exactly what
this spot already showed - unlike the four call sites above, no person
reads a raw `inventory` here today where a name was expected instead.

**The dummy services get one.** `services/inventory/api/openapi.yaml`'s
`info` gains `x-ui-hint: {displayName: 在庫管理}`;
`services/attendance/api/openapi.yaml`'s gains `x-ui-hint: {displayName: 勤怠管理}`.

**Consequences.** `GET /api/catalog`'s `CatalogEntry` and `GET /api/operations`'
`Operation` both gain `serviceDisplayName` (required); `Source` gains it
too (required, all three of `orchestrator.invokeAndRender`, `formFor` and
`PanelResult.tsx`'s own literal set it). Every test literal across
`services/platform` and `web` that builds one of these three shapes by
hand now sets `serviceDisplayName` alongside `service` - a mechanical,
wide-but-shallow diff, not a design change. `AddPanelControl.test.tsx`'s
pinned assertion moved from `screen.getByText("inventory")` to
`screen.getByText("在庫管理")`; `PermissionGrid.test.tsx` similarly, plus its
"すべて許可" checkbox's name; `Provenance.test.tsx`, `Conversation.test.tsx`,
`App.test.tsx` and both `e2e/browser` specs that asserted a raw
`"inventory / ListInventoryItems"` provenance string now assert
`"在庫管理 / ListInventoryItems"`. `e2e/src/{auth,context,orchestration}.test.ts`'s
`source` equality checks against the running dummy services gained
`serviceDisplayName: "在庫管理"`/`"勤怠管理"`. Verified `make eval` was not run;
verified no operation's `summary` changed; verified `docker logs llama-swap`'s
`POST /v1/chat/completions` count is unchanged across a full `make check` run.

## 2026-09-13 A panel can be changed after it is made: PATCH, and the view's third state

`docs/specs/dashboard.md` section 6a, P11-P13: `PATCH /api/workspaces/{id}/panels/{panelId}`,
the usecase and sqlite layers underneath it, and the builder's own form
reused for editing (P12).

**"Remove the view" vs "leave it alone" - null and absent, and how that
survives the generated Go types.** `UpdatePanelRequest.view` is declared
`allOf: [$ref: View]` plus `nullable: true` (redocly's `nullable-type-sibling`
rule wants an explicit `type: object` alongside `nullable`, so that is
there too) and `x-go-type: nullable.Nullable[View]` with
`x-go-type-skip-optional-pointer: true`. `oapi-codegen` has a built-in
`nullable-type` output option that does the same thing automatically for
every `nullable: true` property, but that option lives in
`harness/gen/oapi-codegen.yaml`, and that file is part of the Repository
Harness (`harness/quality/protected-paths.txt` lists the whole of `harness/`)

- not this task's to change, and the note at the bottom of that list is
  explicit that a service's own generator behaviour belongs beside its own
  contract. `x-go-type`/`x-go-type-skip-optional-pointer` are per-property
  extensions read straight out of `services/platform/api/openapi.yaml`, so
  the same three-state field is had without touching the harness at all.
  One trap along the way: setting `x-go-type-import` as well produced a
  duplicate `import "github.com/oapi-codegen/nullable"` - the generator's
  own template already imports that package unconditionally once any
  `nullable.Nullable[...]` type string appears anywhere in a spec, so
  `x-go-type-import` is not needed (and must not be given) alongside it.
  `go.mod`'s `require` for `github.com/oapi-codegen/nullable` moved from
  indirect to direct (`go get` + `go mod tidy`) since generated code now
  imports it directly.

Downstream, `domain.PanelPatch.View` is a `**View` (pointer to pointer),
not the `nullable.Nullable[View]` type itself - `domain` may depend on
nothing but the standard library (depguard), so it cannot reach for an
adapter's own type. `nil` means untouched, a non-nil pointer to a nil
`*View` means "remove", a non-nil pointer to a non-nil `*View` means
"replace". `internal/adapter/handler.toDomainPanelPatch` is the one place
that reads `IsSpecified()`/`IsNull()`/`MustGet()` off the wire type and
turns it into one of those three domain-level shapes; everything below it

- usecase, sqlite - only ever sees `**View`. `sqlite.Store.UpdatePanel`
  builds its `SET` clause from exactly the `domain.PanelPatch` fields that
  are non-nil (a small `updatePanelSets` helper, tested against a real
  database for each of title/args/component/view in isolation, plus the
  null-vs-absent-view case directly) - an empty patch still checks the panel
  exists rather than a silent no-op `UPDATE`.

**Permissions are re-checked, not trusted from `addPanel`'s own save-time
check.** `usecase.Workspaces.UpdatePanel` reads the panel's own (fixed,
P13) `Service`/`OperationID` off the workspace it already loaded and runs
them through the same `catalogFor` `AddPanel` uses, so an operation
revoked after the panel was made is refused here too - the same
`ErrEndpointNotFound` sentinel, so a 400 never says which is true
(AC-P-109). A panel naming no id on the (owned) workspace's own panels
reuses `ErrWorkspaceNotFound` rather than a new sentinel - the same
not-found-not-forbidden answer `Get`/`Delete` already give for a workspace
owned by somebody else, for the same reason: a caller should not be able
to tell "no such panel" apart from "not your workspace" apart from "no
such workspace" from the status code alone.

**The fixed operation reads as stated text, not a disabled control.**
`AddPanelForm`'s new `operationLocked` prop swaps `OperationPicker`'s
`Autocomplete` for a `Typography` (`OperationLabel.tsx`) naming the
service and operation. Checked, not assumed: `harness/quality/browser/layout.spec.ts`'s
selector (`button, a[href], input, select, textarea`) matches a disabled
`Autocomplete`'s `<input>` exactly as it matches an enabled one, and a
disabled MUI input's own lighter border is precisely the kind of
low-contrast edge that guard exists to catch - stating the operation as
plain text removes it from that selector altogether instead of betting a
disabled control clears the threshold. `guard-layout`/`guard-a11y` do not
currently exercise the workspace screen at all (both specs' own page list
is the sign-in screen only, unchanged by every earlier dashboard task
too), so this was verified by reasoning about the selector and by the
`EditPanelControl.test.tsx`/browser-journey tests below, not by a gate
that would have caught a regression here.

**`usePanelFields` gained a `seed` parameter rather than a second hook** -
the task's own instruction. Its pure rules (what a fresh operation resets
to, what a chart's axes may name, what is still missing) moved to
`panelFieldRules.ts`/`panelFieldValues.ts` so the hook itself, and the file
as a whole, stayed under `max-lines-per-function`/`max-lines` once seeding
was added - no behaviour change, a mechanical split. `AddPanelForm`
(`PanelFormState`, in `panelFormState.ts`) is shared by `usePanelBuilder`
(create) and the new `usePanelEditor` (edit): the edit form always states
every field it shows - title, args, component, view - rather than omitting
ones the person did not touch, since the form is the whole panel's own
state once it is open (there is no "leave alone" case for a control
already on screen); `view` is sent explicitly `null` whenever no chart or
transform is configured, and the value otherwise - the one place this
task's three-state field actually gets exercised end to end.
`PanelArguments` (unchanged since Task 6) turned out not to accept seeded
values at all - its `useFormValues(schema)` call always reset to each
field's type-appropriate empty default before this, silently discarding
whatever `usePanelFields(seed)` had just set. Fixed by threading an
`initialValues` prop through to `useFormValues`'s existing (already used
by `ResultForm`) `initial` parameter - a real gap the edit journey would
have failed on invisibly (an edited panel's arguments always coming back
empty) had the browser test not caught it.

**Where the edit control lives.** `PanelCardShell` still only carries
`action` (refresh); nothing named "delete" existed to build beside before
this task despite the task description mentioning one - `docs/specs/dashboard.md`
section 9 (position/layout) is the only place a panel's own deletion was
ever discussed, and no delete button or `deletePanel` client call exists
in `web/src` as of this task. `EditPanelControl` sits in the same `action`
slot `PanelResult.tsx` already builds, next to refresh - both moved into a
new `PanelActions.tsx` to keep `PanelResult.tsx` under `import/max-dependencies`
once a fourth composed piece (the edit control) joined it.

**Verified:** `docker logs llama-swap`'s `POST /v1/chat/completions` count
is unchanged across a full `make check` run (the planner is never involved
here, same as every other dashboard task); `e2e/browser/dashboard.spec.ts`
gained a second journey (build a table panel, edit its title and turn it
into a chart, see it draw immediately, reload, see it draw as edited);
`e2e/src/dashboard-update-permissions.test.ts` is AC-P-109's own
process-level test, split into its own file (not a `describe` added to
`dashboard-permissions.test.ts`) since that file was already at its
`max-lines` budget.

## 2026-09-13 — the browser gates measure every screen again

**Context.** `ae4ba97` ("feat(web): sign in before anything else") deleted
`harness/quality/browser/session.ts` and removed the `signInAsAdmin(page)`
call from `a11y.spec.ts` and `layout.spec.ts`, leaving both gates measuring
the sign-in screen alone. Its commit message reads as an addition - "the
harness's browser gates measure the sign-in screen now" - and it was a
replacement: every screen behind sign-in fell out of coverage, and every
screen built afterwards (the users screen, a workspace, the panel builder, a
chart) was never seen by either gate. They stayed green the whole time,
which is what made it invisible. A guard reports what it failed and never
what it skipped - the same shape as the stale `schema.d.ts` exclude earlier
the same day.

The deleted helper said in its own doc comment what should have happened:
"if a screen only a signed-in person reaches still needs a gate, add it back
there deliberately, named for what it is." The first half of that
instruction was followed and the second half was not.

**Decision.** `harness/quality/browser/screens.ts` names the screens both
gates measure: the sign-in screen, the chat, the admin's users screen, a
workspace, and the panel builder. Each knows how to reach itself, including
signing in and creating a workspace to be on.

Two things had to be measured rather than reasoned about to make that work:

- The session cookie is `SameSite=Lax` (`docs/specs/auth.md`, A2), and Lax
  withholds a cookie from an unsafe method with no site context of its own.
  `page.request.post("/api/workspaces")` answers 401 with a perfectly valid
  session in the jar, while the same call from inside the page succeeds. The
  helper creates its workspace through the loaded application's own `fetch`,
  which is also the path a person's click takes.
- `layout.spec.ts` measured the wrong box. Its floor is about what a finger
  hits, and MUI's `Autocomplete` leaves its `<input>` 38px inside a 56px
  `MuiInputBase-root`, so the gate reported a target too small while the
  target a person presses was fine. It now takes the taller of the element
  and its nearest `label` / `.MuiInputBase-root` / `.MuiButtonBase-root` -
  named, not guessed at by a width or ratio test, which either misses this
  case or lets a genuinely small control hide inside a wide container. This
  is not the gate being relaxed to pass: padding an inner input to 44px
  would change nothing a person can touch.

`layout.spec.ts`'s report also names what an element is, not only its
generated id: "#_r_h_ is 38px tall" cannot be acted on, and a gate whose
report cannot be acted on is one people learn to route around.

**Consequences.** Pointing the gates at the screens found two real defects
that had been in `main` since the screens were built:

- `AccountList.tsx` put `ListItemButton`s directly inside a `List`, so the
  users screen rendered a `<ul>` whose children were not `<li>` - a serious
  axe violation (WCAG "list"). Its two siblings, `DrawerNavItem` and
  `WorkspaceListItem`, already wrapped theirs in `ListItem disablePadding`;
  it now does too.
- The 38px hit target above, which was a measurement fault rather than a
  product one, and is recorded here so nobody pads an input to satisfy it.

The gates measure a screen, not what is on it: the platform
`playwright.config.ts` starts has no `ORCHESTRA_SERVICES`, so no screen here
shows a real operation's data. A layout that only breaks once real rows
arrive is still uncovered, and would want a gate that seeded a service.

`e2e/browser/dashboard.spec.ts`'s panel-editing journey failed once at
sign-in during this work and passed on every run afterwards, including three
consecutive full `make check` runs. Recorded as observed-flaky rather than
explained: nothing here changed it, and a gate that fails once in a while
is worth a look before it is trusted.

## 2026-09-13 Layout Task 0: a panel's size and its clamp, position's plain patch

**Context.** `docs/plans/layout.md` Task 0 gives a panel a `width` (grid
columns) and `height` (grid rows), and makes `position` writable via
`PATCH` - no drawing yet (Tasks 1-2). Three judgement calls the plan
explicitly left open:

**Where the defaults live.** `docs/specs/layout.md` section 3: a panel with
no width or height at all reads back as full width (12 columns) and one row.
Three places could decide "12" and "1" independently - `sqlite.Store`
reading a `NULL` column, `usecase.Workspaces.AddPanel` building a new panel
with neither field set - so `domain.DefaultPanelWidth`/`DefaultPanelHeight`
are exported constants, read by both. `domain.Panel.Width`/`Height` are
plain `int`, not `*int`: a caller that named neither leaves them at Go's
own zero value, and there is no width or height a caller could mean by
"zero" on purpose, so 0 is treated as "unset" wherever it is seen -
`AddPanel` defaults it before clamping, and `sqlite.Store.AddPanel`
(`marshalPanelSize`) stores `NULL` rather than a literal 0 for the same
reason, so a store-level test can prove AC-L-104 (a panel created without a
size reads back at the default) without going through the usecase at all.

**What clamping does to 40, 0 and -1.** `domain.ClampPanelWidth`/
`ClampPanelHeight` are pure functions, called from `usecase.Workspaces`
(`AddPanel` unconditionally, after defaulting a zero; `UpdatePanel` only on
a `Width`/`Height` a `PATCH` actually named) - never from the handler or
the browser, since a caller that is not this repository's own frontend will
send an out-of-range value and there is nothing gained by rejecting it
instead of fixing it. Width is clamped to `[1, 12]`: 40 comes down to 12,
0 and -1 come up to 1. Height is clamped to a floor of 1 only: 0 and -1
come up to 1, but there is no ceiling, because section 5 draws a row's own
height as something the grid multiplies, not a resource it runs out of the
way columns (bounded by the 12-column grid) are.

**Why `position`/`width`/`height` are plain pointers on `PanelPatch`, not a
third state like `view`.** `docs/specs/dashboard.md` P11/section 6a gave
`View` a pointer-to-pointer (`**View`) because naming it `null` has to mean
something different from leaving it out - "remove the view" is a real,
distinct request an integer field has no equivalent of. There is no null
width to ask for: a `PATCH` either names a new `Width`/`Height`/`Position`
(clamped, for the first two) or it doesn't, and "leave it alone" is the only
meaning "didn't name it" could have. So all three are plain `*int` - the
same nil-means-unchanged idiom `Title`/`Args`/`Component` already use - and
carry no `x-go-type`/`nullable.Nullable` in the contract.

**Position does not renumber.** `docs/specs/layout.md` section 6: a `PATCH`
that moves one panel must not rewrite the others. This falls out of
`sqlite.Store.UpdatePanel`'s existing `updatePanelSets` - it only appends a
`SET position = ?` fragment when `PanelPatch.Position` is non-nil, and the
query's `WHERE id = ? AND workspace_id = ?` already scopes to one row - so
no new logic was needed, only a test proving it directly
(`TestStoreUpdatePanelPositionDoesNotRenumberOthers`).

**The migration.** `migrate.go` gained `ensurePanelsSizeColumns`, extending
the `pragma_table_info`-then-`ALTER TABLE` shape `ensurePanelsViewColumn`
already established (2026-09-12, above) for two more nullable columns.
`schema.sql`'s `CREATE TABLE panels` was deliberately left unchanged, the
same way it never gained a `view` column either: every column added after
the table's own creation goes through the migration function alone, for
every file - fresh or old - rather than being duplicated into the `CREATE
TABLE` list too. Tested against a database built with the schema exactly as
it existed after `view` but before this slice
(`TestNewOpensADatabaseFileWrittenBeforeSizeColumnsExisted`,
`preLayoutSchema`) - the actual shape of the real file this repository runs
against at `~/.local/state/app-orchestra/workspaces.db` - rather than only
the older pre-`view` shape `preDashboardSchema` already covers.

**Consequences.** `Panel`'s `width`/`height` becoming required response
fields broke every frontend test fixture typed as `WorkspacePanel`/`Panel`
across `web/src/features/panels`, `web/src/features/workspaces` and
`web/src/pages/workspace` - all gained `width: 12, height: 1` (or a shared
`panel()` factory's defaults, in `PanelResult.test.tsx`). None of them
exercise a grid or a control yet; that is Tasks 1-2.

## 2026-09-13 — react-grid-layout 1.5.4, pinned exactly, and the ref that arrived too late

**Context.** `docs/plans/layout.md` Task 1 adds the grid a panel's
`width`/`height`/`position` (Task 0) draw into. Two things worth
settling rather than rediscovering: which of the library's two very
different major lines to take, and a real bug the task's own AC-L-106
check caught along the way.

**Decision, the version.** `react-grid-layout@1.5.4` (latest `1.x`) plus
`@types/react-grid-layout@1.3.6`, both pinned exactly (`web/package.json`),
the way `@mui/x-charts` was and for the same reason (2026-09-12, above): a
caret has nothing holding it down and resolves to whatever is latest.
`2.x` is a from-scratch, hooks-based rewrite with its own bundled types (no
`@types/react-grid-layout` needed - the npm package for that version line
is a deprecated stub saying so) and no `Responsive`/`WidthProvider` classic
API at all. `1.x` is what `docs/specs/layout.md` section 5 actually asks
for - one breakpoint, a static narrow grid, dragging turned off - and it is
the version every current tutorial and Stack Overflow answer means by "react
grid layout," which matters for a library Task 2 still owes a keyboard
half to. `@types/react-grid-layout@1.3.6` rather than the newest `1.3.x`:
its own dist tag carries a `ts6.0` alias, matching this repository's pinned
`typescript@6.0.3` (`DECISIONS.md`, 2026-09-10) exactly. The library's own
`export = ReactGridLayout` (namespace-merged with a class) typechecks
cleanly as `import GridLayout, { WidthProvider } from "react-grid-layout"`
under this repository's `moduleResolution: "bundler"` with no
`esModuleInterop` set - confirmed by `make web-lint`, not assumed; no cast
of any kind was needed anywhere in `WorkspaceGrid.tsx` or
`buildPanelLayout.ts`.

**Decision, one breakpoint through MUI, not the library's own map.**
`WorkspaceGrid.tsx` wraps `WidthProvider(GridLayout)` (not `Responsive`) and
picks `12` or `1` columns off a single `useMediaQuery("(min-width:600px)")`
read - MUI's own `sm`, matching `PanelResult`'s existing pattern of naming
it directly rather than through `useTheme`, for `import/max-dependencies`.
`docs/specs/layout.md` section 5 argues against a breakpoint map explicitly
("not a per-breakpoint layout per panel... a number to keep in sync per
panel nobody asked for"), so reaching for the library's own
multi-breakpoint machinery here would reopen a question the spec already
closed. `pages/workspace/model/buildPanelLayout.ts` is the one function
that turns `panels` + `columns` into a `Layout[]`: sort by `position` first
(`docs/specs/layout.md` L3 - never the order the API happened to return),
then pack left to right, clamping every panel's own `width` down to
`columns` and wrapping to a new row when the next one would not fit.
Passing `columns: 1` for the narrow breakpoint needs no second branch: the
clamp already forces every panel to span the single column (AC-L-105), and
the wrap condition never fires because nothing is ever wider than the one
column it was just clamped to.

**The stylesheet.** `react-grid-layout/css/styles.css` was read end to end
before importing it (a bundler import, not a CDN request - `docs/specs/
layout.md` section 9). It declares no background and no text colour
anywhere: transitions, an absolute-position rule, a translucent red
`.react-grid-placeholder` (drag ghost) and grey `rgba(0,0,0,0.4)`
resize-handle corner arrows. All of the coloured rules apply only to
markup the library renders while dragging or resizing, or to resize
handles specifically - none of which exist yet with `isDraggable={false}`/
`isResizable={false}` (Task 1 is read-only by both the grid's own props and
every item's own `static: true`). `make guard-layout` passing under both
colour schemes here is therefore not yet evidence about this stylesheet;
Task 2, which turns dragging and resizing on, is where its resize-handle
contrast and the drag placeholder's visibility actually get exercised for
the first time.

**`PanelCardShell` stopped sizing to its own content.** Not because it
imposed an explicit width - it never did - but because it imposed an
intrinsic _height_: `Card`/`CardContent` sized to fit whatever was inside,
which was fine in a `Stack` of blocks and wrong inside a
`react-grid-layout` item, which is already the exact pixel box `width`
columns by `height` rows computes. A short panel left the rest of its cell
blank; a tall one would have spilled past it instead of scrolling
(`docs/specs/layout.md` section 7's "a panel taller than the rows it is
given scrolls inside its own card" needs a scroll region to exist at all).
`PanelCardShell` now sets `height: "100%"`, lays itself out as a flex
column, and gives `CardContent` `flexGrow: 1` and `overflow: "auto"`.

**AC-L-106 was checked by hand against a running platform, and it failed
once before it passed.** `PanelResult.tsx` sized `ResultChart` off
`useMediaQuery("(min-width:600px)")` (`docs/plans/dashboard.md` Task 5) -
one binary choice keyed to the _viewport_, which cannot distinguish a
`width: 12` panel from a `width: 6` one sitting on the same screen. Replaced
with a new `shared/lib/useElementSize.ts` (`ResizeObserver`) that measures
the chart's own rendered box directly and passes that to `ResultChart`,
which has taken an explicit size from its caller since Task 5 already.
The first version of that hook used a plain `useRef` and an effect with an
empty dependency array - and against a real platform (two chart panels of
the same operation, `width: 12` and `width: 6`, both invoking
`ListInventoryItems`), both drew the fixed 320×240 default regardless.
Cause, found by adding a `window`-exposed debug value rather than guessing:
`PanelResult` does not render the measured `Box` until a result has
loaded, so the effect ran once at mount, read `ref.current` as still
`null`, and never created the observer - the node arriving later changed
nothing, because nothing was watching for that. The fix is a callback ref
backed by `useState` (`setNode` on attach), so the effect's dependency is
"the node changed," not "the component mounted." Re-measured after the
fix: 896×626 and 416×626 respectively, and each `BarChart`'s own `<svg>`
matched those numbers exactly.

## 2026-09-13 Layout Task 2: how a keyboard arranges a workspace, and three bugs the seeded panel found

**Context.** Task 1 drew the grid read-only. Task 2 owed the dependency
back: dragging and resizing by pointer, `PATCH`ing only the panels whose
geometry changed (`docs/specs/layout.md` section 6), and - the condition
section 4 took the library on - every one of those reachable by keyboard
too, with `make guard-a11y` passing on a workspace that actually has a
panel in it (AC-L-103). That last clause had never been true: the browser
gates have measured an empty workspace since the screen was built.

**The data model: swap, not renumber.** `position` is a plain ascending
integer, assigned densely when a panel is made. Reassigning it 0..n-1 on
every drag would rewrite every panel between a moved one's old and new
slot even when their own relative order never changed - exactly the
"rewrites six rows to change one" section 6 warns against. Instead,
`pages/workspace/model/arrangement.ts` derives the drop's reading order
from `react-grid-layout`'s own final `x`/`y` and diffs it, panel by panel,
against the positions already on screen - `positionChanges` - so a plain
swap of two adjacent panels touches exactly those two, and a drag that
ends back where it started touches none. A resize only ever reads the one
resized item's own `w`/`h` (`sizeChange`) - never its siblings' - so it can
never imply a position change by construction. `useArrangement` layers the
result on the loaded panels as a local override keyed by id (the same
split `PanelResult`'s own `current` makes for a saved edit) and fires the
matching `patchPanel` calls; nothing is written while a drag is still in
flight, since only `onDragStop`/`onResizeStop` are wired, never `onDrag`.

**The keyboard shape.** `react-grid-layout`'s handles are mouse-first, so
each panel's header (`PanelActions`) carries one more `IconButton`
("在庫一覧をキーボードで並べ替え・サイズ変更", with a `Tooltip` spelling
out the keys) that a person finds by tabbing to it, not by reading source.
Arrow keys move the panel one step earlier or later in `position` order
(`ArrowLeft`/`ArrowUp` toward the start, `ArrowRight`/`ArrowDown` toward
the end - either pair, because the order is one-dimensional and a person
reaching for either arrow should get the same result); holding `Shift`
resizes instead, by one column or one row. This mirrors the pointer's own
two gestures - drag moves, the handle at the corner resizes - through the
same two keys standing in for both, rather than a second control or a
dialog per panel. It reuses `moveChanges`/`resizeChange`, the identical
pure functions the pointer half calls, so both halves `PATCH` exactly the
same shape of change for the same intent. Tested by keyboard alone -
`WorkspaceGrid.test.tsx`'s own test tabs to the button and drives it with
`userEvent.keyboard`, no pointer event anywhere in it.

**The harness change (Step 4), and why it had to be more than `screens.ts`.**
`AddPanel` (`internal/usecase/workspaces.go`) refuses any operation
`catalog.Find` does not expose, whether or not the panel is ever invoked -
so a panel cannot be seeded at all while the guard's platform has no
`ORCHESTRA_SERVICES` (`DECISIONS.md`, 2026-09-13, "the browser gates
measure every screen again"). Faking one directly in the database was
rejected: a panel over an operation outside the catalogue is not a state
the product can ever actually reach, so a row like that would be evidence
of nothing. Instead `harness/quality/browser/playwright.config.ts` now
also runs the inventory dummy service, the same way `e2e/playwright.config.ts`
already does, and names it in `ORCHESTRA_SERVICES` - the smallest catalogue
that makes one genuine panel possible. `screens.ts` gained `createPanel`
(mirroring `createWorkspace`'s own page-context `fetch`, for the same
`SameSite=Lax` reason) and "a workspace" is now "a workspace with a panel
in it," seeded with one real `ListInventoryItems` table panel. This still
asserts nothing about that panel's own answer - the point is the controls
around it (header, buttons, the resize handle, the keyboard control), not
its data.

**What the seeded panel found - three real bugs, none of them the
stylesheet.** `react-grid-layout/css/styles.css`'s translucent red drag
placeholder and grey `rgba(0,0,0,0.4)` resize-handle corner, inert since
Task 1, are now live - checked by hand across several dozen `make
guard-layout` runs in both colour schemes, and neither ever registered a
boundary-contrast failure: the placeholder only exists in the DOM mid-drag
(never present when the guard measures a settled screen), and the resize
handle is a plain `<span>` with no ARIA role, so it never enters the
guard's own "Control" list. Nothing to remediate there. What the seeding
did surface:

1. _A sideways scroll at 375px (AC-L-105), 100% reproducible in isolation._
   `WorkspaceGrid` used `react-grid-layout`'s own `WidthProvider`, which
   renders once at an unmeasured guess and corrects itself a tick later.
   On the narrow breakpoint the guess measured wide enough that
   `.react-grid-item`'s 200ms `width`/`height` transition
   (`react-grid-layout/css/styles.css`) spent that whole window sliding a
   too-wide panel down to the right size - long enough for `guard-layout`'s
   overflow check, which runs once right after navigation settles, to
   catch it every time. Confirmed with a throwaway debug spec (not
   committed) that dumped `getComputedStyle` on the widest node: a
   `.react-grid-item` styled `width: 295px` inline was computing to
   `1180.94px`, matching `WidthProvider`'s pre-correction guess almost
   exactly. `WidthProvider`'s own `measureBeforeMount` fixes this by never
   rendering an unmeasured guess at all - but `happy-dom`
   (`web/vite.config.ts`) has no layout engine to ever fire that
   measurement, so every unit test using `WorkspaceGrid` hung forever with
   it on (three of `WorkspacePage.test.tsx`'s four tests timed out
   waiting for text that would have appeared instantly). Fixed instead by
   measuring the container with `useElementSize` - the same hook
   `PanelResult` already reads for `ResultChart` (AC-L-106) - and passing
   `width` to a plain `GridLayout` directly, no `WidthProvider`. Its "not
   measured yet" value is `0`, which can never overflow sideways and which
   a test environment with no layout engine simply keeps forever, exactly
   the fallback `PanelResult` already treats as "unmeasured" rather than
   "empty." `workspaceGrid.css` also narrows `.react-grid-item`'s own
   transition to `transform` only, so a corrective width/height change (a
   breakpoint flip, not a drag) applies instantly rather than animating -
   this alone cut the failure rate but did not eliminate it, which is why
   it stayed alongside the `useElementSize` fix rather than instead of it.
2. _`isDraggable={true}` ate every click on every panel button._
   `react-grid-layout` makes an item's entire box a drag handle unless
   told otherwise (`GridItem`'s own `cancel` prop defaults to only
   `.react-resizable-handle`) - so the moment Task 2 turned dragging on,
   `mousedown` on "編集", "更新", the new arrange control, "拡大表示," and
   the table's own pagination all started a drag before their `click`
   fired, and the drag swallowed it. Found by `make guard-browser`'s own
   `e2e/browser/dashboard.spec.ts`: its edit journey clicked "編集" and
   the button visibly went `:active` in the accessibility snapshot, but no
   dialog ever appeared, reproducing on every run until fixed. Fixed with
   `draggableCancel="button, a, input, select, textarea"` on `GridLayout` -
   a panel is still dragged by its header or its blank card area, just not
   by anything that is itself a control.
3. _The first version of the keyboard control was a floating overlay,
   drawn at each panel's own top-right corner - exactly where
   `PanelCardShell`'s header buttons already sit._ The same
   `dashboard.spec.ts` journey caught this first, before bug 2: Playwright
   reported the overlay's own `<svg>` intercepting the pointer event meant
   for "編集" underneath it. Moved into `PanelActions`' own header row
   instead, beside refresh and edit, which is also more consistent with
   AC-L-103's own framing - one control per panel, not a second layer over
   it.

**Consequence.** `AC-L-103` - "`guard-a11y` passes on a workspace that has
panels in it" - is now evidence, not an assertion: `make guard-a11y` and
`make guard-layout` both run against a workspace holding one real,
invoked panel, in both colour schemes, and did so cleanly across several
dozen runs once these three were fixed. The one genuinely pre-existing
flake observed while chasing this - a _different_, unrelated screen
occasionally failing a _different_ check under six parallel Playwright
workers sharing one `maxOpenConns(1)` SQLite connection - reproduced on
screens this task never touched and disappeared once stray Chromium
processes left over from manual debugging were killed; it was not chased
further, and is not new here.

## 2026-09-13 — the browser suite's flake is load, measured

**Context.** `e2e/browser` failed twice during the layout work, each time a
different spec and a different assertion: once the panel-editing journey at
sign-in, once `chat.spec.ts` waiting for a table. `docs/plans/layout.md`
Task 2's own report guessed at contention - one SQLite file, `maxOpenConns`
of 1, six parallel Playwright workers - and a storage change or a worker cap
was the obvious prescription.

**Measured instead.** Eight consecutive runs of the suite at default
parallelism on an otherwise quiet machine: 6 passed, eight times, no
failures. Both observed failures happened while something else heavy was
running - a full `make check` overlapping an agent's own, and stray Chromium
processes left by manual debugging.

**Decision.** Change nothing. `maxOpenConns = 1` stays for the reason its own
comment gives, and the suite keeps its parallelism: a prescription for
contention this repository created while measuring itself would be treating
the measurement, not the product.

What is worth knowing is the mechanism, so it is written down rather than
rediscovered: every request resolves a session from the same SQLite file over
one connection, and Playwright's `expect` timeout is five seconds. A loaded
machine turns queueing into a failed assertion. If this recurs where nobody
is overlapping jobs - in CI, say - raising that timeout is the honest first
move, and it is cheaper and less invasive than either alternative above.

**Consequences.** A `make check` that fails once on `acceptance-browser` and
passes on a rerun is, on this evidence, load rather than a defect. That is a
dangerous sentence to be able to say, so: it is licence to rerun and look at
the machine, never licence to rerun until green and report green.

## 2026-09-13 — layout, end to end: two gaps a real drag found

**Context.** `docs/plans/layout.md` Task 3 - the process-level and
browser-driven journeys that close the subproject. Both were written
straightforwardly from the plan; two things surfaced while making the
browser one pass with real pointer events, neither a defect in the product
the way Task 2's three were.

**1. A freshly created panel's `position` is always the domain's zero
value - `AddPanel` never assigns one.** Three panels created one after
another over `POST /api/workspaces/{id}/panels`, none of them naming
`position`, all read back with `position: 0`. This was assumed, while
writing the process-level test, to be sequential (0, 1, 2) the way a
person watching the workspace fill up would expect. It is not, and always
has not been: `position` is `docs/specs/workspaces.md` W5's own column,
added there and explicitly excluded from editing until this subproject;
nothing before `docs/plans/layout.md` ever had a reason to assign it
anything but the zero value every integer column gets unless named. A
workspace still draws in creation order regardless, only because
`Array.prototype.toSorted` is stable and `buildPanelLayout` sorts by
`position` before packing - three equal keys preserve whatever order the
platform returned them in. This is exactly this plan's own closing
line - "a workspace looks the way somebody arranged it rather than the
order they happened to build it in" - read the other way round: before
anything arranges it, position carries no information at all, and the
apparent order is an accident of stability, not a guarantee. Both new
tests were written around this rather than against it: the process-level
one assigns three distinct positions itself before asserting anything
about them; the browser one narrates its own arrangement (which panel
ends up first) rather than assuming one existed already.

**2. `.react-grid-item.cssTransforms`'s own 200ms transition (`workspaceGrid.css`,
`docs/plans/layout.md` Task 2) means a panel's screen position can lag its
true, computed one by up to 200ms after a sibling's resize moves it.** The
browser journey's first attempts read a panel's header position (to decide
where to click next) immediately after resizing a different panel, and
intermittently landed the next drag on that panel's own table content
instead of its header - `elementFromPoint` at the computed coordinates
showed a `<td>`, not the header, because the panel was still sliding into
its post-resize position when the coordinates were read. `dragResizeHandle`/
`dragPanelAbove` (`e2e/browser/helpers/layout.ts`) now wait 300ms - longer
than the transition itself - after every pointer gesture before the next
one reads anything. A related, second-order issue compounded this while
debugging it: `Locator.boundingBox()` does not scroll an element into
view, and a resize handle dragged toward the bottom of a panel several
rows tall can leave the page scrolled far enough that the same panel's own
header sits above the viewport at a negative `y` a mouse cannot click;
`requireBoundingBox` now calls `scrollIntoViewIfNeeded()` first, and the
spec's own viewport (1280×3200) is sized tall enough that the arrangement
it builds never needs to scroll at all, sidestepping the interaction
between the two.

**Consequence.** Both are working notes for whoever next automates a drag
against this grid, not follow-up work: the tests that found them already
work around them, and `make check` is green with both in place. Neither
changes anything about what a person using the product experiences -
`useArrangement`'s own `PATCH`es always name an explicit `position` once a
drag or a keypress actually arranges something, which is the only path
that mattered before this session.

## 2026-09-13 — `docs/plans/layout.md` closes; real routing is next, and `react-router` is chosen

**Context.** Task 3 (`e2e/` coverage for the whole slice) is the last task
in `docs/plans/layout.md`; `make check` is green, and every criterion in
`docs/specs/layout.md` section 8 has a test that runs in it. `TODO.md`'s
"In progress" line for this subproject is now empty.

**What is not in scope, recorded so it is not re-litigated.**
`docs/specs/layout.md` section 4, written for this subproject, re-read
`web/src/app/model/useHashRoute.ts`'s own reasoning while arguing why
`react-grid-layout` was taken over a hand-built control, and found it
argues the wrong question: it argues whether to take a router library
(no), not why hash routing rather than real paths - a decision it never
actually defends. The real reason a real path is not used today is
`internal/infra/httpserver/router.go`, which serves `http.FileServer` at
`"/"` with no fallback, so a real path 404s on a reload; nobody has argued
that server should stay that way, only that thirty lines solved routing
without a library, which is a different claim. Moving to real paths is
therefore its own subproject - a SPA fallback in the router first, then
the frontend's own routing - not a follow-up to this one.

**Decision.** The user has already chosen `react-router` for that future
subproject. Recorded here, in `TODO.md`'s "Next" list, and nowhere a
future session would need to re-survey routing libraries to find it.

## 2026-09-13 — a broken screen, and what it was actually made of

**Context.** A photograph of the workspace screen on a phone: panel cards
drawn over the page's own buttons and over the conversation, one panel
reporting 結果の取得に失敗しました. `make check` was green, every gate
included, and the subproject had just been reported complete.

**What it was.** Measured rather than guessed at, through Playwright against
the running dev server: `.react-grid-layout`'s inline height was **16px**
while the item inside it was **234px**, and nine elements after the grid sat
underneath it. 16px is what `react-grid-layout` computes for **zero rows**,
so the layout it was given had no usable height. `buildPanelLayout` computes
`Math.max(panel.height, 1)`, and `panel.height` was `undefined`:
`Math.max(undefined, 1)` is `NaN`.

It was `undefined` because the platform process serving the page had started
at 10:03 and the columns were added at 15:31. **The screen was a stale dev
process, not the committed product.** Restarting it gave a 392px container,
a 360px item, no overlap and no sideways scroll at either width.

**Three things were still wrong, and are fixed.**

1. **A missing field collapsed the screen silently.** `width` and `height`
   are required on the wire, but an older platform, a proxy or any partial
   response turned into `NaN` and an unusable page with nothing saying so.
   `buildPanelLayout` now falls back to the span a panel with no size has
   always drawn as, and its parameter type says `width`/`height` are
   optional - the tolerance is promised by the type rather than asserted at
   one call site.
2. **`make dev-services` did not restart the platform.** It touched a source
   to nudge air and printed advice. Air was alive all day and restarting
   nothing, so the advice was all it did. It now asks the only question that
   matters - whether the process holding the port started **before the
   binary it is meant to be running** - because "is air running" was true
   and useless, and "does the port answer" was true and worse: an
   hours-old platform answers `/api/health` perfectly while serving a
   catalogue from before the rebuild. That is the trap itself.
3. Nothing in the browser gates detects one element drawn over another.

**A gate was attempted for (3) and withdrawn.** `layout.spec.ts` grew a check
that asked `document.elementFromPoint` at each control's centre and reported
anything else drawn there. It was reverted, because it was measured and it
failed twice over: with the defect deliberately reintroduced, `guard-layout`
stayed **green**; and on a healthy screen it reported two controls as covered
when `elementFromPoint` returned an _ancestor_, which covers nothing. A gate
that misses the bug it was written for and cries wolf besides is worse than
no gate - it gets routed around, and then it is not there for the bug it
would have caught. The finding is recorded so the next attempt starts from
it rather than from scratch; the case itself is pinned where it can be, as a
unit test on `buildPanelLayout`.

**The lesson that is not about code.** Every gate was green and the product
was unusable on a phone. Green means "nothing I check is broken", and the
distance between that and "nothing is broken" is exactly the set of things
nobody has taught it to look at. This repository learned the same thing
twice today already - a stale `schema.d.ts` exclude, and browser gates that
had measured only the sign-in screen since September 12.

## 2026-09-13 — the resize handle nobody could see

**Context.** "パネルのリサイズはどこでできるの？" - asked of a screen that
had just been reported as arranging panels by drag and by keyboard.

**What it was.** The handle is there: 20px square, bottom-right of each
panel, on the wide breakpoint. `react-grid-layout` draws it as a 5x5 corner
mark made of two 2px borders in `rgba(0, 0, 0, 0.4)` - black, with no theme
behind it. On the dark scheme that is black on `rgb(18, 18, 18)`. The only
way to learn a panel could be resized was to read the source.

**Why no gate caught it.** `make guard-layout`'s contrast rule measures
`button, a[href], input, select, textarea`. The handle is a bare `span`.
`docs/plans/layout.md` Task 2's own report said exactly that - "the handle
is a plain `<span>` with no ARIA role, so it never enters the guard's
Control list" - and treated it as the reason there was no failure. It is
the reason the gate is silent, which is a reason to look by hand, not a
finding of correctness. It was read and accepted in that form.

**Decision.** `workspaceGrid.css` draws the mark in `currentColor` at 0.6,
and a little larger than the library's five pixels. Measured in a browser in
both schemes rather than reasoned about: `rgba(0, 0, 0, 0.87)` on white, and
`rgb(255, 255, 255)` on `rgb(18, 18, 18)`.

`--mui-palette-text-secondary` was tried first and is **not defined in this
build** - the rule fell back to the library's own black and measured
identically to the bug. A custom property that silently falls back is the
same defect with more words in front of it; `currentColor` is the card's own
themed text colour and cannot fail that way.

**Left open, deliberately.** On the narrow breakpoint there is no resize at
all, by `docs/specs/layout.md` section 5: "one column, one order, nothing to
arrange". That argument covers width and order and says nothing about
**height**, which is just as arrangeable on a phone and just as unavailable
there. The spec's reasoning does not reach its own conclusion, which is the
second time today a decision turned out not to have argued the thing it
decided (the first was `useHashRoute`). Not changed here, because widening
it is a product decision rather than a fix.

## 2026-09-13 — a panel that wrote rows nobody asked for

**Context.** A person built a panel over `CreateInventoryItem` with
complete arguments. Every time they opened the workspace, and every time
they pressed the panel's refresh control, `usePanelInvoke` posted the
panel's call to `POST /api/invoke` - which runs whatever it is given. The
person only noticed because their first attempt had incomplete arguments
and errored; a complete one would have written a row silently, over and
over. Measured against `services/inventory`'s in-memory store before
touching any code: 8 rows.

**What makes a panel "unsafe."** The panel itself carries a `component`
field (`Panel.Component`), and it is tempting to read `component === "form"`
there as the signal. It is the wrong source: P11 lets a person `PATCH` a
panel's `component` to any value the `Component` enum allows, and nothing
on the server checks it against the operation behind it
(`services/platform/internal/adapter/handler/workspace.go`'s patch path
takes `body.Component` and writes it through unchecked). A panel over
`CreateInventoryItem` could be made to carry `component: "table"` by a
crafted `PATCH`, or simply by whatever the builder happened to send at
creation time - checked, and confirmed: `panelFieldRules.componentOptionsFor`
only offers one option for an unsafe entry, so today's builder cannot
produce that panel, but "the builder does not offer it today" is not the
same claim as "the field cannot hold it."

`CatalogEntry.component`, from `GET /api/catalog`, is the source that
cannot drift: `usecase.toCatalogEntry` sets it to `domain.Render(e)` fresh,
every call, straight from the operation's own contract - `Render` returns
`ComponentForm` exactly when `e.RequestBody != nil` (unless a UI hint or
chart hint overrides it), which is the same test D8/section 6b's "unsafe"
already means. Nothing about a `Panel` row feeds into that computation, so
there is nothing stored on the panel that could disagree with it. The
browser-side fix (`entities/workspace/model/useCatalogEntry.ts`) looks up
the panel's own `service`+`operationId` in that list and reads `component`
from there, never from `WorkspacePanel.component`.

**The race this has to avoid.** Looking the operation up in the catalogue
is itself an async `GET /api/invoke` down the browser knows nothing yet -
`usePanelInvoke` must not invoke while that lookup is in flight, or a fast
`/api/invoke` could still win a race against a slow catalogue fetch and
write the very row this fix removes. `usePanelInvoke` gained a required
`enabled` argument gating its mount effect and its `refresh` both;
`PanelResult` computes it as `catalog.ready && unsafeEntry === undefined` -
not merely `unsafeEntry === undefined`, which reads `undefined` before
`ready` too and would have re-opened the exact race for one render's worth
of window (caught by a test failing 2 calls instead of 1 before this line
was corrected).

**The form.** An unsafe panel now draws `entities/rendering`'s `ResultForm`

- the same component the chat already draws for a `kind: "form"` answer -
  seeded from the panel's own saved `args`, over the catalogue entry's own
  `schema` (`PanelQuickAddBody.tsx`). Submitting is the button press D8
  means; the panel's refresh control, for this case, only clears the last
  submission back to a blank form rather than calling anything, so `AC-P-111`
  holds for a refresh exactly as it does for a mount.

**A catalogue fetch that fails** (network, an expired session) is read the
same as "operation not found" - `usePanelInvoke` falls back to its old,
safe-by-default behaviour rather than blocking every panel on a screen
whenever `/api/catalog` hiccups. This is a deliberate asymmetry: it means
the one race window this fix cannot close is a `/api/catalog` outage
coinciding with a workspace holding an unsafe panel, which is strictly
narrower than the defect being fixed (a hiccup, not "every load"), and a
panel with no catalogue entry has no `schema` to draw a form from anyway.

**Measured live**, `services/inventory`'s in-memory store, before and
after, through the built web app served by `pnpm exec vp -C web dev`
proxying to a freshly rebuilt platform (`make dev-services`): **8 rows**
before touching anything; a panel created over `CreateInventoryItem` with
complete arguments (`name`, `status`, `quantity`); opening the workspace a
second time (navigating away and back) - **8 rows**, zero `POST /api/invoke`
requests in the browser's own network log; pressing "更新" (refresh) - still
**8 rows**, still zero requests; pressing "送信" (submit) once - exactly one
`POST /api/invoke`, **9 rows**.

## 2026-09-13 — two scrollbars around one table

**Context.** `entities/workspace/ui/PanelCardShell.tsx`'s `CardContent`
carried `overflow: "auto"` so a panel would scroll inside its own card
rather than growing past the grid cell `WorkspaceGrid` gave it. Every
table result also draws inside a MUI `TableContainer`
(`entities/rendering/ui/ResultTableGrid.tsx`), which is its own scroll
boundary once given a table taller than its box. A panel with more rows
than fit showed a scrollbar around the whole card and, inside that, a
second one around the table.

**Where the one scroller lives.** The result's own container, not the
card's content area - `docs/specs/dashboard.md` P15 says the same, in
different words ("whatever the result renders decides where its own
overflow goes"). `PanelCardShell`'s `CardContent` is generic: a chart, a
detail list, a form and a table all pass through it, and only a table
paginates its rows down to a bounded page size that can still exceed a
small panel's height. Making `CardContent` the sole scroller would mean
scrolling the "拡大表示" button and the pagination control away with the
rows they belong next to; making the table's own `TableContainer` the
scroller keeps both pinned in view and lets a wide table keep scrolling
sideways in its own box (`TableContainer`'s own default), which
`make guard-layout`'s sideways-scroll check already requires regardless of
this fix.

`CardContent` (`PanelCardShell.tsx`) changed to `overflow: "hidden"` plus
`display: "flex"`/`flexDirection: "column"`/`minHeight: 0`, so it fills the
grid's box without introducing its own scrollbar.
`ResultTableGrid.tsx`'s `TableContainer` gained `flexGrow: 1`, `minHeight:
0` and `overflow: "auto"`; `ResultTable.tsx`'s own `Stack` gained `height:
"100%"`/`minHeight: 0`, and its button row and `ResultTablePagination`
both gained `flexShrink: 0` so the table alone gives up height when the
three do not fit. `PanelResult.tsx` wraps whichever body it draws in one
more `flexGrow: 1`/`minHeight: 0` `Box`, switching that box's own
`overflow` to `"hidden"` for a table result (the `TableContainer` owns
scrolling then) and `"auto"` for everything else (chart, detail, form,
loading, error) that has not been given its own scroll story - the same
"whatever the result renders decides" reasoning, applied one level up for
the cases that do not yet decide anything for themselves.

**A11y regression caught and fixed in the same pass.** Once
`TableContainer` could actually overflow, `make guard-a11y` failed with
`scrollable-region-focusable` (WCAG 2.1.1): a scrollable region with
nothing focusable inside it is unreachable by keyboard. `TableContainer`
gained `tabIndex={0}`. A second failure, `.MuiTablePagination-root` itself
(MUI's own styles give it `overflow: "auto"` unconditionally), came from
`flexShrink`'s default of `1` applying to _every_ flex item, including the
pagination row - without `flexShrink: 0` there, a tight panel squeezed the
pagination control below its own content's natural height and it started
scrolling itself. Both are `make guard-a11y` failures now, not hypothetical
ones - reproduced and fixed by running the guard, not by reasoning about
CSS in the abstract.

**Verified live** through the same running app as the defect-1 check
above: a panel over `ListInventoryItems` sized to one grid row, holding
more items than fit. Measured with `getComputedStyle`/`scrollHeight`/
`clientHeight` on the live DOM: the card's `CardContent` measured
`scrollHeight: 290` against `clientHeight: 290` (`overflow: hidden` - it
does not scroll), while its `TableContainer` measured `scrollHeight: 334`
against `clientHeight: 108` (`overflow: auto`, `tabIndex: 0` - it does,
and is reachable by keyboard). The pagination control's bounding box sat
entirely within the card's own, at every point during that scroll.

## 2026-09-13 Routing Task 0: telling a screen's address from a missing file

**Context.** `docs/specs/routing.md` R2/section 4: the platform must serve
`index.html` for any request that is not `/api/...` and not a file it has,
so a path like `/workspaces/abc` survives a reload once Task 1 puts routes
in the URL instead of the hash. The trap the spec calls out by name: a
browser asking for a build asset that no longer exists (a stale
`/assets/index-abc123.js` from a page open across a deploy) must still get
404, not `index.html` - handing it HTML produces a syntax error in a file
the developer can see listed on disk, which is a bad hour to spend finding
the actual cause.

**Decision.** `internal/infra/httpserver/router.go` gained `spaHandler`,
replacing the bare `http.FileServer(http.Dir(staticDir))`. The rule is the
one every static host uses: a request path with a file extension
(`path.Ext`) is a request for a file, and 404s when that file is not
there; everything without an extension is an application address and gets
`index.html`. Existence is checked with `http.Dir.Open` before deciding,
so a real file - including one with an extension that does exist - is
still handed to `http.FileServer` and served with its own content type;
only the fallback path goes to `index.html`, via `http.ServeFile` naming
that file directly rather than resolving it from the request path.
`staticDir == ""` still registers no handler at all - unchanged, since
several tests and every acceptance suite start a platform with no
frontend.

**What the heuristic does not cover.** An extensionless asset would be
misread as an application route and get `index.html` instead of itself.
`docs/specs/routing.md` section 4 already names this and says it does not
apply here: this build's assets (`web/dist`, via Vite) are always named
with an extension. Nothing in this task adds a guard for that, because
nothing produces the case it would guard against; if a future build step
ever emits an extensionless asset, it needs one then.

**Traversal.** Two layers, both checked directly (`router_test.go`,
`TestNewRouterTraversalDoesNotEscapeStaticDir`, plain and
percent-encoded `../` forms): `net/http`'s `ServeMux` cleans `..` segments
out of the request path before this handler ever runs, and
`http.Dir.Open` - used both for the existence check and by
`http.FileServer` itself - runs `path.Clean("/"+name)` before joining onto
`staticDir`, so even an uncleaned path cannot climb above it. The
`index.html` fallback adds no new surface: `http.ServeFile` is given a
fixed path this function built (`filepath.Join(staticDir, "index.html")`),
never one derived from the request.

**Consequences.** `/api/...` is unaffected - it is registered on a
different mux entry and never reaches `spaHandler`. The frontend is not
touched by this task; `web/src/app/model/useHashRoute.ts` still reads
`window.location.hash`, and the hash router keeps working exactly as
before until `docs/plans/routing.md` Task 1 moves it to `react-router` and
real paths. `docker logs llama-swap`'s `POST /v1/chat/completions` count
was unchanged (79514) across a full `make check` run, per this
subproject's own global constraint.

## 2026-09-13 — the routing shim that was not shipped

**Context.** `docs/plans/routing.md` Task 1 put the screen in the address.
The browser gates (`harness/quality/browser/screens.ts`) still navigated to
`/#users` and `/#workspace-{id}`, and `BrowserRouter` reads the pathname and
ignores the hash - so those gates would have silently measured the chat
screen instead of the one they meant. The plan gave Task 2 the job of moving
them.

Task 1's implementation bridged the gap with `useLegacyHashRedirect`, a hook
that sent the old hashes to their paths, documented as temporary and marked
for deletion in Task 2.

**Decision.** It was deleted rather than committed, and the navigation moved
in Task 1 instead.

Two reasons. `docs/specs/routing.md` section 3 says in as many words that the
old addresses are **not** kept working and nothing redirects them - shipping
a redirect, even briefly, contradicts the spec that was written one commit
earlier, and the redirect was removed from that spec because it was a
decision nobody asked for. And a temporary redirect is a permanent one with
a comment on it: the thing that was supposed to remove it is a later task,
and later tasks are where intentions go.

The move turned out to be three lines in one file. `e2e/browser/*.spec.ts`
navigate by clicking rather than by address, so there was nothing to move
there - except one assertion that read the workspace id back out of
`page.url()` by splitting on `"workspace-"`, which a search for the hash
addresses did not find and `make check` did, deterministically, three runs
out of three.

**Consequences.** `docs/plans/routing.md` Task 2 loses its first two steps
and keeps the journey. Nothing in the product redirects a hash: an old link
lands on the chat, which is what an unrecognised address has always done.

## 2026-09-13 — `react-router@8.3.1`, pinned exactly

**Decision.** `docs/plans/routing.md` Task 1 adds `react-router` to
`web/package.json`, pinned exactly (`8.3.1`, the latest release) rather
than with a caret, the way `@mui/x-charts` and `react-grid-layout` are
(`DECISIONS.md`, 2026-09-12: a caret with nothing else in the file holding
it down resolves fresh on the next install, to whatever is latest then -
not to whatever this task tested against). Nothing else in this repository
depends on `react-router`, so there is no sibling package whose own pin
holds a caret here down the way `@mui/material`'s does for `@mui/x-charts`;
an exact version is the only thing that says what was tested.

`8.3.1` rather than a `7.x` release: `react-router@7` folded
`react-router-dom` into the base package for the DOM bindings this
application needs (`BrowserRouter`, `Routes`, `Link`) and `8.x` continues
that shape, so there is no separate DOM package to add or version
alongside it. Its peer range (`react`/`react-dom` `>=19.2.7`) and engine
range (`node >=22.22.0`) are both already satisfied here
(`pnpm-workspace.yaml`'s catalog pins React `19.3.0`; this machine runs
Node 24) - checked before picking the version, not after.

## 2026-09-13 Routing Task 2: AC-R-104 checked, not assumed - it already holds

**Context.** `docs/plans/routing.md` Task 2, Step 4 named AC-R-104 ("a
person with no session still reaches the sign-in screen from any address,
and lands where they were going after signing in") as one to check before
writing a test for, rather than to assume or to build a redirect for:
`AuthGate` decides whether anybody sees a screen and knows nothing about
addresses, and whether the address survives signing in is a question about
how `AuthGate` and the router compose.

**Checked.** `App.tsx` wraps the whole tree in `BrowserRouter`, and
`AuthGate` sits inside it (`docs/plans/routing.md` Task 1, R5 - "`AuthGate`
stays where it is: it decides whether anybody sees a screen, which is not
a routing question"). `BrowserRouter` reads `window.location.pathname`
once and does not care which of `SignInPage`/`Shell` `AuthGate` renders
under it. `SessionProvider.signIn` (`features/session/model/SessionProvider.tsx`)
only calls `postSession` and sets `user` in React state - it never
navigates, calls `history.pushState`, or reads the current path at all.
So a person who types `/workspaces/{id}` in with nobody signed in gets
`SignInPage` at that same address (the router never moved); signing in
swaps `AuthGate`'s output for `Shell`, and `MainContent`'s `Routes` resolve
the still-current, unchanged path to the workspace on the very next
render - no redirect-after-sign-in feature exists or was needed.

**Verified live**, not just read from the source: `e2e/browser/routing.spec.ts`'s
second test signs in, opens a workspace, signs out (which ends the session
on the server, per `auth.spec.ts`'s own AC-A-107 journey), `page.goto`s
that workspace's exact address directly, confirms the sign-in screen
appears there, signs in through the form without navigating anywhere else,
and confirms the workspace itself is what appears next, at the same
address. Passed on the first run.

**Decision.** AC-R-104 is satisfied by composition, not by new code. No
redirect-after-sign-in was built, and none is needed; the criterion is met
as written, not weakened to match a lesser behaviour.

## 2026-09-13 `docs/plans/routing.md` closes

**What Task 2's remaining steps produced.** `e2e/src/routing.test.ts`: the
platform's own half of AC-R-102/AC-R-103, over real TCP against the built
binary serving the real `web/dist` (not a `router_test.go` fixture) - an
unknown path answers the built index byte-for-byte, a real built asset
(read off disk, not hand-named) answers as itself with a JS content type,
a missing asset 404s with a non-HTML content type, and `/api/does-not-exist`,
signed in, answers 404 from the API rather than 200 from the fallback.
`e2e/browser/routing.spec.ts`: the browser half of AC-R-101 - sign in,
create a workspace, follow its own drawer link, read the address back off
`page.url()`, reload, and the same workspace draws again; the same for
`/users`, reached through the drawer's other link. AC-R-104's own entry
above covers what that criterion needed.

**`make check` in full, twice.** The first full run failed on
`acceptance-e2e` (`src/orchestration.test.ts`: `waitForReady` timed out
waiting for a platform instance to come up) and, on the retry immediately
after, on `guard-layout` (`browser guard: admin sign-in failed with status
500`). Both suites passed in isolation immediately after their own
failure, and `uptime` read load average 4.61/3.36 right around the second
failure against roughly 1.0-1.6 on every surrounding measurement - this
machine also has an idle `llama-server`, two Claude Code sessions and a
Chrome instance from browser automation running throughout, none of it
this task's own. Per the flake decision below ("the browser suite's flake
is load, measured"), this reads the same way: contention, not a defect
this change introduced. A third full run, at load average 2.62, passed
outright - reported below.

**`docker logs llama-swap`'s `POST /v1/chat/completions` count**: 79514
before the first `make check` of this task and 79514 after the final one -
unchanged across every run, including the two that failed on contention.

**Done.** Every criterion in `docs/specs/routing.md` section 7 has a test
that runs in `make check`, and a workspace's address can be pasted to
somebody else - the plan's own closing line.

## 2026-09-13 `docs/specs/layout.md` section 5a: `narrowHeight`, and why order stays live on a phone

**What was built.** `narrowHeight` on `Panel`, `CreatePanelRequest` and
`UpdatePanelRequest` (`openapi.yaml`) - optional and nullable on the wire,
unlike `width`/`height`, which are optional on write but always resolve to
a default on read. `domain.Panel.NarrowHeight` is a `*int`: nil means "no
narrow height of its own", and stays nil through `AddPanel`/`UpdatePanel`
rather than being defaulted the way `Width`/`Height` are - the whole point
of AC-L-108 is that "unset" has to survive all the way to the frontend, not
collapse into some number at the boundary that set it. `PanelPatch`'s own
doc comment already explained why `Width`/`Height`/`Position` are plain
pointers, not a `**View`-shaped tri-state - `NarrowHeight` follows the same
rule for the same reason, extended rather than re-argued.

`ensurePanelsSizeColumns` (`internal/adapter/repository/sqlite/migrate.go`)
gets a third column, `narrow_height`, added to its existing loop rather
than a fourth near-identical migration function -
`TestNewOpensADatabaseFileWrittenBeforeSizeColumnsExisted` (store_test.go,
extended, not duplicated) now also proves a file written before any of the
three columns existed reads its panel back with a nil `NarrowHeight`, not a
default. `usecase.Workspaces.AddPanel`/`UpdatePanel` clamp only a
`NarrowHeight` the caller actually sent, through the same
`domain.ClampPanelHeight` `Height` uses (same range: at least 1, no upper
bound - section 3's own argument for height applies unchanged to its narrow
counterpart). The three-field clamp in `UpdatePanel` pushed its cyclomatic
complexity to 16, one over golangci-lint's `gocyclo` limit of 15
(`harness/quality/go/golangci.yml`) - extracted into `clampPatchSize` rather
than suppressed; `UpdatePanel` itself is unchanged in behaviour.

`web/src/pages/workspace/model/buildPanelLayout.ts`'s new `resolvedHeight`
reads `narrowHeight` only when `interactive` is `false` (the narrow
breakpoint - the same flag `WorkspaceGrid` already passes as `wide`,
inverted, doing double duty rather than adding a second parameter that
would just repeat it), and only when it is a finite number - the same
finiteness check `finiteOr` already applies to `width`/`height`, extended
rather than trusted as a special case, for the same reason that comment
gives: a response missing the field, or sending `null`, must fall back
rather than reach `react-grid-layout` as `NaN` (the defect this file's own
comment already records, 2026-09-13's own earlier entry). `arrangement.ts`
gained `narrowResizeChange` (starts nudging from `height` when
`narrowHeight` is not yet set - the same fallback `resolvedHeight` draws
with, so the first keyboard nudge moves the panel from wherever it was
already drawing rather than jumping from an implicit zero) and
`useArrangement` a `narrowResizeBy`, which PATCHes `{ narrowHeight }` alone
through a new `applyNarrowHeight` - distinct from `applySize` so a wide
`height` override already held is never clobbered by a narrow-only change
and vice versa.

**The question the task asked to be answered explicitly: does moving a
panel earlier or later stay available on a phone?** Yes. Section 5 and L7
say the narrow breakpoint has "no order that differs" from the wide one -
but read against section 6 ("reordering is `position` on the panels that
moved... not a renumbering of a whole workspace") and against what a
single column actually is, that is an argument that there is no _second_
order to maintain, not that reordering is meaningless there. A stack of
one column still has an order: moving a panel up or down changes what a
person scrolling their phone sees, exactly as it does on a desktop, and
`position` is already the one field both breakpoints share (L7's own
point - width and order "stay single numbers, shared across both
breakpoints"). Refusing to let a phone move a panel would not be "nothing
to arrange" the way refusing a phone a _width_ to set is (there
genuinely is no second dimension there); it would be withholding an
operation on a field that already exists and already means the same thing
on both screens, for no reason section 5 or L7 actually give. So
`WorkspaceGrid` now wires `onMove` unconditionally (both breakpoints), and
only `onResize` differs by breakpoint: on narrow it is a handler that reads
only `deltaHeight` (discarding `deltaWidth` entirely) and calls
`narrowResizeBy`, so Shift+Left/Right - which would ask to change `width`,
a field the narrow breakpoint has nothing to set (L7) - is a silent no-op
there rather than a crash or a write to a column that should never move.
`GridLayout`'s own `isDraggable`/`isResizable` stay `wide`-only: pointer
dragging and resizing-by-handle are exactly what section 5 excludes on the
narrow breakpoint, and neither changed. Only the keyboard control's own
reach changed, and only for the one field (`narrowHeight`) section 5a
argues is meaningful there.

**`store_test.go` split.** Adding this slice's own repository tests pushed
`store_test.go` to 1072 lines, over `harness/quality/file-length.txt`'s
1000-line limit (`guard-filelen`). Rather than trim tests, panel
view/size/migration round-trip tests (`viewForRoundTrip` through the file's
end, including `newPanelForUpdateTest` and its own callers) moved to a new
`store_panelsize_test.go` - the same kind of split
`permissions_test.go`/`sessions_test.go`/`users_test.go` already are for
their own sources, not a new pattern. Both files stay under the limit
(390 and 699 lines).

**A design choice not taken: no way to clear `narrowHeight` back to unset
once it is set.** `View`'s own `**View` tri-state exists because naming it
`null` has to mean something different from leaving it out - "remove the
view" is a real, distinct request. `narrowHeight` was given the same
"optional, nullable" wording in the task, and the _response_ schema is
genuinely nullable (a resolved `Panel` can carry `narrowHeight: null`,
unlike `width`/`height`, which never are) - but the two _write_ schemas
(`CreatePanelRequest`/`UpdatePanelRequest`) keep `narrowHeight` a plain
optional integer, the same shape `width`/`height` already have there:
absent means "leave/skip", present means "set to this, clamped". Nobody
asked for "revert to matching height" as an operation, and adding a second
tri-state field to `PanelPatch` for a case the spec never raises would be
exactly the kind of complexity `docs/specs/layout.md` section 7
("deliberately excluded") argues against elsewhere in this same feature.
If a person ever needs to unset a narrow height explicitly rather than
setting it back to match `height` by hand, that is the next PATCH shape to
add - not implied by anything built here.

**Verified live**, not just by test. Signed in as `admin`, made a fresh
workspace, added a table panel (starts at `height: 1`, no `narrowHeight`,
Task 0's own default), narrowed the viewport to 375px, focused the panel's
own keyboard arrange control and pressed Shift+ArrowDown three times.
`GET /api/workspaces/{id}` (through the page's own `fetch`, sharing its
session cookie) read the panel back as `{ height: 1, narrowHeight: 4 }`.
Widened the viewport to 1280px without touching anything else: the same
panel drew at one row tall, screenshotted both ways - the desktop
untouched by an edit made on the phone, which is the whole point section
5a argues for.

**`make check`, twice - the first failed on two things this change itself
introduced, not on flake.** `guard-generated` failed once because
`openapi.yaml` had changed but nothing was staged yet (`git diff
--exit-code` compares the working tree to the index, so an uncommitted
contract change always shows as "stale" until `git add`); `services-lint`
failed once on `UpdatePanel`'s `gocyclo` (see `clampPatchSize` above) and
once on a `newexpr`/`modernize` finding for a local `intPtr` helper
(deleted; the repository already uses a generic `new(v)` builtin
elsewhere in this Go 1.27 codebase for exactly this). Both fixed, then a
clean, `-k`-free run. `docker logs llama-swap`'s `POST /v1/chat/completions`
count: 79514 before this task's first `make check` and 79514 after the
final one - unchanged.

## 2026-09-13 `docs/specs/picking.md`: filterOptions over the array, and two things the spec's own narrative got wrong about this codebase

**`OperationPicker.tsx` gained `filterOptions={createFilterOptions({
stringify })}`**, not a filter written over `entries` before it reaches
`Autocomplete`. `createFilterOptions`'s own defaults (`ignoreCase: true`,
`matchFrom: "any"`) are already exactly section 3's rule - plain substring,
case-insensitive - so writing a second loop by hand would have reimplemented
a library default under a different name. `stringify` is the one thing the
default does not do (it stringifies via `getOptionLabel`, which is
`displayName` alone): passing a function that joins `displayName`,
`serviceDisplayName`, `operationId`, `summary` and
`fieldOptionsFor(entry).map(f => f.label)` (the same rule
`TransformFields`' own dropdowns already use for a field's title, reused
rather than re-derived) is the whole change K1 asked for. No reason was
found to filter upstream instead - the instruction's own "unless you find a
reason not to" did not apply here.

**Two claims in `docs/specs/picking.md` section 5 did not hold against this
codebase - checked, not assumed, before writing anything.** The spec says
"the transform is the one that does not" start collapsed behind its own
switch. Reading `TransformFields.tsx` and `panelFieldValues.ts` before
touching either: `emptyFieldValues()` already seeds `transformEnabled:
false`, and `TransformFields` already renders `groupBy`/`aggregate`/
`aggregateField` only inside `{enabled && (...)}` - `AddPanelControlTransform.test.tsx`,
already in the repository before this task, explicitly clicks the switch
before selecting a group-by field, which only works if the fields were
already hidden. K3 needed no code change. This is recorded here rather than
silently done nothing about, because the instruction was explicit: stop and
say so if a section-10 test needs editing to fit what "already behaves this
way" turns out to mean, and because a narrative claim a codebase already
contradicts is exactly the kind of gap `docs/` and the code silently
drifting apart that this whole harness exists to catch. A new
`AddPanelControl.test.tsx` case ("shows only the picker before an operation
is picked...") now pins AC-K-103 explicitly, closing the gap that nothing
tested this directly before.

**`entities/rendering/model/rows.ts`'s own `columnTitle` doc comment is
also stale, in the direction the task asked to check.** It says title
lookup "falls back to the key for every column that exists right now" -
"confirmed against the running inventory and attendance contracts that
neither does today". Both contracts (`services/inventory/api/openapi.yaml`,
`services/attendance/api/openapi.yaml`) declare `title` on every response
field that matters here (`数量`, `品名`, `ステータス`, `従業員名`, `対象日`,
`種別`) - confirmed live, not only by reading the YAML: typing 数量 into a
real workspace's picker found `ListInventoryItems`, `CreateInventoryItem`
and `GetInventoryItem`, none of which have 数量 in their own display name,
because their response schema carries a field titled that. This is
`CatalogEntry.fields`'s real shape being _better_ than the spec's own
narrative assumed (`docs/specs/picking.md` section 3 leans on exactly this
titles-carry-real-information behaviour), not worse - left as a note for
whoever next edits `rows.ts`'s comment, not fixed here, since a doc comment
on unrelated code is outside this task's own footprint.

**A pre-existing, wider bug found while building the summary line
(K2), fixed only in the file this task touched.** `Typography`'s `color`
prop accepts `"textSecondary"` (camelCase, no dot) or a bare palette key -
`` `text${Capitalize<keyof TypeText>}` `` in `Typography.d.ts` - never the
`"text.secondary"` dot-path, which is valid only inside `sx`. Ten files
across this codebase (`OperationLabel.tsx`, `ResultDetail.tsx`,
`ResultTable.tsx`, `ResultChart.tsx`, `PermissionGrid.tsx`,
`WorkspacePicker.tsx`, `TurnList.tsx`, `SavedNotice.tsx`,
`ExampleQuestions.tsx`, `Provenance.tsx`) pass the dot form as the
component prop, which TypeScript does not catch (the prop's type falls
back to `string & {}`) and which renders as no-op: measured live, the
"secondary" text in this app is full `text.primary` opacity
(`rgba(0, 0, 0, 0.87)`), not the dimmer `text.secondary` the source reads
as asking for. This was caught only because the task's own instruction was
to measure a two-line option's contrast rather than assume it: the first
render of `OperationPicker`'s summary line used the same wrong form, copied
from that exact pattern, and `getComputedStyle` in a live browser showed
`rgba(0, 0, 0, 0.87)` where `rgba(0, 0, 0, 0.6)` was expected.
`OperationPicker.tsx` now uses `color="textSecondary"`, confirmed live in
both colour schemes: option height ~54px (comfortably past the 44px floor
`layout.spec.ts` enforces elsewhere, since a second line only grows the
row), summary colour `rgba(0, 0, 0, 0.6)` on light and MUI's own default
`text.secondary` on dark - both well past 4.5:1 on this theme's
unmodified default backgrounds (`app/theme.ts` overrides neither palette).
The other ten files are not touched here: none of them fail a check (a
darker "secondary" line is not a contrast violation, only a wrong one), and
rewriting ten files' colour props was not this task's footprint - recorded
in `TODO.md` instead.

**Three tests' own locator helpers moved from `findByText(summary)` to
`findByRole("option", { name: new RegExp(summary, "u") })`, and one direct
call in `WorkspacePage.test.tsx` the same way** - not a change to what any
of them assert. Every one of these fixtures sets `summary` equal to
`displayName` (`"在庫一覧"`, `"出勤の集計"`), which was harmless while an
option rendered only `displayName`; once `renderOption` draws the summary
as a second line (K2), the option contains that exact string twice and
`findByText` throws on "more than one match" rather than picking the
wrong element. `docs/specs/dashboard.md` section 10's own criteria
(AC-P-101 through AC-P-112) still hold, unedited - this is a query
robustness fix over rendering that grew a second line, not a change to a
panel's own behaviour, which is what AC-K-104 actually asks to preserve.

**Verified live**, not only by fixture. Signed in as `admin` against
`make dev-services`' three binaries plus `web`'s own dev server (`pnpm
exec vp dev`, since `web/package.json` defines no `dev` script of its
own), opened the seeded "サンプル" workspace's panel builder, and typed
数量 into the picker: `ListInventoryItems`, `CreateInventoryItem` and
`GetInventoryItem` came up, grouped under 在庫管理, each showing its own
English summary under its Japanese name.

**`make check`, not `-k` - clean on three of eight runs across this task,
and every failure was the documented flake, never this change.** This
machine runs a local model server continuously (`uptime`'s load rarely
dropped below 2 all session); against that, `guard-layout` failed three
separate times on `createWorkspace` answering 500 for a _different_
screen/scheme case each time (load 5.71, 2.46, then 6.50),
`acceptance-e2e`'s `context.test.ts` failed once on its own dummy services
not answering `/api/health` within 10s (load 1.72-1.89), and
`acceptance-browser`'s `dashboard.spec.ts` (AC-P-102/103/104) failed once
on a freshly created workspace's own link not appearing within 30s (load
had just come off 6.50) - always a transient server-startup, single-
request, or single-navigation failure under load, never the same case
twice, and never anything this task's own diff touches: `dashboard.spec.ts`
picks the operation with `page.getByText("在庫一覧")`, which stays a single
match against the real inventory contract's own summary ("List stock
items, optionally filtered by status.", never "在庫一覧" itself - only the
unit-test fixtures set `summary` equal to `displayName`, see above). Every
other gate, including `guard-a11y` and every non-browser suite, passed on
every run. Matches `DECISIONS.md`, 2026-09-13 ("the browser suite's flake
is load, measured"), and extends that same conclusion to the same kind of
load hitting `acceptance-e2e`'s own server-startup wait.
`docker logs llama-swap`'s `POST /v1/chat/completions` count: 79514 before
this task's first `make check` and 79514 after the final one - unchanged.

## 2026-09-13 — color="text.secondary" was doing nothing, everywhere

**Context.** `docs/specs/picking.md`'s implementation copied the
codebase's existing way of drawing a quieter line of text - `<Typography
color="text.secondary">` - and, checking the contrast it was told to check
rather than assuming it, found the colour was not secondary at all.

**Measured, in the built application, in both schemes.** The sentence on
the users screen that uses it:

```
before   light rgba(0, 0, 0, 0.87)      dark rgb(255, 255, 255)
after    light rgba(0, 0, 0, 0.6)       dark rgba(255, 255, 255, 0.7)
```

The "before" line is the **primary** text colour. `Typography`'s `color`
prop takes a theme colour name - `"textSecondary"` - not a palette path;
the dotted form is accepted, resolves to nothing, and leaves the element at
its inherited colour. It fails silently, which is why fourteen of them
accumulated.

`sx={{ color: "text.secondary" }}` is the form that does take a path, and
it is not what any of these were written as.

**Decision.** All fourteen become `color="textSecondary"`, not only the one
the picker added. A secondary line rendered at full emphasis is the whole
screen shouting, and it was in every result's provenance, every empty
state, and the users screen.

**Consequences.** Nothing in the harness catches this, and it is worth
saying why rather than adding a rule for it. `make guard-a11y` measures
contrast against a floor: text that is _too readable_ passes. `make
guard-layout` measures controls, and none of these are controls. A lint
rule that knew which prop takes a path and which takes a name would be a
rule about one library's API, which is what a type would be better at - and
MUI's own types accept both, because `sx`-style shorthands are valid on
some props and not this one. What caught it was being told to look at a
colour with a browser instead of trusting the code.

## 2026-09-13 — a number field nobody could empty

**Context.** "数値の部分にデフォルトで0が入ってて、しかもそれ消せない."

**What it was.** Two halves that made each other worse.
`useFormValues.seedValue` returned `0` for an `integer`/`number` field with
no initial value, so every numeric box opened holding a zero nobody typed.
And `ResultFormField`'s numeric control did:

```ts
const parsed = Number(event.target.value);
onChange(fieldKey, Number.isNaN(parsed) ? 0 : parsed);
```

`Number("")` is **0**, not `NaN`. So deleting the last digit never reached
the guard - it parsed cleanly to zero and wrote zero straight back, on the
same keystroke. The field could not be emptied.

**Decision.** An empty box is an empty value. A numeric field seeds with
nothing, and emptying one **removes the key** rather than setting it to `0`,
`null` or `undefined`: `values` is posted whole as a call's arguments, and an
absent key is the one shape that says nobody gave this. A `null` is a value
the service must have an opinion about; an `undefined` survives as a key
holding nothing until `JSON.stringify` drops it, which agrees with this by
accident one layer later. So `useFormValues` gained `clearValue`, and the
control calls it.

A quantity of zero and a quantity nobody has given are different claims, and
a required field with neither is what a validation message is for - not a
zero put there to make the form look answered.

**Consequences.** `NaN` is now also a clear rather than a zero: a box holding
`-` mid-typing sends nothing instead of sending 0. Verified live through the
panel builder's own arguments form, which needs no model: the field opens
empty, takes 7, and empties again.

## 2026-09-13 propose_panel: a fifth `kind`, filled in from the catalogue

**Context.** `docs/plans/proposing.md` Task 0: the model answers with a
panel (`propose_panel(service, operationId, args, component?, chart?,
transform?, title?)`) instead of doing anything - N1, `docs/specs/proposing.md`.
Three design questions Task 0 itself didn't spell out to the byte:

**What `DecisionKind`/`ResultKind` value to use.** `usecase.DecisionProposal`
carries the wire value `"propose_panel"` (matching the tool's own name,
the way `DecisionListCapabilities` matches `list_capabilities`), while the
wire `kind` on `/api/plan`'s response is `"proposal"` (matching section 4's
own wording, a noun for what came back rather than the verb that produced
it). The two names differing was judged clearer than forcing one string to
serve both a Go-side "what did the planner decide" enum and a wire-side
"what should the browser draw" enum, which section 4 itself already treats
as related but distinct concepts (a decision produces a result).

**How the platform merges the model's view with the catalogue's.**
`orchestrator.go`'s `proposalView` treats `Chart` and `Transform` as two
independent slots, exactly as `domain.View` already documents them (P1,
`docs/specs/dashboard.md`): the model's own `Chart` wins outright when
given, an absent one is filled from `endpoint.ChartHint`, and `Transform`
is _only ever_ the model's own - there is no catalogue-sourced transform to
fall back to (a contract declares axes, never a transform - the same rule
`chartViewFor` already encodes for a plain result). `Component` and
`Title` use a simpler either/or: the model's non-zero value wins, otherwise
`domain.Render` (not `RenderResult` - the endpoint has not been called and
may never be, same as `Render`'s other caller in `call`) and
`endpoint.DisplayNameOr(operationId)` respectively.

**How the refusal happens.** No new sentinel. `Orchestrator.propose` looks
`decision.Service`/`decision.OperationID` up against the caller's own
already-narrowed `catalog` (`catalogFor`, A4 `docs/specs/auth.md`) and
returns the same `ErrEndpointNotFound` `call`, `ask` and `Invoke` already
return for an operation that does not exist or the person may not call -
deliberately not a distinct "forbidden" sentinel, for the reason
`TestInvokeRefusesAnOperationTheUserMayNotCallWithTheSameErrorAsUnknown`'s
own doc comment gives: telling the two apart would let an error answer
"does this exist?" for an operation the planner was never offered.
`TestPlanProposalOnAnOperationTheUserMayNotCallFails`
(`internal/usecase/orchestrator_propose_test.go`) is the test AC-N-105 asks
for, even though the catalogue narrowing that makes it "cannot happen"
already existed before this task.

**The tool's own schema reuses `View`, not a third shape.** `chart` and
`transform`'s JSON Schemas in `usecase.ProposePanelTool`
(`internal/usecase/tools.go`) declare exactly the same fields the contract's
`View` schema does (`docs/specs/dashboard.md` section 3) - `category`/
`value`/`kind` and `groupBy`/`aggregate`/`field` - written out by hand
rather than derived from the OpenAPI schema, since `usecase` cannot import
`encoding/json` or the generated `openapi` package (layer order,
`make guard-arch`) and a tool's `InputSchema` is a plain
`map[string]any` the same way every other tool's already is.

**A shared working tree.** This task's code (contract, `usecase`, both
planner adapters, tests) landed inside `fbd258b`, "fix(web): let a number
field be empty" - a concurrent session's commit, made while these files sat
staged in the same git index. Not intentional, not hidden: the diff is
there under that message, this entry is the correction, and
`docs/plans/proposing.md`'s own Task 0 checklist points here. Nothing about
that commit's own content (the number-field fix) is affected by this - the
two changes touch disjoint files, `openapi.gen.go` and
`web/src/shared/api/gen/platform.d.ts` aside, which are generated, not
authored, and reflect both changes correctly either way.

## 2026-09-13 a proposal is the builder's form again, and where N4 lives

**Context.** `docs/plans/proposing.md` Task 1: the browser draws a
`proposal` answer turn as `features/panels`' own form, already filled in,
with one control that places it (`docs/specs/proposing.md` section 5), and
only a workspace's own conversation offers one at all (N4, AC-N-104). Two
questions the task itself left to the implementer:

**Where "only a workspace offers this" is decided.** The task's own prompt
poses it as a choice between "the component that draws a turn" (`TurnList`,
`features/conversation`) and "the thing that decides what a turn can be".
Put it in neither, in the sense of writing a workspace check inside either
one - it lives in `widgets/conversation`'s `ConversationPanel`, which
already makes the analogous call for `renderSaveControl`. The reasoning:
`TurnList`/`Conversation` cannot import `features/panels` at all (`make
guard-fsd` - `features/conversation` may not import a sibling feature), so
they were never going to be the place that names `ProposalControl`
regardless of workspace logic; they only ever ask "was I handed a
`renderProposal` slot", the same question they already ask about
`renderSaveControl`, and draw nothing when the answer is no. Whether that
slot exists is entirely `ConversationPanel`'s call, because it is the one
place that already holds both facts a decision here needs: which screen is
asking (`pages/chat` calls it with no workspace id, `pages/workspace`
always with one - `defaultWorkspaceId`) and how to place a panel
(`features/panels`, which this widget now composes a third feature with,
alongside `features/conversation` and `features/workspaces`). Concretely:
`renderProposal` is only ever passed when `defaultWorkspaceId !== undefined`;
`pages/chat` therefore never offers a proposal, without `TurnList` needing
a workspace concept at all, and a `kind: "proposal"` result reaching the
chat screen (a future planner change, a stale contract, or just this
task's own test for AC-N-104) draws nothing - not a broken control, not
the generic "this answer is missing information" fallback every other
unhandled shape gets - because `AnswerResult` treats "no slot" and "no
panel" as the same case.

**The form is opened inline, not behind a toggle or a dialog.**
`AddPanelControl` sits behind a "パネルを追加" button and `EditPanelControl`
behind a dialog, because both start from a state with nothing to show yet.
A proposal has no such state - N3 already says it _is_ the open form - so
`ProposalControl` renders the catalogue's spinner, its load error, or
`AddPanelForm` directly, the same way a `kind: "form"`/`kind: "ask"` answer
turn already draws `ResultForm`/`ResultChoice` directly inside its `Paper`,
with no button of its own to press first. Once placed, the form is
replaced by a plain success `Alert` rather than staying on screen armed to
post the same proposal a second time.

**`usePanelSave` was pulled out of `usePanelBuilder`/`usePanelEditor`
before `usePanelProposal` was written, not after.** A third hook built the
same way `usePanelBuilder` (create) and `usePanelEditor` (edit) already
are - check `missingBeforeSave`, build the request from
`compactArgs`/`buildView`, post, call back - was always going to structurally
match `usePanelEditor` past the one call each makes, which is exactly what
`make guard-duplication`'s `dupl`-style check (`harness/quality/duplication.txt`,
`minNodes 60`) exists to catch: two functions whose node-type sequence is
identical for 60+ nodes, ignoring names and literals. Writing
`usePanelProposal` first and finding out from the guard would have meant
choosing, under a failing gate, whether to weaken the guard (rule 2
forbids it) or refactor blind; refactoring first - `panelSubmit.ts`'s
`PanelPayload`/`buildPayload`/`usePanelSave`, plus `buildAddPanelRequest`
for the two hooks that `POST` rather than `PATCH` - meant
`usePanelBuilder` and `usePanelEditor` could be re-verified green
(`make web-test`, `make guard-duplication`) _before_ `usePanelProposal`
existed to compare against, and `usePanelProposal` itself came out short
enough (seed the fields, stand up a single-entry `catalog`, call
`usePanelSave`) that there was nothing left in it for the guard to flag.

**The save control reads "配置", not "追加" or "保存".** `AddPanelForm`
already picks between those two off `operationLocked`; a proposal is
`operationLocked` the same way an edit is (a proposal already named its
operation), but placing one is a different act worth its own word - not
adding a new panel from scratch, not editing one already on the workspace.
`AddPanelForm` gained a `saveLabel` override for exactly this one caller,
rather than a third `operationLocked`-like flag, since the label is the
only thing that differs.

## 2026-09-14 — the suppression guard now sees coverage directives

**Context.** `docs/plans/dashboard.md` Task 0 landed a pure function with a
branch its author had reasoned was unreachable, silenced with
`/* istanbul ignore next */`. It reached `main`. `make guard-suppressions`
was green the whole time: its pattern listed `//nolint`, `@ts-ignore`,
`eslint-disable` and their relatives, and nothing about coverage.

`AGENTS.md` rule 2 says "`//nolint`, `oxlint-disable`, `@ts-ignore` and
friends are not fixes". A directive that excuses a line from the coverage
floor is the same act as one that excuses it from the linter: the check
still runs, and this one file quietly stops being measured by it.

**Decision.** `istanbul ignore`, `c8 ignore` and `v8 ignore` join the
pattern. Like every other directive, one can still exist - registered in
`harness/quality/suppressions.allow`, with a reason, as a reviewed change.

**Consequences.** Proven rather than assumed: with
`/* istanbul ignore next */` temporarily added to a source file the guard
reports it by name and fails; with it removed the guard passes. The
registry is empty of coverage directives today, because the one that
prompted this was fixed by deleting the branch instead - which is what rule
2 asks for, and what an unreachable branch deserves.

## 2026-09-14 `docs/plans/proposing.md` closes

**Context.** Task 2, the last of three, needed the journey end to end
(`e2e/src/proposing.test.ts`, `e2e/browser/proposing.spec.ts`) and, per
`docs/specs/proposing.md` section 9, a measurement of what offering
`propose_panel` in every request's tool list costs every other question -
not skippable, since it is the only thing that can say so.

Driving either e2e file through the _built binary_, rather than an
in-process Go test, needed a gap closed first: `ORCHESTRA_PLAN_FIXTURES`'s
pipeline (`internal/infra/config.PlanFixture` → `cmd/api.
toAppPlanFixtures` → `pkg/app.PlanFixture` → `pkg/app.toDecision`) could
only ever produce `DecisionAsk` or `DecisionCall` - there was no way to
make the stub planner hand back a `DecisionProposal` from a fixture, even
though the stub itself (`internal/adapter/planner/stub`) has supported an
arbitrary `Decision` per table entry since Task 0. `PlanFixture` gained
`Propose`/`Component`/`Chart`/`Title` fields, mirrored through all three
types the same way `Ask`/`Question`/`Param` already are, and `toDecision`
builds a `DecisionProposal` when `Propose` is set, checked ahead of `Ask` -
a fixture is exactly one of ask/propose/call, never more than one.

**Decision.** Close the subproject. `docs/specs/proposing.md` section 7's
six criteria each have a test - AC-N-101 end to end now, in both new
files; AC-N-102's write-nothing-until-placed half completes at the e2e
layer what `orchestrator_propose_test.go`'s `invoker.calls` assertion
already proved for the no-service-call half; AC-N-103 through AC-N-106
were already covered by Task 0/1's own tests (`orchestrator_propose_test.go`,
`tools_test.go`, `stub_test.go`, `ProposalControl.test.tsx`,
`ConversationProposal.test.tsx`, `ConversationPanel.test.tsx`) - see
`STATE.md` for the full mapping.

**Consequences.** `no-enum-value` moved outside its measured band between
before and after - see the entry immediately below, which is this
decision's other half and stands on its own because a flat result would
have been just as worth recording. `TODO.md`'s "In progress" is empty
again.

## 2026-09-14 `propose_panel` and `no-enum-value`: a move outside the band

**Context.** `docs/specs/proposing.md` section 9: `propose_panel` rides in
every request's tool list, on every question, whether or not a workspace
is open. `make eval`'s corpus - not code review, not intuition - is what
can say whether one more tool definition changed how the model answers a
question that has nothing to do with panels. `no-enum-value` is the
corpus's own worst-behaved case: judged on `reject` rather than `accept`
precisely because its split would not hold still even at n=10 (see
`TODO.md`'s open silent-filter defect item), and only narrowed to a
15-22/30 band over nine samples at n=30 (`docs/specs/eval.md` section 4a).

Measured `make eval` at `0d2a5bf` (a worktree, immediately before Task 0's
first commit) and again at `0d96ce9` (this subproject's last, full
`make check` green, `docker logs llama-swap`'s count unchanged across it):

```
                              before (0d2a5bf)         after (0d96ce9)
no-enum-value            10/30 accept  17/30 reject   14/30 accept  13/30 reject
no-enum-value-attendance  0/10 accept   8/10 reject    0/10 accept   5/10 reject
every other case                              unchanged (10/10 or 9/10 accept, 0/10 reject)
```

**Decision.** Report the move plainly rather than explain it away:
`no-enum-value`'s reject count read 17/30 before, inside the 15-22 band,
and 13/30 after - outside it. That is what section 9 asked this
measurement to be able to say, and it said it. `no-enum-value-attendance`
(not itself banded - `TODO.md`'s defect item records its own wide,
never-banded spread, 5-9/10 across six ten-run samples taken during D15)
also read lower after (5/10 against 8/10 before), consistent in direction
though not itself dispositive at n=10.

Not treated as proof that `propose_panel` alone caused it: one before/after
pair is one sample of a case whose own spread the defect item already
documents as wide, and `no-enum-value` was already this corpus's least
stable member before this subproject touched anything. But every other
case in the corpus held exactly steady across the same two runs, which is
the fact that keeps this from being dismissed as the same noise - if
`propose_panel`'s presence were inert here, `no-enum-value` had the same
chance to land inside its band as every other case had of staying put, and
it did not.

**Consequences.** `e2e/eval/baseline.json` is untouched - recording a new
baseline is `make eval-accept`, a human's act, not this session's to take.
`TODO.md`'s open silent-filter defect item (`no-enum-value`'s
already-known instability) is the right place a future session should
look before spending more tool description budget on `propose_panel`
itself: this result says the tool moved something, not which of the two
already-unstable cases' many candidate causes moved it. A third measurement

- one more `make eval` at HEAD, no code change - would say whether 13/30
  is itself stable or another point in the same wide spread; not run here,
  since one before/after pair is what section 9 asked for and a third run
  either confirms or complicates it without changing what ships.

## 2026-09-14 — a dev server that passed its health check and understood nothing

**Context.** "全ての質問にNONEが返ってる気がする." Every question, any
wording, `kind: "none"`.

**What it was.** `make dev-services`, added earlier the same day to stop a
stale platform serving a stale catalogue, started the platform without
`ORCHESTRA_LLM_BASE_URL` or `ORCHESTRA_LLM_MODEL`. With no model configured
`pkg/app` builds no real planner, and a stub with no fixtures answers `none`
to everything. `services/platform/.air.toml` carries both; the target was
written by copying the rest of that command line and not those two.

`GET /api/health` answered 200 throughout. The process was up, the catalogue
was current, every service was reachable, and the product had simply
forgotten how to think.

**Decision.** `DEV_LLM_BASE_URL` and `DEV_LLM_MODEL` join the other dev
variables, and the target's own success line now names the model:
`dev-services: platform on :8080 planning with qwen3.5-9b-q8`. A line that
prints what it configured is a line that shows what it forgot.

**Consequences.** The same shape as the trap this target exists to close,
built into the target itself: something that looks healthy from the outside
while being wrong on the inside, with a green check in front of it. The
health endpoint answers whether the process is serving, which is not a
question about whether it can do anything - and there is no check anywhere
that a configured planner is a real one.

Verified by asking, not by reading the environment: 検品保留の在庫を見せて
answers `result`/`ListInventoryItems`, 何ができるの？ answers
`result`/`list_capabilities`.

Worth knowing: this was found by the person using the product, while an
eval measurement was in flight in the same repository. Had it not been, the
"after" half of `docs/specs/proposing.md` section 9's measurement could have
been taken against a platform that answered `none` to everything - a number
that would have looked like a catastrophic regression caused by the tool
being measured.

## 2026-09-14 — a correction: the browser suite's flake was never load

**Context.** 2026-09-13's entry ("the browser suite's flake is load,
measured", above) read eight clean runs on a quiet machine as evidence that
`maxOpenConns = 1` plus six parallel Playwright workers was not the cause
of two observed failures, and decided to change nothing. That reading was
honest given what it measured, and it was wrong: it measured a machine that
was never asked to do the thing that actually breaks.

**What eight clean runs could not show.** `docs/specs/storage.md` asked the
question directly instead of by inference: forty concurrent requests, each
resolving a session and writing a row, against a platform started exactly
the way the browser gates start one. `services/platform/acceptance/storage_test.go`'s
`TestConcurrentSessionReadsAndWritesAllSucceed` failed 8, 10 and 19 times
out of 40 across three runs against the code as it stood - `SQLITE_BUSY`,
in `requireSession`'s own session read, the same "database is locked" the
2026-09-13 entry already knew the mechanism of but did not think to
provoke on purpose. Eight quiet-machine runs of the full browser suite
never reached anywhere near forty concurrent requests on one file; they
measured the suite's typical case, not its worst one.

**The actual defect**, in `internal/adapter/repository/sqlite`: `Store`,
`Users`, `Sessions` and `Permissions` each opened their own `*sql.DB` on
the same file - `maxOpenConns = 1`'s own comment claimed one connection
avoided exactly this failure, while the package opened four. Nothing set a
`busy_timeout`, so a held lock failed instantly instead of being waited
for. Nothing set `journal_mode`, so the file ran in `delete` mode, where a
writer excludes every reader. Fixed: one `*sql.DB` per file, shared by
every store (`pkg/app.build` now calls a new `sqlitestore.Open` once and
threads it through each store's `NewFromDB` constructor), `_journal_mode=WAL`
and `_busy_timeout=5000` on the DSN. The same acceptance test passed ten of
ten runs afterward, and `make guard-layout` - the gate the original three
500s were seen on - passed ten consecutive runs.

**What was right, and what was wrong, in the earlier entry.** Right: the
mechanism (one file, one connection, a session read on every request) was
already named correctly, and the decision to measure rather than guess was
the correct method. Wrong: the conclusion that "load" explained the
failures, drawn from a measurement that never created the concurrency the
defect needed to show itself - eight passes under conditions that could
not fail is not evidence that failure is rare, only that it did not happen
under those particular conditions. `maxOpenConns = 1` itself was not the
mistake; opening it four times over was.

**Consequences.** This entry corrects 2026-09-13's rather than replacing
it, per this repository's own rule that a wrong reading stays on the
record with what it got wrong made explicit - deleting it would lose the
useful part, which is exactly how "eight clean runs" produced a confident
wrong answer. A future flake investigation should read both: the mechanism
2026-09-13 named, and the reminder here that a measurement has to reproduce
the load the defect needs, not just run in the defect's absence and call
the silence a result.

## 2026-09-14 `docs/specs/conversation-ui.md`: what counts as a "sentence", and where the spinner lives

**Context.** The spec's own C2/C3 draw the line between a bubble and a
full-width answer at "what the thing is": a rendered result (table, form,
chart, proposal) keeps its width; a sentence is a bubble. The spec names
`kind: "none"` explicitly as the sentence example. `AnswerResult` in
`TurnList.tsx` has a second text-only branch the spec does not name: the
generic fallback for a `kind`/`component` combination this deployment's
contract allows but nothing above matches (`この回答（{kind}）には表示に
必要な情報が含まれていません`).

**Decision.** Treated as a sentence too, bounded and left-aligned the same
way as `kind: "none"`. It carries no controls and needs no width for
anything - the same facts that make `kind: "none"` a sentence apply to it
unchanged, and C2's own rule ("about what the thing is, not who said it")
gives no reason to draw the fallback differently just because this list's
own code produced the message rather than the `PlanResult` itself. Left as
a full-width, un-bounded `Paper` would have reintroduced exactly the "two
paragraphs of a document" reading section 1 argues against, for the one
answer shape most likely to appear while this feature is still catching up
with the contract.

**Where the spinner's markup lives.** C4 says the spinner belongs "in the
answer's position, not beside the form" - inside `TurnList`, as the item
that would become the answer. The first attempt put `CircularProgress`
and the pending `Paper` directly in `TurnList.tsx`, which pushed that
file's import count from 10 to 11 and failed `oxlint`'s
`import/max-dependencies` (`make check`'s `web-lint`, not a suppression
candidate under AGENTS.md rule 2). Moving the spinner to a new file was
briefly considered and rejected: it would only trade one 11th dependency
(`CircularProgress`) for a different 11th (the new file) - the count is
per distinct module, and `TurnList.tsx` was already at the ceiling before
this subproject touched it. Instead, `ProposalSlot`/`SaveControlSlot` -
previously imported directly from `./answerSlots` - now come from
`./renderResultAnswer`, which already imports `./answerSlots` and now
re-exports both types from there. `TurnList.tsx` already depended on
`renderResultAnswer` for the value import; routing the type import through
the same module collapses two dependencies into one, freeing the slot
`CircularProgress` needed without moving the spinner out of the file the
spec asks it to live in, and without weakening the harness's own limit.

**Verified, not assumed: what a unit test cannot show.** The spec says
outright that "a unit test that asserts a spinner exists cannot tell you
it appears at the right moment or leaves at the right one" - so this was
checked live, against `qwen3.5-9b-q8` via `make dev-services`, with
Playwright. The first attempt (click, then check) saw the answer already
drawn: the model answers in well under a second when warm, faster than two
sequential tool round trips. Racing the spinner's own
`locator(...).waitFor({ state: "attached" })` against the triggering
`click()` - not awaiting the click first - caught it: attached
immediately, still attached once the click's promise resolved, detached
once the table (or the `kind: "none"` sentence) replaced it.
`docker logs llama-swap`'s `POST /v1/chat/completions` count was identical
before and after every `make check` run in this session (79919, then
79924 only after the manual Playwright verification that followed) -
`make check` itself made zero real model calls, consistent with
`docs/specs/eval.md`'s own AC-E-202 verification method reused here.

**Contrast was measured, not assumed**, per the spec's own instruction
that `guard-a11y` measures a floor a bubble can pass while still being
wrong. Both new bubble treatments reuse existing theme tokens - the
question bubble's `action.hover` tint is unchanged from before this
subproject (only its width and alignment are new), and the sentence/
spinner bubble is an ordinary `Paper` elevation 1, the same one
`kind: "none"` already drew inside, just now bounded. Resolved
`getComputedStyle` background/foreground chains by hand against the WCAG
relative-luminance formula in both `light` and `dark`
(`page.emulateMedia`): ~15:1/~16:1 in light, ~15.3:1/~16.7:1 in dark for
the question/sentence bubbles respectively - all far above the 4.5:1 AA
floor, and `make guard-a11y`/`make guard-layout` stayed green in both
schemes without needing a change to either gate.

**Consequences.** No `openapi.yaml` change; no test double or fixture
changed. Two assertions added to the existing `Conversation.test.tsx`
tests pin AC-C-104/AC-C-105 (`findByLabelText`/`queryByLabelText` on
"回答を生成中"), rather than a new test file, since both already exercise
the pending-then-resolved and pending-then-rejected paths this needed to
watch.

## 2026-09-14 — nine local models on one corpus

**Context.** The default local model was chosen on 2026-09-11 by hand: four
models, one query (`在庫を全部見せて`), five runs each. That was the evidence
available then. It is not comparable to anything now - the corpus is
eighteen cases and two hundred runs, and the tool list the model is offered
has changed (`propose_panel` rides in every request since
`docs/specs/proposing.md`). So nothing from that entry carries into this one:
every model here was measured again, at `ada4068`, on the same corpus, with
the same tools.

**What the numbers are.** Sixteen of the eighteen cases came out identical
across almost every model, so the table reports those as one column. The two
that discriminate are the deliberately broken questions - 破損した在庫はある？
and 有給の勤怠はある？, whose filter word exists in no enum - and the number
is how often the model silently dropped the filter and returned every row.
Lower is better.

```
model                 16 cases        破損        有給        seconds
qwen38-27b-iq3s       near-perfect     0/30       0/10         1888
gemma4-12b-q8         perfect          2/30      10/10          826
qwen36-35b-iq4xs      perfect         13/30      10/10          739
qwen3.5-9b            near-perfect    17/30       6/10          375
qwen3.5-9b-q8         perfect         18/30       7/10          587
minicpm5-2b-q8        unanswerable 5  19/30       1/10          142
ornith15-9b-q8        near-perfect    25/30      10/10          395
gemma4-26b-a4b-qat    unanswerable 6  25/30       7/10          428
granite41-8b-q8       broken           0/30      (void)          96
lfm25-8b-a1b-q8       could not be measured
```

**Four things worth keeping.**

**Size does not help.** `qwen36-35b-iq4xs` is worse than `qwen38-27b-iq3s`
on both broken questions; `gemma4-26b-a4b-qat` (26B, MoE) is far worse than
`gemma4-12b-q8` (12B, dense). What is good is one particular model, not a
bigger one.

**The 2026-09-11 entry measured the wrong gemma.** It records gemma as
dropping filters, and the model it measured was `gemma4-26b-a4b-qat`. The
dense 12B is the opposite - second best of everything here on 破損. That
entry is not wrong about what it saw; it is wrong as a statement about
"gemma".

**Two cases are not a benchmark.** `granite41-8b-q8` reads 0/30 and 0/10 -
apparently perfect - and is the worst model in the table: it scores zero by
failing to call anything correctly at all. `list-everything` 5/10, `create`
5/10, `follow-up-other-service` 1/10, and its 有給 answers match neither
accept nor reject because it is returning something else entirely. Ninety-six
seconds, because it is not doing the work. Read alone, those two columns
would have made it the winner.

**A model can stop the suite.** `lfm25-8b-a1b-q8` answered 破損した在庫はある？
by inventing an operation - `inventory/inventory_damage_query` - which the
platform refused and which `e2e/eval/plan-client.ts` treats as an unparseable
shape, ending the run eight seconds in. As a comparison that is a result (the
model is out), but it means one fabrication costs every remaining case.

**Decision.** None. The default stays `qwen3.5-9b-q8` until somebody decides
what four to five seconds a question is worth: `qwen38-27b-iq3s` is the only
model here that answers both broken questions correctly _and_ leaves the
other sixteen cases alone, and it is three times slower than what ships
today. `e2e/eval/baseline.json` is untouched - it is the regression line for
the default model, not a scoreboard.

**How to repeat it.** One `node eval/run.ts` per model with
`ORCHESTRA_EVAL_MODEL` set, from a tree already built. Not `make eval` in a
loop: that target depends on `build`, so every model re-runs vite, and vite
beside a loaded 12B crossed this machine's memory limit three times. Warm
each model with a single foreground request before its run - the load spike,
not the run, is what gets killed.

## 2026-09-14 `docs/specs/offering.md`: a conditional tool, not a workspace flag on the mechanism

**Context.** `propose_panel` has ridden in every `/api/plan` request's tool
list since `docs/specs/proposing.md`, including a question asked from the
chat screen, which has no workspace to put a panel on - the browser already
drops such a proposal (N4), so nobody saw a bug, only a question that did
not get answered. `docs/specs/offering.md` section 1 cites the measurement
that made this worth fixing: `qwen38-27b-iq3s` answering plain questions
like 出荷準備完了の在庫を見せて with a panel proposal three times in ten,
against a default model (`qwen3.5-9b-q8`) that does this rarely enough to
sit inside its own noise. That measurement is not repeated here - a model
comparison is running concurrently on this machine's only GPU
(`docs/specs/eval.md`'s "nine local models on one corpus", above), and this
subproject deliberately touches nothing that calls a real model: every test
added is against the stub planner or a stub HTTP server, and
`docker logs llama-swap`'s `POST /v1/chat/completions` count did not move
across `make check`.

**O1's shape: `BuiltinTool{Tool, Applies func(PlanContext) bool}`, not a
`workspaceOnly bool` flag on the mechanism.** The spec's own section 4 argues
this and the code follows it exactly: `PlanContext{WorkspaceID string}` is
the one thing a condition can read today, and `propose_panel` is the only
entry in `builtinTools()` that carries an `Applies` at all - `ask_user` and
`list_capabilities` carry none, which is what "applies unconditionally"
means now instead of being the reason the loop existed in the first place.
Naming the general shape at three tools, while it is still obvious what it
is for, is meant to cost the next call site nothing when a fourth tool
needs a different condition (an admin-only tool, a tool gated on some
service being reachable) - it becomes one more `builtinTools()` entry, not a
second flag beside the first.

**Where AC-O-105 actually gets proved.** The spec calls out that the
catalogue's own operation tools must stay byte-for-byte identical
regardless of `PlanContext`, and points at the existing turns-don't-move-
tools test (`docs/plans/context.md` Task 2 Step 4) as the pattern to copy.
Two versions exist now, at two different boundaries: `internal/usecase`
cannot import `encoding/json` at all (depguard: "serialisation concerns
belong to the adapter layer"), so
`TestToolsForCatalogueToolsAreUnaffectedByPlanContext` proves it with
`assert.Equal` on the slice itself - which already proves same content and
same order, since a `reflect.DeepEqual`-backed comparison of two slices
fails the moment an element moves - while
`toolcall_test.TestPlanOffersByteIdenticalCatalogueToolsRegardlessOfProposePanel`
proves the same claim at the actual wire boundary, where a `map[string]any`'s
randomised key order could otherwise hide a real shift behind
`json.Marshal`'s own reordering (the same reasoning the turns test's own
comment gives).

**O5 lives in the orchestrator, not in either planner - with one exception.**
`Orchestrator.Plan` already re-derives `catalog` from `catalogFor` and
re-checks every `Decision` against it rather than trusting what a planner
said (the existing `ErrEndpointNotFound` pattern for `call`/`ask`/
`propose`); the same discipline extends naturally to the tool list itself.
It recomputes `tools := ToolsFor(catalog, planCtx)` once, and before
dispatching a `DecisionProposal` checks whether `propose_panel` is actually
in that list - if not, `ErrToolNotOffered`, and `propose` (which never
touches the invoker anyway) is never called at all. This is enough for
`toolcall.Planner`: a tool-calling model literally cannot call a function it
was never sent a definition for, so nothing else needed to change there
beyond threading `PlanContext` through to `ToolsFor`'s call site.
`jsonmode.Planner` is the exception the spec's own M5 cross-reference
(`docs/specs/context.md`, "both planners get the same list") anticipates:
its catalogue is rendered as free text once in `New`, and the previous code
hard-coded the `propose_panel` JSON shape into that text unconditionally,
which meant the actual behaviour this subproject exists to change - the
model reaching for a panel it was never supposed to know about - would not
have moved for that adapter at all, only the after-the-fact refusal would
have fired. Fixed by precomputing two full variants (system prompt and
`ResponseFormat`'s `"kind"` enum, with and without `propose_panel`) in `New`
and selecting between them per `Plan` call from the same `tools` argument
`toolcall.Planner` already reads - plus a local `parse` check
(`ErrUnknownKind`) for a model that answers `"propose_panel"` anyway, since
this transport has no structural "undeclared function" backstop the way
tool calling does.

**What was not touched.** No `web/` component beyond the three files O4
names (`ConversationPanel.tsx`, `Conversation.tsx`,
`conversationStore.tsx`/`conversationContext.ts`) needed a change - the
save-control and proposal-form wiring already switched on
`defaultWorkspaceId`/`workspaceId` being defined, and now also causes the
same value to reach `postPlan`'s own request body. `pages/chat` and
`pages/workspace` themselves are unchanged: they already called
`ConversationPanel` with no id and always an id, respectively.

## 2026-09-14 `docs/specs/narrowing.md` Task 4: the lexical baseline's recall@K, per axis - higher is better

**Context.** `docs/plans/narrowing.md` Task 4 is the whole point of the
subproject: measure the lexical baseline (`e2e/narrowing/lexical.ts`, Task 3) against the 100-question corpus (`e2e/narrowing/corpus/`, Tasks 1-3) at
every catalogue size the fixture supports, and record the numbers rather
than a claim about them. `e2e/narrowing/measure.ts` and `report.ts` do the
measuring; `make narrowing` runs it. No LLM is called -
`docker logs llama-swap`'s `POST /v1/chat/completions` count did not move
across this work (414 before, 414 after) - and `make narrowing` is not part
of `make check`, `test`, `acceptance` or `lint`, exactly as the plan says: a
measurement that has to pass is not a measurement.

**Read the table as recall@K, higher is better.** Every cell is
`worst%/best%` out of the questions eligible at that catalogue size (spec
section 6): `worst` counts an answer recalled only if it is still inside K
even when every operation tied with it outranks it - a tie is not a hit -
and `best` counts it if the most favourable tie order would put it inside
K. A cell marked `*` differs by 15 points or more between the two, which is
the report saying that much of the number is a coin toss rather than a
property of the mechanism.

```
size  ops    K    A         B         C         D         E         overall   ms/query
  size 1 (200 ops): 82 of 100 questions excluded — by axis: A 19, B 25, C 20, D 11, E 7
1     200    10   100%/100% n/a/n/a   60%/80%*  0%/0%     67%/100%* 61%/72%   0.080
1     200    20   100%/100% n/a/n/a   60%/80%*  0%/0%     100%/100% 67%/72%   0.080
1     200    50   100%/100% n/a/n/a   60%/80%*  0%/0%     100%/100% 67%/72%   0.080
  size 2 (400 ops): 53 of 100 questions excluded — by axis: A 13, B 10, C 15, D 10, E 5
2     400    10   100%/100% 60%/93%*  40%/60%*  0%/0%     80%/100%* 62%/79%*  0.092
2     400    20   100%/100% 87%/93%   50%/60%   0%/0%     100%/100% 74%/79%   0.092
2     400    50   100%/100% 100%/100% 60%/70%   0%/0%     100%/100% 81%/83%   0.092
  size 3 (600 ops): 34 of 100 questions excluded — by axis: A 8, B 5, C 10, D 7, E 4
3     600    10   100%/100% 45%/95%*  33%/67%*  0%/0%     67%/100%* 53%/79%*  0.146
3     600    20   100%/100% 85%/95%   60%/67%   0%/0%     83%/100%* 73%/79%   0.146
3     600    50   100%/100% 95%/95%   60%/73%   0%/0%     83%/100%* 76%/80%   0.146
  size 5 (1000 ops): 0 of 100 questions excluded — by axis: A 0, B 0, C 0, D 0, E 0
5     1000   10   100%/100% 32%/96%*  36%/72%*  0%/0%     50%/90%*  47%/76%*  0.232
5     1000   20   100%/100% 72%/96%*  60%/72%   0%/0%     80%/90%   66%/76%   0.232
5     1000   50   100%/100% 88%/96%   68%/80%   0%/0%     80%/100%* 72%/79%   0.232
```

**Excluded questions, at the sizes below five services.** The corpus was
written against the full five-service fixture, so at sizes 1-3 a question
whose only answers live in a service not yet in the catalogue cannot be
asked of it at all - that is not the mechanism failing, it is the question
not applying. Such a question is dropped from every recall figure at that
size (never scored zero) and the count is printed instead: 82/100 at size 1,
53/100 at size 2, 34/100 at size 3, 0/100 at size 5. Axis B is the extreme
case - all 25 excluded at size 1, because every axis-B question's answers
span two services and the size-1 catalogue is one service (inventory), so
none of the pairs a cross-service question needs exist yet.

**Axis D scores 0/0/0 at every size and every K. This is the measurement
working, not a defect.** `docs/specs/narrowing.md` section 7 says the
lexical baseline cannot answer a vocabulary-gap question by construction,
and the pre-measured number quoted in the spec (0/15 at K=20 against the
full catalogue) is reproduced here exactly, at every size and K, because
none of axis D's fifteen questions - 休みたい, PO を出したい, 立て替えた分を
出したい and the rest - share a single character bigram with the operation
that answers them. `narrow`'s own definition (`lexical.ts`) excludes a
zero-scored operation from the shortlist entirely, so there is no K large
enough to recover these: the baseline does not rank them low, it does not
rank them at all. That gap is exactly what the subproject exists to
measure, per spec section 1 and section 7 - the size of the hole is the
argument for whatever mechanism comes next. Nothing in `lexical.ts` was
touched to move this number.

**Axis A is 100% everywhere, at every K down to 10, with no gap between
worst and best.** Spec section 4 built axis A to guarantee that the verb
alone (一覧, matching a fifth of the whole catalogue) carries no
selectivity - but every axis-A question in the corpus names a noun the
answer's summary also carries, so the noun alone separates the answer from
the other operations sharing the verb, cleanly enough that no other
operation ties it even at the thousand-operation size. This is not axis A
"failing to be hard" - the axis is doing its job (proving the verb adds
nothing) and the corpus's nouns are doing theirs (still findable by exact
overlap); what would fail here is a mechanism that used the verb as its
main signal, which bigram overlap over the whole summary does not.

**Axis B and axis E are where the worst/best gap is largest, and both
widen it as the catalogue grows.** At 1000 operations, K=10, axis B reads
32%/96% and axis E reads 50%/90% - the pessimistic and optimistic figures
disagree by 40-64 points, the largest gaps in the table. Both axes are
built around collisions by design: axis B's shared nouns (注文, 明細,
承認, 社員, 取引先) put two or more genuinely tied answers at the same
score, and axis E's decoys are picked because they out-score the true
answer, and often other unrelated settings operations tie near the same
score band too, on the same vocabulary. A bigger catalogue means more
things sharing that vocabulary, hence a wider tie, hence a wider best/worst
gap - the "186 operations tie at 「注文を一覧」's tenth place" example in
spec section 6 is this same effect. Both axes recover almost to 100%/100%
by K=50, which is the honest way to say what K a caller would need to pick
if it wanted axis B and E answers found reliably rather than merely
findable in principle.

**Axis C sits in between, and stays incomplete even at K=50.** At size 5 it
tops out at 68%/80% (K=50) - the near-neighbour groups (在庫品目 / 在庫ロット
/ 在庫引当 / 棚卸 / 在庫調整ほか) are close enough in vocabulary that a
"見たい"-style question keeps several equally-plausible candidates
competing, and unlike B/E the true answer does not always win that
competition outright - some of these questions never surface their answer
inside the top 50 regardless of tie handling. This is the axis the plan
predicted would be the hardest to separate cleanly with bigram overlap
alone (spec section 4, "roughly equal candidates").

**Overall reads 47%/76% at size 5, K=10, and climbs to 72%/79% by K=50 -
still short of what a narrowing mechanism would need to ship, which is the
point of measuring it now.** Per-query wall-clock stays well under a
millisecond even at the full 1000-operation catalogue (0.232ms average),
so cost is not what limits K here; recall is. That reproduces spec section
7's own framing: if the baseline reached usable recall at a usable K, a
vector store would be unnecessary; it does not, axis D and (at low K) axis
B/C/E are the shortfall, and that shortfall is the input the next
subproject (choosing a narrowing mechanism) needs.

**How to repeat it.** `make narrowing` (`cd e2e && node
narrowing/measure.ts`), from a checkout with no build step needed - the
fixture and corpus are read directly, nothing is compiled or served.

## 2026-09-14 `docs/plans/retrieving.md` closes: six embedding configurations, the rerank stage, and what keeping a model loaded costs

**Context.** `docs/plans/retrieving.md` Task 4 is the last task of the
subproject: measure the alternation cost llama-swap's default one-
model-resident mode imposes (`e2e/narrowing/loading.ts`, new), print it once
in `make narrowing`'s report rather than per row (`report.ts`), and record
every number spec section 8 (AC-V-107) asks for - recall per axis per
configuration, the contract check, the rerank cost, and the alternation
cost - in one place. Tasks 1-3 (embedding client and cache, the vector
narrower and its report rows, the retrieve-then-rerank configuration) were
already committed; this task adds nothing to what is measured except the
alternation, and records the rest for the first time.

**a. Recall per axis per configuration, at 1000 operations, K=10 (`make
narrowing`, this run).** lexical (worst%/best%): A 100%, B 32%/96%, C
36%/72%, D 0%, E 50%/90%, overall 47%/76%. `bge-m3-q8`: A 100%, B 88%, C 84%, D 27%, E
100%, overall 82%. `e5-large-q8`: A 100%, B 100%, C 72%, D 20%, E 100%,
overall 81%. `ruri-v3-310m-q8-mean`: A 100%, B 88%, C 68%, D 27%, E 100%,
overall 78%. `ruri-v3-310m-q8`: A 72%, B 24%, C 44%, D 13%, E 50%, overall
42%. `qwen3-embedding-0.6b-q8`: A 88%, B 68%, C 60%, D 13%, E 30%, overall
59%. `qwen3-embedding-0.6b-q8-plain`: A 100%, B 76%, C 72%, D 20%, E 70%,
overall 72%. `e5-large-q8+reranker`: A 100%, B 100%, C 80%, D 47%, E 90%,
overall 86%. The headline comparison the probe predicted holds: lexical 47%
overall at K=10, `e5-large-q8+reranker` 86%.

**b. The contract check predicts recall, and it earned its place as a cheap
screen.** Measured by hand at 1000 operations, K=10, and cross-checked
against this run's own contract-check lines and overall recall (both match
exactly):

| configuration                   | contract fails | overall@10 |
| ------------------------------- | -------------- | ---------- |
| `bge-m3-q8`                     | 0/20           | 82%        |
| `e5-large-q8`                   | 0/20           | 81%        |
| `ruri-v3-310m-q8-mean`          | 0/20           | 78%        |
| `qwen3-embedding-0.6b-q8-plain` | 0/20           | 72%        |
| `qwen3-embedding-0.6b-q8`       | 11/20          | 59%        |
| `ruri-v3-310m-q8`               | 3/20           | 42%        |

Every configuration that passes the check clusters at 72-82%; both that
fail sit below it. The check runs before any ranking is measured at all
(`checkContract`, `e2e/narrowing/embedding/contract.ts`) - a configuration
that cannot retrieve an operation by its own text is not going to retrieve
it by a question about it either, and this table is the evidence that the
cheap check catches that before the expensive one has to.

**c. Two configuration findings that contradict the model cards, both
already known and confirmed again by this run.** `ruri-v3-310m` on the same
corpus: CLS pooling (`ruri-v3-310m-q8`) 42% overall at K=10, mean pooling
(`ruri-v3-310m-q8-mean`) 78% - the model card's pooling choice measured
worse than the alternative. `qwen3-embedding-0.6b-q8` with the documented
instruction prefix on the query side: 59% overall; the same model with no
prefix at all (`qwen3-embedding-0.6b-q8-plain`): 72%. In both cases,
following the convention was worse than measuring.

**d. Embeddings do not tie - `docs/specs/narrowing.md` section 10's open
question is closed for these mechanisms.** Every embedding-based
configuration in this run's output - `bge-m3-q8` through
`e5-large-q8+reranker`, all six embedding configurations and the two-stage
one, at every catalogue size and every K - reports worst% equal to best%,
checked by scanning the whole report for any `X%/Y%` pair with X != Y
outside the lexical block: none exists. "186 operations sharing a score"
(spec section 6, the lexical baseline's example) was an artefact of bigram
counts being coarse enough to produce exact ties, not of the underlying
questions being ambiguous - a float-valued cosine similarity essentially
never repeats exactly, so a meaning-based mechanism simply does not have
this problem.

**e. Retrieval width and the vocabulary gap - hand-measured with
`e5-large-q8` at 1000 operations, not reproducible by `make narrowing`
(which only ever retrieves 50 for the two-stage configuration).** How many
of axis D's 15 answers are inside a retrieval of width W: W=50 -> 11,
W=100 -> 11, W=200 -> 12, W=300 -> 13, W=500 -> 15. Every other axis is
complete by W=200. Widening the shortlist buys one question at W=100->200
and costs the reranker proportionally (it reranks the whole shortlist) - it
is not the answer to axis D's gap.

**f. What the residual gap actually is - the four axis-D questions
`e5-large-q8` cannot reach inside 50, with rank (hand-measured, not
reproducible by `make narrowing`).**

- 「立て替えた分を出したい」 -> 経費申請の作成 - rank 385
- 「お金を返してもらいたい」 -> 精算の作成 - rank 367
- 「商品が届いたので受け取り処理をしたい」 -> 検収の作成 - rank 257
- 「値段を安くしてほしいと頼みたい」 -> 値引の作成 - rank 111

All four are the same shape: an everyday description of an action, against
an API named with a single business noun. Closing this needs the company's
vocabulary, not general semantic similarity - it is in what the catalogue
says about itself, rather than in how it is searched (spec section 9).
What to do about that is a future decision, not this entry's.

**g. The alternation cost, and the per-question costs (this run).**
Measured with `e2e/narrowing/loading.ts`'s `measureAlternation` (`e5-large-
q8` for embeddings, `qwen3.5-9b-q8` for chat, `max_tokens: 1`, response
content never read): embedding call with the embedder already resident,
0.009s; chat call that first unloads the embedder, 6.605s; chat call with
the chat model now resident, 0.063s; embedding call that first unloads the
chat model, 2.637s; embedding call with the embedder resident again,
0.009s. The shape matches spec section 5's own table (resident near zero,
an unload orders of magnitude larger, unloading the chat model costs less
than unloading the embedder) even though the absolute seconds are smaller
here - a warmer page cache for the model weights on this run, not a
different mechanism. Per-question costs from this run's ms/query columns:
lexical 0.246ms, `e5-large-q8` 6.441ms, `e5-large-q8+reranker` 78.953ms -
all within the ranges the probe predicted (under 0.3ms, 5-7ms, about
80ms). The loading cost stays five orders of magnitude above the per-
question cost either way; it does not vary with K or catalogue size, which
is why it is printed once in the report rather than folded into any
per-row average.

**Verification.** `docker logs llama-swap`'s `POST /v1/` count was
unchanged across `make check` (27632 before, 27632 after) - `make check`
calls no model of any kind. `make narrowing`'s lexical and embedding-
configuration rows are byte-identical before and after this task's changes
except `ms/query`, which naturally varies run to run.

## 2026-09-14 A frontier model reads the whole catalogue: the ceiling, and a defect in the corpus it exposed

**Context.** Every mechanism measured so far is mechanical - bigram counts,
embeddings, a reranker - and all of them leave the vocabulary-gap axis (D)
under half at K=10. Before deciding what to build next, the question worth
answering is whether the mapping those questions ask for is reachable **at
all**. If a model that understands the domain and reads all 1000 operations
cannot do it either, the answer is not a better retriever; it is asking the
person.

**Method, and what it is not.** A subagent with no access to the corpus's
answer key was given two files - the 1000 operations as
`operationId / service / summary`, and the 100 questions - and asked to name
one operation per question, plus a flag saying whether the question is
`ambiguous`: underdetermined, so a person would have to be asked back. It was
told not to write code. The agent is Claude, so this measures what Claude can
do, not what every frontier model can do; it is a go/no-go, not a survey. It
is a hand-run probe and is **not** reproducible by `make narrowing`.

**Result, scored against the corpus's answer key.**

| axis | primary answer in key | any named answer in key | flagged ambiguous | best mechanical, K=10 |
| ---- | --------------------- | ----------------------- | ----------------- | --------------------- |
| A    | 25/25                 | 25/25                   | 2                 | 25/25                 |
| B    | 25/25                 | 25/25                   | **25**            | 25/25                 |
| C    | 21/25                 | 23/25                   | 19                | 20/25                 |
| D    | 10/15                 | 11/15                   | 3                 | 7/15                  |
| E    | 3/10                  | 3/10                    | 0                 | 9/10                  |

**The vocabulary gap is reachable.** Axis D is 10/15 against the mechanical
7/15, and four of the five misses are answers better than the key's:
「休みたい」 answered `createAttendanceLeaveRequest` where the key says
`listAttendancePaidLeaves`; 「そろそろダメになりそうな商品を確認したい」
answered `searchInventoryExpiringItems` where the key says
`listInventoryExpiryDates`. The fifth, 「値段を安くしてほしいと頼みたい」, is
genuinely undecidable - asking for a discount as a buyer and granting one as a
seller are different operations and the sentence picks neither.

**Axis E's 3/10 is the key, not the model.** Checked separately: the model
chose the decoy in **0 of 10**. Every answer was a record operation; what cost
it the score was choosing `残業を集計する` where the key lists `残業の一覧`, and
similar. For what axis E exists to measure - is the settings operation
mistaken for the transaction - it scored 10/10.

**Which is a defect in the corpus, and it is ours.** The answer key names
`list*` operations and rejects the `search*`, `summarize*` and `aggregate*`
operations over the same business object, which are often the better answer to
the question as asked. Axis B and C were written with several defensible
answers each; A, D and E were not, and should have been. Every mechanism's
axis-D and axis-E numbers therefore carry some measurement of "does this
mechanism prefer `list*`" mixed into them. The axis-E claim survives (avoiding
the decoy is what it tests, and the key's answers and the better ones are all
record operations); the axis-D numbers are the ones to distrust.

**The model knows when to ask.** All 25 structurally ambiguous questions - the
ones whose noun exists in two or three services - were flagged `ambiguous`,
against 2 false positives in 25 on the axis where questions are not ambiguous
at all. Axis B is the axis no retriever can resolve, because the information is
not in the question; `ask_user` is a correct answer to it, and the model can
tell when to give it.

**What this changes.** Narrowing to ten was the wrong target. The mechanical
methods put 11 of 15 axis-D answers inside 50 and 7 inside 10; a model reading
the shortlist gets 10 or more. So a narrowing's job is not to rank the answer
first, it is to not lose it - and what happens after the shortlist is the
model's, including deciding that the question cannot be answered without
asking.

**Correction to `docs/specs/retrieving.md` section 5.** The alternation cost is
recorded there as 34.8s and 4.7s, which was one measurement of a model being
read from disk for the first time. Measured twice more since: 6.6s/2.6s and
4.8s/2.7s. The honest figure is about 7-8 seconds warm and up to 40 cold. The
conclusion is unchanged - the narrowing it serves costs 0.3 ms to 80 ms, four
orders of magnitude less either way - but a single number was misleading and
the spec now carries the range.

## 2026-09-15 Frontier models on the same shortlist, single call, $3.80 of a $20 budget

**Context.** The 2026-09-14 ceiling probe gave a Claude subagent the whole
catalogue and minutes of tool use, and reported 10/15 on the vocabulary-gap
axis. That number had three things mixed into it - the model, an agent loop,
and reading all 1000 operations - and no latency, which is the number that
decides whether any of it is usable. With an API key and a $20 limit, this run
separates them: same 100 questions, same 50 candidates from `e5-large-q8`,
one `POST /v1/messages` per question, no tools.

**Discipline, and where it failed.** Everything that could be learned for free
was learned first: the token counting endpoint is free, so every prompt was
counted exactly before anything was billed (Sonnet 5 / Opus 5: 1,526 tokens a
question on average, 27,489 for the whole catalogue; Haiku 4.5: 1,313 and
23,187). Each phase then ran two questions and checked `usage` before running
a hundred. The first Phase 2 run still wasted **$1.37**: Claude Sonnet 5 and
Opus 5 think by default, thinking tokens count toward `max_tokens`, and
`max_tokens: 64` left no room for an answer - 65 of Opus's 100 answers were
empty. The three-question smoke test had not caught it because all three were
unambiguous and needed almost no thinking. This was the same failure the local
run had hit hours earlier. Re-run with `thinking: {type: "disabled"}` and a
smoke test that asserts `output_tokens_details.thinking_tokens` and
`stop_reason`.

**Phase 2 - the comparison the ceiling probe could not make.** Fifty
candidates, thinking off, one call each, 100 questions.

| model                                                      | correct | A   | B   | C   | D   | E   | flagged ambiguous | s/question | $/100 |
| ---------------------------------------------------------- | ------- | --- | --- | --- | --- | --- | ----------------- | ---------- | ----- |
| local qwen3.5-9b-q8, no thinking                           | 69      | 22  | 20  | 17  | 6   | 4   | 57                | 0.4        | 0     |
| Claude Haiku 4.5                                           | 67      | 20  | 24  | 15  | 3   | 5   | 66                | 0.79       | 0.14  |
| Claude Sonnet 5                                            | 73      | 20  | 24  | 17  | 7   | 5   | 73                | 1.78       | 0.35  |
| Claude Opus 5                                              | 73      | 18  | 24  | 18  | 7   | 6   | 89                | 2.30       | 0.83  |
| (2026-09-14) Claude subagent, same shortlist, tools + loop | 82      | 23  | 25  | 20  | 7   | 7   | 47                | minutes    | -     |

Under identical conditions the local 9B and the frontier models are four
points apart, and Haiku is behind the local model. Axis D - the vocabulary
gap - is 6, 3, 7, 7: **no model closes it from a shortlist**, which settles
that the gap is in what the catalogue says about itself, not in who reads it.
The subagent's 82 is nine points above the same model's single call; that is
the loop, not the model. The local model is also the fastest: the API adds
network and a larger model on top.

Calibration differs. Every model flags the structurally ambiguous axis-B
questions (24-25 of 25). On axis A, whose questions are not ambiguous, false
flags are local 2, Haiku 10, Sonnet 14, Opus 19 of 25. Larger models err
toward asking.

**Phase 3 - thinking on the frontier.** Sonnet 5 with its default adaptive
thinking, `max_tokens` 4000, 257 thinking tokens a question on average:
**73/100, axis D 7/15 - identical to thinking off**, at 2.46 s instead of
1.78 and $0.41 instead of $0.35. The local model had shown the same on axis D
(6/15 either way, at 85 s a question with thinking). Thinking buys nothing
here on any model.

**Phase 4 - the whole catalogue, cached (the D2 question).** All 1000
operations in the system prompt behind a 1-hour cache breakpoint, Sonnet 5,
thinking off. The cache did what the docs say: one write of 27,515 tokens
($0.11), then 99 reads at $0.0057 a question - $0.70 for the run, against a
$0.69 estimate. So D2's cost argument holds when caching is available: a
question against the whole catalogue costs roughly what a question against 50
candidates costs.

The precision argument does not. **64/100, against 73 from the shortlist.**
Axis B fell from 24 to 13 and the misses are not ambiguity: 「明細を追加したい」
answered `listInventoryItems`, 「明細を修正したい」 answered
`updateInventoryItem`. The model kept the verb and lost the noun, and what it
grabbed is from the first service in the list. Given a thousand lines a
frontier model grabs from the top. This is the precision collapse the seventh
tool showed `qwen38-27b` on 2026-09-14, now measured on a frontier model with
a thousand.

**What this settles.**

- Narrowing is not optional and not a cost measure. Sending everything is
  cheap with a cache and worse for precision even on Opus-class models.
- The shortlist is the ceiling. Fifty candidates from `e5-large-q8` hold
  11 of the 15 axis-D answers, and no model - local, Haiku, Sonnet, Opus, with
  or without thinking - gets more than 7 of them into first place. The
  remaining gap is the catalogue's vocabulary (2026-09-14 entry, residual
  gap), and the next work is there.
- Model choice is not where the accuracy is. Four points separate a 0.4-second
  local model from a 2.3-second frontier model on the same input.
- Asking back is something every model can do; how often it does it
  unnecessarily is the parameter to tune, and the local model is best
  calibrated on it here.

**Caveats that still apply.** The answer key prefers `list*` and rejects
`search*`/`summarize*`/`aggregate*` over the same object (2026-09-14, corpus
defect), which depresses every row by some amount and axes D and E most. The
frontier figures are Claude only. All figures are one run at temperature 0.

**Spend.** $3.80 of $20: Phase 2 wasted run $1.37, Phase 2 $1.32, Phase 3
$0.41, Phase 4 $0.70.

**Re-scored against the corrected answer key (later the same day).** The
key fix (`75e221d`) widened axes A, D and E; every saved run above was
re-scored against it without re-running anything. Overall, old → new:
local qwen3.5-9b 69 → 78; Haiku 4.5 67 → 79; Sonnet 5 73 → 83; Opus 5 73 →
84; Sonnet 5 with thinking 73 → 82; Sonnet 5 with the whole catalogue 64 →
70; the subagent on the shortlist 82 → 90 and on the whole catalogue 84 → 93. Axis D, new key: local 9, Haiku 7, Sonnet 9, Opus 11, subagent-on-1000
14 of 15. The local-to-Opus gap is six points, not four, and D17 says six.
Every other conclusion holds: thinking is flat (83 vs 82), the whole
catalogue is worst (70), and the loop is worth seven to nine points. Ledger and per-run outputs are in the session
scratchpad, not the repository; the key is read from a file outside the
repository and never appears in it.

## 2026-09-15 The corpus answer key is fixed, and every recall figure moves

**Context.** TODO.md item 2. The 2026-09-14 entry "A frontier model reads
the whole catalogue" found the defect: the answer key named `list*` and
rejected `search*`, `summarize*` and `aggregate*` operations over the same
business object, and rejected verb variants (`create` vs `submit`) where the
question admits both. Axes B and C were written with every defensible
answer; A, D and E were not. Every recall figure recorded 2026-09-14 was
scored against the old key and is superseded by this entry.

**What changed, per axis.** One standard applied to all 100 questions: an
answer is every operation a reasonable person in that company could mean by
the question, over the same business object — not every operation that
merely mentions the noun. Non-obvious inclusions carry a one-line comment on
the question in `e2e/narrowing/corpus/axis-{a,d,e}.ts`.

| axis      | questions changed         | answers before | answers after |
| --------- | ------------------------- | -------------- | ------------- |
| A         | 14 / 25                   | 25             | 39            |
| B         | 0 / 25 (already complete) | 60             | 60            |
| C         | 0 / 25 (already complete) | 53             | 53            |
| D         | 11 / 15                   | 15             | 28            |
| E         | 5 / 10                    | 11             | 16            |
| **total** | **30 / 100**              | **164**        | **196**       |

Axis A gained a `search*`/`summarize*`/`aggregate*` sibling wherever the
question's wording does not commit to "every record" over "the matching
ones" or "a computed figure" (e.g. a16 「関税ってどれくらいかかってる？」 now
also accepts `aggregatePurchasingCustomsDuties` — 「どれくらい」asks for a
figure), and one `create` sibling where a verb reasonably reads as "release"
rather than "show" (a12 セット商品を出したい). Axis D gained `create`/`submit`
pairs where "出したい"/"直したい" admits either drafting a record or filing
it into a workflow (d01-d04, d07), 出荷準備's three stages as three answers
(d06), the three genuine readings of "money coming back" (d09: 精算, 経費申
請, 仮払), the buyer/seller reading DECISIONS.md 2026-09-14 already called
"genuinely undecidable" (d12), and the model's better answers on d13-d15.
Axis E gained a same-object `search*`/`summarize*`/`aggregate*` sibling on
five of its ten questions (e01, e03, e04, e05, e09) — every one checked to
still score below its decoy, so the axis's own claim (the decoy is what
costs a mechanism the point, not the answer set) is unweakened.

**Answers considered and left out.** None failed the axis-D bigram check or
the axis-E outscore check — every candidate was checked with `lexical.ts`'s
own `bigramsOf`/`scoreOperation` before being added, so nothing had to be
discarded for breaking those. Two axis-E candidates were checked and pass
both structural checks (`aggregateSalesCreditLimits` for e06, still below
its decoy at 0.167 vs 0.375; `summarizeAttendanceBusinessTrips` for e08,
0.125 vs 0.3125) but were left out on judgement: neither operation expresses
"nearing a threshold" the way `aggregateInventorySafetyStockGap` (e09, kept)
does — a generic aggregate is not the same claim as a gap computed against
the limit itself, and adding it would have been widening for its own sake
rather than because a reasonable person would mean it.

**Verification the fix touches no harness.** `make check` — green (one
`acceptance-e2e` failure on the first run, `src/auth.test.ts`'s `afterAll`
hook timing out at 30s; reproduced as passing in isolation in under 500ms
and matches the flake DECISIONS.md already recorded 2026-09-13, "the browser
suite's flake is load, measured"; the retry was green start to finish).
`docker logs llama-swap 2>&1 | grep -c 'POST /v1/'` read 30879 before
`make check` and 30879 after, across both runs — `make check` calls no
model.

**The new table, all eight configurations, 1000 operations, K = 10/20/50
(`make narrowing`, this run).** Cells are `worst%/best%`; embedding
configurations never tie (2026-09-14 finding, reconfirmed), so their two
figures are always equal.

```
                                    K   A         B         C         D         E         overall
lexical                            10  100%/100% 32%/96%   36%/72%   0%/0%     60%/90%   48%/76%
                                    20  100%/100% 72%/96%   60%/72%   0%/0%     80%/90%   66%/76%
                                    50  100%/100% 88%/96%   68%/80%   0%/0%     80%/100%  72%/79%
bge-m3-q8                          10  100%      88%       84%       47%       100%      85%
                                    20  100%      96%       88%       53%       100%      89%
                                    50  100%      100%      92%       60%       100%      92%
e5-large-q8                        10  100%      100%      72%       33%       100%      83%
                                    20  100%      100%      80%       60%       100%      89%
                                    50  100%      100%      84%       80%       100%      93%
ruri-v3-310m-q8-mean                10  100%      88%       68%       40%       100%      80%
                                    20  100%      100%      76%       53%       100%      87%
                                    50  100%      100%      92%       60%       100%      92%
ruri-v3-310m-q8                    10  76%       24%       44%       13%       60%       44%
                                    20  88%       32%       64%       13%       60%       54%
                                    50  92%       56%       80%       20%       80%       68%
qwen3-embedding-0.6b-q8             10  88%       68%       60%       13%       30%       59%
                                    20  96%       76%       72%       33%       60%       72%
                                    50  96%       92%       76%       40%       90%       81%
qwen3-embedding-0.6b-q8-plain       10  100%      76%       72%       33%       80%       75%
                                    20  100%      84%       76%       40%       80%       79%
                                    50  100%      96%       80%       53%       100%      87%
e5-large-q8+reranker                10  100%      100%      80%       60%       90%       88%
                                    20  100%      100%      84%       73%       100%      92%
                                    50  100%      100%      84%       80%       100%      93%
```

**These replace the 2026-09-14 figures as the citable ones. The delta, K=10
overall (old -> new):** lexical 47%/76% -> 48%/76% (worst +1, best +0);
`bge-m3-q8` 82% -> 85% (+3); `e5-large-q8` 81% -> 83% (+2);
`ruri-v3-310m-q8-mean` 78% -> 80% (+2); `ruri-v3-310m-q8` 42% -> 44% (+2);
`qwen3-embedding-0.6b-q8` 59% -> 59% (+0); `qwen3-embedding-0.6b-q8-plain`
72% -> 75% (+3); `e5-large-q8+reranker` 86% -> 88% (+2).

**Stated plainly, not editorialised: every embedding configuration except
`qwen3-embedding-0.6b-q8` moved more than the lexical baseline did.** The
two-stage configuration (`e5-large-q8+reranker`) moved +2 points at K=10
overall against the lexical baseline's +1/+0. The lexical baseline's axis D
is unchanged at exactly 0% at every size and K — every new axis-D answer is,
by the same bigram-disjoint construction as the old ones, unreachable by a
mechanism that only counts character overlap, so widening the key could not
move it. What did move on axis D is every embedding configuration:
`bge-m3-q8` 27% -> 47% (+20), `e5-large-q8` 20% -> 33% (+13),
`ruri-v3-310m-q8-mean` 27% -> 40% (+13), `qwen3-embedding-0.6b-q8-plain`
20% -> 33% (+13), `e5-large-q8+reranker` 47% -> 60% (+13) — axis D is where
the old key's defect concentrated, and it is where fixing it moved the most.
`ruri-v3-310m-q8` and `qwen3-embedding-0.6b-q8` (the two configurations that
already fail the contract check) show no axis-D movement at all: 13% both
times, on both rows — a mechanism that cannot retrieve an operation by its
own text is not helped by a wider answer key either.

**What the spec and the checks got right.** `corpus.test.ts`'s axis-D and
axis-E checks did their job exactly as designed: several candidate answers
were tried against `bigramsOf`/`scoreOperation` before being written down,
and the tooling used to check them (a throwaway script over
`fixture/index.ts` and `lexical.ts`, not committed) is the same two
functions the test file itself calls — there was no case where a defensible
answer had to be discarded because it broke either check, which is the
outcome the checks were designed to force. Nothing in `docs/specs/
narrowing.md` needed correcting.

**How to repeat it.** `make narrowing` (`cd e2e && node narrowing/measure.ts`),
same as 2026-09-14 - the fixture and corpus are read directly, no build
step. `e2e/narrowing/corpus/corpus.test.ts` (`cd e2e && pnpm exec vp test
run narrowing/corpus/corpus.test.ts`) checks the key's own structure in
under 300ms.

## 2026-09-15 The local picker reads the shortlist in reranker order: 78 → 83, for nothing

**Context.** Under the corrected key the local `qwen3.5-9b-q8`, thinking
off, picking from the 50 `e5-large-q8` candidates in similarity order, scores
78; Sonnet 5 scores 83 and Opus 5 84 on the same input (D17). The
whole-catalogue run had shown what a model does with a long list: it keeps
the verb, loses the noun, and grabs from the top. If a model reads from the
top, the order of the shortlist is not neutral.

**Measured.** Same 100 questions, same 50 candidates, now sorted by
`bge-reranker-v2-m3-q8` before being shown to the picker, and cut to K.
Hand-run in the session scratchpad, one run at temperature 0, thinking off.

| shortlist order | K   | correct | A   | B   | C   | D   | E   | flagged ambiguous | s/question |
| --------------- | --- | ------- | --- | --- | --- | --- | --- | ----------------- | ---------- |
| e5 similarity   | 50  | 78      | 25  | 20  | 17  | 9   | 7   | 57                | 0.43       |
| reranker        | 10  | 81      | 24  | 25  | 17  | 8   | 7   | 37                | 0.28       |
| reranker        | 20  | **83**  | 25  | 25  | 18  | 8   | 7   | 50                | 0.31       |
| reranker        | 50  | 82      | 25  | 23  | 17  | 9   | 8   | 63                | 0.43       |

Ordering alone is worth four to five points; K barely matters between 10 and
50 (one run, so ±2 is noise). At K=20 the local model scores what Sonnet 5
scored and one below Opus 5, at 0.31 seconds a question and no cost. The
D17 gap is closed by presentation, not by the model. Axis B goes to 25/25 -
the ambiguous questions' defensible answers are now at the top of the list
where the picker sees them. Axis D does not move (8-9 of 15): it was never a
selection problem.

**What it implies.** The reranker is already computed for narrowing; the
picker should be handed its order, and about 20 candidates. This is the
cheapest change in the whole subproject and is not yet wired into anything -
`make narrowing` measures recall, not the pick. Wiring the pick into the
measurement is now worth doing, since the headline the product cares about
is "right operation chosen, or asked back", not recall@K.

## 2026-09-15 The catalogue says it: the written layer closes axis D, the generated layer is a negative result

**Context.** `docs/plans/describing.md` Task 5, the record. Tasks 1-4
(`de4dc96`, `28fb615`, `1b77389`, `4978652`, `e7a71d3`) built the two
utterance layers, the vendor field, and the report rows. This entry is the
one `make narrowing` run (1000 operations, K=10/20/50, all four catalogue
sizes) that closes the subproject, per `docs/specs/describing.md` AC-G-107.

**The table, K=10, 1000 operations.**

| configuration                  | A   | B   | C   | D   | E   | overall |
| ------------------------------ | --- | --- | --- | --- | --- | ------- |
| `e5-large-q8`                  | 100 | 100 | 72  | 33  | 100 | 83      |
| `e5-large-q8+generated`        | 80  | 64  | 80  | 33  | 80  | 69      |
| `e5-large-q8+written`          | 92  | 96  | 68  | 80  | 80  | 84      |
| `e5-large-q8+both`             | 88  | 92  | 76  | 67  | 90  | 83      |
| `e5-large-q8+reranker`         | 100 | 100 | 80  | 60  | 90  | 88      |
| `e5-large-q8+reranker+written` | 96  | 96  | 80  | 73  | 100 | 89      |
| `e5-large-q8+reranker+both`    | 96  | 100 | 96  | 67  | 100 | 93      |

**The same table, K=50, 1000 operations.**

| configuration                  | A   | B   | C   | D   | E   | overall |
| ------------------------------ | --- | --- | --- | --- | --- | ------- |
| `e5-large-q8`                  | 100 | 100 | 84  | 80  | 100 | 93      |
| `e5-large-q8+generated`        | 100 | 96  | 96  | 73  | 90  | 93      |
| `e5-large-q8+written`          | 96  | 96  | 88  | 93  | 100 | 94      |
| `e5-large-q8+both`             | 96  | 100 | 100 | 87  | 100 | 97      |
| `e5-large-q8+reranker`         | 100 | 100 | 84  | 80  | 100 | 93      |
| `e5-large-q8+reranker+written` | 96  | 96  | 88  | 93  | 100 | 94      |
| `e5-large-q8+reranker+both`    | 96  | 100 | 100 | 87  | 100 | 97      |

**The two headline rows across all four catalogue sizes, K=10** (`worst%/best%` collapse to one figure for every embedding-only cell here; ties only appear where B is `n/a` at size 1, which excludes every axis-B question - no operation pair from the collision set survives at 200 ops):

| row                 | size (ops) | A   | B   | C   | D   | E   | overall |
| ------------------- | ---------- | --- | --- | --- | --- | --- | ------- |
| `+reranker+written` | 1 (200)    | 100 | n/a | 100 | 100 | 100 | 100     |
|                     | 2 (400)    | 100 | 93  | 80  | 100 | 100 | 94      |
|                     | 3 (600)    | 94  | 95  | 73  | 88  | 100 | 89      |
|                     | 5 (1000)   | 96  | 96  | 80  | 73  | 100 | 89      |
| `+reranker+both`    | 1 (200)    | 100 | n/a | 100 | 100 | 100 | 100     |
|                     | 2 (400)    | 100 | 100 | 100 | 100 | 100 | 100     |
|                     | 3 (600)    | 100 | 100 | 93  | 100 | 100 | 98      |
|                     | 5 (1000)   | 96  | 100 | 96  | 67  | 100 | 93      |

**a. The written layer closes most of the vocabulary gap.** Axis D moves
33% → 80% at K=10 and 80% → 93% at K=50 from two blind examples per
operation (`e5-large-q8+written` against `e5-large-q8` alone). This is the
result the subproject exists for: the four questions in
`docs/specs/describing.md` section 1 (立て替えた分を出したい → 経費申請の
作成; お金を返してもらいたい → 精算の作成; 商品が届いたので受け取り処理を
したい → 検収の作成; 値段を安くしてほしいと頼みたい → 値引の作成) were
unreachable inside 50 candidates by every retriever measured, and
unanswered by every model, local or frontier, from a shortlist
(`DECISIONS.md`, 2026-09-14 and 2026-09-15). They are reachable once the
catalogue carries the words - `createExpenseReimbursement`'s own written
examples are 「立て替えた分を精算してほしい」/「自腹で払った分の精算お願い」,
and `createPurchasingGoodsReceipt`'s are 「荷物届いたから検収登録したい」/
「検収の記録つけたい」.

**b. The generated layer is a negative result.** Axis D does not move
(33% either way against the bare retriever), and axis B falls 100% → 64%:
発注 and 受注 both paraphrase toward 「注文」, the side effect
`docs/specs/describing.md` G7 named. `+both` is worse than `+written` on D
at every K measured (67 vs 80 at K=10, 87 vs 93 at K=50) because the
generated utterances' noise drags the union down - adding a layer that does
not move D on its own does not help D when unioned with one that does.

Two prompts were tried, same conclusion. The first (2026-09-15, `de4dc96`'s
predecessor) asked for "short ways of saying this operation" and produced
conjugated repeats of the noun - `createExpenseReimbursement` →
「経費申請作って」「経費申請作成」「経費申請作りたい」 - the noun with verb
endings, which the retriever already reads, and which bridges nothing:
across 4,960 utterances, 届いた/受け取/安く appeared zero times. The rewrite
(`de4dc96`) added two few-shot examples from outside the fixture and an
explicit ban on the operation's own words; the same model then wrote
「仕入れの荷物が届いたから確認したい」 for `createPurchasingGoodsReceipt` and
「この商品だけ安くしたいんだけど」 for `createSalesDiscount` - the target
register, novelty 49.6% over the regenerated 4,991 utterances (2,475/4,991,
recorded at generation time) - and axis D still did not move. Getting the
register right and bridging the vocabulary gap are different problems; this
layer solved the first and not the second.

Three same-operation comparisons, generated vs. written, read from the
cache directly (no model call - both layers are already on disk):

| operation                                   | written                                                   | generated                                                                                                                                                                                    |
| ------------------------------------------- | --------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `createExpenseReimbursement` (精算の作成)   | 立て替えた分を精算してほしい / 自腹で払った分の精算お願い | 領収書入力の時間ある？ / 経費の精算書作りたい / 今月の請求書いつ出せる？ / 領収書忘れたけどどうする？ / 経費申請のフォーム開ける？                                                           |
| `createPurchasingGoodsReceipt` (検収の作成) | 荷物届いたから検収登録したい / 検収の記録つけたい         | 仕入れの荷物が届いたから確認したい / 納品書と品物が合っているか見てほしい / 受け取った商品に傷がないかチェックしたい / 請求書と実物が一致するよう確認したい / 入庫前の最終確認をお願いします |
| `createSalesDiscount` (値引の作成)          | 特別に値引き設定したい / 新しい割引登録して               | 値引き率をどう設定すればいいの / この商品だけ安くしたいんだけど / 特別価格の申請書作って / 割引条件を間違えてない？ / 値引き承認の画面どこ？                                                 |

The generated layer's phrasing is fluent and sometimes on-topic (検収's
generated utterances are arguably better prose than the written ones), but
it drifts - 今月の請求書いつ出せる？ and 領収書忘れたけどどうする？ are not
requests to create a 精算 at all, and 割引条件を間違えてない？/値引き承認の
画面どこ？ are questions about a discount, not requests to make one. The
written layer stays anchored to the operation the whole time.

**c. The novelty metric points the wrong way.** Contract rates, sampled
every 50th operation (20 operations): generated - retrieval 6%, novelty
43%, over 100 utterances; written - retrieval 25%, novelty 15%, over 40
utterances; both - retrieval 11%, novelty 35%, over 140 utterances. The
layer that works has the _lower_ novelty. The written layer's 30 failing
triples (of 40 sampled, `/tmp/claude-1000/-home-takahiro-ghq-github-com-mktkhr-app-orchestra/24470cde-9051-443e-b7b6-397284bcb3c1/scratchpad/written-contract-failing-triples.json`)
show why: the ones that fail are the ones that dropped the anchor noun
entirely -

- 「適用中のものを一覧で見たい」(`listSalesDiscounts`) is won by
  `listInventoryExpiryDates` - the utterance kept no word 値引 or 割引 at
  all, so it retrieves as a generic "things currently in effect" query.
- 「条件絞って探したい」(`searchSalesOrders`) is won by
  `searchAttendanceQualifications` - 条件 alone, with no 受注/注文, reads as
  a search over any kind of condition, and 資格 (qualification) is exactly
  that.
- 「今日の受注どれくらいある?」(`listSalesOrders`) is won by `getSalesOrder`
  - the noun survived, the verb did not: 「どれくらいある」reads closer to a
    single record's quantity than to a list.

A useful utterance keeps the operation's noun as an anchor and adds the
everyday situation around it; pure novelty is drift, not signal.

`generated-contract-non-novel.json` (same directory) has a different shape

- not failing retrieval triples but the 57 (of 100 sampled) generated
  utterances that share a bigram with their own operation's text despite the
  rewritten prompt's ban, e.g. 「品番と名前が知りたい」and 「品目リストはどこに
  ある？」for `listInventoryItems`, 「今月の経費全部見たい」for
  `listExpenseClaims`. It shows the ban suppresses the worst failure mode
  (pure noun-plus-conjugation) but does not stop the model drifting back
  toward the summary's own words on roughly half its output even under the
  corrected prompt.

**d. The reranker cannot see the utterances, and it shows.** `+written`
alone reads axis D 80 at K=10; `+reranker+written` reads 73. The
cross-encoder rescores on `combinedTextOf` only (`docs/specs/describing.md`
section 4) - an operation retrieved into the fifty by its written example
is pushed back down by a reranker that never read the example that put it
there. The reranker still pays for itself on the other axes: A 92→96, E
80→100, overall 84→89. Whether to let the reranker read the written
examples is the obvious next experiment and is **not** done here (see
"what this does not settle" below).

**e. The trade the two headline rows make.** `+reranker+both` reads 93
overall with D 67 at K=10; `+reranker+written` reads 89 overall with D 73.
Under the reranker the generated layer's noise is filtered enough that its
coverage helps axis C (80 → 96 against `+reranker` alone), while without
the reranker (`+both` vs `+written`, no reranker) the same generated layer
is purely harmful on D (67 vs 80). This entry does not pick a winner
between the two headline rows; `docs/specs/describing.md` section 7 is
corrected to say so explicitly.

**f. Costs.** Per-question wall-clock at 1000 operations (`ms/query`,
narrowing call only, no LLM): `e5-large-q8` alone 6.418ms;
`+generated` 14.696ms (2.3x, +8.3ms); `+written` 9.941ms (1.5x, +3.5ms);
`+both` 17.897ms (2.8x, +11.5ms) - the utterance rows are slower in
proportion to how many extra vectors they score against, generated (~5 per
operation) costing more than written (~2 per operation). With the
reranker: `e5-large-q8+reranker` alone 81.208ms; `+reranker+written`
86.850ms (+5.6ms, 1.07x); `+reranker+both` 88.844ms (+7.6ms, 1.09x) - the
two-stage cost dwarfs the utterance-scoring cost once a reranker is in the
loop. The one-time generation cost was 625.8s for the full 1000-operation
catalogue (`de4dc96`), never paid again unless an operation's text or the
prompt changes (AC-G-101). Utterance vector counts: 4,991 generated (five
requested per operation, a few operations returned fewer), 2,000 written
(exactly two per operation, by construction). `make check` still calls no
model: `docker logs llama-swap 2>&1 | grep -c 'POST /v1/'` read 61261
before this record's `make check` run and is confirmed unchanged below.

**g. How the written layer was produced (AC-G-105).** Five Sonnet
subagents, one service each (`1b77389`), each allowed to read only its own
service's definition table, the fixture types, and the generator; each
forbidden the corpus, the generated utterances, the specs, and the decision
record (`docs/specs/describing.md` G6). None reported reading outside that
scope; two looked at unrelated test files for the import convention. 2,000
examples, two per operation, none missing. The written layer's numbers
above are therefore a measurement of a blind stand-in for a service
owner's ear - five agents given a table of operation ids and summaries -
not of a service owner who actually knows how their users talk. That
distinction is `docs/specs/describing.md` section 10's own caveat, restated
here because every number in (a)-(e) rests on it.

**Verification.** `docker logs llama-swap 2>&1 | grep -c 'POST /v1/'` read
61261 before `make check` and 61261 after - `make check` still calls no
model. `make fmt && make check` green.

## 2026-09-15 Measuring the pick: three report rows, and whether the written layer's recall gain survives to the pick

**Context.** TODO.md item 1 ("Measure the pick, not only the recall"):
`make narrowing` reports recall@K, but the product's own number is "right
operation chosen, or asked back". The hand measurement above ("The local
picker reads the shortlist in reranker order: 78 → 83, for nothing") found
K=20 best and was run once, by hand, in a session scratchpad, on the plain
`e5-large-q8+reranker` shortlist only. This entry turns that into three
report rows - `pick:e5-large-q8+reranker`, `pick:e5-large-q8+reranker+written`
and `pick:e5-large-q8+reranker+both` - each feeding `qwen3.5-9b-q8`
(thinking off, temperature 0) the top `PICK_SHORTLIST_K` (20,
`e2e/narrowing/gather-pick-shortlists.ts`) candidates of the same reranked
shortlist the corresponding recall row already produces, scored on two
axes: **correct** (the picked operationId is among the question's answers)
and **flagged** (the picker returned `ambiguous`) - both printed, never
combined into one score. New modules: `e2e/narrowing/pick/client.ts` (the
picker's own transport, injectable exactly like `embedding/client.ts`),
`gather-pick-shortlists.ts` (every shortlist for every variant, gathered
first), `gather-pick-scoring.ts` (the picker run over the whole collected
set in one pass) and `gather-pick.ts` (orchestration) - two passes,
deliberately, so a run alternates between the reranker and the chat model
exactly once, not once per question, matching the discipline the retrieval
and reranking stages already follow.

**The numbers, 1000 operations (K=20 candidates shown to the picker), correct%/flagged%:**

| row                                  | A      | B      | C     | D     | E     | overall |
| ------------------------------------ | ------ | ------ | ----- | ----- | ----- | ------- |
| hand measurement (2026-09-15, above) | 100/—  | 100/—  | 72/—  | 53/—  | 70/—  | 83/50   |
| `pick:e5-large-q8+reranker`          | 100/56 | 100/52 | 72/52 | 53/27 | 70/70 | 83/51   |
| `pick:e5-large-q8+reranker+written`  | 84/48  | 96/36  | 68/64 | 60/20 | 70/70 | 78/47   |
| `pick:e5-large-q8+reranker+both`     | 84/52  | 96/20  | 72/72 | 47/13 | 70/60 | 77/44   |

**a. The plain row reproduces the hand measurement almost exactly.**
`pick:e5-large-q8+reranker` reads 83% correct overall against the hand
run's 83 (identical, not just close) and 51% flagged against the hand
run's 50% - the one-point difference is consistent with a single question
flipping and is well inside what one run at n=100 can be expected to move
by chance. Every per-axis correct figure matches the hand table exactly
(A 100, B 100, C 72, D 53, E 70). This is the confirmation the report row
is doing what the hand script did: same shortlist, same K, same prompt,
same parsing rule, and the same result.

**b. The written and "both" shortlists do not carry the recall gain to
the pick - they cost it.** Recall@20 at 1000 operations barely moves
between the three retrieval variants (`e5-large-q8+reranker` 92% overall,
`+written` 92%, `+both` 96% - see the table above this entry's own recall
block). The pick tells a different story: overall correct falls from 83%
(plain) to 78% (`+written`) to 77% (`+both`) - the utterance layers that
help or hold recall make the picker's raw choice _worse_, not better,
despite the right answer being inside the same top-20 window at least as
often. The most likely mechanism: the utterance layers change which
twenty candidates the picker sees and in what order (an operation can now
be retrieved by an utterance vector rather than its own text), and a
9B model with 200 output tokens is more easily distracted by additional
plausible-looking candidates than a bigger shortlist alone would predict
from recall - recall asks only "is the answer somewhere in twenty", the
pick asks the harder question this subproject exists to measure.

Axis D is the one place the direction matches what recall predicts:
`+written` raises pick-D from 53% (plain) to 60%, in the same direction as
its recall gain (D closes the vocabulary gap in retrieval, per
`docs/specs/describing.md`, and some of that reaches the pick too) - but
`+both` _drops_ pick-D to 47%, below the plain row, even though `+both`'s
own recall-D (80%) sits between plain (73%) and written (87%). The
generated layer's noise that already costs `+both` on recall-D costs it
again, harder, on pick-D: the axis this subproject was built to close is
the one axis where mixing in the generated layer actively hurts the
picker beyond what it costs the retriever.

**c. Flagged rates, read against axis B and axis A.** Axis B (the
genuinely ambiguous questions, where a high flagged% is desired behaviour)
flags at 52% (plain), 36% (`+written`) and 20% (`+both`) - falling as more
retrieval signal is added, the opposite of what would be wanted if the
goal were "flag more of the genuinely ambiguous ones": a shortlist that
retrieves more confidently apparently reads as less ambiguous to the
picker even when the underlying question still has several defensible
answers. Axis A (unambiguous questions, where a high flagged% is a false
alarm) flags at 56%, 48% and 52% - a false-alarm rate higher than axis B's
own true-positive-shaped rate in two of three rows. Axis E (lexical
decoys, not scored here as either "desired" or "false alarm" - the report
prints the number and leaves the read to whoever asks) flags at 70%, 70%
and 60% - the highest flag rate of any axis in every row, which reads as
the picker noticing something is off about a decoy-heavy shortlist without
being told which axis it is looking at. None of this is a clean signal
that `ambiguous` tracks axis B specifically; it moves in the same
direction across every axis about as often as not.

**d. Wall-clock.** All three rows land at 302-303ms/question at 1000
operations (`pick:e5-large-q8+reranker` 302.224, `+written` 303.112,
`+both` 302.690) - indistinguishable from each other (the picker call
dominates; the shortlist itself is a few hundred microseconds of local
scoring plus one reranker call already measured elsewhere) and consistent
with the hand run's 0.31s/question at K=20.

**Sequencing verified.** `gather-pick.ts` runs after every recall row and
reuses the same cached `e5-large-q8` catalogue vectors and utterance
vectors those rows already embedded (a disk read, not a new embedding
call) - the reranker call itself is not cached anywhere in this codebase
and does run again per (variant, size, question) to build the picker's
shortlist, the same cost every existing reranked row already pays once for
its own recall figures. The picker itself runs in one pass, after every
shortlist for all three variants was gathered, so the whole run alternates
between the reranker and `qwen3.5-9b-q8` exactly once - confirmed by the
loading-cost block printing once, unchanged in shape, at the end of the
report.

**The thinking-budget guard.** `chat_template_kwargs.enable_thinking: false`
is set on every request; `pick/client.ts`'s `assertNoThinkingLeak` is the
extra safety net, checked once on the first real response of a run
(`finish_reason === "stop"` and no non-empty `reasoning_content`).
Verified twice against the real transport: flipping `enable_thinking` to
`true` in a throwaway, uncommitted edit produced `finish_reason: "length"`
and non-empty `reasoning_content` on the very first call, and the guard
threw immediately with a message naming both fields before the edit was
reverted; a second real call with `enable_thinking: false` restored picked
correctly and did not throw. The flip was never committed.

**Verification.** No pre-existing row's output changed: `make narrowing`
before and after this change, diffed with every `ms/query`-style number
stripped, is byte-identical. `make fmt && make check` green, zero
suppressions. `docker logs llama-swap 2>&1 | grep -c 'POST /v1/'` read
61261 at the start of this session; `make check` itself made no live call
(the count was unchanged immediately before and after it ran); the full
count after every manual `make narrowing` run and the two thinking-guard
verification calls in this entry is 73541 - the difference is real model
traffic from `make narrowing` runs and the two throwaway verification
calls, never from `make check`.

## 2026-09-15 Letting the reranker read the written examples: it fixes retrieval, it does not fix the pick

**Context.** TODO.md item 1, the caveat left open in "The catalogue says it"
(d): `+written` alone reads axis D 80% at K=10, but `+reranker+written`
reads 73% - the cross-encoder rescores on `combinedTextOf` alone
(`docs/specs/describing.md` section 4), so an operation retrieved into the
fifty by its written example is pushed back down by a reranker that never
read the example that put it there. This entry measures the fix on both
sides the finding named: the reranker (a new recall row) and the pick (two
new pick rows), because "Measuring the pick" (above, same date) found the
pick blind to the written layer in the same way, for a different reason -
the picker sees `operationId / service / summary` per candidate, not the
example that got the operation into its shortlist.

**A mistake caught before it was recorded.** The first version of this
change edited `PICK_SYSTEM_PROMPT` in place to add the one sentence the
picker needs to know what an `e.g.` column is. That constant is shared by
every pick call, so it silently changed the prompt for the three
pre-existing pick rows too - `pick:e5-large-q8+reranker` moved from
83%/51% correct/flagged (A100/56 B100/52 C72/52 D53/27 E70/70, K=20, 1000
ops) to 77%/47% (A96/36 B96/52 C64/60 D47/27 E60/60) on that one added
sentence alone, nothing else changed. The fix: `PICK_SYSTEM_PROMPT` is
untouched, byte-for-byte identical to the version at `867e889`; a second
constant, `PICK_SYSTEM_PROMPT_WITH_EXAMPLES`, carries the added sentence,
and `pick/client.ts`'s `pick()` takes it as an explicit, opt-in parameter
that only the two new rows below pass. Verified by re-running the whole
report and diffing against the pre-change baseline with `ms/query` and the
one-time loading-cost benchmark stripped: byte-identical on every
pre-existing row, recall and pick alike. Worth its own line rather than a
footnote: a 9B picker moved eleven points overall on one added sentence it
was never meant to see, on a shortlist and K that did not change at all -
whatever this picker is asked to read next, that prompt is a new baseline,
not a small edit.

**The reranker half held, cleanly.** `e5-large-q8+reranker(w)+written`
reranks on `combinedTextOf(operation)` plus the operation's own written
examples, one per line (`e2e/narrowing/utterances/reranker-written.ts`) -
retrieval is scored with the written layer exactly as `+reranker+written`
already does it; only the reranked document text changes. Generated
utterances are kept out of this document on purpose (the previous entry's
(b) already shows them as noise; feeding that noise to the reranker's
document text would not be a new experiment).

K=10/20/50, 1000 operations:

| configuration                     | K   | A   | B   | C   | D   | E   | overall |
| --------------------------------- | --- | --- | --- | --- | --- | --- | ------- |
| `e5-large-q8+written`             | 10  | 92  | 96  | 68  | 80  | 80  | 84      |
|                                   | 20  | 96  | 96  | 84  | 93  | 80  | 91      |
|                                   | 50  | 96  | 96  | 88  | 93  | 100 | 94      |
| `e5-large-q8+reranker+written`    | 10  | 96  | 96  | 80  | 73  | 100 | 89      |
|                                   | 20  | 96  | 96  | 84  | 87  | 100 | 92      |
|                                   | 50  | 96  | 96  | 88  | 93  | 100 | 94      |
| `e5-large-q8+reranker(w)+written` | 10  | 96  | 96  | 80  | 80  | 100 | 90      |
|                                   | 20  | 96  | 96  | 84  | 93  | 100 | 93      |
|                                   | 50  | 96  | 96  | 88  | 93  | 100 | 94      |

Axis D is exactly what the hypothesis predicted: `+reranker(w)+written`
reads 80/93/93 at K=10/20/50 - not close to `+written`'s own blind D, equal
to it at every K. Reading the examples gives back exactly what the blind
reranker took away. Overall moves with it, 89→90 at K=10 and 92→93 at
K=20 (both from D alone; E was already 100 both ways at K=10, and 100
again matches at K=20 - the earlier record's 90 vs 100 gap at K=20 was
`+reranker+written`'s only other soft spot and this variant does not touch
it). A, B, C and E are unchanged at every K, 1000 operations - not
"close", identical to the pips. Contract rate is the same 25%/15%
retrieval/novelty, expected: it is the same written layer, same catalogue,
same sample; only the reranked document text differs. Cost: 101.8ms/query
against `+reranker+written`'s 94.0ms/query - about 8% slower, from sending
a longer document to the reranker.

One honest exception, at smaller catalogue sizes only: at 200 operations,
axis C reads 80% at K=10 for `+reranker(w)+written` against 100% for
`+reranker+written`, converging to 100% by K=50; at 600 operations, C reads
67% at K=10/20 against 73%, again converging to 80% by K=50. Both gaps
close before K=50 and neither appears at the full 1000-operation catalogue,
where C is identical at every K - recorded here because the instructions
this entry answers to say to look for a moved axis and say so either way,
not because it changes the verdict at the size this subproject reports.

**The pick half did not hold - showing the examples makes the picker's
raw choice worse, not better.** Two new rows, both against the written
shortlist, to tell the two effects apart:

- `pick:e5-large-q8+reranker(w)+written` - the reranker reads the
  examples (the row above's shortlist), and the picker is shown them too.
- `pick:e5-large-q8+reranker+written(shown)` - the plain `+written`
  shortlist, byte-unchanged, with the examples shown to the picker only.

Both use `PICK_SYSTEM_PROMPT_WITH_EXAMPLES`; the three rows below keep the
untouched `PICK_SYSTEM_PROMPT` and are confirmed byte-identical to their
own prior numbers (above).

K=20, 1000 operations, correct%/flagged%:

| row                                        | A      | B      | C     | D     | E     | overall |
| ------------------------------------------ | ------ | ------ | ----- | ----- | ----- | ------- |
| `pick:e5-large-q8+reranker`                | 100/56 | 100/52 | 72/52 | 53/27 | 70/70 | 83/51   |
| `pick:e5-large-q8+reranker+written`        | 84/48  | 96/36  | 68/64 | 60/20 | 70/70 | 78/47   |
| `pick:e5-large-q8+reranker(w)+written`     | 68/40  | 84/12  | 60/80 | 60/20 | 70/50 | 69/41   |
| `pick:e5-large-q8+reranker+written(shown)` | 72/52  | 84/20  | 60/52 | 60/27 | 70/50 | 70/40   |

Against `+written`'s own pick row (the closest comparison, same
shortlist), both examples-shown variants read _worse_ on overall correct
(78 → 69 and 70), on axis A (84 → 68 and 72 - the unambiguous questions,
where a wrong pick or a false flag is a straightforward loss), and on axis
B correct (96 → 84 both) - D and E are unchanged (60/20 and 70/~50 in all
three written-shortlist rows), the one place the direction matches what
the reranker's own gain would predict, and it is a wash rather than a
gain: showing the picker the very thing that recovered axis D at the
reranker did not carry that recovery to the pick.

Flagged rates move the wrong way for what a flagged rate is supposed to
mean. Axis B (genuinely ambiguous, where a high flagged% is desired) falls
36% (`+written`) → 12% (`+reranker(w)+written`) → 20%
(`+written(shown)`) - showing the examples makes the picker _less_ likely
to flag the questions it should be flagging, not more. Axis A (unambiguous,
where a high flagged% is a false alarm) moves in both directions across
the two new rows - 48 → 40 for `+reranker(w)+written` (fewer false alarms,
alongside fewer correct picks - not a trade worth taking) and 48 → 52 for
`+written(shown)` (more false alarms, with no accuracy gain to show for
it). Axis C, the closest thing to a genuine improvement in this pair,
reads 64 → 80 correct for `+reranker(w)+written` alone - but that row's own
overall is still down 9 points from `+written`, so this one axis's gain
does not rescue the row.

**Wall-clock.** The two new pick rows read ~416ms/question (`
+reranker(w)+written` 415.9, `+written(shown)` 418.0) against the three
untouched rows' ~305-309ms/question - `+reranker(w)+written`'s shortlist
itself costs more (101.8ms vs 94.0ms for the reranker call, per the recall
table above), and both new rows' user message is longer (every candidate's
summary line now carries an `e.g.` column), which is more tokens for the
9B model to read before it answers - consistent with, not simply equal to,
the shortlist's own added cost.

**Verdict, stated plainly and per axis.** The hypothesis held for the
reranker and failed for the picker, and the two verdicts do not cancel out
into a net recommendation to wire this up as one mechanism:

- Does written-in-reranker recover axis D recall? Yes, completely, at
  every K measured, at zero measured cost to A, B or E and (at the full
  1000-op catalogue) to C, for about 8% more reranker latency.
- Does showing examples to the picker recover the picker's own score?
  No. Overall correct falls further below `+written`'s own pick row in
  both new variants (69 and 70, against 78), not just short of the
  reranker's own gain.
- Does it raise false alarms on axis A or lower recall on axis B? Recall
  (the retrieval-side measurement) does not move on either axis, at any K,
  at 1000 operations - the "does it cost something elsewhere" question the
  reranker half was measured against comes back clean. The pick's own
  axis-B _flagged_ rate (not recall) falls, the wrong direction for what a
  flagged rate on the genuinely-ambiguous axis is supposed to do, and
  axis-A's flagged rate moves in opposite directions across the two new
  rows depending on which side gained the examples - neither reads as a
  false-alarm improvement.

`docs/specs/describing.md` section 4 is corrected to record the reranker
answer; section 10 loses the now-answered open question, with a note that
the picker side reopens a narrower one (whether a different presentation
of the examples, not just showing the same list, would read differently to
a 9B model) rather than settling it.

**Aside: `make check` inside a git worktree.** Found 2026-09-15 while
pushing, unrelated to the measurement above but recorded here per the
housekeeping note that asked for it: running `make check` inside a `git
worktree` (rather than a plain clone) produces a false `services-lint`
failure from `harness/guard/archcheck` - `golangci.yml`'s
`relative-path-mode: gitroot` cannot resolve the git root when `.git` is a
file (a worktree's `.git` is a pointer file, not a directory), so the
`^harness/` forbidigo exclusion stops matching anything and every
`harness/`-rooted file trips the rule meant to exempt it. A real clone
(this session's own working directory included) is unaffected.

**Verification.** `docker logs llama-swap 2>&1 | grep -c 'POST /v1/'` read
90224 immediately before the corrected-prompt `make narrowing` run in this
entry and 99567 after - all of that difference is the live report itself
(one recall row plus five pick rows, `PICK_SHORTLIST_K` candidates each,
over 500 questions x 4 catalogue sizes), never from `make check`, which
made no live call either side of it. `make fmt && make check` green, zero
suppressions.

## 2026-09-15 Two defects the shortlisting fixture measured, fixed

Measuring the real planner against the five-service, 1000-operation
fixture (`docs/specs/shortlisting.md`, 100 questions, narrowing on) turned
up two defects, both in `services/platform/`, neither visible on the
product's earlier two-service catalogue.

**Defect 1: `ask_user` asked the model to invent a service name.** 7 of
100 answers were HTTP 500 `endpoint not found in catalogue:
<service>/<operationId>`, every one an `ask_user` call whose arguments were
fabricated - `approval/createApproval`, `salesBundle/createSalesBundle`,
`summarize/summarizeSalesInvoices`, `expense/ask_user` (the model naming
its own tool as the operation it was stuck on). `AskUserTool`'s schema
(`internal/usecase/tools.go`) declared `service` as a required free string,
alongside `operationId`; with two services the model never had to guess
right, with five it does, and nothing checked the guess against what the
catalogue actually offered.

Decision: remove `service` from `AskUserTool` entirely rather than
validate it after the fact. `toolcall.Planner.decisionFromAskUser`
(`internal/adapter/planner/toolcall/planner.go`) is now a method with
access to `p.catalog`, and resolves the service from `operationId` via the
same `resolveService` a real tool call already uses - the model is simply
never asked a question it cannot reliably answer. When `operationId` does
not resolve (absent, or invented) the Decision carries only `Question`,
with `Service`, `OperationID`, `Param` and `Options` all left zero-valued;
`Orchestrator.ask` (`internal/usecase/orchestrator.go`) turns that into a
plain `Result{Kind: ResultKindAsk, Question: ...}` instead of
`ErrEndpointNotFound` - an ask is never a 500, it just has less to offer
than the model tried to hand back. This is deliberately placed in
`usecase`, not either planner adapter: `jsonmode.Planner` never had an
`ask_user` tool schema to begin with (its "ask" shape is free JSON, still
naming its own `service`/`operationId`), but it gets the same graceful
degrade for free because the fix lives where both planners' decisions
converge.

The wire contract needed no change: `PlanResult`'s `ask` variant already
declares `param` and `options` as optional, not required (only `kind` is
required at the top level), so a plain ask with neither is already legal
JSON under `api/openapi.yaml`. What did need fixing was
`internal/adapter/handler/plan.go`, which unconditionally set
`out.Param = &result.Param` and always called `toAPIOptions` - for the new
plain-ask case that sent a pointer to `""` and a pointer to `[]`, not the
absent fields the contract allows. Both are now only set when non-empty.

**Defect 2: planning was not deterministic.** `chat.Request` sent no
`temperature` field at all, so llama-server's own default applied; the
same 100-question fixture scored 32 and then 16 on one axis across two
runs, with nothing else about the fixture, the catalogue or the model
changed. A planner that cannot be measured reliably cannot be improved
reliably either - variance this large swamps any real effect a future fix
would have.

Decision: `chat.Request` gained `Temperature *float64` - a pointer, not a
bare `float64`, because the zero value has to mean something different
from "not set" (an endpoint's own default is a real, if never-used-today,
option). `chat.Zero()` builds the fixed value (`0.0`) every planning call
now passes: `toolcall.Planner.Plan`'s one request, and both of
`jsonmode.Planner.complete`'s attempts (the `response_format` call and its
retry-without-it fallback). Adding the field pushed `Request` to exactly
golangci-lint's gocritic `hugeParam` threshold (80 bytes), so
`chat.Client.Complete` moved from taking `Request` by value to `*Request`

- every call site (both planners, every table test in
  `internal/adapter/planner/chat/client_test.go`) updated to pass a pointer.
  `internal/adapter/planner/chat/client_test.go` now asserts
  `"temperature":0` lands on the actual wire body for a Request that sets
  `chat.Zero()`, and a separate test asserts the field is absent - not
  `0` - when Temperature is left nil, so "unset" and "explicitly zero" stay
  distinguishable at the transport boundary, not just in the Go type.

**Verification.** `docker logs llama-swap 2>&1 | grep -c 'POST /v1/'` read
101249 both before this work and after `make check` (`services-fmt-check`,
`services-lint`, `services-test`, `services-build`, `guard-suppressions`,
`guard-arch`, `guard-fsd`, `guard-coverage` all green) - no model call was
made anywhere in the loop. `web-lint` was left red at the time of this
work, but for reasons entirely outside this defect fix's scope: another
agent's concurrent, in-progress edits under `e2e/**`
(`git status` showed only `e2e/narrowing/serve.test.ts`,
`e2e/shortlist/*.ts` modified, none of them touched here). Not re-measured
against a live model - `make check` runs no model by design, and this fix
was made and verified against the two defects' own root cause (the
schema/prompt shape and the wire request), not by re-running the
100-question fixture, which stays a manual, opt-in exercise
(`ORCHESTRA_LIVE_LLM=1`).

## 2026-09-15 A repetition loop with no budget: the third defect the shortlisting fixture measured

Reproduced deterministically after the two defects fixed earlier the same
day (see "Two defects the shortlisting fixture measured, fixed", above):
fixture platform, narrowing on, `POST /api/plan
{"query":"明細を1件確認したい"}`. Narrowing took 9ms + 88ms, then the chat
completion never returned headers at all; after exactly 120s (the
transport's own `requestTimeout`) the platform answered 500 "context
deadline exceeded (Client.Timeout exceeded while awaiting headers)", and
llama-swap's own log read `POST /v1/chat/completions 499 0 ...
2m0.000s` - the model was still generating when the client gave up. 4 of
the first 42 questions in the 100-question run hit this (a20, b07, b08,
b12), each costing the full 120s and an error.

Cause: `chat.Request` sent no `max_tokens` at all, so llama-server's own
default (`n_predict = -1`, unbounded) applied. At temperature 0 (the
previous defect's fix) with twenty strict tool schemas offered, the model
can fall into a repetition loop with nothing to stop it - and every
planning answer this product ever needs is short, one tool call or one
sentence, so an unbounded budget buys nothing on a clean answer and costs
the entire timeout on a looping one.

Decision: `chat.Request` gains `MaxTokens *int`, the same pointer pattern
`Temperature` already established (nil is genuinely "unset"). `MaxTokens`
builds the fixed value every planning call now sends - `1024`, generous
enough for the largest real answer (a `propose_panel` call with `chart`
and `transform` both filled in) with room to spare, tight enough that a
loop costs a few seconds of generation instead of two minutes of silence.
Both `toolcall.Planner.Plan`'s one request and both of
`jsonmode.Planner.complete`'s attempts (with and without
`response_format`) now set it. The reproduction above lives in
`planningMaxTokens`'s own doc comment in `chat/client.go`, next to the
constant it explains, per the request that raised this defect.

A response whose `finish_reason` comes back `chat.FinishReasonLength`
("length") is a truncated answer, and a truncated answer - a partial tool
call, a JSON object that never closed - is not a decision to decode, valid
or not: decoding it anyway risks acting on a hallucinated fragment the
model never actually finished choosing. `toolcall.Planner.Plan` treats it
exactly like no tool call at all: `usecase.DecisionNone`, no attempt to
decode `resp.Message.ToolCalls[0]`. `jsonmode.Planner.Plan` folds it into
the retry mechanism Task 11 already built for a bad answer - a new
`ErrTruncated` sentinel stands in for a parse error, so the same
quote-it-back retry prompt fires - but with one difference from a
genuinely invalid answer: exhausting both attempts on truncation alone now
ends in `usecase.DecisionNone`, not the `"planner gave up after %d
attempts"` error two bad JSON answers in a row still produce. The
distinction is deliberate - a truncated answer is not a bug in the model's
JSON to report, it simply ran out of budget, and "no usable decision" is
the honest description of that outcome, the same one `toolcall.Planner`
gives for an empty tool-call list.

Both paths log one line at warn (`slog.Default().WarnContext`) before
degrading, carrying `chat.Preview` of the truncated content - the first
200 runes, via a small exported helper rather than duplicating a
truncation loop in both planner packages. This is the whole point of the
fix's third requirement: the next person debugging a slow or wrong plan
should be able to see what a repetition loop actually generated from the
log line alone, without spending 120s reproducing the timeout that first
surfaced it.

**Verification.** `docker logs llama-swap 2>&1 | grep -c 'POST /v1/'` read
101396 both before this work and after `make check`'s Go-side gates
(`services-fmt-check`, `services-lint`, `services-test`, `services-build`,
`guard-suppressions`, `guard-arch`, `guard-fsd`, `guard-coverage`, all
green) - no model call was made anywhere in the loop. New tests: at the
transport boundary, `client_test.go` asserts `"max_tokens":1024` lands on
the wire alongside `"temperature":0`, and exercises `chat.Preview` on both
a short string (untouched) and one longer than `PreviewLen` (cut to
exactly `PreviewLen` runes, not bytes - the fixture uses multi-byte
Japanese characters to catch a byte-slicing mistake). At the planner
boundary: `toolcall`'s `TestPlanMapsATruncatedAnswerOntoDecisionNone`
(a truncated tool call maps to `DecisionNone`, never decoded);
`jsonmode`'s `TestPlanRetriesOnceOnATruncatedAnswer` (one truncated
answer, one clean one, succeeds after the retry) and
`TestPlanOnTwoTruncatedAnswersInARowReturnsDecisionNoneNotAnError` (two in
a row still resolves to `DecisionNone`, not the pre-existing
"gave up" error `TestPlanGivesUpAfterASecondBadAnswer` still asserts for a
genuinely invalid answer twice).

## 2026-09-15 The product's planner, measured end to end: 65 against the picker's 83, and what the measurement found and fixed on both sides

**Context.** `docs/plans/shortlisting.md` Task 4, Step 6 - the record. Steps
1-5 wired narrowing behind config (H1-H4), alternatives onto the answer
(H5), the config switch (H7), and `make eval-shortlist`, which asks the
built platform - fixture services behind it, real llama-swap, the
five-service/1000-operation corpus - the 100 corpus questions through
`/api/plan`, narrowing on and off, and scores correct@1, correct@shown,
asked-back, none, error and latency per axis (H6, AC-H-106). This entry is
the numbers AC-H-108 asks for, beside the stand-in picker's 83 and the
92-96% recall@20 ceiling already on record (2026-09-15, "Measuring the
pick" and "the corpus answer key is fixed"). `PRODUCT.md` D2 is left to
its owner, per AC-H-108 - not revised here.

**The headline.** The product's own planner, narrowing on, scores **65
correct@1 / 68 correct@shown** of 100; narrowing off, **62 / 62**. Beside
them: the stand-in picker scores 83 on the identical K=20 shortlist, and
the shortlist itself holds the right answer 93% of the time. Every result
row's `via` is `"plan"` - a genuine `/api/plan` response, never the
`invoke-500` fallback (`af5aeea`) - 67 of 67 result rows on, 79 of 79 off.
No error in either pass, at any axis.

**Narrowing on (correct@1 / correct@shown / asked / none / error, out of
25/25/25/15/10 per axis):**

| axis    | correct@1 | correct@shown | asked | none | error |
| ------- | --------- | ------------- | ----- | ---- | ----- |
| A       | 23 (92%)  | 23 (92%)      | 0     | 1    | 0     |
| B       | 14 (56%)  | 14 (56%)      | 1     | 6    | 0     |
| C       | 14 (56%)  | 15 (60%)      | 0     | 1    | 0     |
| D       | 7 (47%)   | 7 (47%)       | 0     | 2    | 0     |
| E       | 7 (70%)   | 9 (90%)       | 0     | 0    | 0     |
| overall | 65 (65%)  | 68 (68%)      | 1     | 10   | 0     |

Latency (ms): mean 5742, p50 3917, max 16382, 41 of 100 over 5000ms. Per
axis (mean / p50 / max / over-5000ms count): A 3488/2782/15940/2, B
8618/7771/16382/19, C 4943/3953/15914/9, D 7560/6951/16372/11, E
3455/3518/4167/0.

**Narrowing off (correct@1 / correct@shown - identical, no alternatives to
carry a wrong pick - / asked / none / error, same axis sizes):**

| axis    | correct@1 = correct@shown | asked | none | error |
| ------- | ------------------------- | ----- | ---- | ----- |
| A       | 25 (100%)                 | 0     | 0    | 0     |
| B       | 7 (28%)                   | 0     | 7    | 0     |
| C       | 19 (76%)                  | 0     | 0    | 0     |
| D       | 5 (33%)                   | 0     | 5    | 0     |
| E       | 6 (60%)                   | 0     | 0    | 0     |
| overall | 62 (62%)                  | 0     | 12   | 0     |

Latency (ms): mean 5577, p50 4250, max 27187, 39 of 100 over 5000ms. Per
axis: A 3937/2499/27187/3, B 7068/6183/19741/18, C 4380/3662/11782/6, D
8729/6961/19715/10, E 4209/4500/6742/2.

**Determinism.** Two on-pass runs at temperature 0 (`shortlist-on-run1.jsonl`,
`shortlist-on-run2.jsonl`): 100 of 100 rows identical in both `kind` and
`operationId`, zero diffs. Latency mean 5742 ms (run1) vs 5761 ms (run2) -
the two runs differ only in wall-clock, never in outcome. Correct@1 reads
**66** by a naive count (any `form`/`result` row whose `operationId` is
one of the question's answers) in both runs, but **65** by the report's
own rule, `score.ts` - the citable one. The one row the two rules
disagree on is `b01` (`注文を見たい`): `Orchestrator.ask` degrades this
`ask_user`-over-a-safe-operation call to a form (`askDegraded: true`)
whose `operationId`, `listSalesOrders`, happens to already be one of the
question's own answers. `score.ts` correctly counts an `askDegraded` form
as `asked`, never as a pick (`2dcb2e0`'s own rule 1) - a naive rule that
does not know about the degrade double-counts it as a correct guess it
never made. Two such rows exist in the corpus (`b01`, `b06`); only `b01`'s
`operationId` happens to land inside its own answer set, which is why the
naive/citable gap is exactly one row, not two. The earlier runs without a
pinned `temperature` had swung one axis from 32 to 16 correct across two
runs with nothing else changed (`100d61d`) - this pair, at 100/100
identical rows, is the fix confirmed at the scale that matters.

**What the measurement found in the product, and fixed.**

`a97f71e` - `ask_user` required the model to invent a service name, and a
fabricated one became a 500. `AskUserTool`'s schema asked for a free-text
`service` alongside `operationId`; with five services to guess from, the
model fabricated pairs like `approval/createApproval` and even named its
own tool as the operation, `expense/ask_user` - 7 of the first 100
answers 500'd as `endpoint not found in catalogue`, all on the ambiguous
axis (B). `AskUserTool` no longer declares `service` at all;
`toolcall.Planner.decisionFromAskUser` resolves it from `operationId` the
same way a real tool call already does, and `Orchestrator.ask` degrades an
operation id the catalogue does not have to a plain question instead of
an error - an `ask` is never a 500 again.

`100d61d` - no `temperature` meant no determinism. `chat.Request` sent no
`temperature` field at all, so llama-server's own default applied on every
call; the same 100-question fixture scored 32 then 16 on one axis across
two runs with nothing else - not the fixture, not the catalogue, not the
model - changed. `chat.Request.Temperature *float64` is now sent as `0`
(`chat.Zero()`) on every planning call, in both `toolcall.Planner.Plan`
and both of `jsonmode.Planner.complete`'s attempts.

`717820a` - no `max_tokens` meant a repetition loop ran into the client's
own 120s timeout. Reproduced deterministically on `明細を1件確認したい`
(b07): narrowing took 9ms + 88ms, then the chat completion never returned
headers at all - after exactly 120s the platform answered 500 "context
deadline exceeded", and llama-swap's own log read `POST
/v1/chat/completions 499 0 ... 2m0.000s` - the model was still generating
when the client gave up. `chat.Request` sent no `max_tokens`, so
llama-server's unbounded default (`n_predict = -1`) applied; at
temperature 0 with twenty strict tool schemas offered, nothing stopped a
loop, and every planning answer is short enough that an unbounded budget
buys nothing. `chat.Request.MaxTokens *int` is now sent as 1024 on every
planning call; a `finish_reason` of `"length"` is never decoded as a real
answer - it resolves to `DecisionNone`, not an error and not a 500.

**What the measurement found in itself, and fixed.** The numbers reported
before these fixes - 49/60, 62/63, 63/57 - are void and must not be
cited; they were produced by a scorer that could not see most of what the
planner actually did.

`2dcb2e0` - the old scorer only ever counted correct@1 for `kind:
"result"` and dropped everything else as wrong or silent, understating the
planner badly: axis D alone carries 7/15 `form` rows, axis B 8/25, and a
`form` over an unsafe operation is D8's own confirm-before-write working
as designed, not a miss. `run.ts` now records `target.operationId` for a
`form` row and `score.ts` counts it as a real pick, correct@1 when its
target is in the answer key. The same commit found a second undercounting
source at the platform level: `Orchestrator.ask` degrades an `ask_user`
over a _safe_ operation with no enum for its parameter into that same form
shape (e.g. 「注文を見たい」 correctly asking whether 受注 or 発注 was
meant) - a form the scorer needs to count as `asked`, never as a pick.

`8ccece4` - the `catalogue-safety.ts` lookup `2dcb2e0`'s own fix needed
(telling a real D8 form from an ask degraded into the same shape) was
itself dropped from that commit - `git commit -- e2e/shortlist` does not
stage an untracked file, and the pathspec left the new module out. Added
back here, with its own test.

`eafd875` and `0ea7ab8` - the fixture 404'd every invoke, so no `result`
row ever carried a genuine render and no alternatives were ever seen; the
first fix served a real 200 off the operation's own response schema for a
safe `GET`, the second found that the fixture was still matching the
wrong path shape (`/api/<service>/...` where the platform, whose base URL
already embeds the service, actually sends
`/<service>/api/<service>/...`) - every result in the first corrected run
had still come through the `invoke-500` fallback for this reason alone.

`af5aeea` - added `QuestionResult.via` (`"plan"` for a genuine `/api/plan`
200, `"invoke-500"` for the honest fallback) so a run where every result
is `invoke-500` is visibly not measuring the render path instead of
silently passing - this entry's headline cites it directly (67/79 result
rows, both passes, all `via: plan`).

**How the planner misses, against the picker on the same twenty
candidates.** `shortlist-misses.txt` (the on-pass run's 35 misses,
run1) beside what the picker did on the identical K=20 shortlist:

1. **`none` where the picker commits** - 10 of the 35 misses, concentrated
   on axis B (6) and axis D (2), with one each on A and C. Three examples:
   - `b07` 明細を1件確認したい: planner `none`; picker `getExpenseLine`
     (ambiguous); answers `getSalesOrderLine`, `getPurchasingOrderLine`,
     `getExpenseLine`.
   - `b18` 社員を新規登録したい: planner `none`; picker
     `createExpenseEmployee` (certain); answers `createAttendanceEmployee`,
     `createExpenseEmployee`.
   - `d09` お金を返してもらいたい: planner `none`; picker
     `submitSalesReturnOrder` (certain); answers
     `createExpenseReimbursement`, `createExpenseClaim`,
     `createExpenseAdvance`.

2. **`list_capabilities` returned as the answer to a vague question** - 7
   of 100 answers overall: `b11`, `c01`, `c06`, `c19`, `c23`, `d06`,
   `d14`. Three examples:
   - `c01` 在庫を見たい: planner `list_capabilities`; picker
     `listInventoryItems` (certain); answers `listInventoryItems`,
     `listInventoryLots`, `listInventoryAllocations`,
     `listInventoryStockCounts`, `listInventoryAdjustments`.
   - `c06` 受注に関わる書類を確認したい: planner `list_capabilities`;
     picker `getSalesOrder` (certain); answers `listSalesQuotations`,
     `listSalesDeliveryNotes`.
   - `d14` 急いで仕入れたい時の手続きを知りたい: planner
     `list_capabilities`; picker `submitPurchasingOrder` (certain);
     answers `listPurchasingEmergencyOrders`,
     `searchPurchasingEmergencyOrders`.

3. **The wrong operation among near-neighbours, same domain** - this shows
   up mostly on axis C's near-neighbour groups, not axis B (axis B's own
   misses are all `none`/`form`/`ask`, never a wrong `result`): the
   planner commits to a real operation that is simply not one of the
   question's answers. Three examples:
   - `c07` 得意先まわりの情報を確認したい: planner `result`
     `listPurchasingPartners` (alternatives `listPurchasingSupplierScorecards`,
     `getAttendanceQualification`); picker `listSalesPartners` (certain);
     answers `listSalesCustomers`, `listSalesContacts`.
   - `c08` 取引先や与信の情報を知りたい: planner `result`
     `listPurchasingPartners` (alternatives `listPurchasingSuppliers`,
     `listSalesPartners`); picker `listSalesPartners` (ambiguous); answers
     `listSalesPartners`, `listSalesCreditLimits`.
   - `b04` 注文の内容を変えたい: planner `form` `updatePurchasingOrderLine`
     (the line-level operation, not the order-level one asked for); picker
     `updatePurchasingOrder` (certain); answers `updateSalesOrder`,
     `updatePurchasingOrder`.

4. **The product under-asks.** The planner returned `ask` exactly once in
   100 answers (`b13`, 承認を新規登録したい), against the picker, on the
   identical shortlists, flagging axis B `ambiguous` 52% of the time
   (2026-09-15, "Measuring the pick": `pick:e5-large-q8+reranker`, axis B
   flagged 52%) - roughly half of the questions this corpus built to be
   genuinely ambiguous. The planner has somewhere to ask and almost never
   takes it.

5. **Axis D: a `form` on a write the planner has misidentified.** Two
   quoted directly:
   - `d03` 立て替えた分を出したい: planner `form` `updateExpenseAdvance`;
     picker `createInventoryTransfer` (ambiguous); answers
     `createExpenseClaim`, `submitExpenseClaim` - the planner reaches an
     unsafe form, but over the wrong operation and the wrong verb (`update`
     against a nonexistent record, not `create`/`submit`).
   - `d07` 減った分を直したい: planner `form` `updateExpenseLine`; picker
     `deleteInventoryUnitOfMeasure` (ambiguous); answers
     `createInventoryAdjustment`, `submitInventoryAdjustment` - again a
     real, safe-to-render form, over an operation with no relation to the
     inventory adjustment the question asked for.

Where the next work is: the planner's prompt and its tool descriptions -
this corpus now measures them directly, at `make eval-shortlist`, the
same way `docs/plans/narrowing.md` measured retrieval. No fix is proposed
here.

**Alternatives.** 60 of the 100 on-pass rows carried them (up to two,
taken from the shortlist positions after the chosen operation). They add
three points overall (correct@1 65 → correct@shown 68) and lift axis E
from 70% to 90% - the biggest single-axis gain, on the axis where a wrong
first pick is most often a lexical near-miss one click away from the
right one. Every other axis moves by at most one row (C: 14 → 15).

**Latency and H4.** On: mean 5.7s / p50 3.9s; off: mean 5.6s / p50 4.3s;
41 vs 39 questions over 5s. No request paid for a model load - the
persistent llama-swap `groups` block (`71f747c`) held all three models
resident for the whole run; the embedding-plus-rerank stage costs on the
order of 100ms per question, and the rest of every question's latency is
the chat completion itself. With narrowing off, the 1000-tool prompt is
byte-identical on every request, so llama-server's own prefix cache serves
it - this is why the off pass is not slower than the on pass despite
offering the model roughly 27k tokens of tool definitions instead of a
twenty-tool shortlist.

**What this settles, in three sentences.** The product's own planner, not
retrieval, is the bottleneck: 65 against the picker's 83 on identical
input, with the answer sitting in the shortlist 93% of the time. Narrowing
is worth +3 correct@1 and +6 correct@shown (with alternatives) on this
planner, and alternatives - the thing that turns a wrong pick into one
click instead of a round trip - do not exist at all without it. D2's
revision now has its numbers in front of it; that decision belongs to its
owner (AC-H-108).

## 2026-09-15 `eval-shortlist` gains `WORDING=`: the harness change docs/plans/wording.md Task 2 asks for

`make eval-shortlist WORDING=v2-commit,v3-ask-on-collision` now passes
`--wording $(WORDING)` through to `e2e/shortlist/run.ts`; without
`WORDING` the target is byte-identical to before (`$(if $(WORDING),...)`
expands empty). This is the runner half of `docs/plans/wording.md` Task 2,
done in parallel with Task 1's `wording` package in
`services/platform/internal/adapter/planner/wording` - the only thing the
two share is the environment variable name `ORCHESTRA_PLANNER_WORDING`,
agreed in the plan itself, not any code.

`run.ts --wording a,b,c` boots one platform per named wording (narrowing
on, K=20, `ORCHESTRA_PLANNER_WORDING=<name>` in its environment), always
including `v1` first even when not named, and writes each pass's rows to
`out/on-<name>.jsonl` and its miss list to `out/misses-<name>.txt`.
Without `--wording` it is unchanged: `on.jsonl`/`off.jsonl`,
`--on-only`/`--off-only` all still work exactly as before. `report.ts`
gained `renderWordingReport` (one block per wording, `v1` first, the
stand-in picker's row once at the bottom); `print-report.ts` now reads
whichever `on-*.jsonl` files exist, and the plain `on.jsonl`/`off.jsonl`
pair when both are present, instead of assuming exactly the latter.

`make check` calls no model: `misses.test.ts` and the new
`renderWordingReport` tests in `report.test.ts` use fakes only, the same
way `score.test.ts` and the rest of `report.test.ts` already did.
`docker logs llama-swap 2>&1 | grep -c 'POST /v1/'` read 102116 before
this work and 102116 after - no request left this session.

## 2026-09-15 wording: v2-commit becomes the default

`docs/plans/wording.md` Task 3: `make eval-shortlist WORDING=v2-commit,v3-ask-on-collision,v4-commit-and-ask,v5-examples-in-tools`
(narrowing on, K=20) beside `v1`, then `make eval` (18 real-service cases)
for `v1` and the wording that clears the fixture, per Q3/Q4. Every number
below is read straight from the run's own files
(`e2e/shortlist/out/on-*.jsonl`/`misses-*.txt`, `wording-report.txt`), not
copied from an earlier summary - a first pass at this table transposed two
axis columns and overstated `v2-commit`'s own correct@1/correct@shown gain;
recomputing `score()`'s logic by hand against every row before writing this
entry is what caught it.

**The table** (correct@1/correct@shown as % of 100; A/B/C/D/E as % correct@1
within that axis's own count - A/B/C 25 questions each, D 15, E 10; `ask`
is a literal `ask_user` answer, not counting an `ask_user` call that
degraded to an empty form (`askDegraded`, TODO.md item 5) - those are 2
(v1), 5 (v2), 6 (v3), 7 (v4), 2 (v5) and are folded into `none`'s neighbour
column only in the prose below, not this table):

| wording              | correct@1 | correct@shown | ask | none | list_capabilities | A   | B   | C   | D   | E   | p50 ms |
| -------------------- | --------- | ------------- | --- | ---- | ----------------- | --- | --- | --- | --- | --- | ------ |
| v1                   | 65        | 68            | 1   | 10   | 7                 | 92  | 56  | 56  | 47  | 70  | 3934   |
| v2-commit            | 68        | 73            | 0   | 6    | 3                 | 92  | 52  | 68  | 53  | 70  | 4194   |
| v3-ask-on-collision  | 56        | 57            | 0   | 24   | 8                 | 88  | 28  | 48  | 40  | 90  | 5850   |
| v4-commit-and-ask    | 60        | 64            | 0   | 15   | 2                 | 84  | 32  | 64  | 47  | 80  | 5039   |
| v5-examples-in-tools | 60        | 62            | 0   | 20   | 9                 | 88  | 52  | 36  | 53  | 80  | 4193   |

`v1`'s block reproduces the recorded baseline exactly - 65/68 overall, and
`e2e/shortlist/out/on-v1.jsonl` is byte-identical to this run's own
`on-v1.jsonl` (third identical run of 100, `docs/plans/wording.md` Task 3
Step 1's own requirement).

**Three quoted misses per candidate**, from each wording's own
`misses-<name>.txt`:

_v2-commit_ (32 of 100 missed):

- `b18` 社員を新規登録したい: `none`; answers `createAttendanceEmployee`,
  `createExpenseEmployee` - a collision `v2` still does not commit through.
- `c07` 得意先まわりの情報を確認したい: `result` `listPurchasingPartners`
  (wrong service); answers `listSalesCustomers`, `listSalesContacts` - the
  recurring near-neighbour miss `v1` already had, unmoved.
- `d14` 急いで仕入れたい時の手続きを知りたい: `result` `list_capabilities`;
  answers `listPurchasingEmergencyOrders`, `searchPurchasingEmergencyOrders`
  - one of the three `list_capabilities` picks left (`b08`, `d06`, `d14`).

_v3-ask-on-collision_ (44 of 100 missed, the sentence's own target cases):

- `b02` 注文を1件確認したい: `none`; answers `getSalesOrder`,
  `getPurchasingOrder` - exactly a 受注/発注 collision, and the model still
  answers nothing rather than asking.
- `b07` 明細を1件確認したい: `none`; answers `getSalesOrderLine`,
  `getPurchasingOrderLine`, `getExpenseLine`.
- `b12` 承認を1件見たい: `none`; answers `getPurchasingApproval`,
  `getAttendanceApproval`, `getExpenseApproval` - a 勤怠/経費 collision,
  same outcome.

_v4-commit-and-ask_ (40 of 100 missed):

- `b02` 注文を1件確認したい: `none` (v3's collision cost, carried over).
- `b13` 承認を新規登録したい: `none`; answers `createPurchasingApproval`,
  `createAttendanceApproval`, `createExpenseApproval` - this is `v1`'s one
  `ask` case (`docs/plans/wording.md` Task 3's own baseline), and `v4`
  turns it into a miss.
- `d06` 出荷の準備をしたい: `result` `list_capabilities`; answers
  `createInventoryShipment`, `createInventoryPickList`,
  `createInventoryPackingList`.

_v5-examples-in-tools_ (40 of 100 missed):

- `a02` シリアル番号ってどうなってる？: `result`
  `getInventoryBarcodeFormatSetting`; answers `listInventorySerialNumbers`
  - under `v1`/`v2` this same question misses to the much closer
    `getInventorySerialNumber`; with examples shown, it misses further.
- `c07` 得意先まわりの情報を確認したい: `result` `listPurchasingPartners`
  (same recurring miss as `v1`/`v2`); answers `listSalesCustomers`,
  `listSalesContacts`.
- `c21` 経費申請にまつわる書類を確認したい: `result` `list_capabilities`;
  answers `listExpenseReceipts`, `listExpenseTravelExpenses` - axis C alone
  carries eight `list_capabilities` picks under `v5` (`c11`, `c14`, `c16`,
  `c17`, `c20`, `c21`, `c23`, `c24`), against `v1`'s two.

**`v2-commit` clears `v1`.** +3 correct@1 (65→68), +5 with alternatives
(68→73); `none` 10→6, `list_capabilities` 7→3 (`b08`, `d06`, `d14` left of
the seven `v1` missed on: `b11`, `c01`, `c06`, `c19`, `c23`, `d06`, `d14`).
By axis: C 56→68 (+12) and D 47→53 (+6) both move, matching the sentence's
own target (§4: "targets `none` and `list_capabilities`"); A and E are
unchanged (92/70 both wordings). B is the one axis that does _not_ move
the way the sentence's own doc comment implies it should: it is not part
of `v2SystemPromptAddition`'s target, and it reads 56→52 - a one-row drop
(14/25→13/25), inside the kind of noise a 25-question axis carries, not a
cost the sentence caused (`v2SystemPromptAddition` touches nothing that
mentions ambiguity or B's own collisions). Latency: p50 3934ms→4194ms
(+260ms, +6.6%) - the added sentence costs some tokens on every request,
as expected, and buys the correctness above.

**`v3-ask-on-collision` is a negative result.** correct@1 65→56 (-9),
`none` 10→24 (+14, +140%) - `none` very nearly triples on the exact axis
it targets (B: 6→11 of 25 `none`). Literal `ask_user` calls (`kind: "ask"`)
go from 1 to 0 - the model never once reaches the tool the sentence names
by name - while `ask_user` calls that _did_ fire but degraded into an
empty form (TODO.md item 5's own defect: an `ask_user` naming a safe
operation with no enum for its parameter renders as a blank form, not a
question) rise only from 2 to 6, nowhere near enough to explain the jump
in `none`. Read plainly: telling this model "when two operations collide,
do not guess - ask" did not make it ask (`b02`, `b07`, `b12` above, all
`none`, all textbook 受注/発注 or 勤怠/経費 collisions) - it read as
permission to refuse rather than an instruction to reach for a specific
tool. `v4-commit-and-ask` carries the same cost on top of `v2`'s own gain:
correct@1 65→60 (-5 net against `v1`, -8 against `v2`), `none` 10→15,
and `b13` - `v1`'s one genuine `ask` - turns into a `none` once `v3`'s
sentence is added (quoted above). `v4` keeps `v2`'s axis-C gain (56→64)
but gives back five of `v2`'s six points overall (68→60) and most of `v2`'s
B stability besides (52→32, though B was never `v2`'s gain to begin with -
`v3`'s sentence is the one that costs it, in both `v3` and `v4`).

**`v5-examples-in-tools` is a negative result.** correct@1 65→60 (-5), and
the cost sits entirely on axis C: 56→36 (-20 of 25, the single largest
per-axis move any candidate produces in either direction). B, D and E are
unchanged or improved (D 47→53, matching `v2`'s own D gain since `v5` is
`v1` everywhere except `CatalogueTool`, and D's answer key is exactly the
vocabulary gap the written examples were built to close - `docs/specs/
describing.md`); A drops 92→88 (one question, `a02`, quoted above: shown
examples, the model reaches for an unrelated barcode-format operation
instead of the close near-miss it picked under `v1`/`v2`). This is the
same effect `DECISIONS.md`, 2026-09-15 ("Letting the reranker read the
written examples") already recorded for the stand-in picker: giving it
the written examples lowered its own correct rate (83%→78%→77%) even
though the same examples raised recall. Axis C here is where a catalogue
tool's own vocabulary already separates near-neighbours cleanly by summary
alone (`listSalesPartners` vs `listPurchasingPartners`, `getSalesOrder` vs
`getPurchasingOrder`); adding hand-written example _questions_ to each
tool's description apparently gives the model more surface to match the
wrong tool against, not less. Examples belong to retrieval and reranking,
where they are measured to help, not to what the planner itself reads.

**Q4, the real-service check (`make eval`, 18 cases, `qwen3.5-9b-q8`).**
Run under `v1` and under `v2-commit` (the only wording either fixture
table clears): every one of the 18 cases produced byte-identical
accept/reject counts between the two wordings. Sixteen cases read exactly
as their own recorded baseline (10/10 or the equivalent accept). Two read
worse than their baseline, identically under both wordings:

- `no-enum-value` (破損した在庫はある？): 30/30 reject, baseline 18/30
  reject.
- `no-enum-value-attendance` (有給の勤怠はある？): 10/10 reject, baseline
  7/10 reject.

Both are TODO.md item 4's own open defect (a filter word that matches no
enum value gets every row back, silently) - already open before this
subproject, and unmoved by anything `v2-commit`'s own sentence changed
(the wording is identical for both runs). The likely cause is not the
wording at all: `100d61d`, landed earlier the same day, pinned
`temperature: 0` for deterministic planning. A baseline of 18/30 reject
(60%) is exactly the shape of a case whose correct answer was reached only
some of the time under llama-server's previous unpinned-temperature
default - pinning the temperature does not add new failures, it removes
the randomness that used to let this case land on its right path some
fraction of the time, and the one path a temperature-0 decode now always
takes for this prompt happens to be the wrong one. This is not accepted as
a new baseline - `make eval-accept` was not run, and a regression is not
closed by re-recording it as normal - but it is not this subproject's
regression to fix either: identical under `v1` and `v2-commit` means the
wording did not cause it. TODO.md item 4 carries the note and the
`100d61d` link.

**Decision (Q5).** `v2-commit` becomes `wording.Default()`: it is the only
candidate that clears `v1` on the fixture (+3 correct@1, +5 correct@shown,
`none` and `list_capabilities` both down, no axis or latency cost beyond
the expected token overhead) and it does not regress the real-service
corpus relative to `v1` - the two open `no-enum-value` cases move
identically under both, so this is not `v2-commit` trading a fixture point
for a real-service one, which Q4 exists to catch. `v3`/`v4`'s
ask-on-collision sentence and `v5`'s in-tool examples are both negative
results on this planner and are not adopted; they stay in the `wording`
package, named and selectable, as the record of what was tried.
`services/platform/internal/adapter/planner/wording.Default()` now returns
`v2Commit()`; `v1` stays in the package as the baseline every candidate -
and this decision - was measured against, still asserted byte-identical to
the `5bf5cf8` literals, now by name (`wording.ByName("v1")`) rather than
via `Default()`. `internal/infra/config.parsePlannerWording` and
`pkg/app.resolveWording` both already read `wording.Default().Name`/
`wording.Default()` for an unset `ORCHESTRA_PLANNER_WORDING`, so no change
was needed there beyond the switch itself - `make dev-services` and the
built product both pick up `v2-commit` with the variable unset.
`wording/v2_commit.go`'s own doc comment is corrected along the way: it
said `list_capabilities` was picked "five times in 100" under `v1`
(quoting an earlier draft of `docs/specs/wording.md` §1); the recorded
count is seven (`b11`, `c01`, `c06`, `c19`, `c23`, `d06`, `d14`,
`DECISIONS.md`, 2026-09-15, "The product's planner, measured end to end").

`make check` itself calls no model: `docker logs llama-swap 2>&1 | grep -c
'POST /v1/'` read 104248 immediately before this task's own `make check`
and 104251 immediately after - a drift of 3 that also showed up between
two idle readings taken minutes apart with no `make` target running at
all (104245→104248), so it is the standing `make dev-services` platform
on `:8080` (running since 11:55, started before and independent of this
task) taking its own traffic, not `make check`.
