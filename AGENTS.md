# AGENTS.md

You are working in a Repository Harness. The rules are few because the
toolchain enforces the rest.

1. Work until the acceptance criteria in PRODUCT.md are met. Done means
   `make check` is green, not "the code is written". `make check` runs every
   quality gate there is; `make fmt` fixes formatting; there is nothing else
   to remember.
2. The harness is not yours to change. Do not weaken, disable, bypass or
   reconfigure anything under `harness/quality/`, `harness/githooks/`, `harness/guard/`,
   `.github/`, the `Makefile` or the root `vite.config.ts`. `--no-verify`,
   `//nolint`, `oxlint-disable`, `@ts-ignore` and friends are not fixes.
3. A failing check is information about the code. Fix the code.
4. The contract comes first: change `services/platform/api/openapi.yaml`, run `make generate`,
   then implement against the generated code. Never edit generated files.
5. Keep the project memory current before you stop: `STATE.md` (what works),
   `TODO.md` (what is next), `DECISIONS.md` (why). Git plus these files is
   the only memory that survives a session.
6. Use MUI components directly, where the interface is. Build something in
   `src/shared/ui` only when MUI has no component for the job, and say which
   one you looked for. A wrapper around a MUI component adds a name and
   removes the theme.
7. Commit the work, not just the notes about it. One coherent change per
   commit: the code, its tests and the state files it makes true, together.
   `git add` the sources you wrote; a file left untracked is a file the next
   session cannot recover.

Commands: `make fmt`, `make generate`, `make check`. Read `README.md` once;
everything else is enforced.
