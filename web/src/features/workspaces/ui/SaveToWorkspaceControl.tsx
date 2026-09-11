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
  /**
   * Which workspace to preselect, when the caller already sits on one -
   * asking from inside a workspace (`docs/plans/workspaces.md` Task 6)
   * defaults the picker to that workspace rather than the first one
   * loaded. Omitted from the chat screen, where no workspace is current.
   */
  readonly defaultWorkspaceId?: string | undefined;
}

/**
 * A result turn's "save to a workspace" control (AC-W-101,
 * docs/specs/workspaces.md section 7, "From the chat"). Posts exactly the
 * provenance the result is already showing - `source.service`,
 * `source.operationId`, `source.args` - plus the component it was drawn
 * with, to `POST /api/workspaces/{id}/panels`. Nothing is derived from the
 * result's data; a panel is a saved call, not a saved answer (W2).
 *
 * Lives in features/workspaces, not entities/rendering: creating a
 * workspace and adding a panel to one are workspaces' actions, not a
 * rendering concern. `features/conversation` cannot import this slice
 * directly (Feature-Sliced Design forbids one feature importing a sibling),
 * so `TurnList`/`Conversation` (`features/conversation`) take a
 * `renderSaveControl` slot instead and never name this component; the page
 * that composes both features (`widgets/conversation`) is what plugs this
 * control into that slot.
 *
 * State and the save itself live in `useSaveToWorkspace`; this component is
 * only the three things it can show - the button, the open form, or the
 * saved notice.
 */
export function SaveToWorkspaceControl({
  source,
  component,
  defaultTitle,
  defaultWorkspaceId,
}: SaveToWorkspaceControlProps): JSX.Element {
  const state = useSaveToWorkspace(source, component, defaultTitle, defaultWorkspaceId);

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
