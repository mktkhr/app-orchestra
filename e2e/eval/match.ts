import type { ExpectedOutcome, PlanOutcome } from "./types.ts";

/** True when actual has every key/value expected declares (a subset check, not equality). */
function hasArgs(
  actual: Record<string, unknown> | undefined,
  expected: Record<string, unknown>,
): boolean {
  const present = actual ?? {};

  if (Object.keys(expected).length === 0) {
    // args: {} in a case means "no arguments at all" (docs/specs/eval.md
    // section 3's no-matching-value reject) - not "any arguments are fine".
    return Object.keys(present).length === 0;
  }

  return Object.entries(expected).every(([key, value]) => present[key] === value);
}

/**
 * True when actual is the outcome expected describes. Only the fields
 * expected names are checked - a case only writes down what it cares about
 * (docs/specs/eval.md section 3) - and which of actual's fields are read
 * depends on kind exactly the way `PlanResult` itself does:
 * `result` reads `source`, `form` reads `target`/`initial`, `ask` reads
 * `param` (the wire format carries no service/operationId for `ask` -
 * `services/platform/internal/usecase/orchestrator.go`'s `ask` never sets
 * them), `none` reads nothing further.
 */
export function matches(actual: PlanOutcome, expected: ExpectedOutcome): boolean {
  if (actual.kind !== expected.kind) return false;

  switch (expected.kind) {
    case "result": {
      const source = actual.source;

      if (expected.service !== undefined && source?.service !== expected.service) return false;
      if (expected.operationId !== undefined && source?.operationId !== expected.operationId) {
        return false;
      }

      if (expected.args !== undefined && !hasArgs(source?.args, expected.args)) return false;

      return true;
    }
    case "form": {
      const target = actual.target;

      if (expected.service !== undefined && target?.service !== expected.service) return false;
      if (expected.operationId !== undefined && target?.operationId !== expected.operationId) {
        return false;
      }

      if (expected.args !== undefined && !hasArgs(actual.initial, expected.args)) return false;

      return true;
    }
    case "ask":
      return expected.param === undefined || actual.param === expected.param;
    case "none":
      return true;
    default:
      // Unreachable: expected.kind is one of the four literals above.
      return false;
  }
}

/** True when any of outcomes matches actual - a case's accept/reject lists are "any of these". */
export function matchesAny(actual: PlanOutcome, outcomes: readonly ExpectedOutcome[]): boolean {
  return outcomes.some((outcome) => matches(actual, outcome));
}
