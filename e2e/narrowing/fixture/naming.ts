/** Small, shared naming helpers so the five service files stay data, not logic. */

export const ALL_VERBS = ["list", "get", "create", "update", "delete"] as const;

/** English plural of a PascalCase noun id. Good enough for ids and paths. */
export function pluralOf(id: string): string {
  if (/[^aeiou]y$/iu.test(id)) return `${id.slice(0, -1)}ies`;

  return `${id}s`;
}

/** PascalCase to kebab-case, for URL paths: "OrderLine" -> "order-line". */
export function kebabOf(id: string): string {
  return id
    .replaceAll(/([a-z0-9])([A-Z])/gu, "$1-$2")
    .replaceAll(/([A-Z]+)([A-Z][a-z])/gu, "$1-$2")
    .toLowerCase();
}

/** "inventory" -> "Inventory", for building operationId. */
export function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

/**
 * Joins a service name and a resource id into the `<Service><Resource>` half
 * of an operationId, without repeating a service name the resource already
 * carries: the attendance service's `AttendanceRecord` becomes
 * `AttendanceRecords`, not `AttendanceAttendanceRecords`.
 *
 * A real system names `listAttendanceRecords`, which is also the id
 * docs/specs/narrowing.md section 4 uses in its worked example. The stutter
 * was cosmetic for the platform but not for the fixture: an id that repeats
 * the service name is not what the thing being imitated looks like.
 */
export function qualify(service: string, idPart: string): string {
  const prefix = capitalize(service);

  return idPart.startsWith(prefix) ? idPart : `${prefix}${idPart}`;
}
