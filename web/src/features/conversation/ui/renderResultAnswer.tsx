import Paper from "@mui/material/Paper";
import type { JSX } from "react";

import { Provenance, RenderedResult, ResultChart, rowsFromData } from "@/entities/rendering";
import type { PlanResult } from "@/shared/api/client";

import type { SaveControlSlot } from "./answerSlots";

// Re-exported so `TurnList.tsx` can pull both slot types from this module
// instead of importing "./answerSlots" directly - one fewer distinct
// dependency there, which is what keeps it under oxlint's
// `import/max-dependencies` now that it also imports `CircularProgress`
// for the pending spinner (C4).
export type { ProposalSlot, SaveControlSlot } from "./answerSlots";

/**
 * The `kind: "result"` branches of `AnswerResult` - `table`, `detail` and
 * `chart` - split into their own file (and function) so `TurnList.tsx`
 * stays under eslint's `max-lines` and `AnswerResult` under
 * `max-lines-per-function`. Returns null for a `component`/`data`
 * combination this deployment's contract allows but that carries none of
 * what any of the three widgets needs, so the caller falls through to
 * `AnswerResult`'s own generic fallback instead of this one duplicating it.
 *
 * `chart` only reaches here when the contract declares `x-ui-hint.chart`
 * (`domain.Render`); the result then carries `view.chart` - axes only,
 * never a transform (AC-P-105). Each branch also draws the
 * `renderSaveControl` slot (AC-W-101) when the result has a `source` to
 * save - `chart` forwards its own `view` too, so saving carries the
 * contract's axes onto the new panel (AC-P-105's second half).
 */
export function renderResultAnswer(
  result: PlanResult,
  originalQuery: string,
  renderSaveControl?: SaveControlSlot,
): JSX.Element | null {
  if (result.component === "table" && result.source !== undefined && result.data !== undefined) {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        <Provenance source={result.source} />
        <RenderedResult component={result.component} data={result.data} fields={result.fields} />
        {renderSaveControl?.({
          source: result.source,
          component: result.component,
          defaultTitle: originalQuery,
        })}
      </Paper>
    );
  }

  if (result.component === "detail" && result.data !== undefined) {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        {result.source === undefined ? null : <Provenance source={result.source} />}
        <RenderedResult component={result.component} data={result.data} fields={result.fields} />
        {result.source === undefined
          ? null
          : renderSaveControl?.({
              source: result.source,
              component: result.component,
              defaultTitle: originalQuery,
            })}
      </Paper>
    );
  }

  if (
    result.component === "chart" &&
    result.source !== undefined &&
    result.data !== undefined &&
    result.view?.chart !== undefined
  ) {
    const chart = result.view.chart;

    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        <Provenance source={result.source} />
        <ResultChart
          data={rowsFromData(result.data)}
          category={chart.category}
          value={chart.value}
          kind={chart.kind}
          title={originalQuery}
        />
        {renderSaveControl?.({
          source: result.source,
          component: result.component,
          view: result.view,
          defaultTitle: originalQuery,
        })}
      </Paper>
    );
  }

  return null;
}
