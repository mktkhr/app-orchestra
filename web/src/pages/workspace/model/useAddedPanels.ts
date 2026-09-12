import { useState } from "react";

import type { WorkspacePanel } from "@/shared/api/client";

export interface AddedPanels {
  readonly panels: readonly WorkspacePanel[];
  readonly add: (panel: WorkspacePanel) => void;
}

/**
 * Panels added through `AddPanelControl` this visit (`docs/plans/dashboard.md`
 * Task 6, AC-P-102) - kept separately from `useWorkspace`'s own load, so
 * showing a panel just saved needs no second round trip to
 * `GET /api/workspaces/{id}` for a panel already in hand.
 */
export function useAddedPanels(): AddedPanels {
  const [panels, setPanels] = useState<readonly WorkspacePanel[]>([]);

  const add = (panel: WorkspacePanel): void => {
    setPanels((current) => [...current, panel]);
  };

  return { panels, add };
}
