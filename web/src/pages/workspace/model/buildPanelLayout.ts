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
/**
 * A panel as this function needs one. `width` and `height` are required on
 * the wire, and optional here on purpose: the type is where the tolerance
 * below is promised, rather than a cast at one call site pretending the
 * field is there. A response that lacks them - an older platform, a proxy,
 * anything partial - is a shape this function has to survive.
 */
type SizedPanel = Omit<WorkspacePanel, "width" | "height"> & {
  readonly width?: number;
  readonly height?: number;
};

/** value when it is a finite number, otherwise fallback. */
function finiteOr(value: number | undefined, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

export function buildPanelLayout(
  panels: readonly SizedPanel[],
  columns: number,
  interactive = false,
): Layout[] {
  const ordered = panels.toSorted((left, right) => left.position - right.position);
  const layout: Layout[] = [];
  let x = 0;
  let y = 0;
  let rowHeight = 0;

  for (const panel of ordered) {
    // A non-finite span is treated as the default rather than passed on.
    // width and height are required on the wire, but a response that lacks
    // them - an older platform, a proxy, anything partial - would otherwise
    // reach react-grid-layout as NaN, which computes a container 16px tall
    // for a 360px panel and draws every following element underneath it,
    // with nothing on screen saying so. A layout that cannot be computed
    // falls back to the shape a panel with no size has always drawn as
    // (docs/specs/layout.md section 3), which is wrong in a way a person
    // can see and recover from.
    const width = Math.min(Math.max(finiteOr(panel.width, columns), 1), columns);
    const height = Math.max(finiteOr(panel.height, 1), 1);

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
