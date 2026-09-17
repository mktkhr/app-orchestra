/**
 * Flattening a `ServiceFixture`'s synthesised OpenAPI document into its
 * exposed operations. Split out of `index.ts` so `mid.ts` - a subset built
 * from the same services (docs/specs/midsizing.md M1) - can read it without
 * an import cycle back through `index.ts`'s barrel export.
 */
import { toOpenAPI } from "./openapi.ts";
import type { ServiceFixture } from "./types.ts";

/** One exposed operation, flattened out of a synthesised OpenAPI document. */
export interface FixtureOperation {
  readonly operationId: string;
  readonly service: string;
  readonly serviceDisplayName: string;
  readonly summary: string;
  readonly description: string;
  readonly displayName: string;
  /**
   * Whether this operation was generated from a service's `settings` array
   * (`openapi.ts`'s `settingPath`, tagged `"settings"`) rather than from a
   * `Resource`, `Aggregate` or `Workflow`. Structural, not a guess from the
   * operationId's spelling — the corpus (Task 4) uses it to tell a real
   * axis-E decoy (a settings operation) from an answer that must not be one.
   */
  readonly isSetting: boolean;
  /**
   * The operation's `x-orchestra-examples`, from the definition table's
   * `examples` field (docs/specs/describing.md, section 3) - empty, never
   * undefined, when the entry carries none.
   */
  readonly examples: readonly string[];
}

/** Every exposed operation a service's contract carries, read back off the generator. */
export function operationsOf(service: ServiceFixture): readonly FixtureOperation[] {
  const doc = toOpenAPI(service);
  const found: FixtureOperation[] = [];

  for (const item of Object.values(doc.paths)) {
    for (const op of [item.get, item.post, item.put, item.delete]) {
      if (op === undefined || !op["x-orchestra-expose"]) continue;

      found.push({
        operationId: op.operationId,
        service: service.name,
        serviceDisplayName: service.displayName,
        summary: op.summary,
        description: op.description,
        displayName: op["x-ui-hint"].displayName,
        isSetting: op.tags.includes("settings"),
        examples: op["x-orchestra-examples"] ?? [],
      });
    }
  }

  return found;
}
