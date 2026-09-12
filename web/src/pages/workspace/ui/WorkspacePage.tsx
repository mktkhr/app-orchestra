import Alert from "@mui/material/Alert";
import CircularProgress from "@mui/material/CircularProgress";
import Divider from "@mui/material/Divider";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import { AddPanelControl } from "@/features/panels";
import { ConversationPanel } from "@/widgets/conversation";

import { useWorkspacePage } from "../model/useWorkspacePage";
import { PanelResult } from "./PanelResult";

interface WorkspacePageProps {
  readonly workspaceId: string;
}

/**
 * A workspace screen: its name, its panels in a column, each drawn by its
 * own `PanelCard` (`docs/specs/workspaces.md` section 8), the control that
 * adds one without asking a question (`docs/plans/dashboard.md` Task 6,
 * AC-P-102), and below all of that the same conversation the chat screen
 * offers - asking there appends turns to that conversation, and a result's
 * save control defaults to this workspace. Panels load independently of one
 * another - `PanelCard` posts each one to `/api/invoke` itself - so one slow
 * or unreachable service only shows up in its own card, not as a delay on
 * the rest (AC-W-102, AC-W-106).
 */
export function WorkspacePage({ workspaceId }: WorkspacePageProps): JSX.Element {
  const { workspace, loading, error, addedPanels } = useWorkspacePage(workspaceId);

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
        {[...workspace.panels, ...addedPanels.panels].map((panel) => (
          <PanelResult key={panel.id} panel={panel} />
        ))}
      </Stack>
      <AddPanelControl workspaceId={workspaceId} onAdded={addedPanels.add} />
      <Divider />
      <ConversationPanel defaultWorkspaceId={workspaceId} />
    </Stack>
  );
}
