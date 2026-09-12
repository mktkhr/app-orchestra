import { useCallback, useState } from "react";

import { setUserPermissions, type Permission } from "@/shared/api/users";

import { keyOf, type PermissionGroup } from "./permissionGroups";
import { useLoadedPermissions } from "./useLoadedPermissions";

export type { PermissionGroup } from "./permissionGroups";
export { keyOf } from "./permissionGroups";

const SAVE_FAILURE = "権限の保存に失敗しました。";

/** One account's permission grid: every operation, grouped by service, and what the account currently holds. */
export interface PermissionGridState {
  readonly groups: readonly PermissionGroup[];
  /** Every operation the account currently holds, keyed by {@link keyOf}. */
  readonly granted: ReadonlyMap<string, Permission>;
  readonly loading: boolean;
  readonly error: string | null;
  readonly saving: boolean;
  readonly saveError: string | null;
  /** True immediately after a successful `save`, until the grid changes again. */
  readonly saved: boolean;
  /** Toggles one operation on or off. */
  readonly toggleOperation: (service: string, operationId: string) => void;
  /**
   * Grants every operation a service holds in one gesture when the service
   * is not fully granted yet, or revokes all of them when it already is
   * (docs/specs/auth.md section 4: "granting a whole service stays one
   * gesture").
   */
  readonly toggleService: (service: string) => void;
  /** Replaces the account's permissions wholesale with what the grid currently shows checked. */
  readonly save: () => Promise<void>;
}

/**
 * The admin's permission grid for one account (docs/plans/auth.md Task 5).
 * `useLoadedPermissions` loads the whole catalogue and this account's own
 * permissions; this hook adds the checkbox toggles and `save`, which
 * replaces the account's permissions wholesale with
 * `PUT /api/users/{id}/permissions`.
 */
export function usePermissionGrid(userId: string): PermissionGridState {
  const { groups, granted, setGranted, loading, error } = useLoadedPermissions(userId);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  const toggleOperation = useCallback(
    (service: string, operationId: string): void => {
      const key = keyOf({ service, operationId });

      setSaved(false);
      setGranted((current) => {
        const next = new Map(current);

        if (next.has(key)) {
          next.delete(key);
        } else {
          next.set(key, { service, operationId });
        }

        return next;
      });
    },
    [setGranted],
  );

  const toggleService = useCallback(
    (service: string): void => {
      const group = groups.find((candidate) => candidate.service === service);
      const serviceOperations = group?.operations ?? [];
      const allGranted = serviceOperations.every((operation) =>
        granted.has(keyOf({ service, operationId: operation.operationId })),
      );

      setSaved(false);
      setGranted((current) => {
        const next = new Map(current);

        for (const operation of serviceOperations) {
          const key = keyOf({ service, operationId: operation.operationId });

          if (allGranted) {
            next.delete(key);
          } else {
            next.set(key, { service, operationId: operation.operationId });
          }
        }

        return next;
      });
    },
    [groups, granted, setGranted],
  );

  const save = useCallback(async (): Promise<void> => {
    setSaving(true);
    setSaveError(null);

    try {
      await setUserPermissions(userId, Array.from(granted.values()));
      setSaved(true);
    } catch {
      setSaveError(SAVE_FAILURE);
    } finally {
      setSaving(false);
    }
  }, [userId, granted]);

  return {
    groups,
    granted,
    loading,
    error,
    saving,
    saveError,
    saved,
    toggleOperation,
    toggleService,
    save,
  };
}
