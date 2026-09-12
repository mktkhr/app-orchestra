import { useState, type Dispatch, type SetStateAction } from "react";

import type { Aggregate, ChartKind } from "@/entities/rendering";
import {
  addPanel,
  type AddPanelRequest,
  type Component,
  type View,
  type WorkspacePanel,
} from "@/shared/api/client";
import type { CatalogEntry } from "@/shared/api/catalog";
import { useSubmission } from "@/shared/lib/useSubmission";

import { useCatalog, type Catalog } from "./useCatalog";
import { usePanelFields } from "./usePanelFields";

/** `service:operationId` - a stable identity for one catalogue entry, used to key `PanelArguments` so it remounts (and reseeds) on a new operation. */
export function operationKey(entry: CatalogEntry): string {
  return `${entry.service}:${entry.operationId}`;
}

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
 */
function compactArgs(
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

function buildView(
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

export interface PanelBuilder {
  readonly open: boolean;
  readonly handleOpen: () => void;
  readonly catalog: Catalog;
  readonly entry: CatalogEntry | null;
  readonly selectEntry: (entry: CatalogEntry | null) => void;
  readonly setArgsValues: Dispatch<SetStateAction<Record<string, unknown>>>;
  readonly component: Component;
  readonly setComponent: (value: string) => void;
  readonly componentOptions: readonly Component[];
  readonly fieldOptions: readonly string[];
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
  readonly submitting: boolean;
  readonly error: string | null;
  readonly handleSave: () => void;
}

/**
 * All the state behind `docs/specs/dashboard.md` section 6's five steps,
 * and the save itself. Step 1 (`catalog`, `useCatalog`) and steps 2-5
 * (`fields`, `usePanelFields`) are each their own hook, kept under
 * `max-lines-per-function`; this one only opens the form and posts
 * `POST /api/workspaces/{id}/panels` once `fields.missingBeforeSave()`
 * says nothing is left to fill in - and shows what is, rather than
 * ignoring the press, when something still is.
 */
export function usePanelBuilder(
  workspaceId: string,
  onAdded: (panel: WorkspacePanel) => void,
): PanelBuilder {
  const [open, setOpen] = useState(false);
  const [incomplete, setIncomplete] = useState<string | null>(null);
  const catalog = useCatalog();
  const fields = usePanelFields();
  const { submitting, error, run } = useSubmission();

  const handleOpen = (): void => {
    setOpen(true);
    catalog.load();
  };

  const handleSave = (): void => {
    const entry = fields.entry;

    if (entry === null) {
      return;
    }

    const missing = fields.missingBeforeSave();

    if (missing !== null) {
      setIncomplete(missing);

      return;
    }

    setIncomplete(null);

    void run(async () => {
      const view = buildView(
        fields.component,
        fields.category,
        fields.value,
        fields.kind,
        fields.transformEnabled,
        fields.groupBy,
        fields.aggregate,
        fields.aggregateField,
      );

      const request: AddPanelRequest = {
        service: entry.service,
        operationId: entry.operationId,
        args: compactArgs(entry.schema, fields.argsValues),
        component: fields.component,
        title: fields.title.trim(),
        ...(view === undefined ? {} : { view }),
      };

      const panel = await addPanel(workspaceId, request);

      onAdded(panel);
      setOpen(false);
      fields.selectEntry(null);
    });
  };

  // The submission's own error and the "you have not finished" message go
  // out on one channel: there is one Alert, and only one of the two can be
  // true at a time - a press either got as far as the server or did not.
  return {
    open,
    handleOpen,
    catalog,
    ...fields,
    submitting,
    error: error ?? incomplete,
    handleSave,
  };
}
