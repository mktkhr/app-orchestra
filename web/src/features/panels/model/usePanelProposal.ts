import { addPanel, type WorkspacePanel } from "@/shared/api/client";
import type { CatalogEntry } from "@/shared/api/catalog";
import type { ProposedPanel } from "@/shared/api/panels";

import type { PanelFormState } from "./panelFormState";
import { buildAddPanelRequest, usePanelSave } from "./panelSubmit";
import type { Catalog } from "./useCatalog";
import { usePanelFields } from "./usePanelFields";

/**
 * The steps 1-5 form (`AddPanelForm`), reused a third time - over what a
 * `propose_panel` call filled in (`docs/specs/proposing.md` section 5,
 * `docs/plans/proposing.md` Task 1), rather than an empty builder
 * (`usePanelBuilder`) or an existing panel (`usePanelEditor`). `entry` is
 * the catalogue's own entry for `panel`'s operation - the caller
 * (`ProposalControl`) has already matched it, the same way
 * `EditPanelControl` does for an existing panel, so this hook never loads
 * the catalogue itself; `catalog` here is the same single-entry stand-in
 * `usePanelEditor` uses, for the same reason: `OperationPicker` never
 * renders while the operation is locked (P13's own reasoning applies here
 * too - a proposal already named which operation it calls).
 *
 * Posts through `POST /api/workspaces/{id}/panels`, exactly the endpoint a
 * hand-built panel already uses - the model is never in this request (N1).
 * Editing a field before pressing "配置" changes what
 * `usePanelFields`/`usePanelSave` post, so a proposal edited before being
 * placed is placed as edited (AC-N-103).
 */
export function usePanelProposal(
  workspaceId: string,
  panel: ProposedPanel,
  entry: CatalogEntry,
  onPlaced: (panel: WorkspacePanel) => void,
): PanelFormState {
  const fields = usePanelFields({
    entry,
    title: panel.title,
    args: panel.args,
    component: panel.component,
    ...(panel.view === undefined ? {} : { view: panel.view }),
  });

  const catalog: Catalog = {
    loaded: true,
    loadError: null,
    entries: [entry],
    // No-op: entry is already resolved by the caller (see this file's own
    // header comment), so there is nothing left for AddPanelForm's
    // OperationPicker to load - and it never renders one anyway, while
    // the operation is locked.
    load() {
      /* nothing to load */
    },
  };

  const save = usePanelSave<WorkspacePanel>(
    fields,
    entry,
    (resolvedEntry, payload) => addPanel(workspaceId, buildAddPanelRequest(resolvedEntry, payload)),
    onPlaced,
  );

  return { catalog, ...fields, ...save };
}
