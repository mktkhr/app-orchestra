import ChatIcon from "@mui/icons-material/Chat";
import ListItem from "@mui/material/ListItem";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemIcon from "@mui/material/ListItemIcon";
import ListItemText from "@mui/material/ListItemText";
import type { JSX } from "react";

interface ChatNavItemProps {
  readonly onClick: () => void;
}

/** The drawer's link back to the chat. Split out of `NavigationDrawer` to keep its own import count under `import/max-dependencies`. */
export function ChatNavItem({ onClick }: ChatNavItemProps): JSX.Element {
  return (
    <ListItem disablePadding>
      <ListItemButton component="a" href="#chat" onClick={onClick}>
        <ListItemIcon>
          <ChatIcon />
        </ListItemIcon>
        <ListItemText primary="チャット" />
      </ListItemButton>
    </ListItem>
  );
}
