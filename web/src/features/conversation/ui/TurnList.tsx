import Alert from "@mui/material/Alert";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX, ReactNode } from "react";

import {
  Provenance,
  RenderedResult,
  ResultChart,
  ResultChoice,
  ResultForm,
  rowsFromData,
} from "@/entities/rendering";
import type { Component, PlanResult } from "@/shared/api/client";

import type { Turn } from "../model/turn";

/**
 * What a `table`/`detail` result answer needs the save control drawn with -
 * exactly the provenance and widget it is already showing, plus the
 * question that produced it as the title's default.
 */
export interface SaveControlSlotProps {
  readonly source: NonNullable<PlanResult["source"]>;
  readonly component: Component;
  /** The answer's own `view` (only a chart-hinted result carries one), forwarded so saving it carries the axes too (AC-P-105). */
  readonly view?: PlanResult["view"];
  readonly defaultTitle: string;
}

interface TurnListProps {
  readonly turns: readonly Turn[];
  /**
   * Forwarded down to every `ResultForm`/`ResultChoice` this list renders.
   * Defined in `Conversation` (the state owner) and passed down rather than
   * imported, because both live in `entities/rendering` and cannot reach up
   * into `features/conversation` for the `Turn` type or `setTurns` itself.
   */
  readonly onFormSubmitted: (result: PlanResult) => void;
  /**
   * Draws a result turn's "save to a workspace" control. Left as a slot,
   * rather than this list importing `SaveToWorkspaceControl` itself,
   * because that control lives in `features/workspaces` and
   * `features/conversation` cannot import a sibling feature
   * (`make guard-fsd`). The page that owns both features
   * (`widgets/conversation`) supplies the slot; undefined draws no control
   * at all, which is what `Conversation`'s own tests render without.
   */
  readonly renderSaveControl?: ((props: SaveControlSlotProps) => ReactNode) | undefined;
}

/** The conversation's turns, in order: a question, then the answer to it. */
export function TurnList({
  turns,
  onFormSubmitted,
  renderSaveControl,
}: TurnListProps): JSX.Element {
  return (
    <Stack spacing={2}>
      {turns.map((turn, index) => (
        <TurnItem
          key={turn.id}
          turn={turn}
          nearestQuestion={nearestQuestion(turns, index)}
          onFormSubmitted={onFormSubmitted}
          renderSaveControl={renderSaveControl}
        />
      ))}
    </Stack>
  );
}

/**
 * The text of the nearest `role: "question"` turn before `index`, walking
 * backward from it.
 *
 * A `kind: "ask"` answer carries the planner's own disambiguation question
 * (`PlanResult.question`), not what the person actually typed, and
 * resubmitting `answers` to `/api/plan` needs exactly that original text
 * (`docs/specs/orchestration.md` section 6). Rather than thread a `query`
 * field through `Turn` - every producer of an answer turn would have to
 * remember to set it, including `ResultForm`'s `onSubmitted` path, which has
 * no query to give - this reads it back out of the turn list `Conversation`
 * already keeps. Walking backward instead of just taking `turns[index - 1]`
 * keeps this correct once a choice has already been answered once: the
 * turn right before a second `ask` may be another answer, but the nearest
 * question is still the one to resend.
 */
function nearestQuestion(turns: readonly Turn[], index: number): string {
  for (let cursor = index - 1; cursor >= 0; cursor -= 1) {
    const candidate = turns[cursor];

    if (candidate?.role === "question") {
      return candidate.text;
    }
  }

  return "";
}

interface TurnItemProps {
  readonly turn: Turn;
  readonly nearestQuestion: string;
  readonly onFormSubmitted: (result: PlanResult) => void;
  readonly renderSaveControl?: ((props: SaveControlSlotProps) => ReactNode) | undefined;
}

function TurnItem({
  turn,
  nearestQuestion: question,
  onFormSubmitted,
  renderSaveControl,
}: TurnItemProps): JSX.Element {
  if (turn.role === "question") {
    return (
      <Paper elevation={0} sx={{ p: 2, bgcolor: "action.hover" }}>
        <Typography variant="body1">{turn.text}</Typography>
      </Paper>
    );
  }

  return (
    <AnswerResult
      result={turn.result}
      originalQuery={question}
      onFormSubmitted={onFormSubmitted}
      renderSaveControl={renderSaveControl}
    />
  );
}

interface AnswerResultProps {
  readonly result: PlanResult;
  readonly originalQuery: string;
  readonly onFormSubmitted: (result: PlanResult) => void;
  readonly renderSaveControl?: ((props: SaveControlSlotProps) => ReactNode) | undefined;
}

/**
 * One answer turn's body, dispatched on `result.kind`/`result.component`.
 * Split out of `TurnItem` so each render path - `none`, table, detail,
 * form, choice, and the fallback - stays a small `if`, not one function
 * long enough to trip `max-lines-per-function`.
 */
function AnswerResult({
  result,
  originalQuery,
  onFormSubmitted,
  renderSaveControl,
}: AnswerResultProps): JSX.Element {
  if (result.kind === "none") {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        <Typography variant="overline" color="textSecondary">
          {result.kind}
        </Typography>
        <Typography variant="body1">{result.message}</Typography>
      </Paper>
    );
  }

  if (result.kind === "result") {
    const rendered = renderResultAnswer(result, originalQuery, renderSaveControl);

    if (rendered !== null) {
      return rendered;
    }
  }

  if (result.kind === "form" && result.schema !== undefined && result.target !== undefined) {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        <ResultForm
          schema={result.schema}
          initial={result.initial}
          target={result.target}
          onSubmitted={onFormSubmitted}
        />
      </Paper>
    );
  }

  if (
    result.kind === "ask" &&
    result.question !== undefined &&
    result.param !== undefined &&
    result.options !== undefined
  ) {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        <ResultChoice
          question={result.question}
          param={result.param}
          options={result.options}
          originalQuery={originalQuery}
          onAnswered={onFormSubmitted}
        />
      </Paper>
    );
  }

  // A response this deployment's contract allows but no branch above
  // matches - a `kind`/`component` combination missing one of its required
  // fields. Shows what kind of answer came back rather than nothing.
  return (
    <Paper elevation={1} sx={{ p: 2 }}>
      <Typography variant="overline" color="textSecondary">
        {result.kind}
      </Typography>
      <Alert severity="info" sx={{ mt: 1 }}>
        この回答（{result.kind}）には表示に必要な情報が含まれていません。
      </Alert>
    </Paper>
  );
}

/**
 * The `kind: "result"` branches of `AnswerResult` - `table`, `detail` and
 * `chart` - split into their own function so `AnswerResult` stays under
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
function renderResultAnswer(
  result: PlanResult,
  originalQuery: string,
  renderSaveControl?: (props: SaveControlSlotProps) => ReactNode,
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
