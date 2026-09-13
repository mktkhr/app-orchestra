import useMediaQuery from "@mui/material/useMediaQuery";
import GridLayout from "react-grid-layout";
import type { JSX } from "react";

import type { WorkspacePanel } from "@/shared/api/client";
import { useElementSize } from "@/shared/lib/useElementSize";

import { buildPanelLayout } from "../model/buildPanelLayout";
import { useArrangement } from "../model/useArrangement";
import { PanelResult } from "./PanelResult";

// `react-grid-layout` ships its own stylesheet - grid item positioning
// (absolute placement, the transform each item is drawn at) has no other
// source. It draws no colour of its own besides the drag placeholder and
// the resize handle it paints once dragging and resizing are live (Task 2)
// - both were checked by hand against `make guard-layout`'s contrast rules
// in both colour schemes (see DECISIONS.md); everything else still comes
// from `PanelCardShell` and MUI's theme. `workspaceGrid.css`, imported
// right after, narrows one of its transitions - see that file's own
// comment for why (DECISIONS.md).
import "react-grid-layout/css/styles.css";
import "./workspaceGrid.css";

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

/**
 * `react-grid-layout` makes an entire item's box a drag handle unless told
 * otherwise (`GridItem`'s own `cancel` prop defaults to only
 * `.react-resizable-handle`) - so once `isDraggable` went live in Task 2,
 * every button inside a panel (refresh, edit, the keyboard arrange
 * control, even "拡大表示" and the table's own pagination) started a drag
 * on `mousedown` before its own `click` ever fired, and swallowed it: found
 * by `make guard-browser`'s own `dashboard.spec.ts`, whose edit journey
 * clicks "編集" and never saw the dialog open. This excludes every one of
 * those controls from starting a drag - a panel is still dragged by its
 * header or its blank card area, just not by anything that is itself a
 * control.
 */
const DRAGGABLE_CANCEL = "button, a, input, select, textarea";

/**
 * A workspace's panels, drawn in `react-grid-layout`'s grid
 * (`docs/specs/layout.md` L1/L4): twelve columns where there is room, one
 * where there is not, each panel spanning its own `width`/`height`, in
 * `position` order (`buildPanelLayout`).
 *
 * On the wide breakpoint a panel can be dragged and resized by pointer
 * (`isDraggable`/`isResizable`), and every panel's header carries an
 * arrange control (`PanelActions`) for the same two operations by
 * keyboard - both go through `useArrangement`, which `PATCH`es only the
 * panels whose geometry actually changed (`docs/specs/layout.md` section
 * 6) and keeps the result on screen through a local override, immediately,
 * without waiting on a reload.
 *
 * On the narrow breakpoint every panel spans the single column (AC-L-105)
 * and the grid is static - `docs/specs/layout.md` section 5: one column,
 * one order, nothing to arrange, and no way to drag the page sideways by
 * accident. `onMove`/`onResize` are left undefined there, which is what
 * tells `PanelActions` to draw no arrange control at all.
 *
 * The container's width is measured with `useElementSize` (the same hook
 * `PanelResult` reads for `ResultChart`, AC-L-106) rather than through
 * `react-grid-layout`'s own `WidthProvider`. `WidthProvider` renders once
 * at an unmeasured guess and corrects itself a tick later; Task 2 Step 4's
 * seeded panel is what caught that guess landing wide enough, on the
 * narrow breakpoint, to fail `guard-layout`'s sideways-scroll check
 * (AC-L-105) for the whole `react-grid-item` stylesheet's 200ms width
 * transition. `WidthProvider`'s own `measureBeforeMount` fixes the guess
 * by not rendering until measured - but `happy-dom` (`web/vite.config.ts`)
 * has no layout engine to ever fire that measurement, so it never renders
 * in a unit test at all, and every test here would hang rather than see a
 * panel. `useElementSize`'s own "not measured yet" value is `0`, the same
 * fallback `PanelResult` already treats as "not measured yet" rather than
 * "empty" - zero columns wide can never overflow sideways, so the guess
 * this replaces is safe instead of merely fast, and a test where nothing
 * is ever measured still renders every panel to look for.
 */
export function WorkspaceGrid({ workspaceId, panels }: WorkspaceGridProps): JSX.Element {
  const wide = useMediaQuery(WIDE_QUERY);
  const columns = wide ? WIDE_COLUMNS : NARROW_COLUMNS;
  const [containerRef, containerSize] = useElementSize<HTMLDivElement>();
  const {
    panels: arranged,
    onDragStop,
    onResizeStop,
    moveTo,
    resizeBy,
  } = useArrangement(workspaceId, panels);
  const layout = buildPanelLayout(arranged, columns, wide);
  const ordered = arranged.toSorted((left, right) => left.position - right.position);

  return (
    <div ref={containerRef}>
      <GridLayout
        className="layout"
        layout={layout}
        cols={columns}
        width={containerSize.width}
        rowHeight={ROW_HEIGHT_PX}
        margin={GRID_MARGIN}
        compactType={null}
        isDraggable={wide}
        isResizable={wide}
        draggableCancel={DRAGGABLE_CANCEL}
        onDragStop={onDragStop}
        onResizeStop={onResizeStop}
      >
        {ordered.map((panel) => (
          <div key={panel.id}>
            <PanelResult
              workspaceId={workspaceId}
              panel={panel}
              onMove={wide ? moveTo : undefined}
              onResize={wide ? resizeBy : undefined}
            />
          </div>
        ))}
      </GridLayout>
    </div>
  );
}
