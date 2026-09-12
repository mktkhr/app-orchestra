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
 * `POST /api/workspaces/{id}/panels` once `fields.canSave()` says the
 * current choice is complete.
 */
export function usePanelBuilder(
  workspaceId: string,
  onAdded: (panel: WorkspacePanel) => void,
): PanelBuilder {
  const [open, setOpen] = useState(false);
  const catalog = useCatalog();
  const fields = usePanelFields();
  const { submitting, error, run } = useSubmission();

  const handleOpen = (): void => {
    setOpen(true);
    catalog.load();
  };

  const handleSave = (): void => {
    if (!fields.canSave() || fields.entry === null) {
      return;
    }

    const entry = fields.entry;

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
        args: fields.argsValues,
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

  return { open, handleOpen, catalog, ...fields, submitting, error, handleSave };
}
