# harness/quality/ — the Repository Harness policy

Everything in this directory is policy, not product. It is protected by
`harness/guard/protected-paths.sh`: changing it is a deliberate, separately
acknowledged change (see the root `README.md`, "Changing the harness").

| File                  | Enforced by                                               | What it decides                                                                                                                                                                          |
| --------------------- | --------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `go/golangci.yml`     | `make services-lint`, `make services-fmt`                 | Go linters (explicit list, all errors), formatters, `depguard` package denials, `forbidigo` bans (`panic`, `os.Exit`, `log.Fatal`, `time.Sleep`, `fmt.Print*`), `nolintlint` discipline. |
| `oxlint/policy.ts`    | `make web-lint` (`vp check`)                              | Oxlint rules, type aware via tsgolint. Bans `any`, unsafe assertions, `@ts-*` comments, `console`, `eval`, default exports, direct `fetch`/storage outside `src/shared`.                 |
| `oxfmt/policy.ts`     | `make web-fmt`                                            | Formatting of TS/JS/JSON/CSS/MD. Go is formatted by gofmt/goimports/gci.                                                                                                                 |
| `architecture.json`   | `make guard-arch`, `make guard-fsd`                       | One layer order applied to every service, plus the web Feature-Sliced Design layers.                                                                                                     |
| `suppressions.allow`  | `make guard-suppressions`                                 | The registry of every allowed suppression directive (`//nolint`, `oxlint-disable`, `@ts-expect-error`, ...). Unregistered ones fail.                                                     |
| `protected-paths.txt` | `make guard-protected`, `harness/githooks/pre-commit`, CI | The list of harness paths.                                                                                                                                                               |
| `coverage.txt`        | `make guard-coverage`                                     | Minimum statement coverage per Go package; `default 90`, with a per-package floor where a package is pure wiring.                                                                        |
| `file-length.txt`     | `make guard-filelen`                                      | Maximum lines in a hand-written source file (1000), and the generated files exempt from it.                                                                                              |
| `duplication.txt`     | `make guard-duplication`                                  | How long a shared node-type sequence must be before two frontend functions count as duplicates.                                                                                          |
| `ui-primitives.txt`   | `make guard-ui`                                           | Which HTML controls may never be written as JSX (or reached via `component=`), so every control comes from MUI.                                                                          |
| `commit-types.txt`    | `harness/githooks/commit-msg`, CI                         | The Conventional Commit types a subject line may use.                                                                                                                                    |
| `browser/`            | `make guard-a11y`, `make guard-layout`                    | The WCAG 2.2 AA audit and the measured layout invariants (target size, visible boundary, no sideways scroll), both run in light and dark.                                                |
| `toolchain.mk`        | `Makefile`, CI                                            | Pinned tool versions outside `go.mod` / `pnpm`.                                                                                                                                          |

Related, outside this directory: `.claude/settings.json` and
`harness/claude/post-edit.sh` (the Claude Code layer - a permission allowlist and
an after-edit report; nothing depends on it, see `CLAUDE.md`), the root
`vite.config.ts` (imports the policies), `services/platform/api/oapi-codegen.yaml` and `redocly.yaml` (code generation and
contract lint), `harness/guard/` (the guard implementations),
`harness/quiet.sh` (the one-line-on-success output contract of every check),
`harness/githooks/` and `.github/workflows/ci.yml` (where the checks run).

## Deliberately disabled rules

Every rule that is off has a comment next to it saying why. The short list:

- Oxlint `typescript/prefer-readonly-parameter-types`: rejects every React and DOM type.
- Oxlint `react/react-in-jsx-scope`: the automatic JSX runtime needs no import.
- Oxlint `eslint/no-await-in-loop`: sequential awaits are a design choice.
- Oxlint `eslint/max-lines-per-function` in test files: suites are flat lists.
- golangci-lint `govet.fieldalignment`: readability beats struct packing.
- Redocly `operation-4xx-response`: system endpoints have no 4XX outcome.

## Changing a policy

1. Make it a separate change that touches only the harness.
2. Explain the why in `DECISIONS.md`.
3. Commit with `ORCHESTRA_ALLOW_HARNESS_CHANGE=1`; label the pull request `harness`.
