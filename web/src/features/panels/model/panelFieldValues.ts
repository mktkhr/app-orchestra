import type { Dispatch, SetStateAction } from "react";

import type { Aggregate, ChartKind } from "@/entities/rendering";
import type { Component, View } from "@/shared/api/client";
import type { CatalogEntry } from "@/shared/api/catalog";

import {
  chartFieldOptionsFor,
  componentOptionsFor,
  DEFAULT_AGGREGATE,
  DEFAULT_KIND,
  fieldOptionsFor,
  guardedSetter,
  isAggregate,
  isChartKind,
  missingBeforeSave,
  type FieldOption,
} from "./panelFieldRules";

/**
 * What `usePanelFields` seeds itself from, instead of the empty builder
 * defaults - the panel-edit form opened over an existing panel (P12), as
 * against the create form's own empty start. `entry` is the catalogue's
 * own entry for the panel's (fixed, P13) operation, not the panel itself -
 * `usePanelFields` needs its `schema` and `fields` the same way the create
 * form does, and a seeded form never changes it (see `PanelFields.entry`'s
 * own callers, none of which offer `selectEntry` in edit mode).
 */
export interface PanelFieldsSeed {
  readonly entry: CatalogEntry;
  readonly title: string;
  readonly args: Record<string, unknown>;
  readonly component: Component;
  readonly view?: View;
}

/** The plain field values `usePanelFields` derives, whichever way it started. */
export interface PanelFieldValues {
  readonly entry: CatalogEntry | null;
  readonly argsValues: Record<string, unknown>;
  readonly component: Component;
  readonly category: string;
  readonly value: string;
  readonly kind: ChartKind;
  readonly transformEnabled: boolean;
  readonly groupBy: string;
  readonly aggregate: Aggregate;
  readonly aggregateField: string;
  readonly title: string;
}

/** The plain builder-default values `usePanelFields` starts from with no seed, and `selectEntry(null)` resets to. */
export function emptyFieldValues(): PanelFieldValues {
  return {
    entry: null,
    argsValues: {},
    component: "table",
    category: "",
    value: "",
    kind: DEFAULT_KIND,
    transformEnabled: false,
    groupBy: "",
    aggregate: DEFAULT_AGGREGATE,
    aggregateField: "",
    title: "",
  };
}

/** `usePanelFields`'s initial state: the empty builder defaults, or, when `seed` is given, that seed's own fields (the edit form's own start, P12). */
export function initialFieldValues(seed: PanelFieldsSeed | undefined): PanelFieldValues {
  if (seed === undefined) {
    return emptyFieldValues();
  }

  const chart = seed.view?.chart;
  const transform = seed.view?.transform;

  return {
    entry: seed.entry,
    argsValues: seed.args,
    component: seed.component,
    category: chart?.category ?? "",
    value: chart?.value ?? "",
    kind: chart?.kind ?? DEFAULT_KIND,
    transformEnabled: transform !== undefined,
    groupBy: transform?.groupBy ?? "",
    aggregate: transform?.aggregate ?? DEFAULT_AGGREGATE,
    aggregateField: transform?.field ?? "",
    title: seed.title,
  };
}

/** What picking a different operation resets every other field to (P8: there is no question behind any of this, so there is nothing to carry over) - `null` is the "clear the picker" case, `emptyFieldValues` with that entry's own defaults layered on. */
export function valuesForEntry(next: CatalogEntry | null): PanelFieldValues {
  const empty = emptyFieldValues();
  const chartDefault = next?.view?.chart;

  return {
    ...empty,
    entry: next,
    component: next?.component ?? empty.component,
    title: next?.displayName ?? empty.title,
    category: chartDefault?.category ?? empty.category,
    value: chartDefault?.value ?? empty.value,
    kind: chartDefault?.kind ?? empty.kind,
  };
}

/** The setters `usePanelFields` needs to apply one `PanelFieldValues` - every field it seeds or resets, in one call each. */
export interface PanelFieldSetters {
  readonly setEntry: (entry: CatalogEntry | null) => void;
  readonly setArgsValues: Dispatch<SetStateAction<Record<string, unknown>>>;
  readonly setComponent: Dispatch<SetStateAction<Component>>;
  readonly setCategory: (value: string) => void;
  readonly setValue: (value: string) => void;
  readonly setKind: Dispatch<SetStateAction<ChartKind>>;
  readonly setTransformEnabled: (value: boolean) => void;
  readonly setGroupBy: (value: string) => void;
  readonly setAggregate: Dispatch<SetStateAction<Aggregate>>;
  readonly setAggregateField: (value: string) => void;
  readonly setTitle: (value: string) => void;
}

/** Applies one `PanelFieldValues` through every setter in `setters` - `selectEntry`'s own body, factored out so that callback stays a one-line call. */
export function applyFieldValues(values: PanelFieldValues, setters: PanelFieldSetters): void {
  setters.setEntry(values.entry);
  setters.setArgsValues(values.argsValues);
  setters.setComponent(values.component);
  setters.setCategory(values.category);
  setters.setValue(values.value);
  setters.setKind(values.kind);
  setters.setTransformEnabled(values.transformEnabled);
  setters.setGroupBy(values.groupBy);
  setters.setAggregate(values.aggregate);
  setters.setAggregateField(values.aggregateField);
  setters.setTitle(values.title);
}

/** Everything `usePanelFields` derives from its current values, rather than storing directly - the component/chart-axis options a picker offers, the guarded setters that keep a `<select>`'s bare string inside its own enum, and whether the form is complete enough to save. */
export interface DerivedFieldHelpers {
  readonly componentOptions: readonly Component[];
  readonly fieldOptions: readonly FieldOption[];
  readonly chartFieldOptions: readonly FieldOption[];
  readonly setComponent: (value: string) => void;
  readonly setKind: (value: string) => void;
  readonly setAggregate: (value: string) => void;
  readonly missingBeforeSave: () => string | null;
}

/** Builds `DerivedFieldHelpers` from `usePanelFields`'s current state - split out so that hook's own body stays under `max-lines-per-function`. */
export function deriveFieldHelpers(
  values: PanelFieldValues,
  setComponentState: Dispatch<SetStateAction<Component>>,
  setKindState: Dispatch<SetStateAction<ChartKind>>,
  setAggregateState: Dispatch<SetStateAction<Aggregate>>,
): DerivedFieldHelpers {
  const componentOptions = componentOptionsFor(values.entry);
  const fieldOptions = fieldOptionsFor(values.entry);
  const chartFieldOptions = chartFieldOptionsFor(
    fieldOptions,
    values.transformEnabled,
    values.groupBy,
    values.aggregate,
  );

  const setComponent = guardedSetter(
    (candidate): candidate is Component => componentOptions.some((option) => option === candidate),
    setComponentState,
  );

  return {
    componentOptions,
    fieldOptions,
    chartFieldOptions,
    setComponent,
    setKind: guardedSetter(isChartKind, setKindState),
    setAggregate: guardedSetter(isAggregate, setAggregateState),
    missingBeforeSave: () =>
      missingBeforeSave(
        values.title,
        values.component,
        values.category,
        values.value,
        values.transformEnabled,
        values.groupBy,
        values.aggregate,
        values.aggregateField,
      ),
  };
}
