import type { Layout } from "react-grid-layout";

import type { WorkspacePanel } from "@/shared/api/client";

/** `docs/specs/layout.md` section 3: width is 1-12 columns, height is at least 1 row. */
const MIN_WIDTH = 1;
const MAX_WIDTH = 12;
const MIN_HEIGHT = 1;

export interface PanelPositionChange {
  readonly id: string;
  readonly position: number;
}

export interface PanelSizeChange {
  readonly id: string;
  readonly width: number;
  readonly height: number;
}

/** Clamp width to the grid's own range - the same range the platform clamps to (Task 0). */
export function clampWidth(width: number): number {
  return Math.min(Math.max(width, MIN_WIDTH), MAX_WIDTH);
}

/** Clamp height to at least one row. */
export function clampHeight(height: number): number {
  return Math.max(height, MIN_HEIGHT);
}

/**
 * The `position` every panel in `layout` implies, read in the grid's own
 * reading order (top row first, left to right within a row), against what
 * `panels` already carry - only the ones whose value actually differs come
 * back (`docs/specs/layout.md` section 6: a drag that ends writes only the
 * panels whose geometry changed, not a renumbering of the whole
 * workspace - a request that rewrites six rows to change one can lose the
 * other five). A drag that ends back where it started, or a resize that
 * never reorders anyone, implies no position change at all and returns an
 * empty list.
 */
export function positionChanges(
  panels: readonly WorkspacePanel[],
  layout: readonly Layout[],
): PanelPositionChange[] {
  const ordered = layout.toSorted((left, right) => left.y - right.y || left.x - right.x);
  const byId = new Map(panels.map((panel) => [panel.id, panel]));
  const changes: PanelPositionChange[] = [];

  ordered.forEach((item, index) => {
    const panel = byId.get(item.i);

    if (panel !== undefined && panel.position !== index) {
      changes.push({ id: panel.id, position: index });
    }
  });

  return changes;
}

/**
 * The one panel a resize changed - its own width and height, clamped
 * (`docs/specs/layout.md` section 6: "a resize PATCHes one panel"). Nothing
 * else about the workspace is read from `newItem`'s siblings.
 */
export function sizeChange(newItem: Layout): PanelSizeChange {
  return { id: newItem.i, width: clampWidth(newItem.w), height: clampHeight(newItem.h) };
}

export type MoveDirection = "previous" | "next";

/**
 * Moving `panelId` one step earlier or later swaps its `position` with
 * whichever panel currently holds that neighbouring slot - the keyboard
 * half's move (`docs/specs/layout.md` AC-L-103), expressed the same way a
 * drag that swaps two adjacent panels is: exactly the two panels that moved,
 * and no renumbering of anyone else. Moving past either end of the order is
 * a no-op.
 */
export function moveChanges(
  panels: readonly WorkspacePanel[],
  panelId: string,
  direction: MoveDirection,
): PanelPositionChange[] {
  const ordered = panels.toSorted((left, right) => left.position - right.position);
  const index = ordered.findIndex((panel) => panel.id === panelId);

  if (index === -1) {
    return [];
  }

  const targetIndex = direction === "previous" ? index - 1 : index + 1;
  const current = ordered[index];
  const target = ordered.at(targetIndex);

  if (current === undefined || target === undefined || targetIndex < 0) {
    return [];
  }

  return [
    { id: current.id, position: target.position },
    { id: target.id, position: current.position },
  ];
}

/**
 * Resizing `panel` by keyboard: its width and height, each nudged by one
 * column or row and clamped - the keyboard half's resize (AC-L-103),
 * touching only this one panel, the same as a pointer resize does.
 */
export function resizeChange(
  panel: WorkspacePanel,
  deltaWidth: number,
  deltaHeight: number,
): PanelSizeChange {
  return {
    id: panel.id,
    width: clampWidth(panel.width + deltaWidth),
    height: clampHeight(panel.height + deltaHeight),
  };
}
