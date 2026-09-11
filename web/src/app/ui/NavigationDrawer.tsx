import ChatIcon from "@mui/icons-material/Chat";
import Drawer from "@mui/material/Drawer";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemIcon from "@mui/material/ListItemIcon";
import ListItemText from "@mui/material/ListItemText";
import Toolbar from "@mui/material/Toolbar";
import type { JSX } from "react";

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

/** The application's left navigation. One entry today: チャット. */
export function NavigationDrawer({ open, beside, onClose }: NavigationDrawerProps): JSX.Element {
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
        <ListItem disablePadding>
          <ListItemButton component="a" href="#chat" onClick={onClose}>
            <ListItemIcon>
              <ChatIcon />
            </ListItemIcon>
            <ListItemText primary="チャット" />
          </ListItemButton>
        </ListItem>
      </List>
    </Drawer>
  );
}
