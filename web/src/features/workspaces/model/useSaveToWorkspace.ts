import { useRef, useState } from "react";

import { addPanel, createWorkspace, listWorkspaces } from "@/shared/api/client";
import type { Component, PlanResult, WorkspaceSummary } from "@/shared/api/client";
import { useSubmission } from "@/shared/lib/useSubmission";

// Re-exported so `SaveToWorkspaceControl`/`WorkspacePicker` can pull both
// the hook and the types they need from this one module - see the
// `max-dependencies` note on `SaveToWorkspaceControl`.
export type { Component, WorkspaceSummary };
export type Source = NonNullable<PlanResult["source"]>;

const LOAD_FAILURE = "ワークスペースの取得に失敗しました。";
/** Not a real workspace id - the picker's "make one now" option. */
export const NEW_WORKSPACE = "__new__";

/**
 * Which workspace the picker should start on: `defaultWorkspaceId` when the
 * list it just loaded actually has one by that id, otherwise the list's
 * first entry, otherwise "make one now". Split out of `load` so that
 * function - and `useSaveToWorkspace` around it - stays under
 * `max-lines-per-function`.
 */
function initialWorkspaceId(
  list: readonly WorkspaceSummary[],
  defaultWorkspaceId: string | undefined,
): string {
  const defaulted = list.find((workspace) => workspace.id === defaultWorkspaceId);

  return defaulted?.id ?? list[0]?.id ?? NEW_WORKSPACE;
}

interface SaveParams {
  readonly source: Source;
  readonly component: Component;
  readonly title: string;
  readonly workspaceId: string;
  readonly newWorkspaceName: string;
  readonly workspaces: readonly WorkspaceSummary[];
}

/**
 * The save itself: resolves which workspace to post to - creating one first
 * when "make one now" is picked - then `POST /api/workspaces/{id}/panels`.
 * Split out of `useSaveToWorkspace` so that hook stays under
 * `max-lines-per-function`; returns the workspace saved to, so the caller
 * can show its name. Typed by `id`/`name` alone, not the full
 * `WorkspaceSummary`: `createWorkspace`'s response carries no
 * `panelCount`, and nothing here needs it.
 */
async function performSave(params: SaveParams): Promise<{ id: string; name: string }> {
  const target =
    params.workspaceId === NEW_WORKSPACE
      ? await createWorkspace({ name: params.newWorkspaceName.trim() })
      : params.workspaces.find((workspace) => workspace.id === params.workspaceId);

  if (target === undefined) {
    throw new Error("no workspace selected");
  }

  await addPanel(target.id, {
    service: params.source.service,
    operationId: params.source.operationId,
    args: params.source.args ?? {},
    component: params.component,
    title: params.title.trim(),
  });

  return target;
}

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
 *
 * `defaultWorkspaceId`, when it names a workspace the load actually
 * returns, is selected instead of the list's first entry - asking from
 * inside a workspace (`docs/plans/workspaces.md` Task 6) defaults the save
 * control to that workspace rather than whichever one loaded first.
 */
export function useSaveToWorkspace(
  source: Source,
  component: Component,
  defaultTitle: string,
  defaultWorkspaceId?: string,
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
      setWorkspaceId(initialWorkspaceId(list, defaultWorkspaceId));
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
    const target = await performSave({
      source,
      component,
      title,
      workspaceId,
      newWorkspaceName,
      workspaces,
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
