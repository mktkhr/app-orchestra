import { useEffect, useState } from "react";

import { getWorkspace, type WorkspaceDetail } from "@/shared/api/client";

const LOAD_FAILURE = "ワークスペースの取得に失敗しました。";

interface WorkspaceState {
  readonly workspace: WorkspaceDetail | null;
  readonly loading: boolean;
  readonly error: string | null;
}

/**
 * Reads one workspace and its panels on mount, and again whenever
 * `workspaceId` changes - following the drawer from one workspace straight
 * to another reuses the same `WorkspacePage` rather than remounting it.
 *
 * This only reads the workspace's row and its panel list
 * (`GET /api/workspaces/{id}`); no panel's answer is fetched here - each
 * `PanelCard` asks for its own through `usePanelInvoke`
 * (`entities/workspace`), independently of this load and of every other
 * panel's.
 */
export function useWorkspace(workspaceId: string): WorkspaceState {
  const [workspace, setWorkspace] = useState<WorkspaceDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const load = async (): Promise<void> => {
      setLoading(true);
      setError(null);
      setWorkspace(null);

      try {
        const data = await getWorkspace(workspaceId);

        if (!cancelled) {
          setWorkspace(data);
        }
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
  }, [workspaceId]);

  return { workspace, loading, error };
}
