# STATE.md — current implementation state

_Last updated: 2026-09-11_

## Summary

**The harness is complete and the first vertical slice is under construction.**
Five of the seventeen tasks in `docs/plans/orchestration.md` are done: the
platform's scaffold, two services that answer real requests, the rendering rule,
and the catalogue the platform builds by asking those services what they offer.
`make check` is green.

## What works

**Two services, each its own Go module, each a whole contract-first stack.**
`services/inventory` serves stock items on port 8081 and `services/attendance`
serves attendance records on port 8082. Both expose four operations - list with
an enum filter, create, get by id, and `GET /openapi.yaml` - over eight fixture
rows apiece, and both carry an enum whose values a Japanese speaker cannot guess
from the English (`quarantined` = 検品保留, `substitute` = 振替休日). Each labels
every enum value with `x-enum-labels`; `make api-lint` fails a spec that does
not.

Each service serves its own contract from the spec oapi-codegen embedded in its
generated code, so the document it serves and the code that serves it cannot
drift. The embedding normalises operation ids to PascalCase and changes nothing
else, which is deliberate (`DECISIONS.md`, 2026-09-11).

**The platform's domain**, `services/platform/internal/domain`, imports nothing
outside the standard library and is fully covered. It holds the catalogue types
and `Render`, the pure rule that picks a component from a response schema: an
explicit `x-ui-hint` wins, then a request body means `form`, an array of objects
means `table`, an object means `detail`.

**The catalogue**, `services/platform/internal/adapter/specsource/http` behind
the `usecase.SpecSource` port. It reads `ORCHESTRA_SERVICES` (`name=url` pairs),
fetches each service's `/openapi.yaml`, and converts every operation into a
`domain.Endpoint`, carrying `x-enum-labels` through to `Schema.EnumLabels` -
the only route by which the Japanese label for an enum value reaches the model.
A service that cannot be reached fails the whole fetch rather than yielding a
partial catalogue. Verified against both services running: eight endpoints, both
enums labelled, every component the rule picks correct.

**The harness**, imported from takamai and renamed (`DECISIONS.md`,
2026-09-10). `harness/` holds all of it - `quality/` (policy), `guard/`
(implementations), `githooks/`, `claude/`, `gen/` and `quiet.sh`. Every target,
guard, hook and CI job discovers services rather than naming them: a service is
a directory under `services/` with a `go.mod`, and nothing declares the list.

Two guards were added for this design: `guard-generated-ops` fails when a
generated file is missing an operation id its spec declares (oapi-codegen drops
OpenAPI 3.2 `query` operations silently), and a Redocly plugin fails a spec
whose `enum` carries no `x-enum-labels`.

## What does not exist yet

Tasks 5 to 16 of `docs/plans/orchestration.md`: the tool definitions the
catalogue becomes, `/api/plan` and `/api/invoke`, `ask_user`, the two planner
adapters, and the whole of `web/src` beyond the shell. `make check` is green
because the guards report on the code that is there, not because the product is
finished.

## Known gaps in the harness

- **TypeScript is held at 6.0.3 by a dependency, not by choice.**
  `openapi-typescript` builds its output with the TypeScript Compiler API, which
  TypeScript 7's native implementation does not provide, so `make generate` fails
  under 7. Everything else - including type-aware Oxlint - passed under 7. Go and
  pnpm are current (`DECISIONS.md`, 2026-09-10).
