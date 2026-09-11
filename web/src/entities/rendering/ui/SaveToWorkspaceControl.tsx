import BookmarkAddIcon from "@mui/icons-material/BookmarkAdd";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import type { ChangeEvent, JSX } from "react";

import { useSaveToWorkspace, type Component, type Source } from "../model/useSaveToWorkspace";
import { SavedNotice } from "./SavedNotice";
import { WorkspacePicker } from "./WorkspacePicker";

interface SaveToWorkspaceControlProps {
  /** The result's provenance, copied into the panel as-is - nothing derived (W1). */
  readonly source: Source;
  /** The widget the result was drawn with, copied into the panel as-is. */
  readonly component: Component;
  /** The question that produced this result - the title's default, editable before saving. */
  readonly defaultTitle: string;
}

/**
 * A result turn's "save to a workspace" control (AC-W-101,
 * docs/specs/workspaces.md section 7, "From the chat"). Posts exactly the
 * provenance the result is already showing - `source.service`,
 * `source.operationId`, `source.args` - plus the component it was drawn
 * with, to `POST /api/workspaces/{id}/panels`. Nothing is derived from the
 * result's data; a panel is a saved call, not a saved answer (W2).
 *
 * Lives in entities/rendering, not features/workspaces: `TurnList`
 * (`features/conversation`) already imports this slice for `ResultForm`/
 * `ResultChoice`, and Feature-Sliced Design forbids one feature from
 * importing a sibling feature - `features/conversation` cannot reach into
 * `features/workspaces`. `entities/rendering` cannot either: entities sits
 * below features, and importing `features/workspaces` from here would be
 * an import pointing upward, which `make guard-fsd` forbids regardless of
 * sibling rules. So `useSaveToWorkspace` calls `shared/api/client`'s
 * `listWorkspaces`/`createWorkspace`/`addPanel` directly, the same way
 * `ResultForm` calls `postInvoke` directly instead of routing through a
 * feature above it.
 *
 * State and the save itself live in `useSaveToWorkspace`; this component is
 * only the three things it can show - the button, the open form, or the
 * saved notice.
 */
export function SaveToWorkspaceControl({
  source,
  component,
  defaultTitle,
}: SaveToWorkspaceControlProps): JSX.Element {
  const state = useSaveToWorkspace(source, component, defaultTitle);

  if (state.savedName !== null) {
    return <SavedNotice name={state.savedName} />;
  }

  if (!state.open) {
    return (
      <Box sx={{ mt: 1 }}>
        <Button variant="outlined" startIcon={<BookmarkAddIcon />} onClick={state.handleOpen}>
          ワークスペースに保存
        </Button>
      </Box>
    );
  }

  return (
    <Stack spacing={1.5} sx={{ mt: 1 }}>
      <TextField
        fullWidth
        label="タイトル"
        value={state.title}
        onChange={(event: ChangeEvent<HTMLInputElement>) => {
          state.setTitle(event.target.value);
        }}
      />
      <WorkspacePicker
        loaded={state.loaded}
        loadError={state.loadError}
        workspaces={state.workspaces}
        workspaceId={state.workspaceId}
        newWorkspaceName={state.newWorkspaceName}
        onWorkspaceIdChange={state.setWorkspaceId}
        onNewWorkspaceNameChange={state.setNewWorkspaceName}
      />
      {state.error === null ? null : <Alert severity="error">{state.error}</Alert>}
      <Box>
        <Button variant="contained" disabled={state.submitting} onClick={state.handleSave}>
          保存
        </Button>
      </Box>
    </Stack>
  );
}
