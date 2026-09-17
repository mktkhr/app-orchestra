/**
 * The fixture's public surface: the five services, the operation count a
 * service should hold, and the flattened list of exposed operations a
 * narrowing mechanism (Task 3) or the corpus (Task 4) reads.
 */
import { attendance } from "./attendance.ts";
import { expense } from "./expense.ts";
import { inventory } from "./inventory.ts";
import { operationsOf } from "./operations.ts";
import type { FixtureOperation } from "./operations.ts";
import { purchasing } from "./purchasing.ts";
import { sales } from "./sales.ts";
import type { ServiceFixture } from "./types.ts";

export { midCatalog, midServices } from "./mid.ts";
export { operationsOf } from "./operations.ts";
export type { FixtureOperation } from "./operations.ts";

export function services(): readonly ServiceFixture[] {
  return [inventory, sales, purchasing, attendance, expense];
}

/** 150 CRUD + 20 aggregates + 20 settings + 10 workflow actions. */
export function operationCount(service: ServiceFixture): number {
  const crud = service.resources.reduce((n, r) => n + r.verbs.length, 0);
  const workflow = service.workflows.reduce((n, w) => n + w.actions.length, 0);

  return crud + service.aggregates.length + service.settings.length + workflow;
}

/** The exposed operations of the first `size` services: 200, 400, 600 or 1000. */
export function catalogOf(size: 1 | 2 | 3 | 5): readonly FixtureOperation[] {
  return services()
    .slice(0, size)
    .flatMap((service) => operationsOf(service));
}
