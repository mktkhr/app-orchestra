import { useEffect, useState } from "react";

import {
  createWorkspace,
  deleteWorkspace,
  listWorkspaces,
  type WorkspaceSummary,
} from "@/shared/api/client";

const LOAD_FAILURE = "ワークスペースの取得に失敗しました。";
const CREATE_FAILURE = "ワークスペースの作成に失敗しました。";
const DELETE_FAILURE = "ワークスペースの削除に失敗しました。";

interface Workspaces {
  readonly workspaces: readonly WorkspaceSummary[];
  readonly error: string | null;
  readonly create: (name: string) => Promise<void>;
  readonly remove: (id: string) => Promise<void>;
}

/**
 * The drawer's workspace list: loaded once on mount, and kept in sync with
 * what the person does to it locally rather than reloaded from the server -
 * `POST`/`DELETE /api/workspaces` already report the row that changed.
 *
 * A new workspace has no panels yet, so `create` can append the row
 * `POST /api/workspaces` returns (id, name) with `panelCount: 0` filled in,
 * without a second round trip to read it back.
 */
export function useWorkspaces(): Workspaces {
  const [workspaces, setWorkspaces] = useState<readonly WorkspaceSummary[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const load = async (): Promise<void> => {
      try {
        const data = await listWorkspaces();

        // Array.from into a plain array: openapi-fetch's array response type
        // and `readonly WorkspaceSummary[]` describe the same shape but are
        // not identical types (see WorkspacesList's doc in
        // shared/api/client.ts), and the response type is array-like rather
        // than iterable, so a spread cannot read it.
        setWorkspaces(Array.from(data));
      } catch {
        setError(LOAD_FAILURE);
      }
    };

    void load();
  }, []);

  const create = async (name: string): Promise<void> => {
    try {
      const created = await createWorkspace({ name });

      setWorkspaces((current) => [...current, { ...created, panelCount: 0 }]);
      setError(null);
    } catch {
      setError(CREATE_FAILURE);
    }
  };

  const remove = async (id: string): Promise<void> => {
    try {
      await deleteWorkspace(id);
      setWorkspaces((current) => current.filter((workspace) => workspace.id !== id));
      setError(null);
    } catch {
      setError(DELETE_FAILURE);
    }
  };

  return { workspaces, error, create, remove };
}
