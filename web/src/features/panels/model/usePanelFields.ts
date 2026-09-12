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
function canSavePanel(
  entry: CatalogEntry | null,
  title: string,
  component: Component,
  category: string,
  value: string,
  transformEnabled: boolean,
  groupBy: string,
  aggregate: Aggregate,
  aggregateField: string,
): boolean {
  if (entry === null || title.trim() === "") {
    return false;
  }

  if (component === "chart" && (category === "" || value === "")) {
    return false;
  }

  return !transformEnabled || (groupBy !== "" && (aggregate === "count" || aggregateField !== ""));
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
  readonly canSave: () => boolean;
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

  const setComponent = guardedSetter(
    (candidate): candidate is Component => componentOptions.some((option) => option === candidate),
    setComponentState,
  );
  const setKind = guardedSetter(isChartKind, setKindState);
  const setAggregate = guardedSetter(isAggregate, setAggregateState);

  const canSave = (): boolean =>
    canSavePanel(
      entry,
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
    canSave,
  };
}
