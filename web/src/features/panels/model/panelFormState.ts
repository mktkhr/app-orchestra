import type { Dispatch, SetStateAction } from "react";

import type { Aggregate, ChartKind } from "@/entities/rendering";
import type { Component } from "@/shared/api/client";
import type { CatalogEntry } from "@/shared/api/catalog";

import type { Catalog } from "./useCatalog";
import type { FieldOption } from "./usePanelFields";

/**
 * Everything `AddPanelForm` (`docs/specs/dashboard.md` section 6, steps
 * 1-5) reads and calls, regardless of whether it is building a new panel
 * (`usePanelBuilder`'s `PanelBuilder`) or editing an existing one
 * (`usePanelEditor`'s `PanelEditor`) - the shared surface so both hooks
 * feed the one form (P12), rather than the form growing a second shape to
 * match a second hook.
 */
export interface PanelFormState {
  readonly catalog: Catalog;
  readonly entry: CatalogEntry | null;
  readonly selectEntry: (entry: CatalogEntry | null) => void;
  readonly argsValues: Record<string, unknown>;
  readonly setArgsValues: Dispatch<SetStateAction<Record<string, unknown>>>;
  readonly component: Component;
  readonly setComponent: (value: string) => void;
  readonly componentOptions: readonly Component[];
  readonly fieldOptions: readonly FieldOption[];
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
  readonly submitting: boolean;
  readonly error: string | null;
  readonly handleSave: () => void;
}
