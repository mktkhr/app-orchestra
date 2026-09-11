import Alert from "@mui/material/Alert";
import List from "@mui/material/List";
import { useState, type JSX } from "react";

import { type WorkspaceSummary } from "@/shared/api/client";

import { useWorkspaces } from "../model/useWorkspaces";
import { CreateWorkspaceForm } from "./CreateWorkspaceForm";
import { DeleteWorkspaceDialog } from "./DeleteWorkspaceDialog";
import { WorkspaceListItem } from "./WorkspaceListItem";

interface WorkspaceListProps {
  /**
   * Called after a workspace row is followed, the same way チャット closes
   * the drawer on a phone (`NavigationDrawer`'s own `onClose`). Choosing a
   * workspace does not do anything past that yet: opening one and drawing
   * its panels is Task 3 (`docs/plans/workspaces.md`); this only has to
   * list them and let one be picked.
   */
  readonly onNavigate: () => void;
}

/**
 * The drawer's workspace section: every workspace under チャット, a control
 * to make one, and - per row - a control to remove one, which asks first
 * (`docs/specs/workspaces.md` section 8; deleting a workspace takes its
 * panels with it).
 */
export function WorkspaceList({ onNavigate }: WorkspaceListProps): JSX.Element {
  const { workspaces, error, create, remove } = useWorkspaces();
  const [pendingDelete, setPendingDelete] = useState<WorkspaceSummary | null>(null);

  const handleCreate = (name: string): void => {
    void create(name);
  };

  const handleCancelDelete = (): void => {
    setPendingDelete(null);
  };

  const handleConfirmDelete = (): void => {
    if (pendingDelete === null) {
      return;
    }

    const target = pendingDelete;

    setPendingDelete(null);
    void remove(target.id);
  };

  return (
    <>
      <List>
        {workspaces.map((workspace) => (
          <WorkspaceListItem
            key={workspace.id}
            workspace={workspace}
            onNavigate={onNavigate}
            onRequestDelete={setPendingDelete}
          />
        ))}
      </List>
      <CreateWorkspaceForm onCreate={handleCreate} />
      {error === null ? null : (
        <Alert severity="error" sx={{ mx: 2, mt: 1 }}>
          {error}
        </Alert>
      )}
      <DeleteWorkspaceDialog
        workspace={pendingDelete}
        onCancel={handleCancelDelete}
        onConfirm={handleConfirmDelete}
      />
    </>
  );
}
