/**
 * Parsing an HTTP response body's `unknown` JSON into a known shape, by
 * runtime check rather than an unsafe type assertion - shared by every
 * process-level e2e suite that needs to read a field back off a response
 * (no "dom" lib is configured for this Node-only package, so
 * `response.json()` returns `unknown`).
 */

/** True when value is a JSON object - not an array, not null. */
export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/** value narrowed to a record, or {} when it was not one. */
export function asRecord(value: unknown): Record<string, unknown> {
  return isRecord(value) ? value : {};
}
