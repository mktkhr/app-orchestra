/**
 * A panel's one allowed transformation step: group a result's rows by one
 * field, then reduce each group to a single aggregate. `docs/specs/dashboard.md`
 * section 4 is the design this implements — one step, not a pipeline, so
 * this module does nothing beyond grouping and aggregating: no filtering,
 * no renaming, no second pass over its own output.
 *
 * Pure and dependency-free by design (`docs/plans/dashboard.md` Task 0): a
 * saved panel (`pages/workspace/ui/PanelResult.tsx`) and a panel builder's
 * preview both call this over the same rows a table would otherwise render,
 * so its only contract is this file's tests.
 */

export type Aggregate = "count" | "sum" | "avg";

/**
 * Each aggregate's Japanese label - the one place that knows what
 * `applyTransform`'s own output keys (`count`/`sum`/`avg`) mean, since
 * they name no property any contract describes and so carry no `title` of
 * their own (`entities/rendering/model/rows.ts`'s `columnTitle` has
 * nothing to look up for them). A panel builder's chart axes read this
 * once a transform is on (`features/panels/model/usePanelFields.ts`).
 */
export const AGGREGATE_LABELS: Readonly<Record<Aggregate, string>> = {
  count: "件数",
  sum: "合計",
  avg: "平均",
};

export interface Transform {
  readonly groupBy: string;
  readonly aggregate: Aggregate;
  readonly field?: string;
}

/**
 * Stands in for every row whose `groupBy` value is missing, `null`, or not
 * a string. Those rows are not dropped — a dashboard that silently excludes
 * rows draws a total nobody asked for — they are grouped together, under a
 * bucket whose own `groupBy` value in the output is `null` rather than an
 * attempt to preserve whatever heterogeneous underlying value they had (a
 * missing property and an explicit `null` and a number are all "no usable
 * group value", not three different values worth telling apart).
 */
const UNGROUPED = Symbol("ungrouped");

interface GroupAccumulator {
  /** The value this group's rows share for `groupBy` — `null` for `UNGROUPED`. */
  readonly groupValue: string | null;
  count: number;
  /** Numeric, finite values seen under `transform.field` in this group. */
  values: number[];
}

/**
 * Groups `rows` by `transform.groupBy` and reduces each group to
 * `transform.aggregate`. Rows in, rows out — the output's keys are
 * `transform.groupBy` itself and the aggregate's own name (`count`, `sum`,
 * `avg`), so a chart's `category`/`value` can name fields that exist on it.
 *
 * Decisions pinned by this function's tests (`transform.test.ts`):
 * - A row whose `groupBy` value is missing, `null`, or not a string is
 *   grouped under a shared bucket (`groupValue: null`), never dropped.
 * - Under `sum`/`avg`, a non-numeric (or non-finite) value at `field` is
 *   skipped for that row rather than treated as zero or turning the whole
 *   group's result into `NaN`.
 * - `sum` of a group with no valid values is `0` (the identity, and what a
 *   chart should draw for "nothing to add"); `avg` of one is `null` (there
 *   is no honest number to report, and `0` would claim there was).
 * - `sum`/`avg` with `transform.field` absent behaves exactly as if every
 *   row's value were non-numeric: every group gets `sum: 0` / `avg: null`.
 * - Output order is first-seen order of the group key across `rows`.
 */
export function applyTransform(
  rows: readonly Record<string, unknown>[],
  transform: Transform,
): Record<string, unknown>[] {
  const { groupBy, aggregate, field } = transform;

  // A Map iterates in insertion order, which is exactly the first-seen
  // order of the group key this function's tests pin - so the order needs
  // no second structure to record it, and no lookup that could miss.
  const groups = new Map<string | typeof UNGROUPED, GroupAccumulator>();

  for (const row of rows) {
    const rawGroupValue = row[groupBy];
    const isUsableGroupValue = typeof rawGroupValue === "string";
    const key = isUsableGroupValue ? rawGroupValue : UNGROUPED;

    let group = groups.get(key);

    if (group === undefined) {
      group = { groupValue: isUsableGroupValue ? rawGroupValue : null, count: 0, values: [] };
      groups.set(key, group);
    }

    group.count += 1;

    if (aggregate !== "count" && field !== undefined) {
      const value = row[field];

      if (typeof value === "number" && Number.isFinite(value)) {
        group.values.push(value);
      }
    }
  }

  return [...groups.values()].map((group) => {
    const outputRow: Record<string, unknown> = { [groupBy]: group.groupValue };

    switch (aggregate) {
      case "count":
        outputRow["count"] = group.count;
        break;
      case "sum":
        outputRow["sum"] = group.values.reduce((total, value) => total + value, 0);
        break;
      case "avg":
        outputRow["avg"] =
          group.values.length === 0
            ? null
            : group.values.reduce((total, value) => total + value, 0) / group.values.length;
        break;
    }

    return outputRow;
  });
}
