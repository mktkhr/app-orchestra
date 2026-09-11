import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";
import type { JSX } from "react";

interface DeleteWorkspaceDialogProps {
  /** The workspace pending deletion, or `null` when the dialog is closed. */
  readonly workspace: { readonly name: string } | null;
  readonly onCancel: () => void;
  readonly onConfirm: () => void;
}

/**
 * Asks before a workspace is deleted - it takes every one of its panels
 * with it (`docs/plans/workspaces.md`, Task 2). Rendered always, open only
 * while `workspace` is set, so it can animate closed rather than vanish.
 */
export function DeleteWorkspaceDialog({
  workspace,
  onCancel,
  onConfirm,
}: DeleteWorkspaceDialogProps): JSX.Element {
  return (
    <Dialog open={workspace !== null} onClose={onCancel}>
      <DialogTitle>ワークスペースを削除しますか？</DialogTitle>
      <DialogContent>
        <DialogContentText>
          {workspace === null
            ? ""
            : `「${workspace.name}」を削除します。中のパネルもすべて削除され、元に戻せません。`}
        </DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel}>キャンセル</Button>
        <Button onClick={onConfirm} color="error" variant="contained">
          削除する
        </Button>
      </DialogActions>
    </Dialog>
  );
}
