import { useState } from "react";

import { addPanel, type AddPanelRequest, type WorkspacePanel } from "@/shared/api/client";
import type { CatalogEntry } from "@/shared/api/catalog";
import { useSubmission } from "@/shared/lib/useSubmission";

import type { PanelFormState } from "./panelFormState";
import { buildView, compactArgs } from "./panelViewRequest";
import { useCatalog } from "./useCatalog";
import { usePanelFields } from "./usePanelFields";

export type { PanelFormState };

/** `service:operationId` - a stable identity for one catalogue entry, used to key `PanelArguments` so it remounts (and reseeds) on a new operation. */
export function operationKey(entry: CatalogEntry): string {
  return `${entry.service}:${entry.operationId}`;
}

export interface PanelBuilder extends PanelFormState {
  readonly open: boolean;
  readonly handleOpen: () => void;
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
