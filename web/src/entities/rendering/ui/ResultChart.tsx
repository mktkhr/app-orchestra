import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { BarChart, LineChart, PieChart } from "@mui/x-charts";
import type { JSX } from "react";

import type { Row } from "../model/rows";

/** `View.chart.kind` (`docs/specs/dashboard.md` P4): which of the three shapes to draw. */
export type ChartKind = "bar" | "line" | "pie";

interface ResultChartProps {
  /** The same rows a table would render — grouped or not (`transform.ts` decides that, not this). */
  readonly data: readonly Row[];
  /** The field naming each point's category (a bar's x-axis tick, a line's point, a pie's slice). */
  readonly category: string;
  /** The field naming each point's magnitude. */
  readonly value: string;
  readonly kind: ChartKind;
  /**
   * The chart's text alternative (`docs/specs/dashboard.md` section 11). Drawn as a visible
   * `<figcaption>` rather than handed to `@mui/x-charts`' own `title` prop: that prop lands as an
   * `aria-label` on a `role="none"` container, a combination `aria-prohibited-attr` (axe, WCAG
   * 4.1.2) rejects outright — confirmed against the built chart's accessibility tree before this
   * component was written this way, not assumed from the library's docs.
   */
  readonly title: string;
  /**
   * The chart's pixel width/height, passed to whichever `@mui/x-charts` component draws `kind`.
   * Both default to this component's original fixed size (`DEFAULT_CHART_WIDTH`/`_HEIGHT`) so
   * Task 1's own tests — which render `ResultChart` with neither prop — keep seeing exactly what
   * they always have. A caller that knows its own container's size, such as a workspace panel
   * (`pages/workspace/ui/PanelResult.tsx`), passes what it knows instead
   * (`docs/plans/dashboard.md` Task 5).
   */
  readonly width?: number;
  readonly height?: number;
}

/**
 * One point this component actually draws: a finite value and a string category. The index
 * signature is for `@mui/x-charts`' `dataset` prop (`DatasetElementType<unknown>`,
 * `{ [key: string]: unknown }`) — a plain `{ category, value }` interface has no index signature
 * of its own and TypeScript does not treat "every property happens to fit" as satisfying one.
 */
interface ChartPoint {
  readonly category: string;
  readonly value: number;
  readonly [key: string]: string | number;
}

/**
 * Stands in for a row whose `category` value is missing, `null`, or not a string — the same
 * "no usable value" case `transform.ts`'s `UNGROUPED` bucket names, given the same single,
 * consistent label here rather than a different one per row (`String(42)`, `String({})`, ...).
 * Unlike `transform.ts`, this does not merge such rows into one point: this component draws one
 * mark per surviving row, grouped or not (Task 1's own note), so two such rows draw as two marks
 * that happen to share a label.
 */
const UNKNOWN_CATEGORY = "null";

// `@mui/x-charts` measures its container with `getComputedStyle` when no explicit size is
// given (`useChartDimensions`), which returns 0 in a test environment with no layout engine
// and draws nothing. These defaults sidestep that for a caller that does not care about its
// own size — Task 1's own tests, most notably — by reproducing what this component always
// drew before `width`/`height` became props. A caller that does care (a workspace panel)
// passes its own numbers instead (`docs/plans/dashboard.md` Task 5).
const DEFAULT_CHART_WIDTH = 320;
const DEFAULT_CHART_HEIGHT = 240;

/**
 * Rows in, chart points out. A row is dropped, not drawn as `NaN`, when its `value` field is
 * missing or not a finite number — a chart cannot place a mark with no honest position, and the
 * same call `transform.ts` makes for a non-numeric aggregate input (skip, don't fabricate).
 */
function chartPoints(data: readonly Row[], category: string, value: string): ChartPoint[] {
  const points: ChartPoint[] = [];

  for (const row of data) {
    const rawValue = row[value];

    if (typeof rawValue !== "number" || !Number.isFinite(rawValue)) {
      continue;
    }

    const rawCategory = row[category];

    points.push({
      category: typeof rawCategory === "string" ? rawCategory : UNKNOWN_CATEGORY,
      value: rawValue,
    });
  }

  return points;
}

/**
 * Draws `data` as a bar, line or pie chart (`docs/specs/dashboard.md` P4/P5): one field names
 * each point's category, one holds its value, `kind` picks the shape. This is the one place that
 * maps `kind` to the `@mui/x-charts` component that draws it.
 *
 * Empty input — no rows, or none left once a non-numeric `value` is filtered out — draws the same
 * "no rows" message `ResultTable` shows for an empty result, rather than a blank rectangle a
 * person has to guess the meaning of.
 */
export function ResultChart({
  data,
  category,
  value,
  kind,
  title,
  width = DEFAULT_CHART_WIDTH,
  height = DEFAULT_CHART_HEIGHT,
}: ResultChartProps): JSX.Element {
  const points = chartPoints(data, category, value);

  if (points.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        結果は0件です。
      </Typography>
    );
  }

  return (
    <Box component="figure" sx={{ m: 0 }}>
      <Typography component="figcaption" variant="subtitle2" gutterBottom>
        {title}
      </Typography>
      {kind === "bar" && (
        <BarChart
          dataset={points}
          xAxis={[{ dataKey: "category", scaleType: "band" }]}
          series={[{ dataKey: "value" }]}
          width={width}
          height={height}
        />
      )}
      {kind === "line" && (
        <LineChart
          dataset={points}
          xAxis={[{ dataKey: "category", scaleType: "point" }]}
          series={[{ dataKey: "value", showMark: true }]}
          width={width}
          height={height}
        />
      )}
      {kind === "pie" && (
        <PieChart
          series={[
            {
              data: points.map((point, index) => ({
                id: index,
                value: point.value,
                label: point.category,
              })),
            },
          ]}
          width={width}
          height={height}
        />
      )}
    </Box>
  );
}
