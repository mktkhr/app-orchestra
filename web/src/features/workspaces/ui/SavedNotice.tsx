import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

interface SavedNoticeProps {
  /** The workspace's name, so the turn shows where it ended up. */
  readonly name: string;
}

/** What `SaveToWorkspaceControl` shows once a panel has been saved. */
export function SavedNotice({ name }: SavedNoticeProps): JSX.Element {
  return (
    <Stack direction="row" spacing={0.5} sx={{ alignItems: "center", mt: 1 }}>
      <CheckCircleIcon color="success" fontSize="small" />
      <Typography variant="body2" color="textSecondary">
        {`「${name}」に保存しました`}
      </Typography>
    </Stack>
  );
}
