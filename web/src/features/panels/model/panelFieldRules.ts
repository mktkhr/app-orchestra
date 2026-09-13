import type { Dispatch, SetStateAction } from "react";

import {
  AGGREGATE_LABELS,
  columnTitle,
  type Aggregate,
  type ChartKind,
} from "@/entities/rendering";
import type { Component } from "@/shared/api/client";
import type { CatalogEntry } from "@/shared/api/catalog";

export const DEFAULT_KIND: ChartKind = "bar";
export const DEFAULT_AGGREGATE: Aggregate = "count";

const CHART_KINDS: readonly ChartKind[] = ["bar", "line", "pie"];
const AGGREGATES: readonly Aggregate[] = ["count", "sum", "avg"];

export function isChartKind(value: string): value is ChartKind {
  return CHART_KINDS.some((kind) => kind === value);
}

export function isAggregate(value: string): value is Aggregate {
  return AGGREGATES.some((aggregate) => aggregate === value);
}

/** Wraps a `useState` setter so it only ever accepts a value `isValid` accepts - a controlled select's `onChange` hands over a bare `string`, and this is what keeps it from ever landing something outside its own enum. */
export function guardedSetter<T extends string>(
  isValid: (value: string) => value is T,
  setState: Dispatch<SetStateAction<T>>,
): (value: string) => void {
  return (value: string) => {
    if (isValid(value)) {
      setState(value);
    }
  };
}

/**
 * One field a chart's axes or a transform's own pickers may name: the
 * property name itself, which is what a panel posts (`value`), paired
 * with what a person reads for it (`label`) - the same `title` a table
 * already draws for the same property (`columnTitle`,
 * `entities/rendering/model/rows.ts`), falling back to the property name
 * when the contract declares none.
 */
export interface FieldOption {
  readonly value: string;
  readonly label: string;
}

/** The fields a catalogue entry describes, as a chart's axes or a transform's `groupBy` may - the keys alone, in declared order, each with its display label. */
export function fieldOptionsFor(entry: CatalogEntry | null): readonly FieldOption[] {
  if (entry?.fields === undefined) {
    return [];
  }

  const fields = entry.fields;

  return Object.keys(fields).map((key) => ({ value: key, label: columnTitle(fields, key) }));
}

/**
 * What a chart's category/value axes may actually name. Ordinarily the same
 * as `fieldOptionsFor` - a chart draws the rows as they came. But
 * `applyTransform` (`entities/rendering/lib/transform.ts`) replaces every
 * row with exactly two keys - `transform.groupBy` itself and the
 * aggregate's own name (`count`/`sum`/`avg`) - and `PanelResult` runs that
 * transform before choosing what to draw (`docs/plans/dashboard.md` Task
 * 5). A chart built on top of a transform has to name axes that exist in
 * that output, not in the raw response, or every mark it draws is "not a
 * number" and gets skipped (`ResultChart`'s own rule) - a chart that never
 * draws is this task's own journey failing silently, not passing.
 */
export function chartFieldOptionsFor(
  fieldOptions: readonly FieldOption[],
  transformEnabled: boolean,
  groupBy: string,
  aggregate: Aggregate,
): readonly FieldOption[] {
  if (!transformEnabled) {
    return fieldOptions;
  }

  const aggregateOption: FieldOption = { value: aggregate, label: AGGREGATE_LABELS[aggregate] };

  if (groupBy === "") {
    return [aggregateOption];
  }

  const groupByOption =
    fieldOptions.find((option) => option.value === groupBy) ??
    ({ value: groupBy, label: groupBy } satisfies FieldOption);

  return [groupByOption, aggregateOption];
}

/** Every `Component` a person may pick for `entry`: the rule's own answer, plus `chart` whenever the entry describes fields to draw one from (P2). */
export function componentOptionsFor(entry: CatalogEntry | null): readonly Component[] {
  if (entry === null) {
    return [];
  }

  if (entry.component === "chart" || fieldOptionsFor(entry).length === 0) {
    return [entry.component];
  }

  return [entry.component, "chart"];
}

/**
 * What is still missing before this panel can be saved, as a sentence to
 * show the person, or null when nothing is. It names the first gap rather
 * than every one: a form that answers one question at a time is read, and
 * a list of five complaints is not.
 *
 * It never has to say "pick an operation": `AddPanelForm` draws nothing
 * past the picker until one is chosen, so the save control does not exist
 * to be pressed before then. A branch for it would be one no person can
 * reach.
 *
 * A reason rather than a boolean because the save control is never
 * disabled at rest - `make guard-layout` rejects a disabled `contained`
 * button, which has no edge it can see - so pressing it with an incomplete
 * form has to say something. Returning silently would make the button look
 * broken, which is the same defect as a filter dropped without a word.
 */
export function missingBeforeSave(
  title: string,
  component: Component,
  category: string,
  value: string,
  transformEnabled: boolean,
  groupBy: string,
  aggregate: Aggregate,
  aggregateField: string,
): string | null {
  if (title.trim() === "") {
    return "パネル名を入力してください。";
  }

  if (component === "chart" && (category === "" || value === "")) {
    return "グラフの分類と値にするフィールドを選んでください。";
  }

  if (transformEnabled && groupBy === "") {
    return "グループ化するフィールドを選んでください。";
  }

  if (transformEnabled && aggregate !== "count" && aggregateField === "") {
    return "集計するフィールドを選んでください。";
  }

  return null;
}
