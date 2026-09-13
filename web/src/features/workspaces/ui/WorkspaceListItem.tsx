import DeleteIcon from "@mui/icons-material/Delete";
import FolderIcon from "@mui/icons-material/Folder";
import IconButton from "@mui/material/IconButton";
import ListItem from "@mui/material/ListItem";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemIcon from "@mui/material/ListItemIcon";
import ListItemText from "@mui/material/ListItemText";
import type { JSX } from "react";
import { Link } from "react-router";

import type { WorkspaceSummary } from "@/shared/api/client";

interface WorkspaceListItemProps {
  readonly workspace: WorkspaceSummary;
  readonly onNavigate: () => void;
  readonly onRequestDelete: (workspace: WorkspaceSummary) => void;
}

/**
 * One workspace row in the drawer: a link to it, per `WorkspaceList`'s doc
 * on what selecting one does today, and a delete control that only asks -
 * `WorkspaceList` owns the confirmation itself.
 */
export function WorkspaceListItem({
  workspace,
  onNavigate,
  onRequestDelete,
}: WorkspaceListItemProps): JSX.Element {
  const handleRequestDelete = (): void => {
    onRequestDelete(workspace);
  };

  return (
    <ListItem
      disablePadding
      secondaryAction={
        <IconButton edge="end" aria-label={`${workspace.name}を削除`} onClick={handleRequestDelete}>
          <DeleteIcon />
        </IconButton>
      }
    >
      <ListItemButton component={Link} to={`/workspaces/${workspace.id}`} onClick={onNavigate}>
        <ListItemIcon>
          <FolderIcon />
        </ListItemIcon>
        <ListItemText primary={workspace.name} />
      </ListItemButton>
    </ListItem>
  );
}
