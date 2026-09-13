import PeopleIcon from "@mui/icons-material/People";
import type { JSX } from "react";

import { DrawerNavItem } from "./DrawerNavItem";

interface UsersNavItemProps {
  readonly onClick: () => void;
}

/**
 * The drawer's link to the admin's users screen (docs/plans/auth.md
 * Task 5). `NavigationDrawer` renders this only for an admin - a
 * non-admin never sees the entry, though the endpoints behind it refuse
 * them with 403 regardless (docs/specs/auth.md section 7: a hidden
 * control is not a check).
 */
export function UsersNavItem({ onClick }: UsersNavItemProps): JSX.Element {
  return <DrawerNavItem to="/users" icon={<PeopleIcon />} label="ユーザー管理" onClick={onClick} />;
}
