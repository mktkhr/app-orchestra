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
