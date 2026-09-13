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
 * on the same layout before anyone drags or resizes one.
 *
 * `interactive` (default `false`) sets the computed layout's own `static`
 * flag to its opposite: the narrow breakpoint stays `static` -
 * `react-grid-layout`'s own way of saying "not draggable, not resizable"
 * for an item without a second prop - because `docs/specs/layout.md`
 * section 5 gives it nothing to arrange (one column, one order), while the
 * wide breakpoint is `static: false` so `WorkspaceGrid`'s drag, resize and
 * keyboard handling (`docs/plans/layout.md` Task 2) can reach it.
 */
export function buildPanelLayout(
  panels: readonly WorkspacePanel[],
  columns: number,
  interactive = false,
): Layout[] {
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

    layout.push({ i: panel.id, x, y, w: width, h: height, static: !interactive });
    x += width;
    rowHeight = Math.max(rowHeight, height);
  }

  return layout;
}
