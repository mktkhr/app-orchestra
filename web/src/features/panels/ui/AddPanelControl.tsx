import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import type { JSX } from "react";

import type { WorkspacePanel } from "@/shared/api/client";

import { usePanelBuilder } from "../model/usePanelBuilder";
import { AddPanelForm } from "./AddPanelForm";

interface AddPanelControlProps {
  readonly workspaceId: string;
  /** Called with the panel `POST /api/workspaces/{id}/panels` just returned, so the caller can draw it immediately (AC-P-102). */
  readonly onAdded: (panel: WorkspacePanel) => void;
}

/**
 * The workspace screen's own "add a panel" control
 * (`docs/specs/dashboard.md` section 6, P8): a toggle button that opens the
 * one-screen builder below it. No question is asked and the planner is
 * never called here - every value comes from the catalogue or from what the
 * person typed.
 */
export function AddPanelControl({ workspaceId, onAdded }: AddPanelControlProps): JSX.Element {
  const builder = usePanelBuilder(workspaceId, onAdded);

  if (!builder.open) {
    return (
      <Box>
        <Button variant="outlined" onClick={builder.handleOpen}>
          パネルを追加
        </Button>
      </Box>
    );
  }

  return <AddPanelForm builder={builder} />;
}
