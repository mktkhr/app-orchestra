import Alert from "@mui/material/Alert";
import CircularProgress from "@mui/material/CircularProgress";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import { useWorkspace } from "../model/useWorkspace";
import { PanelResult } from "./PanelResult";

interface WorkspacePageProps {
  readonly workspaceId: string;
}

/**
 * A workspace screen: its name, then its panels in a column, each drawn by
 * its own `PanelCard` (`docs/specs/workspaces.md` section 8). Panels load
 * independently of one another - `PanelCard` posts each one to
 * `/api/invoke` itself - so one slow or unreachable service only shows up
 * in its own card, not as a delay on the rest (AC-W-102, AC-W-106).
 */
export function WorkspacePage({ workspaceId }: WorkspacePageProps): JSX.Element {
  const { workspace, loading, error } = useWorkspace(workspaceId);

  if (loading) {
    return (
      <Stack sx={{ p: 3, alignItems: "center" }}>
        <CircularProgress aria-label="読み込み中" />
      </Stack>
    );
  }

  if (error !== null || workspace === null) {
    return (
      <Stack spacing={3} sx={{ p: 3 }}>
        <Alert severity="error">{error ?? "ワークスペースの取得に失敗しました。"}</Alert>
      </Stack>
    );
  }

  return (
    <Stack spacing={3} sx={{ p: 3 }}>
      <Typography variant="h5" component="h1">
        {workspace.name}
      </Typography>
      <Stack spacing={2}>
        {workspace.panels.map((panel) => (
          <PanelResult key={panel.id} panel={panel} />
        ))}
      </Stack>
    </Stack>
  );
}
