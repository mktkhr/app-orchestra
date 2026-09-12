import { useState, type Dispatch, type SetStateAction } from "react";

import type { Aggregate, ChartKind } from "@/entities/rendering";
import type { Component } from "@/shared/api/client";
import type { CatalogEntry } from "@/shared/api/catalog";

const DEFAULT_KIND: ChartKind = "bar";
const DEFAULT_AGGREGATE: Aggregate = "count";

const CHART_KINDS: readonly ChartKind[] = ["bar", "line", "pie"];
const AGGREGATES: readonly Aggregate[] = ["count", "sum", "avg"];

function isChartKind(value: string): value is ChartKind {
  return CHART_KINDS.some((kind) => kind === value);
}

function isAggregate(value: string): value is Aggregate {
  return AGGREGATES.some((aggregate) => aggregate === value);
}

/** Wraps a `useState` setter so it only ever accepts a value `isValid` accepts - a controlled select's `onChange` hands over a bare `string`, and this is what keeps it from ever landing something outside its own enum. */
function guardedSetter<T extends string>(
  isValid: (value: string) => value is T,
  setState: Dispatch<SetStateAction<T>>,
): (value: string) => void {
  return (value: string) => {
    if (isValid(value)) {
      setState(value);
    }
  };
}

/** The fields a catalogue entry describes, as a chart's axes or a transform's `groupBy` may - the keys alone, in declared order. */
export function fieldOptionsFor(entry: CatalogEntry | null): readonly string[] {
  return entry?.fields === undefined ? [] : Object.keys(entry.fields);
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
function chartFieldOptionsFor(
  fieldOptions: readonly string[],
  transformEnabled: boolean,
  groupBy: string,
  aggregate: Aggregate,
): readonly string[] {
  if (!transformEnabled) {
    return fieldOptions;
  }

  return groupBy === "" ? [aggregate] : [groupBy, aggregate];
}

/** Every `Component` a person may pick for `entry`: the rule's own answer, plus `chart` whenever the entry describes fields to draw one from (P2). */
function componentOptionsFor(entry: CatalogEntry | null): readonly Component[] {
  if (entry === null) {
    return [];
  }

  if (entry.component === "chart" || fieldOptionsFor(entry).length === 0) {
    return [entry.component];
  }

  return [entry.component, "chart"];
}

/** Whether the current choice is complete enough to save: an operation and a name always, plus a chart's axes or a transform's own fields whenever those apply. */
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
function missingBeforeSave(
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

export interface PanelFields {
  readonly entry: CatalogEntry | null;
  readonly selectEntry: (entry: CatalogEntry | null) => void;
  readonly argsValues: Record<string, unknown>;
  readonly setArgsValues: Dispatch<SetStateAction<Record<string, unknown>>>;
  readonly component: Component;
  readonly setComponent: (value: string) => void;
  readonly componentOptions: readonly Component[];
  readonly fieldOptions: readonly string[];
  /** What a chart's axes may name - `fieldOptions` itself, or the transform's own output shape once one is enabled (see `chartFieldOptionsFor`). */
  readonly chartFieldOptions: readonly string[];
  readonly category: string;
  readonly setCategory: (value: string) => void;
  readonly value: string;
  readonly setValue: (value: string) => void;
  readonly kind: ChartKind;
  readonly setKind: (value: string) => void;
  readonly transformEnabled: boolean;
  readonly setTransformEnabled: (value: boolean) => void;
  readonly groupBy: string;
  readonly setGroupBy: (value: string) => void;
  readonly aggregate: Aggregate;
  readonly setAggregate: (value: string) => void;
  readonly aggregateField: string;
  readonly setAggregateField: (value: string) => void;
  readonly title: string;
  readonly setTitle: (value: string) => void;
  /** What still has to be filled in, as a sentence, or null when nothing does. */
  readonly missingBeforeSave: () => string | null;
}

/**
 * Steps 2-5's own state (`docs/specs/dashboard.md` section 6): the chosen
 * operation, its arguments, which component draws it, a chart's axes, an
 * optional transform, and the panel's name - split out of `usePanelBuilder`
 * so that hook stays under `max-lines-per-function`. Picking a different
 * operation (`selectEntry`) resets everything after it (P8: there is no
 * question behind any of this, so there is nothing to carry over).
 */
export function usePanelFields(): PanelFields {
  const [entry, setEntry] = useState<CatalogEntry | null>(null);
  const [argsValues, setArgsValues] = useState<Record<string, unknown>>({});
  const [component, setComponentState] = useState<Component>("table");
  const [category, setCategory] = useState("");
  const [value, setValue] = useState("");
  const [kind, setKindState] = useState<ChartKind>(DEFAULT_KIND);
  const [transformEnabled, setTransformEnabled] = useState(false);
  const [groupBy, setGroupBy] = useState("");
  const [aggregate, setAggregateState] = useState<Aggregate>(DEFAULT_AGGREGATE);
  const [aggregateField, setAggregateField] = useState("");
  const [title, setTitle] = useState("");

  const selectEntry = (next: CatalogEntry | null): void => {
    setEntry(next);
    setArgsValues({});
    setComponentState(next?.component ?? "table");
    setTitle(next?.summary ?? "");

    const chartDefault = next?.view?.chart;

    setCategory(chartDefault?.category ?? "");
    setValue(chartDefault?.value ?? "");
    setKindState(chartDefault?.kind ?? DEFAULT_KIND);
    setTransformEnabled(false);
    setGroupBy("");
    setAggregateState(DEFAULT_AGGREGATE);
    setAggregateField("");
  };

  const componentOptions = componentOptionsFor(entry);
  const fieldOptions = fieldOptionsFor(entry);
  const chartFieldOptions = chartFieldOptionsFor(
    fieldOptions,
    transformEnabled,
    groupBy,
    aggregate,
  );

  const setComponent = guardedSetter(
    (candidate): candidate is Component => componentOptions.some((option) => option === candidate),
    setComponentState,
  );
  const setKind = guardedSetter(isChartKind, setKindState);
  const setAggregate = guardedSetter(isAggregate, setAggregateState);

  const missing = (): string | null =>
    missingBeforeSave(
      title,
      component,
      category,
      value,
      transformEnabled,
      groupBy,
      aggregate,
      aggregateField,
    );

  return {
    entry,
    selectEntry,
    argsValues,
    setArgsValues,
    component,
    setComponent,
    componentOptions,
    fieldOptions,
    chartFieldOptions,
    category,
    setCategory,
    value,
    setValue,
    kind,
    setKind,
    transformEnabled,
    setTransformEnabled,
    groupBy,
    setGroupBy,
    aggregate,
    setAggregate,
    aggregateField,
    setAggregateField,
    title,
    setTitle,
    missingBeforeSave: missing,
  };
}
