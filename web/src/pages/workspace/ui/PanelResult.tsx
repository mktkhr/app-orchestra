import RefreshIcon from "@mui/icons-material/Refresh";
import Alert from "@mui/material/Alert";
import CircularProgress from "@mui/material/CircularProgress";
import IconButton from "@mui/material/IconButton";
import Stack from "@mui/material/Stack";
import useMediaQuery from "@mui/material/useMediaQuery";
import type { JSX } from "react";

import {
  applyTransform,
  Provenance,
  RenderedResult,
  ResultChart,
  rowsFromData,
} from "@/entities/rendering";
import { PanelCardShell, usePanelInvoke } from "@/entities/workspace";
import type { WorkspacePanel } from "@/shared/api/client";

interface PanelResultProps {
  readonly panel: WorkspacePanel;
}

/**
 * `ResultChart`'s size, for a panel: bigger than its own default on a screen wide enough for
 * that to fit beside the panel's other chrome, and small enough on a phone to sit inside a
 * `PanelCardShell`'s `CardContent` without pushing the card wider than the viewport - checked
 * at 375px (`docs/plans/dashboard.md` Task 5, `make guard-layout`). `600px` is MUI's own default
 * `sm` breakpoint (`app/ui/Shell.tsx` reads the same one, as `theme.breakpoints.up("sm")`, for
 * its drawer) - named directly here rather than through `useTheme` so this file's import count
 * stays under `import/max-dependencies` (`eslint`). `ResultChart`'s own default (320×240) is
 * sized for its own tests, rendering the chart alone with no card around it to leave room for;
 * a panel's card has padding this component's caller does not, so it needs its own number
 * rather than reusing that default outright.
 */
const WIDE_BREAKPOINT_QUERY = "(min-width:600px)";
const NARROW_CHART_SIZE = { width: 260, height: 200 } as const;
const WIDE_CHART_SIZE = { width: 560, height: 320 } as const;

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
 */
export function PanelResult({ panel }: PanelResultProps): JSX.Element {
  const { loading, refreshing, error, result, refresh } = usePanelInvoke(panel);
  const chartSize = useMediaQuery(WIDE_BREAKPOINT_QUERY) ? WIDE_CHART_SIZE : NARROW_CHART_SIZE;

  const view = panel.view;
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
      title={panel.title}
      action={
        <IconButton onClick={refresh} aria-label={refreshing ? "更新中" : "更新"}>
          {refreshing ? <CircularProgress size={20} /> : <RefreshIcon />}
        </IconButton>
      }
    >
      <Provenance
        source={{
          service: panel.service,
          // A saved Panel stores only the identifier (W2,
          // docs/specs/workspaces.md) - it never re-fetches the catalogue
          // just to find the service's display name (P9,
          // docs/specs/dashboard.md), so this reads the same as it always
          // has here: the identifier, unlike the chat/plan provenance
          // above it, which does carry the contract's own name
          // (DECISIONS.md, 2026-09-13).
          serviceDisplayName: panel.service,
          operationId: panel.operationId,
          args: panel.args,
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
        <ResultChart
          data={groupedRows ?? rowsFromData(result.data)}
          category={chart.category}
          value={chart.value}
          kind={chart.kind}
          title={panel.title}
          width={chartSize.width}
          height={chartSize.height}
        />
      )}
    </PanelCardShell>
  );
}
