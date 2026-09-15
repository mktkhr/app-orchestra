import { services } from "../narrowing/fixture/index.ts";
import { toOpenAPI } from "../narrowing/fixture/openapi.ts";

/**
 * Which of the fixture's operation ids are safe (`GET`), the same test
 * `domain.Endpoint.IsSafe()` applies on the platform side
 * (services/platform/internal/domain/catalog.go: GET/HEAD/QUERY only -
 * the fixture only ever emits GET for a safe operation, see
 * e2e/narrowing/fixture/openapi.ts's own path builders).
 *
 * Needed to score a `kind: "form"` /api/plan response correctly
 * (docs/plans/shortlisting.md Task 4): `Orchestrator.ask` degrades an
 * `ask_user` over a safe operation with no enum for its parameter into
 * that same form shape a real unsafe-operation confirmation uses
 * (services/platform/internal/usecase/orchestrator.go's own `ask`,
 * around its `optionsForParam` check) - so a form naming a safe
 * operation is the planner asking, not a bad pick, and a form naming an
 * unsafe one is a real D8 confirm-before-write choice.
 */
export function safeOperationIds(): ReadonlySet<string> {
  const ids = new Set<string>();

  for (const service of services()) {
    for (const item of Object.values(toOpenAPI(service).paths)) {
      if (item.get !== undefined) ids.add(item.get.operationId);
    }
  }

  return ids;
}
