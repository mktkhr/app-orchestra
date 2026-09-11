import type { PlanResult } from "@/shared/api/client";

/** One row of a table result: an object with unknown-typed properties. */
export type Row = Readonly<Record<string, unknown>>;

/**
 * A `kind: "result"` response's `fields`: per-column JSON Schema, keyed by
 * column name. `/api/plan`'s generated type carries each field as
 * `unknown` (`additionalProperties: true`), so reading `title` or
 * `enumLabels` out of one needs the runtime narrowing below rather than a
 * hand-written type — see `services/platform/api/openapi.yaml`'s
 * `PlanResult.fields` and `services/platform/internal/usecase/tools.go`'s
 * `schemaToJSONSchema` for what actually populates it.
 */
export type Fields = NonNullable<PlanResult["fields"]>;

/**
 * Narrows an `unknown` value to a plain object whose properties are
 * themselves `unknown` — a type guard, not an assertion, so reading a
 * property off the result never needs an unsafe `as` cast.
 */
function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/** Looks up one column's schema out of `fields`, when there is one. */
function fieldSchema(
  fields: Fields | undefined,
  column: string,
): Record<string, unknown> | undefined {
  const field = fields?.[column];

  return isRecord(field) ? field : undefined;
}

/**
 * A column's display title: `fields[column].title` when the schema carries
 * one, otherwise the column key itself. `title` reaches here only when the
 * originating OpenAPI property declares one — confirmed against the
 * running inventory and attendance contracts that neither does today (see
 * DECISIONS.md), so this falls back to the key for every column that exists
 * right now.
 */
export function columnTitle(fields: Fields | undefined, column: string): string {
  const title = fieldSchema(fields, column)?.["title"];

  return typeof title === "string" && title !== "" ? title : column;
}

/**
 * A cell's Japanese label: `fields[column].enumLabels[value]` when the
 * column is an enum and the value has a label there, otherwise the value
 * formatted as plain text (`formatCellValue`). This is what keeps a status
 * such as `quarantined` from ever reaching the screen as itself.
 */
export function cellText(fields: Fields | undefined, column: string, value: unknown): string {
  if (typeof value === "string") {
    const enumLabels = fieldSchema(fields, column)?.["enumLabels"];

    if (isRecord(enumLabels)) {
      const label = enumLabels[value];

      if (typeof label === "string" && label !== "") {
        return label;
      }
    }
  }

  return formatCellValue(value);
}

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
