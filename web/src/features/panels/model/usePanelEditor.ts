import type { WorkspacePanel } from "@/shared/api/client";
import type { CatalogEntry } from "@/shared/api/catalog";
import { patchPanel, type UpdatePanelRequest } from "@/shared/api/panels";

import type { PanelFormState } from "./panelFormState";
import { usePanelSave } from "./panelSubmit";
import type { Catalog } from "./useCatalog";
import { usePanelFields } from "./usePanelFields";

/**
 * The steps 1-5 form (`AddPanelForm`), reused over an existing panel
 * (P12, section 6a). `entry` is the catalogue's entry for `panel`'s own
 * (fixed, P13) operation - the caller (`EditPanelControl`) already looked
 * it up before this hook is ever called, so this never needs to load the
 * catalogue itself; `catalog` here is a single-entry stand-in only so
 * `AddPanelForm`'s `PanelFormState` shape is satisfied - `OperationPicker`
 * never renders in edit mode (`AddPanelForm`'s `operationLocked`), so
 * nothing ever reads its `entries` or calls `load`.
 *
 * Unlike `usePanelBuilder`'s request, every field here is always named
 * (AC-P-108's "only the fields the request names change" is the contract's
 * own freedom, not a constraint this form has to reproduce control by
 * control) - the edit form is the single source of truth for the whole
 * panel once it is open, seeded from `panel` itself
 * (`usePanelFields(seed)`), so there is nothing partial about what it
 * posts back. `view` is the one field stated as `null` rather than left
 * out whenever the form has no chart or transform configured - "remove
 * the view", not "leave it alone" (section 6a) - since a fully-restated
 * form has no "leave it alone" case for a field it is actively showing.
 *
 * The save button's own behaviour - what is still missing, building the
 * request, posting it - is `usePanelSave` (`panelSubmit.ts`), shared with
 * `usePanelBuilder` and `usePanelProposal`.
 */
export function usePanelEditor(
  workspaceId: string,
  panel: WorkspacePanel,
  entry: CatalogEntry,
  onSaved: (panel: WorkspacePanel) => void,
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
    // No-op: entry is already resolved by the caller before this hook is
    // ever called (see this file's own header comment), so there is
    // nothing left for AddPanelForm's OperationPicker to load - and it
    // never renders one anyway, in edit mode.
    load() {
      /* nothing to load */
    },
  };

  const save = usePanelSave<WorkspacePanel>(
    fields,
    entry,
    (_entry, payload) => {
      const request: UpdatePanelRequest = {
        title: payload.title,
        args: payload.args,
        component: payload.component,
        view: payload.view ?? null,
      };

      return patchPanel(workspaceId, panel.id, request);
    },
    onSaved,
  );

  return { catalog, ...fields, ...save };
}
