# STATE.md — current implementation state

_Last updated: 2026-09-10_

## Summary

**The repository holds the harness and nothing else.** There is no product
code: no `services/platform/api/openapi.yaml`, no `services/platform/internal`, no `web/src`, no
acceptance tests. `make check` fails, and is expected to.

## What exists

The full repository harness, imported from takamai and renamed (see
`DECISIONS.md`, 2026-09-10):

- `harness/` — the whole harness in one directory: `quality/` (policy),
  `guard/` (implementations), `githooks/`, `claude/`, `gen/` and `quiet.sh`.
  One line in `protected-paths.txt` covers all of it.
- `harness/quality/` — every policy file: golangci-lint (explicit linter list, all
  errors), Oxlint and Oxfmt policies, Feature-Sliced Design and backend layer
  declarations, the coverage floor (90%), the file-length limit (1000), the
  MUI-only rule for controls, the structural duplication threshold, the
  protected-path list, the allowed commit types, and the browser guards (WCAG
  2.2 AA, measured layout).
- `harness/githooks/` — `pre-commit` (staged files, seconds), `commit-msg` (Conventional
  Commits plus the guard against committing notes without the work),
  `pre-push` (full `make check`).
- The Claude Code layer — `CLAUDE.md` (imports `AGENTS.md`, does not restate it),
  `.claude/settings.json` (permission allowlist; `--no-verify` denied), and
  `harness/claude/post-edit.sh` (reports after an edit only when the changed file
  is unformatted or is harness). Verified firing. Nothing depends on it.
- `.gitignore` — deny by default: only the file types and named files it allows
  can be committed.
- `harness/guard/` — nine guard implementations; `harness/quiet.sh` — the
  one-line-on-success output contract every target goes through.
- `Makefile`, `.github/workflows/ci.yml`, `services/platform/api/oapi-codegen.yaml`,
  `redocly.yaml`, the Vite+ root config, the TypeScript configs, and the
  package/module manifests of every workspace.

## What does not exist

- Product code of any kind. `services/platform/` holds only `go.mod` and
  `go.sum`, and no contract; `web/` only its manifests and `index.html`; the
  acceptance trees only their manifests.
- Nothing further on the multi-service side: every target, guard, hook and CI
  job discovers services rather than naming them (`DECISIONS.md`, 2026-09-10),
  verified by adding a second service temporarily and watching each guard report
  on it.

## Known gaps in the harness

- **`make check` cannot pass with no product.** The guards have nothing to
  measure and the acceptance suites are empty.
- **TypeScript is held at 6.0.3 by a dependency, not by choice.**
  `openapi-typescript` builds its output with the TypeScript Compiler API, which
  TypeScript 7's native implementation does not provide, so `make generate` fails
  under 7. Everything else - including type-aware Oxlint - passed under 7. Go and
  pnpm are current (`DECISIONS.md`, 2026-09-10).
  Documentation drift inherited from takamai has been fixed: `README.md`'s
  principle count and product description, `harness/quality/README.md`'s policy table (six
  policies were missing) and `docs/architecture.md`'s reference to slices that no
  longer exist.
