import type { JSX } from "react";

import { ChatPage } from "@/pages/chat";
import { UsersPage } from "@/pages/users";
import { WorkspacePage } from "@/pages/workspace";

import { useHashRoute } from "../model/useHashRoute";

/**
 * The screen `Shell`'s `main` holds: the chat, one workspace, or the
 * admin's users screen, chosen by `useHashRoute`. Split out of `Shell` so
 * its own import count stays under `import/max-dependencies` - `Shell`
 * only needs to know this renders *something*, not which screen or how the
 * route is read.
 *
 * No role check here for `"users"`: `NavigationDrawer` already hides the
 * entry from a non-admin, and `UsersPage`'s own requests are refused with
 * 403 regardless (docs/specs/auth.md section 7) - a hidden control is not
 * a check, but this component adding a second one would only duplicate
 * what the endpoints already enforce.
 */
export function MainContent(): JSX.Element {
  const route = useHashRoute();

  if (route.screen === "workspace") {
    return <WorkspacePage workspaceId={route.workspaceId} />;
  }

  if (route.screen === "users") {
    return <UsersPage />;
  }

  return <ChatPage />;
}
