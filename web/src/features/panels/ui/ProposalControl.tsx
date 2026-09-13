import Alert from "@mui/material/Alert";
import CircularProgress from "@mui/material/CircularProgress";
import Stack from "@mui/material/Stack";
import { useEffect, useState, type JSX } from "react";

import type { WorkspacePanel } from "@/shared/api/client";
import type { CatalogEntry } from "@/shared/api/catalog";
import type { ProposedPanel } from "@/shared/api/panels";

import { useCatalog, type Catalog } from "../model/useCatalog";
import { usePanelProposal } from "../model/usePanelProposal";
import { AddPanelForm } from "./AddPanelForm";

interface ProposalFormProps {
  readonly workspaceId: string;
  readonly panel: ProposedPanel;
  readonly entry: CatalogEntry;
  readonly onPlaced: (panel: WorkspacePanel) => void;
}

/**
 * Calls `usePanelProposal` - a hook - only once `entry` is known, by
 * existing only while it is: `ProposalControl` renders a spinner or its
 * catalogue's load error in this component's place until then, the same
 * split `EditPanelDialogContent`/`EditPanelDialogBody` use for the rules
 * of hooks.
 *
 * Shows the form until the panel is placed, then a confirmation in its
 * place - the save control does not offer to place the same proposal
 * twice.
 */
function ProposalForm({ workspaceId, panel, entry, onPlaced }: ProposalFormProps): JSX.Element {
  const [placed, setPlaced] = useState<WorkspacePanel | null>(null);

  const builder = usePanelProposal(workspaceId, panel, entry, (result) => {
    setPlaced(result);
    onPlaced(result);
  });

  if (placed !== null) {
    return <Alert severity="success">「{placed.title}」を配置しました。</Alert>;
  }

  return <AddPanelForm builder={builder} operationLocked saveLabel="配置" />;
}

interface ProposalControlProps {
  readonly workspaceId: string;
  /** The panel a `propose_panel` call filled in, as `/api/plan`'s `kind: "proposal"` result carries it. */
  readonly panel: ProposedPanel;
  /** Called with the panel `POST /api/workspaces/{id}/panels` just returned, so the caller can draw it immediately, the same as `AddPanelControl`'s own `onAdded`. */
  readonly onPlaced: (panel: WorkspacePanel) => void;
}

/**
 * A proposal turn's own body (`docs/specs/proposing.md` section 5,
 * `docs/plans/proposing.md` Task 1): the builder's own form
 * (`AddPanelForm`), opened over the proposal rather than over nothing
 * (`usePanelBuilder`) or an existing panel (`usePanelEditor`) - the same
 * form a third time, not a third one. The catalogue loads once this
 * mounts, so `PanelArguments`/`ComponentPicker`/`ChartFields` have the
 * entry's own schema and fields to draw from, exactly as
 * `EditPanelControl` loads it once its dialog opens.
 *
 * Rendered directly in the conversation, not behind a toggle or a dialog:
 * a proposal already is the open form (N3) - there is no "closed" state
 * to open from, the way `AddPanelControl`'s button has one.
 */
export function ProposalControl({
  workspaceId,
  panel,
  onPlaced,
}: ProposalControlProps): JSX.Element {
  const catalog: Catalog = useCatalog();

  useEffect(() => {
    catalog.load();
    // catalog.load is a no-op once loaded or already loading (its own
    // comment), so listing catalog itself - not just this component
    // mounting - costs nothing, the same reasoning EditPanelControl's own
    // effect gives.
  }, [catalog]);

  const entry =
    catalog.entries.find(
      (candidate) =>
        candidate.service === panel.service && candidate.operationId === panel.operationId,
    ) ?? null;

  if (entry === null) {
    return catalog.loadError === null ? (
      <Stack direction="row" sx={{ justifyContent: "center", py: 2 }}>
        <CircularProgress size={24} aria-label="読み込み中" />
      </Stack>
    ) : (
      <Alert severity="error">{catalog.loadError}</Alert>
    );
  }

  return <ProposalForm workspaceId={workspaceId} panel={panel} entry={entry} onPlaced={onPlaced} />;
}
