import RefreshIcon from "@mui/icons-material/Refresh";
import CircularProgress from "@mui/material/CircularProgress";
import IconButton from "@mui/material/IconButton";
import Stack from "@mui/material/Stack";
import type { JSX } from "react";

import { EditPanelControl } from "@/features/panels";
import type { WorkspacePanel } from "@/shared/api/client";

interface PanelActionsProps {
  readonly workspaceId: string;
  readonly panel: WorkspacePanel;
  readonly refreshing: boolean;
  readonly onRefresh: () => void;
  /** Called with the panel a save just returned, so `PanelResult` can draw it immediately (AC-P-110). */
  readonly onSaved: (panel: WorkspacePanel) => void;
}

/**
 * `PanelResult`'s own header controls: refresh (`usePanelInvoke.refresh`)
 * and edit (`EditPanelControl`, P12), side by side - split out of
 * `PanelResult` only to keep that file's own import count under
 * `import/max-dependencies` (`eslint`), the same reason its own header
 * comment already gives for reading `600px` directly instead of through
 * `useTheme`.
 */
export function PanelActions({
  workspaceId,
  panel,
  refreshing,
  onRefresh,
  onSaved,
}: PanelActionsProps): JSX.Element {
  return (
    <Stack direction="row">
      <IconButton onClick={onRefresh} aria-label={refreshing ? "更新中" : "更新"}>
        {refreshing ? <CircularProgress size={20} /> : <RefreshIcon />}
      </IconButton>
      <EditPanelControl workspaceId={workspaceId} panel={panel} onSaved={onSaved} />
    </Stack>
  );
}
