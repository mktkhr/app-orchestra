# app-orchestra

A **Repository Harness**: a Go + React monorepo whose toolchain, not its
prose, decides what code is acceptable. It exists so that an autonomous coding
agent can work in it for days with no human in the loop and still be unable to
merge a bad implementation.

There is no product in it yet. The harness was imported whole and every line
of the sample product it arrived with was removed, so what remains is the
policy, the guards and the machinery that runs them. `make check` therefore
fails: the guards have nothing to measure. See `STATE.md` and `PRODUCT.md`.

## Principles

- **Guards, not rules.** `AGENTS.md` has seven principles. Style, safety,
  architecture and suppression discipline are static checks that fail the build.
- **Errors, not warnings.** Every enabled rule is an error, locally and in CI.
- **Contract first.** `services/platform/api/openapi.yaml` is the source of truth. The Go server
  interface, request validation and the TypeScript client are generated from it.
- **The harness is protected.** Quality settings, guards, hooks and CI live in
  their own directories and cannot change as a side effect of product work.
- **Memory is Git plus Markdown.** `PRODUCT.md`, `STATE.md`, `TODO.md` and
  `DECISIONS.md` are how a session restores context, not chat history.

## Layout

```
harness/            everything that decides what code is acceptable
  quality/          policy: lint and format configs, architecture, coverage, protected paths
  guard/            guard implementations (archcheck, fsd, suppressions, ...)
  githooks/         git hooks (pre-commit, commit-msg, pre-push), used as core.hooksPath
  claude/           the Claude Code PostToolUse hook
  gen/              Go module that pins the code generator (go tool oapi-codegen)
  quiet.sh          the one-line-on-success output contract every check goes through
services/           one directory per backend service, each its own Go module
  platform/         auth, authorisation, the service catalogue, LLM orchestration
    api/            this service's OpenAPI contract and its generator config
    acceptance/     this service's acceptance tests (a separate module by design)
web/                the frontend: Vite+ React, Feature-Sliced Design under src/
  acceptance/       application level tests of the frontend
e2e/                acceptance tests that span processes, and belong to no one service
docs/               architecture and acceptance notes
.claude/            Claude Code settings: the permission allowlist and the hook
.github/            CI workflow
Makefile            the only entry point you need
vite.config.ts      root Vite+ config: imports the policies from harness/quality/
AGENTS.md CLAUDE.md PRODUCT.md STATE.md TODO.md DECISIONS.md   agent principles and project memory
```

## Getting started

Prerequisites: Go 1.26+, Node 24+, pnpm 10 (`corepack enable` or the
`packageManager` field), `make`, `git`.

```sh
make setup      # installs pinned Go tools into .tools/, pnpm dependencies, git hooks
make check      # the full verification
```

`make check` does not pass today, and is not supposed to: there is no product
for the guards to measure and no acceptance suite to run. `make service-run SERVICE=platform` and
`make web-dev` have nothing to serve for the same reason. Both become
meaningful with the first product code.

Everything is run from the repository root through `make`. `make help` lists
every target.

Output is terse on purpose: every step prints `ok: <step>` on success and
`FAIL: <step>` followed by the diagnostics on failure, nothing else
(`harness/quiet.sh`). Set `ORCHESTRA_VERBOSE=1` to see the raw tool output.

## Commands

| Target                           | What it does                                                                                                                                                                        |
| -------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `make fmt`                       | Format everything in place: Go (`gofmt`, `goimports`, `gci` via `golangci-lint fmt`) and web (`oxfmt` via `vp fmt`).                                                                |
| `make fmt-check`                 | Same, but fail instead of writing.                                                                                                                                                  |
| `make lint`                      | `redocly lint` on the contract, `go vet` + `golangci-lint` on every Go module, `vp check --no-fmt` (type-aware Oxlint + TypeScript type check) on the workspace, then `make guard`. |
| `make test`                      | `go test -race` for the backend, Vitest for the frontend, tests of the guards themselves.                                                                                           |
| `make build`                     | `services/platform/bin/api` (static binary) and `web/dist`.                                                                                                                         |
| `make check`                     | **Every quality gate**: `fmt-check` + `lint` + `test` + `build` + `acceptance` (HTTP and browser). The one command an agent needs; `pre-push` and CI run the same.                  |
| `make acceptance`                | `build`, then the acceptance suites. Part of `make check`.                                                                                                                          |
| `make generate`                  | Regenerate Go and TypeScript code from `services/platform/api/openapi.yaml`.                                                                                                        |
| `make guard`                     | `guard-generated` (generated code is current), `guard-arch`, `guard-fsd`, `guard-suppressions`.                                                                                     |
| `make services-*` / `make web-*` | The per-side variants: `fmt`, `fmt-check`, `lint`, `test`, `build`, plus `service-run SERVICE=<name>`, `web-dev`, `web-typecheck`. `make services` lists the services.              |
| `make hooks`                     | Point `core.hooksPath` at `harness/githooks/` (also done by `pnpm install`).                                                                                                        |
| `make tools`                     | Install the pinned `golangci-lint` into `.tools/bin`.                                                                                                                               |

## Quality gates

### Backend

