import { useEffect, useState, type Dispatch, type SetStateAction } from "react";

import {
  getUserPermissions,
  listOperations,
  type Operation,
  type Permission,
} from "@/shared/api/users";

import { groupByService, keyOf, type PermissionGroup } from "./permissionGroups";

const LOAD_FAILURE = "権限の取得に失敗しました。";

interface LoadedPermissions {
  readonly groups: readonly PermissionGroup[];
  readonly granted: ReadonlyMap<string, Permission>;
  readonly setGranted: Dispatch<SetStateAction<ReadonlyMap<string, Permission>>>;
  readonly loading: boolean;
  readonly error: string | null;
}

/**
 * The read half of `usePermissionGrid`: the whole catalogue grouped by
 * service, and userId's own permissions loaded alongside it, both from one
 * effect keyed on userId. Split out of `usePermissionGrid` itself so that
 * hook stays a composition of this and its own toggle/save logic, rather
 * than one function doing both (eslint's function-complexity gate).
 */
export function useLoadedPermissions(userId: string): LoadedPermissions {
  const [operations, setOperations] = useState<readonly Operation[]>([]);
  const [granted, setGranted] = useState<ReadonlyMap<string, Permission>>(new Map());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const load = async (): Promise<void> => {
      setLoading(true);
      setError(null);

      try {
        const [everyOperation, held] = await Promise.all([
          listOperations(),
          getUserPermissions(userId),
        ]);

        if (cancelled) {
          return;
        }

        setOperations(everyOperation);
        setGranted(new Map(held.map((permission) => [keyOf(permission), permission])));
      } catch {
        if (!cancelled) {
          setError(LOAD_FAILURE);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void load();

    return () => {
      cancelled = true;
    };
  }, [userId]);

  return { groups: groupByService(operations), granted, setGranted, loading, error };
}
