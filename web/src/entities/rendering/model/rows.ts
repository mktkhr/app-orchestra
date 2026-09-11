import type { PlanResult } from "@/shared/api/client";

/** One row of a table result: an object with unknown-typed properties. */
export type Row = Readonly<Record<string, unknown>>;

/**
 * Picks the rows out of a `kind: "result"` / `component: "table"` payload.
 *
 * `/api/plan`'s `data` is always a JSON object (never a bare array — see the
 * generated `PlanResult.data` type), and every table response observed from
 * the running services wraps its rows in an envelope: inventory returns
 * `{items: [...]}`, attendance returns `{records: [...]}`. Rather than
 * hardcode either property name, this takes the envelope's single
 * array-valued property, mirroring `soleArrayProperty` in
 * services/platform/internal/domain/rendering.go — the same judgment the
 * platform already made once to decide this is a table at all.
 *
 * Returns an empty array when the envelope holds no array property, or more
 * than one (not a shape this function can call unambiguous).
 */
export function rowsFromData(data: NonNullable<PlanResult["data"]>): readonly Row[] {
  const arrayValues = Object.values(data).filter((value): value is unknown[] =>
    Array.isArray(value),
  );

  if (arrayValues.length !== 1) {
    return [];
  }

  const [sole] = arrayValues;

  if (sole === undefined) {
    return [];
  }

  return sole.filter((value): value is Row => isRow(value));
}

function isRow(value: unknown): value is Row {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** Renders one cell's value as text. */
export function formatCellValue(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }

  if (typeof value === "string") {
    return value;
  }

  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }

  return JSON.stringify(value);
}
