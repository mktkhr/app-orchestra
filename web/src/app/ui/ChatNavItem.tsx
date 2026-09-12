import ChatIcon from "@mui/icons-material/Chat";
import type { JSX } from "react";

import { DrawerNavItem } from "./DrawerNavItem";

interface ChatNavItemProps {
  readonly onClick: () => void;
}

/** The drawer's link back to the chat. */
export function ChatNavItem({ onClick }: ChatNavItemProps): JSX.Element {
  return <DrawerNavItem href="#chat" icon={<ChatIcon />} label="チャット" onClick={onClick} />;
}
