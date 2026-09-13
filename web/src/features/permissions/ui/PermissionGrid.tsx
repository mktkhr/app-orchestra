import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import { usePermissionGrid } from "../model/usePermissionGrid";
import { ServiceCard } from "./ServiceCard";

interface PermissionGridProps {
  readonly userId: string;
}

/**
 * The admin's permission grid for one account (docs/plans/auth.md Task 5):
 * every operation the catalogue holds, grouped by service (docs/specs/auth.md
 * section 4), with a control that grants or revokes a whole service in one
 * gesture. Saving replaces the account's permissions wholesale
 * (`PUT /api/users/{id}/permissions`) - nothing here is written until then.
 */
export function PermissionGrid({ userId }: PermissionGridProps): JSX.Element {
  const {
    groups,
    granted,
    loading,
    error,
    saving,
    saveError,
    saved,
    toggleOperation,
    toggleService,
    save,
  } = usePermissionGrid(userId);

  if (loading) {
    return (
      <Stack sx={{ p: 2, alignItems: "center" }}>
        <CircularProgress aria-label="読み込み中" />
      </Stack>
    );
  }

  if (error !== null) {
    return <Alert severity="error">{error}</Alert>;
  }

  const handleSave = (): void => {
    void save();
  };

  return (
    <Stack spacing={2}>
      {groups.map((group) => (
        <ServiceCard
          key={group.service}
          group={group}
          granted={granted}
          onToggleOperation={toggleOperation}
          onToggleService={toggleService}
        />
      ))}
      {saveError === null ? null : <Alert severity="error">{saveError}</Alert>}
      <Stack direction="row" spacing={2} sx={{ alignItems: "center" }}>
        <Button variant="contained" onClick={handleSave}>
          保存する
        </Button>
        {saving ? <CircularProgress size={20} aria-label="保存中" /> : null}
        {saved && !saving ? (
          <Typography variant="body2" color="textSecondary">
            保存しました
          </Typography>
        ) : null}
      </Stack>
    </Stack>
  );
}
