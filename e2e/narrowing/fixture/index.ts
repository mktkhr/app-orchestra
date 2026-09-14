/**
 * The fixture's public surface: the five services, the operation count a
 * service should hold, and the flattened list of exposed operations a
 * narrowing mechanism (Task 3) or the corpus (Task 4) reads.
 */
import { attendance } from "./attendance.ts";
import { expense } from "./expense.ts";
import { inventory } from "./inventory.ts";
import { toOpenAPI } from "./openapi.ts";
import { purchasing } from "./purchasing.ts";
import { sales } from "./sales.ts";
import type { ServiceFixture } from "./types.ts";

export function services(): readonly ServiceFixture[] {
  return [inventory, sales, purchasing, attendance, expense];
}

/** 150 CRUD + 20 aggregates + 20 settings + 10 workflow actions. */
export function operationCount(service: ServiceFixture): number {
  const crud = service.resources.reduce((n, r) => n + r.verbs.length, 0);
  const workflow = service.workflows.reduce((n, w) => n + w.actions.length, 0);

  return crud + service.aggregates.length + service.settings.length + workflow;
}

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
      });
    }
  }

  return found;
}

/** The exposed operations of the first `size` services: 200, 400, 600 or 1000. */
export function catalogOf(size: 1 | 2 | 3 | 5): readonly FixtureOperation[] {
  return services()
    .slice(0, size)
    .flatMap((service) => operationsOf(service));
}
