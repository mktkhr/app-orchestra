import Divider from "@mui/material/Divider";
import Drawer from "@mui/material/Drawer";
import List from "@mui/material/List";
import Toolbar from "@mui/material/Toolbar";
import type { JSX } from "react";

import { useSession } from "@/features/session";
import { WorkspaceList } from "@/features/workspaces";

import { ChatNavItem } from "./ChatNavItem";
import { UsersNavItem } from "./UsersNavItem";

export const DRAWER_WIDTH = 240;

interface NavigationDrawerProps {
  readonly open: boolean;
  /**
   * True when the viewport has room to hold the drawer beside the content.
   * A persistent drawer takes {@link DRAWER_WIDTH} out of the width and
   * leaves the rest to `main`; on a 375px phone that is 240 of 375, and the
   * page ends up wider than the screen. Below that width the drawer floats
   * over the content and closes when it is dismissed.
   */
  readonly beside: boolean;
  readonly onClose: () => void;
}

/**
 * The application's left navigation: チャット, then every workspace, and -
 * for an admin only - ユーザー管理 (docs/plans/auth.md Task 5). Hidden from
 * a non-admin here; the endpoints behind it refuse them with 403 either
 * way (docs/specs/auth.md section 7), since a hidden control is not a
 * check.
 */
export function NavigationDrawer({ open, beside, onClose }: NavigationDrawerProps): JSX.Element {
  const { user } = useSession();

  return (
    <Drawer
      variant={beside ? "persistent" : "temporary"}
      open={open}
      onClose={onClose}
      sx={{
        width: beside && open ? DRAWER_WIDTH : 0,
        flexShrink: 0,
        [`& .MuiDrawer-paper`]: { width: DRAWER_WIDTH, boxSizing: "border-box" },
      }}
    >
      <Toolbar />
      <List>
        <ChatNavItem onClick={onClose} />
        {user?.role === "admin" ? <UsersNavItem onClick={onClose} /> : null}
      </List>
      <Divider />
      <WorkspaceList onNavigate={onClose} />
    </Drawer>
  );
}
