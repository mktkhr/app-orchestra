# TODO.md — prioritised work

_Keep three lists. Move items, do not duplicate them._

## In progress

_None._

## Next

1. Two harness gaps the orchestration design exposed
   (`docs/specs/orchestration.md` section 13):
   - a guard that fails when a generated file is missing an operation id the
     spec declares. `guard-generated` only checks freshness, so oapi-codegen
     silently dropping a `query` operation passes it today;
   - a Redocly rule that fails a spec where an `enum` carries no
     `x-enum-labels`, since the model depends on those labels to map Japanese
     to the enum value.
2. Build the first vertical slice against `PRODUCT.md` section 5.
3. Move to TypeScript 7 once `openapi-typescript` supports it. Everything else
   in the repository already passes under 7; only code generation does not.
   orval was measured as a replacement and rejected - it runs under TypeScript 7
   but emits the wrong shape for this product (`DECISIONS.md`, 2026-09-11).

## Done

- Imported the repository harness from takamai at HEAD, renamed every
  identifier, and removed all product code (`DECISIONS.md`, 2026-09-10).
- Rebuilt `DECISIONS.md`: kept the eleven entries that justify the harness,
  dropped the ten that describe a product this repository does not have.
- Rewrote `PRODUCT.md`, `STATE.md` and `TODO.md` for app-orchestra, and fixed
  the documentation drift inherited from takamai.
- Made `.gitignore` deny by default.
- Added the Claude Code layer: `CLAUDE.md`, the permission allowlist and the
  after-edit hook.
- Reorganised the repository into `harness/`, `services/<name>/`, `web/` and
  `e2e/` (`DECISIONS.md`, 2026-09-10), and removed the NO_COLOR machinery
  (`.env`, `.npmrc`, and the `postinstall` rewrite of `node_modules/.bin/vp`).
- Made every target, guard, hook and CI job discover services instead of naming
  one, and verified it against a temporary second service.
- Upgraded the toolchain: Go 1.27.1, pnpm 12.3.4 and the whole catalog. Found
  that golangci-lint must be rebuilt by the Go it analyses, and tied that to
  `toolchain.mk`. TypeScript 7 was tried and reverted (`DECISIONS.md`).
- Designed the first vertical slice (`docs/specs/orchestration.md`) and wrote
  its acceptance criteria into `PRODUCT.md`.
