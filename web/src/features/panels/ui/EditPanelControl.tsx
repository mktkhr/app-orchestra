import EditIcon from "@mui/icons-material/Edit";
import Dialog from "@mui/material/Dialog";
import DialogTitle from "@mui/material/DialogTitle";
import IconButton from "@mui/material/IconButton";
import { useEffect, useState, type JSX } from "react";

import type { WorkspacePanel } from "@/shared/api/client";

import { useCatalog } from "../model/useCatalog";
import { EditPanelDialogContent } from "./EditPanelDialogContent";

interface EditPanelControlProps {
  readonly workspaceId: string;
  readonly panel: WorkspacePanel;
  /** Called with the panel `PATCH /api/workspaces/{id}/panels/{panelId}` just returned, so the caller can draw it immediately (AC-P-110). */
  readonly onSaved: (panel: WorkspacePanel) => void;
}

/**
 * The workspace screen's own "edit this panel" control
 * (`docs/specs/dashboard.md` section 6a, P12): an icon button, beside
 * `PanelResult`'s refresh control (`PanelActions`), that opens the
 * builder's own form over this panel rather than a second one. The
 * catalogue loads once the dialog opens - the same lazy-on-first-open
 * shape `usePanelBuilder` uses - and `EditPanelDialogContent` is where the
 * panel's own entry is matched against it and the form is actually drawn,
 * split out only to keep this file's own import count under
 * `import/max-dependencies`.
 */
export function EditPanelControl({
  workspaceId,
  panel,
  onSaved,
}: EditPanelControlProps): JSX.Element {
  const [open, setOpen] = useState(false);
  const catalog = useCatalog();

  useEffect(() => {
    if (open) {
      catalog.load();
    }
    // catalog.load is a no-op once loaded or already loading (its own
    // comment), so listing catalog itself - not just open - costs nothing:
    // its identity changes every render, but the guard inside load() keeps
    // the resulting extra calls from ever reaching setState twice.
  }, [open, catalog]);

  const handleClose = (): void => {
    setOpen(false);
  };

  const handleSaved = (saved: WorkspacePanel): void => {
    onSaved(saved);
    setOpen(false);
  };

  return (
    <>
      <IconButton
        onClick={() => {
          setOpen(true);
        }}
        aria-label="編集"
      >
        <EditIcon />
      </IconButton>
      <Dialog open={open} onClose={handleClose} fullWidth maxWidth="sm">
        <DialogTitle>パネルを編集</DialogTitle>
        <EditPanelDialogContent
          workspaceId={workspaceId}
          panel={panel}
          catalog={catalog}
          onSaved={handleSaved}
        />
      </Dialog>
    </>
  );
}
