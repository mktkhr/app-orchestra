import { useEffect, useState } from "react";

const WORKSPACE_PREFIX = "#workspace-";
const USERS_HASH = "#users";

/** The one screen `Shell` can show: the chat, one workspace, or the admin's users screen. */
export type Route =
  | { readonly screen: "chat" }
  | { readonly screen: "workspace"; readonly workspaceId: string }
  | { readonly screen: "users" };

function readRoute(): Route {
  const hash = window.location.hash;

  if (hash.startsWith(WORKSPACE_PREFIX)) {
    const workspaceId = hash.slice(WORKSPACE_PREFIX.length);

    if (workspaceId !== "") {
      return { screen: "workspace", workspaceId };
    }
  }

  if (hash === USERS_HASH) {
    return { screen: "users" };
  }

  return { screen: "chat" };
}

/**
 * The current screen, read from `window.location.hash` and kept in sync
 * with it.
 *
 * `WorkspaceListItem` and `ChatNavItem` (`web/src/app/ui`) are already
 * plain anchors - `href="#workspace-{id}"` and `href="#chat"` - because
 * Task 2 (`docs/plans/workspaces.md`) built the drawer without deciding how
 * following one would navigate. This is that decision: the hash is the
 * route, and `Shell` reads it instead of a router library switching what it
 * renders.
 *
 * A client-side router (`react-router` or similar) was the alternative.
 * This application has a handful of screens and no nested or parameterised
 * paths beyond the one workspace id the hash already carries, no need for
 * a back/forward-aware history stack beyond what the browser's own hash
 * navigation already gives for free, and no server-rendered routes to keep
 * in sync with. A router would add a dependency, a `<Routes>` tree, and a
 * new surface for `make guard-fsd`/`guard-ui` to reason about, to solve a
 * problem this hook solves in thirty lines with nothing new to import.
 */
export function useHashRoute(): Route {
  const [route, setRoute] = useState<Route>(() => readRoute());

  useEffect(() => {
    const onHashChange = (): void => {
      setRoute(readRoute());
    };

    window.addEventListener("hashchange", onHashChange);

    return () => {
      window.removeEventListener("hashchange", onHashChange);
    };
  }, []);

  return route;
}