`harness/quality/go/golangci.yml` enables an explicit list of linters (no presets, so
an upgrade never changes the policy silently): `errcheck`, `errorlint`,
`exhaustive`, `gocritic`, `govet` (all analyzers), `ineffassign`, `nilerr`,
`nilnil`, `revive`, `staticcheck`, `unused`, `gosec`, `depguard`,
`forbidigo`, `testifylint`, `wrapcheck`, `err113`, `mnd`, `funlen`, ... All
findings are errors. `forbidigo` bans `panic`, `os.Exit` (outside `cmd/`),
`log.Fatal`, `fmt.Print*`, `time.Sleep`. `depguard` keeps `net/http`,
`encoding/json`, `log` and outer layers out of `domain` and `usecase`.

`//nolint` must name a linter, carry an explanation (`nolintlint`) and be
registered in `harness/quality/suppressions.allow` (`make guard-suppressions`).

### Frontend

Vite+ is the whole toolchain: `vp fmt` (Oxfmt), `vp check` (Oxlint with
`typeAware` + `typeCheck` through tsgolint), `vp test` (Vitest), `vp build`.
The policy in `harness/quality/oxlint/policy.ts` turns on the `correctness`,
`suspicious`, `perf` and `pedantic` categories as errors and bans `any`,
unsafe type assertions, non-null assertions, `@ts-ignore` / `@ts-expect-error`,
`console`, `eval`, default exports, floating promises, and the globals `fetch`,
`XMLHttpRequest`, `localStorage`, `sessionStorage` outside `src/shared`.
`oxlint-disable` / `eslint-disable` comments are caught by the suppression guard.

The compiler policy (`tsconfig.base.json`) is `strict` plus
`noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`,
`noPropertyAccessFromIndexSignature`, `erasableSyntaxOnly` and friends.

### Contract

`services/platform/api/openapi.yaml` is linted by Redocly (`recommended-strict` plus the rules in
`redocly.yaml`). `make generate` produces
`services/platform/internal/adapter/openapi/openapi.gen.go` (models, strict server
interface, embedded spec) and `web/src/shared/api/gen/schema.d.ts`
(types). Both are committed; `make guard-generated` fails when they are stale.
Requests are validated against the spec at runtime before any handler runs.

### Architecture

`harness/quality/architecture.json` declares the layers. `make guard-arch` (Go, reads
`go list`) rejects any backend import that points outwards. `make guard-fsd`
(TypeScript, reads imports with oxc-parser) rejects upward imports, sibling
slice imports and deep imports that bypass a slice's `index.ts`. See
`docs/architecture.md`.

## Git hooks

`harness/githooks/` is the repository's `core.hooksPath` (set by `make hooks` or
`pnpm install`).

- **pre-commit** (seconds): harness protection on the staged files, `gofmt` and
  `go vet` for the packages of staged Go files, `vp staged` (format + lint +
  type check of the staged web files), suppression guard.
- **pre-push** (a few minutes): `make check`, acceptance and browser tests included.

`--no-verify` is not a fix: CI runs the identical checks.

## Claude Code

`.claude/settings.json` lets `make`, the read-only `go` and `git` subcommands and
the file-reading shell commands run without a prompt, and denies
`git commit --no-verify` and `git push --no-verify` outright.

`harness/claude/post-edit.sh` runs after every Edit or Write and reports only when
the file that changed is unformatted or belongs to the harness. Silence is the
normal outcome.

Nothing depends on this layer - `harness/githooks/pre-commit`, `make check` and CI make every
one of these checks again. It only moves the answer earlier. See `CLAUDE.md`.

## CI

`.github/workflows/ci.yml`, on push to `main` and on pull requests:

| Job                             | Steps                                                                                                                      |
| ------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| `harness protection` (PRs only) | `harness/guard/protected-paths.sh` against the base branch; passes only with the `harness` label if harness files changed. |
| `services`                      | `services-fmt-check`, `services-lint`, `services-test`, `services-build`; uploads every `services/*/bin/api` as one tar.   |
| `web`                           | `web-fmt-check`, `web-lint`, `web-test`, `web-build`; uploads `web/dist`.                                                  |
| `guards`                        | `api-lint`, `guard` (generated, arch, fsd, suppressions), `guard-test`.                                                    |
| `acceptance`                    | Downloads both artifacts, runs `acceptance-services`, `acceptance-web`, `acceptance-e2e`. Browser E2E plugs in here.       |

Tool versions are pinned in `harness/quality/toolchain.mk`, `harness/gen/go.mod` and
`pnpm-workspace.yaml` (catalog); the workflow mirrors them.

## Changing the harness

Anything listed in `harness/quality/protected-paths.txt` is the harness. Changing it
is a separate change: explain it in `DECISIONS.md`, commit with
`ORCHESTRA_ALLOW_HARNESS_CHANGE=1`, label the pull request `harness`. Product
changes that touch these files fail the pre-commit hook and CI.

## How an agent is expected to use this repository

1. Read `AGENTS.md`, `PRODUCT.md`, `STATE.md`, `TODO.md`. That is the whole context.
2. Change `services/platform/api/openapi.yaml`, run `make generate`, implement against the
   generated interface and client. Guards and tests say when it is wrong.
3. Commit small; the pre-commit hook is fast. Push at milestones; `pre-push`
   runs `make check`.
4. Before stopping, update `STATE.md`, `TODO.md`, `DECISIONS.md`.
5. Done means `make check` is green and every acceptance criterion in
   `PRODUCT.md` has a passing test.
