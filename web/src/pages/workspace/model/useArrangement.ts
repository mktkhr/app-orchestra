import { useState } from "react";
import type { Layout } from "react-grid-layout";

import type { WorkspacePanel } from "@/shared/api/client";
import { patchPanel } from "@/shared/api/panels";

import {
  moveChanges,
  positionChanges,
  resizeChange,
  sizeChange,
  type MoveDirection,
  type PanelPositionChange,
  type PanelSizeChange,
} from "./arrangement";

interface PanelOverride {
  readonly position?: number;
  readonly width?: number;
  readonly height?: number;
}

export interface Arrangement {
  /** `panels`, with whatever a drag, a resize or the keyboard has changed since load, layered on top. */
  readonly panels: readonly WorkspacePanel[];
  /** Wired to `react-grid-layout`'s own `onDragStop`. */
  readonly onDragStop: (layout: Layout[]) => void;
  /** Wired to `react-grid-layout`'s own `onResizeStop`. */
  readonly onResizeStop: (layout: Layout[], oldItem: Layout, newItem: Layout) => void;
  /** The keyboard half's move (AC-L-103). */
  readonly moveTo: (panelId: string, direction: MoveDirection) => void;
  /** The keyboard half's resize (AC-L-103). */
  readonly resizeBy: (panelId: string, deltaWidth: number, deltaHeight: number) => void;
}

/**
 * Arranges `panels` by drag, by resize, or by keyboard, `PATCH`ing only the
 * panels whose geometry actually changed (`docs/specs/layout.md` section 6)
 * and showing the result immediately through a local override layered on
 * top of what the page loaded - the same split `PanelResult`'s own
 * `current` makes for a saved edit: the `PATCH` is what makes the change
 * true, this state is what makes it visible without a second round trip.
 *
 * The override is keyed by panel id, not replaced wholesale, so an
 * in-flight `PATCH` for one panel never clobbers a change already applied
 * to another. It resets when `workspaceId` changes, since `WorkspacePage`
 * can be reused across workspaces without remounting (`useWorkspace`'s own
 * comment) - adjusted during render, the pattern React's own docs give for
 * "some state needs to reset when a prop changes", rather than an effect
 * that would render once with the previous workspace's overrides still
 * applied before catching up a tick later.
 */
export function useArrangement(
  workspaceId: string,
  panels: readonly WorkspacePanel[],
): Arrangement {
  const [state, setState] = useState<{
    readonly workspaceId: string;
    readonly overrides: Record<string, PanelOverride>;
  }>({ workspaceId, overrides: {} });

  if (state.workspaceId !== workspaceId) {
    setState({ workspaceId, overrides: {} });
  }

  const overrides = state.workspaceId === workspaceId ? state.overrides : {};
  const setOverrides = (
    updater: (current: Record<string, PanelOverride>) => Record<string, PanelOverride>,
  ): void => {
    setState((current) => ({ workspaceId, overrides: updater(current.overrides) }));
  };

  const arranged = panels.map((panel) => ({ ...panel, ...overrides[panel.id] }));

  const applyPositions = (changes: readonly PanelPositionChange[]): void => {
    if (changes.length === 0) {
      return;
    }

    setOverrides((current) => {
      const next = { ...current };

      for (const change of changes) {
        next[change.id] = { ...next[change.id], position: change.position };
      }

      return next;
    });

    for (const change of changes) {
      void patchPanel(workspaceId, change.id, { position: change.position });
    }
  };

  const applySize = (change: PanelSizeChange): void => {
    setOverrides((current) => ({
      ...current,
      [change.id]: { ...current[change.id], width: change.width, height: change.height },
    }));
    void patchPanel(workspaceId, change.id, { width: change.width, height: change.height });
  };

  const onDragStop = (layout: Layout[]): void => {
    applyPositions(positionChanges(arranged, layout));
  };

  const onResizeStop = (_layout: Layout[], _oldItem: Layout, newItem: Layout): void => {
    applySize(sizeChange(newItem));
  };

  const moveTo = (panelId: string, direction: MoveDirection): void => {
    applyPositions(moveChanges(arranged, panelId, direction));
  };

  const resizeBy = (panelId: string, deltaWidth: number, deltaHeight: number): void => {
    const panel = arranged.find((candidate) => candidate.id === panelId);

    if (panel === undefined) {
      return;
    }

    applySize(resizeChange(panel, deltaWidth, deltaHeight));
  };

  return { panels: arranged, onDragStop, onResizeStop, moveTo, resizeBy };
}
