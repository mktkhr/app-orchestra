import { useRef, useState } from "react";

import { addPanel, createWorkspace, listWorkspaces } from "@/shared/api/client";
import type { Component, PlanResult, WorkspaceSummary } from "@/shared/api/client";

import { useSubmission } from "./useSubmission";

// Re-exported so `SaveToWorkspaceControl`/`WorkspacePicker` can pull both
// the hook and the types they need from this one module - see the
// `max-dependencies` note on `SaveToWorkspaceControl`.
export type { Component, WorkspaceSummary };
export type Source = NonNullable<PlanResult["source"]>;

const LOAD_FAILURE = "ワークスペースの取得に失敗しました。";
/** Not a real workspace id - the picker's "make one now" option. */
export const NEW_WORKSPACE = "__new__";

export interface SaveToWorkspace {
  readonly open: boolean;
  readonly loaded: boolean;
  readonly loadError: string | null;
  readonly workspaces: readonly WorkspaceSummary[];
  readonly workspaceId: string;
  readonly newWorkspaceName: string;
  readonly title: string;
  readonly savedName: string | null;
  readonly submitting: boolean;
  readonly error: string | null;
  readonly handleOpen: () => void;
  readonly setWorkspaceId: (id: string) => void;
  readonly setNewWorkspaceName: (name: string) => void;
  readonly setTitle: (title: string) => void;
  readonly handleSave: () => void;
}

/**
 * State and submission for `SaveToWorkspaceControl`: which workspace to
 * save to (loaded lazily, on first open, from `listWorkspaces`), the
 * editable title, and the `POST /api/workspaces/{id}/panels` call itself -
 * creating a workspace first when "make one now" is picked. Pulled out of
 * the component so its own body stays about what to render
 * (`max-lines-per-function`).
 */
export function useSaveToWorkspace(
  source: Source,
  component: Component,
  defaultTitle: string,
): SaveToWorkspace {
  const [open, setOpen] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [workspaces, setWorkspaces] = useState<readonly WorkspaceSummary[]>([]);
  const [workspaceId, setWorkspaceId] = useState(NEW_WORKSPACE);
  const [newWorkspaceName, setNewWorkspaceName] = useState("");
  const [title, setTitle] = useState(defaultTitle);
  const [savedName, setSavedName] = useState<string | null>(null);
  const { submitting, error, run } = useSubmission();
  // `submitting` above is a render behind a click - a fast second click can
  // read it as still false. This ref is set synchronously the instant a
  // click is accepted, so the guard against a double save is exact (see
  // `ResultChoice`'s `inFlight` for the same pattern).
  const inFlight = useRef(false);

  const load = async (): Promise<void> => {
    try {
      const data = await listWorkspaces();
      const list = Array.from(data);

      setWorkspaces(list);
      setWorkspaceId(list[0]?.id ?? NEW_WORKSPACE);
    } catch {
      setLoadError(LOAD_FAILURE);
    } finally {
      setLoaded(true);
    }
  };

  const handleOpen = (): void => {
    setOpen(true);

    if (!loaded) {
      void load();
    }
  };

  const save = async (): Promise<void> => {
    const target =
      workspaceId === NEW_WORKSPACE
        ? await createWorkspace({ name: newWorkspaceName.trim() })
        : workspaces.find((workspace) => workspace.id === workspaceId);

    if (target === undefined) {
      throw new Error("no workspace selected");
    }

    await addPanel(target.id, {
      service: source.service,
      operationId: source.operationId,
      args: source.args ?? {},
      component,
      title: title.trim(),
    });

    setSavedName(target.name);
  };

  const handleSave = (): void => {
    if (inFlight.current || title.trim() === "") {
      return;
    }

    if (workspaceId === NEW_WORKSPACE && newWorkspaceName.trim() === "") {
      return;
    }

    inFlight.current = true;
    void run(save).finally(() => {
      inFlight.current = false;
    });
  };

  return {
    open,
    loaded,
    loadError,
    workspaces,
    workspaceId,
    newWorkspaceName,
    title,
    savedName,
    submitting,
    error,
    handleOpen,
    setWorkspaceId,
    setNewWorkspaceName,
    setTitle,
    handleSave,
  };
}
