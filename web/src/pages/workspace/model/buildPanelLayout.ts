import type { Layout } from "react-grid-layout";

import type { WorkspacePanel } from "@/shared/api/client";

/**
 * `panels`, in `position` order, packed left to right into `columns` and
 * wrapped to a new row when the next panel would not fit
 * (`docs/specs/layout.md` L4/section 5). `columns` is 12 on the wide
 * breakpoint and 1 on the narrow one; passing 1 already gives every panel a
 * full-width row of its own; there is nothing beyond it, so no separate
 * "narrow" branch is needed.
 *
 * Pure and given a fixed `position` order, so the same panels always land
 * on the same layout - this task draws the grid, it does not let anyone
 * rearrange it (Task 2). Every item is `static`, which is `react-grid-layout`'s
 * own way of saying "not draggable, not resizable" for one item without a
 * second prop.
 */
export function buildPanelLayout(panels: readonly WorkspacePanel[], columns: number): Layout[] {
  const ordered = panels.toSorted((left, right) => left.position - right.position);
  const layout: Layout[] = [];
  let x = 0;
  let y = 0;
  let rowHeight = 0;

  for (const panel of ordered) {
    const width = Math.min(Math.max(panel.width, 1), columns);
    const height = Math.max(panel.height, 1);

    if (x > 0 && x + width > columns) {
      x = 0;
      y += rowHeight;
      rowHeight = 0;
    }

    layout.push({ i: panel.id, x, y, w: width, h: height, static: true });
    x += width;
    rowHeight = Math.max(rowHeight, height);
  }

  return layout;
}
