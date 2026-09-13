import type { Aggregate, ChartKind } from "@/entities/rendering";
import type { Component, View } from "@/shared/api/client";

function requiredKeysOf(schema: Record<string, unknown>): ReadonlySet<string> {
  const required = schema["required"];

  return new Set(Array.isArray(required) ? required.filter((key) => typeof key === "string") : []);
}

/**
 * Drops an optional argument `useFormValues` seeded to `""` - its
 * type-appropriate default for a control nobody touched
 * (`entities/rendering/model/useFormValues.ts`'s own `seedValue`), not a
 * value the person chose. Left in, an untouched optional enum parameter
 * (`ListInventoryItems`'s own `status`, say) is posted as `status: ""` and
 * every later invocation of the panel fails `usecase.validateEnumArg` with
 * "" is not a valid value - the panel a person just built never draws at
 * all. A required field's own `""` is left alone: that is a real gap the
 * save button's own `missingBeforeSave` should catch before this ever
 * runs, not something to paper over here.
 *
 * Shared by `usePanelBuilder` (create) and `usePanelEditor` (edit): both
 * post the same `argsValues` shape through the same schema-driven rule.
 */
export function compactArgs(
  schema: Record<string, unknown>,
  values: Record<string, unknown>,
): Record<string, unknown> {
  const required = requiredKeysOf(schema);
  const compacted: Record<string, unknown> = {};

  for (const [key, value] of Object.entries(values)) {
    if (value === "" && !required.has(key)) {
      continue;
    }

    compacted[key] = value;
  }

  return compacted;
}

/**
 * Assembles a panel's `view` from the builder's own chart/transform state,
 * or `undefined` when neither applies - shared by `usePanelBuilder`
 * (create, where an absent `view` means "this panel has none") and
 * `usePanelEditor` (edit, where the caller turns that same `undefined`
 * into an explicit `null` - "remove the view" - since the edit form is the
 * one place that always fully restates it; see `usePanelEditor`'s own
 * comment on section 6a's wrinkle).
 */
export function buildView(
  component: Component,
  category: string,
  value: string,
  kind: ChartKind,
  transformEnabled: boolean,
  groupBy: string,
  aggregate: Aggregate,
  aggregateField: string,
): View | undefined {
  const chart = component === "chart" ? { category, value, kind } : undefined;
  const transform = transformEnabled
    ? { groupBy, aggregate, ...(aggregate === "count" ? {} : { field: aggregateField }) }
    : undefined;

  if (chart === undefined && transform === undefined) {
    return undefined;
  }

  return {
    ...(chart === undefined ? {} : { chart }),
    ...(transform === undefined ? {} : { transform }),
  };
}
