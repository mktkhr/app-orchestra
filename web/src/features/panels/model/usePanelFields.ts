import { useState, type Dispatch, type SetStateAction } from "react";

import type { Aggregate, ChartKind } from "@/entities/rendering";
import type { Component } from "@/shared/api/client";
import type { CatalogEntry } from "@/shared/api/catalog";

import type { FieldOption } from "./panelFieldRules";
import {
  applyFieldValues,
  deriveFieldHelpers,
  initialFieldValues,
  valuesForEntry,
  type PanelFieldsSeed,
} from "./panelFieldValues";

export type { FieldOption } from "./panelFieldRules";
export type { PanelFieldsSeed } from "./panelFieldValues";

export interface PanelFields {
  readonly entry: CatalogEntry | null;
  readonly selectEntry: (entry: CatalogEntry | null) => void;
  readonly argsValues: Record<string, unknown>;
  readonly setArgsValues: Dispatch<SetStateAction<Record<string, unknown>>>;
  readonly component: Component;
  readonly setComponent: (value: string) => void;
  readonly componentOptions: readonly Component[];
  readonly fieldOptions: readonly FieldOption[];
  /** What a chart's axes may name - `fieldOptions` itself, or the transform's own output shape once one is enabled (see `chartFieldOptionsFor`). */
  readonly chartFieldOptions: readonly FieldOption[];
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
 *
 * `seed`, when given, starts every field from an existing panel instead of
 * the empty builder defaults - the panel-edit form opened over that panel
 * (P12, section 6a), reusing this same hook rather than a second one
 * (`docs/specs/dashboard.md`'s own instruction for this task: "that is a
 * change to that hook, not a second one"). Only the initial render reads
 * it - a later prop change is not re-seeded, the same way `useState`'s own
 * initial value is only read once; a caller that needs to seed a different
 * panel remounts this hook (`AddPanelForm`'s `key`, the same trick
 * `PanelArguments` already uses per `operationKey`).
 *
 * The pure rules behind every value here - what a chart may name, what a
 * fresh operation resets to, what is still missing - live in
 * `panelFieldRules.ts`, so this hook is only `useState` plus wiring.
 */
export function usePanelFields(seed?: PanelFieldsSeed): PanelFields {
  const initial = initialFieldValues(seed);

  const [entry, setEntry] = useState(initial.entry);
  const [argsValues, setArgsValues] = useState(initial.argsValues);
  const [component, setComponentState] = useState(initial.component);
  const [category, setCategory] = useState(initial.category);
  const [value, setValue] = useState(initial.value);
  const [kind, setKindState] = useState(initial.kind);
  const [transformEnabled, setTransformEnabled] = useState(initial.transformEnabled);
  const [groupBy, setGroupBy] = useState(initial.groupBy);
  const [aggregate, setAggregateState] = useState(initial.aggregate);
  const [aggregateField, setAggregateField] = useState(initial.aggregateField);
  const [title, setTitle] = useState(initial.title);

  const selectEntry = (next: CatalogEntry | null): void => {
    applyFieldValues(valuesForEntry(next), {
      setEntry,
      setArgsValues,
      setComponent: setComponentState,
      setCategory,
      setValue,
      setKind: setKindState,
      setTransformEnabled,
      setGroupBy,
      setAggregate: setAggregateState,
      setAggregateField,
      setTitle,
    });
  };

  const helpers = deriveFieldHelpers(
    {
      entry,
      argsValues,
      component,
      category,
      value,
      kind,
      transformEnabled,
      groupBy,
      aggregate,
      aggregateField,
      title,
    },
    setComponentState,
    setKindState,
    setAggregateState,
  );

  return {
    entry,
    selectEntry,
    argsValues,
    setArgsValues,
    component,
    componentOptions: helpers.componentOptions,
    setComponent: helpers.setComponent,
    fieldOptions: helpers.fieldOptions,
    chartFieldOptions: helpers.chartFieldOptions,
    category,
    setCategory,
    value,
    setValue,
    kind,
    setKind: helpers.setKind,
    transformEnabled,
    setTransformEnabled,
    groupBy,
    setGroupBy,
    aggregate,
    setAggregate: helpers.setAggregate,
    aggregateField,
    setAggregateField,
    title,
    setTitle,
    missingBeforeSave: helpers.missingBeforeSave,
  };
}
