import { useAddedPanels, type AddedPanels } from "./useAddedPanels";
import { useWorkspace } from "./useWorkspace";
import type { WorkspaceDetail } from "@/shared/api/client";

interface WorkspacePageState {
  readonly workspace: WorkspaceDetail | null;
  readonly loading: boolean;
  readonly error: string | null;
  readonly addedPanels: AddedPanels;
}

/**
 * `useWorkspace` and `useAddedPanels` together, as one import - `WorkspacePage`
 * otherwise sits right at eslint's `max-dependencies` ceiling once it also
 * hosts `AddPanelControl` (`docs/plans/dashboard.md` Task 6). Each hook
 * still does exactly one thing; this only saves the page a second import
 * statement for two concerns it uses together.
 */
export function useWorkspacePage(workspaceId: string): WorkspacePageState {
  const { workspace, loading, error } = useWorkspace(workspaceId);
  const addedPanels = useAddedPanels();

  return { workspace, loading, error, addedPanels };
}
