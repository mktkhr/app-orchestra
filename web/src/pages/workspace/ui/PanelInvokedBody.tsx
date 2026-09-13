import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";
import Stack from "@mui/material/Stack";
import type { JSX } from "react";

import { applyTransform, RenderedResult, ResultChart, rowsFromData } from "@/entities/rendering";
import type { InvokeResult, View } from "@/shared/api/client";
import { useElementSize } from "@/shared/lib/useElementSize";

interface PanelInvokedBodyProps {
  readonly loading: boolean;
  readonly error: string | null;
  readonly result: InvokeResult | null;
  readonly view: View | undefined;
  readonly title: string;
}

/**
 * A safe panel's own body, once `usePanelInvoke` has run: the loading
 * spinner, a failure, or the result itself - a chart when the panel's view
 * names one, `RenderedResult`'s table/detail otherwise. Split out of
 * `PanelResult` so that component stays readable once it also has to
 * choose between this and `PanelQuickAddBody` (`docs/specs/dashboard.md`
 * P14) - the choice itself lives there, this only draws one side of it.
 *
 * `ResultChart`'s size comes from its own container, measured
 * (`useElementSize`), not a guess from the viewport
 * (`docs/plans/dashboard.md` Task 5, `docs/specs/layout.md` AC-L-106):
 * `WorkspaceGrid` gives every panel a `react-grid-layout` box that is
 * exactly `width` columns by `height` rows, so a wider panel's box is
 * wider and this reads that directly. Zero (`useElementSize`'s own "not
 * measured yet" value - true in every unit test, since `happy-dom` has no
 * layout engine to fire a resize) falls back to `undefined`, which is
 * `ResultChart`'s own signal to use its original fixed default instead of
 * drawing at 0×0.
 */
export function PanelInvokedBody({
  loading,
  error,
  result,
  view,
  title,
}: PanelInvokedBodyProps): JSX.Element {
  const [chartRef, chartSize] = useElementSize<HTMLDivElement>();
  const chartWidth = chartSize.width > 0 ? chartSize.width : undefined;
  const chartHeight = chartSize.height > 0 ? chartSize.height : undefined;
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
    <>
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
            title={title}
            {...(chartWidth === undefined ? {} : { width: chartWidth })}
            {...(chartHeight === undefined ? {} : { height: chartHeight })}
          />
        </Box>
      )}
    </>
  );
}
