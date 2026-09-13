import { useState } from "react";

import { addPanel, type WorkspacePanel } from "@/shared/api/client";
import type { CatalogEntry } from "@/shared/api/catalog";

import type { PanelFormState } from "./panelFormState";
import { buildAddPanelRequest, usePanelSave } from "./panelSubmit";
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
 * `max-lines-per-function`; the save button's own behaviour - what is
 * still missing, building the request, posting it - is `usePanelSave`
 * (`panelSubmit.ts`), shared with `usePanelEditor` and `usePanelProposal`.
 * This hook only opens the form and, on a successful save, closes it again
 * and clears the picker.
 */
export function usePanelBuilder(
  workspaceId: string,
  onAdded: (panel: WorkspacePanel) => void,
): PanelBuilder {
  const [open, setOpen] = useState(false);
  const catalog = useCatalog();
  const fields = usePanelFields();

  const handleOpen = (): void => {
    setOpen(true);
    catalog.load();
  };

  const save = usePanelSave<WorkspacePanel>(
    fields,
    fields.entry,
    (entry, payload) => addPanel(workspaceId, buildAddPanelRequest(entry, payload)),
    (panel) => {
      onAdded(panel);
      setOpen(false);
      fields.selectEntry(null);
    },
  );

  return { open, handleOpen, catalog, ...fields, ...save };
}
