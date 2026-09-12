import { useCallback, useEffect, useMemo, useState, type JSX, type ReactNode } from "react";

import {
  deleteSession,
  getSession,
  onUnauthorized,
  postSession,
  type SessionUser,
} from "@/shared/api/client";

import { SessionContext, type SessionContextValue } from "./sessionContext";

const SIGN_IN_FAILURE = "名前またはパスワードが正しくありません。";

interface SessionProviderProps {
  readonly children: ReactNode;
}

/**
 * Reads the session once, at startup, and again immediately after a
 * successful sign-in - never on an interval (docs/plans/auth.md Task 4,
 * Step 2). Every other 401, from any endpoint, arrives through
 * {@link onUnauthorized} and drops the user back to `null` without waiting
 * for whichever screen was open to notice on its own, which is how a 401
 * anywhere returns to the sign-in screen without a reload.
 */
export function SessionProvider({ children }: SessionProviderProps): JSX.Element {
  const [user, setUser] = useState<SessionUser | null>(null);
  const [status, setStatus] = useState<"loading" | "ready">("loading");
  const [signInError, setSignInError] = useState<string | null>(null);

  useEffect(() => {
    const load = async (): Promise<void> => {
      try {
        setUser(await getSession());
      } catch {
        setUser(null);
      } finally {
        setStatus("ready");
      }
    };

    void load();
  }, []);

  useEffect(
    () =>
      onUnauthorized(() => {
        setUser(null);
        setStatus("ready");
      }),
    [],
  );

  const signIn = useCallback(async (name: string, password: string): Promise<void> => {
    try {
      setUser(await postSession({ name, password }));
      setSignInError(null);
    } catch {
      setSignInError(SIGN_IN_FAILURE);
    }
  }, []);

  const signOut = useCallback(async (): Promise<void> => {
    try {
      await deleteSession();
    } finally {
      setUser(null);
    }
  }, []);

  const value = useMemo<SessionContextValue>(
    () => ({ user, status, signInError, signIn, signOut }),
    [user, status, signInError, signIn, signOut],
  );

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}
