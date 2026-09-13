import type { Operation, Permission } from "@/shared/api/users";

/** One service's operations, grouped for the grid (docs/specs/auth.md section 4). */
export interface PermissionGroup {
  readonly service: string;
  /**
   * The service's own name for a person to read - every operation of one
   * service carries the same `serviceDisplayName` (it comes from the
   * service's own contract, not the operation's), so the group just reads
   * it off the first one. `ServiceCard`'s header reads this, never
   * `service` (DECISIONS.md, 2026-09-13).
   */
  readonly serviceDisplayName: string;
  readonly operations: readonly Operation[];
}

/** The key a permission grid's granted map is indexed by: a service and an operation id never collide across services. */
export function keyOf(permission: Pick<Permission, "service" | "operationId">): string {
  return `${permission.service} ${permission.operationId}`;
}

/** Groups operations by service, in the order each service is first seen. */
export function groupByService(operations: readonly Operation[]): readonly PermissionGroup[] {
  const bucket = new Map<string, Operation[]>();
  const order: string[] = [];

  for (const operation of operations) {
    let list = bucket.get(operation.service);

    if (list === undefined) {
      list = [];
      bucket.set(operation.service, list);
      order.push(operation.service);
    }

    list.push(operation);
  }

  return order.map((service) => {
    const serviceOperations = bucket.get(service) ?? [];

    return {
      service,
      serviceDisplayName: serviceOperations[0]?.serviceDisplayName ?? service,
      operations: serviceOperations,
    };
  });
}
