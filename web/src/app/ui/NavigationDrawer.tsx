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
}

/** The application's left navigation. One entry today: チャット. */
export function NavigationDrawer({ open }: NavigationDrawerProps): JSX.Element {
  return (
    <Drawer
      variant="persistent"
      open={open}
      sx={{
        width: open ? DRAWER_WIDTH : 0,
        flexShrink: 0,
        [`& .MuiDrawer-paper`]: { width: DRAWER_WIDTH, boxSizing: "border-box" },
      }}
    >
      <Toolbar />
      <List>
        <ListItem disablePadding>
          <ListItemButton component="a" href="#chat">
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
