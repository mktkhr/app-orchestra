import ListItem from "@mui/material/ListItem";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemIcon from "@mui/material/ListItemIcon";
import ListItemText from "@mui/material/ListItemText";
import type { JSX, ReactNode } from "react";
import { Link } from "react-router";

interface DrawerNavItemProps {
  readonly to: string;
  readonly icon: ReactNode;
  readonly label: string;
  readonly onClick: () => void;
}

/**
 * One row of the drawer's top-level navigation: an icon, a label, and a
 * `react-router` `Link` to `to` - which changes the screen without a page
 * load, unlike the plain anchor (`href="#..."`) this replaced. Shared by
 * `ChatNavItem` and `UsersNavItem` (docs/plans/auth.md Task 5) so the two
 * are not the same markup copied twice (`make guard-duplication`).
 */
export function DrawerNavItem({ to, icon, label, onClick }: DrawerNavItemProps): JSX.Element {
  return (
    <ListItem disablePadding>
      <ListItemButton component={Link} to={to} onClick={onClick}>
        <ListItemIcon>{icon}</ListItemIcon>
        <ListItemText primary={label} />
      </ListItemButton>
    </ListItem>
  );
}
