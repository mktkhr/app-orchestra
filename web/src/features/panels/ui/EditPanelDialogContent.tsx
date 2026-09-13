import Alert from "@mui/material/Alert";
import CircularProgress from "@mui/material/CircularProgress";
import DialogContent from "@mui/material/DialogContent";
import Stack from "@mui/material/Stack";
import type { JSX } from "react";

import type { WorkspacePanel } from "@/shared/api/client";
import type { CatalogEntry } from "@/shared/api/catalog";

import type { Catalog } from "../model/useCatalog";
import { usePanelEditor } from "../model/usePanelEditor";
import { AddPanelForm } from "./AddPanelForm";

interface EditPanelDialogBodyProps {
  readonly workspaceId: string;
  readonly panel: WorkspacePanel;
  readonly entry: CatalogEntry;
  readonly onSaved: (panel: WorkspacePanel) => void;
}

/**
 * Calls `usePanelEditor` - which itself calls `usePanelFields`, a hook -
 * only once `entry` is known, by existing only while it is:
 * `EditPanelDialogContent` renders this in the dialog's place until then,
 * so the rules of hooks are never at stake over a catalogue that has not
 * answered yet.
 */
function EditPanelDialogBody({
  workspaceId,
  panel,
  entry,
  onSaved,
}: EditPanelDialogBodyProps): JSX.Element {
  const editor = usePanelEditor(workspaceId, panel, entry, onSaved);

  return <AddPanelForm builder={editor} operationLocked />;
}

interface EditPanelDialogContentProps {
  readonly workspaceId: string;
  readonly panel: WorkspacePanel;
  readonly catalog: Catalog;
  readonly onSaved: (panel: WorkspacePanel) => void;
}

/**
 * `EditPanelControl`'s dialog body, split into its own file so that
 * component's own import count stays under `import/max-dependencies`
 * (`eslint`). Finds `panel`'s own catalogue entry (matched by
 * `service`/`operationId`) once `catalog` has loaded, and draws a spinner
 * or its load error until then - `AddPanelForm` never mounts (and
 * `usePanelEditor`, a hook, is never called) before that entry exists.
 */
export function EditPanelDialogContent({
  workspaceId,
  panel,
  catalog,
  onSaved,
}: EditPanelDialogContentProps): JSX.Element {
  const entry =
    catalog.entries.find(
      (candidate) =>
        candidate.service === panel.service && candidate.operationId === panel.operationId,
    ) ?? null;

  return (
    <DialogContent>
      {entry === null ? (
        catalog.loadError === null ? (
          <Stack direction="row" sx={{ justifyContent: "center", py: 2 }}>
            <CircularProgress size={24} aria-label="読み込み中" />
          </Stack>
        ) : (
          <Alert severity="error">{catalog.loadError}</Alert>
        )
      ) : (
        <EditPanelDialogBody
          workspaceId={workspaceId}
          panel={panel}
          entry={entry}
          onSaved={onSaved}
        />
      )}
    </DialogContent>
  );
}
