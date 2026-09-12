import { createContext, useContext } from "react";

import type { SessionUser } from "@/shared/api/client";

/** What the rest of the application can do with the session. */
export interface SessionContextValue {
  /** The signed-in user, or `null` when nobody is. */
  readonly user: SessionUser | null;
  /**
   * `"loading"` until the startup `GET /api/session` (docs/plans/auth.md
   * Task 4, Step 2) has answered. Nothing renders the chat, the bar or the
   * sign-in screen's absence while this is `"loading"` - only that one read
   * decides which of them is true.
   */
  readonly status: "loading" | "ready";
  /** Why the last sign-in attempt failed, or `null`. */
  readonly signInError: string | null;
  readonly signIn: (name: string, password: string) => Promise<void>;
  readonly signOut: () => Promise<void>;
}

export const SessionContext = createContext<SessionContextValue | null>(null);

/**
 * The signed-in user, and the means to sign in or out. Every reader sits
 * under the one `SessionProvider` `App` mounts, so a `null` context here is
 * a wiring mistake, not a state to render around.
 */
export function useSession(): SessionContextValue {
  const value = useContext(SessionContext);

  if (value === null) {
    throw new Error("useSession must be used inside a SessionProvider");
  }

  return value;
}
