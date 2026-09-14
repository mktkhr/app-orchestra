/**
 * The written and "both" layers (spec section 3, section 7): the written
 * layer is `FixtureOperation.examples` - already on every operation, no
 * generation or cache involved - and "both" is the union of it with the
 * generated layer, read by `measure.ts` to build the four new rows.
 */
import type { FixtureOperation } from "../fixture/index.ts";
import type { OperationUtterances } from "./cache.ts";

/** The written layer: `FixtureOperation.examples`, keyed by `operationId` (spec section 3, G6). */
export function writtenUtterancesOf(catalog: readonly FixtureOperation[]): OperationUtterances {
  return new Map(catalog.map((operation) => [operation.operationId, operation.examples]));
}

/** The union of two utterance layers, operation by operation - generated utterances first, then written. */
export function mergeUtterances(
  a: OperationUtterances,
  b: OperationUtterances,
): OperationUtterances {
  const operationIds = new Set([...a.keys(), ...b.keys()]);
  const merged = new Map<string, readonly string[]>();

  for (const operationId of operationIds) {
    merged.set(operationId, [...(a.get(operationId) ?? []), ...(b.get(operationId) ?? [])]);
  }

  return merged;
}
