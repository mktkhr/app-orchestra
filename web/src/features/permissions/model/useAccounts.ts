import { useEffect, useState } from "react";

import { listUsers, type UserAccount } from "@/shared/api/users";

const LOAD_FAILURE = "アカウントの取得に失敗しました。";

interface Accounts {
  readonly accounts: readonly UserAccount[];
  readonly loading: boolean;
  readonly error: string | null;
}

/**
 * Every account the platform knows, loaded once from `GET /api/users`
 * (docs/plans/auth.md Task 5). Admin only - a non-admin never reaches the
 * page this backs, since the drawer hides the entry, but the endpoint
 * itself refuses them with 403 either way (docs/specs/auth.md section 7).
 */
export function useAccounts(): Accounts {
  const [accounts, setAccounts] = useState<readonly UserAccount[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const load = async (): Promise<void> => {
      try {
        setAccounts(await listUsers());
      } catch {
        setError(LOAD_FAILURE);
      } finally {
        setLoading(false);
      }
    };

    void load();
  }, []);

  return { accounts, loading, error };
}
