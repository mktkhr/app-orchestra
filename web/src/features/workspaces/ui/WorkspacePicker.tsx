import Alert from "@mui/material/Alert";
import MenuItem from "@mui/material/MenuItem";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import type { ChangeEvent, JSX } from "react";

import { NEW_WORKSPACE, type WorkspaceSummary } from "../model/useSaveToWorkspace";

interface WorkspacePickerProps {
  readonly loaded: boolean;
  readonly loadError: string | null;
  readonly workspaces: readonly WorkspaceSummary[];
  readonly workspaceId: string;
  readonly newWorkspaceName: string;
  readonly onWorkspaceIdChange: (id: string) => void;
  readonly onNewWorkspaceNameChange: (name: string) => void;
}

/**
 * Which workspace `SaveToWorkspaceControl` saves to, plus - when there are
 * none yet, or "make one" is picked - the field to name a new one. Split
 * out of that component so its own body stays about submitting, not every
 * branch of what the picker can show (`max-lines-per-function`).
 */
export function WorkspacePicker({
  loaded,
  loadError,
  workspaces,
  workspaceId,
  newWorkspaceName,
  onWorkspaceIdChange,
  onNewWorkspaceNameChange,
}: WorkspacePickerProps): JSX.Element {
  if (!loaded) {
    return (
      <Typography variant="body2" color="textSecondary">
        ワークスペースを読み込んでいます…
      </Typography>
    );
  }

  if (loadError !== null) {
    return <Alert severity="error">{loadError}</Alert>;
  }

  return (
    <>
      <TextField
        select
        fullWidth
        label="保存先のワークスペース"
        value={workspaceId}
        onChange={(event: ChangeEvent<HTMLInputElement>) => {
          onWorkspaceIdChange(event.target.value);
        }}
      >
        {workspaces.map((workspace) => (
          <MenuItem key={workspace.id} value={workspace.id}>
            {workspace.name}
          </MenuItem>
        ))}
        <MenuItem value={NEW_WORKSPACE}>新しいワークスペースを作る</MenuItem>
      </TextField>
      {workspaceId === NEW_WORKSPACE ? (
        <TextField
          fullWidth
          label="新しいワークスペース名"
          value={newWorkspaceName}
          onChange={(event: ChangeEvent<HTMLInputElement>) => {
            onNewWorkspaceNameChange(event.target.value);
          }}
        />
      ) : null}
    </>
  );
}
