import type { JSX } from "react";

import { ChatPage } from "@/pages/chat";
import { WorkspacePage } from "@/pages/workspace";

import { useHashRoute } from "../model/useHashRoute";

/**
 * The screen `Shell`'s `main` holds: the chat, or one workspace, chosen by
 * `useHashRoute`. Split out of `Shell` so its own import count stays under
 * `import/max-dependencies` - `Shell` only needs to know this renders
 * *something*, not which screen or how the route is read.
 */
export function MainContent(): JSX.Element {
  const route = useHashRoute();

  if (route.screen === "workspace") {
    return <WorkspacePage workspaceId={route.workspaceId} />;
  }

  return <ChatPage />;
}
