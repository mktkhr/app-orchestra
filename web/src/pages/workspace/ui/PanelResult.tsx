import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";
import Stack from "@mui/material/Stack";
import { useState, type JSX } from "react";

import {
  applyTransform,
  Provenance,
  RenderedResult,
  ResultChart,
  rowsFromData,
} from "@/entities/rendering";
import { PanelCardShell, usePanelInvoke } from "@/entities/workspace";
import type { WorkspacePanel } from "@/shared/api/client";
import { useElementSize } from "@/shared/lib/useElementSize";

import { PanelActions } from "./PanelActions";

interface PanelResultProps {
  readonly workspaceId: string;
  readonly panel: WorkspacePanel;
  /** Threaded straight through to `PanelActions` - see its own doc comment. */
  readonly onMove?: ((panelId: string, direction: "previous" | "next") => void) | undefined;
  readonly onResize?:
    | ((panelId: string, deltaWidth: number, deltaHeight: number) => void)
    | undefined;
}

/**
 * `ResultChart`'s size, for a panel: the chart's own container, measured
 * (`useElementSize`), not a guess from the viewport (`docs/plans/dashboard.md`
 * Task 5, `docs/specs/layout.md` AC-L-106). `WorkspaceGrid` gives every panel
 * a `react-grid-layout` box that is exactly `width` columns by `height` rows,
 * so a wider panel's box is wider and this reads that directly - a media
 * query keyed to the viewport, which this used before, cannot tell a
 * `width: 12` panel from a `width: 6` one on the same screen; only the
 * panel's own rendered box can. Zero (`useElementSize`'s own "not measured
 * yet" value - true in every unit test, since `happy-dom` has no layout
 * engine to fire a resize) falls back to `undefined`, which is `ResultChart`'s
 * own signal to use its original fixed default instead of drawing at 0×0.
 */

/**
 * One workspace panel, fully assembled: `entities/workspace`'s card shell
 * around `entities/rendering`'s provenance and result widgets. This is the
 * one place that composes the two - both are entity slices, and
 * `make guard-fsd` forbids one entity importing a sibling, so `PanelCardShell`
 * stays presentational and `RenderedResult`/`Provenance` stay ignorant of
 * panels; a page, which may import either, is where they meet.
 *
 * The answer is asked for on mount (`usePanelInvoke`) rather than passed
 * in: a panel is a saved call, not a saved result (W2,
 * docs/specs/workspaces.md). A failed call shows its message inside this
 * card instead of throwing, so a workspace with one unreachable panel still
 * draws the rest of them (AC-W-106).
 *
 * The refresh control is an `IconButton`, not a disabled `Button
 * variant="contained"`: `make guard-layout` rejects a disabled contained
 * button outright (it has no edge the layout guard can see), and the
 * control must stay usable-looking anyway - AC-W-103 asks that it show it
 * is working, not that it lock itself. So it never disables; while
 * `refreshing` it swaps its icon for a small `CircularProgress` and its
 * label from "更新" to "更新中", and double-clicks are absorbed by
 * `usePanelInvoke`'s own in-flight guard rather than by disabling the
 * button.
 *
 * `panel` is copied into local state (`current`) rather than read straight
 * off the prop: `EditPanelControl` (P12) returns the panel as it reads
 * after a save, and this card draws that answer immediately - AC-P-110's
 * "edited then reloaded draws as edited" holds for the reload half because
 * the platform itself now has the change; this state is what makes it
 * true without one first, for the tab already open.
 */
export function PanelResult({
  workspaceId,
  panel,
  onMove,
  onResize,
}: PanelResultProps): JSX.Element {
  const [current, setCurrent] = useState(panel);
  const { loading, refreshing, error, result, refresh } = usePanelInvoke(current);
  const [chartRef, chartSize] = useElementSize<HTMLDivElement>();
  // `0` is `useElementSize`'s "not measured yet" value, not a real box size
  // (see the comment above) - `undefined` here lets `ResultChart` fall back
  // to its own fixed default instead of drawing at zero width or height.
  const chartWidth = chartSize.width > 0 ? chartSize.width : undefined;
  const chartHeight = chartSize.height > 0 ? chartSize.height : undefined;

  const view = current.view;
  const chart = view?.chart;
  const transform = view?.transform;

  // Applied here, where the result's rows arrive, before the code below decides which
  // component to draw them with - `applyTransform` itself has no opinion on that, and
  // `entities/rendering` exposes it for exactly this reason (see `transform.ts`'s own
  // comment: this call and the panel builder's preview are its only two callers).
  const groupedRows =
    result === null || transform === undefined
      ? undefined
      : applyTransform(rowsFromData(result.data), transform);

  return (
    <PanelCardShell
      title={current.title}
      action={
        <PanelActions
          workspaceId={workspaceId}
          panel={current}
          refreshing={refreshing}
          onRefresh={refresh}
          onSaved={setCurrent}
          onMove={onMove}
          onResize={onResize}
        />
      }
    >
      <Provenance
        source={{
          service: current.service,
          // A saved Panel stores only the identifier (W2,
          // docs/specs/workspaces.md) - it never re-fetches the catalogue
          // just to find the service's display name (P9,
          // docs/specs/dashboard.md), so this reads the same as it always
          // has here: the identifier, unlike the chat/plan provenance
          // above it, which does carry the contract's own name
          // (DECISIONS.md, 2026-09-13).
          serviceDisplayName: current.service,
          operationId: current.operationId,
          args: current.args,
        }}
      />
      {loading ? (
        <Stack direction="row" sx={{ justifyContent: "center", py: 2 }}>
          <CircularProgress size={24} aria-label="読み込み中" />
        </Stack>
      ) : null}
      {error === null ? null : <Alert severity="error">{error}</Alert>}
      {result === null ? null : chart === undefined ? (
        <RenderedResult
          component={result.component}
          data={groupedRows === undefined ? result.data : { rows: groupedRows }}
          fields={result.fields}
        />
      ) : (
        <Box ref={chartRef} sx={{ width: "100%", height: "100%" }}>
          <ResultChart
            data={groupedRows ?? rowsFromData(result.data)}
            category={chart.category}
            value={chart.value}
            kind={chart.kind}
            title={current.title}
            {...(chartWidth === undefined ? {} : { width: chartWidth })}
            {...(chartHeight === undefined ? {} : { height: chartHeight })}
          />
        </Box>
      )}
    </PanelCardShell>
  );
}
