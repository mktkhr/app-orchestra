import type { JSX } from "react";
import { Route, Routes, useParams } from "react-router";

import { ChatPage } from "@/pages/chat";
import { UsersPage } from "@/pages/users";
import { WorkspacePage } from "@/pages/workspace";

/**
 * `/workspaces/:workspaceId` matched with no `workspaceId` is not a shape
 * `react-router` produces: a dynamic segment requires at least one
 * character to match at all, so a request for `WorkspaceRoute` always
 * carries one. `useParams`'s own type cannot say that, so this names the
 * invariant instead of asserting past it.
 */
function WorkspaceRoute(): JSX.Element {
  const { workspaceId } = useParams<"workspaceId">();

  if (workspaceId === undefined) {
    throw new Error("workspace route matched without a workspace id");
  }

  return <WorkspacePage workspaceId={workspaceId} />;
}

/**
 * The screen `Shell`'s `main` holds: the chat, one workspace, or the
 * admin's users screen, chosen by the path (`docs/specs/routing.md`
 * section 3). Split out of `Shell` so its own import count stays under
 * `import/max-dependencies` - `Shell` only needs to know this renders
 * *something*, not which screen or how the route is read.
 *
 * An address matching nothing renders the chat, the same as the hash
 * router this replaces.
 *
 * No role check here for `"users"`: `NavigationDrawer` already hides the
 * entry from a non-admin, and `UsersPage`'s own requests are refused with
 * 403 regardless (docs/specs/auth.md section 7) - a hidden control is not
 * a check, but this component adding a second one would only duplicate
 * what the endpoints already enforce.
 */
export function MainContent(): JSX.Element {
  return (
    <Routes>
      <Route path="/" element={<ChatPage />} />
      <Route path="/workspaces/:workspaceId" element={<WorkspaceRoute />} />
      <Route path="/users" element={<UsersPage />} />
      <Route path="*" element={<ChatPage />} />
    </Routes>
  );
}
