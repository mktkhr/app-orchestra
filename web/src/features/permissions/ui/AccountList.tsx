import Alert from "@mui/material/Alert";
import CircularProgress from "@mui/material/CircularProgress";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";
import Stack from "@mui/material/Stack";
import type { JSX } from "react";

import type { UserAccount } from "@/shared/api/users";

import { useAccounts } from "../model/useAccounts";

/** `docs/specs/auth.md` A5's two roles, in Japanese - the same labelling the platform's own `Role` schema carries as `x-enum-labels`. */
const ROLE_LABEL: Record<UserAccount["role"], string> = { admin: "管理者", user: "一般" };

interface AccountListProps {
  readonly selectedId: string | null;
  readonly onSelect: (id: string) => void;
}

/**
 * Every account the platform knows (docs/plans/auth.md Task 5), each row
 * naming its role. Selecting a row is what `UsersPage` uses to decide whose
 * permission grid to show.
 */
export function AccountList({ selectedId, onSelect }: AccountListProps): JSX.Element {
  const { accounts, loading, error } = useAccounts();

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

  return (
    <List aria-label="アカウント一覧">
      {accounts.map((account) => (
        // ListItem, not a bare ListItemButton: List renders a <ul> and
        // ListItemButton renders a <div role="button">, so a <ul> holding
        // them directly is a list whose children are not list items - a
        // serious axe violation (WCAG "list"), and the reason
        // harness/quality/browser only started catching it once its gates
        // were pointed at this screen. DrawerNavItem and WorkspaceListItem
        // already wrap theirs the same way.
        <ListItem key={account.id} disablePadding>
          <ListItemButton
            selected={account.id === selectedId}
            onClick={() => {
              onSelect(account.id);
            }}
          >
            <ListItemText primary={account.name} secondary={ROLE_LABEL[account.role]} />
          </ListItemButton>
        </ListItem>
      ))}
    </List>
  );
}
