import useMediaQuery from "@mui/material/useMediaQuery";
import GridLayout, { WidthProvider } from "react-grid-layout";
import type { JSX } from "react";

import type { WorkspacePanel } from "@/shared/api/client";

import { buildPanelLayout } from "../model/buildPanelLayout";
import { PanelResult } from "./PanelResult";

// `react-grid-layout` ships its own stylesheet - grid item positioning
// (absolute placement, the transform each item is drawn at) has no other
// source. It draws no colour of its own (no background, no border, no
// text) - every one of those still comes from `PanelCardShell` and MUI's
// theme, in both colour schemes, so `make guard-layout`'s contrast checks
// have nothing new to see from this import.
import "react-grid-layout/css/styles.css";

interface WorkspaceGridProps {
  readonly workspaceId: string;
  readonly panels: readonly WorkspacePanel[];
}

/**
 * MUI's own `sm` breakpoint (`app/ui/Shell.tsx` reads the same 600px, as does
 * `PanelResult`), named directly rather than through `useTheme` for the same
 * reason as there: this file's import count under `import/max-dependencies`.
 * `docs/specs/layout.md` section 5: one breakpoint, not a map of them - a
 * per-breakpoint layout is a number to keep in sync per panel that nobody
 * asked for.
 */
const WIDE_QUERY = "(min-width:600px)";
const WIDE_COLUMNS = 12;
const NARROW_COLUMNS = 1;

/**
 * A row's own height, in pixels, that `height` (in `Panel`) multiplies. Not
 * configurable per panel - `docs/specs/layout.md` section 5 draws `height`
 * as a count of these, not a pixel value of its own.
 */
const ROW_HEIGHT_PX = 360;
const GRID_MARGIN: [number, number] = [16, 16];

const AutoWidthGridLayout = WidthProvider(GridLayout);

/**
 * A workspace's panels, drawn in `react-grid-layout`'s grid
 * (`docs/specs/layout.md` L1/L4): twelve columns where there is room, one
 * where there is not, each panel spanning its own `width`/`height`, in
 * `position` order (`buildPanelLayout`).
 *
 * Read-only in this task (Task 1 of `docs/plans/layout.md`): `isDraggable`
 * and `isResizable` are both false, and every item in the computed layout
 * is `static` besides, so nothing here can be dragged or resized by a
 * pointer or a keyboard yet - and nothing is `PATCH`ed. That is Task 2,
 * which is also where the narrow breakpoint stops being merely
 * one-column-static-nothing-to-drag and starts being "dragging off"
 * (section 5) as its own decision.
 *
 * On the narrow breakpoint every panel spans the single column
 * (AC-L-105) - `buildPanelLayout` clamps every width down to `columns`,
 * so a `width: 12` panel becomes a full-width one instead of overflowing
 * sideways.
 */
export function WorkspaceGrid({ workspaceId, panels }: WorkspaceGridProps): JSX.Element {
  const wide = useMediaQuery(WIDE_QUERY);
  const columns = wide ? WIDE_COLUMNS : NARROW_COLUMNS;
  const layout = buildPanelLayout(panels, columns);
  const ordered = panels.toSorted((left, right) => left.position - right.position);

  return (
    <AutoWidthGridLayout
      className="layout"
      layout={layout}
      cols={columns}
      rowHeight={ROW_HEIGHT_PX}
      margin={GRID_MARGIN}
      compactType={null}
      isDraggable={false}
      isResizable={false}
    >
      {ordered.map((panel) => (
        <div key={panel.id}>
          <PanelResult workspaceId={workspaceId} panel={panel} />
        </div>
      ))}
    </AutoWidthGridLayout>
  );
}
