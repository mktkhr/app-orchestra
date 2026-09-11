import { columnTitle, type Fields } from "./rows";

/** One property/column, ready to render: its key and its display label. */
export interface FieldEntry {
  readonly key: string;
  readonly label: string;
}

/**
 * Turns a list of property keys into `{key, label}` pairs, labelled through
 * `columnTitle`.
 *
 * `ResultDetail` (one object's own properties) and `ResultForm` (a `kind:
 * "form"` schema's properties) both need exactly this: the ordered list of
 * keys to render, each with the label a person should see instead of the
 * raw key. Pulling it out here is what keeps the two components from
 * repeating it (see `harness/quality/duplication.txt`).
 */
export function fieldEntries(
  fields: Fields | undefined,
  keys: readonly string[],
): readonly FieldEntry[] {
  return keys.map((key) => ({ key, label: columnTitle(fields, key) }));
}
