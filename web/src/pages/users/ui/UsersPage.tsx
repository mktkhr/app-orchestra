import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import { useState, type JSX } from "react";

import { AccountList, PermissionGrid } from "@/features/permissions";

/**
 * The admin's screen (docs/plans/auth.md Task 5): every account on the
 * left, and - once one is selected - its permission grid on the right,
 * grouped by service (docs/specs/auth.md section 4). Reached only through
 * the drawer's admin-only entry; the endpoints behind it refuse a
 * non-admin with 403 regardless (docs/specs/auth.md section 7 - a hidden
 * control is not a check).
 */
export function UsersPage(): JSX.Element {
  const [selectedId, setSelectedId] = useState<string | null>(null);

  return (
    <Stack spacing={3} sx={{ p: 3 }}>
      <Typography variant="h5" component="h1">
        ユーザー管理
      </Typography>
      <Stack direction={{ xs: "column", md: "row" }} spacing={3}>
        <Box sx={{ minWidth: 240 }}>
          <AccountList selectedId={selectedId} onSelect={setSelectedId} />
        </Box>
        <Box sx={{ flexGrow: 1 }}>
          {selectedId === null ? (
            <Typography color="text.secondary">
              権限を設定するアカウントを選んでください。
            </Typography>
          ) : (
            <PermissionGrid key={selectedId} userId={selectedId} />
          )}
        </Box>
      </Stack>
    </Stack>
  );
}
