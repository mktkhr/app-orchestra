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
